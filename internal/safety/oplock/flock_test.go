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
