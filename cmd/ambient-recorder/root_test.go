package main

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestHelpAndVersion(t *testing.T) {
	for _, agent := range []string{"0", "1"} {
		t.Run(agent, func(t *testing.T) {
			t.Setenv("AGENT", agent)
			out, diagnostic, err := execute(t, context.Background(), "--help")
			if err != nil || diagnostic != "" || !strings.Contains(out, "recording") {
				t.Fatalf("help out=%q stderr=%q err=%v", out, diagnostic, err)
			}
			out, diagnostic, err = execute(t, context.Background(), "recording", "--help")
			if err != nil || diagnostic != "" || !strings.Contains(out, "start") {
				t.Fatalf("recording help out=%q stderr=%q err=%v", out, diagnostic, err)
			}
			out, diagnostic, err = execute(t, context.Background(), "--version")
			if err != nil || diagnostic != "" || out != version+"\n" {
				t.Fatalf("version out=%q stderr=%q err=%v", out, diagnostic, err)
			}
		})
	}
}

func TestServiceConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	root := testdir.New(t) + "/audio & notes"
	out, diagnostic, err := execute(t, context.Background(), "service", "print", "--output", root)
	if err != nil || diagnostic != "" {
		t.Fatalf("service configuration: %v %s", err, diagnostic)
	}
	cmd := exec.Command("plutil", "-lint", "-")
	cmd.Stdin = strings.NewReader(out)
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invalid launchd plist: %v\n%s\n%s", err, result, out)
	}
	if !strings.Contains(out, "audio &amp; notes") || !strings.Contains(out, "<true/>") || !strings.Contains(out, "recording") || !strings.Contains(out, "start") {
		t.Fatalf("incomplete launchd configuration: %s", out)
	}
}

func TestRecordingValidationAndCancellation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	for _, flags := range [][]string{{"--bitrate", "1"}, {"--complexity", "11"}, {"--sync-interval", "0s"}, {"extra"}} {
		_, _, err := execute(t, context.Background(), append([]string{"recording", "start", "--output", testdir.New(t)}, flags...)...)
		if err == nil {
			t.Fatalf("accepted invalid arguments: %v", flags)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := execute(t, ctx, "recording", "start", "--output", testdir.New(t)); err != nil {
		t.Fatalf("pre-canceled recording: %v", err)
	}
}

func execute(t *testing.T, ctx context.Context, args ...string) (string, string, error) {
	t.Helper()
	cmd, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostic)
	cmd.SetArgs(args)
	err = cmd.ExecuteContext(ctx)
	return out.String(), diagnostic.String(), err
}
