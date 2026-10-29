//go:build windows

package mic

// Presets tune the existing DSP. They never change devices, monitoring or capture consent.
type Preset struct {
	Name, Detail   string
	Gain, Gate     int
	Deep           bool
	NoiseReduction int
	SmartVoice     bool
}

var Presets = []Preset{
	{"Seimbang", "Karakter alami untuk percakapan", 100, -38, false, 70, false},
	{"Jernih", "Batas noise lebih tinggi", 100, -32, false, 85, false},
	{"Bass", "Bass berisi tanpa mengubah nada", 100, -38, true, 80, false},
	{"Ruangan bising", "Mengurangi sensitivitas input", 75, -26, false, 90, false},
	{"Suara pelan", "Memperkuat input mikrofon", 125, -44, false, 55, false},
	{"Fokus bicara", "Level ucapan dengan gate lebih tegas", 100, -32, false, 90, false},
	{"Bass otomatis", "Bass dan level suara terkendali", 100, -38, true, 85, true},
}

func ApplyPreset(c Config, index int) Config {
	if index < 0 || index >= len(Presets) {
		return c
	}
	p := Presets[index]
	c.InputGain, c.Gate, c.Deep = p.Gain, p.Gate, p.Deep
	c.NoiseReduction = p.NoiseReduction
	c.SmartVoice = p.SmartVoice
	return c
}
func PresetIndex(c Config) int {
	for i, p := range Presets {
		if c.InputGain == p.Gain && c.Gate == p.Gate && c.Deep == p.Deep && c.NoiseReduction == p.NoiseReduction && c.SmartVoice == p.SmartVoice {
			return i
		}
	}
	return -1
}
