package mic

import "testing"

func TestDeviceKindHonestFallback(t *testing.T) {
	for _, c := range []struct {
		name  string
		input bool
		want  string
	}{
		{"Microphone (Rexus Xora-II)", true, "mic"},
		{"Headset Microphone", true, "headset"},
		{"Speakers (Realtek)", false, "speaker"},
		{"Microphone (Voicemod)", true, "virtual"},
		{"Headphones (Polaris)", false, "headset"},
		{"Unknown endpoint", false, "output"},
	} {
		if got := DeviceKind(c.name, c.input); got != c.want {
			t.Fatalf("%s: %s", c.name, got)
		}
	}
}
