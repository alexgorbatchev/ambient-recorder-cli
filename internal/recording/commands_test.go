package recording

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

func TestFinalizedFileCommands(t *testing.T) {
	r := newTrackRunner(t)
	t.Setenv("RECORDER_COMMAND_TEST", "inherited")
	// Check real finalized Ogg files, shell-sensitive paths and inherited env.
	r.commands = newCommands([]string{"/bin/sh", "-c", `test -s "$1" && printf '%s\n' "$1" "$RECORDER_COMMAND_TEST" > "$2.md"`, "command", "{file}", "{stem}"})
	r.out.commands = r.commands
	start := time.Date(2026, 10, 14, 9, 59, 59, 0, time.UTC)
	if err := r.out.write(start, [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
		t.Fatal(err)
	}
	paths := r.out.paths()
	for _, path := range paths {
		if _, err := os.Stat(strings.TrimSuffix(path, ".opus") + ".md"); !os.IsNotExist(err) {
			t.Fatalf("command ran before finalization: %v", err)
		}
	}
	if err := r.out.write(start.Add(time.Second), [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, r.out.paths()...)
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	r.closeCommands()
	for _, path := range paths {
		b, err := os.ReadFile(strings.TrimSuffix(path, ".opus") + ".md")
		if err != nil || string(b) != path+"\ninherited\n" {
			t.Fatalf("output for %s: %q %v", path, b, err)
		}
	}
}

func TestCommandsDoNotBlockRecording(t *testing.T) {
	r := newTrackRunner(t)
	gate := filepath.Join(r.cfg.Output, "gate")
	started := filepath.Join(r.cfg.Output, "started")
	r.commands = newCommands([]string{"/bin/sh", "-c", `touch "$1"; while test ! -f "$2"; do sleep 0.01; done`, "command", started, gate})
	r.out.commands = r.commands
	t.Cleanup(func() {
		if err := os.WriteFile(gate, nil, 0600); err != nil {
			t.Error(err)
		}
		r.closeCommands()
	})
	start := time.Now()
	if err := r.out.write(start, [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
		t.Fatal(err)
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("command did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if err := r.out.write(start.Add(time.Second), [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
		t.Fatal(err)
	}
	if len(r.out.paths()) != 2 {
		t.Fatal("recording stopped while command waited")
	}
	if err := r.out.close(); err != nil {
		t.Fatal(err)
	}
}

func TestCommandExecution(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command []string
		cancel  bool
		wantErr bool
	}{
		{"literal arguments", []string{"/bin/echo", "{file}", "{stem}", "$(touch injected)"}, false, false},
		{"failed", []string{"/bin/sh", "-c", "echo command-failed >&2; exit 7"}, false, true},
		{"missing", []string{"/no/such/executable"}, false, true},
		{"cancelled", []string{"/bin/sleep", "30"}, true, true},
		{"cancelled shell children", []string{"/bin/sh", "-c", "sleep 30 & wait"}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newTrackRunner(t)
			path := filepath.Join(r.cfg.Output, "a '$ weird.opus")
			ctx := context.Background()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Millisecond)
				defer cancel()
			}
			err := runFileCommand(ctx, tt.command, path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("command error: %v", err)
			}
			if !tt.wantErr {
				b, err := os.ReadFile(strings.TrimSuffix(path, ".opus") + ".command.log")
				if err != nil || !strings.Contains(string(b), path) || !strings.Contains(string(b), "$(touch injected)") {
					t.Fatalf("literal args lost: %q %v", b, err)
				}
			}
		})
	}
}

func TestDisabledAndFailedFileCommands(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "failure does not stop worker"}[enabled], func(t *testing.T) {
			r := newTrackRunner(t)
			var args []string
			if enabled {
				args = []string{"/bin/sh", "-c", "echo failed >&2; exit 7"}
			}
			r.commands = newCommands(args)
			r.out.commands = r.commands
			if err := r.out.write(time.Now(), [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
				t.Fatal(err)
			}
			paths := r.out.paths()
			if err := r.out.close(); err != nil {
				t.Fatal(err)
			}
			r.closeCommands()
			for _, path := range paths {
				b, err := os.ReadFile(strings.TrimSuffix(path, ".opus") + ".command.log")
				if enabled && (err != nil || !strings.Contains(string(b), "failed")) {
					t.Fatalf("failed job was skipped: %s %q %v", path, b, err)
				}
				if !enabled && !os.IsNotExist(err) {
					t.Fatalf("disabled hook created log: %v", err)
				}
			}
			if enabled {
				b, err := os.ReadFile(filepath.Join(r.cfg.Output, time.Now().Format("2006/01/02"), "log.nljson"))
				if err != nil || strings.Count(string(b), "Recording command failed") != 2 {
					t.Fatalf("missing failure diagnostics: %s %v", b, err)
				}
			}
		})
	}
}

func TestUnfinalizedFilesDoNotRunCommands(t *testing.T) {
	r := newTrackRunner(t)
	r.commands = newCommands([]string{"/bin/echo", "unexpected"})
	r.out.commands = r.commands
	if err := r.out.write(time.Now(), [][]float32{make([]float32, 480), make([]float32, 480)}); err != nil {
		t.Fatal(err)
	}
	s := r.out.sinks["mic-1"]
	path := strings.TrimSuffix(s.file.Name(), ".opus") + ".command.log"
	if err := s.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.out.close(); err == nil {
		t.Fatal("closed descriptor should fail finalization")
	}
	r.closeCommands()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("failed finalization ran command: %v", err)
	}
}

func TestCommandLogFailureAndShutdownProgress(t *testing.T) {
	r := newTrackRunner(t)
	if err := runFileCommand(context.Background(), []string{"/bin/true"}, filepath.Join(r.cfg.Output, "missing", "source.opus")); err == nil {
		t.Fatal("missing log directory accepted")
	}
	r.commands = newCommands([]string{"/bin/sleep", "1.1"})
	r.commands.enqueue(filepath.Join(r.cfg.Output, "source.opus"))
	r.closeCommands()
	select {
	case <-r.progress:
	default:
		t.Fatal("waiting for commands starved watchdog")
	}
}

func TestRunOptionalCommandWithoutCapture(t *testing.T) {
	r := newTrackRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	cfg := Config{Output: r.cfg.Output, Bitrate: 32000, Complexity: 2, SyncInterval: time.Second, Microphones: []string{"!*"}, OnCloseCommand: []string{"/bin/true"}}
	if err := Run(ctx, cfg, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CurrentFiles(cfg.Output); err == nil {
		t.Fatal("excluded inputs opened recording")
	}
	cfg.OnCloseCommand = []string{""}
	if err := Run(context.Background(), cfg, nil); err == nil {
		t.Fatal("invalid command accepted by Run")
	}
}
