package routerweb_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/secret"
)

func TestSessionMACRules(t *testing.T) {
	t.Parallel()

	var adds atomic.Int32
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			_, _ = w.Write([]byte("var deviceId='DEVICE'; {key:'test-key',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1}`))
		case "/cgi-bin/luci/api/xqsystem/login":
			_, _ = w.Write([]byte(`{"code":0,"token":"secret/token"}`))
		case "/cgi-bin/luci/;stok=secret/token/api/xqsystem/set_macfilter_rules":
			if !strings.Contains(r.URL.EscapedPath(), "secret%2Ftoken") {
				t.Errorf("stok was not path escaped: %s", r.URL.EscapedPath())
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			switch r.PostForm.Get("option") {
			case "add":
				adds.Add(1)
				if r.PostForm.Get("mac") != "0A:11:22:33:44:10" || r.PostForm.Get("name") != "x$(nvram set ssh_en=1)y" {
					t.Errorf("add form = %v", r.PostForm)
				}
			case "del":
				deletes.Add(1)
				if r.PostForm.Get("name") != "x" {
					t.Errorf("delete form = %v", r.PostForm)
				}
			}
			_, _ = w.Write([]byte(`{"code":0}`))
		default:
			t.Errorf("unexpected path = %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := secret.NewPassword("admin")
	session, err := client.Login(t.Context(), password)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.AddMACRule(t.Context(), "0A:11:22:33:44:10", "nvram set ssh_en=1"); err != nil {
		t.Fatalf("AddMACRule() error = %v", err)
	}
	if err := session.DeleteMACRule(t.Context(), "0A:11:22:33:44:10"); err != nil {
		t.Fatalf("DeleteMACRule() error = %v", err)
	}
	_ = session.AddMACRule(t.Context(), "invalid", "true")
	_ = session.AddMACRule(t.Context(), "0A:11:22:33:44:11", "bad;command")
	if adds.Load() != 1 || deletes.Load() != 1 {
		t.Fatalf("requests = add:%d delete:%d; invalid input triggered a request", adds.Load(), deletes.Load())
	}
}

func TestMACAddNetworkFailureIsUnknownAndNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			_, _ = w.Write([]byte("var deviceId='DEVICE'; {key:'test-key',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1}`))
		case "/cgi-bin/luci/api/xqsystem/login":
			_, _ = w.Write([]byte(`{"code":0,"token":"token"}`))
		default:
			calls.Add(1)
			panic("close connection")
		}
	}))
	defer server.Close()
	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := secret.NewPassword("admin")
	session, err := client.Login(t.Context(), password)
	if err != nil {
		t.Fatal(err)
	}
	err = session.AddMACRule(t.Context(), "0A:11:22:33:44:10", "true")
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeRemoteResultUnknown {
		t.Fatalf("AddMACRule() error = %T %v", err, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("add calls = %d, want no retry", calls.Load())
	}
	if strings.Contains(err.Error(), "token") {
		t.Fatalf("error contains token: %v", err)
	}
}
