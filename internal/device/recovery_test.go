package device_test

import (
	"regexp"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

func TestRecoveryKeyIdentifiesTheHostOnly(t *testing.T) {
	t.Parallel()

	key := func(host string, ports ...int) string {
		target := device.Target{Host: host, WebPort: 80, SSHPort: 22, TelnetPort: 23}
		if len(ports) == 3 {
			target.WebPort, target.SSHPort, target.TelnetPort = ports[0], ports[1], ports[2]
		}
		return device.RecoveryKey(target)
	}
	base := key("192.168.31.1")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(base) {
		t.Fatalf("RecoveryKey() = %q, want 64 lowercase hex characters", base)
	}
	same := map[string]string{
		"other ports":         key("192.168.31.1", 8080, 2222, 2323),
		"IPv4-mapped IPv6":    key("::ffff:192.168.31.1"),
		"IPv6 spelling":       key("FE80:0:0:0:0:0:0:1"),
		"host name case":      key("Router.Local"),
		"trailing dot":        key("router.local."),
		"IPv6 compressed":     key("fe80::1"),
		"host name lowercase": key("router.local"),
	}
	if same["other ports"] != base || same["IPv4-mapped IPv6"] != base {
		t.Fatalf("ports or IPv4 spelling changed the key: %+v", same)
	}
	if same["IPv6 spelling"] != same["IPv6 compressed"] {
		t.Fatal("two spellings of one IPv6 address produced different keys")
	}
	if same["host name case"] != same["host name lowercase"] || same["trailing dot"] != same["host name lowercase"] {
		t.Fatal("host name case or a trailing dot changed the key")
	}
	if key("192.168.31.2") == base || same["host name lowercase"] == base {
		t.Fatal("different hosts share a recovery key")
	}
}
