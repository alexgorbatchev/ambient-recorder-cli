package recording

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
)

const maxLogTail = 64 * 1024

type journal struct {
	store   *storage.Store
	file    *os.File
	day     string
	handler *slog.JSONHandler
	stderr  io.Writer
	agent   bool
}

func (j *journal) record(r slog.Record) error {
	day := r.Time.Format("2006/01/02")
	var closeErr error
	if j.file == nil || day != j.day {
		closeErr = j.close()
		f, err := j.store.OpenLog(r.Time)
		if err != nil {
			return errors.Join(closeErr, err)
		}
		j.file, j.day = f, day
		j.handler = slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
		if err := j.repairTail(r.Time); err != nil {
			return errors.Join(closeErr, err, j.close())
		}
	}
	return errors.Join(closeErr, j.handler.Handle(context.Background(), r))
}

func (j *journal) repairTail(at time.Time) error {
	info, err := j.file.Stat()
	if err != nil || info.Size() == 0 {
		return err
	}
	size := info.Size()
	start := max(int64(0), size-maxLogTail)
	b := make([]byte, size-start)
	if _, err := j.file.ReadAt(b, start); err != nil {
		return fmt.Errorf("read diagnostic log tail: %w", err)
	}
	if b[len(b)-1] == '\n' {
		return nil
	}
	last := bytes.LastIndexByte(b, '\n')
	if last < 0 && start != 0 {
		return errors.New("incomplete diagnostic log tail exceeds 64 KiB; preserving file and using stderr")
	}
	partial := b[last+1:]
	if json.Valid(partial) {
		_, err := j.file.WriteString("\n")
		return err
	}
	if err := j.file.Truncate(start + int64(last+1)); err != nil {
		return fmt.Errorf("recover incomplete diagnostic record: %w", err)
	}
	r := slog.NewRecord(at, slog.LevelWarn, "Recovered incomplete diagnostic record", 0)
	r.Add("partial_tail", string(partial))
	if err := j.handler.Handle(context.Background(), r); err != nil {
		return fmt.Errorf("preserve interrupted log tail %q: %w", partial, err)
	}
	return j.file.Sync()
}

func (j *journal) event(level slog.Level, message string, args ...any) {
	r := slog.NewRecord(time.Now(), level, message, 0)
	r.Add(args...)
	if err := j.record(r); err != nil {
		r.Add("log_error", err)
		if j.stderr != nil {
			if fallbackErr := slog.NewJSONHandler(j.stderr, nil).Handle(context.Background(), r); fallbackErr != nil {
				// Both diagnostic sinks failed. Audio capture continues independently.
				return
			}
		}
	}
	if level >= slog.LevelWarn && j.stderr != nil {
		prefix := "[WARN]"
		if j.agent {
			prefix = "WARN:"
		}
		if _, err := fmt.Fprintf(j.stderr, "%s %s", prefix, message); err != nil {
			return // best-effort console diagnostic after writing the daily log
		}
		r.Attrs(func(a slog.Attr) bool {
			if j.agent || a.Key == "error" {
				if _, err := fmt.Fprintf(j.stderr, " %s=%v", a.Key, a.Value.Any()); err != nil {
					return false
				}
			}
			return true
		})
		if _, err := fmt.Fprintln(j.stderr); err != nil {
			return // best-effort console diagnostic after writing the daily log
		}
	}
}

func (j *journal) sync() error {
	if j.file == nil {
		return nil
	}
	return j.file.Sync()
}

func (j *journal) close() error {
	if j.file == nil {
		return nil
	}
	err := errors.Join(j.file.Sync(), j.file.Close())
	j.file, j.handler = nil, nil
	return err
}
