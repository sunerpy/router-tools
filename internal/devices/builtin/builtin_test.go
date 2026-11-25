package builtin_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/builtin"
)

func TestBuiltinAdaptersRegisterAndListRD08(t *testing.T) {
	t.Parallel()

	registry, err := device.NewRegistry(builtin.Adapters()...)
	if err != nil {
		t.Fatalf("NewRegistry(builtin.Adapters()) error = %v", err)
	}
	catalog := registry.Catalog()
	if len(catalog) != 1 || catalog[0].ModelID != "xiaomi-rd08" || catalog[0].Model != "BE6500 Pro" ||
		len(catalog[0].Profiles) != 1 || catalog[0].Profiles[0].ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("Catalog() = %+v, want RD08 with firmware 1.1.96 only", catalog)
	}
	deep := catalog[0].Profiles[0].Deep
	if deep == nil || deep.Acknowledgement != "RD08-DEEP" || deep.RebootTransitions != 3 {
		t.Fatalf("RD08 deep policy = %+v", deep)
	}
	if builtin.Discovery() == nil {
		t.Fatal("Discovery() = nil")
	}
}
