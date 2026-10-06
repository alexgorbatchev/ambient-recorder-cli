package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
)

func withServiceInfo(version, configPath string, cfg recording.Config, run func(recording.Config) error) (err error) {
	path := os.Getenv(serviceinfo.Environment)
	if path == "" {
		return run(cfg)
	}
	output, err := filepath.Abs(cfg.Output)
	if err != nil {
		return fmt.Errorf("resolve service recording directory: %w", err)
	}
	s, err := serviceinfo.Listen(path, serviceinfo.Info{Version: version, OutputDir: output, ConfigPath: configPath})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	cfg.Output = output
	cfg.CurrentFileChanged = s.SetCurrentFile
	return run(cfg)
}
