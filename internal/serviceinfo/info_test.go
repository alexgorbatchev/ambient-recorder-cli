package serviceinfo

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func socketPath(t *testing.T) string {
	t.Helper()
	dir := testdir.New(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(rel, "v.sock")
}

func TestLiveVersion(t *testing.T) {
	path := Path(filepath.Join(filepath.Dir(socketPath(t)), "example.plist"))
	s, err := Listen(path, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, file := range []string{path, path + ".lock"} {
		info, err := os.Stat(file)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private endpoint: %v %v", info, err)
		}
	}
	if _, err := Listen(path, "2.0.0"); err == nil {
		t.Fatal("replaced a live version endpoint")
	}
	for range 3 {
		v, err := Version(context.Background(), path)
		if err != nil || v != "1.1.0" {
			t.Fatalf("live identity: %q %v", v, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Version(context.Background(), path); err == nil {
		t.Fatal("closed endpoint remained available")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}
	next, err := Listen(path, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := next.Close(); err != nil {
			t.Error(err)
		}
	})
	if v, err := Version(context.Background(), path); err != nil || v != "2.0.0" {
		t.Fatalf("new identity: %q %v", v, err)
	}
}

func TestStaleSocket(t *testing.T) {
	path := socketPath(t)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	l.SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Listen(path, "3.0.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	if v, err := Version(context.Background(), path); err != nil || v != "3.0.0" {
		t.Fatalf("stale socket recovery: %q %v", v, err)
	}
}

func TestListenFailures(t *testing.T) {
	path := socketPath(t)
	if err := os.WriteFile(path, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, path, version string }{
		{"empty version", path, ""},
		{"non socket", path, "1"},
		{"missing parent", filepath.Join(path, "missing"), "1"},
		{"long path", filepath.Join(filepath.Dir(path), strings.Repeat("x", 110)), "1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Listen(tt.path, tt.version); err == nil {
				t.Fatal("invalid endpoint accepted")
			}
		})
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "preserved" {
		t.Fatalf("non-socket overwritten: %q %v", data, err)
	}
}

func TestVersionFailures(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		code       int
		delay      time.Duration
	}{
		{"invalid json", "broken", 200, 0},
		{"empty version", `{"version":""}`, 200, 0},
		{"error response", `{"version":"1"}`, 500, 0},
		{"oversize", strings.Repeat(" ", 4096) + `{"version":"1"}`, 200, 0},
		{"timeout", `{"version":"1"}`, 200, timeout + 100*time.Millisecond},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := socketPath(t)
			l, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.delay)
				w.WriteHeader(tt.code)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					return
				}
			})}
			done := make(chan struct{})
			go func() {
				defer close(done)
				if err := s.Serve(l); !errors.Is(err, http.ErrServerClosed) {
					t.Error(err)
				}
			}()
			defer func() {
				if err := s.Close(); err != nil {
					t.Error(err)
				}
				<-done
			}()
			if _, err := Version(context.Background(), path); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
	if _, err := Version(nil, socketPath(t)); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Version(ctx, socketPath(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}
