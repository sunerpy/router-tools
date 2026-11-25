// Package telnettransport implements the restricted root Telnet fallback.
package telnettransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

var (
	loginPrompt    = regexp.MustCompile(`(?im)(?:login|username):\s*$`)
	passwordPrompt = regexp.MustCompile(`(?im)password:\s*$`)
	shellPrompt    = regexp.MustCompile(`(?m)^[^\r\n]*[#$>]\s*$`)
	authFailure    = regexp.MustCompile(`(?i)login incorrect|authentication fail|access denied`)
)

// Config contains the fixed root Telnet login inputs.
type Config struct {
	Address  string
	Password secret.Password
}

// Shell is a stateful Telnet command-frame transport.
type Shell struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	conn    net.Conn
	decoder Decoder
	buffer  []byte
	closed  bool
}

// Dial performs bounded Telnet negotiation and root login.
func Dial(ctx context.Context, config Config) (*Shell, error) {
	if err := config.Password.Validate(); err != nil {
		return nil, err
	}
	if _, _, err := net.SplitHostPort(config.Address); err != nil {
		return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "invalid Telnet address"}
	}
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", config.Address)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, device.WrapError(device.CodeCanceled, "operation canceled", err)
		}
		return nil, &device.Error{Code: device.CodeTelnetUnreachable, Message: "Telnet endpoint is unreachable", Retryable: true}
	}
	remote := &Shell{conn: connection}
	fail := func(err error) (*Shell, error) {
		_ = connection.Close()
		if errors.Is(err, context.Canceled) {
			return nil, device.WrapError(device.CodeCanceled, "operation canceled", err)
		}
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) {
			return nil, err
		}
		return nil, device.WrapError(device.CodeTelnetUnreachable, "Telnet login failed", err)
	}
	if err := remote.login(ctx, config.Password); err != nil {
		return fail(err)
	}
	return remote, nil
}

func (s *Shell) login(ctx context.Context, password secret.Password) error {
	if err := s.withDeadline(ctx, 10*time.Second, func(ctx context.Context) error {
		_, err := s.readUntil(ctx, loginPrompt)
		return err
	}); err != nil {
		return err
	}
	if err := s.writeAll([]byte("root\n")); err != nil {
		return err
	}
	if err := s.withDeadline(ctx, 10*time.Second, func(ctx context.Context) error {
		_, err := s.readUntil(ctx, passwordPrompt)
		return err
	}); err != nil {
		return err
	}
	if err := s.writeAll([]byte(password.Expose() + "\n")); err != nil {
		return err
	}
	return s.withDeadline(ctx, 15*time.Second, func(ctx context.Context) error {
		_, err := s.readUntil(ctx, shellPrompt)
		return err
	})
}

// Run executes one marker-framed command and never reuses a failed connection.
func (s *Shell) Run(ctx context.Context, command string, options shell.RunOptions) (shell.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := shell.CommandResult{}
	if s.closed {
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "Telnet connection is closed"}
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	_, outputLimit := options.Normalized()
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return result, fmt.Errorf("generate Telnet command marker: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	begin := "__BEGIN_" + token + "__"
	end := "__END_" + token + "__"
	frame := "printf '\\n%s\\n' " + shell.Quote(begin) +
		"; sh -c " + shell.Quote(command) + " 2>&1; __rd08_status=$?; " +
		"printf '\\n%s:%s\\n' " + shell.Quote(end) + " \"$__rd08_status\"\n"
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if deadline, ok := runCtx.Deadline(); ok {
		_ = s.conn.SetDeadline(deadline)
		defer func() { _ = s.conn.SetDeadline(time.Time{}) }()
	}
	if err := s.writeAll([]byte(frame)); err != nil {
		_ = s.closeLocked()
		return result, device.WrapError(device.CodeRemoteResultUnknown, "Telnet command submission failed", err)
	}
	result.Started = true
	endPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(end) + `:([0-9]+)\r?$`)
	data, err := s.readUntil(runCtx, endPattern)
	if err != nil {
		_ = s.closeLocked()
		return result, device.WrapError(device.CodeRemoteResultUnknown, "Telnet command result is unknown", err)
	}
	beginPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(begin) + `\r?\n`)
	beginMatch := beginPattern.FindIndex(data)
	endMatch := endPattern.FindSubmatchIndex(data)
	if beginMatch == nil || endMatch == nil || endMatch[0] < beginMatch[1] {
		_ = s.closeLocked()
		return result, &device.Error{Code: device.CodeRemoteResultUnknown, Message: "Telnet command markers are missing or misordered"}
	}
	output := data[beginMatch[1]:endMatch[0]]
	if len(output) > outputLimit {
		_ = s.closeLocked()
		return result, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "Telnet command output exceeded the limit"}
	}
	result.Stdout = []byte(strings.ReplaceAll(string(output), "\r\n", "\n"))
	exitCode, err := strconv.Atoi(string(data[endMatch[2]:endMatch[3]]))
	if err != nil {
		_ = s.closeLocked()
		return result, &device.Error{Code: device.CodeRemoteResultUnknown, Message: "Telnet exit status is invalid"}
	}
	result.ExitCode = exitCode
	if exitCode != 0 {
		return result, &shell.RemoteCommandError{Result: result}
	}
	return result, nil
}

func (s *Shell) readUntil(ctx context.Context, pattern *regexp.Regexp) ([]byte, error) {
	for {
		if authFailure.Match(s.buffer) {
			return nil, &device.Error{Code: device.CodeTelnetAuthFailed, Message: "Telnet authentication failed"}
		}
		if pattern.Match(s.buffer) {
			data := s.buffer
			s.buffer = nil
			return data, nil
		}
		chunk := make([]byte, 4096)
		count, err := s.conn.Read(chunk)
		if count > 0 {
			data, responses := s.decoder.Push(chunk[:count])
			s.buffer = append(s.buffer, data...)
			if len(responses) > 0 {
				if err := s.writeAll(responses); err != nil {
					return nil, err
				}
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
	}
}

func (s *Shell) withDeadline(ctx context.Context, timeout time.Duration, operation func(context.Context) error) error {
	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if deadline, ok := operationCtx.Deadline(); ok {
		_ = s.conn.SetDeadline(deadline)
		defer func() { _ = s.conn.SetDeadline(time.Time{}) }()
	}
	return operation(operationCtx)
}

func (s *Shell) writeAll(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for len(data) > 0 {
		count, err := s.conn.Write(data)
		if err != nil {
			return err
		}
		data = data[count:]
	}
	return nil
}

// Close closes the underlying connection.
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
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
