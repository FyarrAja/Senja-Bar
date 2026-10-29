package mic

import (
	"math"
	"testing"
)

func tone(p *Processor, amplitude, hz float64) float64 {
	var energy float64
	for block := 0; block < 100; block++ {
		s := make([]int16, 960)
		for i := range s {
			s[i] = int16(amplitude * 32767 * math.Sin(2*math.Pi*hz*float64(block*960+i)/48000))
		}
		p.Process(s)
		if block >= 50 {
			for _, x := range s {
				energy += float64(x) * float64(x)
			}
		}
	}
	return math.Sqrt(energy/(50*960)) / 32768
}
func TestQuietNoiseAttenuatedAndSpeechPreserved(t *testing.T) {
	quiet := tone(&Processor{GateDB: -38}, .003, 1000)
	voice := tone(&Processor{GateDB: -38}, .12, 1000)
	if quiet > .0001 || voice < .07 {
		t.Fatalf("gate quiet=%g voice=%g", quiet, voice)
	}
}
func TestDeepAddsLowFrequencyBody(t *testing.T) {
	plain := tone(&Processor{GateDB: -45}, .1, 160)
	deep := tone(&Processor{GateDB: -45, Deep: true}, .1, 160)
	if deep < plain*1.2 {
		t.Fatalf("body unchanged plain=%g deep=%g", plain, deep)
	}
}
func TestInputGainChangesProcessedSignal(t *testing.T) {
	low := tone(&Processor{GateDB: -50, InputGain: .5}, .1, 1000)
	standard := tone(&Processor{GateDB: -50, InputGain: 1}, .1, 1000)
	high := tone(&Processor{GateDB: -50, InputGain: 1.5}, .1, 1000)
	if low >= standard*.6 || high <= standard*1.35 {
		t.Fatalf("input gain ineffective: %g %g %g", low, standard, high)
	}
}
func TestSilenceAndOverloadAreFinite(t *testing.T) {
	p := Processor{Deep: true, GateDB: -38}
	s := make([]int16, 960)
	if p.Process(s) != 0 {
		t.Fatal("silence generated audio")
	}
	for i := range s {
		if i%2 == 0 {
			s[i] = 32767
		} else {
			s[i] = -32768
		}
	}
	if peak := p.Process(s); math.IsNaN(peak) || peak > 1 {
		t.Fatalf("invalid peak %g", peak)
	}
}

func TestGateDoesNotLeakLongBackgroundTail(t *testing.T) {
	p := Processor{GateDB: -32}
	for block := 0; block < 10; block++ {
		samples := make([]int16, 960)
		for i := range samples {
			samples[i] = int16(.15 * 32767 * math.Sin(2*math.Pi*1000*float64(i)/48000))
		}
		p.Process(samples)
	}
	var residual float64
	for block := 0; block < 10; block++ {
		samples := make([]int16, 960)
		for i := range samples {
			samples[i] = int16(.006 * 32767 * math.Sin(2*math.Pi*1000*float64(i)/48000))
		}
		p.Process(samples)
		if block >= 8 {
			for _, s := range samples {
				residual += float64(s) * float64(s)
			}
		}
	}
	if rms := math.Sqrt(residual/(2*960)) / 32768; rms > .0002 {
		t.Fatalf("background tail still open after 160ms: %g", rms)
	}
}
