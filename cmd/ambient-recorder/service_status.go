package main

import (
	"fmt"
	"io"

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
