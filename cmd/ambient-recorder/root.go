package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

func newRootCommand() (*cobra.Command, error) {
	root := &cobra.Command{
		Use: "ambient-recorder", Short: "Record microphone and computer playback locally",
		Version: version, SilenceErrors: true, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	root.SetVersionTemplate("{{.Version}}\n")
	group := &cobra.Command{Use: "recording", Short: "Capture microphone and computer playback"}
	group.AddCommand(newStartCommand())
	root.AddCommand(group)
	service := &cobra.Command{Use: "service", Short: "Configure recording in the background"}
	service.AddCommand(newServicePrintCommand())
	root.AddCommand(service)
	catalog := cobrahelptree.TechCatalog{
		"ambient-recorder recording start": {
			Summary: "Record continuously until interrupted",
			Env: []cobrahelptree.EnvSpec{
				{Name: "XDG_DATA_HOME", Description: "Absolute directory for user data; defaults to ~/.local/share"},
				{Name: "AGENT", Description: "Set to 1, true, or yes for compact command output"},
			},
			Metadata: map[string]string{"output": "mono Ogg Opus", "platform": "macOS 14.2+", "writes": "recordings and diagnostic logs"},
		},
	}
	if err := cobrahelptree.SetupWithOptions(root, cobrahelptree.HelpOptions{Catalog: catalog, Tree: cobrahelptree.TreeOptions{HideGeneratedCommands: true}}); err != nil {
		return nil, fmt.Errorf("configure help: %w", err)
	}
	return root, nil
}

func newStartCommand() *cobra.Command {
	cfg := recording.Config{Bitrate: 32000, Complexity: 2, SyncInterval: 5 * time.Second}
	cmd := &cobra.Command{
		Use: "start", Short: "Record continuously until interrupted", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Output == "" {
				path, err := defaultOutput()
				if err != nil {
					return err
				}
				cfg.Output = path
			}
			cfg.Agent = cobrahelptree.IsAgentMode()
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return recording.Run(ctx, cfg, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&cfg.Output, "output", "", "Recording directory (defaults to XDG user data/ambient-recorder)")
	cmd.Flags().IntVar(&cfg.Bitrate, "bitrate", cfg.Bitrate, "Opus bitrate in bits per second (6000..128000)")
	cmd.Flags().IntVar(&cfg.Complexity, "complexity", cfg.Complexity, "Opus complexity (0..10; higher uses more CPU)")
	cmd.Flags().DurationVar(&cfg.SyncInterval, "sync-interval", cfg.SyncInterval, "Interval between storage synchronization attempts")
	return cmd
}

func defaultOutput() (string, error) {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		if !filepath.IsAbs(base) {
			return "", fmt.Errorf("XDG_DATA_HOME must be an absolute path: %q", base)
		}
		return filepath.Join(base, "ambient-recorder"), nil
	}
	base, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve recording directory: %w", err)
	}
	return filepath.Join(base, ".local", "share", "ambient-recorder"), nil
}
