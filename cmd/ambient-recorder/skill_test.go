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
	binary, dir := buildStandaloneBinary(t)
	for _, mode := range []string{"0", "1"} {
		cmd := exec.Command(binary, "skill")
		cmd.Dir = dir
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

// TestSkillTablesMatchCommandTree checks what SkillOmissions leaves to the
// repository: the command table lists exactly the live commands with their
// argument syntax, and the flag table lists exactly the live flags under the
// command that owns each one, with its shorthand, type and parser default. A
// flag every command accepts with the same metadata is listed once as `all`.
func TestSkillTablesMatchCommandTree(t *testing.T) {
	commands, flags := skillTableRows(readSkill(t))
	wantCommands, wantFlags := liveTableRows(liveCommands(t))
	compareRows(t, "command table", commands, wantCommands)
	compareRows(t, "flag table", flags, wantFlags)
}

// skillTableRows returns the identifying cells of every command-table row
// (command and arguments) and flag-table row (scope through parser default).
func skillTableRows(doc string) (commands, flags map[string]bool) {
	commands, flags = map[string]bool{}, map[string]bool{}
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.TrimPrefix(line, "| "), " | ")
		if len(cells) >= 5 && strings.HasPrefix(cells[1], "`--") {
			flags[strings.Join(cells[:5], " | ")] = true
		} else if len(cells) >= 2 {
			commands[strings.Join(cells[:2], " | ")] = true
		}
	}
	return commands, flags
}

func liveTableRows(root *cobra.Command) (commands, flags map[string]bool) {
	commands = map[string]bool{}
	owners := map[string][]string{}
	visited := 0
	walkCommands(root, func(c *cobra.Command) {
		visited++
		arguments := strings.TrimSpace(strings.TrimPrefix(c.Use, c.Name()))
		if arguments == "" {
			arguments = "none"
		}
		commands[tableCell(c.CommandPath())+" | "+tableCell(arguments)] = true
		c.InitDefaultHelpFlag()
		c.InitDefaultVersionFlag()
		for _, set := range []*pflag.FlagSet{c.LocalFlags(), c.InheritedFlags()} {
			set.VisitAll(func(f *pflag.Flag) {
				if !f.Hidden {
					owners[flagMetadata(f)] = append(owners[flagMetadata(f)], c.CommandPath())
				}
			})
		}
	})
	flags = map[string]bool{}
	for metadata, paths := range owners {
		if len(paths) == visited {
			flags[tableCell("all")+" | "+metadata] = true
			continue
		}
		for _, path := range paths {
			flags[tableCell(path)+" | "+metadata] = true
		}
	}
	return commands, flags
}

func flagMetadata(f *pflag.Flag) string {
	short := "none"
	if f.Shorthand != "" {
		short = "-" + f.Shorthand
	}
	value := f.DefValue
	if value == "" {
		value = `""`
	}
	return strings.Join([]string{tableCell("--" + f.Name), tableCell(short), tableCell(f.Value.Type()), tableCell(value)}, " | ")
}

func tableCell(text string) string { return "`" + text + "`" }

func compareRows(t *testing.T, table string, got, want map[string]bool) {
	t.Helper()
	for row := range want {
		if !got[row] {
			t.Errorf("SKILL.md %s lacks live row: | %s |", table, row)
		}
	}
	for row := range got {
		if !want[row] {
			t.Errorf("SKILL.md %s has a row the command tree does not match: | %s |", table, row)
		}
	}
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
