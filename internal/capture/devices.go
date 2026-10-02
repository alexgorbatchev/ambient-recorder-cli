package capture

/*
#include <CoreAudio/CoreAudio.h>
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"cmp"
	"fmt"
	"slices"
	"unsafe"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/microphone"
)

// Device describes a currently available microphone. UID persists across reconnects;
// ID is a temporary Core Audio handle and must not be saved as a preference.
type Device struct {
	ID           uint32
	UID          string
	Name         string
	Transport    string
	Type         string
	Manufacturer string
	Model        string
	Default      bool
}

// Microphones lists live input devices without requesting capture permission.
func Microphones() ([]Device, error) {
	var ids *C.uint32_t
	var count, defaultInput C.uint32_t
	if status := C.ar_microphones(&ids, &count, &defaultInput); status != 0 {
		return nil, fmt.Errorf("list microphones: Core Audio status %d", int(status))
	}
	defer C.free(unsafe.Pointer(ids))
	devices := make([]Device, 0, int(count))
	for _, id := range unsafe.Slice(ids, int(count)) {
		d := describeDevice(uint32(id))
		d.Default = id == defaultInput
		devices = append(devices, d)
	}
	return Prefer(devices, nil), nil
}

func describeDevice(id uint32) Device {
	return Device{ID: id, UID: deviceString(id, C.kAudioDevicePropertyDeviceUID), Name: deviceString(id, C.kAudioObjectPropertyName), Manufacturer: deviceString(id, C.kAudioObjectPropertyManufacturer), Model: deviceString(id, C.kAudioObjectPropertyModelName), Transport: C.GoString(C.ar_device_transport(C.uint32_t(id))), Type: C.GoString(C.ar_device_type(C.uint32_t(id)))}
}

func deviceString(id, property uint32) string {
	if value := C.ar_device_string(C.uint32_t(id), C.uint32_t(property)); value != nil {
		text := C.GoString(value)
		C.free(unsafe.Pointer(value))
		return text
	}
	return ""
}

// Prefer applies global exclusions before ranking name patterns and exact UIDs.
// Unmatched eligible devices follow in default-first order as implicit fallback.
func Prefer(devices []Device, preferences []microphone.Selector) []Device {
	remaining := slices.DeleteFunc(slices.Clone(devices), func(d Device) bool {
		for _, selector := range preferences {
			if selector.Exclude && selector.Match(d.Name, d.UID) {
				return true
			}
		}
		return false
	})
	slices.SortFunc(remaining, func(a, b Device) int {
		if a.Default != b.Default {
			if a.Default {
				return -1
			}
			return 1
		}
		if n := cmp.Compare(a.UID, b.UID); n != 0 {
			return n
		}
		return cmp.Compare(a.ID, b.ID)
	})
	ordered := make([]Device, 0, len(devices))
	for _, selector := range preferences {
		if selector.Exclude {
			continue
		}
		for i := 0; i < len(remaining); {
			d := remaining[i]
			if selector.Match(d.Name, d.UID) {
				ordered = append(ordered, d)
				remaining = slices.Delete(remaining, i, i+1)
			} else {
				i++
			}
		}
	}
	return append(ordered, remaining...)
}

// Monitor observes device inventory and default-input changes. The recording
// goroutine owns it; native callbacks only update its atomic dirty flag.
type Monitor struct{ native *C.ARDeviceMonitor }

func WatchMicrophones() (*Monitor, error) {
	var status, cleanup C.int32_t
	native := C.ar_monitor_open(&status, &cleanup)
	if native == nil {
		if cleanup != 0 {
			return nil, fmt.Errorf("watch microphones: status %d: %w", int(status), ErrCleanup)
		}
		return nil, fmt.Errorf("watch microphones: Core Audio status %d", int(status))
	}
	return &Monitor{native: native}, nil
}

func (m *Monitor) Changed() bool { return m.native != nil && C.ar_monitor_changed(m.native) != 0 }

func (m *Monitor) Close() error {
	if m.native == nil {
		return nil
	}
	status := C.ar_monitor_close(m.native)
	m.native = nil
	if status != 0 {
		return fmt.Errorf("stop watching microphones: status %d: %w", int(status), ErrCleanup)
	}
	return nil
}
