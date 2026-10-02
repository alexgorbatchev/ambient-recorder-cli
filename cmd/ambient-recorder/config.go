package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/config"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

func newConfigInitCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use: "init", Short: "Create a default TOML configuration without replacing an existing file", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := *configPath
			if path == "" {
				var err error
				path, err = config.DefaultPath()
				if err != nil {
					return err
				}
			}
			path, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve configuration path: %w", err)
			}
			if err := config.Create(path); err != nil {
				return err
			}
			if cobrahelptree.IsAgentMode() {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "config=%s\n", path)
			} else {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Configuration created: %s\n", path)
			}
			return err
		},
	}
}

func recordingConfig(cmd *cobra.Command, path string) (recording.Config, string, error) {
	cfg, loaded, err := config.Load(path)
	if err != nil {
		return recording.Config{}, "", err
	}
	flags := cmd.Flags()
	var failures []error
	if flags.Changed("output") {
		cfg.Output.Dir, err = flags.GetString("output")
		failures = append(failures, err)
	}
	if flags.Changed("bitrate") {
		cfg.Output.Bitrate, err = flags.GetInt("bitrate")
		failures = append(failures, err)
	}
	if flags.Changed("complexity") {
		cfg.Output.Complexity, err = flags.GetInt("complexity")
		failures = append(failures, err)
	}
	if flags.Changed("sync-interval") {
		var interval time.Duration
		interval, err = flags.GetDuration("sync-interval")
		failures = append(failures, err)
		cfg.Output.SyncInterval = interval.String()
	}
	if err := errors.Join(failures...); err != nil {
		return recording.Config{}, "", fmt.Errorf("read recording options: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return recording.Config{}, "", err
	}
	interval, err := time.ParseDuration(cfg.Output.SyncInterval)
	if err != nil {
		return recording.Config{}, "", err
	}
	return recording.Config{
		Output: cfg.Output.Dir, Bitrate: cfg.Output.Bitrate, Complexity: cfg.Output.Complexity,
		SyncInterval: interval, Microphones: cfg.Input.Microphones, Agent: cobrahelptree.IsAgentMode(),
	}, loaded, nil
}
