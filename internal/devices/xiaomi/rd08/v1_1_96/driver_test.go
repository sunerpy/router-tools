package rd08v1196_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	rd08v1196 "github.com/sunerpy/router-tools/internal/devices/xiaomi/rd08/v1_1_96"
)

func TestDriverMatch(t *testing.T) {
	t.Parallel()

	driver := rd08v1196.New()
	tests := []struct {
		name string
		obs  device.Observation
		want device.MatchLevel
	}{
		{name: "hardware and firmware", obs: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"}, want: device.ExactMatch},
		{name: "hardware case insensitive and firmware trimmed", obs: device.Observation{Hardware: "rd08", ROMVersion: " 1.1.96 "}, want: device.ExactMatch},
		{name: "display name fallback", obs: device.Observation{DisplayName: "Xiaomi Router BE6500 Pro", ROMVersion: "1.1.96"}, want: device.ExactMatch},
		{name: "hardware has priority over display name", obs: device.Observation{Hardware: "RD09", DisplayName: "BE6500 Pro", ROMVersion: "1.1.96"}, want: device.NoMatch},
		{name: "known hardware with new firmware", obs: device.Observation{Hardware: "RD08", ROMVersion: "1.1.97"}, want: device.DeviceMatch},
		{name: "known hardware with missing firmware", obs: device.Observation{Hardware: "RD08"}, want: device.DeviceMatch},
		{name: "unknown device", obs: device.Observation{DisplayName: "Another Router", ROMVersion: "1.1.96"}, want: device.NoMatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driver.Match(tt.obs)
			if got.Level != tt.want {
				t.Fatalf("Match() level = %v, want %v", got.Level, tt.want)
			}
		})
	}
}

func TestDriverProfile(t *testing.T) {
	t.Parallel()

	profile := rd08v1196.New().Profile()
	if profile.ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("Profile().ID = %q", profile.ID)
	}
	if !profile.Supports(device.CapabilityDoctor) {
		t.Fatal("RD08 profile does not advertise doctor")
	}
	if !profile.Supports(device.CapabilityWebAuth) {
		t.Fatal("RD08 profile does not advertise Web authentication")
	}
	if profile.ModelID != "xiaomi-rd08" || profile.ID != profile.ModelID+"-"+profile.Firmware {
		t.Fatalf("Profile() model ID = %q, ID = %q", profile.ModelID, profile.ID)
	}
	if profile.Deep == nil || profile.Deep.Acknowledgement != "RD08-DEEP" || profile.Deep.RebootTransitions != 3 {
		t.Fatalf("Profile().Deep = %+v, want RD08-DEEP with three reboots", profile.Deep)
	}
	adapter := rd08v1196.Adapter()
	if adapter.Operations == nil || adapter.Profile().ID != profile.ID {
		t.Fatalf("Adapter() = %+v", adapter)
	}
	if _, err := device.NewRegistry(adapter); err != nil {
		t.Fatalf("NewRegistry(Adapter()) error = %v", err)
	}
}
