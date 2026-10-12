package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/plist"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
)

func (a launchAgent) readPlist(ctx context.Context, value any) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", "--", a.plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("read service configuration: %s: %w", strings.TrimSpace(string(data)), errors.Join(err, ctx.Err()))
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode service configuration: %w", err)
	}
	return nil
}

// refresh keeps the installed recording options and launchd settings, updating
// the executable and live-status endpoint to match the CLI handling the request.
func (a launchAgent) refresh(ctx context.Context) (bool, error) {
	var fields map[string]any
	if err := a.readPlist(ctx, &fields); err != nil {
		return false, err
	}
	args, ok := fields["ProgramArguments"].([]any)
	if !ok || len(args) == 0 {
		return false, errors.New("service has no executable arguments; reinstall with service uninstall and service install")
	}
	env, ok := fields["EnvironmentVariables"].(map[string]any)
	if !ok {
		if _, exists := fields["EnvironmentVariables"]; exists {
			return false, errors.New("service environment must be a dictionary")
		}
		env = make(map[string]any)
	}
	socket := serviceinfo.Path(a.plist)
	changed := args[0] != a.binary || env[serviceinfo.Environment] != socket
	args[0], env[serviceinfo.Environment] = a.binary, socket
	fields["EnvironmentVariables"] = env
	if program, exists := fields["Program"]; exists && program != a.binary {
		fields["Program"] = a.binary
		changed = true
	}
	if !changed {
		return false, nil
	}
	data, err := plist.XML(fields)
	if err != nil {
		return false, err
	}
	if err := a.replacePlist(data); err != nil {
		return false, fmt.Errorf("update installed service: %w", err)
	}
	return true, nil
}

func (a launchAgent) replacePlist(data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(a.plist), ".ambient-recorder-*.plist")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(f.Name()); !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return err
	}
	return os.Rename(f.Name(), a.plist)
}
