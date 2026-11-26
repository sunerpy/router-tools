package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/mcpserver"
)

// TestMain re-executes the test binary as routerctl when ROUTERCTL_TEST_MAIN=1, so the end-to-end tests
// drive the real main without a separate build.
func TestMain(m *testing.M) {
	if os.Getenv("ROUTERCTL_TEST_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// routerctl returns a command for the real main with every user directory inside a temporary home.
func routerctl(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	home := t.TempDir()
	command := exec.Command(os.Args[0], args...)
	command.Env = append(os.Environ(),
		"ROUTERCTL_TEST_MAIN=1",
		"HOME="+home,
		"USERPROFILE="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"),
	)
	return command
}

func closedPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func textOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content %T", result.Content[0])
	}
	return text.Text
}

func exercise(t *testing.T, session *mcp.ClientSession) {
	t.Helper()
	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "guide_apply" || tool.Name == "soft_remove" {
			t.Fatalf("write tool %s offered without --allow-writes", tool.Name)
		}
	}
	profiles, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_profiles"})
	if err != nil || profiles.IsError || !strings.Contains(textOf(t, profiles), "xiaomi-rd08-1.1.96") {
		t.Fatalf("list_profiles: %v %+v", err, profiles)
	}
	port := closedPort(t)
	preflight, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "preflight", Arguments: map[string]any{
		"target": map[string]any{"host": "127.0.0.1", "web_port": port, "ssh_port": port}, "wait_seconds": 50,
	}})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	var result struct {
		Done   bool   `json:"done"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(textOf(t, preflight)), &result); err != nil {
		t.Fatal(err)
	}
	if !preflight.IsError || !result.Done || result.Status != "failed" {
		t.Fatalf("preflight to a closed port = %s", textOf(t, preflight))
	}
}

func TestMCPOverStdio(t *testing.T) {
	t.Parallel()
	command := routerctl(t, "mcp")
	var stderr strings.Builder
	command.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	exercise(t, session)
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v (stderr %q)", err, stderr.String())
	}
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 0 {
		t.Fatalf("exit = %v, stderr = %q", command.ProcessState, stderr.String())
	}
	if !strings.Contains(stderr.String(), `"audit":"mcp"`) {
		t.Fatalf("no audit lines: %q", stderr.String())
	}
}

type bearer struct{ token string }

func (b bearer) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func TestMCPOverHTTP(t *testing.T) {
	t.Parallel()
	token, err := mcpserver.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	command := routerctl(t, "mcp", "--http", "127.0.0.1:0")
	command.Env = append(command.Env, mcpserver.TokenEnv+"="+token)
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()

	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	listening := regexp.MustCompile(`listening on (http://127\.0\.0\.1:[0-9]+/mcp)`)
	var url string
	deadline := time.After(30 * time.Second)
	for url == "" {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("routerctl mcp --http exited before listening")
			}
			if strings.Contains(line, token) {
				t.Fatalf("stderr leaks the token: %q", line)
			}
			if match := listening.FindStringSubmatch(line); match != nil {
				url = match[1]
			}
		case <-deadline:
			t.Fatal("routerctl mcp --http did not report its address")
		}
	}
	go func() {
		for line := range lines {
			if strings.Contains(line, token) {
				panic("stderr leaks the token")
			}
		}
	}()

	unauthorized, err := http.Post(url, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, unauthorized.Body)
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without token: %d", unauthorized.StatusCode)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-http", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: url, HTTPClient: &http.Client{Transport: bearer{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	exercise(t, session)
	_ = session.Close()

	if runtime.GOOS == "windows" {
		// Windows cannot deliver os.Interrupt to another process; the stop path is covered on Linux and macOS.
		return
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("routerctl mcp --http exit: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("routerctl mcp --http did not stop on SIGINT")
	}
}

const (
	e2eAdminSecret = "E2e-Adm1n-Secret-Value"
	e2eRootSecret  = "E2e-R00t-Secret-Value"
)

// secretCalls sends passwords in every way an agent could: as the passwords of real calls, in a field of the
// wrong type, and as a goal value.
func secretCalls(t *testing.T, session *mcp.ClientSession) {
	t.Helper()
	port := closedPort(t)
	closed := map[string]any{"host": "127.0.0.1", "web_port": port, "ssh_port": port}
	calls := []mcp.CallToolParams{
		{Name: "preflight", Arguments: map[string]any{"target": closed, "check_admin_login": true, "check_root_login": true, "admin_password": e2eAdminSecret, "root_password": e2eRootSecret, "wait_seconds": 50}},
		{Name: "guide_plan", Arguments: map[string]any{"target": closed, "root_password": e2eRootSecret, "wait_seconds": 50}},
		{Name: "deep_status", Arguments: map[string]any{"target": map[string]any{"host": "127.0.0.1", "web_port": e2eRootSecret}}},
		{Name: "guide_plan", Arguments: map[string]any{"target": closed, "goals": []string{e2eAdminSecret}}},
	}
	for _, params := range calls {
		if _, err := session.CallTool(context.Background(), &params); err != nil {
			t.Fatalf("%s: %v", params.Name, err)
		}
	}
}

func assertNoSecrets(t *testing.T, where, text string) {
	t.Helper()
	for _, secret := range []string{e2eAdminSecret, e2eRootSecret} {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contains a password: %q", where, text)
		}
	}
}

func TestMCPNeverPrintsPasswordsOverStdio(t *testing.T) {
	t.Parallel()
	command := routerctl(t, "mcp")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr, protocol strings.Builder
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-secrets", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.IOTransport{
		Reader: io.NopCloser(io.TeeReader(stdout, &protocol)), Writer: stdin,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	secretCalls(t, session)
	_ = session.Close()
	if err := command.Wait(); err != nil {
		t.Fatalf("exit: %v (stderr %q)", err, stderr.String())
	}
	if !strings.Contains(protocol.String(), `"jsonrpc"`) || !strings.Contains(stderr.String(), `"audit":"mcp"`) {
		t.Fatalf("nothing captured: stdout %q stderr %q", protocol.String(), stderr.String())
	}
	assertNoSecrets(t, "stdout", protocol.String())
	assertNoSecrets(t, "stderr", stderr.String())
}

// teeBodies records every HTTP response body the client reads.
type teeBodies struct {
	token string
	mu    sync.Mutex
	seen  strings.Builder
}

func (t *teeBodies) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	response, err := http.DefaultTransport.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	response.Body = struct {
		io.Reader
		io.Closer
	}{io.TeeReader(response.Body, writerFunc(func(p []byte) (int, error) {
		t.mu.Lock()
		defer t.mu.Unlock()
		return t.seen.Write(p)
	})), response.Body}
	return response, nil
}

func (t *teeBodies) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.seen.String()
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestMCPNeverPrintsPasswordsOverHTTP(t *testing.T) {
	t.Parallel()
	token, err := mcpserver.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	command := routerctl(t, "mcp", "--http", "127.0.0.1:0")
	command.Env = append(command.Env, mcpserver.TokenEnv+"="+token)
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	var stderr strings.Builder
	var stderrMu sync.Mutex
	urls := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		listening := regexp.MustCompile(`listening on (http://127\.0\.0\.1:[0-9]+/mcp)`)
		for scanner.Scan() {
			stderrMu.Lock()
			stderr.WriteString(scanner.Text() + "\n")
			stderrMu.Unlock()
			if match := listening.FindStringSubmatch(scanner.Text()); match != nil {
				urls <- match[1]
			}
		}
	}()
	var url string
	select {
	case url = <-urls:
	case <-time.After(30 * time.Second):
		t.Fatal("no listening line")
	}
	bodies := &teeBodies{token: token}
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-secrets-http", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: url, HTTPClient: &http.Client{Transport: bodies},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	secretCalls(t, session)
	_ = session.Close()
	if !strings.Contains(bodies.String(), `"jsonrpc"`) {
		t.Fatalf("no response bodies captured: %q", bodies.String())
	}
	assertNoSecrets(t, "HTTP response bodies", bodies.String())
	if runtime.GOOS != "windows" {
		_ = command.Process.Signal(os.Interrupt)
		_ = command.Wait()
	} else {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	stderrMu.Lock()
	defer stderrMu.Unlock()
	assertNoSecrets(t, "stderr", stderr.String())
	if strings.Contains(stderr.String(), token) {
		t.Fatalf("stderr contains the token: %q", stderr.String())
	}
}
