//go:build windows

package mic

import "testing"

func TestAudioWorkerCanJoinWindowsMMCSS(t *testing.T) {
	end, err := beginAudioThread()
	if err != nil {
		t.Skipf("MMCSS Audio task unavailable: %v", err)
	}
	end()
}
