package rd08v1196_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	rd08v1196 "github.com/sunerpy/router-tools/internal/devices/xiaomi/rd08/v1_1_96"
	"github.com/sunerpy/router-tools/internal/secret"
)

func TestProberVerifyAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		loginBody string
		wantCode  string
	}{
		{name: "success", loginBody: `{"code":0,"token":"session-token"}`},
		{name: "rejected", loginBody: `{"code":1}`, wantCode: device.CodeWebAuthFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/cgi-bin/luci/web":
					_, _ = w.Write([]byte("var deviceId='DEVICE'; {key:'test-key',}"))
				case "/cgi-bin/luci/api/xqsystem/init_info":
					_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":0}`))
				case "/cgi-bin/luci/api/xqsystem/login":
					_, _ = w.Write([]byte(tt.loginBody))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			prober := rd08v1196.NewProber(nil)
			password, _ := secret.NewPassword("admin-password")
			err := prober.VerifyAdmin(t.Context(), targetForServer(t, server.URL), password)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("VerifyAdmin() error = %v", err)
				}
				return
			}
			var deviceErr *device.Error
			if !errors.As(err, &deviceErr) || deviceErr.Code != tt.wantCode {
				t.Fatalf("VerifyAdmin() error = %T %v, want %s", err, err, tt.wantCode)
			}
			if strings.Contains(err.Error(), "session-token") || strings.Contains(err.Error(), "admin-password") {
				t.Fatalf("VerifyAdmin() error leaks secret: %v", err)
			}
		})
	}
}

func TestProberVerifyAdminRejectsInvalidPasswordBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()
	prober := rd08v1196.NewProber(nil)
	err := prober.VerifyAdmin(t.Context(), targetForServer(t, server.URL), secret.Password{})
	if err == nil {
		t.Fatal("VerifyAdmin() error = nil")
	}
	if requests.Load() != 0 {
		t.Fatalf("request count = %d, want 0", requests.Load())
	}
}
