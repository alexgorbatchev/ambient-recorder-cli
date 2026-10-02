package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func TestHumanHelpTreeAndWidth(t *testing.T) {
	t.Setenv("AGENT", "0")
	for _, columns := range []string{"", "80", "100"} {
		t.Setenv("COLUMNS", columns)
		out, diagnostic, err := execute(t, context.Background(), "--help")
		if err != nil || diagnostic != "" || !strings.Contains(out, "├─") || !strings.Contains(out, "╰─") || !strings.Contains(out, "skill") || strings.Contains(out, "completion") {
			t.Fatalf("root tree: err=%v stderr=%q stdout=%q", err, diagnostic, out)
		}
		width := 80
		if columns == "100" {
			width = 100
		}
		for _, line := range strings.Split(out, "\n") {
			if runewidth.StringWidth(line) > width {
				t.Fatalf("human help exceeded %d columns: %q", width, line)
			}
		}
		walkCommands(liveCommands(t), func(c *cobra.Command) {
			args := strings.Fields(strings.TrimPrefix(c.CommandPath(), "ambient-recorder"))
			out, diagnostic, err := execute(t, context.Background(), append(args, "--help")...)
			if err != nil || diagnostic != "" || (c.HasAvailableSubCommands() && !strings.Contains(out, "╰─")) {
				t.Fatalf("subtree %s: err=%v stdout=%q stderr=%q", c.CommandPath(), err, out, diagnostic)
			}
			for _, line := range strings.Split(out, "\n") {
				if runewidth.StringWidth(line) > width {
					t.Errorf("%s help exceeded %d columns: %q", c.CommandPath(), width, line)
				}
			}
		})
	}
}

func TestAgentHelpUsageAndWriteErrors(t *testing.T) {
	t.Setenv("AGENT", "1")
	t.Setenv("COLUMNS", "20")
	root := liveCommands(t)
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	if err := root.Usage(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "ALERT:") || !strings.Contains(output.String(), "TOML configuration file (defaults to XDG configuration/ambient-recorder/config.toml)") {
		t.Fatalf("agent usage lost alert or clipped details: %s", &output)
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
	root.SetOut(w)
	root.SetErr(&output)
	if err := root.Usage(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("usage must return failed alert write: %v", err)
	}
	output.Reset()
	root.HelpFunc()(root, nil)
	if !strings.Contains(output.String(), "file already closed") {
		t.Fatalf("help must report failed alert write: %s", &output)
	}
}
