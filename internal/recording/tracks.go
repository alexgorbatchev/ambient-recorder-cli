package recording

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/opus"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

// tracks owns synchronized mono files, keeping surviving encoders through capture
// handovers. An additional microphone is padded from this segment's common start.
type tracks struct {
	store                     *storage.Store
	journal                   *journal
	sinks                     map[string]*sink
	names                     []string
	started                   time.Time
	reserved                  time.Time
	hour                      string
	frames                    uint64
	rate, bitrate, complexity int
	currentFilesChanged       func([]string)
	commands                  *commands
}

func sourceNames(microphones int) []string {
	names := make([]string, microphones+1)
	for i := range microphones {
		names[i] = fmt.Sprintf("mic-%d", i+1)
	}
	names[microphones] = "playback"
	return names
}

func (t *tracks) write(at time.Time, pcm [][]float32) error {
	if len(pcm) < 2 {
		return errors.New("capture requires microphone channels and playback")
	}
	for _, channel := range pcm {
		if len(channel) != len(pcm[0]) {
			return errors.New("source channels have different frame counts")
		}
	}
	for offset := 0; offset < len(pcm[0]); {
		start := at.Add(frameDuration(int64(offset), t.rate))
		hour := start.Format("2006/01/02/15-0700")
		if len(t.sinks) == 0 || t.hour != hour {
			if err := t.close(); err != nil {
				return err
			}
			if err := t.open(start, len(pcm)-1); err != nil {
				return err
			}
		}
		if err := t.addMicrophones(len(pcm) - 1); err != nil {
			return err
		}
		remaining := time.Hour - time.Duration(start.Minute())*time.Minute - time.Duration(start.Second())*time.Second - time.Duration(start.Nanosecond())
		n := min(len(pcm[0])-offset, int((remaining.Nanoseconds()*int64(t.rate)+int64(time.Second)-1)/int64(time.Second)))
		for i, name := range t.names {
			s := t.sinks[name]
			if name == "playback" {
				i = len(pcm) - 1
			}
			if i >= len(pcm)-1 && name != "playback" {
				if err := s.silence(uint64(n)); err != nil {
					return err
				}
			} else if err := s.write(pcm[i][offset : offset+n]); err != nil {
				return err
			}
		}
		t.frames += uint64(n)
		offset += n
	}
	return nil
}

func (t *tracks) open(at time.Time, microphones int) error {
	names := sourceNames(microphones)
	group, err := t.store.CreateSegments(at, names)
	if err != nil {
		return err
	}
	t.sinks = make(map[string]*sink)
	t.names, t.started, t.reserved, t.hour, t.frames = names, at, group.At, at.Format("2006/01/02/15-0700"), 0
	for i, f := range group.Files {
		// Own all reserved descriptors before initializing encoders so every failure
		// can finalize and close the entire group.
		t.sinks[names[i]] = &sink{file: f, source: names[i], journal: t.journal}
	}
	for _, name := range names {
		s := t.sinks[name]
		s.encoder, err = opus.New(s.file, opus.Config{SampleRate: t.rate, Bitrate: t.bitrate, Complexity: t.complexity})
		if err != nil {
			return errors.Join(err, t.close())
		}
		t.journal.event(slog.LevelInfo, "Recording segment opened", "path", s.file.Name(), "source", name, "acquired_at", at, "sample_rate", t.rate, "channels", 1)
	}
	if err := t.publish(); err != nil {
		return errors.Join(err, t.close())
	}
	return nil
}

func (t *tracks) addMicrophones(count int) error {
	if count <= len(t.names)-1 {
		return nil
	}
	for i := len(t.names) - 1; i < count; i++ {
		name := fmt.Sprintf("mic-%d", i+1)
		f, err := t.store.CreateSegment(t.reserved, name)
		if err != nil {
			return err
		}
		s := &sink{file: f, source: name, journal: t.journal}
		t.sinks[name] = s
		t.names = append(t.names[:len(t.names)-1], name, "playback")
		s.encoder, err = opus.New(f, opus.Config{SampleRate: t.rate, Bitrate: t.bitrate, Complexity: t.complexity})
		if err != nil {
			return err
		}
		if err := s.silence(t.frames); err != nil {
			return err
		}
		t.journal.event(slog.LevelInfo, "Recording segment opened", "path", f.Name(), "source", name, "acquired_at", t.started, "sample_rate", t.rate, "channels", 1)
	}
	return t.publish()
}

func (t *tracks) paths() []string {
	paths := make([]string, 0, len(t.names))
	for _, name := range t.names {
		paths = append(paths, t.sinks[name].file.Name())
	}
	return paths
}

func (t *tracks) publish() error {
	files := make([]*os.File, 0, len(t.names))
	for _, name := range t.names {
		files = append(files, t.sinks[name].file)
	}
	if err := t.store.MarkCurrent(files); err != nil {
		return err
	}
	if t.currentFilesChanged != nil {
		t.currentFilesChanged(t.paths())
	}
	return nil
}

func (t *tracks) sync() error {
	var err error
	for _, name := range t.names {
		err = errors.Join(err, t.sinks[name].file.Sync())
	}
	return err
}

func (t *tracks) close() error {
	if len(t.sinks) == 0 {
		return nil
	}
	if t.currentFilesChanged != nil {
		t.currentFilesChanged([]string{})
	}
	err := t.store.ClearCurrent()
	for _, name := range t.names {
		s := t.sinks[name]
		ready := s.encoder != nil && s.frames > 0
		closeErr := s.close()
		err = errors.Join(err, closeErr)
		if ready && closeErr == nil {
			t.commands.enqueue(s.file.Name())
		}
	}
	t.sinks, t.names = nil, nil
	return err
}
