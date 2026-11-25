// Package backup implements MTD backup manifests and integrity verification.
package backup

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var mtdLinePattern = regexp.MustCompile(`^mtd([0-9]+):[ \t]+([0-9A-Fa-f]+)[ \t]+([0-9A-Fa-f]+)[ \t]+"([^"]*)"[ \t]*$`)

// Partition identifies an MTD partition by index, not by name.
type Partition struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Size      uint64 `json:"size"`
	EraseSize uint64 `json:"erase_size"`
	Source    string `json:"source"`
	File      string `json:"file"`
	SHA256    string `json:"sha256"`
}

// DeviceIdentity binds a backup to one exact device/profile.
type DeviceIdentity struct {
	DeviceID   string `json:"device_id"`
	Hardware   string `json:"hardware"`
	ROMVersion string `json:"rom_version"`
	Profile    string `json:"profile"`
}

// Manifest is generated only after every partition passes size and hash checks.
type Manifest struct {
	SchemaVersion int            `json:"schema_version"`
	ToolVersion   string         `json:"tool_version"`
	CreatedAt     time.Time      `json:"created_at"`
	Complete      bool           `json:"complete"`
	Device        DeviceIdentity `json:"device"`
	BootID        string         `json:"boot_id"`
	MTDTextFile   string         `json:"mtd_text_file"`
	Partitions    []Partition    `json:"partitions"`
}

// ParseMTD parses Linux /proc/mtd using 64-bit hexadecimal sizes.
func ParseMTD(text string) ([]Partition, error) {
	seen := make(map[int]struct{})
	var partitions []Partition
	for lineNumber, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "dev:") {
			continue
		}
		matches := mtdLinePattern.FindStringSubmatch(line)
		if matches == nil {
			return nil, fmt.Errorf("invalid /proc/mtd line %d", lineNumber+1)
		}
		index64, err := strconv.ParseUint(matches[1], 10, 31)
		if err != nil {
			return nil, fmt.Errorf("invalid MTD index on line %d", lineNumber+1)
		}
		size, err := strconv.ParseUint(matches[2], 16, 64)
		if err != nil || size == 0 {
			return nil, fmt.Errorf("invalid MTD size on line %d", lineNumber+1)
		}
		eraseSize, err := strconv.ParseUint(matches[3], 16, 64)
		if err != nil || eraseSize == 0 {
			return nil, fmt.Errorf("invalid MTD erase size on line %d", lineNumber+1)
		}
		index := int(index64)
		if _, exists := seen[index]; exists {
			return nil, fmt.Errorf("duplicate MTD index %d", index)
		}
		seen[index] = struct{}{}
		partitions = append(partitions, Partition{Index: index, Name: matches[4], Size: size, EraseSize: eraseSize})
	}
	if len(partitions) == 0 {
		return nil, fmt.Errorf("/proc/mtd contains no partitions")
	}
	return partitions, nil
}
