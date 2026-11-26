package app

import "github.com/sunerpy/router-tools/internal/device"

// PlanDigestForTest lets external tests recompute the digest of a deliberately edited confirmation.
var PlanDigestForTest = planDigest

// LockForTest takes the service's own target lock, as a concurrent operation would.
func (s *Service) LockForTest(target device.Target) (func() error, error) {
	return s.locker.Acquire(target)
}
