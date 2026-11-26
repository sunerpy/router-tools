package mcpserver

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// SettingsPersister loads and stores the desktop settings; SettingsStore is the production one.
type SettingsPersister interface {
	Load() (Settings, error)
	Save(Settings) error
}

// SettingsChange is what the desktop page sets. The token is changed only by NewToken.
type SettingsChange struct {
	Enabled     bool   `json:"enabled"`
	ListenHost  string `json:"listen_host"`
	Port        int    `json:"port"`
	AllowWrites bool   `json:"allow_writes"`
}

// ListenerStatus is what is actually listening, next to the stored settings.
type ListenerStatus struct {
	Listening bool `json:"listening"`
	// Address is the bound host:port while listening.
	Address        string    `json:"address,omitempty"`
	Enabled        bool      `json:"enabled"`
	ListenHost     string    `json:"listen_host"`
	Port           int       `json:"port"`
	AllowWrites    bool      `json:"allow_writes"`
	TokenSet       bool      `json:"token_set"`
	TokenCreatedAt time.Time `json:"token_created_at,omitzero"`
	LastError      string    `json:"last_error,omitempty"`
}

// ApplyResult carries a token created by enabling without one; it is returned once and kept only as a hash.
type ApplyResult struct {
	Status ListenerStatus `json:"status"`
	Token  string         `json:"token,omitempty"`
}

// ListenerOptions configure a Listener.
type ListenerOptions struct {
	Store SettingsPersister
	// Local lists this computer's addresses for validation; nil uses the network interfaces.
	Local func() ([]netip.Addr, error)
	// OnChange is called after every change of the status.
	OnChange func(ListenerStatus)
	// Describe renders an error for ListenerStatus.LastError; nil uses err.Error().
	Describe func(error) string
	now      func() time.Time
	// listen binds an address; tests replace it to make a bind fail.
	listen func(string) (net.Listener, error)
}

// Listener is the desktop's Streamable HTTP endpoint. Its lifetime is separate from the server's operations:
// turning it off, moving it, rotating the token or failing to bind never cancels an operation; only
// Server.Shutdown does. Stored settings always equal the last configuration that was applied successfully.
type Listener struct {
	server   *Server
	options  ListenerOptions
	verifier *TokenVerifier
	handler  http.Handler

	mu       sync.Mutex
	settings Settings
	active   *serving
	lastErr  string
}

type serving struct {
	listener net.Listener
	server   *http.Server
	// address is the configured host:port the listener serves.
	address string
}

// NewListener prepares the endpoint; Start applies the stored settings.
func NewListener(server *Server, options ListenerOptions) *Listener {
	if options.Local == nil {
		options.Local = interfaceAddresses
	}
	if options.now == nil {
		options.now = time.Now
	}
	if options.listen == nil {
		options.listen = listen
	}
	if options.Describe == nil {
		options.Describe = func(err error) string { return err.Error() }
	}
	verifier := NewTokenVerifier()
	return &Listener{server: server, options: options, verifier: verifier, handler: server.Handler(verifier), settings: DefaultSettings()}
}

// Start loads the stored settings, sets the write switch and the token, and listens when enabled. A file that
// cannot be read or an address that cannot be bound is reported in the status and changes nothing.
func (l *Listener) Start() ListenerStatus {
	l.mu.Lock()
	settings, err := l.options.Store.Load()
	if err != nil {
		l.lastErr = l.options.Describe(err)
		status := l.statusLocked()
		l.mu.Unlock()
		l.notify(status)
		return status
	}
	l.settings = settings
	l.server.SetAllowWrites(settings.AllowWrites)
	l.setHashLocked(settings.TokenSHA256)
	l.lastErr = ""
	if settings.Enabled && settings.TokenSHA256 != "" {
		listener, err := l.options.listen(settings.Address())
		if err != nil {
			l.lastErr = l.options.Describe(err)
		} else {
			l.active = l.serve(listener, settings.Address())
		}
	}
	status := l.statusLocked()
	l.mu.Unlock()
	l.notify(status)
	return status
}

// Apply validates and applies a change: it binds the new address first, stores the settings, then serves on the
// new address and stops the old one. When binding or storing fails, the previous listener keeps serving and the
// stored settings stay as they were.
func (l *Listener) Apply(change SettingsChange) (ApplyResult, error) {
	// One lock from the merge to the store, so a token rotated meanwhile is not overwritten with the old hash.
	l.mu.Lock()
	next := l.settings
	next.Enabled, next.ListenHost, next.Port, next.AllowWrites = change.Enabled, change.ListenHost, change.Port, change.AllowWrites
	if err := l.validate(next); err != nil {
		l.mu.Unlock()
		return ApplyResult{}, err
	}
	result, err := l.applyLocked(next)
	if err != nil {
		l.lastErr = l.options.Describe(err)
	} else {
		l.lastErr = ""
	}
	result.Status = l.statusLocked()
	l.mu.Unlock()
	l.notify(result.Status)
	return result, err
}

// validate checks a change. Only a change that listens on a specific address needs this computer's interfaces:
// turning the service off must work after the stored address left this computer, and loopback and the wildcards
// are valid while the interfaces cannot be listed.
func (l *Listener) validate(next Settings) error {
	if err := next.validateStored(); err != nil || !next.Enabled {
		return err
	}
	if next.Validate(nil) == nil {
		return nil
	}
	local, err := l.options.Local()
	if err != nil {
		return err
	}
	return next.Validate(local)
}

func (l *Listener) applyLocked(next Settings) (ApplyResult, error) {
	var result ApplyResult
	if next.Enabled && next.TokenSHA256 == "" {
		token, err := NewToken()
		if err != nil {
			return result, err
		}
		next.TokenSHA256, next.TokenCreatedAt, result.Token = hashHex(token), l.options.now().UTC(), token
	}

	previous := l.active
	var bound net.Listener
	closedPrevious := false
	if next.Enabled && (previous == nil || previous.address != next.Address()) {
		listener, err := l.options.listen(next.Address())
		if err != nil && previous != nil && AddressInUse(err) && samePort(previous.address, next.Address()) {
			// A wildcard and a specific address on one port exclude each other: free the port and try once more.
			previous.stop(false)
			closedPrevious = true
			listener, err = l.options.listen(next.Address())
		}
		if err != nil {
			l.restoreLocked(previous, closedPrevious)
			return ApplyResult{}, err
		}
		bound = listener
	}

	if err := l.options.Store.Save(next); err != nil {
		if bound != nil {
			_ = bound.Close()
		}
		l.restoreLocked(previous, closedPrevious)
		return ApplyResult{}, err
	}

	l.settings = next
	l.server.SetAllowWrites(next.AllowWrites)
	l.setHashLocked(next.TokenSHA256)
	switch {
	case !next.Enabled:
		if previous != nil && !closedPrevious {
			previous.stop(true)
		}
		l.active = nil
	case bound != nil:
		l.active = l.serve(bound, next.Address())
		if previous != nil && !closedPrevious {
			previous.stop(true)
		}
	}
	return result, nil
}

// restoreLocked brings back a listener that was closed to free its port.
func (l *Listener) restoreLocked(previous *serving, closed bool) {
	if !closed {
		return
	}
	l.active = nil
	if listener, err := l.options.listen(previous.address); err == nil {
		l.active = l.serve(listener, previous.address)
	}
}

// NewToken replaces the access token and returns it once; only its hash is stored. The old token is refused
// from the next request on.
func (l *Listener) NewToken() (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	l.mu.Lock()
	next := l.settings
	next.TokenSHA256, next.TokenCreatedAt = hashHex(token), l.options.now().UTC()
	if err := l.options.Store.Save(next); err != nil {
		l.mu.Unlock()
		return "", err
	}
	l.settings = next
	l.setHashLocked(next.TokenSHA256)
	status := l.statusLocked()
	l.mu.Unlock()
	l.notify(status)
	return token, nil
}

// Status reports what is listening and the stored settings.
func (l *Listener) Status() ListenerStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.statusLocked()
}

// Close stops listening without changing the stored settings or any operation. It can be called repeatedly.
func (l *Listener) Close() {
	l.mu.Lock()
	active := l.active
	l.active = nil
	status := l.statusLocked()
	l.mu.Unlock()
	if active != nil {
		active.stop(true)
		l.notify(status)
	}
}

func (l *Listener) statusLocked() ListenerStatus {
	status := ListenerStatus{
		Enabled: l.settings.Enabled, ListenHost: l.settings.ListenHost, Port: l.settings.Port, AllowWrites: l.settings.AllowWrites,
		TokenSet: l.settings.TokenSHA256 != "", TokenCreatedAt: l.settings.TokenCreatedAt, LastError: l.lastErr,
	}
	if l.active != nil {
		status.Listening, status.Address = true, l.active.address
	}
	return status
}

func (l *Listener) notify(status ListenerStatus) {
	if l.options.OnChange != nil {
		l.options.OnChange(status)
	}
}

func (l *Listener) setHashLocked(value string) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		l.verifier.Clear()
		return
	}
	var hash [32]byte
	copy(hash[:], decoded)
	l.verifier.SetHash(hash)
}

func (l *Listener) serve(listener net.Listener, address string) *serving {
	server := &http.Server{
		Handler: l.handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 16 << 10,
	}
	go func() { _ = server.Serve(listener) }()
	return &serving{listener: listener, server: server, address: address}
}

// listen binds host:port in the family of its IP literal, so 0.0.0.0 means IPv4 only and :: IPv6 only, as the
// user chose.
func listen(address string) (net.Listener, error) {
	network := "tcp4"
	if host, _, err := net.SplitHostPort(address); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil && addr.Is6() {
			network = "tcp6"
		}
	}
	return net.Listen(network, address)
}

// stop closes the socket at once, so its port is free, and lets open requests finish for up to five seconds
// (graceful) or ends them.
func (s *serving) stop(graceful bool) {
	_ = s.listener.Close()
	if !graceful {
		_ = s.server.Close()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			_ = s.server.Close()
		}
	}()
}

func hashHex(token string) string {
	sum := HashToken(token)
	return hex.EncodeToString(sum[:])
}

// AddressInUse reports whether err is the bind error for an address another socket holds.
func AddressInUse(err error) bool {
	return errors.Is(err, errAddressInUse)
}

func samePort(a, b string) bool {
	_, portA, errA := net.SplitHostPort(a)
	_, portB, errB := net.SplitHostPort(b)
	return errA == nil && errB == nil && portA == portB
}

func interfaceAddresses() ([]netip.Addr, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	local := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		if prefix, err := netip.ParsePrefix(address.String()); err == nil {
			local = append(local, prefix.Addr())
		}
	}
	return local, nil
}

// LocalAddresses lists this computer's interface addresses that a listener could bind, without loopback and
// link-local ones, for the desktop's address picker.
func LocalAddresses() []string {
	addresses, err := interfaceAddresses()
	if err != nil {
		return nil
	}
	var listed []string
	for _, address := range addresses {
		if address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
			continue
		}
		listed = append(listed, address.String())
	}
	return listed
}
