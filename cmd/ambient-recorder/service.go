package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/config"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/plist"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

const serviceLabel = "com.alexgorbatchev.ambient-recorder"

func newServiceCommand(configPath *string, connect func() (launchAgent, error)) *cobra.Command {
	group := &cobra.Command{Use: "service", Short: "Manage background recording and startup at login", Args: cobra.NoArgs}
	group.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }
	group.AddCommand(newServicePrintCommand(configPath), newServiceInstallCommand(configPath, connect))
	for _, action := range []struct{ verb, description string }{
		{"start", "Enable startup at login and request recording now"},
		{"stop", "Stop background recording and disable startup at login"},
		{"restart", "Restart background recording to read configuration changes"},
		{"uninstall", "Stop background recording and remove startup at login"},
	} {
		cmd := &cobra.Command{Use: action.verb, Short: action.description, Args: cobra.NoArgs}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			a, err := connect()
			if err != nil {
				return err
			}
			if err := a.manage(cmd.Context(), cmd.Name()); err != nil {
				return err
			}
			return writeServiceResult(cmd, cmd.Name(), a.plist)
		}
		group.AddCommand(cmd)
	}
	status := &cobra.Command{Use: "status", Short: "Show background service state, versions and recording paths", Args: cobra.NoArgs}
	status.Flags().Bool("details", false, "Include the native macOS service diagnostic report")
	status.RunE = func(cmd *cobra.Command, args []string) error {
		a, err := connect()
		if err != nil {
			return err
		}
		details, err := cmd.Flags().GetBool("details")
		if err != nil {
			return err
		}
		return a.status(cmd.Context(), cmd.OutOrStdout(), details, cobrahelptree.IsAgentMode())
	}
	group.AddCommand(status)
	return group
}

func newServiceInstallCommand(configPath *string, connect func() (launchAgent, error)) *cobra.Command {
	cmd := &cobra.Command{Use: "install", Short: "Install startup at login and request recording now", Args: cobra.NoArgs}
	cmd.Flags().String("output", "", "Recording directory (defaults to configured output directory)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		a, err := connect()
		if err != nil {
			return err
		}
		settings, err := serviceSettings(cmd, *configPath)
		if err != nil {
			return err
		}
		if settings.config == "" {
			settings.config, err = config.DefaultPath()
			if err != nil {
				return err
			}
			if err := config.Create(settings.config); err != nil {
				return err
			}
		}
		if err := a.install(cmd.Context(), settings); err != nil {
			return err
		}
		return writeServiceResult(cmd, cmd.Name(), a.plist)
	}
	return cmd
}

type agentSettings struct{ binary, output, config, label, socket string }

func serviceSettings(cmd *cobra.Command, configPath string) (agentSettings, error) {
	cfg, loaded, err := recordingConfig(cmd, configPath)
	if err != nil {
		return agentSettings{}, err
	}
	path, err := filepath.Abs(cfg.Output)
	if err != nil {
		return agentSettings{}, fmt.Errorf("resolve recording directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return agentSettings{}, fmt.Errorf("resolve service home directory: %w", err)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist")
	return agentSettings{output: path, config: loaded, label: serviceLabel, socket: serviceinfo.Path(plist)}, nil
}

func recorderExecutable() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve recorder executable: %w", err)
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return "", fmt.Errorf("resolve recorder executable symlink: %w", err)
	}
	return binary, nil
}

func newServicePrintCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use: "print", Short: "Print a macOS configuration that restarts recording after exit", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := serviceSettings(cmd, *configPath)
			if err != nil {
				return err
			}
			settings.binary, err = recorderExecutable()
			if err != nil {
				return err
			}
			return writeAgent(cmd.OutOrStdout(), settings)
		},
	}
	cmd.Flags().String("output", "", "Recording directory (defaults to XDG user data/ambient-recorder)")
	return cmd
}

// launchd owns process supervision. KeepAlive also starts the agent at load.
func writeAgent(w io.Writer, settings agentSettings) error {
	args := []string{settings.binary, "recording", "start", "--output", settings.output}
	if settings.config != "" {
		args = append(args, "--config", settings.config)
	}
	fields := map[string]any{
		"Label":                  settings.label,
		"ProgramArguments":       args,
		"KeepAlive":              true,
		"LimitLoadToSessionType": "Aqua",
		"ThrottleInterval":       5,
		"Umask":                  0o077,
		"StandardErrorPath":      filepath.Join(settings.output, "supervisor.stderr.log"),
	}
	if settings.socket != "" {
		fields["EnvironmentVariables"] = map[string]any{serviceinfo.Environment: settings.socket}
	}
	data, err := plist.XML(fields)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func writeServiceResult(cmd *cobra.Command, action, path string) error {
	if cobrahelptree.IsAgentMode() {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "service=%s plist=%q\n", action, path)
		return err
	}
	messages := map[string]string{
		"install":   "Service installed. Starts at login; recording launch requested.",
		"start":     "Service enabled at login; recording launch requested.",
		"stop":      "Service stopped. Startup at login disabled.",
		"restart":   "Service restarted. Startup at login enabled; recording launch requested.",
		"uninstall": "Service uninstalled. Recordings and configuration retained.",
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\nService file: %s\n", messages[action], path)
	return err
}
