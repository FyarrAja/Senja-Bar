package mic

import (
	"math"
	"math/rand"
	"testing"
)

func TestNoiseReducedWhileTargetVoiceIsPresent(t *testing.T) {
	const rate = 48000
	const length = rate * 3
	random := rand.New(rand.NewSource(42))
	data := make([]int16, length)
	target := make([]float64, length)
	for i := range data {
		if i >= rate {
			target[i] = .14*math.Sin(2*math.Pi*1500*float64(i)/rate) + .06*math.Sin(2*math.Pi*3000*float64(i)/rate)
		}
		noise := .009*random.NormFloat64() + .008*math.Sin(2*math.Pi*375*float64(i)/rate)
		data[i] = int16((target[i] + noise) * 32767)
	}
	original := append([]int16(nil), data...)
	var reducer noiseReducer
	for start := 0; start < len(data); start += 960 {
		reducer.Process(data[start:min(start+960, len(data))], 85, .02)
	}
	var before, after, targetEnergy, cross float64
	for i := rate * 2; i < length; i++ {
		wanted := target[i-noiseFFTSize]
		got := float64(data[i]) / 32768
		before += math.Pow(float64(original[i-noiseFFTSize])/32768-wanted, 2)
		targetEnergy += wanted * wanted
		cross += wanted * got
	}
	gain := cross / targetEnergy
	for i := rate * 2; i < length; i++ {
		after += math.Pow(float64(data[i])/32768-gain*target[i-noiseFFTSize], 2)
	}
	improvement := 10 * math.Log10(before/after)
	if improvement < 5 || gain < .8 {
		t.Fatalf("insufficient active-speech suppression %.2fdB, target gain %.3f", improvement, gain)
	}
	t.Logf("synthetic mixed signal: noise improvement %.2fdB, target gain %.3f", improvement, gain)
}
func TestNoiseStreamingChunkBoundariesAndSilence(t *testing.T) {
	data := make([]int16, 48000)
	for i := range data {
		data[i] = int16(1000 * math.Sin(float64(i)*.13))
	}
	a, b := append([]int16(nil), data...), append([]int16(nil), data...)
	var first, second noiseReducer
	first.Process(a, 80, .02)
	for i := 0; i < len(b); i += 137 {
		second.Process(b[i:min(i+137, len(b))], 80, .02)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("chunk-dependent audio")
		}
	}
	var quiet noiseReducer
	clear(data)
	quiet.Process(data, 100, .02)
	for _, sample := range data {
		if sample != 0 {
			t.Fatal("silence generated sound")
		}
	}
}
func BenchmarkNoiseSuppression(b *testing.B) {
	var reducer noiseReducer
	samples := make([]int16, 960)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reducer.Process(samples, 85, .02)
	}
}

func TestQuietVoicedSignalIsNotLearnedAsNoise(t *testing.T) {
	const rate = 48000
	data := make([]int16, rate*3)
	for i := range data {
		data[i] = int16(.012 * 32767 * math.Sin(2*math.Pi*1500*float64(i)/rate))
	}
	var reducer noiseReducer
	for start := 0; start < len(data); start += 960 {
		reducer.Process(data[start:min(start+960, len(data))], 90, .02)
	}
	var energy float64
	for _, sample := range data[rate*2:] {
		energy += math.Pow(float64(sample)/32768, 2)
	}
	gain := math.Sqrt(energy/rate) / (.012 / math.Sqrt2)
	if gain < .85 {
		t.Fatalf("quiet voice absorbed into noise model: gain=%.3f", gain)
	}
}
