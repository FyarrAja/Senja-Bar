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
