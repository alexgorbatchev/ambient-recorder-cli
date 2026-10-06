package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/recording"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestRecordingVersionLifetime(t *testing.T) {
	dir := testdir.New(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "v.sock")
	t.Setenv(serviceinfo.Environment, path)
	err = withServiceInfo("running-old-version", "", recording.Config{}, func(recording.Config) error {
		v, err := serviceinfo.Query(context.Background(), path)
		if err != nil || v.Version != "running-old-version" {
			t.Fatalf("live version: %q %v", v, err)
		}
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("recording error lost: %v", err)
	}
	if _, err := serviceinfo.Query(context.Background(), path); err == nil {
		t.Fatal("stopped recorder still reports a version")
	}
	t.Setenv(serviceinfo.Environment, filepath.Join(path, "invalid"))
	if err := withServiceInfo("1", "", recording.Config{}, func(recording.Config) error { t.Fatal("recording ran after endpoint failure"); return nil }); err == nil {
		t.Fatal("endpoint failure hidden")
	}
	t.Setenv(serviceinfo.Environment, "")
	if err := withServiceInfo("1", "", recording.Config{}, func(recording.Config) error { return os.ErrPermission }); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("foreground error lost: %v", err)
	}
}

func TestLoadedServiceVersion(t *testing.T) {
	a := temporaryLaunchAgent(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a.plist, err = filepath.Rel(cwd, a.plist)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.install(context.Background(), agentSettings{output: testdir.New(t)}); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, err := executeService(t, a, "", "status")
		if err != nil || !strings.Contains(out, "unavailable") {
			t.Fatalf("loaded process without version: %q %v", out, err)
		}
	}
	liveFile := filepath.Join(testdir.New(t), "unrelated.opus")
	s, err := serviceinfo.Listen(serviceinfo.Path(a.plist), serviceinfo.Info{Version: "older-than-cli", OutputDir: filepath.Dir(liveFile), CurrentFile: liveFile})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, err := executeService(t, a, "", "status", "--details")
		if err != nil || !strings.Contains(out, "older-than-cli") || !strings.Contains(out, version) || !strings.Contains(out, a.label) {
			t.Fatalf("version status: %q %v", out, err)
		}
	}
	if _, err := executeService(t, a, "", "stop"); err != nil {
		t.Fatal(err)
	}
	out, err := executeService(t, a, "", "status")
	if err != nil || !strings.Contains(out, "not running") || strings.Contains(out, "older-than-cli") || strings.Contains(out, liveFile) {
		t.Fatalf("unloaded service queried unrelated live endpoint: %q %v", out, err)
	}
}
