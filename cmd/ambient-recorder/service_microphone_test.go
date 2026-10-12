package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestRecordingServiceMicrophone(t *testing.T) {
	dir := testdir.New(t)
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	// Use a relative socket address to fit Darwin's Unix socket path limit.
	path, err := filepath.Rel(cwd, filepath.Join(dir, "mic.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(serviceinfo.Environment, path)
	err = withServiceInfo("mic-version", "", recording.Config{}, func(cfg recording.Config) error {
		for _, name := range []string{"", "USB Headset", "", "Built-in Microphone", ""} {
			cfg.MicrophoneChanged(name)
			info, err := serviceinfo.Query(context.Background(), path)
			if err != nil || info.Microphone == nil || *info.Microphone != name {
				t.Fatalf("active microphone snapshot: %+v err=%v; want %q", info, err, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceStatusMicrophone(t *testing.T) {
	a := temporaryLaunchAgent(t)
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	a.plist, err = filepath.Rel(cwd, a.plist)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.install(context.Background(), agentSettings{output: testdir.New(t)}); err != nil {
		t.Fatal(err)
	}
	s, err := serviceinfo.Listen(serviceinfo.Path(a.plist), serviceinfo.Info{Version: "mic-version"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, tt := range []struct{ name, want string }{
		{"unreported", "unavailable"},
		{"USB Headset", "USB Headset"},
		{"", "none"},
		{"Built-in Microphone", "Built-in Microphone"},
	} {
		if tt.name != "unreported" {
			s.SetMicrophone(tt.name)
		}
		for _, mode := range []string{"0", "1"} {
			t.Setenv("AGENT", mode)
			out, err := executeService(t, a, "", "status")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "0" && humanStatusValue(t, out, "Current microphone") != tt.want || mode == "1" && !strings.Contains(out, fmt.Sprintf("microphone=%q", tt.want)) {
				t.Fatalf("missing active microphone %q: %q", tt.want, out)
			}
		}
	}
	if err := a.manage(context.Background(), "stop"); err != nil {
		t.Fatal(err)
	}
	out, err := executeService(t, a, "", "status")
	if err != nil || strings.Contains(out, "Built-in Microphone") || !strings.Contains(out, `microphone="not running"`) {
		t.Fatalf("unloaded job reported an unrelated microphone: %q %v", out, err)
	}
}
