// Package opus streams mono Ogg Opus through statically linked libopusenc.
package opus

/*
#cgo darwin CFLAGS: -I${SRCDIR}/../../.tmp/native/include/opus -mmacosx-version-min=14.2
#cgo darwin LDFLAGS: ${SRCDIR}/../../.tmp/native/lib/libopusenc.a ${SRCDIR}/../../.tmp/native/lib/libopus.a -lm
#include <opusenc.h>

static int configure_encoder(OggOpusEnc *enc, int bitrate, int complexity) {
    int error;
    if ((error = ope_encoder_ctl(enc, OPUS_SET_APPLICATION(OPUS_APPLICATION_VOIP))) != OPE_OK) return error;
    if ((error = ope_encoder_ctl(enc, OPUS_SET_SIGNAL(OPUS_SIGNAL_VOICE))) != OPE_OK) return error;
    if ((error = ope_encoder_ctl(enc, OPUS_SET_BITRATE(bitrate))) != OPE_OK) return error;
    if ((error = ope_encoder_ctl(enc, OPUS_SET_COMPLEXITY(complexity))) != OPE_OK) return error;
    if ((error = ope_encoder_ctl(enc, OPUS_SET_DTX(0))) != OPE_OK) return error;
    if ((error = ope_encoder_ctl(enc, OPE_SET_DECISION_DELAY(0))) != OPE_OK) return error;
    return ope_encoder_ctl(enc, OPE_SET_MUXING_DELAY(4800));
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"unsafe"
)

// Config selects input rate and the speech encoder's quality/CPU tradeoff.
type Config struct {
	SampleRate int
	Bitrate    int
	Complexity int
}

// Encoder is owned by one goroutine. It does not close the output writer.
type Encoder struct {
	enc *C.OggOpusEnc
	w   io.Writer
}

// New writes container headers immediately. Non-48-kHz input is resampled by
// libopusenc's bundled resampler; its pre-skip and final trimming stay native.
func New(w io.Writer, cfg Config) (*Encoder, error) {
	if w == nil || cfg.SampleRate <= 0 || cfg.SampleRate > 192000 || cfg.Bitrate < 6000 || cfg.Bitrate > 128000 || cfg.Complexity < 0 || cfg.Complexity > 10 {
		return nil, errors.New("invalid mono speech encoder configuration")
	}
	comments := C.ope_comments_create()
	if comments == nil {
		return nil, errors.New("allocate Opus comments")
	}
	defer C.ope_comments_destroy(comments)
	var code C.int
	enc := C.ope_encoder_create_pull(comments, C.opus_int32(cfg.SampleRate), 1, 0, &code)
	if enc == nil {
		return nil, codecError("create encoder", code)
	}
	e := &Encoder{enc: enc, w: w}
	if code := C.configure_encoder(enc, C.int(cfg.Bitrate), C.int(cfg.Complexity)); code != C.OPE_OK {
		C.ope_encoder_destroy(enc)
		return nil, codecError("configure encoder", code)
	}
	if code := C.ope_encoder_flush_header(enc); code != C.OPE_OK {
		C.ope_encoder_destroy(enc)
		return nil, codecError("write Opus headers", code)
	}
	if err := e.pages(false); err != nil {
		C.ope_encoder_destroy(enc)
		return nil, err
	}
	return e, nil
}

// Encode consumes normalized mono PCM without retaining the input slice.
func (e *Encoder) Encode(pcm []float32) error {
	if e.enc == nil {
		return errors.New("Opus encoder is closed")
	}
	if len(pcm) == 0 {
		return nil
	}
	if code := C.ope_encoder_write_float(e.enc, (*C.float)(unsafe.Pointer(&pcm[0])), C.int(len(pcm))); code != C.OPE_OK {
		return codecError("encode audio", code)
	}
	return e.pages(false)
}

// Close drains encoder delay and writes the final trimmed EOS page.
func (e *Encoder) Close() error {
	if e.enc == nil {
		return nil
	}
	defer func() {
		C.ope_encoder_destroy(e.enc)
		e.enc = nil
	}()
	if code := C.ope_encoder_drain(e.enc); code != C.OPE_OK {
		return codecError("finish Opus stream", code)
	}
	return e.pages(true)
}

func (e *Encoder) pages(flush bool) error {
	var force C.int
	if flush {
		force = 1
	}
	for {
		var ptr *C.uchar
		var size C.opus_int32
		if C.ope_encoder_get_page(e.enc, &ptr, &size, force) == 0 {
			return nil
		}
		page := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))
		n, err := e.w.Write(page)
		if err != nil {
			return fmt.Errorf("write Ogg page: %w", err)
		}
		if n != len(page) {
			return fmt.Errorf("write Ogg page: %w", io.ErrShortWrite)
		}
	}
}

func codecError(action string, code C.int) error {
	return fmt.Errorf("%s: %s (%d)", action, C.GoString(C.ope_strerror(code)), int(code))
}
