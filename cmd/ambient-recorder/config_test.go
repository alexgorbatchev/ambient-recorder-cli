package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
	"github.com/spf13/cobra"
)

func TestConfigInitAndDefaultDiscovery(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ambient-recorder", "config.toml")
	out, diagnostic, err := execute(t, context.Background(), "config", "init")
	if err != nil || diagnostic != "" || !strings.Contains(out, path) {
		t.Fatalf("init: %q %q %v", out, diagnostic, err)
	}
	if _, _, err := execute(t, context.Background(), "config", "init"); err == nil {
		t.Fatal("config init overwrote an existing file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := execute(t, ctx, "recording", "start"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[output]\nbitrate = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, ctx, "recording", "start"); err == nil {
		t.Fatal("recording ignored invalid default config")
	}
	if _, _, err := execute(t, ctx, "recording", "start", "--bitrate", "32000"); err != nil {
		t.Fatalf("explicit flag failed to override file value: %v", err)
	}
}

func TestConfigOverrides(t *testing.T) {
	path := filepath.Join(testdir.New(t), "config.toml")
	input := "[input]\nmicrophones = ['uid:headset', '*']\n[output]\ndir = 'configured audio'\nbitrate = 24000\ncomplexity = 0\nsync_interval = '2s'\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		cmd, err := newRootCommand()
		if err != nil {
			t.Fatal(err)
		}
		var actual recording.Config
		start, _, err := cmd.Find([]string{"recording", "start"})
		if err != nil {
			t.Fatal(err)
		}
		// Inspect the real configuration resolver without opening audio hardware.
		start.RunE = func(cmd *cobra.Command, args []string) error {
			actual, _, err = recordingConfig(cmd, path)
			return err
		}
		args := []string{"recording", "start", "--config", path}
		if override {
			args = append(args, "--output", "flag audio", "--bitrate", "48000", "--complexity", "4", "--sync-interval", "7s")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(actual.Microphones, []string{"uid:headset", "*"}) {
			t.Fatalf("preferences lost: %+v", actual)
		}
		if override {
			if actual.Output != "flag audio" || actual.Bitrate != 48000 || actual.Complexity != 4 || actual.SyncInterval != 7*time.Second {
				t.Fatalf("flag precedence: %+v", actual)
			}
		} else if actual.Output != "configured audio" || actual.Bitrate != 24000 || actual.Complexity != 0 || actual.SyncInterval != 2*time.Second {
			t.Fatalf("file settings: %+v", actual)
		}
	}
}

func TestServiceUsesExplicitConfiguration(t *testing.T) {
	dir := testdir.New(t)
	path := filepath.Join(dir, "config & audio.toml")
	output := filepath.Join(dir, "recordings")
	input := "[input]\nmicrophones = ['uid:display', '*']\n[output]\ndir = '" + output + "'\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	out, diagnostic, err := execute(t, context.Background(), "service", "print", "--config", path)
	if err != nil || diagnostic != "" {
		t.Fatalf("service: %v %s", err, diagnostic)
	}
	if !strings.Contains(out, "--config") || !strings.Contains(out, "config &amp; audio.toml") || !strings.Contains(out, output) {
		t.Fatalf("service lost configuration: %s", out)
	}
	if _, _, err := execute(t, context.Background(), "service", "print", "--config", path+".missing"); err == nil {
		t.Fatal("service ignored missing explicit config")
	}
}
