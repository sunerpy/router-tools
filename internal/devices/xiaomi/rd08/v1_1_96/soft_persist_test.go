package rd08v1196

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/transport/shell"
)

type commandFixture struct {
	stdout string
	code   int
	err    error
}

type fakeCommandShell struct {
	commands []string
	handler  func(string) commandFixture
}

func (s *fakeCommandShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	s.commands = append(s.commands, command)
	fixture := s.handler(command)
	result := shell.CommandResult{Stdout: []byte(fixture.stdout), ExitCode: fixture.code, Started: true}
	if fixture.err != nil {
		return result, fixture.err
	}
	if fixture.code != 0 {
		return result, &shell.RemoteCommandError{Result: result}
	}
	return result, nil
}
func (s *fakeCommandShell) Close() error { return nil }

func TestAutoSSScriptVector(t *testing.T) {
	t.Parallel()

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(autoSSHScript)))
	if hash != "ac1e6149e096d731ed505b4eccf0caa67e11aa232e438064f5b35a2e1b370450" {
		t.Fatalf("script hash = %s", hash)
	}
	if !strings.HasSuffix(autoSSHScript, "\n") || strings.Contains(autoSSHScript, "\r") {
		t.Fatal("script line ending drift")
	}
	const wantBase64 = "IyEvYmluL3NoCiMgYXV0b19zc2g6IHJlc3RvcmUgZHJvcGJlYXIgYWNjZXNzIGF0IGJvb3QKc2xlZXAgNQpzZWQgLWkgJ3MvY2hhbm5lbD0uKi9jaGFubmVsPSJkZWJ1ZyIvZycgL2V0Yy9pbml0LmQvZHJvcGJlYXIgfHwgZXhpdCAxCi9ldGMvaW5pdC5kL2Ryb3BiZWFyIHJlc3RhcnQK"
	if got := base64.StdEncoding.EncodeToString([]byte(autoSSHScript)); got != wantBase64 {
		t.Fatalf("script Base64 = %s", got)
	}
}

func TestSoftInstallTransactionAndVerification(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell()
	result, err := installSoft(t.Context(), remote)
	if err != nil {
		t.Fatalf("installSoft() error = %v", err)
	}
	if !result.Verified || result.RebootVerified || result.Outcome != "installed" {
		t.Fatalf("result = %+v", result)
	}
	uciWrite := indexContaining(remote.commands, "uci set firewall.auto_ssh=include")
	move := indexContaining(remote.commands, "mv /data/auto_ssh/.auto_ssh.sh.rd08ctl.tmp")
	if move < 0 || uciWrite < move {
		t.Fatalf("UCI write happened before final file install: %v", remote.commands)
	}
}

func TestSoftInstallStopsBeforeUCIWhenSyntaxFails(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell()
	original := remote.handler
	remote.handler = func(command string) commandFixture {
		if command == "sh -n /data/auto_ssh/.auto_ssh.sh.rd08ctl.tmp" {
			return commandFixture{code: 1}
		}
		return original(command)
	}
	_, err := installSoft(t.Context(), remote)
	if err == nil {
		t.Fatal("installSoft() error = nil")
	}
	if indexContaining(remote.commands, "uci set firewall.auto_ssh=include") >= 0 {
		t.Fatalf("UCI was written after syntax failure: %v", remote.commands)
	}
}

func TestSoftInstallStopsWhenExistingHashCannotBeRead(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell(true)
	original := remote.handler
	remote.handler = func(command string) commandFixture {
		if command == "sha256sum "+autoSSHPath {
			return commandFixture{code: 2}
		}
		return original(command)
	}
	if _, err := installSoft(t.Context(), remote); err == nil {
		t.Fatal("installSoft() ignored existing-file hash failure")
	}
	for _, command := range remote.commands {
		if strings.HasPrefix(command, "mkdir") || strings.HasPrefix(command, "uci set") {
			t.Fatalf("install continued after hash read failure: %v", remote.commands)
		}
	}
}

func TestSoftInstallRejectsConflictingConfiguration(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell()
	original := remote.handler
	remote.handler = func(command string) commandFixture {
		if command == "uci -q get firewall.auto_ssh.path" {
			return commandFixture{stdout: "/user/custom.sh\n"}
		}
		return original(command)
	}
	_, err := installSoft(t.Context(), remote)
	if err == nil {
		t.Fatal("installSoft() conflict error = nil")
	}
	for _, command := range remote.commands {
		if strings.HasPrefix(command, "mkdir") || strings.HasPrefix(command, "uci set") {
			t.Fatalf("conflicting configuration was modified: %v", remote.commands)
		}
	}
}

func TestSoftInstallAlreadyInstalledDoesNotRewrite(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell(true)
	result, err := installSoft(t.Context(), remote)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "already_installed" {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	for _, command := range remote.commands {
		if strings.HasPrefix(command, "mkdir") || strings.HasPrefix(command, "mv ") || strings.HasPrefix(command, "uci set") {
			t.Fatalf("idempotent install performed writes: %v", remote.commands)
		}
	}
}

func TestSoftRemoveOnlyTouchesManagedFiles(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell(true)
	result, err := removeSoft(t.Context(), remote)
	if err != nil {
		t.Fatalf("removeSoft() error = %v", err)
	}
	if result.Outcome != "removed" {
		t.Fatalf("result = %+v", result)
	}
	for _, command := range remote.commands {
		for _, forbidden := range []string{"nvram", "bdata", "mtd ", "passwd", "dropbear stop"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("remove executed forbidden command %q", command)
			}
		}
	}
}

func TestSoftRemoveRejectsForeignPath(t *testing.T) {
	t.Parallel()

	remote := softSuccessShell(true)
	original := remote.handler
	remote.handler = func(command string) commandFixture {
		if command == "uci -q get firewall.auto_ssh.path" {
			return commandFixture{stdout: "/user/custom.sh\n"}
		}
		return original(command)
	}
	if _, err := removeSoft(t.Context(), remote); err == nil {
		t.Fatal("removeSoft() conflict error = nil")
	}
	for _, command := range remote.commands {
		if strings.HasPrefix(command, "uci delete") || strings.HasPrefix(command, "rm ") {
			t.Fatalf("foreign configuration was modified: %v", remote.commands)
		}
	}
}

func softSuccessShell(existing ...bool) *fakeCommandShell {
	values := map[string]string{}
	if len(existing) > 0 && existing[0] {
		values = map[string]string{
			"firewall.auto_ssh":         "include",
			"firewall.auto_ssh.type":    "script",
			"firewall.auto_ssh.path":    autoSSHPath,
			"firewall.auto_ssh.enabled": "1",
		}
	}
	return &fakeCommandShell{handler: func(command string) commandFixture {
		if strings.HasPrefix(command, "uci -q get ") {
			key := strings.TrimPrefix(command, "uci -q get ")
			if value, ok := values[key]; ok {
				return commandFixture{stdout: value + "\n"}
			}
			return commandFixture{code: 1}
		}
		if strings.HasPrefix(command, "uci delete firewall.auto_ssh") {
			for key := range values {
				delete(values, key)
			}
			return commandFixture{}
		}
		if strings.HasPrefix(command, "uci set ") {
			for _, segment := range strings.Split(command, " && ") {
				if !strings.HasPrefix(segment, "uci set ") {
					continue
				}
				assignment := strings.TrimPrefix(segment, "uci set ")
				parts := strings.SplitN(assignment, "=", 2)
				if len(parts) == 2 {
					values[parts[0]] = strings.Trim(parts[1], "'")
				}
			}
			return commandFixture{}
		}
		if strings.HasPrefix(command, "sha256sum ") {
			return commandFixture{stdout: "ac1e6149e096d731ed505b4eccf0caa67e11aa232e438064f5b35a2e1b370450  /data/auto_ssh/auto_ssh.sh\n"}
		}
		return commandFixture{}
	}}
}

func indexContaining(commands []string, fragment string) int {
	for index, command := range commands {
		if strings.Contains(command, fragment) {
			return index
		}
	}
	return -1
}
