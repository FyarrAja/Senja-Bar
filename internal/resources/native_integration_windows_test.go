//go:build windows

package resources

import (
	"bufio"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestNativeResourceHelper(t *testing.T) {
	if os.Getenv("SENJA_RESOURCE_HELPER") != "1" {
		return
	}
	b := make([]byte, 64<<20)
	for i := 0; i < len(b); i += 4096 {
		b[i] = 1
	}
	fmt.Println("READY")
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
	runtime.KeepAlive(b)
	os.Exit(0)
}

// This exercise affects only a child created by this test, never user apps.
func TestNativeWorkingSetAndEcoQoS(t *testing.T) {
	if os.Getenv("SENJA_NATIVE_INTEGRATION") != "1" {
		t.Skip("opt-in native integration")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestNativeResourceHelper$")
	cmd.Env = append(os.Environ(), "SENJA_RESOURCE_HELPER=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { in.Write([]byte{1}); in.Close(); cmd.Wait() }()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "READY\n" {
		t.Fatalf("helper not ready: %q %v", line, err)
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_QUOTA|windows.PROCESS_SET_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	before := memory(h)
	if before < 32<<20 {
		t.Fatalf("working set not measurable: %d", before)
	}
	original, err := readPower(h)
	if err != nil {
		t.Fatal(err)
	}
	defer writePower(h, original)
	next := original
	next.Control |= 1
	next.State |= 1
	if err = writePower(h, next); err != nil {
		t.Fatal(err)
	}
	current, err := readPower(h)
	if err != nil || current.Control&1 == 0 || current.State&1 == 0 {
		t.Fatalf("EcoQoS not applied: %+v %v", current, err)
	}
	if err = writePower(h, original); err != nil {
		t.Fatal(err)
	}
	restored, err := readPower(h)
	if err != nil || restored.Control&1 != original.Control&1 || restored.State&1 != original.State&1 {
		t.Fatalf("EcoQoS not restored: %+v %v", restored, err)
	}
	ok, _, err := k32.NewProc("K32EmptyWorkingSet").Call(uintptr(h))
	if ok == 0 {
		t.Fatal(err)
	}
	after := memory(h)
	if after >= before {
		t.Fatalf("working set did not fall: %d -> %d", before, after)
	}
	t.Logf("Child working set: %.1f -> %.1f MiB; EcoQoS applied and restored", float64(before)/(1<<20), float64(after)/(1<<20))
}
