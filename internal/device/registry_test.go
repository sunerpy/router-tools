package device_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

type fakeDriver struct {
	profile device.Profile
	match   device.MatchResult
}

func (d fakeDriver) Profile() device.Profile                     { return d.profile }
func (d fakeDriver) Match(device.Observation) device.MatchResult { return d.match }

// fakeOps stands in for an adapter implementation; the registry never inspects it.
type fakeOps struct{ name string }

func profile(modelID, firmware string, capabilities ...device.Capability) device.Profile {
	return device.Profile{
		ID:           modelID + "-" + firmware,
		ModelID:      modelID,
		Vendor:       "Vendor " + modelID,
		Model:        "Model " + modelID,
		Hardware:     strings.ToUpper(modelID),
		Firmware:     firmware,
		Capabilities: capabilities,
	}
}

func adapter(p device.Profile, level device.MatchLevel) device.Adapter {
	return device.Adapter{
		Driver:     fakeDriver{profile: p, match: device.MatchResult{Level: level}},
		Operations: fakeOps{name: p.ID},
	}
}

func TestRegistryResolve(t *testing.T) {
	t.Parallel()

	rd08 := adapter(profile("xiaomi-rd08", "1.1.96", device.CapabilityDoctor), device.ExactMatch)
	other := adapter(profile("other-model", "1.0", device.CapabilityDoctor), device.NoMatch)
	registry, err := device.NewRegistry(rd08, other)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got, err := registry.Resolve(device.Observation{Hardware: "RD08"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Profile().ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("Resolve() profile = %q", got.Profile().ID)
	}
	if got.Operations != (fakeOps{name: "xiaomi-rd08-1.1.96"}) {
		t.Fatalf("Resolve() operations = %#v, want the resolved profile's own implementation", got.Operations)
	}
}

func TestRegistryFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		adapters []device.Adapter
		wantCode string
	}{
		{
			name:     "unknown device",
			adapters: []device.Adapter{adapter(profile("rd08", "1"), device.NoMatch)},
			wantCode: device.CodeUnsupportedDevice,
		},
		{
			name:     "known device with unsupported firmware",
			adapters: []device.Adapter{adapter(profile("rd08", "1"), device.DeviceMatch)},
			wantCode: device.CodeUnsupportedFirmware,
		},
		{
			name: "ambiguous exact matches",
			adapters: []device.Adapter{
				adapter(profile("one", "1"), device.ExactMatch),
				adapter(profile("two", "1"), device.ExactMatch),
			},
			wantCode: device.CodeAmbiguousDevice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := device.NewRegistry(tt.adapters...)
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

	requested := adapter(profile("requested", "1"), device.DeviceMatch)
	other := adapter(profile("other", "1"), device.ExactMatch)
	registry, err := device.NewRegistry(requested, other)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = registry.ResolveProfile("requested-1", device.Observation{})
	assertErrorCode(t, err, device.CodeUnsupportedFirmware)
	_, err = registry.ResolveProfile("missing-1", device.Observation{})
	assertErrorCode(t, err, device.CodeUnsupportedProfile)
}

// Two firmware profiles of one model: auto takes only the exact one; an explicit profile never moves to
// its sibling, and an unregistered firmware stays unsupported.
func TestRegistryTwoFirmwaresOfOneModel(t *testing.T) {
	t.Parallel()

	v96 := device.Adapter{Driver: firmwareDriver{p: profile("xiaomi-rd08", "1.1.96")}, Operations: fakeOps{name: "v96"}}
	v97 := device.Adapter{Driver: firmwareDriver{p: profile("xiaomi-rd08", "1.1.97")}, Operations: fakeOps{name: "v97"}}
	registry, err := device.NewRegistry(v96, v97)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got, err := registry.Resolve(device.Observation{Hardware: "XIAOMI-RD08", ROMVersion: "1.1.97"})
	if err != nil || got.Operations != (fakeOps{name: "v97"}) {
		t.Fatalf("Resolve(1.1.97) = %#v, %v; want the 1.1.97 implementation", got.Operations, err)
	}
	_, err = registry.Resolve(device.Observation{Hardware: "XIAOMI-RD08", ROMVersion: "1.1.98"})
	assertErrorCode(t, err, device.CodeUnsupportedFirmware)
	_, err = registry.ResolveProfile("xiaomi-rd08-1.1.96", device.Observation{Hardware: "XIAOMI-RD08", ROMVersion: "1.1.97"})
	assertErrorCode(t, err, device.CodeUnsupportedFirmware)

	candidates := registry.Candidates(device.Observation{Hardware: "XIAOMI-RD08", ROMVersion: "1.1.97"})
	if len(candidates) != 2 || candidates[0].Profile.ID != "xiaomi-rd08-1.1.96" || candidates[0].Exact ||
		candidates[1].Profile.ID != "xiaomi-rd08-1.1.97" || !candidates[1].Exact {
		t.Fatalf("Candidates() = %+v, want both firmwares in registration order with only 1.1.97 exact", candidates)
	}
	if got := registry.Candidates(device.Observation{Hardware: "OTHER"}); len(got) != 0 {
		t.Fatalf("Candidates(unknown) = %+v, want none", got)
	}
}

// firmwareDriver matches its model by hardware and its firmware exactly, like the RD08 driver.
type firmwareDriver struct{ p device.Profile }

func (d firmwareDriver) Profile() device.Profile { return d.p }
func (d firmwareDriver) Match(o device.Observation) device.MatchResult {
	if !strings.EqualFold(o.Hardware, d.p.Hardware) {
		return device.MatchResult{Level: device.NoMatch}
	}
	if o.ROMVersion != d.p.Firmware {
		return device.MatchResult{Level: device.DeviceMatch}
	}
	return device.MatchResult{Level: device.ExactMatch}
}

func TestRegistryCatalogAndLookup(t *testing.T) {
	t.Parallel()

	a96 := adapter(profile("xiaomi-rd08", "1.1.96", device.CapabilityDoctor), device.NoMatch)
	other := adapter(profile("acme-x1", "2.0", device.CapabilityDoctor), device.NoMatch)
	a97 := adapter(profile("xiaomi-rd08", "1.1.97", device.CapabilityDoctor), device.NoMatch)
	registry, err := device.NewRegistry(a96, other, a97)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	catalog := registry.Catalog()
	if len(catalog) != 2 {
		t.Fatalf("Catalog() = %+v, want two models", catalog)
	}
	first := catalog[0]
	if first.ModelID != "xiaomi-rd08" || first.Vendor != "Vendor xiaomi-rd08" || first.Model != "Model xiaomi-rd08" ||
		first.Hardware != "XIAOMI-RD08" || len(first.Profiles) != 2 ||
		first.Profiles[0].Firmware != "1.1.96" || first.Profiles[1].Firmware != "1.1.97" {
		t.Fatalf("Catalog()[0] = %+v, want RD08 with 1.1.96 then 1.1.97", first)
	}
	if catalog[1].ModelID != "acme-x1" || len(catalog[1].Profiles) != 1 {
		t.Fatalf("Catalog()[1] = %+v, want acme-x1", catalog[1])
	}

	got, ok := registry.Lookup("xiaomi-rd08-1.1.97")
	if !ok || got.Profile().Firmware != "1.1.97" {
		t.Fatalf("Lookup(1.1.97) = %+v, %v", got.Profile(), ok)
	}
	if _, ok := registry.Lookup("auto"); ok {
		t.Fatal("Lookup(auto) = true, want only registered profile IDs")
	}
}

func TestRegistryRejectsInvalidAdapters(t *testing.T) {
	t.Parallel()

	deep := profile("rd08", "1", device.CapabilityDoctor, device.CapabilityDeepPersist)
	deep.Deep = &device.DeepPolicy{Acknowledgement: "RD08-DEEP", RebootTransitions: 3}
	valid := adapter(deep, device.NoMatch)
	if _, err := device.NewRegistry(valid); err != nil {
		t.Fatalf("NewRegistry(valid deep profile) error = %v", err)
	}

	mutate := func(change func(*device.Profile)) device.Adapter {
		p := profile("rd08", "1", device.CapabilityDoctor, device.CapabilityEnableSSH)
		change(&p)
		return adapter(p, device.NoMatch)
	}
	tests := []struct {
		name     string
		adapters []device.Adapter
	}{
		{name: "nil driver", adapters: []device.Adapter{{Operations: fakeOps{}}}},
		{name: "empty ID", adapters: []device.Adapter{mutate(func(p *device.Profile) { p.ID = "" })}},
		{name: "empty model ID", adapters: []device.Adapter{mutate(func(p *device.Profile) { p.ModelID, p.ID = "", "-1" })}},
		{name: "empty firmware", adapters: []device.Adapter{mutate(func(p *device.Profile) { p.Firmware, p.ID = "", "rd08-" })}},
		{name: "ID is not model-firmware", adapters: []device.Adapter{mutate(func(p *device.Profile) { p.ID = "rd08-2" })}},
		{name: "duplicate ID", adapters: []device.Adapter{mutate(func(*device.Profile) {}), mutate(func(*device.Profile) {})}},
		{
			name: "model metadata differs between firmwares",
			adapters: []device.Adapter{
				mutate(func(*device.Profile) {}),
				mutate(func(p *device.Profile) { p.Firmware, p.ID, p.Model = "2", "rd08-2", "Renamed" }),
			},
		},
		{name: "deep declared without policy", adapters: []device.Adapter{mutate(func(p *device.Profile) {
			p.Capabilities = append(p.Capabilities, device.CapabilityDeepPersist)
		})}},
		{name: "deep policy without capability", adapters: []device.Adapter{mutate(func(p *device.Profile) {
			p.Deep = &device.DeepPolicy{Acknowledgement: "RD08-DEEP", RebootTransitions: 3}
		})}},
		{name: "deep policy without phrase", adapters: []device.Adapter{mutate(func(p *device.Profile) {
			p.Capabilities = append(p.Capabilities, device.CapabilityDeepPersist)
			p.Deep = &device.DeepPolicy{RebootTransitions: 3}
		})}},
		{name: "deep policy without reboot count", adapters: []device.Adapter{mutate(func(p *device.Profile) {
			p.Capabilities = append(p.Capabilities, device.CapabilityDeepPersist)
			p.Deep = &device.DeepPolicy{Acknowledgement: "RD08-DEEP"}
		})}},
		{name: "operations missing for declared capability", adapters: []device.Adapter{{
			Driver: fakeDriver{profile: profile("rd08", "1", device.CapabilityDoctor, device.CapabilityVerify)},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := device.NewRegistry(tt.adapters...); err == nil {
				t.Fatal("NewRegistry() error = nil, want a registration failure")
			}
		})
	}

	doctorOnly := device.Adapter{Driver: fakeDriver{profile: profile("probe-only", "1", device.CapabilityDoctor)}}
	if _, err := device.NewRegistry(doctorOnly); err != nil {
		t.Fatalf("NewRegistry(doctor-only without operations) error = %v", err)
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
