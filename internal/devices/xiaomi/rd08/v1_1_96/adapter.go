package rd08v1196

import (
	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

// Every capability the profile declares is served by this package's Prober.
var (
	_ app.AdminVerifier        = (*Prober)(nil)
	_ app.RootVerifier         = (*Prober)(nil)
	_ app.SSHEnabler           = (*Prober)(nil)
	_ app.SoftPersistence      = (*Prober)(nil)
	_ app.RebootVerifier       = (*Prober)(nil)
	_ app.BackupCreator        = (*Prober)(nil)
	_ app.OnlineBackupVerifier = (*Prober)(nil)
	_ app.DeepPersister        = (*Prober)(nil)
)

// Adapter registers the RD08 1.1.96 profile together with the implementation that serves only it.
func Adapter() device.Adapter {
	return device.Adapter{Driver: New(), Operations: NewProber(nil)}
}
