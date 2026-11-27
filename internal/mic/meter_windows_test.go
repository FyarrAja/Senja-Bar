//go:build windows

package mic

import "testing"

func TestActiveMeterPreservesDistinctProcessingStates(t *testing.T) {
	seen := map[uint32]bool{}
	for _, message := range activeMeterMessages {
		kind := meterMessageKind(message)
		if seen[kind] || meterKindMessage(kind) != message {
			t.Fatalf("status collapsed: %s", message)
		}
		seen[kind] = true
	}
	if meterKindMessage(^uint32(0)) != activeMeterMessages[0] {
		t.Fatal("invalid status did not fall back")
	}
}

func TestRouteStatusDoesNotAppearAsLocalTest(t *testing.T) {
	message := "Kirim ke input virtual aktif · pilih CABLE Output di Discord/FiveM"
	if meterKindMessage(meterMessageKind(message)) != message {
		t.Fatal("active route status collapsed to local test")
	}
}
