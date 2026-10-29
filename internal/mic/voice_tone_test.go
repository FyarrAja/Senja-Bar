package mic

import (
	"math"
	"testing"
)

func TestQuietNeuralSyllablesSurviveUncertainVAD(t *testing.T) {
	p := Processor{GateDB: -38, SmartVoice: true, Neural: true, SpeechProbability: .04}
	if rms := tone(&p, .008, 180); rms < .005 {
		t.Fatalf("quiet voiced signal chopped: %g", rms)
	}
	p.SpeechProbability = 0
	shortGap := make([]int16, 4800)
	p.Process(shortGap)
	if p.gain < .99 {
		t.Fatalf("short VAD dropout closed gate: %g", p.gain)
	}
}

func TestDeepBodyDoesNotLiftRumbleAsMuchAsVoice(t *testing.T) {
	ratio := func(hz float64) float64 {
		return tone(&Processor{GateDB: -60, Deep: true}, .08, hz) / tone(&Processor{GateDB: -60}, .08, hz)
	}
	if voice, rumble := ratio(180), ratio(35); voice < 1.2 || voice <= rumble {
		t.Fatalf("voice body=%g rumble=%g", voice, rumble)
	}
}

func TestVoiceCompressorOnlyReducesLoudPeaks(t *testing.T) {
	p := Processor{GateDB: -38, SmartVoice: true, Neural: true, SpeechProbability: .9, Deep: true}
	render := func(amplitude float64) {
		for b := 0; b < 150; b++ {
			s := make([]int16, 480)
			for i := range s {
				s[i] = int16(amplitude * 32767 * math.Sin(2*math.Pi*180*float64(b*480+i)/48000))
			}
			p.Process(s)
		}
	}
	render(.5)
	if p.compressionGain >= .95 || p.compressionGain <= 0 {
		t.Fatalf("loud compression=%g", p.compressionGain)
	}
	render(.02)
	if p.compressionGain < .99 || p.compressionGain > 1.000001 {
		t.Fatalf("quiet recovery=%g", p.compressionGain)
	}
	silence := make([]int16, 48000)
	p.Process(silence)
	for _, s := range silence[len(silence)-480:] {
		if s != 0 {
			t.Fatal("tone leaves delayed voice in silence")
		}
	}
}
