// Package capture owns native Core Audio microphone and playback capture.
package capture

/*
#cgo darwin CFLAGS: -fblocks -mmacosx-version-min=14.2
#cgo darwin LDFLAGS: -framework CoreAudio -framework Foundation -framework AVFoundation
#include <CoreAudio/CoreAudio.h>
#include <CoreAudio/HostTime.h>
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"time"
	"unsafe"
)

// MaxFrames is the capacity required by Read for one native capture block.
const MaxFrames = int(C.AR_MAX_FRAMES)

const SampleTimeValid = uint32(C.kAudioTimeStampSampleTimeValid)

const clockTolerance = time.Second

// ErrCleanup requires process restart because native callbacks may still own memory.
var ErrCleanup = errors.New("native capture cleanup failed; process restart required")

// Capture is owned by one goroutine. Native callbacks access only native memory.
type Capture struct {
	native  *C.ARCapture
	Rate    int
	MicID   uint32
	MicName string
	anchor  time.Time
	host    int64
}

// Chunk carries the acquisition timestamp, not the time Go received the data.
type Chunk struct {
	Frames     int
	HostTime   uint64
	SampleTime float64
	TimeFlags  uint32
	At         time.Time
}

// Open requests microphone permission and starts the private aggregate.
func Open() (*Capture, error) {
	var message [512]C.char
	var cleanup C.int32_t
	native := C.ar_capture_open(&message[0], C.size_t(len(message)), &cleanup)
	if native == nil {
		if cleanup != 0 {
			return nil, fmt.Errorf("open audio capture: %s: %w", C.GoString(&message[0]), ErrCleanup)
		}
		return nil, fmt.Errorf("open audio capture: %s", C.GoString(&message[0]))
	}
	anchor, host := hostSnapshot()
	var name string
	if value := C.ar_capture_microphone_name(native); value != nil {
		name = C.GoString(value)
		C.free(unsafe.Pointer(value))
	}
	return &Capture{native: native, Rate: int(C.ar_capture_rate(native)), MicID: uint32(C.ar_capture_microphone(native)), MicName: name, anchor: anchor, host: host}, nil
}

// RecalibrateClock follows wall-clock changes without assigning queued blocks
// their arrival time. The owner must call it periodically during capture.
func (c *Capture) RecalibrateClock() time.Duration {
	return c.reanchor(hostSnapshot())
}

func hostSnapshot() (time.Time, int64) {
	before := time.Now()
	host := int64(C.AudioConvertHostTimeToNanos(C.AudioGetCurrentHostTime()))
	after := time.Now()
	return before.Add(after.Sub(before) / 2), host
}

func (c *Capture) reanchor(now time.Time, host int64) time.Duration {
	// Strip Go monotonic readings: wall time, including clock corrections, is
	// what determines the calendar path. Native host time still measures PCM.
	shift := now.Round(0).Sub(c.anchor.Add(time.Duration(host - c.host)).Round(0))
	if shift <= clockTolerance && shift >= -clockTolerance {
		return 0
	}
	c.anchor, c.host = now, host
	return shift
}

// Read copies the next block into caller-owned memory. Zero frames means empty.
func (c *Capture) Read(pcm []float32) (Chunk, error) {
	if c.native == nil || len(pcm) < MaxFrames {
		return Chunk{}, errors.New("capture is closed or PCM buffer is too small")
	}
	var chunk C.ARChunk
	if result := C.ar_capture_read(c.native, (*C.float)(unsafe.Pointer(&pcm[0])), C.uint32_t(len(pcm)), &chunk); result < 0 {
		return Chunk{}, errors.New("capture block exceeds PCM buffer capacity")
	}
	if chunk.frames == 0 {
		return Chunk{}, nil
	}
	if chunk.time_flags&C.kAudioTimeStampHostTimeValid == 0 {
		return Chunk{}, errors.New("capture block has no acquisition host timestamp")
	}
	host := int64(C.AudioConvertHostTimeToNanos(C.UInt64(chunk.host_time)))
	return Chunk{Frames: int(chunk.frames), HostTime: uint64(chunk.host_time), SampleTime: float64(chunk.sample_time), TimeFlags: uint32(chunk.time_flags), At: c.anchor.Add(time.Duration(host - c.host))}, nil
}

// Stop halts callbacks but keeps queued samples available for final draining.
func (c *Capture) Stop() error {
	if c.native == nil {
		return nil
	}
	if status := C.ar_capture_stop(c.native); status != 0 {
		return fmt.Errorf("stop audio capture: Core Audio status %d", int(status))
	}
	return nil
}

// Health reports total overflow frames and any device/format change.
func (c *Capture) Health() (uint64, bool) {
	return uint64(C.ar_capture_dropped(c.native)), C.ar_capture_changed(c.native) != 0
}

// Close stops capture and releases its native objects.
func (c *Capture) Close() error {
	if c.native == nil {
		return nil
	}
	status := C.ar_capture_close(c.native)
	c.native = nil
	if status != 0 {
		return fmt.Errorf("close audio capture: Core Audio status %d: %w", int(status), ErrCleanup)
	}
	return nil
}
