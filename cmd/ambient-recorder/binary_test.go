package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

// buildStandaloneBinary builds the CLI outside the repository's source tree and
// returns the executable plus an empty working directory away from the sources.
func buildStandaloneBinary(t *testing.T) (binary, dir string) {
	t.Helper()
	dir = testdir.New(t)
	binary = filepath.Join(filepath.Dir(filepath.Dir(dir)), "bin", filepath.Base(dir), "ambient-recorder")
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-work", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone binary: %v\n%s", err, out)
	}
	return binary, dir
}

func TestBinaryTargetsMinimumMacOSVersion(t *testing.T) {
	binary, _ := buildStandaloneBinary(t)
	loadCommands, err := exec.Command("otool", "-l", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect linked deployment target: %v\n%s", err, loadCommands)
	}
	minimumSeen := false
	for _, line := range strings.Split(string(loadCommands), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "minos" {
			minimumSeen = true
			if fields[1] != "14.2" {
				t.Fatalf("linked binary requires macOS %s; documented minimum is 14.2", fields[1])
			}
		}
	}
	if !minimumSeen {
		t.Fatalf("linked binary has no minimum macOS load command: %s", loadCommands)
	}
}
