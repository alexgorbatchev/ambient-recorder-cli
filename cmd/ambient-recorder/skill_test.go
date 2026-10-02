package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestEmbeddedSkill(t *testing.T) {
	want, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"0", "1", "true", "yes"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			out, diagnostic, err := execute(t, context.Background(), "skill")
			if err != nil || diagnostic != "" || out != string(want) {
				t.Fatalf("skill must print source bytes: err=%v stderr=%q stdout=%q", err, diagnostic, out)
			}
		})
	}
}

func TestSkillRejectsArgumentsAndFlags(t *testing.T) {
	for _, args := range [][]string{{"skill", "extra"}, {"skill", "--unknown"}} {
		if _, _, err := execute(t, context.Background(), args...); err == nil {
			t.Fatalf("skill accepted %v", args)
		}
	}
}

func TestSkillOutputFailure(t *testing.T) {
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
	cmd, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	cmd.SetOut(w)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"skill"})
	if err := cmd.Execute(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("skill must return its write failure: %v", err)
	}
}

func TestSkillWithoutRepository(t *testing.T) {
	want, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	root := testdir.New(t)
	binary := filepath.Join(filepath.Dir(filepath.Dir(root)), "bin", filepath.Base(root), "ambient-recorder")
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone binary: %v\n%s", err, out)
	}
	for _, mode := range []string{"0", "1"} {
		cmd := exec.Command(binary, "skill")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "AGENT="+mode, "GOPROXY=off")
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		out, err := cmd.Output()
		if err != nil || diagnostic.Len() != 0 || !bytes.Equal(out, want) {
			t.Fatalf("offline skill mode=%s: err=%v stderr=%s", mode, err, &diagnostic)
		}
	}
}

func TestHelpSkillAlert(t *testing.T) {
	root := liveCommands(t)
	for _, mode := range []string{"0", "1", "TRUE", " yes "} {
		t.Setenv("AGENT", mode)
		walkCommands(root, func(c *cobra.Command) {
			path := strings.Fields(strings.TrimPrefix(c.CommandPath(), root.Name()))
			for _, args := range [][]string{append(append([]string{}, path...), "--help"), append([]string{"help"}, path...)} {
				out, diagnostic, err := execute(t, context.Background(), args...)
				if err != nil || diagnostic != "" {
					t.Fatalf("help %v mode=%q: err=%v stderr=%q", args, mode, err, diagnostic)
				}
				alert := "ALERT: Agents must read `AGENT=1 ambient-recorder skill` before using this tool.\n"
				if mode == "0" {
					if strings.Contains(out, "ALERT:") {
						t.Fatalf("human help contains agent alert: %v", args)
					}
				} else if !strings.HasPrefix(out, alert) || strings.Count(out, alert) != 1 || !strings.Contains(out, "command: "+c.CommandPath()+"\n") {
					t.Fatalf("agent help %v must begin with one alert and retain framework output: %q", args, out)
				}
			}
		})
	}
}

func TestSkillLiveInterface(t *testing.T) {
	text, _, err := execute(t, context.Background(), "skill")
	if err != nil {
		t.Fatal(err)
	}
	root := liveCommands(t)
	walkCommands(root, func(c *cobra.Command) {
		arguments := strings.TrimSpace(strings.TrimPrefix(c.Use, c.Name()))
		if arguments == "" {
			arguments = "none"
		}
		row := "| `" + c.CommandPath() + "` | `" + arguments + "` |"
		if !strings.Contains(text, row) {
			t.Errorf("skill missing live command/arguments: %s", row)
		}
		c.InitDefaultHelpFlag()
		c.InitDefaultVersionFlag()
		c.Flags().AddFlagSet(c.InheritedFlags())
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			short := "none"
			if f.Shorthand != "" {
				short = "-" + f.Shorthand
			}
			value := f.DefValue
			if value == "" {
				value = `""`
			}
			metadata := " | `--" + f.Name + "` | `" + short + "` | `" + f.Value.Type() + "` | `" + value + "` |"
			if !strings.Contains(text, "| `"+c.CommandPath()+"`"+metadata) && !strings.Contains(text, "| `all`"+metadata) {
				t.Errorf("skill missing live flag for %s:%s", c.CommandPath(), metadata)
			}
		})
	})
}

func liveCommands(t *testing.T) *cobra.Command {
	t.Helper()
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return root
}

func walkCommands(c *cobra.Command, visit func(*cobra.Command)) {
	if c.Hidden {
		return
	}
	visit(c)
	for _, child := range c.Commands() {
		walkCommands(child, visit)
	}
}
