package routerweb_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
)

func TestParseInitInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		wantMode webauth.EncryptionMode
	}{
		{name: "numeric SHA-256 mode", body: `{"code":0,"newEncryptMode":1,"displayName":"Xiaomi Router BE6500 Pro","hardware":"RD08","romversion":"1.1.96"}`, wantMode: webauth.EncryptionSHA256},
		{name: "string SHA-256 mode", body: `{"code":"0","newEncryptMode":"1","hardware":"RD08","romversion":"1.1.96"}`, wantMode: webauth.EncryptionSHA256},
		{name: "other defined mode uses SHA-1", body: `{"code":0,"newEncryptMode":0,"hardware":"RD08"}`, wantMode: webauth.EncryptionSHA1},
		{name: "negative defined mode uses SHA-1", body: `{"code":0,"newEncryptMode":-1}`, wantMode: webauth.EncryptionSHA1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := routerweb.ParseInitInfo([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseInitInfo() error = %v", err)
			}
			if got.NewEncryptMode != tt.wantMode {
				t.Fatalf("NewEncryptMode = %v, want %v", got.NewEncryptMode, tt.wantMode)
			}
		})
	}

	got, err := routerweb.ParseInitInfo([]byte(`{"code":0,"newEncryptMode":1,"displayName":"Xiaomi Router BE6500 Pro","hardware":"RD08","romversion":"1.1.96"}`))
	if err != nil {
		t.Fatalf("ParseInitInfo() full fixture error = %v", err)
	}
	if got.Hardware != "RD08" || got.DisplayName != "Xiaomi Router BE6500 Pro" || got.ROMVersion != "1.1.96" {
		t.Fatalf("ParseInitInfo() public fields = %+v", got)
	}
}

func TestParseInitInfoRejectsInvalidProtocolData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "missing API code", body: `{"newEncryptMode":1}`},
		{name: "numeric rejected API code", body: `{"code":1,"newEncryptMode":1}`},
		{name: "noncanonical string API code", body: `{"code":"00","newEncryptMode":1}`},
		{name: "missing encryption mode", body: `{"code":0}`},
		{name: "boolean encryption mode", body: `{"code":0,"newEncryptMode":true}`},
		{name: "fractional encryption mode", body: `{"code":0,"newEncryptMode":1.5}`},
		{name: "unknown string encryption mode", body: `{"code":0,"newEncryptMode":"sha256"}`},
		{name: "top-level array", body: `[]`},
		{name: "trailing JSON value", body: `{"code":0,"newEncryptMode":1} {}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := routerweb.ParseInitInfo([]byte(tt.body)); err == nil {
				t.Fatal("ParseInitInfo() error = nil")
			}
		})
	}
}
