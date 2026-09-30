package main

import (
	"fmt"
	"os"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
)

var version = "dev"

func main() {
	cmd, err := newRootCommand()
	if err == nil {
		err = cmd.Execute()
	}
	if err != nil {
		prefix := "[ERROR]"
		if cobrahelptree.IsAgentMode() {
			prefix = "ERR:"
		}
		if _, printErr := fmt.Fprintf(os.Stderr, "%s %v\n", prefix, err); printErr != nil {
			// The failing diagnostic stream cannot prevent a nonzero exit.
		}
		os.Exit(1)
	}
}
