package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

func writeStatusTable(w io.Writer, rows []table.Row) error {
	t := table.NewWriter()
	t.Style().Options = table.OptionsNoBordersAndSeparators
	t.Style().Box.PaddingLeft = ""
	t.Style().Box.PaddingRight = "  "
	t.SuppressTrailingSpaces()
	t.AppendRows(rows)
	_, err := fmt.Fprintln(w, t.Render())
	return err
}

func shortenHomePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) || !filepath.IsAbs(path) {
		return path
	}
	home = filepath.Clean(home)
	if path == home {
		return "~/"
	}
	prefix := strings.TrimSuffix(home, string(filepath.Separator)) + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return "~/" + strings.TrimPrefix(path, prefix)
	}
	return path
}
