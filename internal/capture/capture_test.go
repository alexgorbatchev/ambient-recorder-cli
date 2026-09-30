package capture

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestClockRecalibration(t *testing.T) {
	start := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	for _, shift := range []time.Duration{0, 2 * time.Hour, -2 * time.Hour} {
		c := &Capture{anchor: start, host: 100}
		now := start.Add(time.Minute + shift)
		host := int64(100 + time.Minute)
		if got := c.reanchor(now, host); got != shift {
			t.Fatalf("clock shift=%s, want %s", got, shift)
		}
		// A block captured 20 ms before the calibration keeps its acquisition
		// offset rather than being assigned the reader's arrival time.
		at := c.anchor.Add(time.Duration(host-c.host) - 20*time.Millisecond)
		if !at.Equal(now.Add(-20 * time.Millisecond)) {
			t.Fatalf("acquisition=%s after shift %s", at, shift)
		}
	}
}

func TestNativeQueue(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "queue-test")
	cmd := exec.Command("clang", "-fblocks", "-mmacosx-version-min=14.2", "testdata/queue.m", "-framework", "CoreAudio", "-framework", "Foundation", "-framework", "AVFoundation", "-o", binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile capture queue test: %v\n%s", err, out)
	}
	if out, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("capture queue behavior: %v\n%s", err, out)
	}
}
