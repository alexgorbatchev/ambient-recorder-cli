package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/plist"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

const serviceProcessTest = "AMBIENT_RECORDER_TEST_SERVICE"
const serviceTestShutdown = 2 * time.Second

// This subprocess reports identity without opening audio. Delayed SIGTERM cleanup
// reproduces a recorder still exiting when launchctl bootout has already returned.
func TestServiceRestartProcess(t *testing.T) {
	if os.Getenv(serviceProcessTest) != "1" {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	s, err := serviceinfo.Listen(os.Getenv(serviceinfo.Environment), serviceinfo.Info{Version: "test-process"})
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	time.Sleep(serviceTestShutdown)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRestartWaitsForPreviousProcess(t *testing.T) {
	a := temporaryLaunchAgent(t)
	// Unix socket names must fit Darwin's 104-byte sockaddr_un.sun_path.
	dir, err := os.MkdirTemp(filepath.Dir(testdir.New(t)), "svc-")
	if err != nil {
		t.Fatal(err)
	}
	a.plist = filepath.Join(dir, "restart.plist")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a.binary = binary
	fields := map[string]any{
		"Label": a.label, "ProgramArguments": []string{binary, "-test.run=^TestServiceRestartProcess$"},
		"KeepAlive": true, "ThrottleInterval": 1,
		"EnvironmentVariables": map[string]string{serviceProcessTest: "1", serviceinfo.Environment: serviceinfo.Path(a.plist)},
		"StandardErrorPath":    filepath.Join(filepath.Dir(a.plist), "stderr.log"),
	}
	writeTestService(t, a.plist, fields)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.start(ctx, false); err != nil {
		t.Fatal(err)
	}
	waitForServiceIdentity(t, ctx, a)
	if err := a.manage(ctx, "restart"); err != nil {
		t.Fatal(err)
	}
	// Wait past the old process's teardown: its lingering socket is not proof
	// that restart created a replacement job or process.
	time.Sleep(serviceTestShutdown + 100*time.Millisecond)
	_, loaded, err := a.loaded(ctx)
	if err != nil || !loaded {
		t.Fatalf("restart returned success but the service disappeared: loaded=%t err=%v", loaded, err)
	}
	waitForServiceIdentity(t, ctx, a)
}

func TestServiceRefreshesInstalledExecutableAndIdentity(t *testing.T) {
	for _, tt := range []struct {
		name, action string
		loaded       bool
	}{
		{"start unloaded", "start", false},
		{"start loaded", "start", true},
		{"restart loaded", "restart", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := temporaryLaunchAgent(t)
			dir := filepath.Dir(a.plist)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			args := []string{"/old/ambient-recorder", "recording", "start", "--output", filepath.Join(dir, "audio"), "--config", filepath.Join(dir, "config.toml"), "--bitrate", "24000"}
			fields := map[string]any{
				"Label": a.label, "ProgramArguments": args, "KeepAlive": true,
				"EnvironmentVariables": map[string]string{"KEEP_ME": "value"},
				"ThrottleInterval":     5, "StandardErrorPath": filepath.Join(dir, "stderr.log"),
			}
			writeTestService(t, a.plist, fields)
			if tt.loaded {
				if _, err := a.run(context.Background(), "enable", a.target); err != nil {
					t.Fatal(err)
				}
				if _, err := a.run(context.Background(), "bootstrap", a.domain, a.plist); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := executeService(t, a, "", tt.action); err != nil {
				t.Fatal(err)
			}
			data, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", "--", a.plist).Output()
			if err != nil {
				t.Fatal(err)
			}
			var saved struct {
				ProgramArguments     []string
				EnvironmentVariables map[string]string
			}
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved.ProgramArguments) == 0 || saved.ProgramArguments[0] != a.binary {
				t.Errorf("%s retained the obsolete executable: %v", tt.action, saved.ProgramArguments)
			} else if strings.Join(saved.ProgramArguments[1:], "\x00") != strings.Join(args[1:], "\x00") {
				t.Errorf("%s changed saved recording arguments: %v", tt.action, saved.ProgramArguments)
			}
			if saved.EnvironmentVariables[serviceinfo.Environment] != serviceinfo.Path(a.plist) || saved.EnvironmentVariables["KEEP_ME"] != "value" {
				t.Errorf("%s did not configure live identity while retaining other environment: %v", tt.action, saved.EnvironmentVariables)
			}
			out, loaded, err := a.loaded(context.Background())
			if err != nil || !loaded || !strings.Contains(string(out), serviceinfo.Environment) || !strings.Contains(string(out), a.binary) {
				t.Errorf("%s did not load the refreshed configuration: loaded=%t err=%v report=%s", tt.action, loaded, err, out)
			}
		})
	}
}

func writeTestService(t *testing.T, path string, fields map[string]any) {
	t.Helper()
	data, err := plist.XML(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForServiceIdentity(t *testing.T, ctx context.Context, a launchAgent) {
	t.Helper()
	for ctx.Err() == nil {
		if info, err := serviceinfo.Query(ctx, serviceinfo.Path(a.plist)); err == nil && info.Version == "test-process" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service identity did not become available: %v", ctx.Err())
}
