package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const serviceLabel = "com.alexgorbatchev.ambient-recorder"

func newServicePrintCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use: "print", Short: "Print a macOS configuration that restarts recording after exit", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				var err error
				output, err = defaultOutput()
				if err != nil {
					return err
				}
			}
			path, err := filepath.Abs(output)
			if err != nil {
				return fmt.Errorf("resolve recording directory: %w", err)
			}
			binary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve recorder executable: %w", err)
			}
			binary, err = filepath.EvalSymlinks(binary)
			if err != nil {
				return fmt.Errorf("resolve recorder executable symlink: %w", err)
			}
			return writeAgent(cmd.OutOrStdout(), binary, path)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Recording directory (defaults to XDG user data/ambient-recorder)")
	return cmd
}

// launchd owns process supervision. Printing a plist is read-only; the user
// chooses the stable binary location and explicitly loads the per-user agent.
func writeAgent(w io.Writer, binary, output string) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	e := xml.NewEncoder(w)
	e.Indent("", "  ")
	plist := xml.StartElement{Name: xml.Name{Local: "plist"}, Attr: []xml.Attr{{Name: xml.Name{Local: "version"}, Value: "1.0"}}}
	dict := xml.StartElement{Name: xml.Name{Local: "dict"}}
	for _, token := range []xml.Token{plist, dict} {
		if err := e.EncodeToken(token); err != nil {
			return err
		}
	}
	fields := []struct {
		key   string
		value any
	}{
		{"Label", serviceLabel},
		{"ProgramArguments", []string{binary, "recording", "start", "--output", output}},
		{"KeepAlive", true},
		{"ThrottleInterval", 5},
		{"StandardErrorPath", filepath.Join(output, "supervisor.stderr.log")},
	}
	for _, field := range fields {
		if err := e.EncodeElement(field.key, xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
			return err
		}
		if err := encodePlistValue(e, field.value); err != nil {
			return err
		}
	}
	for _, token := range []xml.Token{dict.End(), plist.End()} {
		if err := e.EncodeToken(token); err != nil {
			return err
		}
	}
	return e.Flush()
}

func encodePlistValue(e *xml.Encoder, value any) error {
	switch v := value.(type) {
	case string:
		return e.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: "string"}})
	case int:
		return e.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: "integer"}})
	case bool:
		name := "false"
		if v {
			name = "true"
		}
		return e.EncodeElement("", xml.StartElement{Name: xml.Name{Local: name}})
	case []string:
		array := xml.StartElement{Name: xml.Name{Local: "array"}}
		if err := e.EncodeToken(array); err != nil {
			return err
		}
		for _, arg := range v {
			if err := encodePlistValue(e, arg); err != nil {
				return err
			}
		}
		return e.EncodeToken(array.End())
	default:
		return fmt.Errorf("unsupported launchd value type %T", value)
	}
}
