package app

import (
	"time"

	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// CredentialUse controls saved passwords for one request.
type CredentialUse struct {
	// UseSaved fills passwords that were not typed from the vault once the profile is resolved.
	UseSaved bool
	// Remember saves typed passwords after the device accepted them.
	Remember bool
	// Redactor, when set, learns every saved value the application fills in, so the caller's own output
	// pass hides it too. It never receives typed passwords from the application.
	Redactor *secret.Redactor
}

// Credential note actions.
const (
	CredentialUsedSaved  = "used_saved"
	CredentialSaved      = "saved"
	CredentialSaveFailed = "save_failed"
	CredentialRejected   = "rejected"
	// CredentialUnavailable: the vault could not be read; detection continued without saved passwords.
	CredentialUnavailable = "unavailable"
)

// CredentialNote reports what happened to a saved or remembered password; it never holds the value.
type CredentialNote struct {
	Kind   string        `json:"kind"`
	Action string        `json:"action"`
	Error  *device.Error `json:"error,omitempty"`
}

// checkTypedPassword runs before any network access: a typed password must be valid, and an empty one is
// allowed only when saved passwords may fill it.
func checkTypedPassword(password secret.Password, use CredentialUse) error {
	if password.Expose() == "" && use.UseSaved {
		return nil
	}
	return password.Validate()
}

// credentialSession applies the saved-password rules of one request after its profile is resolved.
type credentialSession struct {
	service  *Service
	use      CredentialUse
	target   device.Target
	profile  device.Profile
	redactor *secret.Redactor
	slots    map[credentials.Kind]*credentialSlot
	notes    []CredentialNote
}

type credentialSlot struct {
	key     credentials.Key
	value   secret.Password
	saved   bool
	savedAt time.Time
}

func (s *Service) credentialSession(use CredentialUse, target device.Target, profile device.Profile, redactor *secret.Redactor) *credentialSession {
	return &credentialSession{
		service: s, use: use, target: target, profile: profile, redactor: redactor,
		slots: make(map[credentials.Kind]*credentialSlot),
	}
}

// admin returns the typed admin password, or with UseSaved the saved one for this router's Web port.
func (c *credentialSession) admin(typed secret.Password) (secret.Password, error) {
	return c.fill(credentials.KindWebAdmin, c.target.WebPort, typed)
}

// root returns the typed root password, or with UseSaved the saved one for this router's SSH port. A
// saved root password is never sent over Telnet.
func (c *credentialSession) root(typed secret.Password, telnet bool) (secret.Password, error) {
	if typed.Expose() == "" && c.use.UseSaved && telnet {
		return secret.Password{}, &device.Error{
			Code:    device.CodeInvalidArgument,
			Message: "a saved root password is never sent over Telnet; type the root password or run without Telnet",
		}
	}
	return c.fill(credentials.KindRoot, c.target.SSHPort, typed)
}

func (c *credentialSession) fill(kind credentials.Kind, port int, typed secret.Password) (secret.Password, error) {
	key := credentials.NewKey(kind, c.profile.ModelID, c.target.Host, port)
	if typed.Expose() != "" {
		c.slots[kind] = &credentialSlot{key: key, value: typed}
		return typed, nil
	}
	if !c.use.UseSaved || c.service.credentials == nil {
		return secret.Password{}, typed.Validate()
	}
	value, entry, ok, err := c.service.credentials.Get(key)
	switch {
	case err != nil:
		return secret.Password{}, err
	case !ok:
		return secret.Password{}, &device.Error{Code: device.CodeCredentialNotFound, Message: "no " + string(kind) + " password is saved for this router; type it"}
	case entry.Rejected():
		return secret.Password{}, &device.Error{Code: device.CodeCredentialNotFound, Message: "the saved " + string(kind) + " password was rejected by the router; type it"}
	}
	c.redactor.Add(value.Expose())
	if c.use.Redactor != nil {
		c.use.Redactor.Add(value.Expose())
	}
	c.slots[kind] = &credentialSlot{key: key, value: value, saved: true, savedAt: entry.SavedAt}
	c.notes = append(c.notes, CredentialNote{Kind: string(kind), Action: CredentialUsedSaved})
	return value, nil
}

// hostKeys never accepts an unknown SSH host key automatically when a saved root password would be sent:
// the key must be known, pinned, or confirmed interactively.
func (c *credentialSession) hostKeys(policy hostkey.Policy) hostkey.Policy {
	if slot := c.slots[credentials.KindRoot]; slot != nil && slot.saved {
		policy.AcceptNew = false
	}
	return policy
}

// observe marks a saved password rejected when the device refused it; nothing is retried.
func (c *credentialSession) observe(err error) {
	for kind, code := range map[credentials.Kind]string{credentials.KindWebAdmin: device.CodeWebAuthFailed, credentials.KindRoot: device.CodeSSHAuthFailed} {
		slot := c.slots[kind]
		if slot == nil || !slot.saved || !hasCode(err, code) {
			continue
		}
		note := CredentialNote{Kind: string(kind), Action: CredentialRejected}
		if markErr := c.service.credentials.MarkRejected(slot.key, slot.savedAt, c.service.clock()); markErr != nil {
			note.Error = normalizeError(markErr)
		}
		c.notes = append(c.notes, note)
	}
}

// remember saves the typed passwords of the given kinds once the device accepted them. A failure to save
// is reported in the notes and never fails the device operation.
func (c *credentialSession) remember(kinds ...credentials.Kind) {
	if !c.use.Remember || c.service.credentials == nil {
		return
	}
	for _, kind := range kinds {
		slot := c.slots[kind]
		if slot == nil || slot.saved {
			continue
		}
		note := CredentialNote{Kind: string(kind), Action: CredentialSaved}
		if err := c.service.credentials.Put(slot.key, slot.value, c.service.clock()); err != nil {
			redacted := *normalizeError(err)
			redacted.Message = c.redactor.Text(redacted.Message)
			note.Action, note.Error = CredentialSaveFailed, &redacted
		}
		c.notes = append(c.notes, note)
	}
}

// available reports whether a usable saved password of kind exists for this router, without using it.
func (c *credentialSession) available(kind credentials.Kind, port int) bool {
	if !c.use.UseSaved || c.service.credentials == nil {
		return false
	}
	_, entry, ok, err := c.service.credentials.Get(credentials.NewKey(kind, c.profile.ModelID, c.target.Host, port))
	if err != nil {
		c.notes = append(c.notes, CredentialNote{Kind: string(kind), Action: CredentialUnavailable, Error: normalizeError(err)})
		return false
	}
	return ok && !entry.Rejected()
}

// savedRoot reports whether the root password in use came from the vault and was not refused.
func (c *credentialSession) savedRoot() bool {
	slot := c.slots[credentials.KindRoot]
	if slot == nil || !slot.saved {
		return false
	}
	for _, note := range c.notes {
		if note.Kind == string(credentials.KindRoot) && note.Action == CredentialRejected {
			return false
		}
	}
	return true
}

func (c *credentialSession) report() []CredentialNote {
	if c == nil {
		return nil
	}
	return c.notes
}

// CredentialReport lists saved passwords, or the ones a forget removed; it never holds a value.
type CredentialReport struct {
	SchemaVersion int                 `json:"schema_version"`
	Operation     string              `json:"operation"`
	Status        string              `json:"status"`
	Entries       []credentials.Entry `json:"entries"`
	Error         *device.Error       `json:"error,omitempty"`
}

func (s *Service) credentialReport(operation string, run func(credentials.Store) ([]credentials.Entry, error)) (CredentialReport, error) {
	report := CredentialReport{SchemaVersion: 1, Operation: operation, Status: "running", Entries: []credentials.Entry{}}
	if s.credentials == nil {
		err := &device.Error{Code: device.CodeCredentialStoreFailed, Message: "saved passwords are not configured"}
		report.Status, report.Error = StatusFailed, err
		return report, err
	}
	entries, err := run(s.credentials)
	if err != nil {
		normalized := normalizeError(err)
		report.Status, report.Error = StatusFailed, normalized
		return report, normalized
	}
	if entries != nil {
		report.Entries = entries
	}
	report.Status = StatusSucceeded
	return report, nil
}

// ListCredentials returns the metadata of every saved password.
func (s *Service) ListCredentials() (CredentialReport, error) {
	return s.credentialReport("credentials list", func(store credentials.Store) ([]credentials.Entry, error) {
		return store.List()
	})
}

// ForgetCredentials deletes the saved passwords matching a filter that sets at least one field.
func (s *Service) ForgetCredentials(filter credentials.Filter) (CredentialReport, error) {
	return s.credentialReport("credentials forget", func(store credentials.Store) ([]credentials.Entry, error) {
		if filter.Kind != "" && filter.Kind != credentials.KindWebAdmin && filter.Kind != credentials.KindRoot {
			return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "the kind is web-admin or root"}
		}
		if filter.Port < 0 || filter.Port > 65535 {
			return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "the port must be in [1, 65535]"}
		}
		return store.Delete(filter)
	})
}

// ForgetAllCredentials deletes every saved password and the vault key.
func (s *Service) ForgetAllCredentials() (CredentialReport, error) {
	return s.credentialReport("credentials forget --all", func(store credentials.Store) ([]credentials.Entry, error) {
		return nil, store.DeleteAll()
	})
}
