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

func TestStartupAnnouncement(t *testing.T) {
	for _, agent := range []bool{false, true} {
		for _, name := range []string{"USB Microphone – Desk", ""} {
			var output bytes.Buffer
			path := filepath.Join(t.TempDir(), "meeting audio")
			r := &runner{cfg: Config{Output: path}, journal: &journal{stderr: &output, agent: agent}}
			r.announce(&capture.Capture{MicID: 126, MicName: name, Rate: 16000})
			got := output.String()
			for _, want := range []string{path, "126", "16000"} {
				if !strings.Contains(got, want) {
					t.Fatalf("startup output missing %q: %s", want, got)
				}
			}
			if name != "" && !strings.Contains(got, name) {
				t.Fatalf("startup output missing microphone name: %s", got)
			}
			if !agent && !strings.Contains(got, "Computer playback: all applications") {
				t.Fatalf("startup output missing playback source: %s", got)
			}
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
