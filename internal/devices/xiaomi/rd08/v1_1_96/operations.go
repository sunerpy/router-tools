package rd08v1196

import (
	"context"
	"errors"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/workflow"
)

// InstallSoft opens a verified root shell and executes the soft-install transaction.
func (p *Prober) InstallSoft(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, allowTelnet bool) (app.SoftResult, error) {
	remote, _, err := p.openRootShell(ctx, target, password, policy, allowTelnet)
	if err != nil {
		return app.SoftResult{}, err
	}
	defer func() { _ = remote.Close() }()
	return installSoft(ctx, remote)
}

// RemoveSoft removes only this adapter's managed auto_ssh state.
func (p *Prober) RemoveSoft(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, allowTelnet bool) (app.SoftResult, error) {
	remote, _, err := p.openRootShell(ctx, target, password, policy, allowTelnet)
	if err != nil {
		return app.SoftResult{}, err
	}
	defer func() { _ = remote.Close() }()
	return removeSoft(ctx, remote)
}

// CreateBackup uses SSH only and binds the manifest to the current boot_id.
func (p *Prober) CreateBackup(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, options backup.CreateOptions) (backup.Manifest, error) {
	remote, err := p.sshDial(ctx, target, password, policy)
	if err != nil {
		return backup.Manifest{}, err
	}
	defer func() { _ = remote.Close() }()
	if _, err := shell.VerifyRoot(ctx, remote); err != nil {
		return backup.Manifest{}, err
	}
	bootID, err := workflow.ReadBootID(ctx, remote)
	if err != nil {
		return backup.Manifest{}, err
	}
	streaming, ok := remote.(shell.StreamingShell)
	if !ok {
		return backup.Manifest{}, &device.Error{Code: device.CodeUnsupportedCapability, Message: "SSH transport does not support binary streaming"}
	}
	options.BootID = bootID
	return backup.Create(ctx, streaming, options)
}

// VerifyBackupOnline compares current read-only identity, MTD layout, and boot_id with a backup.
func (p *Prober) VerifyBackupOnline(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, manifest backup.Manifest, observation device.Observation, profileID string) error {
	remote, err := p.sshDial(ctx, target, password, policy)
	if err != nil {
		return err
	}
	defer func() { _ = remote.Close() }()
	if _, err := shell.VerifyRoot(ctx, remote); err != nil {
		return err
	}
	bootID, err := workflow.ReadBootID(ctx, remote)
	if err != nil {
		return err
	}
	mtdResult, err := remote.Run(ctx, "cat /proc/mtd", shell.RunOptions{})
	if err != nil {
		return err
	}
	return backup.VerifyOnline(manifest, observation, profileID, string(mtdResult.Stdout), bootID)
}

// RebootVerify proves the installed soft configuration across a real reboot.
func (p *Prober) RebootVerify(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, allowTelnet bool) (app.RebootResult, error) {
	result := app.RebootResult{}
	remote, _, err := p.openRootShell(ctx, target, password, policy, allowTelnet)
	if err != nil {
		return result, err
	}
	if err := verifySoft(ctx, remote); err != nil {
		_ = remote.Close()
		return result, err
	}
	reboot, err := workflow.RebootAndWait(ctx, remote, p.rebootConfig(target, password, policy, false))
	if err != nil {
		return result, err
	}
	defer func() { _ = reboot.Shell.Close() }()
	if err := verifySoft(ctx, reboot.Shell); err != nil {
		return result, err
	}
	result.OldBootID = reboot.OldBootID
	result.NewBootID = reboot.NewBootID
	result.SSHVerified = true
	result.SoftVerified = true
	return result, nil
}

func (p *Prober) openRootShell(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, allowTelnet bool) (shell.Shell, string, error) {
	remote, err := p.sshDial(ctx, target, password, policy)
	if err == nil {
		if _, verifyErr := shell.VerifyRoot(ctx, remote); verifyErr == nil {
			return remote, "ssh", nil
		} else {
			_ = remote.Close()
			err = verifyErr
		}
	}
	var deviceErr *device.Error
	if errors.As(err, &deviceErr) && (deviceErr.Code == device.CodeHostKeyMismatch || deviceErr.Code == device.CodeSSHHostKeyRejected) {
		return nil, "", err
	}
	if !allowTelnet || !errors.As(err, &deviceErr) || (deviceErr.Code != device.CodeSSHUnreachable && deviceErr.Code != device.CodeSSHAuthFailed) {
		return nil, "", err
	}
	remote, err = p.telnetDial(ctx, target, password)
	if err != nil {
		return nil, "", err
	}
	if _, err := shell.VerifyRoot(ctx, remote); err != nil {
		_ = remote.Close()
		return nil, "", err
	}
	return remote, "telnet", nil
}

func (p *Prober) rebootConfig(target device.Target, password secret.Password, policy hostkey.Policy, allowTelnet bool) workflow.RebootConfig {
	return workflow.RebootConfig{
		Sleep: p.sleep,
		PortsDown: func(ctx context.Context) (bool, error) {
			for _, port := range []int{target.WebPort, target.SSHPort, target.TelnetPort} {
				if p.probePort(ctx, target.Host, port) == device.PortOpen {
					return false, nil
				}
			}
			return true, nil
		},
		Connect: func(ctx context.Context) (shell.Shell, error) {
			if allowTelnet {
				remote, _, err := p.openRootShell(ctx, target, password, policy, true)
				return remote, err
			}
			remote, err := p.sshDial(ctx, target, password, policy)
			if err != nil {
				return nil, err
			}
			if _, err := shell.VerifyRoot(ctx, remote); err != nil {
				_ = remote.Close()
				return nil, err
			}
			return remote, nil
		},
		DownRounds:    p.downRounds,
		OnlineTimeout: p.onlineTimeout,
	}
}
