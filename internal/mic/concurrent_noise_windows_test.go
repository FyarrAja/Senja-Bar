//go:build windows

package mic

import (
	"math"
	"math/rand"
	"testing"
)

func TestGeneratedSpeechWithFanAndTypingResidual(t *testing.T) {
	fan, typing, speech, voice, quiet := measureConcurrentNoise(t)
	t.Logf("estimated fan reduction %.1f dB, typing reduction %.1f dB, speech gain %.2f, clean voice gain %.2f, quiet noise reduction %.1f dB", fan, typing, speech, voice, quiet)
	if speech < .8 || voice < .8 || typing < 3 || quiet < 40 {
		t.Fatalf("speech/noise regression: fan %.1f dB, typing %.1f dB, speech %.2f, voice %.2f, quiet %.1f dB", fan, typing, speech, voice, quiet)
	}
}

func measureConcurrentNoise(t *testing.T) (float64, float64, float64, float64, float64) {
	t.Helper()
	speech := readSpeechFixture(t)
	cleanModel, mixedModel := neuralTestModel(t), neuralTestModel(t)
	cleanDSP := Processor{Neural: true, SmartVoice: true, GateDB: -38, InputGain: 1.05}
	mixedDSP := Processor{Neural: true, SmartVoice: true, GateDB: -38, InputGain: 1.05}
	const warmup, frame = 48000, 480
	delay := 1440
	length := warmup + len(speech) + warmup
	cleanOut, mixedOut := make([]int16, length), make([]int16, length)
	cleanInput, mixedInput := make([]int16, length), make([]int16, length)
	random := rand.New(rand.NewSource(314))
	fanState := 0.
	for i := 0; i < length; i++ {
		voice := 0.
		if i >= warmup && i < warmup+len(speech) {
			voice = .7 * float64(speech[i-warmup])
		}
		fanState = .975*fanState + .025*random.NormFloat64()
		fan := 32767 * (.018*math.Sin(2*math.Pi*110*float64(i)/48000) + .015*fanState)
		click := 0.
		position := i % 7200
		if position < 160 {
			click = 7000 * random.NormFloat64() * math.Exp(-float64(position)/25)
		}
		cleanInput[i] = int16(max(-32768, min(32767, voice)))
		mixedInput[i] = int16(max(-32768, min(32767, voice+fan+click)))
	}
	for start := 0; start+frame <= length; start += frame {
		clean := append([]int16(nil), cleanInput[start:start+frame]...)
		mixed := append([]int16(nil), mixedInput[start:start+frame]...)
		cleanDSP.SpeechProbability = cleanModel.Process(clean)
		mixedDSP.SpeechProbability = mixedModel.Process(mixed)
		cleanDSP.Process(clean)
		mixedDSP.Process(mixed)
		copy(cleanOut[start:], clean)
		copy(mixedOut[start:], mixed)
	}
	var cross, cleanEnergy, sourceCross, sourceEnergy float64
	for i := warmup + delay; i < warmup+len(speech); i++ {
		cross += float64(cleanOut[i]) * float64(mixedOut[i])
		cleanEnergy += float64(cleanOut[i]) * float64(cleanOut[i])
		sourceCross += float64(cleanInput[i-delay]) * float64(cleanOut[i])
		sourceEnergy += float64(cleanInput[i-delay]) * float64(cleanInput[i-delay])
	}
	speechGain := cross / max(1, cleanEnergy)
	voiceGain := sourceCross / max(1, sourceEnergy)
	var inputFan, inputClicks, outputFan, outputClicks float64
	for i := warmup + delay; i < warmup+len(speech); i++ {
		inputResidual := float64(mixedInput[i-delay]) - float64(cleanInput[i-delay])
		outputResidual := float64(mixedOut[i]) - speechGain*float64(cleanOut[i])
		if (i-delay)%7200 < 300 {
			inputClicks += inputResidual * inputResidual
			outputClicks += outputResidual * outputResidual
		} else {
			inputFan += inputResidual * inputResidual
			outputFan += outputResidual * outputResidual
		}
	}
	var quietInput, quietOutput float64
	for i := warmup + len(speech) + 4800; i < length; i++ {
		input := float64(mixedInput[i-delay])
		output := float64(mixedOut[i])
		quietInput += input * input
		quietOutput += output * output
	}
	return 10 * math.Log10(max(1, inputFan)/max(1, outputFan)), 10 * math.Log10(max(1, inputClicks)/max(1, outputClicks)), speechGain, voiceGain, 10 * math.Log10(max(1, quietInput)/max(1, quietOutput))
}
