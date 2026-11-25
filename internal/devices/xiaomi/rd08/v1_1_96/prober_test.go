package rd08v1196_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	rd08v1196 "github.com/sunerpy/router-tools/internal/devices/xiaomi/rd08/v1_1_96"
)

func TestProberChecksWebThenHTTPThenAuxiliaryPorts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/cgi-bin/luci/web":
			_, _ = w.Write([]byte("var deviceId='DEVICE'; {key:'secret',}"))
		case "/cgi-bin/luci/api/xqsystem/init_info":
			_, _ = w.Write([]byte(`{"code":0,"newEncryptMode":1,"hardware":"RD08","romversion":"1.1.96"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := targetForServer(t, server.URL)
	var ports []int
	prober := rd08v1196.NewProber(func(_ context.Context, _ string, port int) device.PortState {
		ports = append(ports, port)
		if port == target.TelnetPort {
			return device.PortClosed
		}
		return device.PortOpen
	})

	result, err := prober.Probe(t.Context(), target)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	wantPorts := []int{target.WebPort, target.SSHPort, target.TelnetPort}
	if !reflect.DeepEqual(ports, wantPorts) {
		t.Fatalf("port probe order = %v, want %v", ports, wantPorts)
	}
	if result.Observation.Hardware != "RD08" || result.SSH != device.PortOpen || result.Telnet != device.PortClosed {
		t.Fatalf("Probe() result = %+v", result)
	}
}

func TestProberStopsWhenWebTCPPortIsUnavailable(t *testing.T) {
	t.Parallel()

	prober := rd08v1196.NewProber(func(context.Context, string, int) device.PortState {
		return device.PortClosed
	})
	_, err := prober.Probe(t.Context(), device.Target{Host: "192.0.2.1", WebPort: 80, SSHPort: 22, TelnetPort: 23})
	if err == nil {
		t.Fatal("Probe() error = nil")
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeWebUnreachable {
		t.Fatalf("Probe() error = %T %v, want WEB_UNREACHABLE", err, err)
	}
}

func TestProberPreservesCancellation(t *testing.T) {
	t.Parallel()

	prober := rd08v1196.NewProber(func(context.Context, string, int) device.PortState {
		return device.PortOpen
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := prober.Probe(ctx, device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23})
	if err == nil {
		t.Fatal("Probe() error = nil")
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeCanceled {
		t.Fatalf("Probe() error = %T %v, want CANCELED", err, err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Probe() error = %v, want context.Canceled in chain", err)
	}
}

func targetForServer(t *testing.T, rawURL string) device.Target {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return device.Target{Host: host, WebPort: port, SSHPort: 22, TelnetPort: 23}
}
