package recording

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
)

// source owns mono conversion state for every channel in one capture session.
// The runner owns the shared track group across microphone handovers.
type source struct {
	runner       *runner
	converters   []*resample.Converter
	buffers      [][]float32
	rate         int
	inputFrames  int64
	outputFrames int64
	anchor       time.Time
}

func (r *runner) newSource(rate, microphones int) (*source, error) {
	if microphones < 1 {
		return nil, errors.New("capture has no microphone channels")
	}
	s := &source{runner: r, rate: rate}
	for range microphones + 1 {
		c, err := resample.New(rate)
		if err != nil {
			for _, previous := range s.converters {
				err = errors.Join(err, previous.Close())
			}
			return nil, err
		}
		s.converters = append(s.converters, c)
		s.buffers = append(s.buffers, make([]float32, capture.MaxFrames))
	}
	return s, nil
}

func (s *source) write(ctx context.Context, at time.Time, pcm []float32) error {
	channels := len(s.converters)
	if len(pcm)%channels != 0 || len(pcm)/channels > capture.MaxFrames {
		return errors.New("invalid interleaved capture block")
	}
	frames := len(pcm) / channels
	converted := make([][]float32, channels)
	for channel, c := range s.converters {
		samples := s.buffers[channel][:frames]
		for frame := range frames {
			samples[frame] = pcm[frame*channels+channel]
		}
		var err error
		converted[channel], err = c.Convert(samples)
		if err != nil {
			return err
		}
	}
	// Derive every source's position from the same acquisition clock.
	s.anchor = at.Add(-frameDuration(s.inputFrames, s.rate))
	s.inputFrames += int64(frames)
	return s.persist(ctx, converted)
}

func (s *source) persist(ctx context.Context, pcm [][]float32) error {
	for _, samples := range pcm {
		if len(samples) != len(pcm[0]) {
			return fmt.Errorf("sample-rate converters returned unaligned source frames")
		}
	}
	if len(pcm[0]) == 0 {
		return nil
	}
	at := s.anchor.Add(frameDuration(s.outputFrames, resample.Rate))
	if err := s.runner.persist(ctx, s.runner.out, at, pcm); err != nil {
		return err
	}
	s.outputFrames += int64(len(pcm[0]))
	return nil
}

func (s *source) close(ctx context.Context) error {
	pcm := make([][]float32, len(s.converters))
	var failure error
	for i, c := range s.converters {
		samples, err := c.Finish()
		pcm[i] = samples
		failure = errors.Join(failure, err)
	}
	if failure == nil {
		failure = s.persist(ctx, pcm)
	}
	for _, c := range s.converters {
		failure = errors.Join(failure, c.Close())
	}
	return failure
}

func frameDuration(frames int64, rate int) time.Duration {
	// Divide before multiplying so long-running captures cannot overflow.
	return time.Duration(frames/int64(rate))*time.Second + time.Duration(frames%int64(rate))*time.Second/time.Duration(rate)
}
