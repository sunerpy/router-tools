package routerweb_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
)

func TestClientProbeUsesCookieSessionAndReadOnlyRequests(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("request method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test-session", Path: "/"})
			_, _ = w.Write([]byte("var deviceId='AABBCCDDEEFF'; {key:'secret-key',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "test-session" {
				t.Errorf("init_info session cookie = %v, %v", cookie, err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1,"displayName":"Xiaomi Router BE6500 Pro","hardware":"RD08","romversion":"1.1.96"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	observation, err := client.Probe(t.Context())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("request count = %d, want 2", requests.Load())
	}
	if observation.DeviceID != "AABBCCDDEEFF" || observation.Hardware != "RD08" || observation.ROMVersion != "1.1.96" {
		t.Fatalf("Probe() observation = %+v", observation)
	}
	if observation.NewEncryptMode != int(webauth.EncryptionSHA256) {
		t.Fatalf("NewEncryptMode = %d", observation.NewEncryptMode)
	}
}

func TestClientRejectsUnsafeBaseURL(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"https://192.168.31.1",
		"http://user:password@192.168.31.1",
		"http://192.168.31.1/path?token=secret",
		"192.168.31.1",
	} {
		if _, err := routerweb.NewClient(rawURL); err == nil {
			t.Fatalf("NewClient(%q) error = nil", rawURL)
		}
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", "/cgi-bin/luci/api/xqsystem/init_info")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Probe(t.Context()); err == nil {
		t.Fatal("Probe() redirect error = nil")
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d, want no redirect follow", requests.Load())
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 1<<20+1)))
	}))
	defer server.Close()

	client, err := routerweb.NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Probe(t.Context()); err == nil {
		t.Fatal("Probe() oversized response error = nil")
	}
}
