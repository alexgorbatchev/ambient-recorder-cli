package recording

import (
	"context"
	"errors"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
)

// source owns only the conversion state for one microphone capture session.
// The runner owns the shared sink across sessions, including capture failures.
type source struct {
	runner       *runner
	converter    *resample.Converter
	rate         int
	inputFrames  int64
	outputFrames int64
	anchor       time.Time
}

func (r *runner) newSource(rate int) (*source, error) {
	c, err := resample.New(rate)
	if err != nil {
		return nil, err
	}
	return &source{runner: r, converter: c, rate: rate}, nil
}

func (s *source) write(ctx context.Context, at time.Time, pcm []float32) error {
	converted, err := s.converter.Convert(pcm)
	if err != nil {
		return err
	}
	// Re-derive the anchor from acquisition time so dropped input and wall-clock
	// corrections remain visible. Converter look-ahead keeps its earlier position.
	s.anchor = at.Add(-frameDuration(s.inputFrames, s.rate))
	s.inputFrames += int64(len(pcm))
	return s.persist(ctx, converted)
}

func (s *source) persist(ctx context.Context, pcm []float32) error {
	if len(pcm) == 0 {
		return nil
	}
	at := s.anchor.Add(frameDuration(s.outputFrames, resample.Rate))
	if err := s.runner.persist(ctx, s.runner.out, at, pcm); err != nil {
		return err
	}
	s.outputFrames += int64(len(pcm))
	return nil
}

func (s *source) close(ctx context.Context) error {
	pcm, err := s.converter.Finish()
	if err == nil {
		err = s.persist(ctx, pcm)
	}
	return errors.Join(err, s.converter.Close())
}

func frameDuration(frames int64, rate int) time.Duration {
	// Divide before multiplying so frame counts can span uninterrupted capture
	// lasting months without overflowing an intermediate nanosecond value.
	return time.Duration(frames/int64(rate))*time.Second + time.Duration(frames%int64(rate))*time.Second/time.Duration(rate)
}
