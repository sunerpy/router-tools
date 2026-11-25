package routerweb_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
)

func TestParseLoginPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantDevice  string
		wantKey     string
		wantFailure bool
	}{
		{
			name:       "single quotes with whitespace",
			body:       "var deviceId = 'AABBCCDDEEFF'; var config = { key: 'test-key', };",
			wantDevice: "AABBCCDDEEFF",
			wantKey:    "test-key",
		},
		{
			name: "double quotes",
			body: `var deviceId="DEVICE-1";
{key : "key-1",}`,
			wantDevice: "DEVICE-1",
			wantKey:    "key-1",
		},
		{
			name:       "consistent duplicates",
			body:       `var deviceId='DEVICE-1'; var deviceId = "DEVICE-1"; {key:'same',key : "same",}`,
			wantDevice: "DEVICE-1",
			wantKey:    "same",
		},
		{name: "missing device ID", body: "{key:'key'}", wantFailure: true},
		{name: "empty key", body: "var deviceId='DEVICE'; {key:'',}", wantFailure: true},
		{
			name:        "conflicting device IDs",
			body:        "var deviceId='ONE'; var deviceId='TWO'; {key:'key'}",
			wantFailure: true,
		},
		{
			name:        "conflicting keys",
			body:        "var deviceId='DEVICE'; {key:'one'}; {key:'two'}",
			wantFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := routerweb.ParseLoginPage([]byte(tt.body))
			if tt.wantFailure {
				if err == nil {
					t.Fatalf("ParseLoginPage() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLoginPage() error = %v", err)
			}
			if got.DeviceID != tt.wantDevice || got.Key != tt.wantKey {
				t.Fatalf("ParseLoginPage() = {DeviceID:%q Key:%q}, want {%q %q}", got.DeviceID, got.Key, tt.wantDevice, tt.wantKey)
			}
		})
	}
}
