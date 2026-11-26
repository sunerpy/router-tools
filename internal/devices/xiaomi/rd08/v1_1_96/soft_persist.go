package rd08v1196

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

const (
	autoSSHPath   = "/data/auto_ssh/auto_ssh.sh"
	autoSSHTemp   = "/data/auto_ssh/.auto_ssh.sh.rd08ctl.tmp"
	autoSSHHash   = "ac1e6149e096d731ed505b4eccf0caa67e11aa232e438064f5b35a2e1b370450"
	autoSSHScript = "#!/bin/sh\n" +
		"# auto_ssh: restore dropbear access at boot\n" +
		"sleep 5\n" +
		"sed -i 's/channel=.*/channel=\"debug\"/g' /etc/init.d/dropbear || exit 1\n" +
		"/etc/init.d/dropbear restart\n"
)

var expectedUCI = map[string]string{
	"firewall.auto_ssh":         "include",
	"firewall.auto_ssh.type":    "script",
	"firewall.auto_ssh.path":    autoSSHPath,
	"firewall.auto_ssh.enabled": "1",
}

func installSoft(ctx context.Context, remote shell.Shell) (app.SoftResult, error) {
	result := app.SoftResult{}
	if err := runChecked(ctx, remote, "command -v base64 && command -v sha256sum && command -v uci && command -v sed && command -v sh && command -v chmod && command -v mv"); err != nil {
		return result, err
	}
	if err := runChecked(ctx, remote, "test -f /etc/init.d/dropbear"); err != nil {
		return result, err
	}
	values, err := readSoftUCI(ctx, remote)
	if err != nil {
		return result, err
	}
	fullConfig := softUCIMatches(values)
	anyConfig := false
	for _, value := range values {
		anyConfig = anyConfig || value != ""
	}
	if values["firewall.auto_ssh.path"] != "" && values["firewall.auto_ssh.path"] != autoSSHPath {
		return result, &device.Error{Code: device.CodeSoftConfigConflict, Message: "firewall.auto_ssh points to a user-managed path"}
	}
	if anyConfig && !fullConfig {
		return result, &device.Error{Code: device.CodeSoftConfigConflict, Message: "firewall.auto_ssh exists with an unrecognized configuration"}
	}
	currentHash, err := remoteHash(ctx, remote, autoSSHPath)
	if err != nil {
		return result, err
	}
	if fullConfig && currentHash == autoSSHHash {
		if err := verifySoft(ctx, remote); err != nil {
			return result, err
		}
		result.Outcome = "already_installed"
		result.Verified = true
		return result, nil
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(autoSSHScript))
	result.WriteStarted = true
	transaction := []string{
		"mkdir -p /data/auto_ssh",
		"printf '%s' '" + encoded + "' | base64 -d > " + autoSSHTemp,
		"sh -n " + autoSSHTemp,
	}
	for _, command := range transaction {
		if err := runChecked(ctx, remote, command); err != nil {
			return result, err
		}
	}
	tempHash, err := remoteHashRequired(ctx, remote, autoSSHTemp)
	if err != nil {
		return result, err
	}
	if tempHash != autoSSHHash {
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "temporary auto_ssh script hash mismatch"}
	}
	for _, command := range []string{
		"chmod 0755 " + autoSSHTemp,
		"mv " + autoSSHTemp + " " + autoSSHPath,
		"rm -f " + autoSSHTemp,
		"uci set firewall.auto_ssh=include && uci set firewall.auto_ssh.type='script' && uci set firewall.auto_ssh.path='" + autoSSHPath + "' && uci set firewall.auto_ssh.enabled='1' && uci commit firewall",
	} {
		if err := runChecked(ctx, remote, command); err != nil {
			return result, err
		}
	}
	if err := verifySoft(ctx, remote); err != nil {
		return result, err
	}
	result.Outcome = "installed"
	result.Verified = true
	return result, nil
}

func verifySoft(ctx context.Context, remote shell.Shell) error {
	for _, command := range []string{"test -x " + autoSSHPath, "sh -n " + autoSSHPath} {
		if err := runChecked(ctx, remote, command); err != nil {
			return err
		}
	}
	hash, err := remoteHashRequired(ctx, remote, autoSSHPath)
	if err != nil {
		return err
	}
	if hash != autoSSHHash {
		return &device.Error{Code: device.CodeRemoteCommandFailed, Message: "installed auto_ssh script hash mismatch"}
	}
	values, err := readSoftUCI(ctx, remote)
	if err != nil {
		return err
	}
	if !softUCIMatches(values) {
		return &device.Error{Code: device.CodeSoftConfigConflict, Message: "installed firewall.auto_ssh values do not match"}
	}
	return nil
}

func removeSoft(ctx context.Context, remote shell.Shell) (app.SoftResult, error) {
	result := app.SoftResult{}
	pathResult, err := remote.Run(ctx, "uci -q get firewall.auto_ssh.path", shell.RunOptions{})
	if err != nil {
		var remoteErr *shell.RemoteCommandError
		if errors.As(err, &remoteErr) && pathResult.ExitCode == 1 {
			result.Outcome = "not_installed"
			result.Verified = true
			return result, nil
		}
		return result, err
	}
	if strings.TrimSpace(string(pathResult.Stdout)) != autoSSHPath {
		return result, &device.Error{Code: device.CodeSoftConfigConflict, Message: "refusing to remove a foreign firewall include path"}
	}
	result.WriteStarted = true
	for _, command := range []string{
		"uci delete firewall.auto_ssh && uci commit firewall",
		"rm -f " + autoSSHPath + " " + autoSSHTemp,
	} {
		if err := runChecked(ctx, remote, command); err != nil {
			return result, err
		}
	}
	if err := runChecked(ctx, remote, "rmdir /data/auto_ssh"); err != nil {
		result.Warnings = append(result.Warnings, "/data/auto_ssh was not empty and was retained")
	}
	remaining, err := remote.Run(ctx, "uci -q get firewall.auto_ssh.path", shell.RunOptions{})
	if err == nil {
		return result, &device.Error{Code: device.CodeSoftConfigConflict, Message: "firewall.auto_ssh.path still exists after removal"}
	}
	var remoteErr *shell.RemoteCommandError
	if !errors.As(err, &remoteErr) || remaining.ExitCode != 1 {
		return result, err
	}
	if err := runChecked(ctx, remote, "test ! -e "+autoSSHPath+" && test ! -e "+autoSSHTemp); err != nil {
		return result, err
	}
	result.Outcome = "removed"
	result.Verified = true
	return result, nil
}

func readSoftUCI(ctx context.Context, remote shell.Shell) (map[string]string, error) {
	values := make(map[string]string, len(expectedUCI))
	for key := range expectedUCI {
		result, err := remote.Run(ctx, "uci -q get "+key, shell.RunOptions{})
		if err != nil {
			var remoteErr *shell.RemoteCommandError
			if errors.As(err, &remoteErr) && result.ExitCode == 1 {
				values[key] = ""
				continue
			}
			return nil, device.WrapError(device.CodeRemoteCommandFailed, "cannot read firewall.auto_ssh", err)
		}
		values[key] = strings.TrimSpace(string(result.Stdout))
	}
	return values, nil
}

func softUCIMatches(values map[string]string) bool {
	for key, expected := range expectedUCI {
		if values[key] != expected {
			return false
		}
	}
	return true
}

func remoteHash(ctx context.Context, remote shell.Shell, path string) (string, error) {
	exists, err := remote.Run(ctx, "test -e "+path, shell.RunOptions{})
	if err != nil {
		var remoteErr *shell.RemoteCommandError
		if errors.As(err, &remoteErr) && exists.ExitCode == 1 {
			return "", nil
		}
		return "", err
	}
	return remoteHashRequired(ctx, remote, path)
}

func remoteHashRequired(ctx context.Context, remote shell.Shell, path string) (string, error) {
	result, err := remote.Run(ctx, "sha256sum "+path, shell.RunOptions{})
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(result.Stdout))
	if len(fields) == 0 {
		return "", fmt.Errorf("sha256sum returned no hash")
	}
	return fields[0], nil
}

func runChecked(ctx context.Context, remote shell.Shell, command string) error {
	result, err := remote.Run(ctx, command, shell.RunOptions{})
	if err != nil {
		return device.WrapError(device.CodeRemoteCommandFailed, "remote soft-persistence command failed", err)
	}
	if result.ExitCode != 0 {
		return &shell.RemoteCommandError{Result: result}
	}
	return nil
}
