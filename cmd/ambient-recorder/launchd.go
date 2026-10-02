package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// launchctl identifies an absent service with 113 (launchctl error 113).
const launchctlServiceAbsent = 113

type launchAgent struct{ plist, domain, target, control, label, binary string }

func userLaunchAgent() (launchAgent, error) {
	if os.Getuid() == 0 {
		return launchAgent{}, errors.New("run service commands as your logged-in user, without sudo")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return launchAgent{}, fmt.Errorf("resolve home directory: %w", err)
	}
	binary, err := recorderExecutable()
	if err != nil {
		return launchAgent{}, err
	}
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	return launchAgent{
		plist:  filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist"),
		domain: domain, target: domain + "/" + serviceLabel, control: "/bin/launchctl",
		label:  serviceLabel,
		binary: binary,
	}, nil
}

func (a launchAgent) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.control, args...)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("launchctl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), errors.Join(err, ctx.Err()))
	}
	return out, nil
}

func (a launchAgent) loaded(ctx context.Context) ([]byte, bool, error) {
	out, err := a.run(ctx, "print", a.target)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == launchctlServiceAbsent {
		return nil, false, nil
	}
	return out, err == nil, err
}

func (a launchAgent) installed() (bool, error) {
	info, err := os.Lstat(a.plist)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect service file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("service file must be a regular file: %s", a.plist)
	}
	return true, nil
}

func (a launchAgent) install(ctx context.Context, settings agentSettings) error {
	var plist bytes.Buffer
	settings.label = a.label
	settings.binary = a.binary
	if err := writeAgent(&plist, settings); err != nil {
		return err
	}
	for _, dir := range []string{filepath.Dir(a.plist), settings.output} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create service directory %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(a.plist, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("service already installed; use service restart, or service uninstall before reinstalling: %w", err)
	}
	if err != nil {
		return fmt.Errorf("create service file: %w", err)
	}
	_, writeErr := f.Write(plist.Bytes())
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return errors.Join(fmt.Errorf("write service file: %w", err), os.Remove(a.plist))
	}
	if err := a.start(ctx); err != nil {
		return fmt.Errorf("service file saved at %s; correct the error and retry service start: %w", a.plist, err)
	}
	return nil
}

func (a launchAgent) start(ctx context.Context) error {
	installed, err := a.installed()
	if err != nil {
		return err
	}
	if !installed {
		return errors.New("service is not installed; run service install first")
	}
	if _, err := a.run(ctx, "enable", a.target); err != nil {
		return err
	}
	_, loaded, err := a.loaded(ctx)
	if err != nil {
		return err
	}
	if !loaded {
		_, err := a.run(ctx, "bootstrap", a.domain, a.plist)
		return err
	}
	_, err = a.run(ctx, "kickstart", a.target)
	return err
}

func (a launchAgent) unload(ctx context.Context) error {
	_, loaded, err := a.loaded(ctx)
	if err != nil || !loaded {
		return err
	}
	// Removing the job sends SIGTERM; kickstart -k would kill the recording.
	_, err = a.run(ctx, "bootout", a.target)
	return err
}

func (a launchAgent) manage(ctx context.Context, action string) error {
	switch action {
	case "start":
		return a.start(ctx)
	case "restart":
		installed, err := a.installed()
		if err != nil {
			return err
		}
		if !installed {
			return errors.New("service is not installed; run service install first")
		}
		if err := a.unload(ctx); err != nil {
			return err
		}
		return a.start(ctx)
	case "stop", "uninstall":
		if _, err := a.run(ctx, "disable", a.target); err != nil {
			return err
		}
		if err := a.unload(ctx); err != nil {
			return err
		}
		if action == "uninstall" {
			if err := os.Remove(a.plist); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove service file: %w", err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown service action %q", action)
	}
}

func (a launchAgent) status(ctx context.Context, w io.Writer, details, agent bool) error {
	installed, err := a.installed()
	if err != nil {
		return err
	}
	out, loaded, err := a.loaded(ctx)
	if err != nil {
		return err
	}
	if agent {
		_, err = fmt.Fprintf(w, "installed=%t loaded=%t plist=%q\n", installed, loaded, a.plist)
	} else {
		_, err = fmt.Fprintf(w, "Installed: %t\nLoaded by macOS: %t\nService file: %s\n", installed, loaded, a.plist)
	}
	if err != nil || !details || !loaded {
		return err
	}
	// Native diagnostics are displayed verbatim; their text is not an API.
	_, err = w.Write(out)
	return err
}
