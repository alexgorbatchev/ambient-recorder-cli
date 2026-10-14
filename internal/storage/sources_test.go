package storage

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestSourceSegmentsAndCurrentSnapshot(t *testing.T) {
	s := openStore(t, testdir.New(t))
	at := time.Date(2026, 10, 13, 9, 0, 0, 123456789, time.UTC)
	sources := []string{"mic-1", "mic-2", "playback"}
	group, err := s.CreateSegments(at, sources)
	if err != nil {
		t.Fatal(err)
	}
	files := group.Files
	var want []string
	for i, f := range files {
		want = append(want, f.Name())
		if !strings.HasSuffix(f.Name(), "09-00-00.123456789-"+sources[i]+".opus") {
			t.Fatal(f.Name())
		}
	}
	if err := s.MarkCurrent(files); err != nil {
		t.Fatal(err)
	}
	got, err := CurrentFiles(s.root.Name())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot=%v err=%v want=%v", got, err, want)
	}
	closeFile(t, files[1])
	got, err = CurrentFiles(s.root.Name())
	if err != nil || !reflect.DeepEqual(got, []string{want[0], want[2]}) {
		t.Fatalf("closed source still active: %v %v", got, err)
	}
	closeFile(t, files[0])
	closeFile(t, files[2])
	if got, err := CurrentFiles(s.root.Name()); !errors.Is(err, ErrNoCurrentFiles) || len(got) != 0 {
		t.Fatalf("stale sources: %v %v", got, err)
	}
}

func TestSourceReservationCollision(t *testing.T) {
	s := openStore(t, testdir.New(t))
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	old, err := s.CreateSegment(at, "mic-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.WriteString("preserve"); err != nil {
		t.Fatal(err)
	}
	closeFile(t, old)
	group, err := s.CreateSegments(at, []string{"mic-1", "mic-2", "playback"})
	if err != nil {
		t.Fatal(err)
	}
	files := group.Files
	base := strings.TrimSuffix(files[0].Name(), "-mic-1.opus")
	for i, source := range []string{"mic-1", "mic-2", "playback"} {
		if files[i].Name() != base+"-"+source+".opus" {
			t.Fatalf("sources have different timestamps: %v", files)
		}
		closeFile(t, files[i])
	}
	if b, err := os.ReadFile(old.Name()); err != nil || string(b) != "preserve" {
		t.Fatalf("collision overwrote audio: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(s.root.Name(), "2026/10/13/09-00-00.000000000-mic-1.opus")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial group left behind: %v", err)
	}
}

func TestInvalidSourceReservations(t *testing.T) {
	s := openStore(t, testdir.New(t))
	for _, sources := range [][]string{nil, {"mic-1", "mic-1"}, {"../outside"}, {"mic-0"}, {"mic-01"}, {"mic-x"}} {
		if group, err := s.CreateSegments(time.Now(), sources); err == nil {
			for _, f := range group.Files {
				closeFile(t, f)
			}
			t.Fatalf("invalid sources accepted: %v", sources)
		}
	}
	if f, err := s.CreateSegment(time.Now(), "../outside"); err == nil {
		closeFile(t, f)
		t.Fatal("source escaped storage")
	}
	if err := s.MarkCurrent(nil); err != nil {
		t.Fatal(err)
	}
	if files, err := CurrentFiles(s.root.Name()); !errors.Is(err, ErrNoCurrentFiles) || len(files) != 0 {
		t.Fatalf("empty group was published: %v %v", files, err)
	}
}

func TestInvalidCurrentSourceManifest(t *testing.T) {
	for _, kind := range []string{"directory", "oversize", "duplicate", "blocked clear"} {
		t.Run(kind, func(t *testing.T) {
			s := openStore(t, testdir.New(t))
			path := filepath.Join(s.root.Name(), currentName)
			switch kind {
			case "directory", "blocked clear":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if kind == "blocked clear" {
					if err := os.WriteFile(filepath.Join(path, "keep"), nil, 0o600); err != nil {
						t.Fatal(err)
					}
					if err := s.ClearCurrent(); err == nil {
						t.Fatal("snapshot removal failure was swallowed")
					}
				}
			case "oversize":
				if err := os.WriteFile(path, make([]byte, maxCurrentSize+1), 0o600); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				if err := os.WriteFile(path, []byte(`["same.opus","same.opus"]`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if paths, err := CurrentFiles(s.root.Name()); err == nil || len(paths) != 0 {
				t.Fatalf("invalid snapshot accepted: %v %v", paths, err)
			}
		})
	}
}
