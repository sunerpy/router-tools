package main

import (
	"context"
	"os"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sunerpy/router-tools/internal/agentaccess"
	"github.com/sunerpy/router-tools/internal/agentsetup"
	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/wiring"
)

// App exposes typed desktop operations to Wails.
type App struct {
	ctx     context.Context
	service *guiapi.Service
	broker  *promptBroker
	agents  *agentaccess.Controller
}

// NewApp constructs the desktop binding around the shared application service. Agent operations run through
// the same guiapi.Service, so their progress reaches both the window and the MCP server, an unknown host key
// in an agent run opens the same dialog, and their reports join the history marked as agent runs.
func NewApp(application guiapi.Application, paths wiring.Paths) *App {
	broker := &promptBroker{pending: make(map[string]chan bool)}
	app := &App{broker: broker}
	operations := mcpserver.NewOperations()
	app.service = guiapi.New(application, fanout{runtimeEmitter{app: app}, operations}, broker)
	app.agents = agentaccess.New(app.service, operations, agentaccess.Options{
		SettingsDir: paths.MCP, Version: buildinfo.Current().Version, GOOS: goruntime.GOOS,
		Home: os.UserHomeDir, Getenv: os.Getenv, OnChange: app.emitAgentStatus,
	})
	broker.owns = app.agents.Owns
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.broker.ctx = ctx
	a.agents.Start()
}

// shutdown stops the agent listener and cancels running agent operations (spec §16.5) before the app exits.
func (a *App) shutdown(context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a.agents.Shutdown(ctx)
}

// AgentStatus reports the agent listener, the endpoint, agent operations and recent agent activity.
func (a *App) AgentStatus() agentaccess.Status { return a.agents.Status() }

// AgentApply turns agent access on or off, moves its address or port, or changes the write switch.
func (a *App) AgentApply(change mcpserver.SettingsChange) (mcpserver.ApplyResult, error) {
	return a.agents.Apply(change)
}

// AgentNewToken replaces the access token and returns it once.
func (a *App) AgentNewToken() (string, error) { return a.agents.NewToken() }

// AgentSetup returns one host's configuration for the desktop's endpoint.
func (a *App) AgentSetup(agent string) (agentsetup.Setup, error) { return a.agents.Setup(agent) }

// AgentInstallSkill installs the skill for a host or into a directory the user picked.
func (a *App) AgentInstallSkill(request agentaccess.SkillRequest) (agentaccess.SkillReport, error) {
	return a.agents.InstallSkill(request)
}

// AgentCancel requests the cancellation of a running agent operation.
func (a *App) AgentCancel(operationID string) bool { return a.agents.Cancel(operationID) }

func (a *App) emitAgentStatus(status agentaccess.Status) {
	runtime.EventsEmit(a.ctx, "agent:status", status)
}

func (a *App) Doctor(request guiapi.OperationRequest) (app.DoctorReport, error) {
	return a.service.Doctor(a.ctx, request)
}

func (a *App) Verify(request guiapi.OperationRequest) (app.VerifyReport, error) {
	return a.service.Verify(a.ctx, request)
}

func (a *App) EnableSSH(request guiapi.OperationRequest) (app.EnableReport, error) {
	return a.service.EnableSSH(a.ctx, request)
}

func (a *App) PersistSoft(request guiapi.OperationRequest) (app.SoftReport, error) {
	return a.service.PersistSoft(a.ctx, request)
}

func (a *App) PersistRemove(request guiapi.OperationRequest) (app.SoftReport, error) {
	return a.service.PersistRemove(a.ctx, request)
}

func (a *App) RebootVerify(request guiapi.OperationRequest) (app.RebootReport, error) {
	return a.service.RebootVerify(a.ctx, request)
}

func (a *App) BackupCreate(request guiapi.OperationRequest) (app.BackupCreateReport, error) {
	return a.service.BackupCreate(a.ctx, request)
}

func (a *App) BackupVerify(request guiapi.OperationRequest) (app.BackupVerifyReport, error) {
	return a.service.BackupVerify(a.ctx, request)
}

func (a *App) BackupVerifyOnline(request guiapi.OperationRequest) (app.BackupVerifyReport, error) {
	return a.service.BackupVerifyOnline(a.ctx, request)
}

func (a *App) DeepPersist(request guiapi.OperationRequest) (app.DeepReport, error) {
	return a.service.DeepPersist(a.ctx, request)
}

// Catalog lists the registered device models and their exact firmware profiles.
// GuidePlan detects the router and returns the guided plan without changing it.
func (a *App) GuidePlan(request guiapi.GuideRequest) (app.GuidePlanReport, error) {
	return a.service.GuidePlan(a.ctx, request)
}

// GuideApply runs a confirmed guided plan.
func (a *App) GuideApply(request guiapi.GuideRequest) (app.GuideApplyReport, error) {
	return a.service.GuideApply(a.ctx, request)
}

// AcknowledgeRecovery records that the router was checked after an uncertain change.
func (a *App) AcknowledgeRecovery(request guiapi.OperationRequest) (app.RecoveryReport, error) {
	return a.service.AcknowledgeRecovery(a.ctx, request)
}

// DeepStatus reads the deep-persistence page's state without changing the router.
func (a *App) DeepStatus(request guiapi.OperationRequest) (app.DeepStatusReport, error) {
	return a.service.DeepStatus(a.ctx, request)
}

// CredentialStatus lists saved passwords without their values.
func (a *App) CredentialStatus() (app.CredentialReport, error) { return a.service.CredentialStatus() }

// ForgetCredential forgets the saved passwords matching the filter.
func (a *App) ForgetCredential(filter guiapi.CredentialFilter) (app.CredentialReport, error) {
	return a.service.ForgetCredential(filter)
}

// ForgetAllCredentials forgets every saved password.
func (a *App) ForgetAllCredentials() (app.CredentialReport, error) {
	return a.service.ForgetAllCredentials()
}

func (a *App) Catalog() []device.CatalogEntry { return a.service.Catalog() }

func (a *App) Cancel(operationID string) bool              { return a.service.Cancel(operationID) }
func (a *App) ListReports() []guiapi.HistoryEntry          { return a.service.ListReports() }
func (a *App) NewOperationID() (string, error)             { return guiapi.NewOperationID() }
func (a *App) ResolveHostKey(promptID string, accept bool) { a.broker.resolve(promptID, accept) }
func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select a directory"})
}

// fanout sends progress to every emitter: the window, and the MCP server's operations.
type fanout []guiapi.EventEmitter

func (f fanout) Emit(operationID string, event app.Event) {
	for _, emitter := range f {
		emitter.Emit(operationID, event)
	}
}

type runtimeEmitter struct{ app *App }

func (e runtimeEmitter) Emit(operationID string, event app.Event) {
	runtime.EventsEmit(e.app.ctx, "operation:progress", struct {
		OperationID string    `json:"operation_id"`
		Event       app.Event `json:"event"`
	}{OperationID: operationID, Event: event})
}

// HostKeyPrompt is public key evidence shown in the desktop confirmation dialog.
type HostKeyPrompt struct {
	PromptID    string `json:"prompt_id"`
	OperationID string `json:"operation_id"`
	Address     string `json:"address"`
	Algorithm   string `json:"algorithm"`
	Fingerprint string `json:"fingerprint"`
	// Source is "agent" when an agent started the operation.
	Source string `json:"source,omitempty"`
}

type promptBroker struct {
	ctx     context.Context
	mu      sync.Mutex
	pending map[string]chan bool
	// owns reports whether an operation ID belongs to an agent operation.
	owns func(string) bool
}

func (b *promptBroker) Confirm(ctx context.Context, operationID string, prompt hostkey.Prompt) bool {
	promptID, err := guiapi.NewOperationID()
	if err != nil {
		return false
	}
	channel := make(chan bool, 1)
	b.mu.Lock()
	b.pending[promptID] = channel
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, promptID)
		b.mu.Unlock()
	}()
	source := ""
	if b.owns != nil && b.owns(operationID) {
		source = guiapi.SourceAgent
	}
	runtime.EventsEmit(b.ctx, "hostkey:confirm", HostKeyPrompt{
		PromptID: promptID, OperationID: operationID,
		Address: prompt.Address, Algorithm: prompt.Algorithm, Fingerprint: prompt.Fingerprint, Source: source,
	})
	select {
	case accept := <-channel:
		return accept
	case <-ctx.Done():
	case <-time.After(2 * time.Minute):
	}
	// Canceled, ended or unanswered: the key is refused and the window withdraws the prompt, since answering it
	// now would change nothing.
	runtime.EventsEmit(b.ctx, "hostkey:expired", promptID)
	return false
}

func (b *promptBroker) resolve(promptID string, accept bool) {
	b.mu.Lock()
	channel := b.pending[promptID]
	b.mu.Unlock()
	if channel != nil {
		select {
		case channel <- accept:
		default:
		}
	}
}
