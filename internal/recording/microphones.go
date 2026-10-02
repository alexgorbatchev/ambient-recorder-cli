package recording

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
)

const microphoneRefreshInterval = 5 * time.Second
const microphoneRetryInterval = 30 * time.Second

var errMicrophoneSwitch = errors.New("microphone selection changed")
var errCaptureStalled = errors.New("no capture frames received for five seconds")

func (r *runner) checkCaptureTimeout(d capture.Device, lastFrame time.Time) error {
	if time.Since(lastFrame) <= captureTimeout {
		return nil
	}
	r.microphoneFailed(d, errCaptureStalled)
	return errCaptureStalled
}

func (r *runner) microphoneFailed(d capture.Device, err error) {
	if r.microphoneFailures == nil {
		r.microphoneFailures = make(map[string]time.Time)
	}
	r.microphoneFailures[microphoneKey(d)] = time.Now().Add(microphoneRetryInterval)
	fields := append(microphoneFields(d), "error", err, "retry_in", microphoneRetryInterval.String())
	r.journal.event(slog.LevelWarn, "Microphone capture failed; trying next preference", fields...)
}

func microphoneKey(d capture.Device) string {
	if d.UID != "" {
		return d.UID
	}
	return fmt.Sprintf("id:%d", d.ID)
}

func microphoneFields(d capture.Device) []any {
	return []any{"microphone_id", d.ID, "microphone_uid", d.UID, "microphone_name", d.Name, "microphone_transport", d.Transport, "microphone_type", d.Type, "microphone_manufacturer", d.Manufacturer, "microphone_model", d.Model}
}

func (r *runner) refreshMicrophones(force bool) error {
	changed := r.monitor != nil && r.monitor.Changed()
	if !force && !changed && time.Since(r.lastMicrophoneRefresh) < microphoneRefreshInterval {
		return nil
	}
	r.lastMicrophoneRefresh = time.Now()
	devices, err := capture.Microphones()
	if err != nil {
		return err
	}
	r.updateMicrophones(devices)
	return nil
}

func (r *runner) updateMicrophones(devices []capture.Device) {
	filtered := make([]capture.Device, 0, len(devices))
	for _, d := range devices {
		if d.ID != r.aggregateID {
			filtered = append(filtered, d)
		}
	}
	devices = filtered
	previous, current := make(map[string]capture.Device), make(map[string]capture.Device)
	for _, d := range r.microphones {
		previous[microphoneKey(d)] = d
	}
	for _, d := range devices {
		current[microphoneKey(d)] = d
	}
	for _, d := range r.microphones {
		if next, ok := current[microphoneKey(d)]; !ok || next.ID != d.ID {
			r.journal.event(slog.LevelInfo, "Microphone disconnected", microphoneFields(d)...)
			delete(r.microphoneFailures, microphoneKey(d))
		}
	}
	for _, d := range devices {
		if old, ok := previous[microphoneKey(d)]; !ok || old.ID != d.ID {
			message := "Microphone connected"
			if !r.microphonesInitialized {
				message = "Microphone available"
			}
			r.journal.event(slog.LevelInfo, message, microphoneFields(d)...)
		}
	}
	r.microphones, r.microphonesInitialized = devices, true
}

func (r *runner) preferredMicrophones() []capture.Device {
	ordered := capture.Prefer(r.microphones, r.preferences)
	available := ordered[:0]
	for _, d := range ordered {
		if time.Now().Before(r.microphoneFailures[microphoneKey(d)]) {
			continue
		}
		available = append(available, d)
	}
	return available
}

func (r *runner) openMicrophone() (*capture.Capture, error) {
	if err := r.refreshMicrophones(true); err != nil {
		return nil, err
	}
	var failures []error
	for _, d := range r.preferredMicrophones() {
		c, err := capture.Open(d.ID)
		if err == nil {
			return c, nil
		}
		if errors.Is(err, capture.ErrCleanup) {
			return nil, err
		}
		r.microphoneFailed(d, err)
		failures = append(failures, err)
	}
	return nil, errors.Join(append(failures, errors.New("no microphone currently available for capture"))...)
}

func (r *runner) microphoneSelectionChanged(c *capture.Capture) bool {
	if err := r.refreshMicrophones(false); err != nil {
		r.journal.event(slog.LevelWarn, "Microphone inventory refresh failed", "error", err)
		return false
	}
	available := r.preferredMicrophones()
	if len(available) > 0 && available[0].ID == c.Mic.ID {
		return false
	}
	fields := []any{"previous_microphone_id", c.Mic.ID, "previous_microphone_uid", c.Mic.UID, "previous_microphone_name", c.Mic.Name}
	if len(available) > 0 {
		fields = append(fields, microphoneFields(available[0])...)
	}
	r.journal.event(slog.LevelInfo, "Microphone preference switch requested", fields...)
	return true
}
