package app

import (
	"sync"

	"github.com/sunerpy/router-tools/internal/device"
)

// OperationLocker prevents concurrent operations against one router, whatever its management ports.
type OperationLocker interface {
	Acquire(device.Target) (release func() error, err error)
}

type localLocker struct {
	mu   sync.Mutex
	held map[string]struct{}
}

func newLocalLocker() *localLocker {
	return &localLocker{held: make(map[string]struct{})}
}

func (l *localLocker) Acquire(target device.Target) (func() error, error) {
	key := device.RecoveryKey(target)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.held[key]; exists {
		return nil, &device.Error{Code: device.CodeOperationLocked, Message: "another operation already holds this target lock"}
	}
	l.held[key] = struct{}{}
	var once sync.Once
	return func() error {
		once.Do(func() {
			l.mu.Lock()
			delete(l.held, key)
			l.mu.Unlock()
		})
		return nil
	}, nil
}
