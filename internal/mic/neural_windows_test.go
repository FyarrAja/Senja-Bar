//go:build windows

package mic

import (
	"debug/pe"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func neuralTestModel(t testing.TB) *neuralSuppressor {
	t.Helper()
	if os.Getenv("SENJA_NEURAL_DLL") == "" {
		t.Skip("explicit native model integration")
	}
	model, err := openNeuralSuppressor()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(model.Close)
	return model
}
func TestNeuralStreamingAndKeyboardNoise(t *testing.T) {
	const length = 48000 * 3
	source := make([]int16, length)
	random := rand.New(rand.NewSource(71))
	for i := range source {
		amplitude := .001
		if i%9600 < 240 {
			amplitude += .22 * math.Exp(-float64(i%9600)/35)
		}
		source[i] = int16(max(-.99, min(.99, amplitude*random.NormFloat64())) * 32767)
	}
	a, b := append([]int16(nil), source...), append([]int16(nil), source...)
	first, second := neuralTestModel(t), neuralTestModel(t)
	for i := 0; i < len(a); i += 960 {
		prob := first.Process(a[i:min(i+960, len(a))])
		if prob < 0 || prob > 1 || math.IsNaN(prob) {
			t.Fatal("invalid VAD")
		}
	}
	for i := 0; i < len(b); i += 137 {
		second.Process(b[i:min(i+137, len(b))])
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("chunk dependent at %d", i)
		}
	}
	var before, after float64
	for i := 48000; i < len(a); i++ {
		before += float64(source[i]) * float64(source[i])
		after += float64(a[i]) * float64(a[i])
	}
	improvement := 10 * math.Log10(before/max(1, after))
	if improvement < 6 {
		t.Fatalf("neural keyboard suppression too weak %.2fdB", improvement)
	}
	t.Logf("RNNoise synthetic keyboard-only attenuation %.2fdB", improvement)
}
func TestNeuralSilenceAndLifecycle(t *testing.T) {
	model := neuralTestModel(t)
	samples := make([]int16, 960)
	for i := 0; i < 50; i++ {
		model.Process(samples)
	}
	for _, s := range samples {
		if s != 0 {
			t.Fatal("model generated sound in silence")
		}
	}
	model.Close()
	model.Close()
}

func TestNeuralUsesOnlyWindowsRuntimeLibraries(t *testing.T) {
	path := os.Getenv("SENJA_NEURAL_DLL")
	if path == "" {
		t.Skip("native model integration")
	}
	file, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	imports, err := file.ImportedLibraries()
	if err != nil {
		t.Fatal(err)
	}
	for _, library := range imports {
		name := strings.ToLower(library)
		if name != "kernel32.dll" && name != "msvcrt.dll" && name != "ucrtbase.dll" && !strings.HasPrefix(name, "api-ms-win-") {
			t.Fatalf("unexpected extra runtime %s", library)
		}
	}
	t.Logf("runtime imports: %v", imports)
}
func BenchmarkNeuralSuppression(b *testing.B) {
	model := neuralTestModel(b)
	samples := make([]int16, 960)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range samples {
			samples[j] = int16(7000*math.Sin(2*math.Pi*180*float64(i*960+j)/48000) + 2000*math.Sin(2*math.Pi*720*float64(i*960+j)/48000))
		}
		model.Process(samples)
	}
}

func readSpeechFixture(t testing.TB) []int16 {
	t.Helper()
	path := os.Getenv("SENJA_SPEECH_FIXTURE")
	if path == "" {
		t.Skip("optional generated Windows TTS fixture")
	}
	wave, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(wave) < 12 || string(wave[:4]) != "RIFF" || string(wave[8:12]) != "WAVE" {
		t.Fatal("invalid WAV")
	}
	var data []byte
	valid := false
	for p := 12; p+8 <= len(wave); {
		size := int(binary.LittleEndian.Uint32(wave[p+4 : p+8]))
		next := p + 8 + size
		if next > len(wave) {
			t.Fatal("truncated WAV")
		}
		if string(wave[p:p+4]) == "fmt " && size >= 16 {
			valid = binary.LittleEndian.Uint16(wave[p+8:]) == 1 && binary.LittleEndian.Uint16(wave[p+10:]) == 1 && binary.LittleEndian.Uint32(wave[p+12:]) == 48000 && binary.LittleEndian.Uint16(wave[p+22:]) == 16
		}
		if string(wave[p:p+4]) == "data" {
			data = wave[p+8 : next]
		}
		p = next + size%2
	}
	if !valid || len(data) == 0 {
		t.Fatal("fixture must be 48k mono PCM16")
	}
	pcm := make([]int16, len(data)/2)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return pcm
}
func TestNeuralGeneratedSpeechAndTypedClicks(t *testing.T) {
	speech := readSpeechFixture(t)
	model := neuralTestModel(t)
	mixed := make([]int16, 48000+len(speech)+48000)
	random := rand.New(rand.NewSource(9))
	var before, after float64
	for i := range mixed {
		value := 0.
		if i >= 48000 && i < 48000+len(speech) {
			value = float64(speech[i-48000]) * .7
		}
		if i%7200 < 160 {
			value += 6500 * random.NormFloat64() * math.Exp(-float64(i%7200)/25)
		}
		mixed[i] = int16(max(-32768, min(32767, value)))
	}
	maxProbability := 0.
	for i := 0; i < len(mixed); i += 960 {
		maxProbability = max(maxProbability, model.Process(mixed[i:min(i+960, len(mixed))]))
	}
	// Measure speech audibility with a loose energy bound, not toy-tone identity.
	for _, sample := range speech {
		before += float64(sample) * float64(sample) * .49
	}
	for _, sample := range mixed[48000 : 48000+len(speech)+960] {
		after += float64(sample) * float64(sample)
	}
	ratio := math.Sqrt(after / max(1, before))
	if ratio < .35 || maxProbability < .5 {
		t.Fatalf("speech suppressed excessively: level ratio %.3f, max VAD %.3f", ratio, maxProbability)
	}
	t.Logf("generated TTS + typed clicks: speech-region RMS ratio %.3f, max VAD %.3f", ratio, maxProbability)
}
