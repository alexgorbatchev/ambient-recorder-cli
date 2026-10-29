package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/homepath"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/storage"
	"github.com/spf13/cobra"
)

func newPrintDirCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use: "print-dir", Short: "Print the configured recording directory", Args: cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := recordingConfig(cmd, *configPath)
			if err != nil {
				return err
			}
			path, err := filepath.Abs(cfg.Output)
			if err != nil {
				return fmt.Errorf("resolve recording directory: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), homepath.Fold(path))
			return err
		},
	}
}

func newPrintFileCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use: "print-file", Short: "Print paths of open source recordings, one per line", Args: cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := recordingConfig(cmd, *configPath)
			if err != nil {
				return err
			}
			paths, err := storage.CurrentFiles(cfg.Output)
			if err != nil {
				return fmt.Errorf("inspect recording in %q: %w", cfg.Output, err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), homepath.FoldLines(strings.Join(paths, "\n")))
			return err
		},
	}
	cmd.Flags().String("output", "", "Recording directory to inspect (defaults to configured output directory)")
	return cmd
}
