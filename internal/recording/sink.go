package recording

import (
	"errors"
	"log/slog"
	"os"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/opus"
)

// sink owns one mono source encoder. The track group owns rotation and reporting.
type sink struct {
	file    *os.File
	encoder *opus.Encoder
	frames  uint64
	source  string
	journal *journal
}

func (s *sink) write(pcm []float32) error {
	if err := s.encoder.Encode(pcm); err != nil {
		return err
	}
	s.frames += uint64(len(pcm))
	return nil
}

func (s *sink) silence(frames uint64) error {
	const blockFrames = 4800
	var silence [blockFrames]float32
	for frames > 0 {
		n := min(frames, blockFrames)
		if err := s.write(silence[:n]); err != nil {
			return err
		}
		frames -= n
	}
	return nil
}

func (s *sink) close() error {
	var err error
	if s.encoder != nil {
		err = s.encoder.Close()
	}
	err = errors.Join(err, s.file.Sync(), s.file.Close())
	s.journal.event(slog.LevelInfo, "Recording segment closed", "path", s.file.Name(), "source", s.source, "input_frames", s.frames, "error", err)
	s.encoder = nil
	return err
}
