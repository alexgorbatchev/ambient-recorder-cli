package recording

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

func TestSourceRejectsInvalidCapture(t *testing.T) {
	r := newTrackRunner(t)
	for _, tt := range []struct{ rate, microphones int }{{48000, 0}, {-1, 2}} {
		if s, err := r.newSource(tt.rate, tt.microphones); err == nil {
			t.Fatalf("invalid capture created source: %+v", s)
		}
	}
	s, err := r.newSource(48000, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, pcm := range [][]float32{{1}, make([]float32, (capture.MaxFrames+1)*2)} {
		if err := s.write(context.Background(), time.Now(), pcm); err == nil {
			t.Fatal("invalid capture layout accepted")
		}
	}
	if err := s.persist(context.Background(), [][]float32{{1}, {}}); err == nil {
		t.Fatal("unaligned conversion accepted")
	}
	if err := s.converters[0].Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.write(context.Background(), time.Now(), []float32{1, 1}); err == nil {
		t.Fatal("closed converter accepted audio")
	}
	if err := s.close(context.Background()); err == nil {
		t.Fatal("converter shutdown error was swallowed")
	}
	if paths, err := storage.CurrentFiles(r.cfg.Output); !errors.Is(err, storage.ErrNoCurrentFiles) || len(paths) != 0 {
		t.Fatalf("invalid capture published files: %v %v", paths, err)
	}
}

func TestTrackOpeningFailures(t *testing.T) {
	for _, kind := range []string{"missing sources", "unaligned sources", "blocked storage", "invalid encoder"} {
		t.Run(kind, func(t *testing.T) {
			r := newTrackRunner(t)
			pcm := [][]float32{{1}, {2}}
			switch kind {
			case "missing sources":
				pcm = pcm[:1]
			case "unaligned sources":
				pcm[1] = nil
			case "blocked storage":
				if err := os.WriteFile(filepath.Join(r.cfg.Output, "2026"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "invalid encoder":
				r.out.bitrate = 0
			}
			if err := r.out.write(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC), pcm); err == nil {
				t.Fatal("invalid track group opened")
			}
			if len(r.out.sinks) != 0 {
				t.Fatal("failed open retained source resources")
			}
		})
	}
}

func TestTrackHandoverAndIOFailures(t *testing.T) {
	for _, kind := range []string{"write", "missing microphone", "rotation close", "add collision", "add encoder"} {
		t.Run(kind, func(t *testing.T) {
			r := newTrackRunner(t)
			at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
			pcm := [][]float32{make([]float32, 4800), make([]float32, 4800), make([]float32, 4800)}
			if kind != "missing microphone" {
				pcm = pcm[:2]
			}
			if err := r.out.write(at, pcm); err != nil {
				t.Fatal(err)
			}
			if err := r.out.sync(); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "write", "rotation close":
				if err := r.out.sinks["mic-1"].file.Close(); err != nil {
					t.Fatal(err)
				}
				if err := r.out.sync(); err == nil {
					t.Fatal("sync hid a closed source file")
				}
				if kind == "rotation close" {
					at = at.Add(time.Hour)
				}
			case "missing microphone":
				if err := r.out.sinks["mic-2"].file.Close(); err != nil {
					t.Fatal(err)
				}
				pcm = pcm[:2]
			case "add collision":
				path := strings.TrimSuffix(r.out.sinks["mic-1"].file.Name(), "-mic-1.opus") + "-mic-2.opus"
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				pcm = append(pcm, make([]float32, 4800))
			case "add encoder":
				r.out.bitrate = 0
				pcm = append(pcm, make([]float32, 4800))
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := r.persist(ctx, r.out, at, pcm); err == nil {
				t.Fatal("source write failure was swallowed")
			}
			if len(r.out.sinks) != 0 {
				t.Fatal("failed storage retained source encoders")
			}
			if paths, err := storage.CurrentFiles(r.cfg.Output); !errors.Is(err, storage.ErrNoCurrentFiles) || len(paths) != 0 {
				t.Fatalf("failed group remained current: %v %v", paths, err)
			}
		})
	}
}
