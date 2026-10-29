package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// skillInstructions is the exact text `ambient-recorder skill` must print.
const skillInstructions = `Install the ambient-recorder skill using the Skills CLI:

  npx skills add alexgorbatchev/ambient-recorder-cli --skill ambient-recorder

Skill:

  https://github.com/alexgorbatchev/ambient-recorder-cli/tree/main/skills/ambient-recorder
`

const skillAlert = "ALERT: Agents must read `AGENT=1 ambient-recorder skill` before using this tool.\n"

// repositoryRoot is the git repository root relative to this package directory.
var repositoryRoot = filepath.Join("..", "..")

func readSkill(t *testing.T) string {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join(repositoryRoot, "skills", "ambient-recorder", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(doc)
}

func TestSkillPrintsFixedInstructions(t *testing.T) {
	for _, mode := range []string{"0", "1", "true", "yes"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			out, diagnostic, err := execute(t, context.Background(), "skill")
			if err != nil || diagnostic != "" || out != skillInstructions {
				t.Fatalf("skill must print the fixed instructions: err=%v stderr=%q stdout=%q", err, diagnostic, out)
			}
		})
	}
}

func TestSkillPointerLocatesRepositorySkill(t *testing.T) {
	out, _, err := execute(t, context.Background(), "skill")
	if err != nil {
		t.Fatal(err)
	}
	var skillName, skillDir string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if _, name, found := strings.Cut(line, " --skill "); found && strings.HasPrefix(line, "npx skills add ") {
			skillName = name
		}
		if rest, found := strings.CutPrefix(line, "https://github.com/"+skillSource.Repository+"/tree/"+skillSource.Branch+"/"); found {
			skillDir = rest
		}
	}
	if skillName == "" || skillDir == "" {
		t.Fatalf("skill output lacks the --skill value or repository URL: %q", out)
	}
	doc, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(skillDir), "SKILL.md"))
	if err != nil {
		t.Fatalf("printed skill path %s has no SKILL.md in the repository: %v", skillDir, err)
	}
	if name := frontmatterName(t, string(doc)); name != skillName {
		t.Fatalf("%s/SKILL.md frontmatter name %q does not match printed --skill %q", skillDir, name, skillName)
	}
}

func frontmatterName(t *testing.T, doc string) string {
	t.Helper()
	body, found := strings.CutPrefix(doc, "---\n")
	if !found {
		t.Fatal("SKILL.md does not open with YAML frontmatter")
	}
	frontmatter, _, found := strings.Cut(body, "\n---\n")
	if !found {
		t.Fatal("SKILL.md frontmatter is not terminated")
	}
	for _, line := range strings.Split(frontmatter, "\n") {
		if name, found := strings.CutPrefix(line, "name: "); found {
			return strings.TrimSpace(name)
		}
	}
	t.Fatal("SKILL.md frontmatter has no name")
	return ""
}

func TestSkillRejectsArgumentsAndFlags(t *testing.T) {
	for _, mode := range []string{"0", "1"} {
		t.Setenv("AGENT", mode)
		for _, args := range [][]string{{"skill", "extra"}, {"skill", "--unknown"}} {
			out, _, err := execute(t, context.Background(), args...)
			if err == nil || strings.Contains(out, "npx skills add") {
				t.Fatalf("AGENT=%s: skill accepted %v: err=%v stdout=%q", mode, args, err, out)
			}
		}
	}
}

func TestSkillOutputFailure(t *testing.T) {
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
	cmd, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	cmd.SetOut(w)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"skill"})
	if err := cmd.Execute(); !errors.Is(err, os.ErrClosed) || !strings.Contains(err.Error(), "writing the skill") {
		t.Fatalf("skill must return its write failure: %v", err)
	}
}

func TestSkillWithoutRepository(t *testing.T) {
	root := testdir.New(t)
	binary := filepath.Join(filepath.Dir(filepath.Dir(root)), "bin", filepath.Base(root), "ambient-recorder")
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-work", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone binary: %v\n%s", err, out)
	}
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
	for _, mode := range []string{"0", "1"} {
		cmd := exec.Command(binary, "skill")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "AGENT="+mode, "GOPROXY=off")
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		out, err := cmd.Output()
		if err != nil || diagnostic.Len() != 0 || string(out) != skillInstructions {
			t.Fatalf("offline skill mode=%s: err=%v stderr=%s", mode, err, &diagnostic)
		}
	}
}

func TestHelpSkillAlert(t *testing.T) {
	root := liveCommands(t)
	for _, mode := range []string{"0", "1", "TRUE", " yes "} {
		t.Setenv("AGENT", mode)
		walkCommands(root, func(c *cobra.Command) {
			path := strings.Fields(strings.TrimPrefix(c.CommandPath(), root.Name()))
			for _, args := range [][]string{append(append([]string{}, path...), "--help"), append([]string{"help"}, path...)} {
				out, diagnostic, err := execute(t, context.Background(), args...)
				if err != nil || diagnostic != "" {
					t.Fatalf("help %v mode=%q: err=%v stderr=%q", args, mode, err, diagnostic)
				}
				if mode == "0" {
					if strings.Contains(out, "ALERT:") {
						t.Fatalf("human help contains agent alert: %v", args)
					}
				} else if !strings.HasPrefix(out, skillAlert) || strings.Count(out, skillAlert) != 1 || !strings.Contains(out, "command: "+c.CommandPath()+"\n") {
					t.Fatalf("agent help %v must begin with one alert and retain framework output: %q", args, out)
				}
			}
		})
	}
}

func TestSkillOmitsNothing(t *testing.T) {
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	if missing := cobrahelptree.SkillOmissions(root, techCatalog, readSkill(t)); len(missing) > 0 {
		t.Fatalf("SKILL.md omits:\n%s", strings.Join(missing, "\n"))
	}
}

func TestSkillLiveInterface(t *testing.T) {
	text := readSkill(t)
	root := liveCommands(t)
	walkCommands(root, func(c *cobra.Command) {
		arguments := strings.TrimSpace(strings.TrimPrefix(c.Use, c.Name()))
		if arguments == "" {
			arguments = "none"
		}
		row := "| `" + c.CommandPath() + "` | `" + arguments + "` |"
		if !strings.Contains(text, row) {
			t.Errorf("skill missing live command/arguments: %s", row)
		}
		c.InitDefaultHelpFlag()
		c.InitDefaultVersionFlag()
		c.Flags().AddFlagSet(c.InheritedFlags())
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			short := "none"
			if f.Shorthand != "" {
				short = "-" + f.Shorthand
			}
			value := f.DefValue
			if value == "" {
				value = `""`
			}
			metadata := " | `--" + f.Name + "` | `" + short + "` | `" + f.Value.Type() + "` | `" + value + "` |"
			if !strings.Contains(text, "| `"+c.CommandPath()+"`"+metadata) && !strings.Contains(text, "| `all`"+metadata) {
				t.Errorf("skill missing live flag for %s:%s", c.CommandPath(), metadata)
			}
		})
	})
}

func liveCommands(t *testing.T) *cobra.Command {
	t.Helper()
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return root
}

func walkCommands(c *cobra.Command, visit func(*cobra.Command)) {
	if c.Hidden {
		return
	}
	visit(c)
	for _, child := range c.Commands() {
		walkCommands(child, visit)
	}
}
