package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func humanStatusValue(t *testing.T, out, label string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if value, ok := strings.CutPrefix(line, label+"  "); ok {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("status row %q missing from %q", label, out)
	return ""
}

func TestHumanServiceStatusTable(t *testing.T) {
	a := temporaryLaunchAgent(t)
	t.Setenv("HOME", filepath.Dir(a.plist))
	t.Setenv("AGENT", "0")
	out, err := executeService(t, a, "", "status")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ label, value string }{
		{"Installed", "false"},
		{"Loaded by macOS", "false"},
		{"CLI version", version},
		{"Running service version", "not running"},
		{"Service file", "~/" + filepath.Base(a.plist)},
		{"Recording directory", "unavailable"},
		{"Configuration file", "unavailable"},
		{"Current recording file", "not running"},
		{"Current microphone", "not running"},
		{"Paths from", "unavailable"},
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != len(want) {
		t.Fatalf("status has extra headers or borders: %q", out)
	}
	column := -1
	for i, row := range want {
		if !strings.HasPrefix(lines[i], row.label+"  ") || strings.TrimSpace(strings.TrimPrefix(lines[i], row.label)) != row.value {
			t.Errorf("status row %q does not contain value %q in two plain columns: %q", row.label, row.value, lines[i])
			continue
		}
		start := len(lines[i]) - len(strings.TrimLeft(strings.TrimPrefix(lines[i], row.label), " "))
		if column < 0 {
			column = start
		} else if start != column {
			t.Errorf("status values are not aligned: %q", out)
		}
	}
}

func TestServiceStatusHomePaths(t *testing.T) {
	a := temporaryLaunchAgent(t)
	dir := testdir.New(t)
	if err := a.install(context.Background(), agentSettings{output: filepath.Join(dir, "audio & notes"), config: filepath.Join(dir, "config.toml")}); err != nil {
		t.Fatal(err)
	}
	for _, home := range []string{dir, dir + "-sibling", ""} {
		t.Setenv("HOME", home)
		for _, mode := range []string{"0", "1"} {
			t.Setenv("AGENT", mode)
			out, err := executeService(t, a, "", "status")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "0" && home == dir {
				if !strings.Contains(out, "~/audio & notes") || !strings.Contains(out, "~/config.toml") || strings.Contains(out, dir) {
					t.Fatalf("home-directory paths were not shortened: %q", out)
				}
			} else if !strings.Contains(out, filepath.Join(dir, "audio & notes")) || !strings.Contains(out, filepath.Join(dir, "config.toml")) {
				t.Fatalf("status changed paths outside HOME or in machine output: %q", out)
			}
		}
	}
	// A directory equal to HOME displays the requested trailing slash too.
	t.Setenv("HOME", filepath.Join(dir, "audio & notes"))
	t.Setenv("AGENT", "0")
	out, err := executeService(t, a, "", "status")
	if err != nil || !strings.Contains(out, "~/\n") {
		t.Fatalf("home directory itself was not abbreviated: %q %v", out, err)
	}
	// Live recording paths use the same display policy as saved settings.
	// Resolve relative to the working directory to fit Darwin's socket limit.
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	a.plist, err = filepath.Rel(cwd, a.plist)
	if err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(dir, "audio & notes", "current.opus")
	s, err := serviceinfo.Listen(serviceinfo.Path(a.plist), serviceinfo.Info{Version: "live-version", OutputDir: filepath.Dir(current), ConfigPath: filepath.Join(dir, "config.toml"), CurrentFile: current})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("HOME", dir)
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, err := executeService(t, a, "", "status")
		if err != nil {
			t.Fatal(err)
		}
		if mode == "0" && humanStatusValue(t, out, "Current recording file") != "~/audio & notes/current.opus" || mode == "1" && !strings.Contains(out, current) {
			t.Fatalf("live recording path: %q", out)
		}
	}
}
