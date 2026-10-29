//go:build windows

package mic

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var mm = syscall.NewLazyDLL("winmm.dll")

type Device struct {
	ID   uint32
	Name string
}
type caps struct {
	Manufacturer, Product uint16
	Version               uint32
	Name                  [32]uint16
	Formats               uint32
	Channels, Reserved    uint16
	Support               uint32
}
type format struct {
	Tag, Channels      uint16
	Rate, Bytes        uint32
	Align, Bits, Extra uint16
}
type header struct {
	Data             uintptr
	Length, Recorded uint32
	User             uintptr
	Flags, Loops     uint32
	Next, Reserved   uintptr
}

func Devices(input bool) []Device {
	prefix := "waveOut"
	if input {
		prefix = "waveIn"
	}
	count, _, _ := mm.NewProc(prefix + "GetNumDevs").Call()
	var devices []Device
	for i := uint32(0); i < uint32(count); i++ {
		var c caps
		r, _, _ := mm.NewProc(prefix+"GetDevCapsW").Call(uintptr(i), uintptr(unsafe.Pointer(&c)), unsafe.Sizeof(c))
		if r == 0 {
			devices = append(devices, Device{i, syscall.UTF16ToString(c.Name[:])})
		}
	}
	return devices
}
func check(op string, result uintptr) error {
	if result == 0 {
		return nil
	}
	var message [256]uint16
	mm.NewProc("waveInGetErrorTextW").Call(result, uintptr(unsafe.Pointer(&message[0])), 256)
	return fmt.Errorf("%s: %s (%d)", op, syscall.UTF16ToString(message[:]), result)
}

//go:uintptrescapes
func call(op string, args ...uintptr) error {
	r, _, _ := mm.NewProc(op).Call(args...)
	return check(op, r)
}

type Session struct {
	captureOnly         bool
	neural              bool
	stop, done          chan struct{}
	once                sync.Once
	peak                atomic.Uint32
	mu                  sync.Mutex
	err                 error
	deep                atomic.Bool
	gate                atomic.Int32
	inputGain           atomic.Int32
	noiseReduction      atomic.Int32
	monitorVolume       atomic.Int32
	smartVoice          atomic.Bool
	samplePreview       bool
	previewStage        atomic.Int32
	playbackSubmissions atomic.Uint32
}

func (s *Session) Stop() { s.once.Do(func() { close(s.stop) }); <-s.done }
func (s *Session) Active() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}
func (s *Session) Error() error                  { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *Session) Level() int                    { return int(s.peak.Load()) }
func (s *Session) Configure(deep bool, gate int) { s.deep.Store(deep); s.gate.Store(int32(gate)) }
func (s *Session) SetInputGain(gain int)         { s.inputGain.Store(int32(gain)) }
func (s *Session) SetNoiseReduction(strength int) {
	s.noiseReduction.Store(int32(max(0, min(100, strength))))
}
func (s *Session) SetMonitorVolume(volume int) {
	if !s.captureOnly && !s.samplePreview {
		volume = min(volume, 50)
	}
	s.monitorVolume.Store(int32(max(0, min(100, volume))))
}
func (s *Session) SetSmartVoice(enabled bool) { s.smartVoice.Store(enabled) }

// StartCapture processes the microphone locally; no audio is recorded or sent to another application.
func StartCapture(input Device, deep bool, gate int) (*Session, error) {
	return startSession(input, nil, deep, gate, false)
}

// StartMonitor plays the processed signal to an explicitly selected local output.
func StartMonitor(input, output Device, deep bool, gate int) (*Session, error) {
	found := false
	for _, d := range Devices(false) {
		if d == output {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("output tidak ditemukan; pilih headset atau speaker lagi")
	}
	return startSession(input, &output, deep, gate, false)
}
func startSession(input Device, output *Device, deep bool, gate int, neural bool) (*Session, error) {
	return startSessionWithConfig(input, output, deep, gate, neural, nil)
}

func startSessionWithConfig(input Device, output *Device, deep bool, gate int, neural bool, config *Config) (*Session, error) {
	if strings.Contains(strings.ToLower(input.Name), "cable") {
		return nil, fmt.Errorf("pilih mikrofon fisik sebagai sumber")
	}
	found := false
	for _, d := range Devices(true) {
		if d == input {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("mikrofon terputus; muat ulang perangkat")
	}
	s := &Session{stop: make(chan struct{}), done: make(chan struct{}), captureOnly: true}
	s.neural = neural
	outputID := uint32(0)
	if output != nil {
		s.captureOnly = false
		outputID = output.ID
	}
	s.Configure(deep, gate)
	s.SetMonitorVolume(70)
	if config != nil {
		s.samplePreview = config.PreviewSample
		s.SetInputGain(config.InputGain)
		s.SetNoiseReduction(config.NoiseReduction)
		s.SetMonitorVolume(config.MonitorVolume)
		s.SetSmartVoice(config.SmartVoice)
	}
	ready := make(chan error, 1)
	go func() {
		err := s.run(input.ID, outputID, ready)
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
	}()
	if err := <-ready; err != nil {
		<-s.done
		return nil, err
	}
	return s, nil
}
func (s *Session) run(input, output uint32, ready chan<- error) (err error) {
	signaled := false
	defer func() {
		if !signaled {
			ready <- err
		}
	}()
	var neural *neuralSuppressor
	if s.neural {
		neural, err = openNeuralSuppressor()
		if err != nil {
			return
		}
		defer neural.Close()
	}
	var in, out uintptr
	kernel := syscall.NewLazyDLL("kernel32.dll")
	event, _, eventErr := kernel.NewProc("CreateEventW").Call(0, 1, 0, 0)
	if event == 0 {
		return fmt.Errorf("audio event: %w", eventErr)
	}
	defer kernel.NewProc("CloseHandle").Call(event)
	f := format{Tag: 1, Channels: 1, Rate: 48000, Bytes: 96000, Align: 2, Bits: 16}
	if !s.captureOnly {
		if err = call("waveOutOpen", uintptr(unsafe.Pointer(&out)), uintptr(output), uintptr(unsafe.Pointer(&f)), event, 0, 0x50000); err != nil {
			return
		}
		defer mm.NewProc("waveOutClose").Call(out)
	}
	if err = call("waveInOpen", uintptr(unsafe.Pointer(&in)), uintptr(input), uintptr(unsafe.Pointer(&f)), event, 0, 0x50000); err != nil {
		return
	}
	defer mm.NewProc("waveInClose").Call(in)
	// Pinned Go storage remains valid while the Windows driver owns each header.
	// 10ms capture/playback blocks reduce delayed self-monitoring.
	const blockSamples = 480
	// Sample preview can tolerate a larger capture reserve while a game
	// briefly occupies the CPU; playback is queued as one complete sample.
	var inputData [12][blockSamples]int16
	var outputData [8][blockSamples]int16
	var inputs [12]header
	var outputs [8]header
	var pin runtime.Pinner
	defer pin.Unpin()
	preparedIn, preparedOut := 0, 0
	defer func() {
		mm.NewProc("waveInReset").Call(in)
		if out != 0 {
			mm.NewProc("waveOutReset").Call(out)
		}
		for i := 0; i < preparedIn; i++ {
			mm.NewProc("waveInUnprepareHeader").Call(in, uintptr(unsafe.Pointer(&inputs[i])), unsafe.Sizeof(header{}))
		}
		for i := 0; i < preparedOut; i++ {
			mm.NewProc("waveOutUnprepareHeader").Call(out, uintptr(unsafe.Pointer(&outputs[i])), unsafe.Sizeof(header{}))
		}
	}()
	for i := range inputs {
		pin.Pin(&inputData[i][0])
		pin.Pin(&inputs[i])
		inputs[i] = header{Data: uintptr(unsafe.Pointer(&inputData[i][0])), Length: blockSamples * 2}
		if err = call("waveInPrepareHeader", in, uintptr(unsafe.Pointer(&inputs[i])), unsafe.Sizeof(header{})); err != nil {
			return
		}
		preparedIn++
		if i < len(outputs) {
			pin.Pin(&outputData[i][0])
			pin.Pin(&outputs[i])
			outputs[i] = header{Data: uintptr(unsafe.Pointer(&outputData[i][0])), Length: blockSamples * 2}
		}
		if out != 0 && !s.samplePreview && i < len(outputs) {
			if err = call("waveOutPrepareHeader", out, uintptr(unsafe.Pointer(&outputs[i])), unsafe.Sizeof(header{})); err != nil {
				return
			}
			preparedOut++
		}
		if err = call("waveInAddBuffer", in, uintptr(unsafe.Pointer(&inputs[i])), unsafe.Sizeof(header{})); err != nil {
			return
		}
	}
	if err = call("waveInStart", in); err != nil {
		return
	}
	ready <- nil
	signaled = true
	p := Processor{}
	p.Neural = s.neural
	// Wake on driver completion, rather than relying on Go timer polling
	// while a game is loading. Timeout keeps stop/device-loss checks bounded.
	wait := kernel.NewProc("WaitForSingleObject")
	resetEvent := kernel.NewProc("ResetEvent")
	playbackFade := 0.0
	var previewPCM []int16
	previewWarmup := 9600
	if s.samplePreview {
		previewPCM = make([]int16, 0, 3*48000)
		s.previewStage.Store(0)
		defer func() { clear(previewPCM) }()
	}
	lastInput := time.Now()
	var outputBlocked time.Time
	next := 0
	for {
		select {
		case <-s.stop:
			return nil
		default:
		}
		if result, _, waitErr := wait.Call(event, 10); result == 0xffffffff {
			return fmt.Errorf("audio event wait: %w", waitErr)
		}
		// Reset before inspecting all headers, so a completion racing with
		// the reset is still observed in the header flags.
		resetEvent.Call(event)
		// Consume capture headers in order, including after a scheduling stall.
		for consumed := 0; consumed < len(inputs); consumed++ {
			i := next
			if atomic.LoadUint32(&inputs[i].Flags)&1 == 0 {
				break
			}
			j := -1
			for k := range outputs {
				if atomic.LoadUint32(&outputs[k].Flags)&16 == 0 {
					j = k
					break
				}
			}
			if j < 0 {
				// Output and capture clocks are independent. Keep this completed
				// input header pending until playback releases a buffer, instead
				// of treating a short scheduling delay as a device failure.
				if outputBlocked.IsZero() {
					outputBlocked = time.Now()
				}
				if time.Since(outputBlocked) > time.Second {
					return fmt.Errorf("output audio tidak merespons; pilih ulang headset")
				}
				break
			}
			outputBlocked = time.Time{}
			lastInput = time.Now()
			next = (next + 1) % len(inputs)
			n := int(inputs[i].Recorded / 2)
			if n > blockSamples {
				n = blockSamples
			}
			copy(outputData[j][:n], inputData[i][:n])
			if neural != nil {
				p.SpeechProbability = neural.Process(outputData[j][:n])
			}
			p.Deep = s.deep.Load()
			p.GateDB = float64(s.gate.Load())
			p.InputGain = float64(s.inputGain.Load()) / 100
			p.NoiseReduction = int(s.noiseReduction.Load())
			p.SmartVoice = s.smartVoice.Load()
			s.peak.Store(uint32(p.Process(outputData[j][:n]) * 100))
			outputs[j].Length = uint32(n * 2)
			if s.samplePreview {
				skip := min(previewWarmup, n)
				previewWarmup -= skip
				if previewWarmup == 0 {
					s.previewStage.Store(1)
				}
				previewPCM = append(previewPCM, outputData[j][skip:skip+min(n-skip, cap(previewPCM)-len(previewPCM))]...)
				if len(previewPCM) == cap(previewPCM) {
					// Break the acoustic feedback loop BEFORE any sample is played.
					if err = call("waveInStop", in); err != nil {
						return
					}
					if err = call("waveInReset", in); err != nil {
						return
					}
					s.previewStage.Store(2)
					return s.playSample(out, event, previewPCM)
				}
			} else if n > 0 && out != 0 {
				// Playback volume is independent of capture gain and the processed meter.
				for k := 0; k < n; k++ {
					playbackFade = min(1, playbackFade+1.0/2400)
					outputData[j][k] = int16(float64(outputData[j][k]) * float64(s.monitorVolume.Load()) / 100 * playbackFade)
				}
				if err = call("waveOutWrite", out, uintptr(unsafe.Pointer(&outputs[j])), unsafe.Sizeof(header{})); err != nil {
					return
				}
			}
			inputs[i].Recorded = 0
			if err = call("waveInAddBuffer", in, uintptr(unsafe.Pointer(&inputs[i])), unsafe.Sizeof(header{})); err != nil {
				return
			}
		}
		if time.Since(lastInput) > 3*time.Second {
			return fmt.Errorf("tidak ada data mikrofon; periksa koneksi dan izin mikrofon Windows")
		}
	}
}
