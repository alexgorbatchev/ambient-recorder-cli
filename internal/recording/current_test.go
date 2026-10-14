package recording

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

func TestTrackCurrentFiles(t *testing.T) {
	r := newTrackRunner(t)
	var reported []string
	var reports [][]string
	r.out.currentFilesChanged = func(paths []string) {
		reported = append([]string(nil), paths...)
		reports = append(reports, append([]string(nil), paths...))
	}
	start := time.Date(2026, 10, 5, 23, 59, 59, 0, time.UTC)
	for _, at := range []time.Time{start, start.Add(10 * time.Millisecond), start.Add(time.Second)} {
		if err := r.out.write(at, [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
			t.Fatal(err)
		}
		got, err := storage.CurrentFiles(r.cfg.Output)
		if err != nil || !reflect.DeepEqual(got, r.out.paths()) {
			t.Fatalf("current=%q err=%v want=%q", got, err, r.out.paths())
		}
		if !reflect.DeepEqual(reported, got) {
			t.Fatalf("service reports %v; open files are %v", reported, got)
		}
		if at == start.Add(time.Second) && filepath.Base(got[0]) != "00-00-00.000000000-mic-1.opus" {
			t.Fatalf("rotation left old snapshot: %v", got)
		}
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	if got, err := storage.CurrentFiles(r.cfg.Output); !errors.Is(err, storage.ErrNoCurrentFiles) || len(got) != 0 {
		t.Fatalf("closed segments reported: %v %v", got, err)
	}
	if len(reported) != 0 || len(reports) != 4 || len(reports[1]) != 0 || len(reports[3]) != 0 {
		t.Fatalf("open/rotation/close reports: %v", reports)
	}
}

func TestTrackCurrentFilesPublicationFailure(t *testing.T) {
	r := newTrackRunner(t)
	pointer := filepath.Join(r.cfg.Output, ".recording.current")
	if err := os.Mkdir(pointer, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pointer, "preserve"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r.out.currentFilesChanged = func(paths []string) {
		if len(paths) > 0 {
			t.Fatalf("failed group reported as current: %v", paths)
		}
	}
	if err := r.out.write(time.Now(), [][]float32{make([]float32, 480), make([]float32, 480)}); err == nil {
		t.Fatal("recording proceeded without publishing active files")
	}
	if len(r.out.sinks) != 0 {
		t.Fatal("failed open retained recording resources")
	}
}
