package app_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// modelDriver matches one model by hardware and one firmware exactly.
type modelDriver struct{ p device.Profile }

func (d modelDriver) Profile() device.Profile { return d.p }
func (d modelDriver) Match(o device.Observation) device.MatchResult {
	if !strings.EqualFold(o.Hardware, d.p.Hardware) {
		return device.MatchResult{Level: device.NoMatch}
	}
	if o.ROMVersion != d.p.Firmware {
		return device.MatchResult{Level: device.DeviceMatch}
	}
	return device.MatchResult{Level: device.ExactMatch}
}

func modelProfile(firmware string, capabilities ...device.Capability) device.Profile {
	return device.Profile{
		ID: "acme-r1-" + firmware, ModelID: "acme-r1", Vendor: "Acme", Model: "R1", Hardware: "R1",
		Firmware: firmware, Capabilities: capabilities,
	}
}

// countingRoot records root verifications served by one adapter implementation.
type countingRoot struct{ calls int }

func (c *countingRoot) VerifyRoot(context.Context, device.Target, secret.Password, hostkey.Policy, string, bool) (app.RootVerification, error) {
	c.calls++
	return app.RootVerification{Transport: "ssh", RootVerified: true, BootID: "00000000-0000-0000-0000-000000000001"}, nil
}

// discoveryWithRoot also implements RootVerifier; the service must never use it for capabilities.
type discoveryWithRoot struct {
	fakeProber
	countingRoot
}

type countingLocker struct{ acquired int }

func (l *countingLocker) Acquire(device.Target) (func() error, error) {
	l.acquired++
	return func() error { return nil }, nil
}

func TestCapabilitiesDispatchOnlyToTheResolvedAdapter(t *testing.T) {
	t.Parallel()

	v1, v2 := &countingRoot{}, &countingRoot{}
	registry, err := device.NewRegistry(
		device.Adapter{Driver: modelDriver{p: modelProfile("1.0", device.CapabilityVerify)}, Operations: v1},
		device.Adapter{Driver: modelDriver{p: modelProfile("2.0", device.CapabilityVerify)}, Operations: v2},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	discovery := &discoveryWithRoot{fakeProber: fakeProber{result: device.ProbeResult{
		Observation: device.Observation{Hardware: "R1", ROMVersion: "2.0"},
	}}}
	service, _ := app.NewService(registry, discovery)
	password, _ := secret.NewPassword("root")

	report, err := service.Verify(t.Context(), app.VerifyRequest{Target: device.DefaultTarget(), RootPassword: password}, nil)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if report.Profile.ID != "acme-r1-2.0" || v2.calls != 1 || v1.calls != 0 || discovery.countingRoot.calls != 0 {
		t.Fatalf("profile %q; calls v1=%d v2=%d discovery=%d, want only the 2.0 implementation",
			report.Profile.ID, v1.calls, v2.calls, discovery.countingRoot.calls)
	}
}

func TestUnknownExplicitProfileFailsBeforeLockAndNetwork(t *testing.T) {
	t.Parallel()

	const unknown = "acme-r1-9.9"
	password, _ := secret.NewPassword("root")
	target := device.DefaultTarget()
	soft := app.SoftRequest{Target: target, ProfileID: unknown, RootPassword: password, Confirmed: true}
	calls := map[string]func(*app.Service) error{
		"doctor": func(s *app.Service) error {
			_, err := s.Doctor(t.Context(), app.DoctorRequest{Target: target, ProfileID: unknown}, nil)
			return err
		},
		"verify": func(s *app.Service) error {
			_, err := s.Verify(t.Context(), app.VerifyRequest{Target: target, ProfileID: unknown, RootPassword: password}, nil)
			return err
		},
		"enable": func(s *app.Service) error {
			_, err := s.EnableSSH(t.Context(), app.EnableRequest{Target: target, ProfileID: unknown, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
			return err
		},
		"persist soft":   func(s *app.Service) error { _, err := s.PersistSoft(t.Context(), soft, nil); return err },
		"persist remove": func(s *app.Service) error { _, err := s.PersistRemove(t.Context(), soft, nil); return err },
		"reboot verify":  func(s *app.Service) error { _, err := s.RebootVerify(t.Context(), soft, nil); return err },
		"backup create": func(s *app.Service) error {
			_, err := s.BackupCreate(t.Context(), app.BackupCreateRequest{Target: target, ProfileID: unknown, RootPassword: password, OutputDir: t.TempDir()}, nil)
			return err
		},
		"backup verify online": func(s *app.Service) error {
			_, err := s.BackupVerifyOnline(t.Context(), app.BackupOnlineVerifyRequest{Target: target, ProfileID: unknown, RootPassword: password, BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0")}, nil)
			return err
		},
		"deep persist": func(s *app.Service) error {
			_, err := s.DeepPersist(t.Context(), app.DeepRequest{Target: target, ProfileID: unknown, RootPassword: password, AllowTelnet: true, AckDeep: "RD08-DEEP"}, nil)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			ops := newAllOps()
			registry, _ := device.NewRegistry(device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: ops})
			discovery := &fakeProber{}
			locker := &countingLocker{}
			service, _ := app.NewServiceWithLocker(registry, discovery, locker)
			assertCode(t, call(service), device.CodeUnsupportedProfile)
			if discovery.calls != 0 || locker.acquired != 0 || ops.total() != 0 {
				t.Fatalf("probes=%d locks=%d adapter calls=%d, want none", discovery.calls, locker.acquired, ops.total())
			}
		})
	}
}

func TestImplementationWithoutTheCapabilityInterfaceFailsClosed(t *testing.T) {
	t.Parallel()

	registry, _ := device.NewRegistry(device.Adapter{
		Driver:     modelDriver{p: modelProfile("1.0", device.CapabilityVerify)},
		Operations: struct{}{},
	})
	discovery := &discoveryWithRoot{fakeProber: fakeProber{result: device.ProbeResult{
		Observation: device.Observation{Hardware: "R1", ROMVersion: "1.0"},
	}}}
	service, _ := app.NewService(registry, discovery)
	password, _ := secret.NewPassword("root")

	_, err := service.Verify(t.Context(), app.VerifyRequest{Target: device.DefaultTarget(), RootPassword: password}, nil)
	assertCode(t, err, device.CodeUnsupportedCapability)
	if discovery.countingRoot.calls != 0 {
		t.Fatal("the discovery prober served a capability")
	}
}

func TestCatalogAndDoctorCandidates(t *testing.T) {
	t.Parallel()

	registry, _ := device.NewRegistry(
		device.Adapter{Driver: modelDriver{p: modelProfile("1.0")}},
		device.Adapter{Driver: modelDriver{p: modelProfile("2.0")}},
	)
	discovery := &fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "R1", ROMVersion: "3.0"}}}
	service, _ := app.NewService(registry, discovery)

	catalog := service.Catalog()
	if len(catalog) != 1 || catalog[0].ModelID != "acme-r1" || len(catalog[0].Profiles) != 2 {
		t.Fatalf("Catalog() = %+v", catalog)
	}

	report, err := service.Doctor(t.Context(), app.DoctorRequest{Target: device.DefaultTarget()}, nil)
	assertCode(t, err, device.CodeUnsupportedFirmware)
	if len(report.Candidates) != 2 || report.Candidates[0].Exact || report.Candidates[1].Profile.Firmware != "2.0" {
		t.Fatalf("Doctor candidates = %+v, want both supported firmwares, none exact", report.Candidates)
	}
}

func TestDeepAcknowledgementComesFromTheResolvedProfile(t *testing.T) {
	t.Parallel()

	alpha := modelProfile("1.0", device.CapabilityDeepPersist)
	alpha.Deep = &device.DeepPolicy{Acknowledgement: "ALPHA-DEEP", RebootTransitions: 3}
	beta := modelProfile("2.0", device.CapabilityDeepPersist)
	beta.Deep = &device.DeepPolicy{Acknowledgement: "BETA-DEEP", RebootTransitions: 2}
	alphaOps, betaOps := &deepAppProber{}, &deepAppProber{result: app.DeepResult{
		Status: app.StatusSucceeded, FinalSSHVerified: true, SoftVerified: true,
		Transitions: []app.BootTransition{{OldBootID: "1", NewBootID: "2"}, {OldBootID: "2", NewBootID: "3"}},
	}}
	registry, _ := device.NewRegistry(
		device.Adapter{Driver: modelDriver{p: alpha}, Operations: alphaOps},
		device.Adapter{Driver: modelDriver{p: beta}, Operations: betaOps},
	)
	observation := device.Observation{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "2.0"}
	discovery := &fakeProber{result: device.ProbeResult{Observation: observation}}
	service, _ := app.NewService(registry, discovery)
	password, _ := secret.NewPassword("root")
	backupDir := makeDeepBackupFixture(t, beta.ID)
	request := app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true,
		AckDeep: "ALPHA-DEEP", BackupDir: backupDir, RunDir: t.TempDir(),
	}

	// A phrase registered for another profile passes the offline check, but not the resolved profile's.
	report, err := service.DeepPersist(t.Context(), request, nil)
	assertCode(t, err, device.CodeDeepAckRequired)
	if alphaOps.calls != 0 || betaOps.calls != 0 {
		t.Fatal("an adapter ran with the wrong acknowledgement")
	}
	if report.JournalPath != "" {
		if _, statErr := os.Stat(report.JournalPath); statErr == nil {
			t.Fatal("a journal was created with the wrong acknowledgement")
		}
	}

	// An explicit profile checks its own phrase before any network access.
	request.ProfileID = beta.ID
	probes := discovery.calls
	_, err = service.DeepPersist(t.Context(), request, nil)
	assertCode(t, err, device.CodeDeepAckRequired)
	if discovery.calls != probes {
		t.Fatal("the explicit-profile phrase check accessed the network")
	}

	// The right phrase succeeds with the profile's own reboot count (2, not RD08's 3).
	request.AckDeep = "BETA-DEEP"
	request.RunDir = t.TempDir()
	report, err = service.DeepPersist(t.Context(), request, nil)
	if err != nil || report.Status != app.StatusSucceeded || betaOps.calls != 1 || alphaOps.calls != 0 {
		t.Fatalf("DeepPersist() = %+v, %v; calls alpha=%d beta=%d", report.Status, err, alphaOps.calls, betaOps.calls)
	}
}

func TestDeepExplicitProfileWithoutDeepFailsBeforeNetwork(t *testing.T) {
	t.Parallel()

	registry, _ := device.NewRegistry(device.Adapter{Driver: modelDriver{p: modelProfile("1.0")}})
	discovery := &fakeProber{}
	service, _ := app.NewService(registry, discovery)
	password, _ := secret.NewPassword("root")
	_, err := service.DeepPersist(t.Context(), app.DeepRequest{
		Target: device.DefaultTarget(), ProfileID: "acme-r1-1.0", RootPassword: password, AllowTelnet: true, AckDeep: "RD08-DEEP",
	}, nil)
	assertCode(t, err, device.CodeUnsupportedCapability)
	if discovery.calls != 0 {
		t.Fatal("deep persistence probed a profile that cannot run it")
	}
}

var allCapabilities = []device.Capability{
	device.CapabilityDoctor, device.CapabilityWebAuth, device.CapabilityVerify, device.CapabilityEnableSSH,
	device.CapabilitySoftPersist, device.CapabilityBackup, device.CapabilityDeepPersist,
}

func fullProfile(firmware string) device.Profile {
	p := modelProfile(firmware, allCapabilities...)
	p.Deep = &device.DeepPolicy{Acknowledgement: "ACME-DEEP", RebootTransitions: 1}
	return p
}

// allOps implements every capability interface and counts calls per method.
type allOps struct{ calls map[string]int }

func newAllOps() *allOps { return &allOps{calls: make(map[string]int)} }

func (o *allOps) total() int {
	n := 0
	for _, c := range o.calls {
		n += c
	}
	return n
}

func (o *allOps) VerifyAdmin(context.Context, device.Target, secret.Password) error {
	o.calls["VerifyAdmin"]++
	return nil
}

func (o *allOps) VerifyRoot(context.Context, device.Target, secret.Password, hostkey.Policy, string, bool) (app.RootVerification, error) {
	o.calls["VerifyRoot"]++
	return app.RootVerification{Transport: "ssh", RootVerified: true, BootID: "00000000-0000-0000-0000-000000000001"}, nil
}

func (o *allOps) EnableSSH(context.Context, device.Target, secret.Password, secret.Password, hostkey.Policy) (app.EnableResult, error) {
	o.calls["EnableSSH"]++
	return app.EnableResult{Status: app.StatusSucceeded, SSHVerified: true}, nil
}

func (o *allOps) InstallSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.SoftResult, error) {
	o.calls["InstallSoft"]++
	return app.SoftResult{Verified: true}, nil
}

func (o *allOps) RemoveSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.SoftResult, error) {
	o.calls["RemoveSoft"]++
	return app.SoftResult{Verified: true}, nil
}

func (o *allOps) RebootVerify(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.RebootResult, error) {
	o.calls["RebootVerify"]++
	return app.RebootResult{}, nil
}

func (o *allOps) CreateBackup(context.Context, device.Target, secret.Password, hostkey.Policy, backup.CreateOptions) (backup.Manifest, error) {
	o.calls["CreateBackup"]++
	return backup.Manifest{}, nil
}

func (o *allOps) VerifyBackupOnline(context.Context, device.Target, secret.Password, hostkey.Policy, backup.Manifest, device.Observation, string) error {
	o.calls["VerifyBackupOnline"]++
	return nil
}

func (o *allOps) DeepPersist(context.Context, app.DeepParams) (app.DeepResult, error) {
	o.calls["DeepPersist"]++
	return app.DeepResult{}, nil
}

// discoveryAll identifies the device and also implements every capability; it must only be probed.
type discoveryAll struct {
	fakeProber
	*allOps
}

// Every capability, including each write path, reaches only the resolved firmware's implementation.
func TestEveryCapabilityDispatchesOnlyToTheResolvedFirmware(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("root")
	target := device.DefaultTarget()
	soft := app.SoftRequest{Target: target, RootPassword: password, Confirmed: true}
	cases := map[string]struct {
		method string
		call   func(*app.Service)
	}{
		"doctor admin auth": {"VerifyAdmin", func(s *app.Service) {
			_, _ = s.Doctor(t.Context(), app.DoctorRequest{Target: target, WithAdminAuth: true, AdminPassword: password}, nil)
		}},
		"doctor root verify": {"VerifyRoot", func(s *app.Service) {
			_, _ = s.Doctor(t.Context(), app.DoctorRequest{Target: target, WithRootVerify: true, RootPassword: password}, nil)
		}},
		"verify": {"VerifyRoot", func(s *app.Service) {
			_, _ = s.Verify(t.Context(), app.VerifyRequest{Target: target, RootPassword: password}, nil)
		}},
		"enable": {"EnableSSH", func(s *app.Service) {
			_, _ = s.EnableSSH(t.Context(), app.EnableRequest{Target: target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
		}},
		"persist soft":   {"InstallSoft", func(s *app.Service) { _, _ = s.PersistSoft(t.Context(), soft, nil) }},
		"persist remove": {"RemoveSoft", func(s *app.Service) { _, _ = s.PersistRemove(t.Context(), soft, nil) }},
		"reboot verify":  {"RebootVerify", func(s *app.Service) { _, _ = s.RebootVerify(t.Context(), soft, nil) }},
		"backup create": {"CreateBackup", func(s *app.Service) {
			_, _ = s.BackupCreate(t.Context(), app.BackupCreateRequest{Target: target, RootPassword: password, OutputDir: t.TempDir()}, nil)
		}},
		"backup verify online": {"VerifyBackupOnline", func(s *app.Service) {
			_, _ = s.BackupVerifyOnline(t.Context(), app.BackupOnlineVerifyRequest{Target: target, RootPassword: password, BackupDir: makeDeepBackupFixture(t, "acme-r1-2.0")}, nil)
		}},
		"deep persist": {"DeepPersist", func(s *app.Service) {
			_, _ = s.DeepPersist(t.Context(), app.DeepRequest{
				Target: target, RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP",
				BackupDir: makeDeepBackupFixture(t, "acme-r1-2.0"), RunDir: t.TempDir(),
			}, nil)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v1, v2 := newAllOps(), newAllOps()
			registry, err := device.NewRegistry(
				device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: v1},
				device.Adapter{Driver: modelDriver{p: fullProfile("2.0")}, Operations: v2},
			)
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}
			discovery := &discoveryAll{
				fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "2.0"}}},
				allOps:     newAllOps(),
			}
			service, _ := app.NewService(registry, discovery)
			tc.call(service)
			if v2.calls[tc.method] != 1 || v1.total() != 0 || discovery.total() != 0 {
				t.Fatalf("%s: 2.0 calls=%v, 1.0 calls=%v, discovery calls=%v; want exactly one call on 2.0",
					tc.method, v2.calls, v1.calls, discovery.allOps.calls)
			}
		})
	}
}
