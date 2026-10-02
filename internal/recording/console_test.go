package recording

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestConsoleTerminalColors(t *testing.T) {
	if os.Getenv("RECORDER_CONSOLE_CHILD") == "1" {
		j := testJournal(t, os.Stderr, os.Getenv("RECORDER_CONSOLE_AGENT") == "true")
		j.event(slog.LevelWarn, "Capture unavailable", "microphone_name", "Desk mic")
		return
	}
	cases := []struct {
		name, term, noColor string
		agent, color        bool
	}{
		{name: "terminal", term: "xterm-256color", color: true},
		{name: "NO_COLOR", term: "xterm-256color", noColor: "1"},
		{name: "dumb terminal", term: "dumb"},
		{name: "agent", term: "xterm-256color", agent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			t.Setenv("NO_COLOR", tc.noColor)
			path := filepath.Join(testdir.New(t), "terminal.log")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// macOS script supplies a real pseudo-terminal without a test double.
			cmd := exec.CommandContext(ctx, "script", "-q", path, os.Args[0], "-test.run=^TestConsoleTerminalColors$")
			cmd.Env = append(os.Environ(), "RECORDER_CONSOLE_CHILD=1", "RECORDER_CONSOLE_AGENT="+strconv.FormatBool(tc.agent))
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "Capture unavailable") || !strings.Contains(string(out), "PASS") {
				t.Fatalf("terminal logging subprocess failed: %v\n%s", err, out)
			}
			if color := strings.Contains(string(out), "\x1b["); color != tc.color {
				t.Fatalf("terminal color=%v, want %v: %q", color, tc.color, out)
			}
			if tc.agent && !strings.Contains(string(out), `"level":"WARN"`) {
				t.Fatalf("agent output lost its JSON format on a terminal: %q", out)
			}
		})
	}
}
