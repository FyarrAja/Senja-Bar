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
		{"Senja Microphone (VB-Audio Virt", true, "virtual"},
		{"Virtual Headset", true, "virtual"},
		{"Headphones (Polaris)", false, "headset"},
		{"Unknown endpoint", false, "output"},
	} {
		if got := DeviceKind(c.name, c.input); got != c.want {
			t.Fatalf("%s: %s", c.name, got)
		}
	}
}

func TestVirtualCaptureCannotStartSenjaMic(t *testing.T) {
	_, err := startSessionWithConfig(Device{ID: 1, Name: "Senja Microphone (VB-Audio Virt"}, nil, false, -38, false, nil)
	if err == nil {
		t.Fatal("virtual capture source accepted; may create an audio loop")
	}
}
