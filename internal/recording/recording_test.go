package recording

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestStartupCaptureLog(t *testing.T) {
	for _, agent := range []bool{false, true} {
		for _, name := range []string{"USB Microphone – Desk", ""} {
			var output bytes.Buffer
			path := filepath.Join(testdir.New(t), "meeting audio")
			r := &runner{cfg: Config{Output: path}, journal: testJournal(t, &output, agent)}
			r.logCapture(&capture.Capture{Rate: 16000, Mic: capture.Device{ID: 126, Name: name, Transport: "Bluetooth", Type: "Headset microphone", Manufacturer: "Example Audio", Model: "Desk headset"}})
			got := output.String()
			if name != "" && !agent {
				t.Logf("human capture log:\n%s", got)
			}
			for _, want := range []string{path, "Bluetooth"} {
				if !strings.Contains(got, want) {
					t.Fatalf("startup output missing %q: %s", want, got)
				}
			}
			if name != "" && !strings.Contains(got, name) {
				t.Fatalf("startup output missing microphone name: %s", got)
			}
			if !strings.Contains(got, "Capture device opened") {
				t.Fatalf("capture startup must include its log message: %s", got)
			}
			if agent {
				if strings.Count(got, "\n") != 1 {
					t.Fatalf("agent capture event must remain one JSON line: %s", got)
				}
				var event map[string]any
				if err := json.Unmarshal([]byte(got), &event); err != nil {
					t.Fatal(err)
				}
				if event["level"] != "INFO" || event["playback"] != "system" || event["output_directory"] != path {
					t.Fatalf("startup log lost its level or sources: %v", event)
				}
			} else {
				for _, want := range []string{" INF Capture device opened ", "connection=Bluetooth", `playback="all applications"`, `output="` + path + `"`} {
					if !strings.Contains(got, want) {
						t.Fatalf("startup output missing readable detail %q: %s", want, got)
					}
				}
				if strings.Count(got, "\n") != 1 || strings.Contains(got, "\x1b") || strings.Contains(got, "time=") {
					t.Fatalf("redirected human output must be one plain tint event: %s", got)
				}
			}
			if records := journalRecords(t, r.journal); len(records) != 1 || records[0]["microphone_name"] != name || records[0]["output_directory"] != path || records[0]["playback"] != "system" {
				t.Fatalf("startup capture metadata missing from the daily log: %v", records)
			}
		}
	}
}

func TestStartupMissingDeviceDetails(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var output bytes.Buffer
		r := &runner{journal: testJournal(t, &output, agent)}
		r.logCapture(&capture.Capture{})
		got := output.String()
		records := journalRecords(t, r.journal)
		if len(records) != 1 || records[0]["microphone_manufacturer"] != "" || records[0]["microphone_model"] != "" || (!agent && (strings.Contains(got, "microphone_manufacturer=") || strings.Contains(got, "microphone_model="))) {
			t.Fatalf("missing metadata must remain empty in the capture log: %s records=%v", got, records)
		}
	}
}

func TestWatchdog(t *testing.T) {
	if os.Getenv("RECORDER_WATCHDOG_TEST") == "1" {
		r := &runner{progress: make(chan struct{}, 1)}
		r.watchdog(os.Stderr, 20*time.Millisecond)
		time.Sleep(time.Second)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWatchdog$")
	cmd.Env = append(os.Environ(), "RECORDER_WATCHDOG_TEST=1")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 || !bytes.Contains(out, []byte("exiting for supervisor restart")) {
		t.Fatalf("stalled recorder did not exit: %v\n%s", err, out)
	}
	r := &runner{progress: make(chan struct{}, 1)}
	stop := r.watchdog(os.Stderr, 20*time.Millisecond)
	stop()
	time.Sleep(40 * time.Millisecond)

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := errors.Join(read.Close(), write.Close()); err != nil {
			t.Error(err)
		}
	}()
	if err := write.SetWriteDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := write.Write(make([]byte, 1024*1024)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("fill stderr pipe: %v", err)
	}
	if err := write.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWatchdog$")
	cmd.Env = append(os.Environ(), "RECORDER_WATCHDOG_TEST=1")
	cmd.Stderr = write
	err = cmd.Run()
	exit, ok = err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("watchdog did not exit with blocked stderr: %v", err)
	}
}

func TestAcquisitionHourRotation(t *testing.T) {
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	j := &journal{store: s}
	out := &sink{store: s, journal: j, rate: 48000, bitrate: 32000, complexity: 2}
	start := time.Date(2026, 9, 30, 23, 59, 59, 990000000, time.UTC)
	if err := out.write(start, make([]float32, 960)); err != nil {
		t.Fatal(err)
	}
	if err := out.close(); err != nil {
		t.Fatal(err)
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"2026/09/30/23-59-59.990000000.opus", "2026/10/01/00-00-00.000000000.opus"} {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if got := opusDuration(t, b); got != 480 {
			t.Fatalf("%s duration=%d samples, want 480", path, got)
		}
	}
}

func TestDailyJournal(t *testing.T) {
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	j := &journal{store: s}
	start := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	for _, at := range []time.Time{start, start, start.Add(time.Minute)} {
		r := slog.NewRecord(at, slog.LevelDebug, "capture", 0)
		r.Add("frames", 960)
		if err := j.record(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{"2026/09/30/log.nljson": 2, "2026/10/01/log.nljson": 1} {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
		if len(lines) != want {
			t.Fatalf("%s contains %d records, want %d", path, len(lines), want)
		}
		for _, line := range lines {
			var event map[string]any
			if err := json.Unmarshal(line, &event); err != nil {
				t.Fatal(err)
			}
			if event["msg"] != "capture" || event["frames"] != float64(960) {
				t.Fatalf("event=%v", event)
			}
		}
	}
}

func TestJournalIncompleteTail(t *testing.T) {
	root := testdir.New(t)
	s, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	f, err := s.OpenLog(at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"msg\":\"preserve\"}\n{\"msg\":\"interrupted"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	j := &journal{store: s}
	if err := j.record(slog.NewRecord(at, slog.LevelInfo, "restart", 0)); err != nil {
		t.Fatal(err)
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "2026/09/30/log.nljson"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines=%d, want preserved event, recovered tail, restart", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("invalid log record: %s", line)
		}
	}
	if !strings.Contains(lines[1], "interrupted") {
		t.Fatal("incomplete tail was not retained in recovery event")
	}
}

func opusDuration(t *testing.T, b []byte) uint64 {
	t.Helper()
	var granule uint64
	var preSkip uint16
	var eos bool
	for len(b) >= 27 {
		header := 27 + int(b[26])
		if string(b[:4]) != "OggS" || len(b) < header {
			t.Fatal("invalid Ogg header")
		}
		size := header
		for _, n := range b[27:header] {
			size += int(n)
		}
		if len(b) < size {
			t.Fatal("partial Ogg page")
		}
		if size >= header+19 && string(b[header:header+8]) == "OpusHead" {
			preSkip = binary.LittleEndian.Uint16(b[header+10 : header+12])
		}
		granule = binary.LittleEndian.Uint64(b[6:14])
		eos = b[5]&4 != 0
		b = b[size:]
	}
	if len(b) != 0 || !eos || granule < uint64(preSkip) {
		t.Fatal("stream was not fully finalized")
	}
	return granule - uint64(preSkip)
}
