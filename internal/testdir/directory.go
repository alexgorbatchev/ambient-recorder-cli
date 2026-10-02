// Package testdir retains test artifacts in the module's .tmp directory.
package testdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// New allocates a distinct owner-only directory without automatic removal.
func New(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		_, err := os.Stat(filepath.Join(root, "go.mod"))
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("cannot locate Go module for retained test artifacts")
		}
		root = parent
	}
	base := filepath.Join(root, ".tmp")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	path, err := os.MkdirTemp(base, strings.ReplaceAll(t.Name(), "/", "-")+"-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained test artifacts: %s", path)
	return path
}
