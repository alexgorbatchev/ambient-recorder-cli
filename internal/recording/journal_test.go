package recording

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestJournalConsoleLevels(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var console bytes.Buffer
		j := testJournal(t, &console, agent)
		for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
			j.event(level, "capture event", "microphone_name", "USB Microphone", "error", errors.New("device unavailable"))
		}
		lines := strings.Split(strings.TrimSpace(console.String()), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected INFO/WARN/ERROR once each on stderr: %q", console.String())
		}
		for i, line := range lines {
			level := []string{"INFO", "WARN", "ERROR"}[i]
			if agent {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				if record["level"] != level || record["microphone_name"] != "USB Microphone" || record["error"] != "device unavailable" {
					t.Fatalf("incorrect console log: %v", record)
				}
			} else {
				tag := []string{"INF", "WRN", "ERR"}[i]
				parts := strings.SplitN(line, " "+tag+" ", 2)
				if len(parts) != 2 || !strings.HasPrefix(parts[1], "capture event ") || strings.Contains(line, "\x1b") {
					t.Fatalf("redirected console must show a plain tint event: %s", line)
				}
				if _, err := time.Parse("2006-01-02 15:04:05.000 -0700", parts[0]); err != nil {
					t.Fatalf("console header missing readable timestamp: %v", err)
				}
			}
		}
		if !agent && (!strings.Contains(console.String(), `microphone="USB Microphone"`) || !strings.Contains(console.String(), `error="device unavailable"`)) {
			t.Fatalf("human logs lost diagnostic fields: %s", &console)
		}
		if records := journalRecords(t, j); len(records) != 4 || records[0]["level"] != "DEBUG" {
			t.Fatalf("daily log must retain debug events: %v", records)
		}
	}
}

func TestConsoleDetailSelection(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var console bytes.Buffer
		j := testJournal(t, &console, agent)
		j.event(slog.LevelInfo, "Recorder started", "pid", 123, "bitrate", 32000, "complexity", 2, "sync_interval", "5s", "microphone_preferences", []string{"*"}, "output_directory", "/recordings")
		fields := []any{"microphone_id", 126, "microphone_uid", "desk-uid", "microphone_name", "Desk mic", "microphone_transport", "USB", "microphone_type", "Microphone", "microphone_manufacturer", "Example Audio", "microphone_model", "Desk headset", "sample_rate", 16000, "playback", "system", "output_directory", "/recordings"}
		for _, message := range []string{"Microphone available", "Capture device opened", "Microphone disconnected"} {
			j.event(slog.LevelInfo, message, fields...)
		}
		j.event(slog.LevelWarn, "Audio capture unavailable; retrying", "error", errors.New("device unavailable"), "retry_in", "2s")
		if agent {
			lines := bytes.Split(bytes.TrimSpace(console.Bytes()), []byte("\n"))
			if len(lines) != 5 {
				t.Fatalf("agent console lost diagnostic events: %s", &console)
			}
			var event map[string]any
			if err := json.Unmarshal(lines[1], &event); err != nil {
				t.Fatal(err)
			}
			if event["microphone_id"] != float64(126) || event["microphone_uid"] != "desk-uid" || event["microphone_manufacturer"] != "Example Audio" || event["sample_rate"] != float64(16000) {
				t.Fatalf("agent console lost microphone details: %v", event)
			}
		} else {
			for _, unwanted := range []string{"Microphone available", "microphone_id=", "microphone_uid=", "manufacturer=", "microphone_type=", "microphone_model=", "sample_rate=", "pid=", "bitrate=", "complexity=", "sync_interval=", "microphone_preferences="} {
				if strings.Contains(console.String(), unwanted) {
					t.Fatalf("human console contains diagnostic-only information %q: %s", unwanted, &console)
				}
			}
			for _, want := range []string{`microphone="Desk mic"`, "connection=USB", "output=/recordings", `playback="all applications"`, `error="device unavailable"`, "retry_in=2s", "Microphone disconnected"} {
				if !strings.Contains(console.String(), want) {
					t.Fatalf("human console missing useful detail %q: %s", want, &console)
				}
			}
		}
		records := journalRecords(t, j)
		if len(records) != 5 || records[0]["pid"] != float64(123) || records[1]["microphone_uid"] != "desk-uid" || records[1]["microphone_manufacturer"] != "Example Audio" || records[1]["sample_rate"] != float64(16000) {
			t.Fatalf("console filtering changed daily diagnostic records: %v", records)
		}
	}
}

func TestJournalConsoleFallback(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var console bytes.Buffer
		j := testJournal(t, &console, agent)
		// A real file blocks creation of the calendar directory beneath the store.
		root := testdir.New(t)
		if err := os.WriteFile(filepath.Join(root, time.Now().Format("2006")), []byte("blocked directory"), 0600); err != nil {
			t.Fatal(err)
		}
		s, err := storage.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
		j.store = s
		j.event(slog.LevelWarn, "capture unavailable", "microphone_name", "USB Microphone")
		if agent {
			if strings.Count(console.String(), "\n") != 1 || !strings.Contains(console.String(), "log_error") {
				t.Fatalf("failed daily log must produce one JSON console event: %q", console.String())
			}
		} else if strings.Count(console.String(), " WRN ") != 1 || !strings.Contains(console.String(), "log_error=") || strings.Count(console.String(), "\n") != 1 {
			t.Fatalf("failed daily log must produce one readable console event: %q", console.String())
		}
		if agent && !json.Valid(bytes.TrimSpace(console.Bytes())) {
			t.Fatalf("agent fallback must remain JSON: %q", console.String())
		}
	}
}

func TestHumanConsoleEscaping(t *testing.T) {
	var console bytes.Buffer
	j := testJournal(t, &console, false)
	message := "Capture failed\n[INFO] forged event"
	name := "USB\x1b[2J\nMicrophone"
	j.event(slog.LevelWarn, message, "microphone_name", name, "microphone_model", "", "error", errors.New("first failure\nsecond failure"))
	if strings.Count(console.String(), "\n") != 1 || !strings.Contains(console.String(), `Capture failed\n[INFO] forged event`) || strings.Contains(console.String(), "\x1b") || !strings.Contains(console.String(), "Microphone") || strings.Contains(console.String(), "microphone_model=") {
		t.Fatalf("control characters must be escaped and empty details omitted: %q", console.String())
	}
	records := journalRecords(t, j)
	if records[0]["msg"] != message || records[0]["microphone_name"] != name || records[0]["microphone_model"] != "" {
		t.Fatalf("human formatting altered diagnostic values: %v", records)
	}
}

func TestJournalClosedConsole(t *testing.T) {
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
	j := testJournal(t, w, false)
	j.event(slog.LevelInfo, "capture continues", "input_frames", 960)
	if records := journalRecords(t, j); len(records) != 1 || records[0]["input_frames"] != float64(960) {
		t.Fatalf("console failure interrupted daily logging: %v", records)
	}
}

func testJournal(t *testing.T, stderr io.Writer, agent bool) *journal {
	t.Helper()
	s, err := storage.Open(testdir.New(t))
	if err != nil {
		t.Fatal(err)
	}
	j := &journal{store: s, stderr: stderr, agent: agent}
	t.Cleanup(func() {
		if err := errors.Join(j.close(), s.Close()); err != nil {
			t.Error(err)
		}
	})
	return j
}

func journalRecords(t *testing.T, j *journal) []map[string]any {
	t.Helper()
	if j.file == nil {
		t.Fatal("daily log was not opened")
	}
	b, err := os.ReadFile(j.file.Name())
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}
