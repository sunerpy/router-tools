package app

import (
	"context"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// SoftState is the read-only state of the managed soft persistence.
type SoftState string

const (
	SoftNotInstalled      SoftState = "not_installed"
	SoftInstalledVerified SoftState = "installed_verified"
	// SoftFileDrift: the managed UCI include points at the managed script but its content differs; an
	// install repairs it.
	SoftFileDrift SoftState = "file_drift"
	// SoftModeDrift: the content matches but the script is not executable; it needs remove then install.
	SoftModeDrift SoftState = "mode_drift"
	// SoftUCIDrift: the include uses the managed path but other values differ; it needs remove then install.
	SoftUCIDrift SoftState = "uci_drift"
	// SoftConflict: a user-managed path or unrecognized configuration; the tool never overwrites it.
	SoftConflict SoftState = "conflict"
	// SoftOrphanFile: the managed script exists without its UCI include.
	SoftOrphanFile SoftState = "orphan_file"
)

// SoftInspector reads the soft-persistence state over a verified SSH root shell without writing.
type SoftInspector interface {
	InspectSoft(context.Context, device.Target, secret.Password, hostkey.Policy) (SoftState, error)
}

// DeepFlags summarizes the boot flags deep persistence sets.
type DeepFlags string

const (
	DeepFlagsAllSet DeepFlags = "all_set"
	DeepFlagsNone   DeepFlags = "none"
	DeepFlagsMixed  DeepFlags = "mixed"
)

// DeepInspection is the read-only deep-persistence state: the boot flags, and whether the crash partition
// the deep flow writes is listed in the MTD layout and readable.
type DeepInspection struct {
	Flags          DeepFlags `json:"flags"`
	CrashPartition bool      `json:"crash_partition"`
	CrashReadable  bool      `json:"crash_readable"`
}

// DeepInspector reads the deep-persistence state over a verified SSH root shell without writing.
type DeepInspector interface {
	InspectDeep(context.Context, device.Target, secret.Password, hostkey.Policy) (DeepInspection, error)
}
