package backup

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/sunerpy/router-tools/internal/device"
)

// LoadManifest reads a manifest after the caller has completed local Verify.
func LoadManifest(directory string) (Manifest, error) {
	data, err := readRegularSmall(directory, "manifest.json", 1<<20)
	if err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, device.WrapError(device.CodeBackupIncomplete, "cannot decode manifest", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest contains trailing JSON data"}
	}
	return manifest, nil
}

// VerifyOnline compares current read-only device evidence with a verified backup.
func VerifyOnline(manifest Manifest, observation device.Observation, profileID, mtdText, bootID string) error {
	if manifest.Device.DeviceID != observation.DeviceID ||
		!strings.EqualFold(manifest.Device.Hardware, observation.Hardware) ||
		manifest.Device.ROMVersion != strings.TrimSpace(observation.ROMVersion) ||
		manifest.Device.Profile != profileID {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "current device identity differs from the backup"}
	}
	if manifest.BootID != bootID {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "device boot_id changed after the backup"}
	}
	partitions, err := ParseMTD(mtdText)
	if err != nil {
		return device.WrapError(device.CodeBackupLayoutMismatch, "current /proc/mtd is invalid", err)
	}
	if !sameLayout(manifest.Partitions, partitions) {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "current MTD layout differs from the backup"}
	}
	return nil
}
