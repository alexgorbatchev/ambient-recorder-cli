package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
)

type servicePaths struct {
	output, config, current, source string
}

func (a launchAgent) recordingPaths(ctx context.Context, installed, loaded bool, live serviceinfo.Info) (servicePaths, error) {
	p := servicePaths{output: "unavailable", config: "unavailable", current: "not running", source: "unavailable"}
	if loaded {
		p.current = "unavailable"
	}
	if live.OutputDir != "" {
		p.output, p.config, p.current, p.source = live.OutputDir, live.ConfigPath, live.CurrentFile, "live"
		if p.config == "" {
			p.config = "none (defaults)"
		}
		if p.current == "" {
			p.current = "none"
		}
		return p, nil
	}
	if installed {
		output, config, err := a.savedRecordingPaths(ctx)
		if err == nil {
			p.output, p.config, p.source = output, config, "saved"
		} else if ctx.Err() != nil {
			return p, ctx.Err()
		}
	}
	return p, nil
}

// Read the saved arguments with Apple's plist parser and the same Cobra command
// tree used at startup. Never load today's TOML to infer a live process's paths.
func (a launchAgent) savedRecordingPaths(ctx context.Context) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", "--", a.plist).Output()
	if err != nil {
		return "", "", fmt.Errorf("read saved service paths: %w", err)
	}
	var saved struct{ ProgramArguments []string }
	if err := json.Unmarshal(data, &saved); err != nil {
		return "", "", err
	}
	if len(saved.ProgramArguments) == 0 {
		return "", "", errors.New("service has no recording arguments")
	}
	root, err := newRootCommand()
	if err != nil {
		return "", "", err
	}
	cmd, args, err := root.Find(saved.ProgramArguments[1:])
	if err != nil || cmd.CommandPath() != "ambient-recorder recording start" {
		return "", "", errors.New("service does not invoke recording start")
	}
	if err := cmd.ParseFlags(args); err != nil {
		return "", "", err
	}
	if err := cmd.ValidateArgs(cmd.Flags().Args()); err != nil {
		return "", "", err
	}
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", "", err
	}
	config, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", "", err
	}
	if !filepath.IsAbs(output) || !filepath.IsAbs(config) {
		return "", "", errors.New("saved service paths must be absolute")
	}
	return output, config, nil
}
