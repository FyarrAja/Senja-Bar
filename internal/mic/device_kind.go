package mic

import "strings"

// WinMM supplies endpoint names rather than form factors. Prefer explicit names
// and retain an honest generic fallback instead of inventing hardware identity.
func DeviceKind(name string, input bool) string {
	s := strings.ToLower(name)
	for _, word := range []string{"voicemod", "virtual", "vb-audio", "cable", "stereo mix", "stereomix"} {
		if strings.Contains(s, word) {
			return "virtual"
		}
	}
	for _, word := range []string{"headset", "headphone", "earphone", "hands-free", "handsfree", "polaris"} {
		if strings.Contains(s, word) {
			return "headset"
		}
	}
	if strings.Contains(s, "speaker") || strings.Contains(s, "loudspeaker") {
		return "speaker"
	}
	if input {
		return "mic"
	}
	return "output"
}
