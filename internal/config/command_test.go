package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestOnCloseCommandConfiguration(t *testing.T) {
	for _, tt := range []struct {
		input string
		valid bool
	}{
		{"", true}, {"output.on_close_command = []", true},
		{`output.on_close_command = ['/bin/echo', '{file}', '{stem}.md']`, true},
		{`output.on_close_command = ['']`, false}, {`output.on_close_command = ['   ']`, false},
		{`output.on_close_command = 'echo file'`, false},
		{`output.on_close_command = ["echo", "\u0000"]`, false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			path := filepath.Join(testdir.New(t), "config.toml")
			if err := os.WriteFile(path, []byte(tt.input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, _, err := Load(path)
			if err == nil {
				err = cfg.Validate()
			}
			if (err == nil) != tt.valid {
				t.Fatalf("configuration error: %v", err)
			}
			if tt.valid && tt.input != "" && tt.input != "output.on_close_command = []" && !reflect.DeepEqual(cfg.Output.OnCloseCommand, []string{"/bin/echo", "{file}", "{stem}.md"}) {
				t.Fatal(cfg.Output.OnCloseCommand)
			}
		})
	}
}
