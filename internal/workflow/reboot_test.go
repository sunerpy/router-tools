package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/workflow"
)

type rebootShell struct {
	bootID    string
	uid       string
	rebootErr error
}

func (s *rebootShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	switch command {
	case "id -u":
		return shell.CommandResult{Stdout: []byte(s.uid), Started: true}, nil
	case "cat /proc/sys/kernel/random/boot_id":
		return shell.CommandResult{Stdout: []byte(s.bootID + "\n"), Started: true}, nil
	case "reboot":
		if s.rebootErr != nil {
			return shell.CommandResult{Started: true}, s.rebootErr
		}
		return shell.CommandResult{Started: true}, nil
	default:
		return shell.CommandResult{ExitCode: 1, Started: true}, &shell.RemoteCommandError{}
	}
}
func (s *rebootShell) Close() error { return nil }

func TestRebootAndWaitRequiresDownAndNewBootID(t *testing.T) {
	t.Parallel()

	oldShell := &rebootShell{bootID: "00000000-0000-0000-0000-000000000001", uid: "0"}
	newShell := &rebootShell{bootID: "00000000-0000-0000-0000-000000000002", uid: "0"}
	downCalls := 0
	result, err := workflow.RebootAndWait(t.Context(), oldShell, workflow.RebootConfig{
		Sleep: func(time.Duration) {},
		PortsDown: func(context.Context) (bool, error) {
			downCalls++
			return downCalls >= 2, nil
		},
		Connect:       func(context.Context) (shell.Shell, error) { return newShell, nil },
		DownRounds:    3,
		OnlineTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("RebootAndWait() error = %v", err)
	}
	if result.OldBootID != oldShell.bootID || result.NewBootID != newShell.bootID || result.Shell != newShell {
		t.Fatalf("result = %+v", result)
	}
	if downCalls != 2 {
		t.Fatalf("down probes = %d", downCalls)
	}
}

func TestRebootAndWaitStopsWhenNeverDown(t *testing.T) {
	t.Parallel()

	oldShell := &rebootShell{bootID: "00000000-0000-0000-0000-000000000001", uid: "0"}
	connectCalls := 0
	_, err := workflow.RebootAndWait(t.Context(), oldShell, workflow.RebootConfig{
		Sleep:         func(time.Duration) {},
		PortsDown:     func(context.Context) (bool, error) { return false, nil },
		Connect:       func(context.Context) (shell.Shell, error) { connectCalls++; return nil, errors.New("unexpected") },
		DownRounds:    2,
		OnlineTimeout: time.Second,
	})
	assertWorkflowCode(t, err, device.CodeRebootNotObserved)
	if connectCalls != 0 {
		t.Fatalf("connect calls = %d", connectCalls)
	}
}

func TestRebootAndWaitRejectsUnchangedBootID(t *testing.T) {
	t.Parallel()

	oldShell := &rebootShell{bootID: "00000000-0000-0000-0000-000000000001", uid: "0"}
	_, err := workflow.RebootAndWait(t.Context(), oldShell, workflow.RebootConfig{
		Sleep:     func(time.Duration) {},
		PortsDown: func(context.Context) (bool, error) { return true, nil },
		Connect: func(context.Context) (shell.Shell, error) {
			return &rebootShell{bootID: oldShell.bootID, uid: "0"}, nil
		},
		DownRounds:    1,
		OnlineTimeout: 20 * time.Millisecond,
	})
	assertWorkflowCode(t, err, device.CodeRebootTimeout)
}

func TestRebootAndWaitRejectsInvalidInitialBootIDBeforeWrite(t *testing.T) {
	t.Parallel()

	oldShell := &rebootShell{bootID: "invalid", uid: "0", rebootErr: errors.New("must not happen")}
	_, err := workflow.RebootAndWait(t.Context(), oldShell, workflow.RebootConfig{})
	if err == nil {
		t.Fatal("RebootAndWait() error = nil")
	}
}

func assertWorkflowCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != want {
		t.Fatalf("error = %T %v, want %s", err, err, want)
	}
}
