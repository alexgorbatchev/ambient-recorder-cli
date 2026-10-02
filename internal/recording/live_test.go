package recording

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

// Explicit opt-in opens two real inputs sequentially. It leaves the output for
// inspection and never changes macOS's default input device.
func TestLiveMicrophoneSwitch(t *testing.T) {
	root := os.Getenv("RECORDER_LIVE_OUTPUT")
	if root == "" {
		t.Skip("set RECORDER_LIVE_OUTPUT to preserve a live microphone switching recording")
	}
	devices, err := capture.Microphones()
	if err != nil {
		t.Fatal(err)
	}
	var selected []capture.Device
	for _, d := range devices {
		if d.UID != "" && d.Transport != "Virtual" && d.Transport != "Aggregate" {
			selected = append(selected, d)
		}
		if len(selected) == 2 {
			break
		}
	}
	if len(selected) != 2 {
		t.Fatal("live switching check requires two identifiable physical microphone inputs")
	}
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	lock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Close(); err != nil {
			t.Error(err)
		}
	}()
	var diagnostic bytes.Buffer
	j := &journal{store: s, stderr: &diagnostic}
	defer func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
	}()
	r := &runner{store: s, journal: j, progress: make(chan struct{}, 1), cfg: Config{Output: root, SyncInterval: time.Second}}
	r.out = &sink{store: s, journal: j, rate: resample.Rate, bitrate: 32000, complexity: 2}
	defer func() {
		if err := r.out.close(); err != nil {
			t.Error(err)
		}
	}()
	var path string
	for i, d := range selected {
		r.cfg.Microphones = []string{d.Name + "*", "!uid:" + selected[1-i].UID}
		setPreferences(t, r)
		diagnostic.Reset()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := r.session(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		var opened map[string]any
		for _, event := range journalRecords(t, j) {
			if event["msg"] == "Capture device opened" {
				opened = event
			}
		}
		if opened["microphone_id"] != float64(d.ID) {
			t.Fatalf("requested input %q was not selected: %s", d.Name, diagnostic.String())
		}
		if r.out.file == nil {
			t.Fatal("session ended without retaining the output file")
		}
		if i == 0 {
			path = r.out.file.Name()
		}
		if r.out.file.Name() != path {
			t.Fatal("microphone switch opened another file")
		}
		t.Logf("recorded %q (uid=%s) into %s", d.Name, d.UID, path)
	}
	if err := errors.Join(r.out.close(), j.close()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(b, []byte("OpusHead")) != 1 {
		t.Fatal("mic switch created multiple logical streams")
	}
	if opusDuration(t, b) < 2*48000 {
		t.Fatal("live stream contains less than two seconds of recorded samples")
	}
}
