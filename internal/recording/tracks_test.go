package recording

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/opus"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func newTrackRunner(t *testing.T) *runner {
	t.Helper()
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	j := &journal{store: s}
	r := &runner{store: s, journal: j, progress: make(chan struct{}, 1), cfg: Config{Output: root}}
	r.out = &tracks{store: s, journal: j, rate: resample.Rate, bitrate: 32000, complexity: 2}
	t.Cleanup(func() {
		if err := r.out.close(); err != nil {
			t.Error(err)
		}
		if err := j.close(); err != nil {
			t.Error(err)
		}
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return r
}

func TestSeparateSourcesRotateTogether(t *testing.T) {
	r := newTrackRunner(t)
	start := time.Date(2026, 10, 13, 23, 59, 59, 990000000, time.UTC)
	s, err := r.newSource(48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]float32, 960*3)
	for i := range 960 {
		pcm[3*i], pcm[3*i+1], pcm[3*i+2] = 0.1, 0.2, 0.3
	}
	if err := s.write(context.Background(), start, pcm); err != nil {
		t.Fatal(err)
	}
	paths, err := storage.CurrentFiles(r.cfg.Output)
	if err != nil || len(paths) != 3 {
		t.Fatalf("active files: %v %v", paths, err)
	}
	if err := s.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	var current []string
	for _, source := range []string{"mic-1", "mic-2", "playback"} {
		current = append(current, r.out.sinks[source].file.Name())
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	root := r.cfg.Output
	for _, source := range []string{"mic-1", "mic-2", "playback"} {
		for _, stem := range []string{"2026/10/13/23-59-59.990000000", "2026/10/14/00-00-00.000000000"} {
			path := filepath.Join(root, stem+"-"+source+".opus")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if opusDuration(t, b) != 480 || bytes.Count(b, []byte("OpusHead")) != 1 {
				t.Fatalf("source rotation lost samples or reopened logical streams: %s", path)
			}
		}
	}
}

func TestSourceChannelHandover(t *testing.T) {
	r := newTrackRunner(t)
	start := time.Date(2026, 10, 13, 9, 0, 0, 123456789, time.UTC)
	var reports [][]string
	r.out.currentFilesChanged = func(paths []string) { reports = append(reports, append([]string(nil), paths...)) }
	var firstEncoder *opus.Encoder
	var firstPath string
	for i, channels := range []int{1, 2, 1} {
		s, err := r.newSource(48000, channels)
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; offset < 48000; offset += 480 {
			at := start.Add(time.Duration(i)*time.Second + frameDuration(int64(offset), 48000))
			if err := s.write(context.Background(), at, make([]float32, 480*(channels+1))); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstEncoder, firstPath = r.out.sinks["mic-1"].encoder, r.out.sinks["mic-1"].file.Name()
		}
		if r.out.sinks["mic-1"].encoder != firstEncoder || r.out.sinks["mic-1"].file.Name() != firstPath {
			t.Fatal("handover replaced the active mic-1 encoder")
		}
	}
	var want []string
	for _, name := range []string{"mic-1", "mic-2", "playback"} {
		f := r.out.sinks[name].file
		if !strings.HasSuffix(f.Name(), "09-00-00.123456789-"+name+".opus") {
			t.Fatal(f.Name())
		}
		want = append(want, f.Name())
	}
	if !reflect.DeepEqual(reports[len(reports)-1], want) {
		t.Fatalf("active sources: %v want %v", reports, want)
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range want {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := opusDuration(t, b); got != 3*48000 {
			t.Fatalf("unaligned source %s has %d frames", path, got)
		}
	}
}
