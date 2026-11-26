// Package sshtransport implements password-authenticated SSH without insecure host-key defaults.
package sshtransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

const connectionTimeout = 5 * time.Second

// Config contains one SSH connection's non-secret and secret inputs.
type Config struct {
	Host            string
	Port            int
	Password        secret.Password
	HostKeyCallback ssh.HostKeyCallback
	// timeout bounds the TCP connection and the handshake; zero means connectionTimeout. Tests shorten it.
	timeout time.Duration
}

// Shell executes each command in an independent SSH session.
type Shell struct {
	mu     sync.Mutex
	conn   net.Conn
	client *ssh.Client
	closed bool
}

// Dial establishes TCP and SSH within bounded deadlines.
func Dial(ctx context.Context, config Config) (*Shell, error) {
	if config.HostKeyCallback == nil {
		return nil, &device.Error{Code: device.CodeSSHHostKeyRejected, Message: "SSH host key callback is required"}
	}
	if err := config.Password.Validate(); err != nil {
		return nil, err
	}
	if config.Host == "" || config.Port < 1 || config.Port > 65535 {
		return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "invalid SSH target"}
	}
	timeout := config.timeout
	if timeout <= 0 {
		timeout = connectionTimeout
	}
	address := net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))
	connection, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, sshNetworkError(err)
	}
	_ = connection.SetDeadline(time.Now().Add(timeout))
	decide := config.HostKeyCallback
	var handshaking atomic.Bool
	handshaking.Store(true)
	clientConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.Password(config.Password.Expose())},
		// A person may be deciding about an unknown key (the desktop waits up to two minutes, the CLI asks on the
		// terminal), so the handshake deadline stops while the callback runs and starts again when it returns.
		// Later key exchanges call the callback too; they keep the deadline of the command that is running.
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if !handshaking.Load() {
				return decide(hostname, remote, key)
			}
			_ = connection.SetDeadline(time.Time{})
			defer func() { _ = connection.SetDeadline(time.Now().Add(timeout)) }()
			return decide(hostname, remote, key)
		},
		Timeout: timeout,
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, address, clientConfig)
	handshaking.Store(false)
	if err != nil {
		_ = connection.Close()
		// A decision ended by the operation's cancellation reads as a canceled run, not as a refused key.
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, device.WrapError(device.CodeCanceled, "operation canceled", err)
		}
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) {
			return nil, err
		}
		if strings.Contains(strings.ToLower(err.Error()), "unable to authenticate") || strings.Contains(strings.ToLower(err.Error()), "authentication failed") {
			return nil, &device.Error{Code: device.CodeSSHAuthFailed, Message: "SSH password authentication failed"}
		}
		return nil, device.WrapError(device.CodeSSHUnreachable, "SSH handshake failed", err)
	}
	_ = connection.SetDeadline(time.Time{})
	return &Shell{conn: connection, client: ssh.NewClient(clientConnection, channels, requests)}, nil
}

// Run implements shell.Shell.
func (s *Shell) Run(ctx context.Context, command string, options shell.RunOptions) (shell.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := shell.CommandResult{}
	if s.closed {
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "SSH connection is closed"}
	}
	timeout, outputLimit := options.Normalized()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if deadline, ok := runCtx.Deadline(); ok {
		_ = s.conn.SetDeadline(deadline)
		defer func() { _ = s.conn.SetDeadline(time.Time{}) }()
	}
	session, err := s.client.NewSession()
	if err != nil {
		return result, device.WrapError(device.CodeRemoteCommandFailed, "cannot create SSH session", err)
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
	stdoutChannel := readAsync(stdout, outputLimit)
	stderrChannel := readAsync(stderr, outputLimit)
	if err := session.Start(command); err != nil {
		return result, device.WrapError(device.CodeRemoteCommandFailed, "cannot start SSH command", err)
	}
	result.Started = true
	waitChannel := make(chan error, 1)
	go func() { waitChannel <- session.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitChannel:
	case <-runCtx.Done():
		_ = s.closeLocked()
		waitErr = runCtx.Err()
		<-waitChannel
	}
	stdoutResult := <-stdoutChannel
	stderrResult := <-stderrChannel
	result.Stdout = stdoutResult.data
	result.Stderr = stderrResult.data
	if stdoutResult.tooLong || stderrResult.tooLong {
		_ = s.closeLocked()
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "SSH command output exceeded the limit"}
	}
	if stdoutResult.err != nil || stderrResult.err != nil {
		return result, &device.Error{Code: device.CodeRemoteResultUnknown, Message: "SSH output stream failed after command submission"}
	}
	if waitErr != nil {
		var exitError *ssh.ExitError
		if errors.As(waitErr, &exitError) {
			result.ExitCode = exitError.ExitStatus()
			return result, &shell.RemoteCommandError{Result: result}
		}
		return result, device.WrapError(device.CodeRemoteResultUnknown, "SSH command result is unknown after submission", waitErr)
	}
	return result, nil
}

type streamResult struct {
	data    []byte
	err     error
	tooLong bool
}

func readAsync(reader io.Reader, limit int) <-chan streamResult {
	channel := make(chan streamResult, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
		channel <- streamResult{data: data, err: err, tooLong: len(data) > limit}
	}()
	return channel
}

// Close closes the SSH client and underlying connection.
func (s *Shell) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *Shell) closeLocked() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.client != nil {
		return s.client.Close()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func sshNetworkError(err error) error {
	if errors.Is(err, context.Canceled) {
		return device.WrapError(device.CodeCanceled, "operation canceled", err)
	}
	deviceErr := device.WrapError(device.CodeSSHUnreachable, "SSH endpoint is unreachable", err)
	deviceErr.Retryable = true
	return deviceErr
}
