package backup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sunerpy/router-tools/internal/device"
)

var lowercaseSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Verify validates a complete local backup, including layout, paths, files, and sums.
func Verify(directory string) error {
	manifestData, err := readRegularSmall(directory, "manifest.json", 1<<20)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(manifestData))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return backupError(device.CodeBackupIncomplete, "invalid manifest JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest contains trailing JSON data"}
	}
	if manifest.SchemaVersion != 1 || !manifest.Complete || manifest.ToolVersion == "" || manifest.CreatedAt.IsZero() {
		return &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest is incomplete or has an unsupported schema"}
	}
	if manifest.Device.Profile == "" || manifest.Device.Hardware == "" || manifest.Device.ROMVersion == "" || manifest.BootID == "" {
		return &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest lacks device or boot identity"}
	}
	if len(manifest.Partitions) == 0 {
		return &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest contains no partitions"}
	}
	expectedSums := map[string]struct{}{"manifest.json": {}, manifest.MTDTextFile: {}}
	indices := make(map[int]struct{})
	files := make(map[string]struct{})
	for _, partition := range manifest.Partitions {
		if partition.Size == 0 || partition.EraseSize == 0 || !lowercaseSHA256.MatchString(partition.SHA256) {
			return &device.Error{Code: device.CodeBackupIncomplete, Message: "manifest contains an invalid partition record"}
		}
		if _, exists := indices[partition.Index]; exists {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "manifest contains duplicate MTD indices"}
		}
		if _, exists := files[partition.File]; exists {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "manifest contains duplicate partition paths"}
		}
		indices[partition.Index] = struct{}{}
		files[partition.File] = struct{}{}
		expectedSums[partition.File] = struct{}{}
		_, actualHash, err := hashRegularFile(directory, partition.File)
		if err != nil {
			return err
		}
		info, err := statRegular(directory, partition.File)
		if err != nil {
			return err
		}
		if uint64(info.Size()) != partition.Size {
			return &device.Error{Code: device.CodeBackupHashMismatch, Message: fmt.Sprintf("partition %d length changed", partition.Index)}
		}
		if actualHash != partition.SHA256 {
			return &device.Error{Code: device.CodeBackupHashMismatch, Message: fmt.Sprintf("partition %d hash changed", partition.Index)}
		}
	}
	mtdData, err := readRegularSmall(directory, manifest.MTDTextFile, 1<<20)
	if err != nil {
		return err
	}
	mtdPartitions, err := ParseMTD(string(mtdData))
	if err != nil {
		return backupError(device.CodeBackupLayoutMismatch, "mtd.txt is invalid", err)
	}
	if !sameLayout(manifest.Partitions, mtdPartitions) {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "mtd.txt does not match the manifest layout"}
	}
	sumsData, err := readRegularSmall(directory, "SHA256SUMS", 1<<20)
	if err != nil {
		return err
	}
	if err := verifySums(directory, string(sumsData), expectedSums); err != nil {
		return err
	}
	return nil
}

func verifySums(directory, text string, expected map[string]struct{}) error {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) != len(expected) {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS coverage does not match the manifest"}
	}
	seen := make(map[string]struct{}, len(lines))
	var paths []string
	for _, line := range lines {
		if len(line) < 67 || !lowercaseSHA256.MatchString(line[:64]) || line[64:66] != "  " {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS has an invalid line"}
		}
		expectedHash := line[:64]
		relativePath := line[66:]
		if _, exists := seen[relativePath]; exists {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS contains a duplicate path"}
		}
		if _, ok := expected[relativePath]; !ok {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS contains an unexpected path"}
		}
		seen[relativePath] = struct{}{}
		paths = append(paths, relativePath)
		_, actualHash, err := hashRegularFile(directory, relativePath)
		if err != nil {
			return err
		}
		if actualHash != expectedHash {
			return &device.Error{Code: device.CodeBackupHashMismatch, Message: "SHA256SUMS hash does not match file content"}
		}
	}
	if !sort.StringsAreSorted(paths) {
		return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS is not sorted by path"}
	}
	for required := range expected {
		if _, ok := seen[required]; !ok {
			return &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "SHA256SUMS omits a required file"}
		}
	}
	return nil
}

func sameLayout(manifest, mtd []Partition) bool {
	if len(manifest) != len(mtd) {
		return false
	}
	for index := range manifest {
		left := manifest[index]
		right := mtd[index]
		if left.Index != right.Index || left.Name != right.Name || left.Size != right.Size || left.EraseSize != right.EraseSize {
			return false
		}
	}
	return true
}

func readRegularSmall(root, relative string, limit int64) ([]byte, error) {
	if _, err := statRegular(root, relative); err != nil {
		return nil, err
	}
	fullPath, err := safePath(root, relative)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, backupError(device.CodeBackupIncomplete, "cannot read backup file", err)
	}
	if int64(len(data)) > limit {
		return nil, &device.Error{Code: device.CodeBackupIncomplete, Message: "backup metadata file is too large"}
	}
	return data, nil
}

func hashRegularFile(root, relative string) (int64, string, error) {
	if _, err := statRegular(root, relative); err != nil {
		return 0, "", err
	}
	fullPath, err := safePath(root, relative)
	if err != nil {
		return 0, "", err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return 0, "", backupError(device.CodeBackupIncomplete, "cannot open backup file", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", backupError(device.CodeBackupIncomplete, "cannot hash backup file", err)
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func statRegular(root, relative string) (os.FileInfo, error) {
	fullPath, err := safePath(root, relative)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, backupError(device.CodeBackupIncomplete, "backup file is missing", err)
	}
	if !info.Mode().IsRegular() {
		return nil, &device.Error{Code: device.CodeBackupIncomplete, Message: "backup path is not a regular file"}
	}
	return info, nil
}

func safePath(root, relative string) (string, error) {
	if relative == "" || path.IsAbs(relative) || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", &device.Error{Code: device.CodeBackupIncomplete, Message: "backup contains an absolute or non-portable path"}
	}
	clean := path.Clean(relative)
	if clean != relative || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", &device.Error{Code: device.CodeBackupIncomplete, Message: "backup contains path traversal"}
	}
	rootEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", backupError(device.CodeBackupIncomplete, "cannot resolve backup root", err)
	}
	fullPath := filepath.Join(rootEval, filepath.FromSlash(clean))
	parentEval, err := filepath.EvalSymlinks(filepath.Dir(fullPath))
	if err != nil {
		return "", backupError(device.CodeBackupIncomplete, "cannot resolve backup parent", err)
	}
	rel, err := filepath.Rel(rootEval, parentEval)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &device.Error{Code: device.CodeBackupIncomplete, Message: "backup path escapes its root through a symlink"}
	}
	return fullPath, nil
}

func backupError(code, message string, cause error) error {
	return device.WrapError(code, message, cause)
}
