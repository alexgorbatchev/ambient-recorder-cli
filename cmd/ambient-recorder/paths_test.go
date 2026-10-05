package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
				out, diagnostic, err := execute(t, context.Background(), append([]string{"config", "print-dir"}, tt.args...)...)
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
	f, err := s.CreateSegment(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkCurrent(f); err != nil {
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
	f, err := s.CreateSegment(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := s.MarkCurrent(f); err != nil {
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
	for _, cmd := range []*cobra.Command{newPrintDirCommand(&path), newPrintFileCommand(&path)} {
		cmd.SetOut(w)
		if cmd.Name() == "print-file" {
			cmd.SetArgs([]string{"--output", root})
		} else {
			cmd.SetArgs(nil)
		}
		if err := cmd.Execute(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("%s lost output failure: %v", cmd.Name(), err)
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
	for _, command := range [][]string{{"config", "print-dir"}, {"recording", "print-file"}} {
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
	cmd.SetArgs([]string{"config", "print-dir"})
	if err := cmd.Execute(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("lost output failure: %v", err)
	}
}
