package rd08v1196

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
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

// WriteStarted is set before the first device write and never for paths that stop before one.
func TestSoftWriteEvidence(t *testing.T) {
	t.Parallel()

	failing := func(existing bool, failCommand string) *fakeCommandShell {
		remote := softSuccessShell(existing)
		original := remote.handler
		remote.handler = func(command string) commandFixture {
			if command == failCommand {
				return commandFixture{code: 1}
			}
			return original(command)
		}
		return remote
	}
	cases := []struct {
		name  string
		run   func() (bool, error)
		write bool
	}{
		{"install precheck fails", func() (bool, error) {
			r, err := installSoft(t.Context(), failing(false, "test -f /etc/init.d/dropbear"))
			return r.WriteStarted, err
		}, false},
		{"install conflict", func() (bool, error) {
			remote := softSuccessShell()
			original := remote.handler
			remote.handler = func(command string) commandFixture {
				if command == "uci -q get firewall.auto_ssh.path" {
					return commandFixture{stdout: "/user/custom.sh\n"}
				}
				return original(command)
			}
			r, err := installSoft(t.Context(), remote)
			return r.WriteStarted, err
		}, false},
		{"install already installed", func() (bool, error) {
			r, err := installSoft(t.Context(), softSuccessShell(true))
			return r.WriteStarted, err
		}, false},
		{"install fails inside the transaction", func() (bool, error) {
			r, err := installSoft(t.Context(), failing(false, "sh -n "+autoSSHTemp))
			return r.WriteStarted, err
		}, true},
		{"install succeeds", func() (bool, error) {
			r, err := installSoft(t.Context(), softSuccessShell())
			return r.WriteStarted, err
		}, true},
		{"remove foreign path", func() (bool, error) {
			remote := softSuccessShell(true)
			original := remote.handler
			remote.handler = func(command string) commandFixture {
				if command == "uci -q get firewall.auto_ssh.path" {
					return commandFixture{stdout: "/user/custom.sh\n"}
				}
				return original(command)
			}
			r, err := removeSoft(t.Context(), remote)
			return r.WriteStarted, err
		}, false},
		{"remove not installed", func() (bool, error) {
			r, err := removeSoft(t.Context(), softSuccessShell())
			return r.WriteStarted, err
		}, false},
		{"remove fails at the UCI delete", func() (bool, error) {
			r, err := removeSoft(t.Context(), failing(true, "uci delete firewall.auto_ssh && uci commit firewall"))
			return r.WriteStarted, err
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			started, _ := tc.run()
			if started != tc.write {
				t.Fatalf("WriteStarted = %v, want %v", started, tc.write)
			}
		})
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

// RebootVerify carries the workflow's write evidence: a reboot that was submitted stays a possible write
// even when the reconnect then fails, and a failure before the reboot is reported as no write.
func TestRebootVerifyReportsWhetherTheRebootWasSubmitted(t *testing.T) {
	t.Parallel()

	rootShell := func() *fakeCommandShell {
		soft := softSuccessShell(true)
		return &fakeCommandShell{handler: func(command string) commandFixture {
			switch command {
			case "id -u":
				return commandFixture{stdout: "0\n"}
			case "cat /proc/sys/kernel/random/boot_id":
				return commandFixture{stdout: "00000000-0000-0000-0000-000000000001\n"}
			}
			return soft.handler(command)
		}}
	}
	password, _ := secret.NewPassword("root")

	t.Run("reboot submitted, then the host key changed", func(t *testing.T) {
		t.Parallel()
		first := rootShell()
		prober := NewProber(func(context.Context, string, int) device.PortState { return device.PortClosed })
		prober.sleep = func(time.Duration) {}
		dials := 0
		prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
			dials++
			if dials == 1 {
				return first, nil
			}
			return nil, &device.Error{Code: device.CodeHostKeyMismatch, Message: "host key changed"}
		}
		result, err := prober.RebootVerify(t.Context(), device.DefaultTarget(), password, hostkey.Policy{}, false)
		var deviceErr *device.Error
		if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeHostKeyMismatch || !result.WriteStarted {
			t.Fatalf("RebootVerify() = %+v, %v; want HOST_KEY_MISMATCH with the reboot recorded as submitted", result, err)
		}
		if countCommand(first.commands, "reboot") != 1 {
			t.Fatalf("commands = %v, want exactly one reboot", first.commands)
		}
	})
	t.Run("soft check fails before the reboot", func(t *testing.T) {
		t.Parallel()
		broken := &fakeCommandShell{handler: func(command string) commandFixture {
			if command == "id -u" {
				return commandFixture{stdout: "0\n"}
			}
			return commandFixture{code: 1}
		}}
		prober := NewProber(func(context.Context, string, int) device.PortState { return device.PortClosed })
		prober.sleep = func(time.Duration) {}
		prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
			return broken, nil
		}
		result, err := prober.RebootVerify(t.Context(), device.DefaultTarget(), password, hostkey.Policy{}, false)
		if err == nil || result.WriteStarted || countCommand(broken.commands, "reboot") != 0 {
			t.Fatalf("RebootVerify() = %+v, %v, commands %v; want a failure with no reboot", result, err, broken.commands)
		}
	})
}
