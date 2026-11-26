package mcpserver_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/mcpserver"
)

type memoryStore struct {
	mu       sync.Mutex
	settings mcpserver.Settings
	saves    int
	fail     error
}

func (m *memoryStore) Load() (mcpserver.Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings, nil
}

func (m *memoryStore) Save(settings mcpserver.Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return m.fail
	}
	m.settings = settings
	m.saves++
	return nil
}

func (m *memoryStore) current() mcpserver.Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

func (m *memoryStore) failWith(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fail = err
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func newListener(t *testing.T, h *harness, store *memoryStore) *mcpserver.Listener {
	t.Helper()
	listener := mcpserver.NewListener(h.server, mcpserver.ListenerOptions{
		Store: store,
		Local: func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil },
	})
	t.Cleanup(listener.Close)
	return listener
}

func enabledAt(host string, port int) mcpserver.SettingsChange {
	return mcpserver.SettingsChange{Enabled: true, ListenHost: host, Port: port}
}

// initialize posts an MCP initialize request to host:port and returns the HTTP status, or 0 when nothing listens.
func initialize(t *testing.T, address, token string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "http://"+address+"/mcp", strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	// A fresh connection per request: a pooled one could belong to a listener the test just replaced.
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	response, err := client.Do(request)
	if err != nil {
		return 0
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}

func address(port int) string { return "127.0.0.1:" + strconv.Itoa(port) }

func TestListenerIsOffByDefault(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	status := newListener(t, h, &memoryStore{settings: mcpserver.DefaultSettings()}).Start()
	if status.Listening || status.Enabled || status.LastError != "" || status.TokenSet {
		t.Fatalf("status = %+v", status)
	}
}

// Enabling without a token creates one, keeps only its hash, and serves it; the next apply keeps the token.
func TestListenerEnableCreatesATokenOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	port := freePort(t)
	result, err := listener.Apply(enabledAt("127.0.0.1", port))
	if err != nil || !strings.HasPrefix(result.Token, "rtmcp_") || !result.Status.Listening || result.Status.Address != address(port) || !result.Status.TokenSet {
		t.Fatalf("Apply() = %+v, %v", result, err)
	}
	sum := sha256.Sum256([]byte(result.Token))
	if stored := store.current(); stored.TokenSHA256 != hex.EncodeToString(sum[:]) || stored.TokenCreatedAt.IsZero() || !stored.Enabled || stored.Port != port {
		t.Fatalf("stored %+v", stored)
	}
	if code := initialize(t, address(port), ""); code != http.StatusUnauthorized {
		t.Fatalf("without the token: %d", code)
	}
	if code := initialize(t, address(port), result.Token); code != http.StatusOK {
		t.Fatalf("with the token: %d", code)
	}
	again, err := listener.Apply(enabledAt("127.0.0.1", port))
	if err != nil || again.Token != "" || !again.Status.Listening {
		t.Fatalf("second Apply() = %+v, %v", again, err)
	}
	if code := initialize(t, address(port), result.Token); code != http.StatusOK {
		t.Fatalf("the token changed: %d", code)
	}
}

// A failed bind leaves the previous listener serving and the stored settings unchanged.
func TestListenerBindFailureKeepsTheOldListener(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	first := freePort(t)
	enabled, err := listener.Apply(enabledAt("127.0.0.1", first))
	if err != nil {
		t.Fatal(err)
	}
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	second := busy.Addr().(*net.TCPAddr).Port
	if _, err := listener.Apply(enabledAt("127.0.0.1", second)); err == nil {
		t.Fatal("bound a port in use")
	}
	status := listener.Status()
	if !status.Listening || status.Address != address(first) || status.Port != first || status.LastError == "" {
		t.Fatalf("status after the failed bind = %+v", status)
	}
	if stored := store.current(); stored.Port != first {
		t.Fatalf("stored port = %d, want %d", stored.Port, first)
	}
	if code := initialize(t, address(first), enabled.Token); code != http.StatusOK {
		t.Fatalf("the old listener stopped: %d", code)
	}
}

// When the settings cannot be stored, the new listener is closed and the previous one keeps serving.
func TestListenerPersistFailureRollsBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	first := freePort(t)
	enabled, err := listener.Apply(enabledAt("127.0.0.1", first))
	if err != nil {
		t.Fatal(err)
	}
	store.failWith(errors.New("disk full"))
	second := freePort(t)
	if _, err := listener.Apply(enabledAt("127.0.0.1", second)); err == nil {
		t.Fatal("applied without storing")
	}
	// Not even bound: an unserved socket would still accept connections.
	if connection, err := net.DialTimeout("tcp", address(second), time.Second); err == nil {
		_ = connection.Close()
		t.Fatal("the new address is still bound")
	}
	if code := initialize(t, address(first), enabled.Token); code != http.StatusOK {
		t.Fatalf("the old listener stopped: %d", code)
	}
	if status := listener.Status(); status.Address != address(first) || status.Port != first {
		t.Fatalf("status = %+v", status)
	}
	change := enabledAt("127.0.0.1", first)
	change.AllowWrites = true
	if _, err := listener.Apply(change); err == nil || h.server.WritesEnabled() {
		t.Fatalf("writes turned on without storing: %v, %v", err, h.server.WritesEnabled())
	}
	if _, err := listener.NewToken(); err == nil {
		t.Fatal("rotated the token without storing")
	}
	if code := initialize(t, address(first), enabled.Token); code != http.StatusOK {
		t.Fatalf("a failed rotation replaced the token: %d", code)
	}
}

// A wildcard and a specific address on the same port replace each other.
func TestListenerSwapsAddressesOnTheSamePort(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	port := freePort(t)
	enabled, err := listener.Apply(enabledAt("0.0.0.0", port))
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"127.0.0.1", "0.0.0.0", "127.0.0.1"} {
		result, err := listener.Apply(enabledAt(host, port))
		if err != nil || result.Status.Address != net.JoinHostPort(host, strconv.Itoa(port)) || store.current().ListenHost != host {
			t.Fatalf("swap to %s: %+v, %v", host, result, err)
		}
		if code := initialize(t, address(port), enabled.Token); code != http.StatusOK {
			t.Fatalf("after the swap to %s: %d", host, code)
		}
	}
}

// Turning the service off or moving it never cancels an agent operation.
func TestListenerChangesLeaveOperationsRunning(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	if _, err := listener.Apply(enabledAt("127.0.0.1", freePort(t))); err != nil {
		t.Fatal(err)
	}
	started := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	if started.Done || started.OperationID == "" {
		t.Fatalf("started = %+v", started)
	}
	if _, err := listener.Apply(enabledAt("127.0.0.1", freePort(t))); err != nil {
		t.Fatal(err)
	}
	if _, err := listener.Apply(mcpserver.SettingsChange{ListenHost: "127.0.0.1", Port: mcpserver.DefaultPort}); err != nil {
		t.Fatal(err)
	}
	if status := listener.Status(); status.Listening || status.Enabled {
		t.Fatalf("still listening: %+v", status)
	}
	listener.Close()
	for _, summary := range h.ops.List() {
		if summary.OperationID == started.OperationID && summary.Done {
			t.Fatalf("the operation ended with the listener: %+v", summary)
		}
	}
	close(backend.gate)
	final := h.call("operation_wait", map[string]any{"operation_id": started.OperationID, "wait_seconds": 5})
	if !final.Done || final.Status != "succeeded" {
		t.Fatalf("final = %+v", final)
	}
}

// A new token replaces the old one at once.
func TestListenerRotatesTheToken(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	port := freePort(t)
	enabled, err := listener.Apply(enabledAt("127.0.0.1", port))
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := listener.NewToken()
	if err != nil || !strings.HasPrefix(rotated, "rtmcp_") || rotated == enabled.Token {
		t.Fatalf("NewToken() = %q, %v", rotated, err)
	}
	if code := initialize(t, address(port), enabled.Token); code != http.StatusUnauthorized {
		t.Fatalf("the old token still works: %d", code)
	}
	if code := initialize(t, address(port), rotated); code != http.StatusOK {
		t.Fatalf("the new token is refused: %d", code)
	}
	sum := sha256.Sum256([]byte(rotated))
	if store.current().TokenSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("the stored hash is not the new token's")
	}
}

func TestListenerWriteSwitchIsStored(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	change := enabledAt("127.0.0.1", freePort(t))
	change.AllowWrites = true
	result, err := listener.Apply(change)
	if err != nil || !result.Status.AllowWrites || !h.server.WritesEnabled() || !store.current().AllowWrites {
		t.Fatalf("Apply() = %+v, %v", result, err)
	}
	if !contains(h.toolNames(), "guide_apply") {
		t.Fatal("guide_apply is not offered")
	}
	change.AllowWrites = false
	if _, err := listener.Apply(change); err != nil || h.server.WritesEnabled() || contains(h.toolNames(), "guide_apply") {
		t.Fatalf("writes still on: %v", err)
	}
}

// Stored settings that cannot be served are reported, not changed.
func TestListenerStartReportsABindFailure(t *testing.T) {
	t.Parallel()
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	port := busy.Addr().(*net.TCPAddr).Port
	stored := mcpserver.Settings{Enabled: true, ListenHost: "127.0.0.1", Port: port, TokenSHA256: strings.Repeat("ab", 32)}
	store := &memoryStore{settings: stored}
	h := newHarness(t, newFakeBackend(), false)
	status := newListener(t, h, store).Start()
	if status.Listening || !status.Enabled || status.LastError == "" || store.current() != stored {
		t.Fatalf("status = %+v", status)
	}
}

func TestListenerDescribesItsErrors(t *testing.T) {
	t.Parallel()
	busy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	port := busy.Addr().(*net.TCPAddr).Port
	stored := mcpserver.Settings{Enabled: true, ListenHost: "127.0.0.1", Port: port, TokenSHA256: strings.Repeat("ab", 32)}
	h := newHarness(t, newFakeBackend(), false)
	listener := mcpserver.NewListener(h.server, mcpserver.ListenerOptions{
		Store:    &memoryStore{settings: stored},
		Local:    func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil },
		Describe: func(err error) string { return "described: " + strconv.FormatBool(mcpserver.AddressInUse(err)) },
	})
	t.Cleanup(listener.Close)
	if status := listener.Start(); status.LastError != "described: true" {
		t.Fatalf("start: last error = %q", status.LastError)
	}
	_, err = listener.Apply(enabledAt("127.0.0.1", port))
	if !mcpserver.AddressInUse(err) || listener.Status().LastError != "described: true" {
		t.Fatalf("apply: err = %v, last error = %q", err, listener.Status().LastError)
	}
}

func TestListenerTurnsOffAnAddressThisComputerNoLongerHas(t *testing.T) {
	t.Parallel()
	stored := mcpserver.Settings{Enabled: true, ListenHost: "192.0.2.10", Port: 18731, AllowWrites: true, TokenSHA256: strings.Repeat("ab", 32)}
	store := &memoryStore{settings: stored}
	h := newHarness(t, newFakeBackend(), false)
	noInterfaces := errors.New("no interfaces")
	listener := mcpserver.NewListener(h.server, mcpserver.ListenerOptions{
		Store: store,
		Local: func() ([]netip.Addr, error) { return nil, noInterfaces },
	})
	t.Cleanup(listener.Close)
	listener.Start()
	if _, err := listener.Apply(mcpserver.SettingsChange{Enabled: true, ListenHost: "192.0.2.10", Port: 18731, AllowWrites: true}); !errors.Is(err, noInterfaces) {
		t.Fatalf("enable: err = %v", err)
	}
	result, err := listener.Apply(mcpserver.SettingsChange{ListenHost: "192.0.2.10", Port: 18731})
	if err != nil {
		t.Fatalf("turn off: %v", err)
	}
	got := store.current()
	if result.Status.Listening || result.Status.Enabled || got.Enabled || got.AllowWrites || got.ListenHost != "192.0.2.10" || got.TokenSHA256 != stored.TokenSHA256 {
		t.Fatalf("status = %+v, stored = %+v", result.Status, got)
	}
}

// Loopback and the wildcards are valid without listing this computer's interfaces, so the service can be
// turned on there and its write switch turned off while that list is unavailable.
func TestListenerNeedsNoInterfaceListForLoopbackOrWildcards(t *testing.T) {
	t.Parallel()
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	h := newHarness(t, newFakeBackend(), false)
	noInterfaces := errors.New("no interfaces")
	listener := mcpserver.NewListener(h.server, mcpserver.ListenerOptions{
		Store: store,
		Local: func() ([]netip.Addr, error) { return nil, noInterfaces },
	})
	t.Cleanup(listener.Close)
	listener.Start()
	port := freePort(t)
	change := enabledAt("127.0.0.1", port)
	change.AllowWrites = true
	if _, err := listener.Apply(change); err != nil {
		t.Fatalf("enable on loopback: %v", err)
	}
	change.AllowWrites = false
	if _, err := listener.Apply(change); err != nil || h.server.WritesEnabled() || store.current().AllowWrites {
		t.Fatalf("writes off: err = %v, writes = %v", err, h.server.WritesEnabled())
	}
	wildcard := enabledAt("0.0.0.0", freePort(t))
	if _, err := listener.Apply(wildcard); err != nil {
		t.Fatalf("enable on 0.0.0.0: %v", err)
	}
	if _, err := listener.Apply(enabledAt("192.0.2.10", port)); !errors.Is(err, noInterfaces) {
		t.Fatalf("a specific address must still be checked: %v", err)
	}
	if _, err := listener.Apply(enabledAt("127.0.0.1", 80)); err == nil || errors.Is(err, noInterfaces) {
		t.Fatalf("a bad port must be reported as such: %v", err)
	}
}

func TestListenerKeepsATokenRotatedDuringApply(t *testing.T) {
	t.Parallel()
	// A specific address makes Apply list the interfaces, which is where the race window was. 127.0.0.2 is
	// loopback on Linux and Windows; macOS configures only 127.0.0.1.
	probe, err := net.Listen("tcp4", "127.0.0.2:0")
	if err != nil {
		t.Skip("127.0.0.2 cannot be bound here")
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	h := newHarness(t, newFakeBackend(), false)
	var (
		listener *mcpserver.Listener
		once     sync.Once
		rotated  string
	)
	done := make(chan struct{})
	listener = mcpserver.NewListener(h.server, mcpserver.ListenerOptions{
		Store: store,
		Local: func() ([]netip.Addr, error) {
			once.Do(func() {
				// 生成新令牌 clicked while 应用更改 is validating.
				go func() {
					defer close(done)
					token, err := listener.NewToken()
					if err == nil {
						rotated = token
					}
				}()
				select {
				case <-done:
				case <-time.After(200 * time.Millisecond):
				}
			})
			return []netip.Addr{netip.MustParseAddr("127.0.0.2")}, nil
		},
	})
	t.Cleanup(listener.Close)
	listener.Start()
	if _, err := listener.NewToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := listener.Apply(enabledAt("127.0.0.2", port)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the rotation never finished")
	}
	sum := mcpserver.HashToken(rotated)
	if rotated == "" || store.current().TokenSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("stored hash = %s, want the rotated token's", store.current().TokenSHA256)
	}
	if code := initialize(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)), rotated); code != http.StatusOK {
		t.Fatalf("rotated token: status %d", code)
	}
}

func TestListenerWildcardIPv4ListensOnIPv4Only(t *testing.T) {
	t.Parallel()
	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback")
	}
	_ = probe.Close()
	h := newHarness(t, newFakeBackend(), false)
	listener := newListener(t, h, &memoryStore{settings: mcpserver.DefaultSettings()})
	listener.Start()
	port := freePort(t)
	result, err := listener.Apply(enabledAt("0.0.0.0", port))
	if err != nil {
		t.Fatal(err)
	}
	if code := initialize(t, address(port), result.Token); code != http.StatusOK {
		t.Fatalf("IPv4: status %d", code)
	}
	if connection, err := net.DialTimeout("tcp6", net.JoinHostPort("::1", strconv.Itoa(port)), time.Second); err == nil {
		_ = connection.Close()
		t.Fatal("0.0.0.0 also accepted IPv6 connections")
	}
}

func TestListenerStartAppliesTheStoredWriteSwitch(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	stored := mcpserver.DefaultSettings()
	stored.AllowWrites = true
	listener := newListener(t, h, &memoryStore{settings: stored})
	if status := listener.Start(); !status.AllowWrites || !h.server.WritesEnabled() {
		t.Fatalf("status = %+v, writes = %v", status, h.server.WritesEnabled())
	}
}

func TestListenerRefusesInvalidSettings(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	store := &memoryStore{settings: mcpserver.DefaultSettings()}
	listener := newListener(t, h, store)
	listener.Start()
	for _, change := range []mcpserver.SettingsChange{enabledAt("8.8.8.8", 18731), enabledAt("127.0.0.1", 80), enabledAt("localhost", 18731)} {
		if _, err := listener.Apply(change); err == nil {
			t.Fatalf("applied %+v", change)
		}
	}
	if store.saves != 0 || listener.Status().Listening {
		t.Fatalf("an invalid change took effect: saves = %d", store.saves)
	}
}
