package rd08v1196

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

var (
	_ app.SoftInspector = (*Prober)(nil)
	_ app.DeepInspector = (*Prober)(nil)
)

// InspectSoft reads the managed soft-persistence state over SSH only, using fixed read-only commands.
func (p *Prober) InspectSoft(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy) (app.SoftState, error) {
	remote, err := p.inspectionShell(ctx, target, password, policy)
	if err != nil {
		return "", err
	}
	defer func() { _ = remote.Close() }()
	return inspectSoft(ctx, remote)
}

// InspectDeep reads the deep-persistence state over SSH only, using fixed read-only commands.
func (p *Prober) InspectDeep(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy) (app.DeepInspection, error) {
	remote, err := p.inspectionShell(ctx, target, password, policy)
	if err != nil {
		return app.DeepInspection{}, err
	}
	defer func() { _ = remote.Close() }()
	return inspectDeep(ctx, remote)
}

// inspectDeep reads the boot flags and checks the crash partition with the same read-only commands the
// deep precheck runs (cat /proc/mtd, test -r /dev/mtd<N>).
func inspectDeep(ctx context.Context, remote shell.Shell) (app.DeepInspection, error) {
	flags, err := inspectDeepFlags(ctx, remote)
	if err != nil {
		return app.DeepInspection{}, err
	}
	inspection := app.DeepInspection{Flags: flags}
	layout, err := remote.Run(ctx, "cat /proc/mtd", shell.RunOptions{})
	if err != nil {
		return app.DeepInspection{}, device.WrapError(device.CodeRemoteCommandFailed, "cannot read the MTD layout", err)
	}
	partitions, err := backup.ParseMTD(string(layout.Stdout))
	if err != nil {
		return app.DeepInspection{}, err
	}
	for _, partition := range partitions {
		if partition.Name != "crash" {
			continue
		}
		inspection.CrashPartition = true
		_, err := remote.Run(ctx, fmt.Sprintf("test -r /dev/mtd%d", partition.Index), shell.RunOptions{})
		var remoteErr *shell.RemoteCommandError
		switch {
		case err == nil:
			inspection.CrashReadable = true
		case !errors.As(err, &remoteErr):
			return app.DeepInspection{}, device.WrapError(device.CodeRemoteCommandFailed, "cannot check the crash partition", err)
		}
		break
	}
	return inspection, nil
}

func (p *Prober) inspectionShell(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy) (shell.Shell, error) {
	remote, err := p.sshDial(ctx, target, password, policy)
	if err != nil {
		return nil, err
	}
	if _, err := shell.VerifyRoot(ctx, remote); err != nil {
		_ = remote.Close()
		return nil, err
	}
	return remote, nil
}

// inspectSoft mirrors the decisions installSoft makes, without writing anything.
func inspectSoft(ctx context.Context, remote shell.Shell) (app.SoftState, error) {
	values, err := readSoftUCI(ctx, remote)
	if err != nil {
		return "", err
	}
	path := values["firewall.auto_ssh.path"]
	anyConfig := false
	for _, value := range values {
		anyConfig = anyConfig || value != ""
	}
	full := softUCIMatches(values)
	switch {
	case path != "" && path != autoSSHPath:
		return app.SoftConflict, nil
	case anyConfig && !full && path == autoSSHPath:
		return app.SoftUCIDrift, nil
	case anyConfig && !full:
		return app.SoftConflict, nil
	}
	hash, err := remoteHash(ctx, remote, autoSSHPath)
	if err != nil {
		return "", err
	}
	if !full {
		if hash != "" {
			return app.SoftOrphanFile, nil
		}
		return app.SoftNotInstalled, nil
	}
	if hash != autoSSHHash {
		return app.SoftFileDrift, nil
	}
	for _, command := range []string{"test -x " + autoSSHPath, "sh -n " + autoSSHPath} {
		if _, err := remote.Run(ctx, command, shell.RunOptions{}); err != nil {
			var remoteErr *shell.RemoteCommandError
			if errors.As(err, &remoteErr) {
				return app.SoftModeDrift, nil
			}
			return "", err
		}
	}
	return app.SoftInstalledVerified, nil
}

func inspectDeepFlags(ctx context.Context, remote shell.Shell) (app.DeepFlags, error) {
	expected := map[string]string{"ssh_en": "1", "telnet_en": "1", "uart_en": "1", "boot_wait": "on"}
	matched, total := 0, 0
	for _, source := range []string{"bdata", "nvram"} {
		for _, key := range []string{"ssh_en", "telnet_en", "uart_en", "boot_wait"} {
			result, err := remote.Run(ctx, source+" get "+key, shell.RunOptions{})
			total++
			var remoteErr *shell.RemoteCommandError
			if errors.As(err, &remoteErr) {
				continue // an unset key reads as not set
			}
			if err != nil {
				return "", device.WrapError(device.CodeRemoteCommandFailed, "cannot read the deep-persistence flags", err)
			}
			if strings.TrimSpace(string(result.Stdout)) == expected[key] {
				matched++
			}
		}
	}
	switch matched {
	case total:
		return app.DeepFlagsAllSet, nil
	case 0:
		return app.DeepFlagsNone, nil
	default:
		return app.DeepFlagsMixed, nil
	}
}
