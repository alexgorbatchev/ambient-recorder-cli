package recording

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/opus"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

type sink struct {
	store      *storage.Store
	journal    *journal
	file       *os.File
	encoder    *opus.Encoder
	hour       string
	frames     uint64
	rate       int
	bitrate    int
	complexity int
}

func (s *sink) write(at time.Time, pcm []float32) error {
	for offset := 0; offset < len(pcm); {
		start := at.Add(time.Duration(offset) * time.Second / time.Duration(s.rate))
		hour := start.Format("2006/01/02/15-0700")
		if s.encoder == nil || hour != s.hour {
			if err := s.close(); err != nil {
				return err
			}
			if err := s.open(start, hour); err != nil {
				return err
			}
		}
		remaining := time.Hour - time.Duration(start.Minute())*time.Minute - time.Duration(start.Second())*time.Second - time.Duration(start.Nanosecond())
		frames := min(len(pcm)-offset, int((remaining.Nanoseconds()*int64(s.rate)+int64(time.Second)-1)/int64(time.Second)))
		if err := s.encoder.Encode(pcm[offset : offset+frames]); err != nil {
			return err
		}
		s.frames += uint64(frames)
		offset += frames
	}
	return nil
}

func (s *sink) open(at time.Time, hour string) error {
	f, err := s.store.CreateSegment(at)
	if err != nil {
		return err
	}
	e, err := opus.New(f, opus.Config{SampleRate: s.rate, Bitrate: s.bitrate, Complexity: s.complexity})
	if err != nil {
		return errors.Join(err, f.Close())
	}
	if err := s.store.MarkCurrent(f); err != nil {
		return errors.Join(err, e.Close(), f.Close())
	}
	s.file, s.encoder, s.hour, s.frames = f, e, hour, 0
	s.journal.event(slog.LevelInfo, "Recording segment opened", "path", f.Name(), "acquired_at", at, "sample_rate", s.rate, "channels", 1)
	return nil
}

func (s *sink) sync() error {
	if s.file == nil {
		return nil
	}
	return s.file.Sync()
}

func (s *sink) close() error {
	if s.encoder == nil {
		return nil
	}
	err := errors.Join(s.store.ClearCurrent(), s.encoder.Close(), s.file.Sync(), s.file.Close())
	s.journal.event(slog.LevelInfo, "Recording segment closed", "path", s.file.Name(), "input_frames", s.frames, "error", err)
	s.encoder, s.file = nil, nil
	return err
}
