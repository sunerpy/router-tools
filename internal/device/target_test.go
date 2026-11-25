package device_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

func TestTargetWebBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target device.Target
		want   string
	}{
		{
			name:   "IPv4",
			target: device.Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23},
			want:   "http://192.168.31.1:80",
		},
		{
			name:   "IPv6",
			target: device.Target{Host: "fd00::1", WebPort: 8080, SSHPort: 22, TelnetPort: 23},
			want:   "http://[fd00::1]:8080",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.target.WebBaseURL()
			if err != nil {
				t.Fatalf("WebBaseURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("WebBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargetRejectsInvalidInputBeforeProbing(t *testing.T) {
	t.Parallel()

	valid := device.Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23}
	tests := []device.Target{
		{Host: "", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		{Host: "http://192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		{Host: "192.168.31.1/path", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		{Host: " 192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		{Host: "192.168.31.1", WebPort: 0, SSHPort: 22, TelnetPort: 23},
		{Host: "192.168.31.1", WebPort: 80, SSHPort: -1, TelnetPort: 23},
		{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 65536},
	}
	for _, target := range tests {
		if err := target.Validate(); err == nil {
			t.Fatalf("Target.Validate(%+v) error = nil; valid target = %+v", target, valid)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid Target.Validate() error = %v", err)
	}
}
