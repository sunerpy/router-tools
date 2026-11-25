package sshtransport

import (
	"context"
	"errors"
	"io"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

// Stream copies bounded binary stdout directly to destination while draining stderr.
func (s *Shell) Stream(ctx context.Context, command string, destination io.Writer, maxBytes int64) (shell.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := shell.CommandResult{}
	if s.closed {
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "SSH connection is closed"}
	}
	if destination == nil || maxBytes <= 0 {
		return result, &device.Error{Code: device.CodeInvalidArgument, Message: "invalid SSH stream destination or limit"}
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = s.conn.SetDeadline(deadline)
		defer func() { _ = s.conn.SetDeadline(time.Time{}) }()
	}
	session, err := s.client.NewSession()
	if err != nil {
		return result, device.WrapError(device.CodeRemoteCommandFailed, "cannot create SSH stream session", err)
	}
	defer func() { _ = session.Close() }()
	stdout, err := session.StdoutPipe()
	if err != nil {
		return result, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return result, err
	}
	stderrChannel := readAsync(stderr, shell.DefaultOutputLimit)
	if err := session.Start(command); err != nil {
		return result, device.WrapError(device.CodeRemoteCommandFailed, "cannot start SSH stream", err)
	}
	result.Started = true
	type copyResult struct {
		count int64
		err   error
	}
	copyChannel := make(chan copyResult, 1)
	go func() {
		count, err := io.Copy(destination, io.LimitReader(stdout, maxBytes+1))
		copyChannel <- copyResult{count: count, err: err}
	}()
	waitChannel := make(chan error, 1)
	go func() { waitChannel <- session.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitChannel:
	case <-ctx.Done():
		_ = s.closeLocked()
		waitErr = ctx.Err()
		<-waitChannel
	}
	copyOutcome := <-copyChannel
	stderrOutcome := <-stderrChannel
	result.Stderr = stderrOutcome.data
	if copyOutcome.err != nil || stderrOutcome.err != nil || copyOutcome.count > maxBytes || stderrOutcome.tooLong {
		_ = s.closeLocked()
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "SSH binary stream failed or exceeded its bound"}
	}
	if waitErr != nil {
		var exitError *ssh.ExitError
		if errors.As(waitErr, &exitError) {
			result.ExitCode = exitError.ExitStatus()
			return result, &shell.RemoteCommandError{Result: result}
		}
		return result, device.WrapError(device.CodeRemoteResultUnknown, "SSH stream result is unknown", waitErr)
	}
	return result, nil
}
