//go:build windows

package resources

import "testing"

func TestPeriodicObservationRequiresActiveAutomaticPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"fresh install", Config{AllBackground: true}, false},
		{"manual selection", Config{Paths: []string{"C:\\app.exe"}}, false},
		{"automatic without targets", Config{Automatic: true}, false},
		{"automatic with targets", Config{Automatic: true, Paths: []string{"C:\\app.exe"}}, true},
		{"EcoCPU all background", Config{EcoCPU: true, AllBackground: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsPeriodicObservation(tc.cfg); got != tc.want {
				t.Fatalf("periodic observation = %t, want %t", got, tc.want)
			}
		})
	}
}
