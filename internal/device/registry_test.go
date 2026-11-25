package device_test

import (
	"errors"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

type fakeDriver struct {
	profile device.Profile
	match   device.MatchResult
}

func (d fakeDriver) Profile() device.Profile                     { return d.profile }
func (d fakeDriver) Match(device.Observation) device.MatchResult { return d.match }

func TestRegistryResolve(t *testing.T) {
	t.Parallel()

	rd08 := fakeDriver{
		profile: device.Profile{ID: "xiaomi-rd08-1.1.96"},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	other := fakeDriver{
		profile: device.Profile{ID: "other-model-1.0"},
		match:   device.MatchResult{Level: device.NoMatch},
	}
	registry, err := device.NewRegistry(rd08, other)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got, err := registry.Resolve(device.Observation{Hardware: "RD08"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Profile().ID != rd08.profile.ID {
		t.Fatalf("Resolve() profile = %q, want %q", got.Profile().ID, rd08.profile.ID)
	}
}

func TestRegistryFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		drivers  []device.Driver
		wantCode string
	}{
		{
			name: "unknown device",
			drivers: []device.Driver{fakeDriver{
				profile: device.Profile{ID: "rd08"},
				match:   device.MatchResult{Level: device.NoMatch},
			}},
			wantCode: device.CodeUnsupportedDevice,
		},
		{
			name: "known device with unsupported firmware",
			drivers: []device.Driver{fakeDriver{
				profile: device.Profile{ID: "rd08"},
				match:   device.MatchResult{Level: device.DeviceMatch},
			}},
			wantCode: device.CodeUnsupportedFirmware,
		},
		{
			name: "ambiguous exact matches",
			drivers: []device.Driver{
				fakeDriver{profile: device.Profile{ID: "one"}, match: device.MatchResult{Level: device.ExactMatch}},
				fakeDriver{profile: device.Profile{ID: "two"}, match: device.MatchResult{Level: device.ExactMatch}},
			},
			wantCode: device.CodeAmbiguousDevice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := device.NewRegistry(tt.drivers...)
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}
			_, err = registry.Resolve(device.Observation{})
			assertErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestRegistryResolveProfileDoesNotFallback(t *testing.T) {
	t.Parallel()

	requested := fakeDriver{
		profile: device.Profile{ID: "requested"},
		match:   device.MatchResult{Level: device.DeviceMatch},
	}
	other := fakeDriver{
		profile: device.Profile{ID: "other"},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	registry, err := device.NewRegistry(requested, other)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = registry.ResolveProfile("requested", device.Observation{})
	assertErrorCode(t, err, device.CodeUnsupportedFirmware)
}

func TestRegistryRejectsDuplicateOrInvalidDrivers(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "duplicate"}}
	if _, err := device.NewRegistry(driver, driver); err == nil {
		t.Fatal("NewRegistry() duplicate error = nil")
	}
	if _, err := device.NewRegistry(fakeDriver{}); err == nil {
		t.Fatal("NewRegistry() empty profile error = nil")
	}
}

func TestProfileCapabilities(t *testing.T) {
	t.Parallel()

	profile := device.Profile{
		ID:           "test",
		Capabilities: []device.Capability{device.CapabilityDoctor},
	}
	if !profile.Supports(device.CapabilityDoctor) {
		t.Fatal("Profile.Supports(doctor) = false")
	}
	if profile.Supports(device.CapabilityEnableSSH) {
		t.Fatal("Profile.Supports(enable_ssh) = true for unimplemented capability")
	}
}

func assertErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %q", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) {
		t.Fatalf("error type = %T, want *device.Error", err)
	}
	if deviceErr.Code != want {
		t.Fatalf("error code = %q, want %q", deviceErr.Code, want)
	}
}
