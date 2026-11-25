package shell_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sunerpy/router-tools/internal/transport/shell"
)

func TestQuote(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":    "''",
		"abc": "'abc'",
		"a'b": `'a'"'"'b'`,
		"a b": "'a b'",
		"$()": "'$()'",
	}
	for input, want := range tests {
		if got := shell.Quote(input); got != want {
			t.Fatalf("Quote(%q) = %q, want %q", input, got, want)
		}
	}
}

type fakeShell struct {
	result  shell.CommandResult
	err     error
	command string
}

func (s *fakeShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	s.command = command
	return s.result, s.err
}
func (s *fakeShell) Close() error { return nil }

func TestVerifyRootRequiresExactUIDZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result shell.CommandResult
		want   bool
	}{
		{name: "root", result: shell.CommandResult{Stdout: []byte("0\n"), ExitCode: 0, Started: true}, want: true},
		{name: "non-root", result: shell.CommandResult{Stdout: []byte("1000\n"), ExitCode: 0, Started: true}},
		{name: "extra output", result: shell.CommandResult{Stdout: []byte("0\n0\n"), ExitCode: 0, Started: true}},
		{name: "non-zero exit", result: shell.CommandResult{Stdout: []byte("0\n"), ExitCode: 1, Started: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeShell{result: tt.result}
			got, err := shell.VerifyRoot(t.Context(), fake)
			if tt.want && err != nil {
				t.Fatalf("VerifyRoot() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("VerifyRoot() = %t, want %t", got, tt.want)
			}
			if fake.command != "id -u" {
				t.Fatalf("command = %q", fake.command)
			}
		})
	}
}

func TestRemoteCommandErrorPreservesEvidence(t *testing.T) {
	t.Parallel()

	result := shell.CommandResult{Stdout: []byte("partial"), Stderr: []byte("failure"), ExitCode: 7, Started: true}
	err := &shell.RemoteCommandError{Result: result}
	var target *shell.RemoteCommandError
	if !errors.As(err, &target) || target.Result.ExitCode != 7 || string(target.Result.Stdout) != "partial" {
		t.Fatalf("RemoteCommandError = %#v", err)
	}
}
