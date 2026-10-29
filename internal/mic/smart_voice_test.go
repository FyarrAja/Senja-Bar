package mic

import (
	"math"
	"math/rand"
	"testing"
)

func TestSmartVoiceMakesQuietVoicedSpeechEasier(t *testing.T) {
	plain := tone(&Processor{GateDB: -32, NoiseReduction: 85}, .012, 160)
	smart := tone(&Processor{GateDB: -32, NoiseReduction: 85, SmartVoice: true, Deep: true}, .012, 160)
	if smart < .015 || smart < plain*2 {
		t.Fatalf("quiet voiced level not restored: plain=%g smart=%g", plain, smart)
	}
}
func TestKeyboardTransientReductionDuringVoicedSignal(t *testing.T) {
	const rate = 48000
	const length = rate * 3
	random := rand.New(rand.NewSource(27))
	data := make([]int16, length)
	target := make([]float64, length)
	for i := range data {
		if i >= rate {
			target[i] = .08 * math.Sin(2*math.Pi*1500*float64(i)/rate)
		}
		click := 0.
		relative := i % (rate / 5)
		if i >= rate && relative < 240 {
			click = .32 * random.NormFloat64() * math.Exp(-float64(relative)/35)
		}
		data[i] = int16(max(-.99, min(.99, target[i]+click+.001*random.NormFloat64())) * 32767)
	}
	process := func(smart bool) (float64, float64) {
		out := append([]int16(nil), data...)
		reducer := noiseReducer{Smart: smart}
		for start := 0; start < len(out); start += 960 {
			reducer.Process(out[start:min(start+960, len(out))], 85, .02)
		}
		var cross, signal, errorEnergy float64
		for i := rate * 2; i < length; i++ {
			want := target[i-noiseFFTSize]
			got := float64(out[i]) / 32768
			cross += want * got
			signal += want * want
		}
		gain := cross / signal
		for i := rate * 2; i < length; i++ {
			errorEnergy += math.Pow(float64(out[i])/32768-gain*target[i-noiseFFTSize], 2)
		}
		return errorEnergy, gain
	}
	plain, _ := process(false)
	smart, gain := process(true)
	improvement := 10 * math.Log10(plain/smart)
	if improvement < 3 || gain < .85 {
		t.Fatalf("typing improvement %.2fdB, target gain %.3f", improvement, gain)
	}
	t.Logf("synthetic typing during voiced tone: residual improvement %.2fdB, target gain %.3f", improvement, gain)
}
func TestAutoDoesNotRaiseSilentNoise(t *testing.T) {
	p := Processor{GateDB: -32, NoiseReduction: 85, SmartVoice: true, Deep: true}
	samples := make([]int16, 960)
	for i := 0; i < 100; i++ {
		p.Process(samples)
	}
	if p.levelGain > 1.30001 {
		t.Fatalf("auto boosted silence: %g", p.levelGain)
	}
	for _, s := range samples {
		if s != 0 {
			t.Fatal("silence generated audio")
		}
	}
}

func TestStableMakeupDoesNotChaseSyllables(t *testing.T) {
	p := Processor{GateDB: -38, SmartVoice: true, Neural: true, SpeechProbability: .8}
	for block := 0; block < 150; block++ {
		level := .012
		if block%10 < 5 {
			level = .16
		}
		samples := make([]int16, 960)
		for i := range samples {
			samples[i] = int16(level * 32767 * math.Sin(2*math.Pi*180*float64(i)/48000))
		}
		p.Process(samples)
		if block > 50 && math.Abs(p.levelGain-1.3) > .001 {
			t.Fatalf("gain chased loudness: %g", p.levelGain)
		}
	}
}
