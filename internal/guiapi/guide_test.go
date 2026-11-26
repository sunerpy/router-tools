package guiapi_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
)

func TestGuidePlanConvertsInputsAndKeepsNoHistory(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{planReport: app.GuidePlanReport{Status: app.StatusSucceeded, Ready: true}}
	events := &eventRecorder{}
	service := guiapi.New(application, events, &confirmer{})
	report, err := service.GuidePlan(t.Context(), guiapi.GuideRequest{
		OperationID: "plan-1", Target: device.DefaultTarget(), ProfileID: "xiaomi-rd08-1.1.96",
		Goals: []string{app.GoalSSH, app.GoalSoft}, RootPassword: "root-secret", ResetRootPassword: true,
	})
	if err != nil || !report.Ready {
		t.Fatalf("GuidePlan() = %+v, %v", report, err)
	}
	request := application.guideRequest
	if request.RootPassword.Expose() != "root-secret" || !request.ResetRootPassword || request.ProfileID != "xiaomi-rd08-1.1.96" ||
		strings.Join(request.Goals, ",") != "ssh,soft" || request.HostKeyPolicy.Confirm == nil {
		t.Fatalf("plan request = %+v", request)
	}
	if len(service.ListReports()) != 0 {
		t.Fatal("a plan is detection only and must not enter the run history")
	}
	if len(events.ids) != 1 || events.ids[0] != "plan-1" {
		t.Fatalf("events = %v, want the plan's progress under its operation ID", events.ids)
	}

	// A blocked plan resolves with its report so the window can show why.
	application.planReport = app.GuidePlanReport{Status: app.StatusFailed, Error: &device.Error{Code: device.CodeRecoveryReviewRequired}}
	application.planErr = application.planReport.Error
	report, err = service.GuidePlan(t.Context(), guiapi.GuideRequest{OperationID: "plan-2", Target: device.DefaultTarget()})
	if err != nil || report.Error == nil || report.Error.Code != device.CodeRecoveryReviewRequired {
		t.Fatalf("blocked plan = %+v, %v; want the report with a nil binding error", report, err)
	}
	if application.guideRequest.RootPassword.Expose() != "" {
		t.Fatal("an empty root password must stay absent, not become a value")
	}
}

func TestGuideApplyPassesTheConfirmationAndRecordsTheRun(t *testing.T) {
	t.Parallel()

	stepErr := &device.Error{Code: device.CodeBackupIncomplete, Message: "short read"}
	application := &fakeApplication{
		applyReport: app.GuideApplyReport{Status: app.StatusPartial, Error: stepErr},
		applyErr:    stepErr,
	}
	events := &eventRecorder{}
	service := guiapi.New(application, events, nil)
	confirmation := &app.GuideConfirmation{SchemaVersion: 1, Target: device.DefaultTarget(), Goals: []string{app.GoalSSH}}
	report, err := service.GuideApply(t.Context(), guiapi.GuideRequest{
		OperationID: "run-1", Target: device.DefaultTarget(), Confirmation: confirmation, PlanDigest: "digest",
		Confirmed: true, AdminPassword: "admin-secret", RootPassword: "root-secret", ToolVersion: "v0.1.0",
	})
	if err != nil || report.Status != app.StatusPartial {
		t.Fatalf("GuideApply() = %+v, %v; want the partial report with a nil binding error", report, err)
	}
	request := application.applyRequest
	if request.PlanDigest != "digest" || !request.Confirmed || request.Confirmation.SchemaVersion != 1 ||
		request.AdminPassword.Expose() != "admin-secret" || request.RootPassword.Expose() != "root-secret" || request.ToolVersion != "v0.1.0" {
		t.Fatalf("apply request = %+v", request)
	}
	history := service.ListReports()
	if len(history) != 1 || history[0].Operation != "guide apply" || history[0].OperationID != "run-1" {
		t.Fatalf("history = %+v, want one guided run", history)
	}
	data, _ := json.Marshal(history)
	if strings.Contains(string(data), "admin-secret") || strings.Contains(string(data), "root-secret") {
		t.Fatal("history contains a password")
	}
}

func TestGuideApplyRejectsMissingConfirmationBeforeTheApplication(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	service := guiapi.New(application, nil, nil)
	if _, err := service.GuideApply(t.Context(), guiapi.GuideRequest{OperationID: "run-1", Confirmed: true, RootPassword: "root"}); err == nil {
		t.Fatal("GuideApply() without a confirmation succeeded")
	}
	if application.applyRequest.PlanDigest != "" || application.applyRequest.Confirmed || len(service.ListReports()) != 0 {
		t.Fatal("the application ran or a history entry was recorded")
	}
}

func TestAcknowledgeRecoveryRecordsTheAcknowledgement(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	service := guiapi.New(application, nil, nil)
	target := device.Target{Host: "192.168.31.2", WebPort: 80, SSHPort: 22, TelnetPort: 23}
	report, err := service.AcknowledgeRecovery(t.Context(), guiapi.OperationRequest{OperationID: "ack-1", Target: target})
	if err != nil || !report.Acknowledged || application.ackTarget != target {
		t.Fatalf("AcknowledgeRecovery() = %+v, %v; target %+v", report, err, application.ackTarget)
	}
	if history := service.ListReports(); len(history) != 1 || history[0].Operation != "recovery ack" {
		t.Fatalf("history = %+v", history)
	}
}
