package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
	"github.com/spf13/cobra"
)

func TestPrintConfiguredDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	path := filepath.Join(testdir.New(t), "settings.toml")
	if err := os.WriteFile(path, []byte("[output]\ndir = 'relative audio'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Abs("relative audio")
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		for _, tt := range []struct {
			name string
			args []string
			want string
		}{
			{"defaults", nil, filepath.Join(os.Getenv("XDG_DATA_HOME"), "ambient-recorder")},
			{"explicit config", []string{"--config", path}, relative},
		} {
			t.Run(agent+"/"+tt.name, func(t *testing.T) {
				out, diagnostic, err := execute(t, context.Background(), append([]string{"recording", "print-dir"}, tt.args...)...)
				if err != nil || diagnostic != "" || out != tt.want+"\n" {
					t.Fatalf("out=%q stderr=%q err=%v want=%q", out, diagnostic, err, tt.want)
				}
			})
		}
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "ambient-recorder")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("query created storage: %v", err)
	}
}

func TestPrintConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", "relative data path")
	defaultPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ambient-recorder", "config.toml")
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o700); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(testdir.New(t), "settings with spaces.toml")
	for _, path := range []string{defaultPath, explicit} {
		if err := os.WriteFile(path, []byte("invalid TOML ["), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, explicit)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(explicit), "linked.toml")
	if err := os.Symlink(explicit, link); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		for _, tt := range []struct {
			name string
			args []string
			want string
		}{
			{"default", nil, defaultPath},
			{"explicit", []string{"--config", explicit}, explicit},
			{"relative", []string{"--config", relative}, explicit},
			{"symlink", []string{"--config", link}, link},
		} {
			t.Run(agent+"/"+tt.name, func(t *testing.T) {
				out, diagnostic, err := execute(t, context.Background(), append([]string{"config", "print-file"}, tt.args...)...)
				if err != nil || diagnostic != "" || out != tt.want+"\n" {
					t.Fatalf("out=%q stderr=%q err=%v want=%q", out, diagnostic, err, tt.want)
				}
			})
		}
	}
}

func TestPrintConfigFileFailures(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	missing := filepath.Join(testdir.New(t), "missing", "config.toml")
	directory := testdir.New(t)
	broken := filepath.Join(directory, "broken.toml")
	if err := os.Symlink(missing, broken); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		for _, tt := range []struct {
			name string
			args []string
		}{
			{"missing default", nil},
			{"missing explicit", []string{"--config", missing}},
			{"directory", []string{"--config", directory}},
			{"broken symlink", []string{"--config", broken}},
			{"extra argument", []string{"extra"}},
		} {
			t.Run(agent+"/"+tt.name, func(t *testing.T) {
				out, diagnostic, err := execute(t, context.Background(), append([]string{"config", "print-file"}, tt.args...)...)
				if err == nil || out != "" || diagnostic != "" {
					t.Fatalf("out=%q stderr=%q err=%v", out, diagnostic, err)
				}
				if strings.HasPrefix(tt.name, "missing") || tt.name == "broken symlink" {
					if !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("lost missing-file error: %v", err)
					}
				}
			})
		}
	}
	for _, path := range []string{filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ambient-recorder"), filepath.Dir(missing)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("query created configuration directory %q: %v", path, err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", "relative config path")
	for _, verb := range []string{"print-file", "init"} {
		out, _, err := execute(t, context.Background(), "config", verb)
		if err == nil || !strings.Contains(err.Error(), "XDG_CONFIG_HOME") || (verb == "print-file" && out != "") {
			t.Fatalf("accepted invalid config location: out=%q err=%v", out, err)
		}
	}
}

func TestConfigPrintDirRemoved(t *testing.T) {
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		out, _, err := execute(t, context.Background(), "config", "print-dir")
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("accepted removed command: out=%q err=%v", out, err)
		}
	}
}

func TestPrintFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	root := testdir.New(t)
	t.Setenv("XDG_DATA_HOME", filepath.Dir(root))
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	f, err := s.CreateSegment(time.Now(), "mic-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkCurrent([]*os.File{f}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testdir.New(t), "settings.toml")
	if err := os.WriteFile(path, []byte("[output]\ndir = '"+root+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		for _, flags := range [][]string{{"--output", root}, {"--config", path}} {
			out, diagnostic, err := execute(t, context.Background(), append([]string{"recording", "print-file"}, flags...)...)
			if err != nil || diagnostic != "" || out != f.Name()+"\n" {
				t.Fatalf("out=%q stderr=%q err=%v", out, diagnostic, err)
			}
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := execute(t, context.Background(), "recording", "print-file", "--output", root)
	if err == nil || out != "" {
		t.Fatalf("reported stale file: out=%q err=%v", out, err)
	}
}

func TestPrintSourceFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	group, err := s.CreateSegments(time.Now(), []string{"mic-1", "mic-2", "playback"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, f := range group.Files {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	if err := s.MarkCurrent(group.Files); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range group.Files {
		paths = append(paths, f.Name())
	}
	for _, agent := range []string{"0", "1"} {
		t.Setenv("AGENT", agent)
		out, diagnostic, err := execute(t, context.Background(), "recording", "print-file", "--output", root)
		if err != nil || diagnostic != "" || out != strings.Join(paths, "\n")+"\n" {
			t.Fatalf("source paths: out=%q stderr=%q err=%v", out, diagnostic, err)
		}
	}
}

func TestPathOutputFailures(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	f, err := s.CreateSegment(time.Now(), "mic-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := s.MarkCurrent([]*os.File{f}); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	path := ""
	configPath := filepath.Join(root, "config.toml")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"recording directory", newPrintDirCommand(&path), nil},
		{"recording file", newPrintFileCommand(&path), []string{"--output", root}},
		{"configuration file", newConfigPrintFileCommand(&configPath), nil},
	} {
		tt.cmd.SetOut(w)
		tt.cmd.SetArgs(tt.args)
		if err := tt.cmd.Execute(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("%s lost output failure: %v", tt.name, err)
		}
	}
}

func TestPathCommandFailures(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	invalid := filepath.Join(testdir.New(t), "bad.toml")
	if err := os.WriteFile(invalid, []byte("[output]\ndir = ''\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"recording", "print-dir"}, {"recording", "print-file"}} {
		for _, flags := range [][]string{{"extra"}, {"--config", invalid}, {"--config", invalid + ".missing"}} {
			out, _, err := execute(t, context.Background(), append(append([]string{}, command...), flags...)...)
			if err == nil || out != "" {
				t.Fatalf("accepted %v %v: out=%q err=%v", command, flags, out, err)
			}
		}
	}
	for _, flags := range [][]string{nil, {"--output", ""}} {
		out, _, err := execute(t, context.Background(), append([]string{"recording", "print-file"}, flags...)...)
		if err == nil || out != "" {
			t.Fatalf("missing file: out=%q err=%v", out, err)
		}
	}
	cmd, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	cmd.SetOut(w)
	cmd.SetArgs([]string{"recording", "print-dir"})
	if err := cmd.Execute(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("lost output failure: %v", err)
	}
}
