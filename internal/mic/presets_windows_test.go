//go:build windows

package mic

import "testing"

func TestPresetsTuneDSPWithoutChangingRouting(t *testing.T) {
	original := Config{Input: "my mic", Output: "my headset", Enabled: true, Monitor: true, MonitorVolume: 85, InputGain: 75, Gate: -40}
	for i := range Presets {
		c := ApplyPreset(original, i)
		if c.Input != original.Input || c.Output != original.Output || !c.Monitor || !c.Enabled || c.MonitorVolume != 85 {
			t.Fatal("preset changed devices or capture consent")
		}
		if PresetIndex(c) != i {
			t.Fatalf("preset %d not recognised", i)
		}
		if c.Gate < -60 || c.Gate > -20 || c.InputGain < 50 || c.InputGain > 150 {
			t.Fatal("invalid DSP values")
		}
	}
	if PresetIndex(original) != -1 {
		t.Fatal("manual settings must be Kustom")
	}
	if ApplyPreset(original, 99) != original {
		t.Fatal("invalid preset changed settings")
	}
}
func TestMissingMonitorOutputNeverFallsBackToDefault(t *testing.T) {
	_, err := StartMonitor(Device{Name: "not a capture device"}, Device{ID: 0xfffffffe, Name: "unplugged output"}, false, -38)
	if err == nil {
		t.Fatal("missing explicit output silently opened default")
	}
}

func TestLiveToggleStopsCaptureAndPlaybackTogether(t *testing.T) {
	c := Config{Input: "physical mic", InputGain: 100, Deep: true, MonitorVolume: 100, PreviewSample: true}
	on := ToggleLive(c, false)
	if !on.Enabled || !on.Monitor || !on.LiveMonitorRequested || on.PreviewSample || on.MonitorVolume > 50 {
		t.Fatal("toggle did not start a fresh live session")
	}
	off := ToggleLive(on, true)
	if off.Enabled || off.Monitor || off.LiveMonitorRequested || off.PreviewSample {
		t.Fatal("toggle left a hidden mic test running")
	}
	if off.Input != c.Input || off.InputGain != c.InputGain || off.Deep != c.Deep {
		t.Fatal("toggle changed voice profile")
	}
}
