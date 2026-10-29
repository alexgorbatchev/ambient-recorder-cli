package main

import (
	"fmt"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v3"
	"github.com/spf13/cobra"
)

// fallbackHelpWidth bounds human help when stdout is not a terminal and COLUMNS
// is unset, so redirected help still fits a standard terminal.
const fallbackHelpWidth = 80

// skillSource is where the agent skill is published:
// skills/ambient-recorder/SKILL.md on the repository's default branch.
var skillSource = cobrahelptree.SkillSource{Repository: "alexgorbatchev/ambient-recorder-cli", Branch: "main"}

var techCatalog = cobrahelptree.TechCatalog{
	"ambient-recorder recording start": {
		Summary: "Record continuously until interrupted",
		Env: []cobrahelptree.EnvSpec{
			{Name: "XDG_DATA_HOME", Description: "Absolute directory for user data; defaults to ~/.local/share"},
			{Name: "XDG_CONFIG_HOME", Description: "Absolute directory for configuration; defaults to ~/.config"},
			{Name: "AGENT", Description: "Set to 1, true, or yes for compact command output"},
			{Name: "NO_COLOR", Description: "Any nonempty value disables colors in recording logs"},
			{Name: "TERM", Description: "Set to dumb to disable colors in recording logs"},
			{Name: serviceinfo.Environment, Description: "Unix socket for reporting live service version and recording paths; set by service installation"},
		},
		Metadata: map[string]string{"output": "mono Ogg Opus", "platform": "macOS 14.2+", "writes": "recordings and diagnostic logs"},
	},
	"ambient-recorder help": {
		Args: []cobrahelptree.ArgSpec{{Name: "[command]", Description: "Command path to describe"}},
	},
}

func configureHelp(root *cobra.Command) error {
	width := cobrahelptree.GetTerminalWidth()
	if width == 0 {
		width = fallbackHelpWidth
	}
	options := cobrahelptree.HelpOptions{
		Catalog: techCatalog,
		Skill:   skillSource,
		Tree:    cobrahelptree.TreeOptions{HideGeneratedCommands: true, TerminalWidth: width},
	}
	if err := cobrahelptree.SetupWithOptions(root, options); err != nil {
		return fmt.Errorf("configure help: %w", err)
	}
	return nil
}
