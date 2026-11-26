package oplock_test

import (
	"errors"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/oplock"
)

func TestFlockLockerIsNonBlockingAndReleases(t *testing.T) {
	t.Parallel()

	locker := oplock.FlockLocker{Dir: t.TempDir()}
	target := device.DefaultTarget()
	release, err := locker.Acquire(target)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if _, err := locker.Acquire(target); err == nil {
		t.Fatal("second Acquire() succeeded while locked")
	} else {
		var deviceErr *device.Error
		if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeOperationLocked {
			t.Fatalf("second Acquire() error = %v", err)
		}
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	releaseAgain, err := locker.Acquire(target)
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	_ = releaseAgain()
}

func TestFlockLockerIgnoresManagementPorts(t *testing.T) {
	t.Parallel()

	locker := oplock.FlockLocker{Dir: t.TempDir()}
	release, err := locker.Acquire(device.DefaultTarget())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer func() { _ = release() }()
	for name, target := range map[string]device.Target{
		"web port":    {Host: "192.168.31.1", WebPort: 8080, SSHPort: 22, TelnetPort: 23},
		"ssh port":    {Host: "192.168.31.1", WebPort: 80, SSHPort: 2222, TelnetPort: 23},
		"telnet port": {Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 2323},
	} {
		_, err := locker.Acquire(target)
		var deviceErr *device.Error
		if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeOperationLocked {
			t.Fatalf("%s: Acquire() error = %v, want OPERATION_LOCKED for the same router", name, err)
		}
	}
	other, err := locker.Acquire(device.Target{Host: "192.168.31.2", WebPort: 80, SSHPort: 22, TelnetPort: 23})
	if err != nil {
		t.Fatalf("Acquire(other router) error = %v", err)
	}
	_ = other()
}
