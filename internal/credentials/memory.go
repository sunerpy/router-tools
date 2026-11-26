package credentials

import (
	"sync"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

// Memory is an in-process Store for tests and previews; it never touches the disk.
type Memory struct {
	mu      sync.Mutex
	entries map[Key]memoryEntry
	// Fail, when set, is returned by every method, as a broken vault would.
	Fail error
}

type memoryEntry struct {
	entry    Entry
	password secret.Password
}

var _ Store = (*Memory)(nil)

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory { return &Memory{entries: make(map[Key]memoryEntry)} }

func (m *Memory) List() ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return nil, m.Fail
	}
	entries := make([]Entry, 0, len(m.entries))
	for _, saved := range m.entries {
		entries = append(entries, saved.entry)
	}
	return entries, nil
}

func (m *Memory) Get(key Key) (secret.Password, Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return secret.Password{}, Entry{}, false, m.Fail
	}
	saved, ok := m.entries[key]
	return saved.password, saved.entry, ok, nil
}

func (m *Memory) Put(key Key, password secret.Password, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return m.Fail
	}
	if err := validateKey(key); err != nil {
		return err
	}
	m.entries[key] = memoryEntry{entry: Entry{Key: key, SavedAt: at.UTC()}, password: password}
	return nil
}

func (m *Memory) MarkRejected(key Key, savedAt, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return m.Fail
	}
	if saved, ok := m.entries[key]; ok && saved.entry.SavedAt.Equal(savedAt) {
		saved.entry.RejectedAt = at.UTC()
		m.entries[key] = saved
	}
	return nil
}

func (m *Memory) Delete(filter Filter) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return nil, m.Fail
	}
	if filter.Empty() {
		return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "choose which saved passwords to forget, or forget all"}
	}
	var deleted []Entry
	for key, saved := range m.entries {
		if filter.Matches(key) {
			deleted = append(deleted, saved.entry)
			delete(m.entries, key)
		}
	}
	return deleted, nil
}

func (m *Memory) DeleteAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Fail != nil {
		return m.Fail
	}
	m.entries = make(map[Key]memoryEntry)
	return nil
}
