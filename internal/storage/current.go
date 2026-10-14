package storage

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const currentName = ".recording.current"
const maxCurrentSize = 1 << 20

// ErrNoCurrentFiles means no open source segments were found in this directory.
var ErrNoCurrentFiles = errors.New("no recording files are currently open")

// MarkCurrent atomically publishes all initialized source segments. Their
// exclusive locks remain held until the recorder closes the file descriptors.
func (s *Store) MarkCurrent(files []*os.File) (err error) {
	if len(files) == 0 {
		return s.ClearCurrent()
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		path, relErr := filepath.Rel(s.root.Name(), f.Name())
		if relErr != nil || !filepath.IsLocal(path) {
			return fmt.Errorf("recording file is outside its storage directory: %s", f.Name())
		}
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return fmt.Errorf("lock current recording file: %w", err)
		}
		paths = append(paths, path)
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	temporary := currentName + "." + rand.Text()
	f, err := s.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return fmt.Errorf("create current recording snapshot: %w", err)
	}
	defer func() {
		if cleanup := s.root.Remove(temporary); !errors.Is(cleanup, os.ErrNotExist) {
			err = errors.Join(err, cleanup)
		}
	}()
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return fmt.Errorf("write current recording snapshot: %w", err)
	}
	if err := s.root.Rename(temporary, currentName); err != nil {
		return fmt.Errorf("publish current recording files: %w", err)
	}
	return nil
}

// ClearCurrent removes the snapshot before segment finalization.
func (s *Store) ClearCurrent() error {
	err := s.root.Remove(currentName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear current recording files: %w", err)
	}
	return nil
}

// CurrentFiles takes a snapshot without creating storage or opening capture.
// Every returned segment still has its recorder-owned exclusive lock.
func CurrentFiles(directory string) (paths []string, err error) {
	directory, err = filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve recording directory: %w", err)
	}
	r, err := os.OpenRoot(directory)
	if err != nil {
		return nil, currentError(err)
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	f, err := r.OpenFile(currentName, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, currentError(err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect current recording snapshot: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxCurrentSize {
		return nil, errors.New("invalid current recording snapshot")
	}
	var targets []string
	if err := json.NewDecoder(io.LimitReader(f, maxCurrentSize)).Decode(&targets); err != nil {
		return nil, fmt.Errorf("read current recording snapshot: %w", err)
	}
	seen := make(map[string]bool)
	for _, target := range targets {
		if !filepath.IsLocal(target) || seen[target] {
			return nil, errors.New("invalid current recording path")
		}
		seen[target] = true
		path, err := currentFile(r, target)
		if errors.Is(err, ErrNoCurrentFiles) {
			continue
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil, ErrNoCurrentFiles
	}
	return paths, nil
}

func currentFile(r *os.Root, target string) (path string, err error) {
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
		return "", errors.New("current recording path does not reference a regular file")
	}
	// Shared probes coexist and cannot mistake another query for the recorder.
	err = unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return filepath.Join(r.Name(), target), nil
	}
	if err != nil {
		return "", fmt.Errorf("check current recording lock: %w", err)
	}
	return "", ErrNoCurrentFiles
}

func currentError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrNoCurrentFiles
	}
	return fmt.Errorf("read current recording files: %w", err)
}
