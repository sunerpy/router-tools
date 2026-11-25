package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

func readAdminPassword(stdin io.Reader, stderr io.Writer, fromStdin, nonInteractive bool) (secret.Password, error) {
	if fromStdin {
		raw, err := readSecretLine(stdin)
		if err != nil {
			return secret.Password{}, err
		}
		return secret.NewPassword(raw)
	}
	if nonInteractive {
		return secret.Password{}, &device.Error{
			Code:    device.CodeInvalidPassword,
			Message: "non-interactive admin authentication requires --admin-password-stdin",
		}
	}
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if file, ok := stdin.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(file.Fd())) {
		_, _ = fmt.Fprint(stderr, "Admin password: ")
		raw, err := term.ReadPassword(int(file.Fd()))
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			return secret.Password{}, &device.Error{Code: device.CodeInvalidPassword, Message: "cannot read hidden admin password"}
		}
		return secret.NewPassword(string(raw))
	}
	_, _ = fmt.Fprintln(stderr, "Warning: password input is not a TTY; input will be visible")
	_, _ = fmt.Fprint(stderr, "Admin password: ")
	raw, err := readSecretLine(stdin)
	if err != nil {
		return secret.Password{}, err
	}
	return secret.NewPassword(raw)
}

func readRootPassword(stdin io.Reader, stderr io.Writer, fromStdin, nonInteractive bool) (secret.Password, error) {
	if fromStdin {
		raw, err := readSecretLine(stdin)
		if err != nil {
			return secret.Password{}, err
		}
		return secret.NewPassword(raw)
	}
	if nonInteractive {
		return secret.Password{}, &device.Error{
			Code:    device.CodeInvalidPassword,
			Message: "non-interactive root verification requires --root-password-stdin",
		}
	}
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if file, ok := stdin.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(file.Fd())) {
		_, _ = fmt.Fprint(stderr, "Root password: ")
		raw, err := term.ReadPassword(int(file.Fd()))
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			return secret.Password{}, &device.Error{Code: device.CodeInvalidPassword, Message: "cannot read hidden root password"}
		}
		return secret.NewPassword(string(raw))
	}
	_, _ = fmt.Fprintln(stderr, "Warning: password input is not a TTY; input will be visible")
	_, _ = fmt.Fprint(stderr, "Root password: ")
	raw, err := readSecretLine(stdin)
	if err != nil {
		return secret.Password{}, err
	}
	return secret.NewPassword(raw)
}

func readSecretLine(stdin io.Reader) (string, error) {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	var raw strings.Builder
	buffer := make([]byte, 1)
	endedWithLF := false
	for {
		count, err := stdin.Read(buffer)
		if count > 0 {
			if buffer[0] == '\n' {
				endedWithLF = true
				break
			}
			raw.WriteByte(buffer[0])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) || raw.Len() == 0 {
				return "", &device.Error{Code: device.CodeInvalidPassword, Message: "cannot read password from stdin"}
			}
			break
		}
	}
	value := raw.String()
	if endedWithLF {
		value = strings.TrimSuffix(value, "\r")
	}
	return value, nil
}
