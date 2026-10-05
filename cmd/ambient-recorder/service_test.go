package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/serviceinfo"
	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestServiceLifecycleCommands(t *testing.T) {
	for _, verb := range []string{"install", "start", "stop", "restart", "status", "uninstall"} {
		t.Run(verb, func(t *testing.T) {
			root, err := newRootCommand()
			if err != nil {
				t.Fatal(err)
			}
			cmd, _, err := root.Find([]string{"service", verb})
			if err != nil || cmd.CommandPath() != "ambient-recorder service "+verb {
				t.Fatalf("missing service %s: command=%v err=%v", verb, cmd, err)
			}
			out, diagnostic, err := execute(t, context.Background(), "service", verb, "--help")
			if err != nil || diagnostic != "" || !strings.Contains(out, verb) {
				t.Fatalf("help: stdout=%q stderr=%q err=%v", out, diagnostic, err)
			}
			if _, _, err := execute(t, context.Background(), "service", verb, "unexpected"); err == nil {
				t.Fatal("service action accepted a positional argument")
			}
		})
	}
}

func TestUserServiceResolution(t *testing.T) {
	dir := testdir.New(t)
	t.Setenv("HOME", dir)
	a, err := userLaunchAgent()
	if err != nil {
		t.Fatal(err)
	}
	if a.plist != filepath.Join(dir, "Library", "LaunchAgents", serviceLabel+".plist") || !filepath.IsAbs(a.binary) {
		t.Fatalf("service paths not resolved from user context: %+v", a)
	}
	if _, err := os.Stat(a.binary); err != nil {
		t.Fatalf("resolved executable cannot be opened: %v", err)
	}
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, _, err := execute(t, context.Background(), "service", "status", "--config", filepath.Join(dir, "absent.toml"))
		if err != nil || !strings.Contains(out, a.plist) {
			t.Fatalf("read-only status must work without recording config: %q %v", out, err)
		}
		want := "CLI version: " + version + "\n"
		if mode == "1" {
			want = fmt.Sprintf("cli_version=%q", version)
		}
		if !strings.Contains(out, want) {
			t.Fatalf("status must show invoked CLI version: %q", out)
		}
	}
	t.Setenv("HOME", "")
	if _, err := userLaunchAgent(); err == nil {
		t.Fatal("service ignored missing user home")
	}
}

func TestServiceInstallConfigurationFailures(t *testing.T) {
	a := temporaryLaunchAgent(t)
	dir := testdir.New(t)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	if _, err := executeService(t, a, filepath.Join(dir, "missing.toml"), "install"); err == nil {
		t.Fatal("install accepted missing explicit config")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := executeService(t, a, "", "install"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("install hid configuration creation failure: %v", err)
	}
	if _, err := os.Stat(a.plist); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid configuration installed service: %v", err)
	}
}

// These jobs use /usr/bin/false and a unique label. They never open hardware or
// write to ~/Library/LaunchAgents; cleanup removes them from the live domain.
func temporaryLaunchAgent(t *testing.T) launchAgent {
	t.Helper()
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	if out, err := exec.Command("/bin/launchctl", "print", domain).CombinedOutput(); err != nil {
		t.Skipf("GUI login session unavailable: %v %s", err, out)
	}
	dir := testdir.New(t)
	label := serviceLabel + ".test." + filepath.Base(dir)
	a := launchAgent{
		plist:  filepath.Join(dir, "agent & files", label+".plist"),
		domain: domain, target: domain + "/" + label, label: label,
		control: "/bin/launchctl", binary: "/usr/bin/false",
	}
	t.Cleanup(func() {
		if data, err := os.ReadFile(a.plist); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "installed.plist"), data, 0o600); err != nil {
				t.Error(err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
		if err := a.manage(context.Background(), "uninstall"); err != nil {
			t.Error(err)
		}
	})
	return a
}

func executeService(t *testing.T, a launchAgent, path string, args ...string) (string, error) {
	t.Helper()
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := root.Find([]string{"service"})
	if err != nil {
		t.Fatal(err)
	}
	root.RemoveCommand(old)
	root.AddCommand(newServiceCommand(&path, func() (launchAgent, error) { return a, nil }))
	var out, diagnostic bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostic)
	root.SetArgs(append([]string{"service"}, args...))
	err = root.ExecuteContext(context.Background())
	if diagnostic.Len() != 0 {
		t.Fatalf("unexpected diagnostic: %s", &diagnostic)
	}
	return out.String(), err
}

func TestNativeServiceLifecycle(t *testing.T) {
	a := temporaryLaunchAgent(t)
	t.Setenv("XDG_CONFIG_HOME", testdir.New(t))
	t.Setenv("XDG_DATA_HOME", testdir.New(t))
	output := filepath.Join(testdir.New(t), "audio & notes")
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, err := executeService(t, a, "", "status")
		if err != nil || !(strings.Contains(out, "Installed: false") || strings.Contains(out, "installed=false")) {
			t.Fatalf("uninstalled status: %q %v", out, err)
		}
		if !strings.Contains(out, "not running") {
			t.Fatalf("unloaded service claimed a live version: %q", out)
		}
	}
	if _, err := executeService(t, a, "", "install", "--output", output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(a.plist)
	if err != nil {
		t.Fatal(err)
	}
	// Inspect generated values through Apple's parser, including XML escaping.
	for _, key := range []string{"Label", "ProgramArguments.0", "ProgramArguments.4", "Umask", "KeepAlive", "LimitLoadToSessionType", "EnvironmentVariables.AMBIENT_RECORDER_SERVICE_SOCKET"} {
		cmd := exec.Command("/usr/bin/plutil", "-extract", key, "raw", "-o", "-", a.plist)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("parse generated %s: %v %s", key, err, out)
		}
		value := strings.TrimSpace(string(out))
		switch key {
		case "Label":
			if value != a.label {
				t.Fatalf("unexpected registered label: %q", value)
			}
		case "ProgramArguments.0":
			if value != a.binary {
				t.Fatalf("executable path changed: %q", value)
			}
		case "ProgramArguments.4":
			if value != output {
				t.Fatalf("output path changed: %q", value)
			}
		case "EnvironmentVariables.AMBIENT_RECORDER_SERVICE_SOCKET":
			if value != serviceinfo.Path(a.plist) {
				t.Fatalf("installed endpoint does not match status query: %q", value)
			}
		}
	}
	info, err := os.Stat(a.plist)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("service plist permissions: info=%v err=%v", info, err)
	}
	if _, err := executeService(t, a, "", "install"); err == nil {
		t.Fatal("installation replaced an existing agent")
	}
	unchanged, err := os.ReadFile(a.plist)
	if err != nil || !bytes.Equal(unchanged, data) {
		t.Fatalf("repeated install changed plist: %v", err)
	}
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		out, err := executeService(t, a, "", "status", "--details")
		if err != nil || !strings.Contains(out, a.target) {
			t.Fatalf("loaded diagnostics: %q %v", out, err)
		}
		for _, action := range []string{"start", "restart", "stop", "stop", "restart", "stop", "start"} {
			out, err := executeService(t, a, "", action)
			if err != nil || !strings.Contains(out, a.plist) {
				t.Fatalf("action %s: %q %v", action, out, err)
			}
			_, loaded, err := a.loaded(context.Background())
			if err != nil || loaded != (action != "stop") {
				t.Fatalf("action %s loaded=%t err=%v", action, loaded, err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(a.plist), "installed.plist"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := executeService(t, a, "", "uninstall"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(a.plist); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("service plist remains after uninstall: %v", err)
	}
	for _, action := range []string{"start", "restart"} {
		if _, err := executeService(t, a, "", action); err == nil || !strings.Contains(err.Error(), "service install") {
			t.Fatalf("%s without installation: %v", action, err)
		}
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("uninstall removed recording directory: %v", err)
	}
}

func TestServiceFailures(t *testing.T) {
	ctx := context.Background()
	dir := testdir.New(t)
	a := launchAgent{plist: filepath.Join(dir, "agent.plist"), control: "/usr/bin/false", binary: "/usr/bin/false", label: serviceLabel + ".failure"}
	settings := agentSettings{output: filepath.Join(dir, "output"), config: filepath.Join(dir, "config.toml")}
	if err := a.install(ctx, settings); err == nil || !strings.Contains(err.Error(), "service file saved") {
		t.Fatalf("failed bootstrap lost retry instructions: %v", err)
	}
	if _, err := os.Stat(a.plist); err != nil {
		t.Fatalf("failed startup lost installed plist: %v", err)
	}
	for _, action := range []string{"start", "stop", "restart", "uninstall", "unknown"} {
		if err := a.manage(ctx, action); err == nil {
			t.Fatalf("service hid controller failure for %s", action)
		}
	}
	if err := a.status(ctx, io.Discard, true, false); err == nil {
		t.Fatal("status hid controller failure")
	}
	if err := a.unload(ctx); err == nil {
		t.Fatal("unload hid controller failure")
	}
	if err := os.Remove(a.plist); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(a.plist, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"start", "restart"} {
		if err := a.manage(ctx, action); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("directory masquerading as service accepted by %s: %v", action, err)
		}
	}
	if err := a.status(ctx, io.Discard, false, false); err == nil {
		t.Fatal("status accepted a directory")
	}
	if err := os.WriteFile(filepath.Join(a.plist, "keep"), []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	// /usr/bin/true succeeds without changing launchd; this isolates filesystem
	// removal failure while the lifecycle itself is tested against real launchd.
	a.control = "/usr/bin/true"
	if err := a.manage(ctx, "uninstall"); err == nil || !strings.Contains(err.Error(), "remove service file") {
		t.Fatalf("uninstall hid removal failure: %v", err)
	}
	a.plist = filepath.Join(a.plist, "keep", "child.plist")
	if _, err := a.installed(); err == nil {
		t.Fatal("service inspection hid invalid parent")
	}
	if err := a.install(ctx, settings); err == nil {
		t.Fatal("installation hid directory creation failure")
	}
	a.plist = filepath.Join(dir, "unwritten.plist")
	settings.output = filepath.Join(dir, "invalid\x00output")
	if err := a.install(ctx, settings); err == nil {
		t.Fatal("installation accepted invalid output")
	}
	readonly := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readonly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(readonly, 0o700); err != nil {
			t.Error(err)
		}
	})
	a.plist = filepath.Join(readonly, "agent.plist")
	settings.output = filepath.Join(dir, "output")
	if err := a.install(ctx, settings); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("installation hid service file creation error: %v", err)
	}
	a.control = filepath.Join(dir, "missing-launchctl")
	if _, err := a.run(ctx, "print", a.target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("launchctl execution error not preserved: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.run(canceled, "print", a.target); !errors.Is(err, context.Canceled) {
		t.Fatalf("launchctl ignored cancellation: %v", err)
	}
}

func TestServiceOutputAndConnectionErrors(t *testing.T) {
	a := temporaryLaunchAgent(t)
	if err := a.install(context.Background(), agentSettings{output: filepath.Join(filepath.Dir(a.plist), "audio")}); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, mode := range []bool{false, true} {
		if err := a.status(context.Background(), w, true, mode); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("status discarded write failure: %v", err)
		}
	}
	path := ""
	group := newServiceCommand(&path, func() (launchAgent, error) { return launchAgent{}, os.ErrPermission })
	for _, cmd := range group.Commands() {
		if cmd.Name() == "print" {
			continue
		}
		if err := cmd.RunE(cmd, nil); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("%s discarded connection error: %v", cmd.Name(), err)
		}
	}
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		for _, cmd := range group.Commands() {
			if cmd.Name() == "print" || cmd.Name() == "status" {
				continue
			}
			cmd.SetOut(w)
			if err := writeServiceResult(cmd, cmd.Name(), a.plist); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("%s discarded result write failure: %v", cmd.Name(), err)
			}
		}
	}
	settings := agentSettings{binary: "/usr/bin/false", output: "audio", label: "test"}
	if err := writeAgent(w, settings); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("plist discarded write failure: %v", err)
	}
}
