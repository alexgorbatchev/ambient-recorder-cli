package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/plist"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestServiceStatusPaths(t *testing.T) {
	a := temporaryLaunchAgent(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a.plist, err = filepath.Rel(cwd, a.plist)
	if err != nil {
		t.Fatal(err)
	}
	dir := testdir.New(t)
	saved := agentSettings{output: filepath.Join(dir, "saved audio & notes"), config: filepath.Join(dir, "saved config.toml")}
	if err := a.install(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	live := serviceinfo.Info{Version: "live-version", OutputDir: filepath.Join(dir, "live audio"), ConfigPath: filepath.Join(dir, "live config.toml")}
	s, err := serviceinfo.Listen(serviceinfo.Path(a.plist), live)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, current := range []string{"", filepath.Join(live.OutputDir, "first.opus"), filepath.Join(live.OutputDir, "next.opus"), ""} {
		s.SetCurrentFile(current)
		for _, mode := range []string{"0", "1"} {
			t.Setenv("AGENT", mode)
			out, err := executeService(t, a, filepath.Join(dir, "unrelated missing.toml"), "status")
			if err != nil {
				t.Fatal(err)
			}
			wantCurrent := current
			if wantCurrent == "" {
				wantCurrent = "none"
			}
			for label, value := range map[string]string{"Recording directory": live.OutputDir, "Configuration file": live.ConfigPath, "Current recording file": wantCurrent, "Paths from": "live service"} {
				want := label + ": " + value + "\n"
				if mode == "1" {
					key := map[string]string{"Recording directory": "output_dir", "Configuration file": "config_path", "Current recording file": "current_file", "Paths from": "paths_source"}[label]
					if key == "paths_source" {
						value = "live"
					}
					want = fmt.Sprintf("%s=%q", key, value)
				}
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in %q", want, out)
				}
			}
			if strings.Contains(out, saved.output) {
				t.Errorf("saved settings override live service: %q", out)
			}
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, stopped := range []bool{false, true} {
		if stopped {
			if _, err := executeService(t, a, "", "stop"); err != nil {
				t.Fatal(err)
			}
		}
		for _, mode := range []string{"0", "1"} {
			t.Setenv("AGENT", mode)
			out, err := executeService(t, a, "", "status")
			if err != nil || !strings.Contains(out, saved.output) || !strings.Contains(out, saved.config) || strings.Contains(out, live.OutputDir) {
				t.Fatalf("saved paths: %q %v", out, err)
			}
			want := "unavailable"
			if stopped {
				want = "not running"
			}
			if !strings.Contains(out, want) {
				t.Fatalf("current state missing: %q", out)
			}
		}
	}
}

func TestRecordingServicePaths(t *testing.T) {
	dir := testdir.New(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rel, "info.sock")
	t.Setenv(serviceinfo.Environment, path)
	cfg := recording.Config{Output: filepath.Join(rel, "audio")}
	configPath := filepath.Join(dir, "config.toml")
	err = withServiceInfo("live-version", configPath, cfg, func(resolved recording.Config) error {
		wantOutput := filepath.Join(dir, "audio")
		for _, current := range []string{"", filepath.Join(wantOutput, "one.opus"), ""} {
			resolved.CurrentFileChanged(current)
			info, err := serviceinfo.Query(context.Background(), path)
			if err != nil || info.OutputDir != wantOutput || info.ConfigPath != configPath || info.CurrentFile != current || info.Version != "live-version" {
				t.Fatalf("live snapshot: %+v %v", info, err)
			}
		}
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("recording error: %v", err)
	}
	if _, err := serviceinfo.Query(context.Background(), path); err == nil {
		t.Fatal("stopped recorder still reports paths")
	}
}

func TestSavedServicePaths(t *testing.T) {
	dir := testdir.New(t)
	output, configPath := filepath.Join(dir, "audio & 路径"), filepath.Join(dir, "config.toml")
	for _, tt := range []struct {
		name  string
		args  []string
		valid bool
	}{
		{"normal", []string{"/binary", "recording", "start", "--output", output, "--config", configPath}, true},
		{"reordered", []string{"/binary", "--config=" + configPath, "recording", "start", "--output=" + output}, true},
		{"empty", nil, false},
		{"other command", []string{"/binary", "config", "init"}, false},
		{"unknown flag", []string{"/binary", "recording", "start", "--invalid"}, false},
		{"extra argument", []string{"/binary", "recording", "start", "unexpected"}, false},
		{"relative output", []string{"/binary", "recording", "start", "--output", "relative", "--config", configPath}, false},
		{"implicit config", []string{"/binary", "recording", "start", "--output", output}, false},
		{"wrong type", nil, false},
		{"invalid plist", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := launchAgent{plist: filepath.Join(dir, tt.name+".plist")}
			var args any = tt.args
			if tt.name == "wrong type" {
				args = 42
			}
			data, err := plist.XML(map[string]any{"ProgramArguments": args})
			if tt.name == "empty" {
				data, err = plist.XML(map[string]any{})
			}
			if tt.name == "invalid plist" {
				data, err = []byte("broken"), nil
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(a.plist, data, 0o600); err != nil {
				t.Fatal(err)
			}
			gotOutput, gotConfig, err := a.savedRecordingPaths(context.Background())
			if (err == nil) != tt.valid {
				t.Fatalf("paths=%q %q err=%v", gotOutput, gotConfig, err)
			}
			if tt.valid && (gotOutput != output || gotConfig != configPath) {
				t.Fatalf("saved paths changed: %q %q", gotOutput, gotConfig)
			}
			p, err := a.recordingPaths(context.Background(), true, false, serviceinfo.Info{})
			if err != nil {
				t.Fatal(err)
			}
			if !tt.valid && (p.source != "unavailable" || p.output != "unavailable" || p.config != "unavailable") {
				t.Fatalf("invalid paths inferred: %+v", p)
			}
		})
	}
	a := launchAgent{plist: filepath.Join(dir, "missing.plist")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.recordingPaths(ctx, true, false, serviceinfo.Info{}); err != context.Canceled {
		t.Fatalf("cancellation ignored: %v", err)
	}
	p, err := a.recordingPaths(context.Background(), false, false, serviceinfo.Info{})
	if err != nil || p.current != "not running" || p.source != "unavailable" {
		t.Fatalf("uninstalled paths: %+v %v", p, err)
	}
	p, err = a.recordingPaths(context.Background(), false, true, serviceinfo.Info{OutputDir: output})
	if err != nil || p.config != "none (defaults)" || p.current != "none" {
		t.Fatalf("implicit defaults: %+v %v", p, err)
	}
}
