package app

import (
	"errors"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

func TestLocalLockerIgnoresManagementPorts(t *testing.T) {
	t.Parallel()

	locker := newLocalLocker()
	release, err := locker.Acquire(device.DefaultTarget())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	_, err = locker.Acquire(device.Target{Host: "192.168.31.1", WebPort: 8080, SSHPort: 2222, TelnetPort: 2323})
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeOperationLocked {
		t.Fatalf("Acquire(same router, other ports) error = %v, want OPERATION_LOCKED", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	again, err := locker.Acquire(device.Target{Host: "192.168.31.1", WebPort: 8080, SSHPort: 2222, TelnetPort: 2323})
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	_ = again()
}
