package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/device"
)

const (
	// TokenEnv is the environment variable that carries the HTTP access token.
	TokenEnv       = "ROUTERCTL_MCP_TOKEN"
	tokenPrefix    = "rtmcp_"
	minTokenLength = 32
	maxBodyBytes   = 1 << 20
	sessionTimeout = 30 * time.Minute
	// EndpointPath is the only path the HTTP transport serves.
	EndpointPath = "/mcp"
)

// NewToken returns a random access token: "rtmcp_" and 43 base64url characters (32 random bytes).
func NewToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(value), nil
}

// CheckToken validates a user-supplied access token without revealing it.
func CheckToken(token string) error {
	if len(token) < minTokenLength {
		return &device.Error{Code: device.CodeInvalidArgument, Message: "the MCP access token must have at least 32 characters; create one with routerctl mcp token"}
	}
	for _, r := range token {
		if r <= ' ' || r > '~' {
			return &device.Error{Code: device.CodeInvalidArgument, Message: "the MCP access token must be printable ASCII without spaces"}
		}
	}
	return nil
}

// HashToken is the only form in which a token is kept.
func HashToken(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

// TokenVerifier accepts a bearer token whose SHA-256 equals the configured hash. It can be rotated while the
// server runs; the next request with the old token is refused.
type TokenVerifier struct {
	mu   sync.RWMutex
	hash [32]byte
	set  bool
}

// NewTokenVerifier returns a verifier that refuses everything until a token or hash is set.
func NewTokenVerifier() *TokenVerifier {
	return &TokenVerifier{}
}

// SetToken validates the token and keeps only its hash.
func (v *TokenVerifier) SetToken(token string) error {
	if err := CheckToken(token); err != nil {
		return err
	}
	v.SetHash(HashToken(token))
	return nil
}

// SetHash replaces the accepted token hash.
func (v *TokenVerifier) SetHash(hash [32]byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hash, v.set = hash, true
}

// Clear refuses every token until a token or hash is set again.
func (v *TokenVerifier) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hash, v.set = [32]byte{}, false
}

// Verify compares in constant time.
func (v *TokenVerifier) Verify(presented string) bool {
	v.mu.RLock()
	hash, set := v.hash, v.set
	v.mu.RUnlock()
	if !set || presented == "" {
		return false
	}
	sum := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(sum[:], hash[:]) == 1
}

func bearer(verifier *TokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || !verifier.Verify(strings.TrimSpace(token)) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="router-tools"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler serves the Streamable HTTP transport at /mcp. Requests need the bearer token; cross-origin browser
// requests are refused; the SDK's DNS-rebinding protection stays on; bodies are capped at 1 MiB.
func (s *Server) Handler(verifier *TokenVerifier) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, &mcp.StreamableHTTPOptions{
		SessionTimeout: sessionTimeout, MaxRequestBodyBytes: maxBodyBytes,
	})
	mux := http.NewServeMux()
	mux.Handle(EndpointPath, bearer(verifier, streamable))
	return http.NewCrossOriginProtection().Handler(mux)
}

// Listen is a validated listen address.
type Listen struct {
	// Address is host:port with the host as an IP literal.
	Address string
	// Loopback is true for 127.0.0.0/8 and ::1.
	Loopback bool
}

// ParseListen accepts host:port where host is an IP address or localhost (taken as 127.0.0.1), so binding
// never depends on name resolution. Port 0 lets the system choose.
func ParseListen(address string) (Listen, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Listen{}, &device.Error{Code: device.CodeInvalidArgument, Message: "the listen address must be host:port, for example 127.0.0.1:18731"}
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return Listen{}, &device.Error{Code: device.CodeInvalidArgument, Message: "the listen host must be an IP address or localhost"}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return Listen{}, &device.Error{Code: device.CodeInvalidArgument, Message: "the listen port must be between 0 and 65535"}
	}
	return Listen{Address: net.JoinHostPort(ip.String(), strconv.Itoa(number)), Loopback: ip.IsLoopback()}, nil
}

// Serve answers HTTP on the listener until ctx ends, then stops within five seconds.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 16 << 10,
	}
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
		<-errs
		return nil
	}
}
