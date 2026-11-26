package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
)

// mcpApplication is a complete application whose only implemented method is Catalog; the MCP tests never
// reach the others.
type mcpApplication struct{ guiapi.Application }

func (mcpApplication) Catalog() []device.CatalogEntry {
	return []device.CatalogEntry{{ModelID: "xiaomi-rd08", Vendor: "Xiaomi", Model: "RD08"}}
}

type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func waitForText(t *testing.T, buffer *syncBuffer, pattern *regexp.Regexp) []string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if match := pattern.FindStringSubmatch(buffer.String()); match != nil {
			return match
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s in %q", pattern, buffer.String())
	return nil
}

func TestMCPTokenPrintsANewToken(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := executeWithInput(t, mcpApplication{}, "", "mcp", "token")
	if code != 0 || !regexp.MustCompile(`^rtmcp_[A-Za-z0-9_-]{43}\n$`).MatchString(stdout) {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestMCPHTTPRefusesMissingOrInvalidTokensAndAddresses(t *testing.T) {
	t.Setenv(mcpserver.TokenEnv, "")
	code, _, stderr := executeWithInput(t, mcpApplication{}, "", "mcp", "--http", "127.0.0.1:0")
	if code != 2 || !strings.Contains(stderr, mcpserver.TokenEnv) {
		t.Fatalf("missing token: code = %d, stderr = %q", code, stderr)
	}

	t.Setenv(mcpserver.TokenEnv, "tok-abc-too-short")
	code, _, stderr = executeWithInput(t, mcpApplication{}, "", "mcp", "--http", "127.0.0.1:0")
	if code != 2 || strings.Contains(stderr, "tok-abc-too-short") {
		t.Fatalf("short token: code = %d, stderr = %q", code, stderr)
	}

	token, _ := mcpserver.NewToken()
	t.Setenv(mcpserver.TokenEnv, token)
	for _, address := range []string{"router.lan:18731", "18731", "127.0.0.1:70000"} {
		if code, _, stderr = executeWithInput(t, mcpApplication{}, "", "mcp", "--http", address); code != 2 {
			t.Fatalf("address %q: code = %d, stderr = %q", address, code, stderr)
		}
	}
	if code, _, stderr = executeWithInput(t, mcpApplication{}, "", "mcp", "--token-file", "x"); code != 2 {
		t.Fatalf("--token-file without --http: code = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stderr, token) {
		t.Fatalf("stderr leaks the token: %q", stderr)
	}
}

func TestMCPTokenFileMustBePrivate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	token, _ := mcpserver.NewToken()
	path := filepath.Join(t.TempDir(), "mcp.token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := executeWithInput(t, mcpApplication{}, "", "mcp", "--http", "127.0.0.1:0", "--token-file", path)
	if code != 2 || !strings.Contains(stderr, "chmod 600") || strings.Contains(stderr, token) {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

func TestMCPHTTPServesUntilCanceled(t *testing.T) {
	t.Parallel()
	token, _ := mcpserver.NewToken()
	path := filepath.Join(t.TempDir(), "mcp.token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stderr := &syncBuffer{}
	var stdout bytes.Buffer
	codes := make(chan int, 1)
	go func() {
		codes <- cli.ExecuteWithInput(ctx, []string{"mcp", "--http", "127.0.0.1:0", "--token-file", path},
			strings.NewReader(""), &stdout, stderr, mcpApplication{}, buildinfo.Info{Version: "test"})
	}()
	url := waitForText(t, stderr, regexp.MustCompile(`listening on (http://127\.0\.0\.1:[0-9]+/mcp)`))[1]

	unauthorized, err := http.Post(url, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without token: %d", unauthorized.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+token)
	authorized, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = authorized.Body.Close()
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("with token: %d", authorized.StatusCode)
	}

	cancel()
	select {
	case code := <-codes:
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("routerctl mcp --http did not stop")
	}
	if strings.Contains(stderr.String(), token) || stdout.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestMCPWarnsAboutNonLoopbackAddresses(t *testing.T) {
	t.Parallel()
	token, _ := mcpserver.NewToken()
	path := filepath.Join(t.TempDir(), "mcp.token")
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stderr := &syncBuffer{}
	codes := make(chan int, 1)
	go func() {
		codes <- cli.ExecuteWithInput(ctx, []string{"mcp", "--http", "0.0.0.0:0", "--token-file", path},
			strings.NewReader(""), io.Discard, stderr, mcpApplication{}, buildinfo.Info{Version: "test"})
	}()
	waitForText(t, stderr, regexp.MustCompile(`listening on http://`))
	if !strings.Contains(stderr.String(), "plaintext") {
		t.Fatalf("no plaintext warning: %q", stderr.String())
	}
	cancel()
	if code := <-codes; code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}

// Over stdio, stdout carries only MCP messages; warnings and audit lines go to stderr.
func TestMCPStdioKeepsStdoutForProtocolMessages(t *testing.T) {
	t.Parallel()
	for _, allowWrites := range []bool{false, true} {
		stdinReader, stdinWriter := io.Pipe()
		stdout, stderr := &syncBuffer{}, &syncBuffer{}
		args := []string{"mcp"}
		if allowWrites {
			args = append(args, "--allow-writes")
		}
		codes := make(chan int, 1)
		go func() {
			codes <- cli.ExecuteWithInput(t.Context(), args, stdinReader, stdout, stderr, mcpApplication{}, buildinfo.Info{Version: "test"})
		}()
		// The writer runs on its own so that a command that never reads stdin fails the test instead of
		// blocking it.
		go func() {
			for _, line := range []string{
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
				`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
				`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_profiles","arguments":{}}}`,
			} {
				if _, err := io.WriteString(stdinWriter, line+"\n"); err != nil {
					return
				}
			}
		}()
		select {
		case code := <-codes:
			t.Fatalf("routerctl mcp exited early with %d: %q", code, stderr.String())
		case <-time.After(50 * time.Millisecond):
		}
		waitForText(t, stdout, regexp.MustCompile(`"id":3`))
		_ = stdinWriter.Close()
		select {
		case code := <-codes:
			if code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
			}
		case <-time.After(15 * time.Second):
			t.Fatal("routerctl mcp did not stop at end of input")
		}
		scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		var listing string
		for scanner.Scan() {
			var message map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil || message["jsonrpc"] != "2.0" {
				t.Fatalf("stdout carries a non-protocol line: %q", scanner.Text())
			}
			if id, _ := message["id"].(float64); id == 2 {
				listing = scanner.Text()
			}
		}
		if strings.Contains(listing, `"guide_apply"`) != allowWrites || !strings.Contains(stdout.String(), "xiaomi-rd08") {
			t.Fatalf("allowWrites=%v: listing = %s", allowWrites, listing)
		}
		if !strings.Contains(stderr.String(), `"audit":"mcp"`) || !strings.Contains(stderr.String(), `"tool":"list_profiles"`) {
			t.Fatalf("no audit line on stderr: %q", stderr.String())
		}
		if allowWrites != strings.Contains(stderr.String(), "router writes are enabled") {
			t.Fatalf("allowWrites=%v: stderr = %q", allowWrites, stderr.String())
		}
	}
}
