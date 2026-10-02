package main

import (
	"fmt"
	"io"
	"strings"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

const agentHelpAlert = "ALERT: Agents must read `AGENT=1 ambient-recorder skill` before using this tool."
const fallbackHelpWidth = 80

type helpRenderer func(*cobra.Command, cobrahelptree.TechCatalog, cobrahelptree.TreeOptions) string

func configureHelp(root *cobra.Command, catalog cobrahelptree.TechCatalog) error {
	width := cobrahelptree.GetTerminalWidth()
	if width == 0 {
		width = fallbackHelpWidth
	}
	options := cobrahelptree.HelpOptions{Catalog: catalog, Tree: cobrahelptree.TreeOptions{HideGeneratedCommands: true, TerminalWidth: width}}
	if err := cobrahelptree.SetupWithOptions(root, options); err != nil {
		return fmt.Errorf("configure help: %w", err)
	}
	// Use the library's renderers for the full command interface. Bound headers
	// and generated examples as well as the description columns it already clips.
	// Generated help/completion commands inherit both hooks.
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		if err := writeHelp(c.OutOrStdout(), c, options, cobrahelptree.RenderTreeHelp); err != nil {
			c.PrintErrln(err)
		}
	})
	root.SetUsageFunc(func(c *cobra.Command) error {
		return writeHelp(c.OutOrStderr(), c, options, cobrahelptree.RenderTreeUsage)
	})
	return nil
}

func writeHelp(w io.Writer, c *cobra.Command, options cobrahelptree.HelpOptions, render helpRenderer) error {
	var screen string
	if cobrahelptree.IsAgentMode() {
		screen = agentHelpAlert + "\n" + cobrahelptree.RenderAgentHelp(c, options.Catalog, options.Agent)
	} else {
		lines := strings.Split(render(c, options.Catalog, options.Tree), "\n")
		for i, line := range lines {
			lines[i] = runewidth.Truncate(line, options.Tree.TerminalWidth, "...")
		}
		screen = strings.Join(lines, "\n")
	}
	_, err := io.WriteString(w, screen)
	return err
}
