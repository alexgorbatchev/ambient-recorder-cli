package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestRefreshServiceConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name      string
		fields    map[string]any
		wantError bool
	}{
		{"missing arguments", map[string]any{}, true},
		{"empty arguments", map[string]any{"ProgramArguments": []string{}}, true},
		{"wrong arguments type", map[string]any{"ProgramArguments": "binary"}, true},
		{"wrong environment type", map[string]any{"ProgramArguments": []string{"old"}, "EnvironmentVariables": []string{"wrong"}}, true},
		{"absent environment", map[string]any{"ProgramArguments": []string{"old", "recording", "start"}}, false},
		{"program override", map[string]any{"Program": "old", "ProgramArguments": []string{"old", "recording", "start"}, "Nice": 10, "EnvironmentVariables": map[string]string{"KEEP_ME": "unchanged"}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := launchAgent{plist: filepath.Join(testdir.New(t), "agent.plist"), binary: "/usr/bin/false"}
			writeTestService(t, a.plist, tt.fields)
			changed, err := a.refresh(context.Background())
			if (err != nil) != tt.wantError || (!tt.wantError && !changed) {
				t.Fatalf("refresh changed=%t err=%v", changed, err)
			}
			if tt.wantError {
				return
			}
			var saved struct {
				Program              string
				ProgramArguments     []string
				EnvironmentVariables map[string]string
				Nice                 int
			}
			if err := a.readPlist(context.Background(), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.ProgramArguments[0] != a.binary || saved.EnvironmentVariables[serviceinfo.Environment] != serviceinfo.Path(a.plist) {
				t.Fatalf("updated service did not select this executable and endpoint: %+v", saved)
			}
			if tt.name == "program override" && (saved.Program != a.binary || saved.Nice != 10 || saved.EnvironmentVariables["KEEP_ME"] != "unchanged") {
				t.Fatalf("refresh lost the effective executable or unrelated settings: %+v", saved)
			}
			before, err := os.Stat(a.plist)
			if err != nil {
				t.Fatal(err)
			}
			if changed, err := a.refresh(context.Background()); err != nil || changed {
				t.Fatalf("unchanged service was rewritten: changed=%t err=%v", changed, err)
			}
			after, err := os.Stat(a.plist)
			if err != nil || !os.SameFile(before, after) || after.Mode().Perm() != 0o600 {
				t.Fatalf("unchanged plist replaced or permissions lost: %v %v", after, err)
			}
		})
	}
}

func TestRefreshServiceFileFailures(t *testing.T) {
	dir := testdir.New(t)
	a := launchAgent{plist: filepath.Join(dir, "agent.plist"), binary: "/usr/bin/false"}
	if _, err := a.refresh(context.Background()); err == nil {
		t.Fatal("missing plist accepted")
	}
	if err := os.WriteFile(a.plist, []byte("not a plist"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.refresh(context.Background()); err == nil {
		t.Fatal("malformed plist accepted")
	}
	writeTestService(t, a.plist, map[string]any{"ProgramArguments": []string{"old"}})
	var wrongType []string
	if err := a.readPlist(context.Background(), &wrongType); err == nil {
		t.Fatal("plist decoder accepted an incompatible destination")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := a.refresh(context.Background()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("refresh hid a failed atomic replacement: %v", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a.plist); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(a.plist, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.replacePlist([]byte("replacement")); err == nil {
		t.Fatal("atomic replacement hid a conflicting directory")
	}
}
