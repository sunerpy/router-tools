package main

import (
	"context"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/wiring"
)

// recordingApplication records the requests the window's bindings pass on. Only the methods this test calls
// are implemented; the embedded interface is nil, so any other call fails the test with a panic.
type recordingApplication struct {
	guiapi.Application
	backup app.BackupCreateRequest
	guide  app.GuideApplyRequest
}

func (r *recordingApplication) BackupCreate(_ context.Context, request app.BackupCreateRequest, _ app.EventSink) (app.BackupCreateReport, error) {
	r.backup = request
	return app.BackupCreateReport{Status: app.StatusSucceeded}, nil
}

func (r *recordingApplication) GuideApply(_ context.Context, request app.GuideApplyRequest, _ app.EventSink) (app.GuideApplyReport, error) {
	r.guide = request
	return app.GuideApplyReport{Status: app.StatusSucceeded}, nil
}

// The window never decides which version a backup manifest records: the binding stamps this binary's own
// version over whatever the request carried (the manifest's tool_version, spec §14.5).
func TestWindowRequestsRecordThisBinarysVersion(t *testing.T) {
	application := &recordingApplication{}
	desktop := newApp(application, wiring.Paths{MCP: t.TempDir()}, "v9.8.7")
	desktop.ctx = context.Background()
	target := device.Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23}

	for _, sent := range []string{"", "dev"} {
		backupID, err := guiapi.NewOperationID()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := desktop.BackupCreate(guiapi.OperationRequest{OperationID: backupID, Target: target, ToolVersion: sent}); err != nil {
			t.Fatalf("backup with tool version %q: %v", sent, err)
		}
		if application.backup.ToolVersion != "v9.8.7" {
			t.Fatalf("backup with tool version %q recorded %q", sent, application.backup.ToolVersion)
		}

		guideID, err := guiapi.NewOperationID()
		if err != nil {
			t.Fatal(err)
		}
		request := guiapi.GuideRequest{OperationID: guideID, Confirmation: &app.GuideConfirmation{}, Confirmed: true, ToolVersion: sent}
		if _, err := desktop.GuideApply(request); err != nil {
			t.Fatalf("guided run with tool version %q: %v", sent, err)
		}
		if application.guide.ToolVersion != "v9.8.7" {
			t.Fatalf("guided run with tool version %q recorded %q", sent, application.guide.ToolVersion)
		}
	}
}
