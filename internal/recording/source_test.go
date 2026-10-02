package recording

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestMicrophoneSwitchPreservesSegment(t *testing.T) {
	s, err := storage.Open(testdir.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	j := &journal{store: s}
	defer func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
	}()
	r := &runner{store: s, journal: j, progress: make(chan struct{}, 1)}
	r.out = &sink{store: s, journal: j, rate: resample.Rate, bitrate: 32000, complexity: 2}
	start := time.Date(2026, 9, 30, 9, 0, 0, 123456789, time.UTC)
	var path string
	for i, rate := range []int{16000, 48000, 44100} {
		source, err := r.newSource(rate)
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; offset < rate; {
			n := min(137, rate-offset)
			at := start.Add(time.Duration(i)*time.Second + time.Duration(offset)*time.Second/time.Duration(rate))
			if err := source.write(context.Background(), at, make([]float32, n)); err != nil {
				t.Fatal(err)
			}
			offset += n
		}
		file, encoder := r.out.file, r.out.encoder
		if err := source.close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if r.out.file != file || r.out.encoder != encoder {
			t.Fatal("capture source close replaced or closed output")
		}
		if i == 0 {
			path = file.Name()
		}
		if file.Name() != path {
			t.Fatalf("microphone switch created a new file: %s", file.Name())
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(b, []byte("OpusHead")); got != 1 {
		t.Fatalf("logical streams=%d, want 1", got)
	}
	if got := opusDuration(t, b); got != 3*48000 {
		t.Fatalf("duration=%d samples, want 144000", got)
	}
}

func TestLongRunningSourceAcquisitionTime(t *testing.T) {
	s, err := storage.Open(testdir.New(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	j := &journal{store: s}
	defer func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
	}()
	r := &runner{store: s, journal: j, progress: make(chan struct{}, 1)}
	r.out = &sink{store: s, journal: j, rate: resample.Rate, bitrate: 32000, complexity: 2}
	source, err := r.newSource(44100)
	if err != nil {
		t.Fatal(err)
	}
	const elapsed = 72 * time.Hour
	source.inputFrames = int64(elapsed/time.Second) * 44100
	source.outputFrames = int64(elapsed/time.Second) * 48000
	at := time.Date(2026, 10, 3, 9, 0, 0, 123456789, time.UTC)
	if err := source.write(context.Background(), at, make([]float32, 960)); err != nil {
		t.Fatal(err)
	}
	path := r.out.file.Name()
	if err := source.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "2026/10/03/09-00-00.123456789.opus") {
		t.Fatalf("long-running source lost acquisition time: %s", path)
	}
}
