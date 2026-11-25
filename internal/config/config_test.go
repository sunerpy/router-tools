package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sunerpy/router-tools/internal/config"
	"github.com/sunerpy/router-tools/internal/device"
)

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"schema_version":1,"host":"router.local","web_port":8080,"ssh_port":22,"telnet_port":23,"profile":"auto","log_level":"info","known_hosts_file":"/tmp/known_hosts"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "router.local" || cfg.WebPort != 8080 || cfg.KnownHostsFile != "/tmp/known_hosts" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadRejectsUnknownAndSecretFields(t *testing.T) {
	t.Parallel()

	for _, data := range []string{
		`{"schema_version":1,"unknown":true}`,
		`{"schema_version":1,"password":"secret"}`,
		`{"schema_version":1,"stok":"secret"}`,
		`{"schema_version":2}`,
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := config.Load(path)
		if err == nil {
			t.Fatalf("Load(%s) error = nil", data)
		}
		var deviceErr *device.Error
		if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeInvalidArgument {
			t.Fatalf("Load() error = %T %v", err, err)
		}
	}
}
