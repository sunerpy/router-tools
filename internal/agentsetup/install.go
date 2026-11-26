package agentsetup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// skillOverrides are the documented variables that move a host's user skills directory to <value>/skills:
// CLAUDE_CONFIG_DIR replaces ~/.claude for Claude Code, and DSH_AGENTS_HOME replaces ~/.agents for dsh.
var skillOverrides = map[string]string{ClaudeCode: "CLAUDE_CONFIG_DIR", DSH: "DSH_AGENTS_HOME"}

// SkillRoot returns the user skills directory of an agent: below home, or below the host's documented override
// variable when getenv reports it. Agents whose skills directory is not verified (WorkBuddy, generic clients)
// and relative override values are errors that point to --dir.
func SkillRoot(agent, home string, getenv func(string) string) (string, error) {
	found, ok := Lookup(agent)
	if !ok {
		return "", fmt.Errorf("unknown agent %q; use one of %s", agent, IDs())
	}
	if found.SkillDir == "" {
		return "", fmt.Errorf("%s has no documented skills directory; install with --dir <directory> and add that folder in %s", found.Name, found.Name)
	}
	if variable, ok := skillOverrides[agent]; ok && getenv != nil {
		if dir := getenv(variable); dir != "" {
			if !filepath.IsAbs(dir) {
				return "", fmt.Errorf("%s is not an absolute path; install with --dir <directory> instead", variable)
			}
			return filepath.Join(dir, "skills"), nil
		}
	}
	return filepath.Join(home, found.SkillDir), nil
}

// ErrRefused marks an installation refused for safety: an installed copy that differs without force, or a
// symbolic link or non-directory where the skill would be written. Other errors come from the file system.
var ErrRefused = errors.New("skill installation refused")

type refusal string

func (r refusal) Error() string { return string(r) }

func (refusal) Is(target error) bool { return target == ErrRefused }

func refuse(format string, arguments ...any) error { return refusal(fmt.Sprintf(format, arguments...)) }

// InstallResult reports a skill installation.
type InstallResult struct {
	Path   string   `json:"path"`
	Action string   `json:"action"`
	Files  []string `json:"files"`
}

// InstallSkill writes the skill's files to root/name. A new skill is written to a temporary directory and
// renamed into place. An identical skill is left alone. A skill whose shipped files differ is refused unless
// force is set; force rewrites only the files the skill ships and keeps any others. Symbolic links and
// non-directories in the way are refused.
func InstallSkill(source fs.FS, files []string, root, name string, force bool) (InstallResult, error) {
	for _, file := range files {
		if !fs.ValidPath(file) || strings.HasPrefix(path.Clean(file), "..") {
			return InstallResult{}, fmt.Errorf("invalid skill file %q", file)
		}
	}
	target := filepath.Join(root, name)
	info, err := os.Lstat(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return installNew(source, files, root, target, name)
	case err != nil:
		return InstallResult{}, err
	case info.Mode()&fs.ModeSymlink != 0:
		return InstallResult{}, refuse("%s is a symbolic link; remove it or install with --dir somewhere else", target)
	case !info.IsDir():
		return InstallResult{}, refuse("%s exists and is not a directory", target)
	}

	var changed []string
	for _, file := range files {
		want, err := fs.ReadFile(source, file)
		if err != nil {
			return InstallResult{}, err
		}
		if !sameRegularFile(filepath.Join(target, filepath.FromSlash(file)), want) {
			changed = append(changed, file)
		}
	}
	if len(changed) == 0 {
		return InstallResult{Path: target, Action: "unchanged", Files: files}, nil
	}
	if !force {
		return InstallResult{}, refuse("%s differs from the skill this routerctl ships (%s); rerun with --force to replace those files", target, strings.Join(changed, ", "))
	}
	for _, file := range changed {
		if err := writeInside(source, target, file); err != nil {
			return InstallResult{}, err
		}
	}
	return InstallResult{Path: target, Action: "updated", Files: changed}, nil
}

// sameRegularFile reports whether path is a regular file (or a link to one) holding want. Anything else counts as
// changed without being opened, so a named pipe cannot block the comparison.
func sameRegularFile(path string, want []byte) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	have, err := os.ReadFile(path)
	return err == nil && bytes.Equal(have, want)
}

func installNew(source fs.FS, files []string, root, target, name string) (InstallResult, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return InstallResult{}, err
	}
	temporary, err := os.MkdirTemp(root, "."+name+"-")
	if err != nil {
		return InstallResult{}, err
	}
	for _, file := range files {
		if err := writeInside(source, temporary, file); err != nil {
			_ = os.RemoveAll(temporary)
			return InstallResult{}, err
		}
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		_ = os.RemoveAll(temporary)
		return InstallResult{}, err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.RemoveAll(temporary)
		return InstallResult{}, err
	}
	return InstallResult{Path: target, Action: "installed", Files: files}, nil
}

// writeInside writes one file below dir through a temporary file and a rename, refusing symbolic links on
// the way.
func writeInside(source fs.FS, dir, file string) error {
	data, err := fs.ReadFile(source, file)
	if err != nil {
		return err
	}
	current := dir
	parts := strings.Split(file, "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Mkdir(current, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case info.Mode()&fs.ModeSymlink != 0 || !info.IsDir():
			return refuse("%s is not a plain directory", current)
		}
	}
	destination := filepath.Join(dir, filepath.FromSlash(file))
	if info, err := os.Lstat(destination); err == nil && (info.Mode()&fs.ModeSymlink != 0 || info.IsDir()) {
		return refuse("%s is not a regular file", destination)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-")
	if err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), destination); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}
