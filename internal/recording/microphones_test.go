package recording

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/microphone"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestMicrophoneConnectionJournal(t *testing.T) {
	s, err := storage.Open(testdir.New(t))
	if err != nil {
		t.Fatal(err)
	}
	j := &journal{store: s}
	t.Cleanup(func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	r := &runner{journal: j, aggregateID: 99}
	builtIn := capture.Device{ID: 1, UID: "mac", Name: "MacBook Pro Microphone", Default: true}
	headset := capture.Device{ID: 2, UID: "headset", Name: "BM202", Transport: "Bluetooth"}
	r.updateMicrophones([]capture.Device{builtIn})
	r.updateMicrophones([]capture.Device{builtIn, headset})
	r.updateMicrophones([]capture.Device{builtIn, headset, {ID: 99, UID: "private", Name: "Ambient Recorder Capture"}})
	if len(r.microphones) != 2 {
		t.Fatal("recorder's own aggregate became a microphone candidate")
	}
	r.updateMicrophones([]capture.Device{builtIn, headset})
	r.updateMicrophones([]capture.Device{builtIn})
	if err := j.sync(); err != nil {
		t.Fatal(err)
	}
	file, err := s.OpenLog(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	var data bytes.Buffer
	if _, err := data.ReadFrom(file); err != nil {
		t.Fatal(err)
	}
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(data.String()), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["msg"] == "Microphone connected" || event["msg"] == "Microphone disconnected" {
			events = append(events, event)
		}
	}
	if len(events) != 2 || events[0]["msg"] != "Microphone connected" || events[1]["msg"] != "Microphone disconnected" {
		t.Fatalf("connection events=%v", events)
	}
	for _, event := range events {
		if event["microphone_uid"] != "headset" || event["microphone_name"] != "BM202" || event["microphone_transport"] != "Bluetooth" {
			t.Fatalf("missing diagnostic identity: %v", event)
		}
	}
}

func TestFailedMicrophoneFallback(t *testing.T) {
	var diagnostic bytes.Buffer
	r := &runner{cfg: Config{Microphones: []string{"uid:headset", "*"}}, journal: &journal{stderr: &diagnostic}, microphones: []capture.Device{{ID: 1, UID: "mac", Default: true}, {ID: 2, UID: "headset"}}, microphoneFailures: map[string]time.Time{"headset": time.Now().Add(time.Minute)}}
	setPreferences(t, r)
	if got := r.preferredMicrophones(); len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("failed preferred microphone was not skipped: %v", got)
	}
	r.microphoneFailures["headset"] = time.Now().Add(-time.Second)
	if got := r.preferredMicrophones(); len(got) != 2 || got[0].ID != 2 {
		t.Fatalf("preferred microphone was not retried: %v", got)
	}
}

func TestCaptureTimeoutFallback(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		var diagnostic bytes.Buffer
		headset := capture.Device{ID: 2, UID: "headset", Name: "Bluetooth Headset", Default: true}
		r := &runner{
			cfg:     Config{Microphones: []string{"!Virtual*", "*"}},
			journal: testJournal(t, &diagnostic, false),
			microphones: []capture.Device{
				headset,
				{ID: 3, UID: "virtual", Name: "Virtual Meeting Input"},
				{ID: 1, UID: "mac", Name: "MacBook Pro Microphone"},
			},
		}
		setPreferences(t, r)
		lastFrame := time.Now()
		if stalled {
			lastFrame = lastFrame.Add(-captureTimeout - time.Second)
		}
		err := r.checkCaptureTimeout(headset, lastFrame)
		if !stalled {
			if err != nil || len(r.microphoneFailures) != 0 || r.preferredMicrophones()[0].ID != headset.ID || diagnostic.Len() != 0 {
				t.Fatalf("healthy microphone was penalized: error=%v failures=%v log=%s", err, r.microphoneFailures, &diagnostic)
			}
			continue
		}
		if !errors.Is(err, errCaptureStalled) {
			t.Fatalf("timeout must be classified for immediate fallback: %v", err)
		}
		if got := r.preferredMicrophones(); len(got) != 1 || got[0].ID != 1 {
			t.Fatalf("stalled default microphone remained eligible instead of falling back: %v", got)
		}
		until := r.microphoneFailures[microphoneKey(headset)]
		if remaining := time.Until(until); remaining <= 0 || remaining > microphoneRetryInterval {
			t.Fatalf("stalled microphone has invalid retry deadline: %v", until)
		}
		records := journalRecords(t, r.journal)
		if len(records) != 1 || records[0]["level"] != "WARN" || records[0]["microphone_uid"] != headset.UID || records[0]["error"] != errCaptureStalled.Error() || !strings.Contains(diagnostic.String(), "trying next preference") {
			t.Fatalf("timeout fallback lost device diagnostics: %v console=%s", records, &diagnostic)
		}
	}
}

func TestExcludedMicrophoneFallback(t *testing.T) {
	r := &runner{
		cfg: Config{Microphones: []string{"LG Ultra*", "*", "!Virtual*"}},
		microphones: []capture.Device{
			{ID: 1, UID: "virtual", Name: "Virtual Meeting Input", Default: true},
			{ID: 2, UID: "display", Name: "LG UltraFine Display Audio"},
			{ID: 3, UID: "mac", Name: "MacBook Pro Microphone"},
		},
		microphoneFailures: map[string]time.Time{"display": time.Now().Add(time.Minute)},
	}
	setPreferences(t, r)
	if got := r.preferredMicrophones(); len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("fallback selected an excluded or failed device: %v", got)
	}
	r.microphoneFailures["display"] = time.Now().Add(-time.Second)
	if got := r.preferredMicrophones(); len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("preferred pattern did not recover with exclusions retained: %v", got)
	}
	r.cfg.Microphones = []string{"!*"}
	setPreferences(t, r)
	if got := r.preferredMicrophones(); len(got) != 0 {
		t.Fatalf("excluded devices were reintroduced as fallback: %v", got)
	}
}

func setPreferences(t *testing.T, r *runner) {
	t.Helper()
	preferences, err := microphone.Parse(r.cfg.Microphones)
	if err != nil {
		t.Fatal(err)
	}
	r.preferences = preferences
}
