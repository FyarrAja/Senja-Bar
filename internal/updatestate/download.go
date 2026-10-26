package updatestate

import (
	"math"
	"time"
)

// Byte counts are exact within the updater's 64 MiB download limit. Zero total
// means unknown, never a size inferred from a release description.
func DownloadWire(done, total int64) uint64 {
	done = max(int64(0), min(done, 64<<20))
	total = max(int64(0), min(total, 64<<20))
	return uint64(total)<<32 | uint64(done)
}

type Download struct {
	Done, Total int64
	Elapsed     time.Duration
}

func ReadDownload(wire uint64, elapsed time.Duration) Download {
	return Download{Done: min(int64(uint32(wire)), 64<<20), Total: min(int64(wire>>32), 64<<20), Elapsed: max(time.Duration(0), elapsed)}
}
func (d Download) Percent() int {
	if d.Total <= 0 {
		return -1
	}
	return int(min(int64(100), d.Done*100/d.Total))
}
func (d Download) BytesPerSecond() float64 {
	if d.Elapsed < 250*time.Millisecond || d.Done <= 0 {
		return 0
	}
	return float64(d.Done) / d.Elapsed.Seconds()
}
func (d Download) Remaining() (time.Duration, bool) {
	rate := d.BytesPerSecond()
	if d.Total <= 0 || rate <= 0 {
		return 0, false
	}
	seconds := math.Ceil(float64(max(int64(0), d.Total-d.Done)) / rate)
	return time.Duration(min(seconds, 7*24*3600)) * time.Second, true
}
