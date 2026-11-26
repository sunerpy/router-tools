package skills_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/skills"
)

// The embedded skill follows the Agent Skills format: a SKILL.md whose YAML frontmatter starts on the first
// line, with a lowercase hyphenated name equal to the directory name and a description within 1024
// characters.
func TestEmbeddedSkillIsWellFormed(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(skills.RouterTools(), "SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatal("SKILL.md must start with YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		t.Fatal("unterminated frontmatter")
	}
	frontmatter := text[4 : 4+end]
	name := regexp.MustCompile(`(?m)^name: (.+)$`).FindStringSubmatch(frontmatter)
	if name == nil || name[1] != skills.Name || !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(name[1]) || len(name[1]) > 64 {
		t.Fatalf("name = %v", name)
	}
	description := frontmatter[strings.Index(frontmatter, "description:"):]
	description = strings.TrimSpace(strings.TrimPrefix(description, "description:"))
	description = strings.TrimSpace(strings.TrimPrefix(description, ">-"))
	description = strings.Join(strings.Fields(description), " ")
	if description == "" || len([]rune(description)) > 1024 {
		t.Fatalf("description has %d characters", len([]rune(description)))
	}
	for _, tool := range []string{"guide_plan", "guide_apply", "operation_wait", "use_saved_password", "RECOVERY_REVIEW_REQUIRED"} {
		if !strings.Contains(text, tool) {
			t.Fatalf("SKILL.md does not mention %s", tool)
		}
	}
	// The guidance a first run needs: the host-key decision before enable_ssh writes, the plan that waits for
	// the current root password, and the recovery command for the router the agent worked on.
	guided := section(t, text, "## Guided setup", "## Passwords and host keys")
	for _, part := range []string{"`accept_new_host_key: true`", "`needs_input`", "`reset_root_password: true`", "`enable_ssh`"} {
		if !strings.Contains(guided, part) {
			t.Fatalf("the guided setup does not mention %s", part)
		}
	}
	for _, part := range []string{"routerctl --host <address> recovery ack", "`SSH_HOST_KEY_REJECTED`", "`HOST_KEY_MISMATCH`"} {
		if !strings.Contains(section(t, text, "## Results and errors", "## Never"), part) {
			t.Fatalf("the results table does not mention %s", part)
		}
	}
	files, err := skills.Files()
	if err != nil || len(files) == 0 || files[0] != "SKILL.md" {
		t.Fatalf("Files() = %v, %v", files, err)
	}
}

func section(t *testing.T, text, start, end string) string {
	t.Helper()
	from := strings.Index(text, start)
	to := strings.Index(text, end)
	if from < 0 || to < from {
		t.Fatalf("SKILL.md lacks the section %q before %q", start, end)
	}
	return text[from:to]
}
