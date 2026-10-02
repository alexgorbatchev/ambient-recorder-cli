// Package resample converts mono capture to a fixed-rate Opus input using Apple's
// streaming sample-rate converter outside the real-time capture callback.
package resample

/*
#cgo darwin CFLAGS: -fblocks -mmacosx-version-min=14.2
#cgo darwin LDFLAGS: -framework AudioToolbox
#include "native.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

const Rate = int(C.AR_RESAMPLE_RATE)
const capacity = int(C.AR_RESAMPLE_CAPACITY)

// Converter belongs to one capture session. Returned samples remain valid only
// until the next call. Native-owned input protects AudioConverter buffer lifetime.
type Converter struct {
	native   *C.ARResampler
	buffer   []float32
	output   []float32
	direct   bool
	finished bool
	closed   bool
}

func New(rate int) (*Converter, error) {
	if rate <= 0 {
		return nil, errors.New("sample rate must be positive")
	}
	c := &Converter{direct: rate == Rate}
	if c.direct {
		return c, nil
	}
	var status C.int32_t
	c.native = C.ar_resampler_open(C.double(rate), &status)
	if c.native == nil {
		return nil, fmt.Errorf("create sample-rate converter: AudioToolbox status %d", int(status))
	}
	c.buffer = make([]float32, capacity)
	return c, nil
}

func (c *Converter) Convert(pcm []float32) ([]float32, error) {
	if c.closed || c.finished {
		return nil, errors.New("sample-rate converter is closed or finished")
	}
	if len(pcm) > capacity {
		return nil, errors.New("sample-rate input exceeds buffer capacity")
	}
	if c.direct || len(pcm) == 0 {
		return pcm, nil
	}
	if status := C.ar_resampler_push(c.native, (*C.float)(unsafe.Pointer(&pcm[0])), C.uint32_t(len(pcm))); status != 0 {
		return nil, fmt.Errorf("supply sample-rate input: AudioToolbox status %d", int(status))
	}
	return c.read(false)
}

// Finish drains buffered look-ahead exactly once before native disposal.
func (c *Converter) Finish() ([]float32, error) {
	if c.closed || c.finished {
		return nil, errors.New("sample-rate converter is closed or finished")
	}
	c.finished = true
	if c.direct {
		return nil, nil
	}
	return c.read(true)
}

func (c *Converter) read(finish bool) ([]float32, error) {
	c.output = c.output[:0]
	var eof C.int
	if finish {
		eof = 1
	}
	for {
		n := C.uint32_t(len(c.buffer))
		status := C.ar_resampler_read(c.native, (*C.float)(unsafe.Pointer(&c.buffer[0])), &n, eof)
		if int(n) > len(c.buffer) {
			return nil, errors.New("sample-rate output exceeds buffer capacity")
		}
		c.output = append(c.output, c.buffer[:int(n)]...)
		if status == C.AR_RESAMPLE_NEED_INPUT {
			return c.output, nil
		}
		if status != 0 {
			return nil, fmt.Errorf("convert sample rate: AudioToolbox status %d", int(status))
		}
		if n == 0 {
			return c.output, nil
		}
	}
}

func (c *Converter) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if c.native == nil {
		return nil
	}
	status := C.ar_resampler_close(c.native)
	c.native = nil
	if status != 0 {
		return fmt.Errorf("dispose sample-rate converter: AudioToolbox status %d", int(status))
	}
	return nil
}
