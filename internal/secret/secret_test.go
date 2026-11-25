package secret_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

func TestPasswordValidation(t *testing.T) {
	t.Parallel()

	valid := []string{
		"admin",
		" leading and trailing ",
		"密码🔐",
		strings.Repeat("x", 1024),
	}
	for _, raw := range valid {
		password, err := secret.NewPassword(raw)
		if err != nil {
			t.Fatalf("NewPassword(%q) error = %v", raw, err)
		}
		if password.Expose() != raw {
			t.Fatalf("password changed from %q to %q", raw, password.Expose())
		}
	}

	invalid := []string{
		"",
		"line\nfeed",
		"carriage\rreturn",
		"tab\tvalue",
		"nul\x00value",
		"delete\x7fvalue",
		string([]byte{0xff}),
		strings.Repeat("x", 1025),
	}
	for _, raw := range invalid {
		_, err := secret.NewPassword(raw)
		if err == nil {
			t.Fatalf("NewPassword(%q) error = nil", raw)
		}
		var deviceErr *device.Error
		if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeInvalidPassword {
			t.Fatalf("NewPassword(%q) error = %T %v, want INVALID_PASSWORD", raw, err, err)
		}
	}
}

func TestPasswordCannotBeSerializedOrPrintedDirectly(t *testing.T) {
	t.Parallel()

	password, err := secret.NewPassword("super-secret")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(password)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("JSON password = %s", encoded)
	}
	if got := password.String(); strings.Contains(got, "super-secret") {
		t.Fatalf("String() = %q", got)
	}
}

func TestRedactorCoversSecretRepresentationsAndStructuredFields(t *testing.T) {
	t.Parallel()

	const (
		password = "p@ss word/密码"
		token    = "token-value-123"
		hash     = "21cf7a9e37d16fa8b58bbaca7ff6bba339c269535a879ceae7a3df736d00b5e4"
	)
	base64Password := base64.StdEncoding.EncodeToString([]byte(password))
	redactor := secret.NewRedactor(password, token, hash)
	input := strings.Join([]string{
		"password=" + password,
		"base64=" + base64Password,
		";stok=" + token + "/api/xqsystem",
		"?token=" + token + "&key=another-secret&nonce=nonce-value",
		`{"token":"` + token + `","key":"json-key","nonce":"json-nonce"}`,
		"hash=" + hash,
	}, "\n")

	got := redactor.Text(input)
	for _, hidden := range []string{password, base64Password, token, hash, "another-secret", "nonce-value", "json-key", "json-nonce"} {
		if strings.Contains(got, hidden) {
			t.Fatalf("Redactor.Text() still contains %q:\n%s", hidden, got)
		}
	}
	for _, kept := range []string{"password=", "base64=", "/api/xqsystem"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("Redactor.Text() removed non-secret context %q:\n%s", kept, got)
		}
	}
}

func TestRedactorIgnoresEmptySecrets(t *testing.T) {
	t.Parallel()

	const input = "ordinary output"
	if got := secret.NewRedactor("").Text(input); got != input {
		t.Fatalf("Redactor.Text() = %q, want %q", got, input)
	}
}
