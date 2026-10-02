package storage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestSegmentRestart(t *testing.T) {
	root := testdir.New(t)
	at := time.Date(2026, time.September, 30, 9, 58, 0, 0, time.UTC)
	for i := range 3 {
		s := openStore(t, root)
		start := at.Add(time.Duration(i) * time.Second)
		f, err := s.CreateSegment(start)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "2026/09/30", start.Format("15-04-05.000000000")+".opus")
		if f.Name() != want {
			t.Fatalf("path = %q, want %q", f.Name(), want)
		}
		if _, err := f.WriteString(fmt.Sprintf("segment %d", i)); err != nil {
			t.Fatal(err)
		}
		closeFile(t, f)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "2026/09/30/09-58-00.000000000.opus"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "segment 0" {
		t.Fatalf("earlier segment overwritten: %q", b)
	}
}

func TestSegmentCalendarBoundaries(t *testing.T) {
	s := openStore(t, testdir.New(t))
	tests := []struct {
		at   time.Time
		path string
	}{
		{time.Date(2026, 9, 30, 23, 59, 0, 123456789, time.UTC), "2026/09/30/23-59-00.123456789.opus"},
		{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "2026/10/01/00-00-00.000000000.opus"},
		{time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC), "2026/10/01/01-00-00.000000000.opus"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			f, err := s.CreateSegment(tt.at)
			if err != nil {
				t.Fatal(err)
			}
			defer closeFile(t, f)
			if want := filepath.Join(s.root.Name(), tt.path); f.Name() != want {
				t.Fatalf("path = %q, want %q", f.Name(), want)
			}
		})
	}
}

func TestSegmentAllocation(t *testing.T) {
	tests := []struct {
		name      string
		existing  []string
		collision bool
	}{
		{"unrelated", []string{"notes.txt", "log.nljson"}, false},
		{"reserved file", []string{"09-00-00.000000000.opus"}, true},
		{"reserved directory", []string{"09-00-00.000000000.opus/"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := testdir.New(t)
			dir := filepath.Join(root, "2026/09/30")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range tt.existing {
				path := filepath.Join(dir, name)
				if strings.HasSuffix(name, "/") {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s := openStore(t, root)
			f, err := s.CreateSegment(time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			defer closeFile(t, f)
			initial := filepath.Join(dir, "09-00-00.000000000.opus")
			if (f.Name() != initial) != tt.collision {
				t.Fatalf("allocated %q, collision=%v", f.Name(), tt.collision)
			}
			if _, err := f.WriteString("new recording"); err != nil {
				t.Fatal(err)
			}
			for _, name := range tt.existing {
				if strings.HasSuffix(name, "/") {
					continue
				}
				b, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(b) != "preserve" {
					t.Fatalf("existing %q changed: %q, %v", name, b, err)
				}
			}
		})
	}
}

func TestConcurrentSegments(t *testing.T) {
	const workers = 16
	s := openStore(t, testdir.New(t))
	paths := make(chan string, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			f, err := s.CreateSegment(time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC))
			if err != nil {
				errs <- err
				return
			}
			paths <- f.Name()
			if err := f.Close(); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	seen := make(map[string]bool)
	for path := range paths {
		if seen[path] {
			t.Errorf("duplicate path %q", path)
		}
		seen[path] = true
	}
	if len(seen) != workers {
		t.Fatalf("created %d segments, want %d", len(seen), workers)
	}
}

func TestDailyLogAppend(t *testing.T) {
	root := testdir.New(t)
	at := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	for _, when := range []time.Time{at, at, at.Add(time.Minute)} {
		s := openStore(t, root)
		f, err := s.OpenLog(when)
		if err != nil {
			t.Fatal(err)
		}
		slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})).Debug("capture event", "device", "built-in")
		closeFile(t, f)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for path, count := range map[string]int{"2026/09/30/log.nljson": 2, "2026/10/01/log.nljson": 1} {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) != count {
			t.Fatalf("%s: %d events, want %d", path, len(lines), count)
		}
		for _, line := range lines {
			var event map[string]any
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatal(err)
			}
			if event["level"] != "DEBUG" || event["device"] != "built-in" {
				t.Fatalf("unexpected event: %v", event)
			}
		}
	}
}

func TestStorageFailure(t *testing.T) {
	root := testdir.New(t)
	if err := os.WriteFile(filepath.Join(root, "2026"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := openStore(t, root)
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	for name, open := range map[string]func(time.Time) (*os.File, error){"segment": s.CreateSegment, "log": s.OpenLog} {
		t.Run(name, func(t *testing.T) {
			if f, err := open(at); err == nil {
				closeFile(t, f)
				t.Fatal("expected filesystem error")
			}
		})
	}
}

func TestStorageCannotEscapeRoot(t *testing.T) {
	root, outside := testdir.New(t), testdir.New(t)
	if err := os.Symlink(outside, filepath.Join(root, "2026")); err != nil {
		t.Fatal(err)
	}
	s := openStore(t, root)
	if f, err := s.CreateSegment(time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)); err == nil {
		closeFile(t, f)
		t.Fatal("created segment outside root")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("modified outside directory")
	}
}

func TestSingleRecorderLock(t *testing.T) {
	root := testdir.New(t)
	first, second := openStore(t, root), openStore(t, root)
	lock, err := first.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := second.Lock(); err == nil {
		closeFile(t, duplicate)
		t.Fatal("two recorders acquired the same directory")
	}
	closeFile(t, lock)
	restarted, err := second.Lock()
	if err != nil {
		t.Fatalf("lock did not release on close: %v", err)
	}
	closeFile(t, restarted)
}

func openStore(t *testing.T, root string) *Store {
	t.Helper()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func closeFile(t *testing.T, f *os.File) {
	t.Helper()
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}
