package credentials

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/gofrs/flock"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

const (
	schemaVersion  = 1
	maxVaultSize   = 1 << 20
	maxKeyFileSize = 4 << 10
	maxCiphertext  = 1024 + 16
	keySize        = 32
	nonceSize      = 12
	vaultFileName  = "vault.json"
	keyFileName    = "vault.key"
	lockFileName   = "vault.lock"
	lockTimeout    = 5 * time.Second
)

var scopePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// FileVault stores entries in Dir/vault.json and the protected key in Dir/vault.key; Dir/vault.lock
// serializes every read and write across processes.
type FileVault struct {
	Dir string
}

var _ Store = FileVault{}

type vaultFile struct {
	SchemaVersion int          `json:"schema_version"`
	Entries       []vaultEntry `json:"entries"`
}

type vaultEntry struct {
	Entry
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// List returns every entry's metadata. It needs no key, so entries stay listable and forgettable even when
// the key is gone.
func (v FileVault) List() ([]Entry, error) {
	var entries []Entry
	err := v.locked(func() error {
		file, err := v.load()
		if err != nil {
			return err
		}
		for _, entry := range file.Entries {
			entries = append(entries, entry.Entry)
		}
		return nil
	})
	return entries, err
}

// Get decrypts the saved password for key. A value that cannot be decrypted is reported, never returned.
func (v FileVault) Get(key Key) (secret.Password, Entry, bool, error) {
	var value secret.Password
	var found Entry
	ok := false
	err := v.locked(func() error {
		file, err := v.load()
		if err != nil {
			return err
		}
		index := file.find(key)
		if index < 0 {
			return nil
		}
		aead, err := v.cipher(false)
		if err != nil {
			return err
		}
		entry := file.Entries[index]
		plaintext, err := open(aead, entry)
		if err != nil {
			return err
		}
		password, err := secret.NewPassword(string(plaintext))
		if err != nil {
			return storeError("a saved password is not valid", err)
		}
		value, found, ok = password, entry.Entry, true
		return nil
	})
	if err != nil {
		return secret.Password{}, Entry{}, false, err
	}
	return value, found, ok, nil
}

// Put saves password for key, replacing an earlier value and its rejection mark.
func (v FileVault) Put(key Key, password secret.Password, at time.Time) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := password.Validate(); err != nil {
		return err
	}
	return v.locked(func() error {
		file, err := v.load()
		if err != nil {
			return err
		}
		aead, err := v.cipher(len(file.Entries) == 0)
		if err != nil {
			return err
		}
		nonce := make([]byte, nonceSize)
		if _, err := rand.Read(nonce); err != nil {
			return storeError("cannot generate a nonce", err)
		}
		sealed := aead.Seal(nil, nonce, []byte(password.Expose()), additionalData(key))
		entry := vaultEntry{
			Entry: Entry{Key: key, SavedAt: at.UTC()}, Nonce: base64.StdEncoding.EncodeToString(nonce),
			Ciphertext: base64.StdEncoding.EncodeToString(sealed),
		}
		if index := file.find(key); index >= 0 {
			file.Entries[index] = entry
		} else {
			file.Entries = append(file.Entries, entry)
		}
		return v.save(file)
	})
}

// MarkRejected records when the device refused the value of key that was saved at savedAt.
func (v FileVault) MarkRejected(key Key, savedAt, at time.Time) error {
	return v.locked(func() error {
		file, err := v.load()
		if err != nil {
			return err
		}
		index := file.find(key)
		if index < 0 || !file.Entries[index].SavedAt.Equal(savedAt) {
			return nil
		}
		file.Entries[index].RejectedAt = at.UTC()
		return v.save(file)
	})
}

// Delete removes the entries matching filter, which must set at least one field.
func (v FileVault) Delete(filter Filter) ([]Entry, error) {
	if filter.Empty() {
		return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "choose which saved passwords to forget, or forget all"}
	}
	var deleted []Entry
	err := v.locked(func() error {
		file, err := v.load()
		if err != nil {
			return err
		}
		kept := file.Entries[:0]
		for _, entry := range file.Entries {
			if filter.Matches(entry.Key) {
				deleted = append(deleted, entry.Entry)
			} else {
				kept = append(kept, entry)
			}
		}
		if len(deleted) == 0 {
			return nil
		}
		file.Entries = kept
		return v.save(file)
	})
	return deleted, err
}

// DeleteAll removes the entries and the vault key.
func (v FileVault) DeleteAll() error {
	return v.locked(func() error {
		for _, name := range []string{vaultFileName, keyFileName} {
			if err := os.Remove(filepath.Join(v.Dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return storeError("cannot delete the password vault", err)
			}
		}
		return syncDir(v.Dir)
	})
}

func (v FileVault) locked(run func() error) error {
	if v.Dir == "" {
		return storeError("the password vault directory is not configured", nil)
	}
	if err := os.MkdirAll(v.Dir, 0o700); err != nil {
		return storeError("cannot create the password vault directory", err)
	}
	if err := os.Chmod(v.Dir, 0o700); err != nil {
		return storeError("cannot restrict the password vault directory", err)
	}
	lock := flock.New(filepath.Join(v.Dir, lockFileName))
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil || !locked {
		return storeError("the password vault is busy", err)
	}
	defer func() { _ = lock.Unlock() }()
	return run()
}

func (v FileVault) load() (vaultFile, error) {
	empty := vaultFile{SchemaVersion: schemaVersion}
	data, err := readLimited(filepath.Join(v.Dir, vaultFileName), maxVaultSize)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var file vaultFile
	if err := decoder.Decode(&file); err != nil {
		return empty, storeError("the password vault cannot be decoded", err)
	}
	if len(bytes.TrimSpace(data[decoder.InputOffset():])) != 0 {
		return empty, storeError("the password vault has trailing data", nil)
	}
	if file.SchemaVersion != schemaVersion {
		return empty, storeError("the password vault has an unsupported schema version", nil)
	}
	seen := make(map[Key]bool, len(file.Entries))
	for _, entry := range file.Entries {
		if err := validateEntry(entry); err != nil {
			return empty, err
		}
		if seen[entry.Key] {
			return empty, storeError("the password vault has a duplicate entry", nil)
		}
		seen[entry.Key] = true
	}
	return file, nil
}

func (v FileVault) save(file vaultFile) error {
	file.SchemaVersion = schemaVersion
	if file.Entries == nil {
		file.Entries = []vaultEntry{}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return storeError("cannot encode the password vault", err)
	}
	return writeAtomic(v.Dir, vaultFileName, append(data, '\n'))
}

// cipher loads the vault key, creating one only when the vault holds no entry, so existing entries are
// never orphaned by a replaced key.
func (v FileVault) cipher(create bool) (cipher.AEAD, error) {
	path := filepath.Join(v.Dir, keyFileName)
	blob, err := readLimited(path, maxKeyFileSize)
	var raw []byte
	switch {
	case errors.Is(err, os.ErrNotExist) && create:
		raw = make([]byte, keySize)
		if _, err := rand.Read(raw); err != nil {
			return nil, storeError("cannot generate the vault key", err)
		}
		protected, err := protectKey(raw)
		if err != nil {
			return nil, storeError("cannot protect the vault key", err)
		}
		if err := writeAtomic(v.Dir, keyFileName, protected); err != nil {
			return nil, err
		}
	case errors.Is(err, os.ErrNotExist):
		return nil, storeError("the vault key is missing; saved passwords cannot be read; forget them and save again", nil)
	case err != nil:
		return nil, err
	default:
		raw, err = unprotectKey(blob)
		if err != nil {
			return nil, storeError("the vault key cannot be unprotected for this user", err)
		}
	}
	if len(raw) != keySize {
		return nil, storeError("the vault key has the wrong size", nil)
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, storeError("cannot use the vault key", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, storeError("cannot use the vault key", err)
	}
	return aead, nil
}

func open(aead cipher.AEAD, entry vaultEntry) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(entry.Nonce)
	if err != nil || len(nonce) != nonceSize {
		return nil, storeError("a saved password has an invalid nonce", err)
	}
	sealed, err := base64.StdEncoding.DecodeString(entry.Ciphertext)
	if err != nil {
		return nil, storeError("a saved password has invalid ciphertext", err)
	}
	plaintext, err := aead.Open(nil, nonce, sealed, additionalData(entry.Key))
	if err != nil {
		return nil, storeError("a saved password cannot be decrypted", err)
	}
	return plaintext, nil
}

// additionalData binds the ciphertext to its entry, so an entry copied to another router, kind or port
// does not decrypt.
func additionalData(key Key) []byte {
	return []byte("router-tools/vault/v1\x00" + string(key.Kind) + "\x00" + key.Scope + "\x00" + key.Host + "\x00" + strconv.Itoa(key.Port))
}

func (f vaultFile) find(key Key) int {
	for i, entry := range f.Entries {
		if entry.Key == key {
			return i
		}
	}
	return -1
}

func validateKey(key Key) error {
	invalid := func(message string) error {
		return &device.Error{Code: device.CodeInvalidArgument, Message: message}
	}
	switch {
	case key.Kind != KindWebAdmin && key.Kind != KindRoot:
		return invalid("a saved password is either web-admin or root")
	case !scopePattern.MatchString(key.Scope):
		return invalid("a saved password needs the device model ID as its scope")
	case key.Port < 1 || key.Port > 65535:
		return invalid("a saved password port must be in [1, 65535]")
	case device.CanonicalHost(key.Host) != key.Host:
		return invalid("a saved password host must use the canonical spelling")
	}
	return device.Target{Host: key.Host, WebPort: 1, SSHPort: 1, TelnetPort: 1}.Validate()
}

func validateEntry(entry vaultEntry) error {
	if err := validateKey(entry.Key); err != nil {
		return storeError("the password vault has an invalid entry", err)
	}
	if entry.SavedAt.IsZero() {
		return storeError("the password vault has an entry without a save time", nil)
	}
	nonce, err := base64.StdEncoding.DecodeString(entry.Nonce)
	if err != nil || len(nonce) != nonceSize {
		return storeError("the password vault has an entry with an invalid nonce", err)
	}
	sealed, err := base64.StdEncoding.DecodeString(entry.Ciphertext)
	if err != nil || len(sealed) == 0 || len(sealed) > maxCiphertext {
		return storeError("the password vault has an entry with invalid ciphertext", err)
	}
	return nil
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, storeError("cannot open the password vault", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, storeError("cannot read the password vault", err)
	}
	if int64(len(data)) > limit {
		return nil, storeError(fmt.Sprintf("%s exceeds %d bytes", filepath.Base(path), limit), nil)
	}
	return data, nil
}

// writeAtomic replaces dir/name with data through a synced temporary file in the same directory.
func writeAtomic(dir, name string, data []byte) error {
	temp, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return storeError("cannot write the password vault", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	fail := func(err error) error {
		_ = temp.Close()
		return storeError("cannot write the password vault", err)
	}
	if err := temp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := temp.Write(data); err != nil {
		return fail(err)
	}
	if err := temp.Sync(); err != nil {
		return fail(err)
	}
	if err := temp.Close(); err != nil {
		return storeError("cannot write the password vault", err)
	}
	if err := os.Rename(tempPath, filepath.Join(dir, name)); err != nil {
		return storeError("cannot write the password vault", err)
	}
	if err := syncDir(dir); err != nil {
		return storeError("cannot sync the password vault directory", err)
	}
	return nil
}
