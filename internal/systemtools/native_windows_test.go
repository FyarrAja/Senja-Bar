//go:build windows

package systemtools

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func TestNativeTelemetryABI(t *testing.T) {
	if unsafe.Sizeof(memoryStatus{}) != 64 || unsafe.Sizeof(powerStatus{}) != 12 || unsafe.Sizeof(displayDevice{}) != 840 {
		t.Fatal("Windows telemetry structure layout mismatch")
	}
}
func TestNativeNetworkCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := nativeNetwork(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled diagnostic continued: %v", err)
	}
}
