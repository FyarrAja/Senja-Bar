//go:build windows

package mic

import (
	"testing"
	"time"
)

func TestRouteOutputChoosesStereoCableInput(t *testing.T) {
	devices := []Device{
		{ID: 1, Name: "Speakers (Rexus)"},
		{ID: 2, Name: "CABLE In 16ch (VB-Audio Virtual"},
		{ID: 3, Name: "CABLE Input (VB-Audio Virtual C"},
	}
	got, err := routeOutputFrom(devices)
	if err != nil || got.ID != 3 {
		t.Fatalf("route output = %+v, %v; want stereo CABLE Input", got, err)
	}
}

func TestRouteOutputFailsClosedWithoutUnambiguousCable(t *testing.T) {
	for _, devices := range [][]Device{
		{{ID: 1, Name: "Speakers (Rexus)"}},
		{{ID: 1, Name: "CABLE Input (Other Driver)"}},
		{{ID: 1, Name: "CABLE Input (VB-Audio)"}, {ID: 2, Name: "CABLE Input (VB-Audio Copy)"}},
	} {
		if _, err := routeOutputFrom(devices); err == nil {
			t.Fatalf("ambiguous or missing cable accepted: %+v", devices)
		}
	}
}

func TestRouteDoesNotResumeFromSavedState(t *testing.T) {
	c := PrepareTest(Config{Enabled: true, Route: true, RouteRequested: true})
	if c.Route || c.RouteRequested {
		t.Fatalf("virtual audio would resume without a new click: %+v", c)
	}
}

func TestCableRouteErrorIsVisible(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	want := "VB-CABLE Input tidak ditemukan; periksa instalasi VB-CABLE"
	if err := writeJSON("mic-status.json", Status{Message: want, Updated: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if got := readStatusFile().Message; got != want {
		t.Fatalf("routing error hidden: %q", got)
	}
}

func TestRouteModeKeepsFullProcessedLevel(t *testing.T) {
	s := &Session{route: true}
	s.SetMonitorVolume(25)
	if got := s.playbackScale(); got != 1 {
		t.Fatalf("virtual microphone output scaled to %g", got)
	}
	s.route = false
	s.SetMonitorVolume(25)
	if got := s.playbackScale(); got != .25 {
		t.Fatalf("self monitor scale = %g", got)
	}
}

func TestToggleRouteExcludesLocalMonitor(t *testing.T) {
	on := ToggleRoute(Config{Monitor: true, LiveMonitorRequested: true, MonitorVolume: 25}, false)
	if !on.Enabled || !on.Route || !on.RouteRequested || on.Monitor || on.LiveMonitorRequested {
		t.Fatalf("route did not take exclusive ownership: %+v", on)
	}
	off := ToggleRoute(on, true)
	if off.Enabled || off.Route || off.RouteRequested {
		t.Fatalf("route kept capturing after stop: %+v", off)
	}
}
