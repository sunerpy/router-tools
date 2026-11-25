package rd08v1196

import (
	"context"
	"errors"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

type rootFakeShell struct {
	uid    string
	bootID string
}

func (s *rootFakeShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	switch command {
	case "id -u":
		return shell.CommandResult{Stdout: []byte(s.uid), Started: true}, nil
	case "cat /proc/sys/kernel/random/boot_id":
		return shell.CommandResult{Stdout: []byte(s.bootID), Started: true}, nil
	default:
		return shell.CommandResult{ExitCode: 1, Started: true}, &shell.RemoteCommandError{}
	}
}
func (s *rootFakeShell) Close() error { return nil }

func TestRootVerifierFallsBackOnlyWhenAllowed(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("password")
	target := device.DefaultTarget()
	prober := NewProber(nil)
	telnetCalls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		return nil, &device.Error{Code: device.CodeSSHAuthFailed}
	}
	prober.telnetDial = func(context.Context, device.Target, secret.Password) (shell.Shell, error) {
		telnetCalls++
		return &rootFakeShell{uid: "0\n", bootID: "00000000-0000-0000-0000-000000000001\n"}, nil
	}
	result, err := prober.VerifyRoot(t.Context(), target, password, hostkey.Policy{}, "auto", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Transport != "telnet" || result.SSHVerified || !result.TelnetRootVerified || telnetCalls != 1 {
		t.Fatalf("result = %+v, telnet calls = %d", result, telnetCalls)
	}
}

func TestRootVerifierNeverFallsBackAfterHostKeyMismatch(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("password")
	prober := NewProber(nil)
	telnetCalls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		return nil, &device.Error{Code: device.CodeHostKeyMismatch}
	}
	prober.telnetDial = func(context.Context, device.Target, secret.Password) (shell.Shell, error) {
		telnetCalls++
		return &rootFakeShell{uid: "0"}, nil
	}
	_, err := prober.VerifyRoot(t.Context(), device.DefaultTarget(), password, hostkey.Policy{}, "auto", true)
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeHostKeyMismatch || telnetCalls != 0 {
		t.Fatalf("error = %v, telnet calls = %d", err, telnetCalls)
	}
}

func TestRootVerifierRequiresExplicitTelnet(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("password")
	prober := NewProber(nil)
	_, err := prober.VerifyRoot(t.Context(), device.DefaultTarget(), password, hostkey.Policy{}, "telnet", false)
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeUnsupportedCapability {
		t.Fatalf("error = %v", err)
	}
}
