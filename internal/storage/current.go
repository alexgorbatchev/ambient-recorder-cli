package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const currentName = ".recording.current"

// ErrNoCurrentFile means no open recording segment was found in this directory.
var ErrNoCurrentFile = errors.New("no recording file is currently open")

// MarkCurrent publishes a segment after encoder initialization. The recorder
// owns the directory lock; the segment lock remains held until f is closed.
func (s *Store) MarkCurrent(f *os.File) error {
	path, err := filepath.Rel(s.root.Name(), f.Name())
	if err != nil || !filepath.IsLocal(path) {
		return fmt.Errorf("recording file is outside its storage directory: %s", f.Name())
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("lock current recording file: %w", err)
	}
	if err := s.ClearCurrent(); err != nil {
		return err
	}
	if err := s.root.Symlink(path, currentName); err != nil {
		return fmt.Errorf("publish current recording file: %w", err)
	}
	return nil
}

// ClearCurrent removes the pointer before segment finalization.
func (s *Store) ClearCurrent() error {
	err := s.root.Remove(currentName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear current recording file: %w", err)
	}
	return nil
}

// CurrentFile takes a snapshot of the open segment without creating storage or
// opening capture. A stale pointer is rejected by checking the segment's lock.
func CurrentFile(directory string) (path string, err error) {
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve recording directory: %w", err)
	}
	r, err := os.OpenRoot(directory)
	if err != nil {
		return "", currentError(err)
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	target, err := r.Readlink(currentName)
	if err != nil {
		return "", currentError(err)
	}
	if !filepath.IsLocal(target) {
		return "", errors.New("current recording pointer is outside its storage directory")
	}
	// A damaged pointer to a FIFO must not block this inspection command.
	f, err := r.OpenFile(target, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return "", currentError(err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect current recording file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("current recording pointer does not reference a regular file")
	}
	// Shared probes coexist and cannot mistake another query for a recorder.
	err = unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return filepath.Join(directory, target), nil
	}
	if err != nil {
		return "", fmt.Errorf("check current recording lock: %w", err)
	}
	return "", ErrNoCurrentFile
}

func currentError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrNoCurrentFile
	}
	return fmt.Errorf("read current recording file: %w", err)
}
