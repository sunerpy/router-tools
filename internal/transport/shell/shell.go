// Package shell defines transport-independent remote command semantics.
package shell

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
)

const (
	DefaultTimeout     = 30 * time.Second
	DefaultOutputLimit = 4 << 20
)

// CommandResult preserves execution evidence, including uncertain submission state.
type CommandResult struct {
	Stdout   []byte `json:"-"`
	Stderr   []byte `json:"-"`
	ExitCode int    `json:"exit_code"`
	Started  bool   `json:"started"`
}

// RunOptions controls one remote command.
type RunOptions struct {
	Timeout     time.Duration
	OutputLimit int
}

// Normalized returns the specification defaults for unset options.
func (o RunOptions) Normalized() (time.Duration, int) {
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.OutputLimit <= 0 {
		o.OutputLimit = DefaultOutputLimit
	}
	return o.Timeout, o.OutputLimit
}

// Shell executes fixed commands supplied by trusted adapters.
type Shell interface {
	Run(ctx context.Context, command string, options RunOptions) (CommandResult, error)
	Close() error
}

// StreamingShell adds bounded binary stdout streaming for SSH backup.
type StreamingShell interface {
	Shell
	Stream(ctx context.Context, command string, destination io.Writer, maxBytes int64) (CommandResult, error)
}

// RemoteCommandError reports a non-zero remote exit without dumping command output.
type RemoteCommandError struct {
	Result CommandResult
}

func (e *RemoteCommandError) Error() string {
	return fmt.Sprintf("remote command exited with status %d", e.Result.ExitCode)
}

// Quote applies POSIX single-quote escaping to dynamic command fragments.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// VerifyRoot requires a successful id -u whose entire trimmed stdout is exactly 0.
func VerifyRoot(ctx context.Context, remote Shell) (bool, error) {
	result, err := remote.Run(ctx, "id -u", RunOptions{})
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, &RemoteCommandError{Result: result}
	}
	if strings.TrimSpace(string(result.Stdout)) != "0" {
		return false, &device.Error{Code: device.CodeRootVerifyFailed, Message: "remote user is not root"}
	}
	return true, nil
}
