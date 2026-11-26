package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
)

const managedStampLayout = "20060102T150405Z"

// backupIdentity is the device identity a backup manifest records and is compared with: hardware falls
// back to the profile's when the device reported none (the display-name match branch), and the firmware
// string is trimmed the way profile matching trims it.
func backupIdentity(observation device.Observation, profile device.Profile) backup.DeviceIdentity {
	hardware := strings.TrimSpace(observation.Hardware)
	if hardware == "" {
		hardware = profile.Hardware
	}
	return backup.DeviceIdentity{
		DeviceID:   observation.DeviceID,
		Hardware:   hardware,
		ROMVersion: strings.TrimSpace(observation.ROMVersion),
		Profile:    profile.ID,
	}
}

func sameBackupIdentity(a, b backup.DeviceIdentity) bool {
	return a.DeviceID == b.DeviceID && strings.EqualFold(a.Hardware, b.Hardware) &&
		a.ROMVersion == b.ROMVersion && a.Profile == b.Profile
}

// managedTargetDir names a router's managed directory: the readable canonical host plus the first 12 hex
// characters of its port-independent recovery key, so every spelling of one router shares the directory and
// IPv6 hosts stay portable.
func managedTargetDir(target device.Target) string {
	return deepSafeHost(device.CanonicalHost(target.Host)) + "-" + device.RecoveryKey(target)[:12]
}

func (s *Service) managedBackupDir(target device.Target, profile device.Profile) string {
	return filepath.Join(s.backupRoot, profile.ModelID, managedTargetDir(target))
}

func (s *Service) managedJournalDir(target device.Target, profile device.Profile) string {
	return filepath.Join(s.journalRoot, profile.ModelID, managedTargetDir(target))
}

// ManagedBackup is a locally verified backup in the managed directory.
type ManagedBackup struct {
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	BootID    string    `json:"boot_id"`
}

// latestManagedBackup returns the newest managed backup that passes local verification and records the
// identity of the current device and profile; nil when there is none.
func (s *Service) latestManagedBackup(target device.Target, observation device.Observation, profile device.Profile) *ManagedBackup {
	if s.backupRoot == "" {
		return nil
	}
	root := s.managedBackupDir(target, profile)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := time.Parse(managedStampLayout, entry.Name()); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	want := backupIdentity(observation, profile)
	for i, name := range names {
		if i >= 50 {
			break
		}
		dir := filepath.Join(root, name)
		if err := backup.Verify(dir); err != nil {
			continue
		}
		manifest, err := backup.LoadManifest(dir)
		if err != nil || !manifest.Complete || !sameBackupIdentity(manifest.Device, want) {
			continue
		}
		return &ManagedBackup{Path: dir, CreatedAt: manifest.CreatedAt, BootID: manifest.BootID}
	}
	return nil
}
