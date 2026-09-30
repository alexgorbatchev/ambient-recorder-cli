// Package storage reserves recording segments and opens daily diagnostic logs.
// It does not encode audio or promise durability of unflushed writes.
package storage

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	directoryMode = 0o700
	fileMode      = 0o600
	dayLayout     = "2006/01/02"
	logName       = "log.nljson"
	opusSuffix    = ".opus"
)

// Store owns a directory handle. Callers own and must close returned files.
type Store struct {
	root     *os.Root
	close    sync.Once
	closeErr error
}

// Open creates the output root if necessary and confines file access to it.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("recording directory is empty")
	}
	if err := os.MkdirAll(path, directoryMode); err != nil {
		return nil, fmt.Errorf("create recording directory: %w", err)
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open recording directory: %w", err)
	}
	return &Store{root: r}, nil
}

// CreateSegment reserves the next HH-NNN.opus path without overwriting existing
// entries. The timestamp's location determines the calendar directory and hour.
// The returned empty file must receive Ogg Opus headers and encoded pages before
// it is playable. Reservation alone does not create a valid recording.
func (s *Store) CreateSegment(at time.Time) (*os.File, error) {
	dir, err := s.day(at)
	if err != nil {
		return nil, err
	}
	prefix := at.Format("15") + "-"
	n, err := s.nextSequence(dir, prefix)
	if err != nil {
		return nil, err
	}
	for {
		path := filepath.Join(dir, fmt.Sprintf("%s%03d%s", prefix, n, opusSuffix))
		f, err := s.root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("reserve recording %s: %w", path, err)
		}
		if n == math.MaxInt {
			return nil, errors.New("recording sequence exhausted")
		}
		n++
	}
}

// OpenLog opens the timestamp's daily log in append mode, preserving earlier
// events after a restart. A slog.JSONHandler can write newline-delimited JSON
// directly to this file. The caller must reopen the log when the date changes.
func (s *Store) OpenLog(at time.Time) (*os.File, error) {
	dir, err := s.day(at)
	if err != nil {
		return nil, err
	}
	f, err := s.root.OpenFile(filepath.Join(dir, logName), os.O_RDWR|os.O_CREATE|os.O_APPEND, fileMode)
	if err != nil {
		return nil, fmt.Errorf("open daily diagnostic log: %w", err)
	}
	return f, nil
}

// Lock excludes another recorder from this output root. Closing the returned
// file releases the kernel lock, including after a process crash.
func (s *Store) Lock() (*os.File, error) {
	f, err := s.root.OpenFile(".recording.lock", os.O_RDWR|os.O_CREATE, fileMode)
	if err != nil {
		return nil, fmt.Errorf("open recording lock: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("lock recording directory (another recorder may be active): %w", err), f.Close())
	}
	return f, nil
}

// Close releases the directory handle. It does not close returned files.
func (s *Store) Close() error {
	s.close.Do(func() { s.closeErr = s.root.Close() })
	return s.closeErr
}

func (s *Store) day(at time.Time) (string, error) {
	dir := filepath.FromSlash(at.Format(dayLayout))
	if err := s.root.MkdirAll(dir, directoryMode); err != nil {
		return "", fmt.Errorf("create daily recording directory: %w", err)
	}
	return dir, nil
}

func (s *Store) nextSequence(dir, prefix string) (int, error) {
	f, err := s.root.Open(dir)
	if err != nil {
		return 0, fmt.Errorf("open recording directory for allocation: %w", err)
	}
	entries, readErr := f.ReadDir(-1)
	if err := errors.Join(readErr, f.Close()); err != nil {
		return 0, fmt.Errorf("read existing recordings: %w", err)
	}
	next := 0
	for _, entry := range entries {
		name, ok := strings.CutPrefix(entry.Name(), prefix)
		if !ok {
			continue
		}
		name, ok = strings.CutSuffix(name, opusSuffix)
		if !ok || len(name) < 3 || strings.IndexFunc(name, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			continue
		}
		n, err := strconv.Atoi(name)
		if err != nil || n == math.MaxInt {
			return 0, fmt.Errorf("recording sequence exceeds supported range: %s", entry.Name())
		}
		if n >= next {
			next = n + 1
		}
	}
	return next, nil
}
