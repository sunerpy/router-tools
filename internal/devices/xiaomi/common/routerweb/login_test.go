package routerweb_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/secret"
)

const (
	testNonce = "0_AABBCCDDEEFF_1700000000_4321"
	testToken = "stok-secret-value"
)

func TestClientLoginUsesFixedFormAndProtectsSessionToken(t *testing.T) {
	t.Parallel()

	var loginRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test-session", Path: "/"})
			_, _ = w.Write([]byte("var deviceId='AABBCCDDEEFF'; {key:'test-key',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1}`))
		case "/cgi-bin/luci/api/xqsystem/login":
			loginRequests.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("login method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %q", got)
			}
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "test-session" {
				t.Errorf("login session cookie = %v, %v", cookie, err)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() error = %v", err)
			}
			wantForm := map[string]string{
				"username": "admin",
				"password": "21cf7a9e37d16fa8b58bbaca7ff6bba339c269535a879ceae7a3df736d00b5e4",
				"logtype":  "2",
				"nonce":    testNonce,
			}
			for field, want := range wantForm {
				if got := r.PostForm.Get(field); got != want {
					t.Errorf("form %s = %q, want %q", field, got, want)
				}
			}
			_, _ = w.Write([]byte(`{"code":0,"token":"` + testToken + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := routerweb.NewClientWithOptions(server.URL, routerweb.ClientOptions{
		NonceFactory: func(deviceID string) (string, error) {
			if deviceID != "AABBCCDDEEFF" {
				t.Errorf("NonceFactory deviceID = %q", deviceID)
			}
			return testNonce, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	password, _ := secret.NewPassword("admin")
	session, err := client.Login(t.Context(), password)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	defer session.Close()
	if loginRequests.Load() != 1 {
		t.Fatalf("login request count = %d", loginRequests.Load())
	}
	if !session.Authenticated() {
		t.Fatal("session is not authenticated")
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{string(encoded), fmt.Sprintf("%+v", session), fmt.Sprintf("%#v", session)} {
		if strings.Contains(output, testToken) {
			t.Fatalf("session output leaks token: %s", output)
		}
	}
	session.Close()
	if session.Authenticated() {
		t.Fatal("session remains authenticated after Close")
	}
}

func TestClientLoginClassifiesFailuresWithoutLeakingSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		wantCode string
	}{
		{name: "authentication rejected", response: `{"code":1,"token":"do-not-use"}`, wantCode: device.CodeWebAuthFailed},
		{name: "string authentication rejected", response: `{"code":"1","token":"do-not-use"}`, wantCode: device.CodeWebAuthFailed},
		{name: "missing token", response: `{"code":0}`, wantCode: device.CodeWebProtocolError},
		{name: "empty token", response: `{"code":0,"token":""}`, wantCode: device.CodeWebProtocolError},
		{name: "wrong token type", response: `{"code":0,"token":123}`, wantCode: device.CodeWebProtocolError},
		{name: "missing code", response: `{"token":"do-not-use"}`, wantCode: device.CodeWebProtocolError},
		{name: "malformed JSON", response: `{`, wantCode: device.CodeWebProtocolError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newLoginServer(t, tt.response)
			defer server.Close()
			client, err := routerweb.NewClientWithOptions(server.URL, routerweb.ClientOptions{
				NonceFactory: func(string) (string, error) { return testNonce, nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			password, _ := secret.NewPassword("admin-password-that-must-not-leak")
			_, err = client.Login(t.Context(), password)
			if err == nil {
				t.Fatal("Login() error = nil")
			}
			var deviceErr *device.Error
			if !errors.As(err, &deviceErr) || deviceErr.Code != tt.wantCode {
				t.Fatalf("Login() error = %T %v, want %s", err, err, tt.wantCode)
			}
			for _, hidden := range []string{"admin-password-that-must-not-leak", "do-not-use", testNonce} {
				if strings.Contains(err.Error(), hidden) {
					t.Fatalf("Login() error leaks %q: %v", hidden, err)
				}
			}
		})
	}
}

func TestClientLoginRejectsInvalidPasswordBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Login(t.Context(), secret.Password{}); err == nil {
		t.Fatal("Login() zero password error = nil")
	}
	if requests.Load() != 0 {
		t.Fatalf("request count = %d, want 0", requests.Load())
	}
}

func newLoginServer(t *testing.T, response string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			_, _ = w.Write([]byte("var deviceId='AABBCCDDEEFF'; {key:'test-key',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1}`))
		case "/cgi-bin/luci/api/xqsystem/login":
			_, _ = w.Write([]byte(response))
		default:
			http.NotFound(w, r)
		}
	}))
}
