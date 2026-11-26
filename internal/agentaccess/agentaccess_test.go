package agentaccess_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/agentaccess"
	"github.com/sunerpy/router-tools/internal/agentsetup"
	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
)

// backend answers every use case at once, or blocks backup_create until gate closes.
type backend struct {
	gate chan struct{}
}

func (b *backend) Doctor(context.Context, guiapi.OperationRequest) (app.DoctorReport, error) {
	return app.DoctorReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) GuidePlan(context.Context, guiapi.GuideRequest) (app.GuidePlanReport, error) {
	return app.GuidePlanReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) GuideApply(context.Context, guiapi.GuideRequest) (app.GuideApplyReport, error) {
	return app.GuideApplyReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) PersistRemove(context.Context, guiapi.OperationRequest) (app.SoftReport, error) {
	return app.SoftReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) BackupCreate(ctx context.Context, _ guiapi.OperationRequest) (app.BackupCreateReport, error) {
	if b.gate != nil {
		select {
		case <-b.gate:
		case <-ctx.Done():
			err := &device.Error{Code: device.CodeCanceled, Message: "canceled"}
			return app.BackupCreateReport{Status: app.StatusFailed, Error: err}, err
		}
	}
	return app.BackupCreateReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) DeepStatus(context.Context, guiapi.OperationRequest) (app.DeepStatusReport, error) {
	return app.DeepStatusReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) CredentialStatus() (app.CredentialReport, error) {
	return app.CredentialReport{Status: app.StatusSucceeded}, nil
}

func (b *backend) Catalog() []device.CatalogEntry { return nil }

type fixture struct {
	controller *agentaccess.Controller
	backend    *backend
	home       string
	changes    atomic.Int32
	mu         sync.Mutex
	env        map[string]string
}

func (f *fixture) setEnv(name, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.env[name] = value
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{backend: &backend{}, home: t.TempDir(), env: map[string]string{}}
	f.controller = agentaccess.New(f.backend, mcpserver.NewOperations(), agentaccess.Options{
		SettingsDir: filepath.Join(t.TempDir(), "mcp"),
		Version:     "v-test",
		GOOS:        "linux",
		Home:        func() (string, error) { return f.home, nil },
		Getenv: func(name string) string {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.env[name]
		},
		Local:    func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil },
		OnChange: func(agentaccess.Status) { f.changes.Add(1) },
	})
	f.controller.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		f.controller.Shutdown(ctx)
	})
	return f
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type bearer struct{ token string }

func (b bearer) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: url, HTTPClient: &http.Client{Transport: bearer{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect %s: %v", url, err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestStatusNamesTheURLAgentsConnectTo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if status := f.controller.Status(); status.Listener.Listening || status.URL != "http://127.0.0.1:18731/mcp" || len(status.Agents) == 0 {
		t.Fatalf("status before enabling = %+v", status)
	}
	port := freePort(t)
	result, err := f.controller.Apply(mcpserver.SettingsChange{Enabled: true, ListenHost: "0.0.0.0", Port: port})
	if err != nil || result.Token == "" {
		t.Fatalf("Apply() = %+v, %v", result, err)
	}
	want := "http://127.0.0.1:" + strconv.Itoa(port) + "/mcp"
	if status := f.controller.Status(); !status.Listener.Listening || status.URL != want {
		t.Fatalf("status = %+v, want URL %s", status, want)
	}
	setup, err := f.controller.Setup(agentsetup.Codex)
	if err != nil || setup.Transport != "http" || !strings.Contains(setup.Snippet, want) || !strings.Contains(setup.Snippet, "ROUTERCTL_MCP_TOKEN") {
		t.Fatalf("Setup(codex) = %+v, %v", setup, err)
	}
	if strings.Contains(setup.Snippet, result.Token) || strings.Contains(setup.Command, result.Token) {
		t.Fatal("a setup carries the token")
	}
	// The desktop's notes are Chinese, like the rest of its UI.
	if notes := strings.Join(setup.Notes, "\n"); !strings.Contains(notes, "令牌") {
		t.Fatalf("notes are not Chinese: %s", notes)
	}
	if _, err := f.controller.Setup(agentsetup.WorkBuddy); err == nil {
		t.Fatal("WorkBuddy got an HTTP setup")
	}
	if _, err := f.controller.Setup("nope"); err == nil {
		t.Fatal("an unknown agent got a setup")
	}
}

// A port another program holds is explained in Chinese, and the status carries the same text.
func TestABusyPortIsExplainedInChinese(t *testing.T) {
	busy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	port := busy.Addr().(*net.TCPAddr).Port
	f := newFixture(t)
	_, err = f.controller.Apply(mcpserver.SettingsChange{Enabled: true, ListenHost: "127.0.0.1", Port: port})
	if err == nil || !strings.Contains(err.Error(), "已被其他程序占用") || !strings.Contains(err.Error(), strconv.Itoa(port)) || !mcpserver.AddressInUse(err) {
		t.Fatalf("err = %v", err)
	}
	if last := f.controller.Status().Listener.LastError; last != err.Error() {
		t.Fatalf("last error = %q, want %q", last, err.Error())
	}
}

// Calls and operations of agents appear in the status; the desktop cancels one by ID; a change notifies.
func TestActivityAndOperations(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.backend.gate = make(chan struct{})
	port := freePort(t)
	result, err := f.controller.Apply(mcpserver.SettingsChange{Enabled: true, ListenHost: "127.0.0.1", Port: port})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, f.controller.Status().URL, result.Token)
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_profiles", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "backup_create", Arguments: map[string]any{
		"target": map[string]any{"host": "192.168.31.1"}, "wait_seconds": 0,
	}}); err != nil {
		t.Fatal(err)
	}
	var running mcpserver.OperationSummary
	waitFor(t, "the running operation", func() bool {
		for _, summary := range f.controller.Status().Operations {
			if summary.Tool == "backup_create" && !summary.Done {
				running = summary
				return true
			}
		}
		return false
	})
	if !f.controller.Owns(running.OperationID) || f.controller.Owns("desktop-1") {
		t.Fatal("Owns does not tell agent operations apart")
	}
	if !f.controller.Cancel(running.OperationID) {
		t.Fatal("Cancel refused the running operation")
	}
	waitFor(t, "the canceled operation", func() bool {
		for _, summary := range f.controller.Status().Operations {
			if summary.OperationID == running.OperationID {
				return summary.Done
			}
		}
		return false
	})
	activity := f.controller.Status().Activity
	tools := map[string]bool{}
	for _, record := range activity {
		tools[record.Tool+"/"+record.Kind] = true
	}
	if !tools["list_profiles/call"] || !tools["backup_create/call"] || !tools["backup_create/finish"] {
		t.Fatalf("activity = %+v", activity)
	}
	waitFor(t, "change notifications", func() bool { return f.changes.Load() >= 3 })
}

func TestStatusNamesWhereEachSkillGoes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	override := filepath.Join(t.TempDir(), "claude")
	f.setEnv("CLAUDE_CONFIG_DIR", override)
	paths := f.controller.Status().SkillPaths
	want := map[string]string{
		agentsetup.ClaudeCode: filepath.Join(override, "skills", "router-tools"),
		agentsetup.Codex:      filepath.Join(f.home, ".agents", "skills", "router-tools"),
	}
	for agent, path := range want {
		if paths[agent] != path {
			t.Errorf("%s: path = %q, want %q", agent, paths[agent], path)
		}
	}
	for _, agent := range []string{agentsetup.WorkBuddy, agentsetup.Generic} {
		if _, ok := paths[agent]; ok {
			t.Errorf("%s has no documented skills directory but got %q", agent, paths[agent])
		}
	}
	// The status names the directory InstallSkill writes to.
	report, err := f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode})
	if err != nil || report.Path != paths[agentsetup.ClaudeCode] {
		t.Fatalf("install = %+v, %v", report, err)
	}
	f.setEnv("CLAUDE_CONFIG_DIR", "relative")
	if path, ok := f.controller.Status().SkillPaths[agentsetup.ClaudeCode]; ok {
		t.Fatalf("a relative override has no path, got %q", path)
	}
}

// A relative directory is refused even when it exists: the desktop only installs into a folder the user picked.
func TestInstallSkillRefusesARelativeDirectory(t *testing.T) {
	f := newFixture(t)
	work := t.TempDir()
	t.Chdir(work)
	_, err := f.controller.InstallSkill(agentaccess.SkillRequest{Dir: "."})
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeInvalidArgument {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "router-tools")); !os.IsNotExist(err) {
		t.Fatalf("installed into the working directory: %v", err)
	}
}

func TestInstallSkill(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	report, err := f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode})
	installed := filepath.Join(f.home, ".claude", "skills", "router-tools")
	if err != nil || report.Status != "installed" || report.Path != installed {
		t.Fatalf("install = %+v, %v", report, err)
	}
	if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode}); err != nil || report.Status != "unchanged" {
		t.Fatalf("second install = %+v, %v", report, err)
	}
	if err := os.WriteFile(filepath.Join(installed, "SKILL.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode}); err != nil || report.Status != "differs" || report.Path != installed || report.Message == "" {
		t.Fatalf("changed copy = %+v, %v", report, err)
	}
	if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode, Force: true}); err != nil || report.Status != "updated" {
		t.Fatalf("forced install = %+v, %v", report, err)
	}

	dir := t.TempDir()
	if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Dir: dir}); err != nil || report.Status != "installed" || report.Path != filepath.Join(dir, "router-tools") {
		t.Fatalf("--dir install = %+v, %v", report, err)
	}
	if runtime.GOOS != "windows" {
		linked := t.TempDir()
		if err := os.Symlink(t.TempDir(), filepath.Join(linked, "router-tools")); err != nil {
			t.Fatal(err)
		}
		if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Dir: linked}); err != nil || report.Status != "refused" {
			t.Fatalf("symbolic link = %+v, %v", report, err)
		}
	}

	claudeDir := t.TempDir()
	f.setEnv("CLAUDE_CONFIG_DIR", claudeDir)
	if report, err = f.controller.InstallSkill(agentaccess.SkillRequest{Agent: agentsetup.ClaudeCode}); err != nil || report.Path != filepath.Join(claudeDir, "skills", "router-tools") {
		t.Fatalf("CLAUDE_CONFIG_DIR install = %+v, %v", report, err)
	}

	for name, request := range map[string]agentaccess.SkillRequest{
		"neither":       {},
		"both":          {Agent: agentsetup.Codex, Dir: dir},
		"no directory":  {Agent: agentsetup.WorkBuddy},
		"generic":       {Agent: agentsetup.Generic},
		"unknown":       {Agent: "nope"},
		"relative dir":  {Dir: "skills"},
		"missing dir":   {Dir: filepath.Join(dir, "missing")},
		"file as a dir": {Dir: filepath.Join(dir, "router-tools", "SKILL.md")},
	} {
		if report, err := f.controller.InstallSkill(request); err == nil {
			t.Fatalf("%s: installed %+v", name, report)
		}
	}
}

// Shutdown stops listening and cancels running agent operations.
func TestShutdownCancelsOperations(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.backend.gate = make(chan struct{})
	port := freePort(t)
	result, err := f.controller.Apply(mcpserver.SettingsChange{Enabled: true, ListenHost: "127.0.0.1", Port: port})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, f.controller.Status().URL, result.Token)
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "backup_create", Arguments: map[string]any{
		"target": map[string]any{"host": "192.168.31.1"}, "wait_seconds": 0,
	}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if remaining := f.controller.Shutdown(ctx); len(remaining) != 0 {
		t.Fatalf("operations left running: %v", remaining)
	}
	if status := f.controller.Status(); status.Listener.Listening {
		t.Fatalf("still listening: %+v", status)
	}
	for _, summary := range f.controller.Status().Operations {
		if !summary.Done {
			t.Fatalf("operation still running: %+v", summary)
		}
	}
}
