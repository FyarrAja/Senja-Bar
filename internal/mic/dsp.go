package mic

import "math"

// Processor runs on mono PCM at 48 kHz. Gate attenuates quiet background
// between phrases; it does not promise to separate simultaneous speakers.
type Processor struct {
	Deep                                                 bool
	GateDB                                               float64
	InputGain                                            float64
	NoiseReduction                                       int
	SmartVoice                                           bool
	Neural                                               bool
	SpeechProbability                                    float64
	levelGain, levelTarget                               float64
	noise                                                noiseReducer
	prev, hp, bass, envelope, gain                       float64
	hold                                                 int
	subBass, silky, compressionEnvelope, compressionGain float64
	lowMid, presenceLow, presenceHigh                    float64
}

func (p *Processor) Process(samples []int16) float64 {
	threshold := math.Pow(10, p.GateDB/20)
	inputGain := p.InputGain
	if inputGain <= 0 {
		inputGain = 1
	}
	if p.NoiseReduction > 0 && !p.Neural {
		p.noise.Smart = p.SmartVoice
		p.noise.Process(samples, p.NoiseReduction, min(.02, threshold/inputGain))
	} else if p.noise.ready {
		p.noise = noiseReducer{}
	}
	if p.SmartVoice && !p.Neural {
		// Follow the learned room floor instead of forcing soft speech over a
		// fixed high threshold. Keyboard rejection happens before this gate.
		threshold = max(.004, min(threshold, p.noise.floorRMS*inputGain*.9))
	}
	// Fixed makeup replaces syllable-chasing AGC. It never ramps up in a gap.
	p.levelTarget = 1
	if p.SmartVoice {
		p.levelTarget = 1.3
	}
	if p.levelGain == 0 {
		p.levelGain = 1
	}
	var peak float64
	for i, sample := range samples {
		x := float64(sample) / 32768 * inputGain
		p.hp = .9896 * (p.hp + x - p.prev) // 80 Hz rumble removal.
		p.prev = x
		p.envelope = math.Max(math.Abs(p.hp), p.envelope*.999)
		open := p.envelope > threshold
		if p.Neural && p.SmartVoice {
			open = p.SpeechProbability >= .15 || p.SpeechProbability > .03 && p.envelope > .002
		}
		if open {
			p.hold = 2880
			if p.Neural {
				p.hold = 11520
			}
		}
		target := .015
		if p.Neural && p.SmartVoice {
			// RNNoise already removes noise; a gentle expander avoids chopping
			// quiet syllables when its VAD briefly becomes uncertain.
			target = .12
		}
		if p.hold > 0 {
			target = 1
			p.hold--
		}
		coefficient := .0015
		if p.Neural {
			coefficient = .00012
		}
		if target > p.gain {
			coefficient = .018
		}
		p.gain += coefficient * (target - p.gain)
		p.bass += .0258 * (p.hp - p.bass)
		p.subBass += .0104 * (p.hp - p.subBass)
		p.lowMid += .0815 * (p.hp - p.lowMid)
		p.presenceLow += .1227 * (p.hp - p.presenceLow)
		p.presenceHigh += .3676 * (p.hp - p.presenceHigh)
		x = p.hp
		if p.Deep {
			body := 1.1
			if p.SmartVoice {
				body = 1.5
			}
			// Body in the voice band, without lifting sub-bass room rumble.
			x += body * (p.bass - p.subBass)
			// A small low-mid cut keeps warmth from masking words, while a
			// restrained presence lift retains consonants without sharp treble.
			x -= .18 * (p.lowMid - p.bass)
			x += .12 * (p.presenceHigh - p.presenceLow)
		}
		p.levelGain += .00015 * (p.levelTarget - p.levelGain)
		x *= p.gain * 1.15 * p.levelGain
		// Gentle treble softening avoids brittle consonants without an echo
		// tap or a time-shifted copy of the voice.
		p.silky += .57 * (x - p.silky)
		if p.Deep {
			x -= .22 * (x - p.silky)
		}
		// Downward-only peak compression: never amplifies pauses or noise.
		c := .00014 // 150ms release
		if math.Abs(x) > p.compressionEnvelope {
			c = .0035 // 6ms attack
		}
		p.compressionEnvelope += c * (math.Abs(x) - p.compressionEnvelope)
		wanted := 1.0
		if p.SmartVoice && p.compressionEnvelope > .28 {
			wanted = math.Pow(.28/p.compressionEnvelope, .5)
		}
		if p.compressionGain == 0 {
			p.compressionGain = 1
		}
		p.compressionGain += .001 * (wanted - p.compressionGain)
		x *= p.compressionGain
		// Soft-knee peak control prevents PCM overflow without gain pumping.
		x = math.Tanh(x)
		peak = math.Max(peak, math.Abs(x))
		samples[i] = int16(math.Round(x * 32767))
	}
	return peak
}
