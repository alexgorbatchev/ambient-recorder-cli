package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestLoadTOML(t *testing.T) {
	path := filepath.Join(testdir.New(t), "config.toml")
	input := "[input]\nmicrophones = ['LG Ultra*', '!Virtual*', 'uid:headset', '*']\n[output]\ndir = 'meeting audio'\nbitrate = 24000\ncomplexity = 0\nsync_interval = '2s'\n"
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path || cfg.Output.Dir != "meeting audio" || cfg.Output.Bitrate != 24000 || cfg.Output.Complexity != 0 || cfg.Output.SyncInterval != "2s" || !slices.Equal(cfg.Input.Microphones, []string{"LG Ultra*", "!Virtual*", "uid:headset", "*"}) {
		t.Fatalf("configuration=%+v path=%q", cfg, loaded)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationRejectsErrors(t *testing.T) {
	for _, input := range []string{
		"unknown = true", "input.unknown = true", "output.unknown = true",
		"output.bitrate = 'fast'", "output.bitrate = 1", "output.complexity = 11",
		"output.sync_interval = 'soon'", "output.sync_interval = '0s'", "output.dir = ''",
		"input.microphones = ['']", "input.microphones = ['uid:']", "input.microphones = ['   ']",
		"input.microphones = ['!']", "input.microphones = ['!   ']", "input.microphones = ['!uid:']",
		"input.microphones = ['uid:   ']", "input.microphones = ['!uid:   ']", "input.microphones = [",
		"output = 'recordings'", "bitrate = 32000", "complexity = 2", "sync_interval = '5s'", "microphones = ['*']",
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(testdir.New(t), "config.toml")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, _, err := Load(path)
			if err == nil {
				err = cfg.Validate()
			}
			if err == nil {
				t.Fatalf("accepted invalid configuration: %s", input)
			}
		})
	}
}

func TestDefaultConfigAndExplicitMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	cfg, loaded, err := Load("")
	if err != nil || loaded != "" {
		t.Fatalf("missing optional default: %v loaded=%q", err, loaded)
	}
	if !slices.Equal(cfg.Input.Microphones, []string{"*"}) || !strings.HasPrefix(cfg.Output.Dir, os.Getenv("XDG_DATA_HOME")) {
		t.Fatalf("fallback configuration=%+v", cfg)
	}
	if _, _, err := Load(filepath.Join(testdir.New(t), "missing.toml")); err == nil {
		t.Fatal("missing explicit config was silently ignored")
	}
	path, err := DefaultPath()
	if err != nil || path != filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ambient-recorder", "config.toml") {
		t.Fatalf("XDG config path=%s error=%v", path, err)
	}
}

func TestPartialConfigurationDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[input]\nmicrophones = ['Office USB microphone']\n[output]\ncomplexity = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path || cfg.Output.Bitrate != 32000 || cfg.Output.Complexity != 0 || cfg.Output.SyncInterval != "5s" || !slices.Equal(cfg.Input.Microphones, []string{"Office USB microphone"}) {
		t.Fatalf("partial file failed to overlay defaults: %+v, path=%s", cfg, loaded)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigCreationDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(testdir.New(t), "settings", "config.toml")
	if err := Create(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Create(path); err == nil {
		t.Fatal("overwrote existing configuration")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("existing configuration changed")
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions: %v %v", info, err)
	}
}
