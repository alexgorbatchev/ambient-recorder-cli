package recording

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const commandTimeout = 10 * time.Minute
const commandWaitDelay = time.Second

type commandResult struct {
	path string
	err  error
}

// commands owns one serial worker. Only the recorder writes its journal;
// completion results return to that goroutine through the protected queue.
type commands struct {
	mu      sync.Mutex
	wake    *sync.Cond
	args    []string
	jobs    []string
	results []commandResult
	closing bool
	done    chan struct{}
}

func newCommands(args []string) *commands {
	if len(args) == 0 {
		return nil
	}
	c := &commands{args: append([]string(nil), args...), done: make(chan struct{})}
	c.wake = sync.NewCond(&c.mu)
	go c.work()
	return c
}

func (c *commands) enqueue(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs = append(c.jobs, path)
	c.wake.Signal()
}

func (c *commands) next() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.jobs) == 0 && !c.closing {
		c.wake.Wait()
	}
	if len(c.jobs) == 0 {
		return "", false
	}
	path := c.jobs[0]
	c.jobs[0] = ""
	c.jobs = c.jobs[1:]
	return path, true
}

func (c *commands) work() {
	defer close(c.done)
	for {
		path, ok := c.next()
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err := runFileCommand(ctx, c.args, path)
		cancel()
		c.mu.Lock()
		c.results = append(c.results, commandResult{path: path, err: err})
		c.mu.Unlock()
	}
}

func (c *commands) report(j *journal) {
	if c == nil {
		return
	}
	c.mu.Lock()
	results := c.results
	c.results = nil
	c.mu.Unlock()
	for _, result := range results {
		level, message := slog.LevelInfo, "Recording command completed"
		if result.err != nil {
			level, message = slog.LevelWarn, "Recording command failed"
		}
		j.event(level, message, "path", result.path, "error", result.err)
	}
}

func (r *runner) closeCommands() {
	c := r.commands
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closing = true
	c.wake.Signal()
	c.mu.Unlock()
	tick := time.NewTicker(clockInterval)
	defer tick.Stop()
	for {
		select {
		case <-c.done:
			c.report(r.journal)
			return
		case <-tick.C:
			r.pulse()
		}
	}
}

func runFileCommand(ctx context.Context, args []string, path string) (err error) {
	stem := strings.TrimSuffix(path, ".opus")
	f, err := os.OpenFile(stem+".command.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open command log: %w", err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	replace := strings.NewReplacer("{file}", path, "{stem}", stem)
	expanded := make([]string, len(args))
	for i, arg := range args {
		expanded[i] = replace.Replace(arg)
	}
	cmd := exec.CommandContext(ctx, expanded[0], expanded[1:]...)
	cmd.Stdout, cmd.Stderr = f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Shell scripts can start children; timeout must terminate their group too.
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = commandWaitDelay
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run recording command (see %s.command.log): %w", stem, errors.Join(err, ctx.Err()))
	}
	return nil
}
