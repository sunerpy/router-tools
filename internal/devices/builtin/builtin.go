// Package builtin is the single list of device adapters compiled into routerctl and the desktop GUI.
// A firmware of a model that Discovery already identifies is added with its package and one line in
// Adapters; a device Discovery cannot identify also needs discovery support.
// internal/app and internal/cli never import this package.
package builtin

import (
	"github.com/sunerpy/router-tools/internal/device"
	rd08v1196 "github.com/sunerpy/router-tools/internal/devices/xiaomi/rd08/v1_1_96"
)

// Adapters returns every registered profile with its own implementation, in catalog order.
func Adapters() []device.Adapter {
	return []device.Adapter{
		rd08v1196.Adapter(),
	}
}

// Discovery returns the read-only prober that identifies devices before a profile is resolved. It is only
// asked to Probe; capabilities always come from the resolved adapter.
func Discovery() device.Prober {
	return rd08v1196.NewProber(nil)
}
