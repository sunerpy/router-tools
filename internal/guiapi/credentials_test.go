package guiapi_test

import (
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
)

func TestSavedPasswordChoicesReachTheApplication(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{planReport: app.GuidePlanReport{Status: app.StatusSucceeded}}
	service := guiapi.New(application, nil, nil)
	if _, err := service.Verify(t.Context(), guiapi.OperationRequest{
		OperationID: "verify-1", Target: device.DefaultTarget(), UseSavedPassword: true, RememberPassword: true,
	}); err != nil {
		t.Fatal(err)
	}
	if got := application.verifyRequest.Credentials; !got.UseSaved || !got.Remember {
		t.Fatalf("verify credentials = %+v", got)
	}
	if _, err := service.GuidePlan(t.Context(), guiapi.GuideRequest{OperationID: "plan-1", Target: device.DefaultTarget(), UseSavedPassword: true}); err != nil {
		t.Fatal(err)
	}
	if got := application.guideRequest.Credentials; !got.UseSaved || got.Remember {
		t.Fatalf("plan credentials = %+v", got)
	}
}

func TestCredentialBindingsPassMetadataOnly(t *testing.T) {
	t.Parallel()

	entry := credentials.Entry{Key: credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22), SavedAt: time.Date(2026, 11, 26, 5, 0, 0, 0, time.UTC)}
	application := &fakeApplication{credentialEntries: []credentials.Entry{entry}}
	service := guiapi.New(application, nil, nil)
	status, err := service.CredentialStatus()
	if err != nil || len(status.Entries) != 1 || status.Entries[0].Key != entry.Key {
		t.Fatalf("CredentialStatus() = %+v, %v", status, err)
	}
	if _, err := service.ForgetCredential(guiapi.CredentialFilter{Kind: "root", Host: "192.168.31.1", Port: 22}); err != nil {
		t.Fatal(err)
	}
	if want := (credentials.Filter{Kind: credentials.KindRoot, Host: "192.168.31.1", Port: 22}); application.forgetFilter != want {
		t.Fatalf("filter = %+v, want %+v", application.forgetFilter, want)
	}
	if _, err := service.ForgetAllCredentials(); err != nil || !application.forgotAll {
		t.Fatalf("ForgetAllCredentials() = %v, forgot all %v", err, application.forgotAll)
	}
	if len(service.ListReports()) != 0 {
		t.Fatal("credential management is not a device operation and must stay out of the run history")
	}
}

func TestDeepStatusPassesThroughWithoutHistory(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	service := guiapi.New(application, nil, nil)
	report, err := service.DeepStatus(t.Context(), guiapi.OperationRequest{
		OperationID: "deep-status", Target: device.DefaultTarget(), RootPassword: "root-secret", UseSavedPassword: true,
	})
	if err != nil || !report.Ready {
		t.Fatalf("DeepStatus() = %+v, %v", report, err)
	}
	request := application.deepStatusRequest
	if request.RootPassword.Expose() != "root-secret" || !request.Credentials.UseSaved {
		t.Fatalf("request = %+v", request)
	}
	if len(service.ListReports()) != 0 {
		t.Fatal("a read-only status check must stay out of the run history")
	}
}
