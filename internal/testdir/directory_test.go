package testdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetainedDirectory(t *testing.T) {
	var path string
	t.Run("recording artifact", func(t *testing.T) {
		path = New(t)
		if err := os.WriteFile(filepath.Join(path, "evidence.txt"), []byte("retained"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	b, err := os.ReadFile(filepath.Join(path, "evidence.txt"))
	if err != nil || string(b) != "retained" {
		t.Fatalf("subtest artifact missing: %q, %v", b, err)
	}
	if !strings.Contains(path, string(filepath.Separator)+".tmp"+string(filepath.Separator)) {
		t.Fatalf("artifact outside project temporary directory: %s", path)
	}
	if second := New(t); second == path {
		t.Fatal("two allocations shared an artifact directory")
	}
}
