package rd08v1196

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/transport/sshtransport"
	"github.com/sunerpy/router-tools/internal/transport/telnettransport"
)

// SSHDialFunc and TelnetDialFunc allow loopback/fake shell contract tests.
type (
	SSHDialFunc    func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error)
	TelnetDialFunc func(context.Context, device.Target, secret.Password) (shell.Shell, error)
)

func dialSSH(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy) (shell.Shell, error) {
	callback, err := policy.Callback(ctx)
	if err != nil {
		return nil, err
	}
	return sshtransport.Dial(ctx, sshtransport.Config{Host: target.Host, Port: target.SSHPort, Password: password, HostKeyCallback: callback})
}

func dialTelnet(ctx context.Context, target device.Target, password secret.Password) (shell.Shell, error) {
	address := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.TelnetPort))
	return telnettransport.Dial(ctx, telnettransport.Config{Address: address, Password: password})
}

// VerifyRoot applies the restricted SSH -> Telnet fallback policy.
func (p *Prober) VerifyRoot(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, transport string, allowTelnet bool) (app.RootVerification, error) {
	switch transport {
	case "ssh":
		return p.verifySSH(ctx, target, password, policy)
	case "telnet":
		if !allowTelnet {
			return app.RootVerification{}, &device.Error{Code: device.CodeUnsupportedCapability, Message: "Telnet requires explicit permission"}
		}
		return p.verifyTelnet(ctx, target, password)
	case "auto":
		result, err := p.verifySSH(ctx, target, password, policy)
		if err == nil {
			return result, nil
		}
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) && (deviceErr.Code == device.CodeHostKeyMismatch || deviceErr.Code == device.CodeSSHHostKeyRejected) {
			return app.RootVerification{}, err
		}
		if !allowTelnet || !errors.As(err, &deviceErr) || (deviceErr.Code != device.CodeSSHUnreachable && deviceErr.Code != device.CodeSSHAuthFailed) {
			return app.RootVerification{}, err
		}
		return p.verifyTelnet(ctx, target, password)
	default:
		return app.RootVerification{}, &device.Error{Code: device.CodeInvalidArgument, Message: "unknown shell transport"}
	}
}

func (p *Prober) verifySSH(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy) (app.RootVerification, error) {
	remote, err := p.sshDial(ctx, target, password, policy)
	if err != nil {
		return app.RootVerification{}, err
	}
	result, err := verifyConnectedRoot(ctx, remote, "ssh")
	result.SSHVerified = result.RootVerified
	return result, err
}

func (p *Prober) verifyTelnet(ctx context.Context, target device.Target, password secret.Password) (app.RootVerification, error) {
	remote, err := p.telnetDial(ctx, target, password)
	if err != nil {
		return app.RootVerification{}, err
	}
	result, err := verifyConnectedRoot(ctx, remote, "telnet")
	result.TelnetRootVerified = result.RootVerified
	return result, err
}

func verifyConnectedRoot(ctx context.Context, remote shell.Shell, transport string) (app.RootVerification, error) {
	defer func() { _ = remote.Close() }()
	result := app.RootVerification{Transport: transport}
	root, err := shell.VerifyRoot(ctx, remote)
	if err != nil {
		return result, err
	}
	result.RootVerified = root
	boot, err := remote.Run(ctx, "cat /proc/sys/kernel/random/boot_id", shell.RunOptions{})
	if err == nil && boot.ExitCode == 0 {
		result.BootID = strings.TrimSpace(string(boot.Stdout))
	}
	return result, nil
}
