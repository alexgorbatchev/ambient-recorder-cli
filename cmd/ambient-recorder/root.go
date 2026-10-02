package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/config"
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
	var configPath string
	root.PersistentFlags().StringVar(&configPath, "config", "", "TOML configuration file (defaults to XDG configuration/ambient-recorder/config.toml)")
	group := &cobra.Command{Use: "recording", Short: "Capture microphone and computer playback"}
	group.AddCommand(newStartCommand(&configPath))
	root.AddCommand(group)
	root.AddCommand(newServiceCommand(&configPath, userLaunchAgent))
	microphone := &cobra.Command{Use: "microphone", Short: "Inspect microphone inputs"}
	microphone.AddCommand(newMicrophoneListCommand())
	root.AddCommand(microphone)
	configuration := &cobra.Command{Use: "config", Short: "Manage recorder configuration"}
	configuration.AddCommand(newConfigInitCommand(&configPath))
	root.AddCommand(configuration)
	root.AddCommand(newSkillCommand())
	catalog := cobrahelptree.TechCatalog{
		"ambient-recorder recording start": {
			Summary: "Record continuously until interrupted",
			Env: []cobrahelptree.EnvSpec{
				{Name: "XDG_DATA_HOME", Description: "Absolute directory for user data; defaults to ~/.local/share"},
				{Name: "XDG_CONFIG_HOME", Description: "Absolute directory for configuration; defaults to ~/.config"},
				{Name: "AGENT", Description: "Set to 1, true, or yes for compact command output"},
				{Name: "NO_COLOR", Description: "Any nonempty value disables colors in recording logs"},
				{Name: "TERM", Description: "Set to dumb to disable colors in recording logs"},
			},
			Metadata: map[string]string{"output": "mono Ogg Opus", "platform": "macOS 14.2+", "writes": "recordings and diagnostic logs"},
		},
	}
	catalog["ambient-recorder help"] = cobrahelptree.TechInfo{Args: []cobrahelptree.ArgSpec{{Name: "[command]", Description: "Command path to describe"}}}
	if err := configureHelp(root, catalog); err != nil {
		return nil, err
	}
	return root, nil
}

func newStartCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use: "start", Short: "Record continuously until interrupted", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, _, err := recordingConfig(cmd, *configPath)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return recording.Run(ctx, resolved, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().String("output", "", "Recording directory (defaults to XDG user data/ambient-recorder)")
	cmd.Flags().Int("bitrate", config.DefaultBitrate, "Opus bitrate in bits per second (6000..128000)")
	cmd.Flags().Int("complexity", config.DefaultComplexity, "Opus complexity (0..10; higher uses more CPU)")
	cmd.Flags().Duration("sync-interval", config.DefaultSyncInterval, "Interval between storage synchronization attempts")
	return cmd
}
