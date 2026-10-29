package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/homepath"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v3"
)

var version = "dev"

func main() {
	cmd, err := newRootCommand()
	if err == nil {
		err = cmd.Execute()
	}
	if err != nil {
		if printErr := writeError(os.Stderr, err); printErr != nil {
			// The failing diagnostic stream cannot prevent a nonzero exit.
		}
		os.Exit(1)
	}
}

// writeError prints err as one diagnostic line with home paths folded for display.
func writeError(w io.Writer, err error) error {
	prefix := "[ERROR]"
	if cobrahelptree.IsAgentMode() {
		prefix = "ERR:"
	}
	_, printErr := fmt.Fprintf(w, "%s %s\n", prefix, homepath.FoldText(err.Error()))
	return printErr
}
