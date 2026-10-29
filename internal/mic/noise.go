package mic

import "math"

const noiseFFTSize = 512
const noiseHop = noiseFFTSize / 2

// Streaming overlap-add Wiener suppression. The background spectrum is learned
// in quiet frames and held during louder speech, so suppression continues while
// the gate is open. It is not a speaker-identity or directional isolation model.
type noiseReducer struct {
	input, overlap, output [noiseFFTSize]float64
	window                 [noiseFFTSize]float64
	bins                   [noiseFFTSize]complex128
	noise, gains           [noiseFFTSize/2 + 1]float64
	roots                  [noiseFFTSize / 2]complex128
	position               int
	ready                  bool
	Smart                  bool
	speechHold, keyHold    int
	highBase, floorRMS     float64
}

func (r *noiseReducer) initialise() {
	for i := range r.window {
		r.window[i] = math.Sqrt(.5 - .5*math.Cos(2*math.Pi*float64(i)/noiseFFTSize))
	}
	for i := range r.roots {
		angle := -2 * math.Pi * float64(i) / noiseFFTSize
		r.roots[i] = complex(math.Cos(angle), math.Sin(angle))
	}
	for i := range r.gains {
		r.gains[i] = 1
		r.noise[i] = 1e-7
	}
	r.ready = true
}
func (r *noiseReducer) transform(inverse bool) {
	for i, j := 1, 0; i < noiseFFTSize; i++ {
		bit := noiseFFTSize >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			r.bins[i], r.bins[j] = r.bins[j], r.bins[i]
		}
	}
	for size := 2; size <= noiseFFTSize; size *= 2 {
		half := size / 2
		step := noiseFFTSize / size
		for start := 0; start < noiseFFTSize; start += size {
			for j := 0; j < half; j++ {
				w := r.roots[j*step]
				if inverse {
					w = complex(real(w), -imag(w))
				}
				a, b := r.bins[start+j], r.bins[start+j+half]*w
				r.bins[start+j], r.bins[start+j+half] = a+b, a-b
			}
		}
	}
	if inverse {
		for i := range r.bins {
			r.bins[i] /= noiseFFTSize
		}
	}
}
func (r *noiseReducer) Process(samples []int16, strength int, quietThreshold float64) {
	if !r.ready {
		r.initialise()
	}
	for i, sample := range samples {
		result := r.output[r.position]
		r.input[noiseHop+r.position] = float64(sample) / 32768
		samples[i] = int16(math.Round(max(-1., min(1., result)) * 32767))
		r.position++
		if r.position == noiseHop {
			r.frame(strength, quietThreshold)
			r.position = 0
		}
	}
}
func (r *noiseReducer) frame(strength int, quietThreshold float64) {
	energy := 0.
	peak := 0.
	for i, x := range r.input {
		energy += x * x
		peak = max(peak, math.Abs(x))
		r.bins[i] = complex(x*r.window[i], 0)
	}
	quiet := math.Sqrt(energy/noiseFFTSize) < quietThreshold
	r.transform(false)
	// Absolute level alone is not a speech detector: quiet voiced syllables can
	// fall below the learning threshold. Protect structured voice-band spectra
	// from being absorbed into the noise estimate over a sustained phrase.
	var logPower, bandPower float64
	count := 0.
	for i := 3; i <= 85; i++ {
		power := real(r.bins[i])*real(r.bins[i]) + imag(r.bins[i])*imag(r.bins[i])
		logPower += math.Log(max(power, 1e-15))
		bandPower += power
		count++
	}
	flatness := math.Exp(logPower/count) / max(bandPower/count, 1e-15)
	voiced := math.Sqrt(energy/noiseFFTSize) > .001 && flatness < .22
	if voiced {
		r.speechHold = 24
	} else if r.speechHold > 0 {
		r.speechHold--
	}
	highPower := 0.
	for i := 32; i <= 170; i++ {
		highPower += real(r.bins[i])*real(r.bins[i]) + imag(r.bins[i])*imag(r.bins[i])
	}
	crest := peak / max(math.Sqrt(energy/noiseFFTSize), 1e-9)
	if r.Smart && crest > 3.5 && highPower > max(1e-5, 3.5*r.highBase) {
		r.keyHold = 5
	}
	if r.keyHold > 0 {
		r.keyHold--
	}
	r.highBase += .025 * (highPower - r.highBase)
	quiet = quiet && !voiced
	amount := float64(max(0, min(100, strength))) / 100
	floor := .20 - .17*amount
	for i := range r.noise {
		power := real(r.bins[i])*real(r.bins[i]) + imag(r.bins[i])*imag(r.bins[i])
		if quiet {
			r.noise[i] = .96*r.noise[i] + .04*power
		}
		target := math.Sqrt(max(floor*floor, 1-(1+3*amount)*r.noise[i]/max(power, 1e-12)))
		coefficient := .18
		if target > r.gains[i] {
			coefficient = .65
		}
		r.gains[i] += coefficient * (target - r.gains[i])
	}
	for i := 0; i <= noiseFFTSize/2; i++ {
		gain := r.gains[i]
		if i > 0 && i < noiseFFTSize/2 {
			gain = (r.gains[i-1] + 2*gain + r.gains[i+1]) / 4
		}
		if r.Smart && r.keyHold > 0 {
			if r.speechHold == 0 {
				gain *= .10
			} else if i >= 32 {
				gain *= .20
			}
		}
		r.bins[i] *= complex(gain, 0)
		if i > 0 && i < noiseFFTSize/2 {
			r.bins[noiseFFTSize-i] *= complex(gain, 0)
		}
	}
	noiseEnergy := r.noise[0] + r.noise[noiseFFTSize/2]
	for i := 1; i < noiseFFTSize/2; i++ {
		noiseEnergy += 2 * r.noise[i]
	}
	r.floorRMS = math.Sqrt(2 * noiseEnergy / (noiseFFTSize * noiseFFTSize))
	r.transform(true)
	for i := 0; i < noiseFFTSize; i++ {
		r.overlap[i] += real(r.bins[i]) * r.window[i]
	}
	copy(r.output[:noiseHop], r.overlap[:noiseHop])
	copy(r.overlap[:noiseHop], r.overlap[noiseHop:])
	clear(r.overlap[noiseHop:])
	copy(r.input[:noiseHop], r.input[noiseHop:])
}
