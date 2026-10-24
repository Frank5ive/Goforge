package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Frank5ive/goforge/internal/domain"
)

// The agent skill is documentation agents follow literally, so it must not
// drift from the CLI: these tests check its metadata, and that every command,
// flag, and error code it mentions exists.

func readSkill(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "skills", "goforge", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestSkillMetadata(t *testing.T) {
	skill := readSkill(t)
	front, _, ok := strings.Cut(strings.TrimPrefix(skill, "---\n"), "\n---\n")
	if !strings.HasPrefix(skill, "---\n") || !ok {
		t.Fatal("SKILL.md must start with a --- frontmatter block")
	}

	fields := map[string]string{}
	for _, line := range strings.Split(front, "\n") {
		key, value, _ := strings.Cut(line, ": ")
		fields[key] = value
	}
	if fields["name"] != "goforge" {
		t.Errorf("name = %q, want goforge (the directory name)", fields["name"])
	}
	// Agents decide whether to load a skill from its description alone
	if n := len(fields["description"]); n == 0 || n > 1024 {
		t.Errorf("description is %d characters, want 1 to 1024", n)
	}
	for _, trigger := range []string{"REST API", ".goforge.json", "PostgreSQL"} {
		if !strings.Contains(fields["description"], trigger) {
			t.Errorf("description doesn't mention %q, so agents may not pick the skill", trigger)
		}
	}
}

func TestSkillCommandsAndFlagsExist(t *testing.T) {
	skill := readSkill(t)
	root := newRootCmd()

	flags := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = true })
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	flags["help"], flags["version"] = true, true

	for _, m := range regexp.MustCompile(`--([a-z][a-z-]*)`).FindAllStringSubmatch(skill, -1) {
		if !flags[m[1]] {
			t.Errorf("SKILL.md mentions --%s, which no goforge command has", m[1])
		}
	}

	// Commands in code spans or at the start of a line, not prose like "goforge did"
	for _, m := range regexp.MustCompile("(?m)(?:^|`)goforge ((?:add )?[a-z]+)").FindAllStringSubmatch(skill, -1) {
		if found, _, err := root.Find(strings.Fields(m[1])); err != nil || found == root {
			t.Errorf("SKILL.md mentions `goforge %s`, which isn't a command", m[1])
		}
	}
}

func TestSkillListsEveryErrorCode(t *testing.T) {
	skill := readSkill(t)
	for _, code := range []string{
		codeUsage, domain.CodeInvalidInput, domain.CodeNotAProject, domain.CodeAlreadyExists,
		domain.CodeMarkerMissing, domain.CodeOutdatedProject, domain.CodeDependencyFailed,
		domain.CodeWriteFailed, codeCancelled, domain.CodeInternal,
	} {
		if !strings.Contains(skill, "`"+code+"`") {
			t.Errorf("SKILL.md doesn't list the error code %s", code)
		}
	}
}

// The example in the skill is the one agents will copy, so it must run.
func TestSkillExampleParses(t *testing.T) {
	skill := readSkill(t)
	example := regexp.MustCompile("(?s)```bash\\n(goforge generate.*?)\n```").FindStringSubmatch(skill)
	if example == nil {
		t.Fatal("SKILL.md has no goforge generate example")
	}
	line := strings.ReplaceAll(strings.ReplaceAll(example[1], "\\\n", " "), "\n", " ")
	var resources []string
	for _, m := range regexp.MustCompile(`--resource "([^"]+)"`).FindAllStringSubmatch(line, -1) {
		resources = append(resources, m[1])
	}
	if len(resources) < 2 {
		t.Fatalf("example has %d resources, want at least 2", len(resources))
	}
	for _, spec := range resources {
		rc, err := domain.ParseResourceSpec(spec)
		if err != nil {
			t.Fatalf("--resource %q: %v", spec, err)
		}
		rc.ProjectDir = "."
		if err := rc.Validate(); err != nil {
			t.Errorf("--resource %q: %v", spec, err)
		}
	}
}
