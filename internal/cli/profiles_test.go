package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

func testCatalog() []device.CatalogEntry {
	return []device.CatalogEntry{{
		ModelID: "xiaomi-rd08", Vendor: "Xiaomi", Model: "BE6500 Pro", Hardware: "RD08",
		Profiles: []device.Profile{{
			ID: "xiaomi-rd08-1.1.96", ModelID: "xiaomi-rd08", Vendor: "Xiaomi", Model: "BE6500 Pro", Hardware: "RD08",
			Firmware: "1.1.96", Capabilities: []device.Capability{device.CapabilityDoctor, device.CapabilityDeepPersist},
			Deep: &device.DeepPolicy{Acknowledgement: "RD08-DEEP", RebootTransitions: 3},
		}},
	}}
}

// catalogService adds the catalog to the full fake service.
type catalogService struct {
	fakeAllService
	catalog []device.CatalogEntry
}

func (s *catalogService) Catalog() []device.CatalogEntry { return s.catalog }

func TestProfilesListsTheCatalogWithoutNetwork(t *testing.T) {
	t.Parallel()

	service := &catalogService{catalog: testCatalog()}
	code, stdout, stderr := execute(t, service, testBuildInfo(), "profiles")
	if code != 0 {
		t.Fatalf("profiles code = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"Xiaomi BE6500 Pro (RD08)", "1.1.96", "xiaomi-rd08-1.1.96", "--profile"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("profiles output missing %q:\n%s", want, stdout)
		}
	}
	if service.fakeDoctorService.request.Target.Host != "" || service.deepRequest.Target.Host != "" {
		t.Fatal("profiles invoked a device operation")
	}

	code, stdout, _ = execute(t, service, testBuildInfo(), "--json", "profiles")
	if code != 0 {
		t.Fatalf("profiles --json code = %d", code)
	}
	var decoded struct {
		SchemaVersion int                   `json:"schema_version"`
		Operation     string                `json:"operation"`
		Models        []device.CatalogEntry `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil || decoded.SchemaVersion != 1 || decoded.Operation != "profiles" ||
		len(decoded.Models) != 1 || decoded.Models[0].Profiles[0].ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("profiles --json = %q, %v", stdout, err)
	}
}

func TestDoctorPrintsSupportedFirmwareOnUnsupportedFirmware(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	service := &fakeDoctorService{
		report: app.DoctorReport{
			Status:     app.StatusFailed,
			Device:     device.Observation{Hardware: "RD08", ROMVersion: "1.1.97"},
			Candidates: []device.ProfileMatch{{Profile: catalog[0].Profiles[0]}},
			Error:      &device.Error{Code: device.CodeUnsupportedFirmware, Message: "the device model is known but its firmware is not supported"},
		},
		err: &device.Error{Code: device.CodeUnsupportedFirmware, Message: "the device model is known but its firmware is not supported"},
	}
	code, stdout, _ := execute(t, service, testBuildInfo(), "doctor")
	if code != 5 {
		t.Fatalf("doctor code = %d, want 5", code)
	}
	if !strings.Contains(stdout, "Supported firmware for Xiaomi BE6500 Pro (RD08): 1.1.96 (profile xiaomi-rd08-1.1.96)") {
		t.Fatalf("doctor output does not list the supported firmware:\n%s", stdout)
	}
}

func TestDeepPromptUsesTheProfilePhrase(t *testing.T) {
	t.Parallel()

	service := &catalogService{catalog: testCatalog()}
	code, _, stderr := executeWithInput(t, service, "root\nRD08-DEEP\n",
		"--root-password-stdin", "--allow-telnet", "--run-dir", t.TempDir(), "deep-persist", "--backup-dir", "/backup")
	if code != 0 {
		t.Fatalf("deep-persist code = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stderr, "reboots 3 times") || !strings.Contains(stderr, "Type RD08-DEEP to continue") || service.deepRequest.AckDeep != "RD08-DEEP" {
		t.Fatalf("prompt = %q, ack = %q", stderr, service.deepRequest.AckDeep)
	}
}

func TestDeepPromptNeedsAProfileWhenPhrasesDiffer(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	other := catalog[0].Profiles[0]
	other.ID, other.Firmware = "xiaomi-rd08-1.1.97", "1.1.97"
	other.Deep = &device.DeepPolicy{Acknowledgement: "RD08-DEEP-97", RebootTransitions: 3}
	catalog[0].Profiles = append(catalog[0].Profiles, other)
	service := &catalogService{catalog: catalog}

	code, _, stderr := executeWithInput(t, service, "root\n",
		"--root-password-stdin", "--allow-telnet", "--run-dir", t.TempDir(), "deep-persist", "--backup-dir", "/backup")
	if code != 2 || !strings.Contains(stderr, "--profile") {
		t.Fatalf("deep-persist code = %d, stderr = %s; want a request for --profile", code, stderr)
	}

	code, _, stderr = executeWithInput(t, service, "root\nRD08-DEEP-97\n",
		"--root-password-stdin", "--allow-telnet", "--profile", "xiaomi-rd08-1.1.97", "--run-dir", t.TempDir(),
		"deep-persist", "--backup-dir", "/backup")
	if code != 0 || !strings.Contains(stderr, "Type RD08-DEEP-97 to continue") {
		t.Fatalf("deep-persist --profile code = %d, stderr = %s", code, stderr)
	}
}
