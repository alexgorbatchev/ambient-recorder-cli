package main

import (
	_ "embed"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

//go:embed SKILL.md
var skillDocument string

func newSkillCommand() *cobra.Command {
	return &cobra.Command{
		Use: "skill", Short: "Print the complete usage reference for agents", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := io.WriteString(cmd.OutOrStdout(), skillDocument); err != nil {
				return fmt.Errorf("print usage reference: %w", err)
			}
			return nil
		},
	}
}
