package device

import "fmt"

// Registry contains statically registered device adapters.
type Registry struct {
	drivers map[string]Driver
}

// NewRegistry validates and registers adapters. Duplicate profile IDs are rejected.
func NewRegistry(drivers ...Driver) (*Registry, error) {
	registry := &Registry{drivers: make(map[string]Driver, len(drivers))}
	for _, driver := range drivers {
		if driver == nil {
			return nil, fmt.Errorf("register device driver: nil driver")
		}
		profile := driver.Profile()
		if profile.ID == "" {
			return nil, fmt.Errorf("register device driver: empty profile ID")
		}
		if _, exists := registry.drivers[profile.ID]; exists {
			return nil, fmt.Errorf("register device driver: duplicate profile ID %q", profile.ID)
		}
		registry.drivers[profile.ID] = driver
	}
	return registry, nil
}

// Resolve returns the only exact match, failing closed on unknown or ambiguous devices.
func (r *Registry) Resolve(observation Observation) (Driver, error) {
	var exact []Driver
	deviceKnown := false
	for _, driver := range r.drivers {
		switch driver.Match(observation).Level {
		case ExactMatch:
			exact = append(exact, driver)
		case DeviceMatch:
			deviceKnown = true
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return nil, newError(CodeAmbiguousDevice, "multiple adapters exactly match the device")
	}
	if deviceKnown {
		return nil, newError(CodeUnsupportedFirmware, "the device model is known but its firmware is not supported")
	}
	return nil, newError(CodeUnsupportedDevice, "the device model is not supported")
}

// ResolveProfile applies only the requested adapter and never falls back to another profile.
func (r *Registry) ResolveProfile(profileID string, observation Observation) (Driver, error) {
	driver, ok := r.drivers[profileID]
	if !ok {
		return nil, newError(CodeUnsupportedProfile, "the requested profile is not registered")
	}
	switch driver.Match(observation).Level {
	case ExactMatch:
		return driver, nil
	case DeviceMatch:
		return nil, newError(CodeUnsupportedFirmware, "the requested profile does not support this firmware")
	default:
		return nil, newError(CodeUnsupportedDevice, "the requested profile does not support this device")
	}
}
