package mcpserver

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

const (
	planTTL  = 30 * time.Minute
	maxPlans = 32
)

// plans keeps ready guided plans for guide_apply. A plan is used at most once, so a repeated or replayed
// guide_apply needs a new guide_plan.
type plans struct {
	mu      sync.Mutex
	entries map[string]*storedPlan
	order   []string
	now     func() time.Time
}

type storedPlan struct {
	confirmation app.GuideConfirmation
	digest       string
	host         string
	expires      time.Time
}

func newPlans(now func() time.Time) *plans {
	return &plans{entries: make(map[string]*storedPlan), now: now}
}

func (p *plans) put(confirmation app.GuideConfirmation, digest string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	id := hex.EncodeToString(value)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries[id] = &storedPlan{
		confirmation: confirmation, digest: digest, host: device.CanonicalHost(confirmation.Target.Host),
		expires: p.now().Add(planTTL),
	}
	p.order = append(p.order, id)
	for len(p.order) > maxPlans {
		delete(p.entries, p.order[0])
		p.order = p.order[1:]
	}
	return id, nil
}

// host returns the canonical host of a stored plan, or "" when there is none.
func (p *plans) host(id string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry, ok := p.entries[id]; ok {
		return entry.host
	}
	return ""
}

var errPlanUnavailable = &device.Error{
	Code:    device.CodeConfirmationRequired,
	Message: "this plan is unknown, expired or already used; run guide_plan again and get the user's approval",
}

// take removes and returns a plan whose digest matches. A wrong digest leaves the plan in place.
func (p *plans) take(id, digest string) (app.GuideConfirmation, string, *device.Error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[id]
	if !ok {
		return app.GuideConfirmation{}, "", errPlanUnavailable
	}
	if p.now().After(entry.expires) {
		p.remove(id)
		return app.GuideConfirmation{}, "", errPlanUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(entry.digest), []byte(digest)) != 1 {
		return app.GuideConfirmation{}, "", &device.Error{
			Code:    device.CodeConfirmationRequired,
			Message: "plan_digest does not match this plan; pass the digest guide_plan returned",
		}
	}
	p.remove(id)
	return entry.confirmation, entry.digest, nil
}

func (p *plans) remove(id string) {
	delete(p.entries, id)
	p.order = slices.DeleteFunc(p.order, func(candidate string) bool { return candidate == id })
}
