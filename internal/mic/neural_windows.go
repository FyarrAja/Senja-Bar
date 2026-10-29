//go:build windows

package mic

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type neuralSuppressor struct {
	dll              *syscall.DLL
	state            uintptr
	process, destroy *syscall.Proc
}

func openNeuralSuppressor() (*neuralSuppressor, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(executable), "SenjaNoise.dll")
	// Explicit developer override for isolated tests. Never search the CWD.
	if override := os.Getenv("SENJA_NEURAL_DLL"); override != "" {
		path = override
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("mesin RNNoise tidak tersedia: %w", err)
	}
	fail := func(err error) (*neuralSuppressor, error) { _ = dll.Release(); return nil, err }
	abi, err := dll.FindProc("senja_noise_abi")
	if err != nil {
		return fail(err)
	}
	version, _, _ := abi.Call()
	if version != 1 {
		return fail(fmt.Errorf("versi mesin RNNoise tidak cocok"))
	}
	create, err := dll.FindProc("senja_noise_create")
	if err != nil {
		return fail(err)
	}
	process, err := dll.FindProc("senja_noise_process")
	if err != nil {
		return fail(err)
	}
	destroy, err := dll.FindProc("senja_noise_destroy")
	if err != nil {
		return fail(err)
	}
	state, _, _ := create.Call()
	if state == 0 {
		return fail(fmt.Errorf("model RNNoise gagal dimuat"))
	}
	return &neuralSuppressor{dll, state, process, destroy}, nil
}
func (n *neuralSuppressor) Process(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	probability, _, _ := n.process.Call(n.state, uintptr(unsafe.Pointer(&samples[0])), uintptr(len(samples)))
	runtime.KeepAlive(samples)
	return float64(min(1000, probability)) / 1000
}
func (n *neuralSuppressor) Close() {
	if n == nil || n.state == 0 {
		return
	}
	n.destroy.Call(n.state)
	n.state = 0
	_ = n.dll.Release()
}
