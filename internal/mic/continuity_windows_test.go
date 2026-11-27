//go:build windows

package mic

import (
	"math"
	"math/rand"
	"testing"
)

func TestGeneratedSpeechFullPipelineContinuity(t *testing.T) {
	speech := readSpeechFixture(t)
	model := neuralTestModel(t)
	p := Processor{Neural: true, SmartVoice: true, GateDB: -38, InputGain: 1.05}
	neuralSignal := make([]int16, 0, len(speech))
	processedSignal := make([]int16, 0, len(speech))
	low, neuralLow, gateLow, voiced, longest, run := 0, 0, 0, 0, 0, 0
	for start := 0; start+480 <= len(speech); start += 480 {
		frame := append([]int16(nil), speech[start:start+480]...)
		var inEnergy float64
		for _, s := range frame {
			inEnergy += float64(s) * float64(s)
		}
		p.SpeechProbability = model.Process(frame)
		neuralSignal = append(neuralSignal, frame...)
		var neuralEnergy float64
		for _, s := range frame {
			neuralEnergy += float64(s) * float64(s)
		}
		p.Process(frame)
		processedSignal = append(processedSignal, frame...)
		var outEnergy float64
		for _, s := range frame {
			outEnergy += float64(s) * float64(s)
		}
		inRMS := math.Sqrt(inEnergy/480) / 32768
		ratio := math.Sqrt(outEnergy / max(1, inEnergy))
		if inRMS > .008 {
			voiced++
			if math.Sqrt(neuralEnergy/max(1, inEnergy)) < .25 {
				neuralLow++
			}
			if math.Sqrt(outEnergy/max(1, neuralEnergy)) < .25 && neuralEnergy > 0 {
				gateLow++
			}
			if ratio < .25 {
				low++
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		} else {
			run = 0
		}
	}
	t.Logf("synthetic TTS active=%d total low=%d RNNoise low=%d DSP low=%d longest=%d ms", voiced, low, neuralLow, gateLow, longest*10)
	bestLag, bestScore := 0, -1.
	for lag := 0; lag <= 1920; lag += 120 {
		var cross, srcEnergy, outEnergy float64
		for i := 0; i+lag < len(neuralSignal); i += 8 {
			a, b := float64(speech[i]), float64(neuralSignal[i+lag])
			cross += a * b
			srcEnergy += a * a
			outEnergy += b * b
		}
		score := cross / math.Sqrt(max(1, srcEnergy*outEnergy))
		if score > bestScore {
			bestScore, bestLag = score, lag
		}
	}
	shiftedLow := 0
	for start := 0; start+bestLag+480 <= len(speech); start += 480 {
		var inEnergy, outEnergy float64
		for i := 0; i < 480; i++ {
			inEnergy += float64(speech[start+i]) * float64(speech[start+i])
			outEnergy += float64(processedSignal[start+bestLag+i]) * float64(processedSignal[start+bestLag+i])
		}
		if math.Sqrt(inEnergy/480)/32768 > .008 && math.Sqrt(outEnergy/max(1, inEnergy)) < .25 {
			shiftedLow++
		}
	}
	t.Logf("best RNNoise lag=%d samples (%.1f ms), correlation=%.3f, shifted low frames=%d", bestLag, float64(bestLag)/48, bestScore, shiftedLow)
	if bestScore < .9 || shiftedLow > 5 {
		t.Fatalf("generated speech has unexpected model/dropout damage: correlation=%.3f low frames=%d", bestScore, shiftedLow)
	}
}

func TestGeneratedSpeechWithTypingKeepsSyllables(t *testing.T) {
	speech := readSpeechFixture(t)
	cleanModel, noisyModel := neuralTestModel(t), neuralTestModel(t)
	cleanDSP := Processor{Neural: true, SmartVoice: true, GateDB: -38, InputGain: 1.05}
	noisyDSP := Processor{Neural: true, SmartVoice: true, GateDB: -38, InputGain: 1.05}
	random := rand.New(rand.NewSource(99))
	low, active, longest, run := 0, 0, 0, 0
	for start := 0; start+480 <= len(speech); start += 480 {
		clean := append([]int16(nil), speech[start:start+480]...)
		noisy := make([]int16, 480)
		for i, sample := range clean {
			position := (start + i) % 7200
			click := 0.
			if position < 160 {
				click = 6500 * random.NormFloat64() * math.Exp(-float64(position)/25)
			}
			noisy[i] = int16(max(-32768, min(32767, float64(sample)+click)))
		}
		cleanDSP.SpeechProbability = cleanModel.Process(clean)
		noisyDSP.SpeechProbability = noisyModel.Process(noisy)
		cleanDSP.Process(clean)
		noisyDSP.Process(noisy)
		var cross, energy float64
		for i, sample := range clean {
			cross += float64(sample) * float64(noisy[i])
			energy += float64(sample) * float64(sample)
		}
		if math.Sqrt(energy/480)/32768 > .008 {
			active++
			if cross/max(1, energy) < .3 {
				low++
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		} else {
			run = 0
		}
	}
	t.Logf("synthetic typing during speech: active=%d weak=%d longest weak run=%d ms", active, low, longest*10)
	if longest > 5 || low > active/20 {
		t.Fatalf("typing interrupts generated speech: %d/%d weak frames, longest %d ms", low, active, longest*10)
	}
}
