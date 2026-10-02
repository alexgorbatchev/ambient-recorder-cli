package main

import (
	"fmt"
	"io"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

func newMicrophoneListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List available microphones and identifiers for preferences", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			devices, err := capture.Microphones()
			if err != nil {
				return err
			}
			return writeMicrophones(cmd.OutOrStdout(), devices, cobrahelptree.IsAgentMode())
		},
	}
}

func writeMicrophones(w io.Writer, devices []capture.Device, agent bool) error {
	if len(devices) == 0 {
		_, err := fmt.Fprintln(w, "No microphones available")
		return err
	}
	for _, d := range devices {
		var line string
		if agent {
			line = fmt.Sprintf("name=%q uid=%q device_id=%d default=%t transport=%q type=%q manufacturer=%q model=%q\n", d.Name, d.UID, d.ID, d.Default, d.Transport, d.Type, d.Manufacturer, d.Model)
		} else {
			mark := ""
			if d.Default {
				mark = " [system default]"
			}
			line = fmt.Sprintf("%q%s\n  Connection: %s\n  Input type: %s\n", d.Name, mark, d.Transport, d.Type)
			if d.Manufacturer != "" {
				line += fmt.Sprintf("  Manufacturer: %s\n", d.Manufacturer)
			}
			if d.Model != "" {
				line += fmt.Sprintf("  Model: %s\n", d.Model)
			}
			if d.UID != "" {
				line += fmt.Sprintf("  Preference: %q\n", "uid:"+d.UID)
			}
			line += "\n"
		}
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}
