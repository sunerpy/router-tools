//go:build unix

package agentsetup_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/agentsetup"
)

// A named pipe where the skill keeps a file must not block the comparison: it counts as a changed file.
func TestInstallSkillDoesNotOpenPipes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "router-tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "router-tools", "SKILL.md"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md"}, root, "router-tools", false)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, agentsetup.ErrRefused) {
			t.Fatalf("a pipe in place of SKILL.md: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("InstallSkill blocked on a named pipe")
	}
}
