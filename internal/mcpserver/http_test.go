package mcpserver_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/mcpserver"
)

const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(clone)
}

func newHTTPServer(t *testing.T, token string) (*httptest.Server, *mcpserver.TokenVerifier) {
	t.Helper()
	verifier := mcpserver.NewTokenVerifier()
	if err := verifier.SetToken(token); err != nil {
		t.Fatal(err)
	}
	server := mcpserver.New(newFakeBackend(), mcpserver.NewOperations(), mcpserver.Options{Version: "v-test"})
	httpServer := httptest.NewServer(server.Handler(verifier))
	t.Cleanup(func() {
		httpServer.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	return httpServer, verifier
}

func post(t *testing.T, url string, headers map[string]string, body []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for name, value := range headers {
		if name == "Host" {
			request.Host = value
			continue
		}
		request.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestHTTPRequiresTheBearerToken(t *testing.T) {
	t.Parallel()
	token, _ := mcpserver.NewToken()
	server, _ := newHTTPServer(t, token)
	for name, headers := range map[string]map[string]string{
		"missing":      {},
		"wrong":        {"Authorization": "Bearer " + strings.Repeat("x", 40)},
		"other scheme": {"Authorization": "Basic " + token},
	} {
		response := post(t, server.URL+"/mcp", headers, []byte(initializeBody))
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s token: status %d", name, response.StatusCode)
		}
		if !strings.HasPrefix(response.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("%s token: WWW-Authenticate = %q", name, response.Header.Get("WWW-Authenticate"))
		}
	}
}

func TestHTTPServesTheToolsWithTheToken(t *testing.T) {
	t.Parallel()
	token, _ := mcpserver.NewToken()
	server, _ := newHTTPServer(t, token)
	client := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   server.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != len(readTools) {
		t.Fatalf("tools = %v, %v", tools, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_profiles"})
	if err != nil || result.IsError {
		t.Fatalf("list_profiles over HTTP: %v %+v", err, result)
	}
}

func TestHTTPRotatedTokenReplacesTheOldOne(t *testing.T) {
	t.Parallel()
	oldToken, _ := mcpserver.NewToken()
	newToken, _ := mcpserver.NewToken()
	server, verifier := newHTTPServer(t, oldToken)
	if err := verifier.SetToken(newToken); err != nil {
		t.Fatal(err)
	}
	if response := post(t, server.URL+"/mcp", map[string]string{"Authorization": "Bearer " + oldToken}, []byte(initializeBody)); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old token: status %d", response.StatusCode)
	}
	if response := post(t, server.URL+"/mcp", map[string]string{"Authorization": "Bearer " + newToken}, []byte(initializeBody)); response.StatusCode != http.StatusOK {
		t.Fatalf("new token: status %d", response.StatusCode)
	}
}

func TestHTTPRefusesRebindingCrossOriginOversizeAndOtherPaths(t *testing.T) {
	t.Parallel()
	token, _ := mcpserver.NewToken()
	server, _ := newHTTPServer(t, token)
	auth := "Bearer " + token

	if response := post(t, server.URL+"/mcp", map[string]string{"Authorization": auth, "Host": "evil.example"}, []byte(initializeBody)); response.StatusCode != http.StatusForbidden {
		t.Fatalf("DNS rebinding Host header: status %d", response.StatusCode)
	}
	crossSite := map[string]string{"Authorization": auth, "Sec-Fetch-Site": "cross-site", "Origin": "http://evil.example"}
	if response := post(t, server.URL+"/mcp", crossSite, []byte(initializeBody)); response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site browser request: status %d", response.StatusCode)
	}
	big := append([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"x":"`), bytes.Repeat([]byte("a"), 2<<20)...)
	big = append(big, []byte(`"}}`)...)
	if response := post(t, server.URL+"/mcp", map[string]string{"Authorization": auth}, big); response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("2 MiB body: status %d", response.StatusCode)
	}
	if response := post(t, server.URL+"/other", map[string]string{"Authorization": auth}, []byte(initializeBody)); response.StatusCode != http.StatusNotFound {
		t.Fatalf("other path: status %d", response.StatusCode)
	}
}

func TestParseListen(t *testing.T) {
	t.Parallel()
	valid := map[string]mcpserver.Listen{
		"127.0.0.1:18731": {Address: "127.0.0.1:18731", Loopback: true},
		"localhost:0":     {Address: "127.0.0.1:0", Loopback: true},
		"[::1]:18731":     {Address: "[::1]:18731", Loopback: true},
		"0.0.0.0:18731":   {Address: "0.0.0.0:18731"},
		"[::]:18731":      {Address: "[::]:18731"},
		"192.168.1.5:80":  {Address: "192.168.1.5:80"},
	}
	for input, want := range valid {
		got, err := mcpserver.ParseListen(input)
		if err != nil || got != want {
			t.Fatalf("ParseListen(%q) = %+v, %v; want %+v", input, got, err, want)
		}
	}
	for _, input := range []string{"18731", "router.lan:80", "127.0.0.1:65536", "127.0.0.1:-1", "127.0.0.1:x", "http://127.0.0.1:80", "[fe80::1%eth0]:80"} {
		if _, err := mcpserver.ParseListen(input); err == nil {
			t.Fatalf("ParseListen(%q) accepted", input)
		}
	}
}

func TestTokens(t *testing.T) {
	t.Parallel()
	token, err := mcpserver.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^rtmcp_[A-Za-z0-9_-]{43}$`).MatchString(token) {
		t.Fatalf("token format: %q", token)
	}
	other, _ := mcpserver.NewToken()
	if other == token {
		t.Fatal("two tokens are equal")
	}
	if err := mcpserver.CheckToken(token); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"short-token", strings.Repeat("a", 31), strings.Repeat("a", 31) + " b", strings.Repeat("é", 20)} {
		err := mcpserver.CheckToken(bad)
		if err == nil {
			t.Fatalf("CheckToken accepted %q", bad)
		}
		if strings.Contains(err.Error(), bad) {
			t.Fatalf("CheckToken echoes the token: %v", err)
		}
	}
	verifier := mcpserver.NewTokenVerifier()
	if verifier.Verify(token) || verifier.Verify("") {
		t.Fatal("an unset verifier accepted a token")
	}
	verifier.SetHash(mcpserver.HashToken(token))
	if !verifier.Verify(token) || verifier.Verify(other) {
		t.Fatal("hash verification is wrong")
	}
}

func TestServeStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mcpserver.Serve(ctx, listener, http.NotFoundHandler()) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not stop")
	}
}
