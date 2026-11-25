package device

import "fmt"

// Adapter pairs one exact profile's compatibility policy with the implementation that serves only that
// profile. Operations implements the small capability interfaces declared in internal/app; the registry
// never inspects it, and nothing falls back to another adapter's implementation.
type Adapter struct {
	Driver
	Operations any
}

// ProfileMatch is a registered profile whose model matches an observation. It explains compatibility
// failures and is never used to choose an adapter.
type ProfileMatch struct {
	Profile Profile `json:"profile"`
	Exact   bool    `json:"exact"`
}

// CatalogEntry lists the registered firmware profiles of one device model in registration order.
type CatalogEntry struct {
	ModelID  string    `json:"model_id"`
	Vendor   string    `json:"vendor"`
	Model    string    `json:"model"`
	Hardware string    `json:"hardware"`
	Profiles []Profile `json:"profiles"`
}

// Registry contains statically registered device adapters.
type Registry struct {
	adapters map[string]Adapter
	order    []string
}

// NewRegistry validates and registers adapters, failing closed on any inconsistent registration.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[string]Adapter, len(adapters))}
	models := make(map[string]Profile)
	for _, adapter := range adapters {
		if adapter.Driver == nil {
			return nil, fmt.Errorf("register device adapter: nil driver")
		}
		profile := adapter.Profile()
		if err := validateProfile(profile, adapter.Operations); err != nil {
			return nil, fmt.Errorf("register device adapter %q: %w", profile.ID, err)
		}
		if _, exists := registry.adapters[profile.ID]; exists {
			return nil, fmt.Errorf("register device adapter: duplicate profile ID %q", profile.ID)
		}
		if first, known := models[profile.ModelID]; known {
			if first.Vendor != profile.Vendor || first.Model != profile.Model || first.Hardware != profile.Hardware {
				return nil, fmt.Errorf("register device adapter %q: model %q metadata differs from %q", profile.ID, profile.ModelID, first.ID)
			}
		} else {
			models[profile.ModelID] = profile
		}
		registry.adapters[profile.ID] = adapter
		registry.order = append(registry.order, profile.ID)
	}
	return registry, nil
}

func validateProfile(profile Profile, operations any) error {
	switch {
	case profile.ID == "":
		return fmt.Errorf("empty profile ID")
	case profile.ModelID == "":
		return fmt.Errorf("empty model ID")
	case profile.Firmware == "":
		return fmt.Errorf("empty firmware")
	case profile.ID != profile.ModelID+"-"+profile.Firmware:
		return fmt.Errorf("profile ID must be %q", profile.ModelID+"-"+profile.Firmware)
	}
	deep := profile.Supports(CapabilityDeepPersist)
	if deep != (profile.Deep != nil) {
		return fmt.Errorf("deep_persist capability and deep policy must be declared together")
	}
	if deep && (profile.Deep.Acknowledgement == "" || profile.Deep.RebootTransitions <= 0) {
		return fmt.Errorf("deep policy needs an acknowledgement phrase and a positive reboot count")
	}
	for _, capability := range profile.Capabilities {
		if capability != CapabilityDoctor && operations == nil {
			return fmt.Errorf("capability %q declared without an implementation", capability)
		}
	}
	return nil
}

// Lookup returns the adapter registered under an exact profile ID.
func (r *Registry) Lookup(profileID string) (Adapter, bool) {
	adapter, ok := r.adapters[profileID]
	return adapter, ok
}

// Resolve returns the only exact match, failing closed on unknown or ambiguous devices.
func (r *Registry) Resolve(observation Observation) (Adapter, error) {
	var exact []Adapter
	deviceKnown := false
	for _, id := range r.order {
		adapter := r.adapters[id]
		switch adapter.Match(observation).Level {
		case ExactMatch:
			exact = append(exact, adapter)
		case DeviceMatch:
			deviceKnown = true
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return Adapter{}, newError(CodeAmbiguousDevice, "multiple adapters exactly match the device")
	}
	if deviceKnown {
		return Adapter{}, newError(CodeUnsupportedFirmware, "the device model is known but its firmware is not supported")
	}
	return Adapter{}, newError(CodeUnsupportedDevice, "the device model is not supported")
}

// ResolveProfile applies only the requested adapter and never falls back to another profile.
func (r *Registry) ResolveProfile(profileID string, observation Observation) (Adapter, error) {
	adapter, ok := r.adapters[profileID]
	if !ok {
		return Adapter{}, newError(CodeUnsupportedProfile, "the requested profile is not registered")
	}
	switch adapter.Match(observation).Level {
	case ExactMatch:
		return adapter, nil
	case DeviceMatch:
		return Adapter{}, newError(CodeUnsupportedFirmware, "the requested profile does not support this firmware")
	default:
		return Adapter{}, newError(CodeUnsupportedDevice, "the requested profile does not support this device")
	}
}

// Candidates lists, in registration order, every profile whose model matches the observation.
func (r *Registry) Candidates(observation Observation) []ProfileMatch {
	var matches []ProfileMatch
	for _, id := range r.order {
		adapter := r.adapters[id]
		switch adapter.Match(observation).Level {
		case ExactMatch:
			matches = append(matches, ProfileMatch{Profile: adapter.Profile(), Exact: true})
		case DeviceMatch:
			matches = append(matches, ProfileMatch{Profile: adapter.Profile()})
		}
	}
	return matches
}

// Catalog groups the registered profiles by model, keeping registration order.
func (r *Registry) Catalog() []CatalogEntry {
	var catalog []CatalogEntry
	index := make(map[string]int)
	for _, id := range r.order {
		profile := r.adapters[id].Profile()
		position, known := index[profile.ModelID]
		if !known {
			position = len(catalog)
			index[profile.ModelID] = position
			catalog = append(catalog, CatalogEntry{
				ModelID: profile.ModelID, Vendor: profile.Vendor, Model: profile.Model, Hardware: profile.Hardware,
			})
		}
		catalog[position].Profiles = append(catalog[position].Profiles, profile)
	}
	return catalog
}
