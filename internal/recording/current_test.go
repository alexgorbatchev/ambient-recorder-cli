package recording

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestSinkCurrentFile(t *testing.T) {
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	j := &journal{store: s}
	t.Cleanup(func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
	})
	var reported string
	var reports []string
	out := &sink{store: s, journal: j, rate: 48000, bitrate: 32000, complexity: 2, currentFileChanged: func(path string) {
		reported = path
		reports = append(reports, path)
	}}
	t.Cleanup(func() {
		if err := out.close(); err != nil {
			t.Error(err)
		}
	})
	start := time.Date(2026, 10, 5, 23, 59, 59, 0, time.UTC)
	for _, at := range []time.Time{start, start.Add(10 * time.Millisecond), start.Add(time.Second)} {
		if err := out.write(at, make([]float32, 480)); err != nil {
			t.Fatal(err)
		}
		got, err := storage.CurrentFile(root)
		if err != nil || got != out.file.Name() {
			t.Fatalf("current=%q err=%v want=%q", got, err, out.file.Name())
		}
		if reported != got {
			t.Fatalf("service reports %q; open file is %q", reported, got)
		}
		if at == start.Add(time.Second) && filepath.Base(got) != "00-00-00.000000000.opus" {
			t.Fatalf("rotation left old pointer: %s", got)
		}
	}
	if err := out.close(); err != nil {
		t.Fatal(err)
	}
	if got, err := storage.CurrentFile(root); !errors.Is(err, storage.ErrNoCurrentFile) || got != "" {
		t.Fatalf("closed segment reported: %q %v", got, err)
	}
	if reported != "" || len(reports) != 4 || reports[1] != "" || reports[3] != "" {
		t.Fatalf("open/rotation/close reports: %q", reports)
	}
}

func TestSinkCurrentFilePublicationFailure(t *testing.T) {
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	pointer := filepath.Join(root, ".recording.current")
	if err := os.Mkdir(pointer, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pointer, "preserve"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out := &sink{store: s, journal: &journal{store: s}, rate: 48000, bitrate: 32000, complexity: 2, currentFileChanged: func(path string) { t.Fatalf("failed segment reported as current: %q", path) }}
	if err := out.write(time.Now(), make([]float32, 480)); err == nil {
		t.Fatal("recording proceeded without publishing its active file")
	}
	if out.file != nil || out.encoder != nil {
		t.Fatal("failed open retained recording resources")
	}
}
