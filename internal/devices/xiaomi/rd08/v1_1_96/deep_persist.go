package rd08v1196

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/workflow"
)

const crashWriteCommand = `zz=$(dd if=/dev/zero bs=1 count=2 2>/dev/null) ; printf '\xA5\x5A%c%c' $zz $zz | mtd write - crash`

// DeepPersist implements the RD08-only three-reboot high-risk state machine.
func (p *Prober) DeepPersist(ctx context.Context, params app.DeepParams) (app.DeepResult, error) {
	result := app.DeepResult{Status: app.StatusFailed}
	fail := func(err error) (app.DeepResult, error) {
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) && deviceErr.Code == device.CodeRemoteResultUnknown {
			result.Status = app.StatusUnknown
			_ = params.Journal.Stage("UNKNOWN")
		} else {
			if result.AcceptedWrites > 0 || len(result.Transitions) > 0 {
				result.Status = app.StatusPartial
			}
			_ = params.Journal.Stage("FAILED")
		}
		return result, err
	}
	if params.Journal == nil {
		return result, &device.Error{Code: device.CodeInternalError, Message: "deep journal is required"}
	}
	if err := params.Journal.Stage("PRECHECK"); err != nil {
		return result, err
	}
	remote, err := p.sshDial(ctx, params.Target, params.RootPassword, params.HostKeyPolicy)
	if err != nil {
		return fail(err)
	}
	current := remote
	defer func() {
		if current != nil {
			_ = current.Close()
		}
	}()
	if _, err := shell.VerifyRoot(ctx, current); err != nil {
		return fail(err)
	}
	bootID, err := workflow.ReadBootID(ctx, current)
	if err != nil {
		return fail(err)
	}
	if err := runChecked(ctx, current, "command -v nvram && command -v bdata && command -v mtd && command -v sed && command -v base64 && command -v sha256sum && command -v uci && command -v dd && command -v printf"); err != nil {
		return fail(err)
	}
	mtdResult, err := current.Run(ctx, "cat /proc/mtd", shell.RunOptions{})
	if err != nil {
		return fail(err)
	}
	if err := backup.VerifyOnline(params.Manifest, params.Observation, params.ProfileID, string(mtdResult.Stdout), bootID); err != nil {
		return fail(err)
	}
	partitions, err := backup.ParseMTD(string(mtdResult.Stdout))
	if err != nil {
		return fail(err)
	}
	crashIndex := -1
	for _, partition := range partitions {
		if partition.Name == "crash" {
			crashIndex = partition.Index
			break
		}
	}
	if crashIndex < 0 {
		return fail(&device.Error{Code: device.CodeBackupLayoutMismatch, Message: "crash partition is missing"})
	}
	if err := runChecked(ctx, current, fmt.Sprintf("test -r /dev/mtd%d", crashIndex)); err != nil {
		return fail(err)
	}

	if err := deepWrite(ctx, current, params.Journal, "CRASH_WRITE_STARTED", "CRASH_WRITE_ACCEPTED", crashWriteCommand); err != nil {
		return fail(err)
	}
	result.AcceptedWrites++
	reboot, err := p.deepReboot(ctx, current, params, 1)
	if err != nil {
		return fail(err)
	}
	current = reboot.Shell
	result.Transitions = append(result.Transitions, app.BootTransition{OldBootID: reboot.OldBootID, NewBootID: reboot.NewBootID})

	if err := params.Journal.Stage("FLAGS_WRITE_STARTED"); err != nil {
		return fail(err)
	}
	if err := runChecked(ctx, current, "nvram set ssh_en=1 && nvram set telnet_en=1 && nvram set uart_en=1 && nvram set boot_wait=on && nvram commit"); err != nil {
		return fail(err)
	}
	result.AcceptedWrites++
	if err := runChecked(ctx, current, "bdata set ssh_en=1 && bdata set telnet_en=1 && bdata set uart_en=1 && bdata set boot_wait=on && bdata commit"); err != nil {
		return fail(err)
	}
	result.AcceptedWrites++
	if err := verifyDeepFlags(ctx, current, true); err != nil {
		return fail(err)
	}
	if err := params.Journal.Stage("FLAGS_VERIFIED"); err != nil {
		return fail(err)
	}
	reboot, err = p.deepReboot(ctx, current, params, 2)
	if err != nil {
		return fail(err)
	}
	current = reboot.Shell
	result.Transitions = append(result.Transitions, app.BootTransition{OldBootID: reboot.OldBootID, NewBootID: reboot.NewBootID})
	if err := verifyDeepFlags(ctx, current, false); err != nil {
		return fail(err)
	}
	if err := deepWrite(ctx, current, params.Journal, "CRASH_ERASE_STARTED", "CRASH_ERASE_ACCEPTED", "mtd erase crash"); err != nil {
		return fail(err)
	}
	result.AcceptedWrites++
	reboot, err = p.deepReboot(ctx, current, params, 3)
	if err != nil {
		return fail(err)
	}
	current = reboot.Shell
	result.Transitions = append(result.Transitions, app.BootTransition{OldBootID: reboot.OldBootID, NewBootID: reboot.NewBootID})
	if err := verifyDeepFlags(ctx, current, false); err != nil {
		return fail(err)
	}
	if err := params.Journal.Stage("FINAL_RESTORE"); err != nil {
		return fail(err)
	}
	if err := runChecked(ctx, current, `sed -i 's/channel=.*/channel="debug"/g' /etc/init.d/dropbear && /etc/init.d/dropbear start`); err != nil {
		return fail(err)
	}
	soft, err := installSoft(ctx, current)
	if err != nil {
		return fail(err)
	}
	result.SoftVerified = soft.Verified
	if err := params.Journal.Stage("FINAL_VERIFY"); err != nil {
		return fail(err)
	}
	verification, err := p.verifySSH(ctx, params.Target, params.RootPassword, params.HostKeyPolicy)
	if err != nil {
		return fail(err)
	}
	result.FinalSSHVerified = verification.RootVerified
	if !result.FinalSSHVerified || !result.SoftVerified {
		return fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "final deep-persist verification is incomplete"})
	}
	result.Status = app.StatusSucceeded
	if err := params.Journal.Stage("SUCCEEDED"); err != nil {
		return result, err
	}
	return result, nil
}

func deepWrite(ctx context.Context, remote shell.Shell, journal app.DeepJournal, started, accepted, command string) error {
	if err := journal.Stage(started); err != nil {
		return err
	}
	result, err := remote.Run(ctx, command, shell.RunOptions{})
	if err != nil {
		if result.Started {
			var remoteErr *shell.RemoteCommandError
			if !errors.As(err, &remoteErr) {
				return device.WrapError(device.CodeRemoteResultUnknown, "deep write result is unknown; do not retry", err)
			}
		}
		return err
	}
	return journal.Stage(accepted)
}

func (p *Prober) deepReboot(ctx context.Context, remote shell.Shell, params app.DeepParams, index int) (workflow.RebootResult, error) {
	if err := params.Journal.Stage(fmt.Sprintf("REBOOT_%d_WAIT_DOWN", index)); err != nil {
		return workflow.RebootResult{}, err
	}
	config := p.rebootConfig(params.Target, params.RootPassword, params.HostKeyPolicy, true)
	portsDown := config.PortsDown
	config.PortsDown = func(ctx context.Context) (bool, error) {
		down, err := portsDown(ctx)
		if down {
			if err := params.Journal.Stage(fmt.Sprintf("REBOOT_%d_WAIT_UP", index)); err != nil {
				return false, err
			}
		}
		return down, err
	}
	result, err := workflow.RebootAndWait(ctx, remote, config)
	if err != nil {
		return result, err
	}
	if err := params.Journal.Transition(result.OldBootID, result.NewBootID); err != nil {
		return result, err
	}
	return result, nil
}

func verifyDeepFlags(ctx context.Context, remote shell.Shell, includeNVRAM bool) error {
	expected := map[string]string{"ssh_en": "1", "telnet_en": "1", "uart_en": "1", "boot_wait": "on"}
	sources := []string{"bdata"}
	if includeNVRAM {
		sources = append(sources, "nvram")
	}
	for _, source := range sources {
		for key, expectedValue := range expected {
			result, err := remote.Run(ctx, source+" get "+key, shell.RunOptions{})
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(result.Stdout)) != expectedValue {
				return &device.Error{Code: device.CodeRemoteCommandFailed, Message: fmt.Sprintf("%s.%s does not match the required value", source, key)}
			}
		}
	}
	return nil
}
