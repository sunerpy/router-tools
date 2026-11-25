package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

// CreateOptions controls one new, non-overwriting backup directory.
type CreateOptions struct {
	OutputDir   string
	ToolVersion string
	Device      DeviceIdentity
	BootID      string
	Now         func() time.Time
	FreeSpace   func(string) (uint64, error)
}

// Create downloads every declared partition through bounded SSH streams.
func Create(ctx context.Context, remote shell.StreamingShell, options CreateOptions) (manifest Manifest, resultErr error) {
	if remote == nil || options.OutputDir == "" {
		return manifest, &device.Error{Code: device.CodeInvalidArgument, Message: "backup output and streaming shell are required"}
	}
	if _, err := os.Lstat(options.OutputDir); err == nil {
		return manifest, &device.Error{Code: device.CodeInvalidArgument, Message: "backup output directory must not already exist"}
	} else if !os.IsNotExist(err) {
		return manifest, err
	}
	if err := os.MkdirAll(options.OutputDir, 0o700); err != nil {
		return manifest, err
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.FreeSpace == nil {
		options.FreeSpace = availableSpace
	}
	if options.ToolVersion == "" {
		options.ToolVersion = "dev"
	}
	if err := os.Mkdir(filepath.Join(options.OutputDir, "partitions"), 0o700); err != nil {
		return manifest, err
	}
	defer func() {
		report := map[string]any{"schema_version": 1, "operation": "backup create", "complete": resultErr == nil}
		if resultErr != nil {
			report["error"] = resultErr.Error()
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(options.OutputDir, "backup-report.json"), data, 0o600)
	}()

	mtdResult, err := remote.Run(ctx, "cat /proc/mtd", shell.RunOptions{})
	if err != nil {
		return manifest, device.WrapError(device.CodeRemoteCommandFailed, "cannot read /proc/mtd", err)
	}
	mtdText := string(mtdResult.Stdout)
	partitions, err := ParseMTD(mtdText)
	if err != nil {
		return manifest, device.WrapError(device.CodeBackupLayoutMismatch, "invalid device MTD layout", err)
	}
	if err := writeFullFile(filepath.Join(options.OutputDir, "mtd.txt"), []byte(mtdText), 0o600); err != nil {
		return manifest, err
	}
	var total uint64
	for _, partition := range partitions {
		if partition.Size > uint64(1<<63-1) {
			return manifest, &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "MTD partition exceeds the streaming limit"}
		}
		if ^uint64(0)-total < partition.Size {
			return manifest, &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "total MTD size overflows uint64"}
		}
		total += partition.Size
	}
	margin := total / 10
	if total > ^uint64(0)-margin {
		return manifest, &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "backup disk requirement overflows uint64"}
	}
	required := total + margin
	free, err := options.FreeSpace(options.OutputDir)
	if err != nil {
		return manifest, err
	}
	if free < required {
		return manifest, &device.Error{Code: device.CodeBackupIncomplete, Message: fmt.Sprintf("insufficient disk space: need %d bytes, have %d", required, free)}
	}

	manifest = Manifest{
		SchemaVersion: 1,
		ToolVersion:   options.ToolVersion,
		CreatedAt:     options.Now().UTC(),
		Device:        options.Device,
		BootID:        options.BootID,
		MTDTextFile:   "mtd.txt",
	}
	for _, partition := range partitions {
		source, err := readableMTDNode(ctx, remote, partition.Index)
		if err != nil {
			return Manifest{}, err
		}
		downloaded, err := downloadPartition(ctx, remote, options.OutputDir, partition, source)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Partitions = append(manifest.Partitions, downloaded)
	}
	manifest.Complete = true
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	manifestTemp := filepath.Join(options.OutputDir, "manifest.json.tmp")
	if err := writeFullFile(manifestTemp, manifestData, 0o600); err != nil {
		return Manifest{}, err
	}
	sumsTemp, err := writeSums(options.OutputDir, manifest, "manifest.json.tmp")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.Rename(sumsTemp, filepath.Join(options.OutputDir, "SHA256SUMS")); err != nil {
		return Manifest{}, err
	}
	if err := os.Rename(manifestTemp, filepath.Join(options.OutputDir, "manifest.json")); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func readableMTDNode(ctx context.Context, remote shell.StreamingShell, index int) (string, error) {
	primary := fmt.Sprintf("/dev/mtd%d", index)
	if _, err := remote.Run(ctx, "test -r "+primary, shell.RunOptions{}); err == nil {
		return primary, nil
	}
	fallback := fmt.Sprintf("/dev/mtdblock%d", index)
	if _, err := remote.Run(ctx, "test -r "+fallback, shell.RunOptions{}); err == nil {
		return fallback, nil
	}
	return "", &device.Error{Code: device.CodeBackupIncomplete, Message: fmt.Sprintf("MTD %d has no readable node", index)}
}

func downloadPartition(ctx context.Context, remote shell.StreamingShell, root string, partition Partition, source string) (Partition, error) {
	partition.Source = source
	partition.File = filepath.ToSlash(filepath.Join("partitions", fmt.Sprintf("mtd%02d_%s.bin", partition.Index, safeMTDName(partition.Name))))
	partial := filepath.Join(root, filepath.FromSlash(partition.File)) + ".partial"
	final := filepath.Join(root, filepath.FromSlash(partition.File))
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		_ = os.Remove(partial)
		file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return partition, err
		}
		hash := sha256.New()
		counter := &countingWriter{}
		writer := io.MultiWriter(file, hash, counter)
		result, streamErr := remote.Stream(ctx, "cat "+source, writer, int64(partition.Size)+1)
		if streamErr == nil && result.ExitCode != 0 {
			streamErr = &shell.RemoteCommandError{Result: result}
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if streamErr == nil && counter.count != int64(partition.Size) {
			streamErr = &device.Error{Code: device.CodeBackupIncomplete, Message: fmt.Sprintf("MTD %d length mismatch: got %d, want %d", partition.Index, counter.count, partition.Size)}
		}
		if streamErr == nil && syncErr != nil {
			streamErr = syncErr
		}
		if streamErr == nil && closeErr != nil {
			streamErr = closeErr
		}
		if streamErr == nil {
			partition.SHA256 = hex.EncodeToString(hash.Sum(nil))
			if err := os.Rename(partial, final); err != nil {
				return partition, err
			}
			return partition, nil
		}
		lastErr = streamErr
	}
	return partition, device.WrapError(device.CodeBackupIncomplete, fmt.Sprintf("MTD %d download failed", partition.Index), lastErr)
}

type countingWriter struct{ count int64 }

func (w *countingWriter) Write(data []byte) (int, error) {
	w.count += int64(len(data))
	return len(data), nil
}

var unsafeMTDName = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func safeMTDName(name string) string {
	name = unsafeMTDName.ReplaceAllString(name, "_")
	if name == "" {
		return "unnamed"
	}
	return name
}

func writeSums(root string, manifest Manifest, manifestTemp string) (string, error) {
	paths := []string{manifest.MTDTextFile, "manifest.json"}
	for _, partition := range manifest.Partitions {
		paths = append(paths, partition.File)
	}
	sort.Strings(paths)
	var text strings.Builder
	for _, relative := range paths {
		hashPath := relative
		if relative == "manifest.json" {
			hashPath = manifestTemp
		}
		_, hash, err := hashRegularFile(root, hashPath)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&text, "%s  %s\n", hash, relative)
	}
	temporary := filepath.Join(root, "SHA256SUMS.tmp")
	if err := writeFullFile(temporary, []byte(text.String()), 0o600); err != nil {
		return "", err
	}
	return temporary, nil
}

func writeFullFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
