// Package workflow contains transport-independent reboot and high-risk state machines.
package workflow

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

var canonicalBootID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// RebootConfig injects clocks and connectivity probes for deterministic tests.
type RebootConfig struct {
	Sleep         func(time.Duration)
	PortsDown     func(context.Context) (bool, error)
	Connect       func(context.Context) (shell.Shell, error)
	DownRounds    int
	OnlineTimeout time.Duration
}

// RebootResult records the observed boot transition and new verified shell. WriteStarted is set before
// the reboot command is sent and stays set for every failure after it.
type RebootResult struct {
	OldBootID    string
	NewBootID    string
	Shell        shell.Shell
	WriteStarted bool
}

// ReadBootID reads and validates a canonical UUID.
func ReadBootID(ctx context.Context, remote shell.Shell) (string, error) {
	result, err := remote.Run(ctx, "cat /proc/sys/kernel/random/boot_id", shell.RunOptions{})
	if err != nil {
		return "", device.WrapError(device.CodeRemoteCommandFailed, "cannot read boot_id", err)
	}
	bootID := strings.TrimSpace(string(result.Stdout))
	if !canonicalBootID.MatchString(bootID) {
		return "", &device.Error{Code: device.CodeBootIDUnchanged, Message: "boot_id is not a canonical UUID"}
	}
	return bootID, nil
}

// RebootAndWait performs one non-retried reboot and verifies down/new-boot/root evidence.
func RebootAndWait(ctx context.Context, remote shell.Shell, config RebootConfig) (RebootResult, error) {
	result := RebootResult{}
	oldBootID, err := ReadBootID(ctx, remote)
	if err != nil {
		return result, err
	}
	result.OldBootID = oldBootID
	if config.Sleep == nil {
		config.Sleep = time.Sleep
	}
	if config.DownRounds <= 0 {
		config.DownRounds = 12
	}
	if config.OnlineTimeout <= 0 {
		config.OnlineTimeout = 420 * time.Second
	}
	if config.PortsDown == nil || config.Connect == nil {
		return result, &device.Error{Code: device.CodeInternalError, Message: "reboot connectivity probes are not configured"}
	}
	result.WriteStarted = true
	commandResult, rebootErr := remote.Run(ctx, "reboot", shell.RunOptions{})
	_ = remote.Close()
	if rebootErr != nil {
		var remoteErr *shell.RemoteCommandError
		if errors.As(rebootErr, &remoteErr) {
			return result, device.WrapError(device.CodeRemoteCommandFailed, "router rejected reboot", rebootErr)
		}
		if !commandResult.Started {
			return result, &device.Error{Code: device.CodeRemoteResultUnknown, Message: "reboot was not confirmed as submitted"}
		}
	}

	config.Sleep(5 * time.Second)
	down := false
	for round := 0; round < config.DownRounds; round++ {
		observed, err := config.PortsDown(ctx)
		if err != nil {
			return result, err
		}
		if observed {
			down = true
			break
		}
		if round+1 < config.DownRounds {
			config.Sleep(5 * time.Second)
		}
	}
	if !down {
		return result, &device.Error{Code: device.CodeRebootNotObserved, Message: "router did not disconnect Web, SSH, and Telnet"}
	}

	deadline := time.Now().Add(config.OnlineTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		candidate, err := config.Connect(ctx)
		if err == nil {
			root, verifyErr := shell.VerifyRoot(ctx, candidate)
			if verifyErr == nil && root {
				newBootID, bootErr := ReadBootID(ctx, candidate)
				if bootErr == nil {
					if newBootID != oldBootID {
						result.NewBootID = newBootID
						result.Shell = candidate
						return result, nil
					}
					lastErr = &device.Error{Code: device.CodeBootIDUnchanged, Message: "boot_id did not change"}
				} else {
					lastErr = bootErr
				}
			} else {
				lastErr = verifyErr
			}
			_ = candidate.Close()
		} else {
			lastErr = err
			var deviceErr *device.Error
			if errors.As(err, &deviceErr) && (deviceErr.Code == device.CodeHostKeyMismatch || deviceErr.Code == device.CodeSSHHostKeyRejected) {
				return result, err
			}
		}
		if remaining := time.Until(deadline); remaining > 0 {
			config.Sleep(min(6*time.Second, remaining))
		}
	}
	return result, device.WrapError(device.CodeRebootTimeout, "router did not return with a new boot_id and root shell", lastErr)
}
