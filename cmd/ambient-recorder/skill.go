package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const skillPointer = `Install the ambient-recorder skill using the Skills CLI:

  npx skills add alexgorbatchev/ambient-recorder-cli --skill ambient-recorder

Skill:

  https://github.com/alexgorbatchev/ambient-recorder-cli/tree/main/skills/ambient-recorder
`

func newSkillCommand() *cobra.Command {
	return &cobra.Command{
		Use: "skill", Short: "Print the install command and URL for the agent skill", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := io.WriteString(cmd.OutOrStdout(), skillPointer); err != nil {
				return fmt.Errorf("print skill pointer: %w", err)
			}
			return nil
		},
	}
}
