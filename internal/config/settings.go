// Package config manages recorder settings and XDG configuration paths.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/microphone"
)

const (
	DefaultBitrate      = 32000
	DefaultComplexity   = 2
	DefaultSyncInterval = 5 * time.Second
)

// Settings is the TOML document. Durations use Go's duration syntax, such as 5s.
type Settings struct {
	Input  Input  `toml:"input"`
	Output Output `toml:"output"`
}

type Input struct {
	Microphones []string `toml:"microphones"`
}

type Output struct {
	Dir          string `toml:"dir"`
	Bitrate      int    `toml:"bitrate"`
	Complexity   int    `toml:"complexity"`
	SyncInterval string `toml:"sync_interval"`
}

func Defaults() (Settings, error) {
	output, err := DefaultOutput()
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Input:  Input{Microphones: []string{"*"}},
		Output: Output{Dir: output, Bitrate: DefaultBitrate, Complexity: DefaultComplexity, SyncInterval: DefaultSyncInterval.String()},
	}, nil
}

func (s Settings) Validate() error {
	interval, err := time.ParseDuration(s.Output.SyncInterval)
	if err != nil || interval <= 0 {
		return fmt.Errorf("output.sync_interval must be a positive duration: %q", s.Output.SyncInterval)
	}
	if s.Output.Dir == "" || s.Output.Bitrate < 6000 || s.Output.Bitrate > 128000 || s.Output.Complexity < 0 || s.Output.Complexity > 10 {
		return errors.New("output.dir is required; output.bitrate must be 6000..128000 and output.complexity 0..10")
	}
	_, err = microphone.Parse(s.Input.Microphones)
	return err
}

func DefaultPath() (string, error) {
	base, err := xdgBase("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ambient-recorder", "config.toml"), nil
}

func DefaultOutput() (string, error) {
	base, err := xdgBase("XDG_DATA_HOME", ".local/share")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ambient-recorder"), nil
}

func xdgBase(variable, fallback string) (string, error) {
	if base := os.Getenv(variable); base != "" {
		if !filepath.IsAbs(base) {
			return "", fmt.Errorf("%s must be an absolute path: %q", variable, base)
		}
		return base, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user directory: %w", err)
	}
	return filepath.Join(home, fallback), nil
}
