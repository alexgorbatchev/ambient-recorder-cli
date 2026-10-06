// Package serviceinfo reports identity and recording paths from a live service.
package serviceinfo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Environment selects the service's private Unix socket in launchd's plist.
const Environment = "AMBIENT_RECORDER_SERVICE_SOCKET"

const (
	timeout         = time.Second
	maxIdentitySize = 4096
)

// Info is a snapshot of the running process, not its executable or config on disk.
type Info struct {
	Version     string `json:"version"`
	OutputDir   string `json:"output_dir"`
	ConfigPath  string `json:"config_path"`
	CurrentFile string `json:"current_file"`
}

// Path keeps the endpoint beside its per-user launchd configuration. A short
// hashed name leaves room for home directories in macOS's Unix socket limit.
func Path(plist string) string {
	sum := sha256.Sum256([]byte(filepath.Base(plist)))
	return filepath.Join(filepath.Dir(plist), fmt.Sprintf(".ar-%x.sock", sum[:4]))
}

// Server owns its listener, goroutine and exclusive endpoint lock.
type Server struct {
	mu   sync.RWMutex
	info Info
	http *http.Server
	lock *os.File
	done chan struct{}
	once sync.Once
	err  error
}

// Listen reserves the endpoint before removing a stale socket left by a crash.
// The socket and retained lock file are readable only by their owner.
func Listen(path string, info Info) (*Server, error) {
	if info.Version == "" {
		return nil, errors.New("service version is empty")
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open service identity lock: %w", err)
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("lock service identity: %w", err), lock.Close())
	}
	l, err := listen(path)
	if err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	s := &Server{info: info, lock: lock, done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.mu.RLock()
		info := s.info
		s.mu.RUnlock()
		if err := json.NewEncoder(w).Encode(info); err != nil {
			// A disconnected status client must not interrupt recording.
			return
		}
	})
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: timeout, WriteTimeout: timeout, IdleTimeout: timeout}
	go func() {
		defer close(s.done)
		if err := s.http.Serve(l); !errors.Is(err, http.ErrServerClosed) {
			s.err = err
		}
	}()
	return s, nil
}

// SetCurrentFile publishes sink transitions; an empty path means no open file.
func (s *Server) SetCurrentFile(path string) {
	s.mu.Lock()
	s.info.CurrentFile = path
	s.mu.Unlock()
}

func listen(path string) (*net.UnixListener, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("service identity path is not a socket")
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen for service identity: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, errors.Join(err, l.Close())
	}
	return l, nil
}

// Close removes the socket and waits for the server before releasing its lock.
func (s *Server) Close() error {
	s.once.Do(func() {
		err := s.http.Close()
		<-s.done
		s.err = errors.Join(s.err, err, s.lock.Close())
	})
	return s.err
}

// Query queries the live process, independently of files on disk.
func Query(ctx context.Context, path string) (Info, error) {
	d := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", path)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://service/version", nil)
	if err != nil {
		return Info{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close() // Read-only local query; no buffered writes to flush.
	var id Info
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxIdentitySize)).Decode(&id); err != nil {
		return Info{}, err
	}
	if resp.StatusCode != http.StatusOK || id.Version == "" {
		return Info{}, errors.New("invalid service identity response")
	}
	return id, nil
}
