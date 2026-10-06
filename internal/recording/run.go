// Package recording maintains local microphone and playback recording sessions.
package recording

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/microphone"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/resample"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

const (
	pollInterval      = 20 * time.Millisecond
	initialRetry      = 250 * time.Millisecond
	maximumRetry      = 5 * time.Second
	captureTimeout    = 5 * time.Second
	operationTimeout  = 90 * time.Second
	statusInterval    = 30 * time.Second
	diagnosticTimeout = 100 * time.Millisecond
	clockInterval     = time.Second
)

// Config sets the output root and the speech encoding/durability tradeoffs.
type Config struct {
	Output       string
	Bitrate      int
	Complexity   int
	SyncInterval time.Duration
	Agent        bool
	Microphones  []string
	// CurrentFileChanged receives the absolute path on open and empty on close.
	// Run invokes it synchronously from the recording goroutine.
	CurrentFileChanged func(string)
}

type runner struct {
	store                  *storage.Store
	journal                *journal
	cfg                    Config
	progress               chan struct{}
	monitor                *capture.Monitor
	microphones            []capture.Device
	preferences            []microphone.Selector
	microphonesInitialized bool
	lastMicrophoneRefresh  time.Time
	microphoneFailures     map[string]time.Time
	aggregateID            uint32
	out                    *sink
}

// Run records until cancellation, retrying capture and storage faults. A native
// call or disk write blocked for 90 seconds exits with status 2 so launchd can
// relaunch the binary; it cannot reconstruct audio captured during a crash.
func Run(ctx context.Context, cfg Config, stderr io.Writer) (err error) {
	if cfg.Output == "" || cfg.Bitrate < 6000 || cfg.Bitrate > 128000 || cfg.Complexity < 0 || cfg.Complexity > 10 || cfg.SyncInterval <= 0 {
		return errors.New("output is required; bitrate must be 6000..128000, complexity 0..10, and sync interval positive")
	}
	preferences, err := microphone.Parse(cfg.Microphones)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	cfg.Output, err = filepath.Abs(cfg.Output)
	if err != nil {
		return fmt.Errorf("resolve recording directory: %w", err)
	}
	r := &runner{cfg: cfg, preferences: preferences, progress: make(chan struct{}, 1)}
	// Register first so the watchdog covers initialization and final deferred I/O.
	stopWatchdog := r.watchdog(stderr, operationTimeout)
	defer stopWatchdog()
	s, err := storage.Open(cfg.Output)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	lock, err := s.Lock()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Close()) }()
	j := &journal{store: s, stderr: stderr, agent: cfg.Agent}
	defer func() { err = errors.Join(err, j.close()) }()
	r.store, r.journal = s, j
	monitor, watchErr := capture.WatchMicrophones()
	if errors.Is(watchErr, capture.ErrCleanup) {
		return watchErr
	}
	if watchErr != nil {
		j.event(slog.LevelWarn, "Microphone notifications unavailable; polling inventory", "error", watchErr)
	} else {
		r.monitor = monitor
		defer func() { err = errors.Join(err, monitor.Close()) }()
	}
	r.out = &sink{store: s, journal: j, rate: resample.Rate, bitrate: cfg.Bitrate, complexity: cfg.Complexity, currentFileChanged: cfg.CurrentFileChanged}
	defer func() { err = errors.Join(err, r.out.close()) }()
	j.event(slog.LevelInfo, "Recorder started", "pid", os.Getpid(), "bitrate", cfg.Bitrate, "complexity", cfg.Complexity, "sync_interval", cfg.SyncInterval.String(), "microphone_preferences", cfg.Microphones)
	retry := initialRetry
	for ctx.Err() == nil {
		r.pulse()
		if err := r.session(ctx); err != nil {
			if errors.Is(err, capture.ErrCleanup) || ctx.Err() != nil {
				j.event(slog.LevelError, "Capture session ended with an error", "error", err)
				return err
			}
			if errors.Is(err, errMicrophoneSwitch) || errors.Is(err, errCaptureStalled) {
				retry = initialRetry
				continue
			}
			j.event(slog.LevelWarn, "Audio capture unavailable; retrying", "error", err, "retry_in", retry.String())
		} else {
			retry = initialRetry
		}
		if !r.wait(ctx, retry) {
			break
		}
		retry = min(maximumRetry, retry*2)
	}
	j.event(slog.LevelInfo, "Recorder stopped by cancellation")
	return nil
}

func (r *runner) session(ctx context.Context) (err error) {
	c, err := r.openMicrophone()
	if err != nil {
		return err
	}
	r.aggregateID = c.AggregateID()
	r.pulse()
	r.logCapture(c)
	source, err := r.newSource(c.Rate)
	if err != nil {
		r.aggregateID = 0
		return errors.Join(err, c.Close())
	}
	pcm := make([]float32, capture.MaxFrames)
	defer func() {
		stopErr := c.Stop()
		if stopErr == nil {
			for {
				chunk, readErr := c.Read(pcm)
				if readErr != nil || chunk.Frames == 0 {
					err = errors.Join(err, readErr)
					break
				}
				if writeErr := source.write(ctx, chunk.At, pcm[:chunk.Frames]); writeErr != nil {
					err = errors.Join(err, writeErr)
					break
				}
			}
		}
		err = errors.Join(err, stopErr, source.close(ctx), c.Close())
		r.aggregateID = 0
	}()
	return r.consume(ctx, c, source, pcm)
}

func (r *runner) logCapture(c *capture.Capture) {
	fields := append(microphoneFields(c.Mic), "sample_rate", c.Rate, "playback", "system", "output_directory", r.cfg.Output)
	r.journal.event(slog.LevelInfo, "Capture device opened", fields...)
}

func (r *runner) consume(ctx context.Context, c *capture.Capture, source *source, pcm []float32) error {
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	lastFrame, lastSync, lastStatus := time.Now(), time.Now(), time.Now()
	lastClock := time.Now()
	var previous capture.Chunk
	var frames, dropped uint64
	for ctx.Err() == nil {
		r.pulse()
		if time.Since(lastClock) >= clockInterval {
			if shift := c.RecalibrateClock(); shift != 0 {
				r.journal.event(slog.LevelWarn, "Capture wall clock recalibrated", "clock_shift", shift.String())
			}
			lastClock = time.Now()
			if r.microphoneSelectionChanged(c) {
				return errMicrophoneSwitch
			}
		}
		chunk, err := c.Read(pcm)
		if err != nil {
			return err
		}
		if chunk.Frames > 0 {
			lastFrame = time.Now()
			if chunk.TimeFlags&capture.SampleTimeValid != 0 && previous.TimeFlags&capture.SampleTimeValid != 0 {
				gap := chunk.SampleTime - previous.SampleTime - float64(previous.Frames)
				if math.Abs(gap) > 0.5 {
					r.journal.event(slog.LevelWarn, "Capture timeline discontinuity", "sample_gap", gap, "host_time", chunk.HostTime, "acquired_at", chunk.At)
				}
			}
			previous = chunk
			if err := source.write(ctx, chunk.At, pcm[:chunk.Frames]); err != nil {
				return err
			}
			frames += uint64(chunk.Frames)
		}
		lost, changed := c.Health()
		if lost != dropped {
			r.journal.event(slog.LevelWarn, "Capture buffer overflow", "dropped_frames", lost-dropped, "total_dropped_frames", lost)
			dropped = lost
		}
		if chunk.Frames == 0 && changed {
			return errors.New("microphone or capture format changed")
		}
		if err := r.checkCaptureTimeout(c.Mic, lastFrame); err != nil {
			return err
		}
		if time.Since(lastSync) >= r.cfg.SyncInterval {
			if err := errors.Join(r.out.sync(), r.journal.sync()); err != nil {
				r.journal.event(slog.LevelWarn, "Storage synchronization failed", "error", err)
			}
			lastSync = time.Now()
		}
		if time.Since(lastStatus) >= statusInterval {
			r.journal.event(slog.LevelDebug, "Capture progress", "input_frames", frames, "dropped_frames", dropped, "acquired_at", previous.At)
			lastStatus = time.Now()
		}
		if chunk.Frames == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-tick.C:
			}
		}
	}
	return nil
}

func (r *runner) persist(ctx context.Context, out *sink, at time.Time, pcm []float32) error {
	retry := initialRetry
	for {
		r.pulse()
		if err := out.write(at, pcm); err != nil {
			closeErr := out.close()
			r.journal.event(slog.LevelWarn, "Audio storage unavailable; replaying pending block into a new segment", "error", errors.Join(err, closeErr), "acquired_at", at, "replayed_frames", len(pcm), "retry_in", retry.String())
			if !r.wait(ctx, retry) {
				return err
			}
			retry = min(maximumRetry, retry*2)
			continue
		}
		return nil
	}
}

func (r *runner) pulse() {
	select {
	case r.progress <- struct{}{}:
	default:
	}
}

func (r *runner) wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		r.pulse()
		return true
	}
}

func (r *runner) watchdog(stderr io.Writer, timeout time.Duration) func() {
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-r.progress:
				timer.Reset(timeout)
			case <-timer.C:
				written := make(chan struct{})
				go func() {
					defer close(written)
					if _, err := fmt.Fprintf(stderr, "ERR: Recorder blocked for %s; exiting for supervisor restart\n", timeout); err != nil {
						// Best effort only; process exit is independent of diagnostics.
					}
				}()
				// A full stderr pipe must not block recovery. The process owns the
				// final writer goroutine and terminates it with the imminent exit.
				select {
				case <-written:
				case <-time.After(diagnosticTimeout):
				}
				os.Exit(2)
			}
		}
	}()
	return func() { close(done); <-stopped }
}
