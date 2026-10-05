package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
	"golang.org/x/sys/unix"
)

func TestCurrentFileProcess(t *testing.T) {
	if root := os.Getenv("RECORDER_CURRENT_TEST_DIR"); root != "" {
		s, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		f, err := s.CreateSegment(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.MarkCurrent(f); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintln(os.Stdout, f.Name()); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			t.Fatal(err)
		}
		// Exit without cleanup to leave the pointer as an interrupted process does.
		os.Exit(0)
	}
	root := testdir.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCurrentFileProcess$")
	cmd.Env = append(os.Environ(), "RECORDER_CURRENT_TEST_DIR="+root)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if got, err := CurrentFile(root); err != nil || got != strings.TrimSuffix(line, "\n") {
		t.Fatalf("live process: %q %v", got, err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				if got, err := CurrentFile(root); !errors.Is(err, ErrNoCurrentFile) || got != "" {
					t.Errorf("exited process: %q %v", got, err)
				}
			}
		})
	}
	wg.Wait()
}

func TestCurrentFileLifecycle(t *testing.T) {
	root := testdir.New(t)
	s := openStore(t, root)
	if _, err := CurrentFile(root); !errors.Is(err, ErrNoCurrentFile) {
		t.Fatalf("empty store: %v", err)
	}
	for range 2 {
		f, err := s.CreateSegment(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.MarkCurrent(f); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			got, err := CurrentFile(root)
			if err != nil || got != f.Name() {
				t.Fatalf("current=%q err=%v want=%q", got, err, f.Name())
			}
		}
		// A descriptor close models the lock release that also happens on a crash.
		closeFile(t, f)
		if _, err := CurrentFile(root); !errors.Is(err, ErrNoCurrentFile) {
			t.Fatalf("stale pointer: %v", err)
		}
	}
	if err := s.ClearCurrent(); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearCurrent(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, currentName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pointer not removed: %v", err)
	}
}

func TestCurrentFileInvalidState(t *testing.T) {
	for _, kind := range []string{"missing root", "root file", "not a link", "missing target", "outside", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			root := testdir.New(t)
			pointer := filepath.Join(root, currentName)
			switch kind {
			case "missing root":
				root = filepath.Join(root, "absent")
			case "root file":
				root = filepath.Join(root, "file")
				if err := os.WriteFile(root, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "not a link":
				if err := os.WriteFile(pointer, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				target := "absent.opus"
				switch kind {
				case "outside":
					target = "../outside.opus"
				case "directory":
					target = "."
				case "fifo":
					target = "fifo.opus"
					if err := unix.Mkfifo(filepath.Join(root, target), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, pointer); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := CurrentFile(root); err == nil || got != "" {
				t.Fatalf("invalid state returned %q: %v", got, err)
			}
			if kind == "missing root" {
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("query created root: %v", err)
				}
			}
		})
	}
}

func TestMarkCurrentFailures(t *testing.T) {
	for _, kind := range []string{"outside", "closed file", "blocked pointer", "closed store"} {
		t.Run(kind, func(t *testing.T) {
			root := testdir.New(t)
			s := openStore(t, root)
			f, err := s.CreateSegment(time.Now())
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "outside":
				closeFile(t, f)
				f, err = os.Create(filepath.Join(testdir.New(t), "outside.opus"))
				if err != nil {
					t.Fatal(err)
				}
			case "closed file":
				closeFile(t, f)
			case "blocked pointer":
				pointer := filepath.Join(root, currentName)
				if err := os.Mkdir(pointer, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(pointer, "preserve"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "closed store":
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.MarkCurrent(f); err == nil {
				t.Fatal("published invalid state")
			}
			if kind != "closed file" {
				closeFile(t, f)
			}
		})
	}
}
