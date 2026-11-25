package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
)

// App exposes typed desktop operations to Wails.
type App struct {
	ctx     context.Context
	service *guiapi.Service
	broker  *promptBroker
}

// NewApp constructs the desktop binding around the shared application service.
func NewApp(application guiapi.Application) *App {
	broker := &promptBroker{pending: make(map[string]chan bool)}
	app := &App{broker: broker}
	app.service = guiapi.New(application, runtimeEmitter{app: app}, broker)
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.broker.ctx = ctx
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
func (a *App) Cancel(operationID string) bool              { return a.service.Cancel(operationID) }
func (a *App) ListReports() []guiapi.HistoryEntry          { return a.service.ListReports() }
func (a *App) NewOperationID() (string, error)             { return guiapi.NewOperationID() }
func (a *App) ResolveHostKey(promptID string, accept bool) { a.broker.resolve(promptID, accept) }
func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select a directory"})
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
}

type promptBroker struct {
	ctx     context.Context
	mu      sync.Mutex
	pending map[string]chan bool
}

func (b *promptBroker) Confirm(operationID string, prompt hostkey.Prompt) bool {
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
	runtime.EventsEmit(b.ctx, "hostkey:confirm", HostKeyPrompt{
		PromptID: promptID, OperationID: operationID,
		Address: prompt.Address, Algorithm: prompt.Algorithm, Fingerprint: prompt.Fingerprint,
	})
	select {
	case accept := <-channel:
		return accept
	case <-time.After(2 * time.Minute):
		return false
	}
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
