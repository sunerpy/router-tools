package rd08v1196

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

type fakeWebSession struct {
	commands        []string
	added           []string
	deleted         []string
	failAddAt       int
	addErr          error
	deleteErr       error
	deleteErrAt     int
	deleteErrRepeat bool
	addCalls        int
	deleteCalls     int
}

func (s *fakeWebSession) AddMACRule(_ context.Context, mac, command string) error {
	s.added = append(s.added, mac)
	s.commands = append(s.commands, command)
	index := s.addCalls
	s.addCalls++
	if s.addErr != nil && (s.failAddAt == index || s.failAddAt == -1) {
		return s.addErr
	}
	return nil
}

func (s *fakeWebSession) DeleteMACRule(_ context.Context, mac string) error {
	s.deleted = append(s.deleted, mac)
	index := s.deleteCalls
	s.deleteCalls++
	if s.deleteErr != nil && (s.deleteErrAt == index || s.deleteErrAt == -1 || (s.deleteErrRepeat && index == s.deleteErrAt+1)) {
		return s.deleteErr
	}
	return nil
}
func (s *fakeWebSession) Close() {}

func TestEnableSSHFixedSequenceAndCleanup(t *testing.T) {
	t.Parallel()

	web := &fakeWebSession{failAddAt: -1, deleteErrAt: -1}
	prober, calls := newEnableTestProber(web, false)
	password, _ := secret.NewPassword("admin")
	rootPassword, _ := secret.NewPassword("root-pass")
	result, err := prober.EnableSSH(t.Context(), device.DefaultTarget(), password, rootPassword, hostkey.Policy{})
	if err != nil {
		t.Fatalf("EnableSSH() error = %v", err)
	}
	if len(web.commands) != 3 {
		t.Fatalf("commands = %v", web.commands)
	}
	if web.commands[0] != "nvram set ssh_en=1 && nvram set telnet_en=1 && nvram commit" {
		t.Fatalf("step 1 = %q", web.commands[0])
	}
	if web.commands[1] != `sed -i "s/channel=.*/channel=\"debug\"/g" /etc/init.d/dropbear && /etc/init.d/dropbear start` {
		t.Fatalf("step 2 = %q", web.commands[1])
	}
	plain := "root-pass\nroot-pass\n"
	wantStep3 := "printf '%s' '" + base64.StdEncoding.EncodeToString([]byte(plain)) + "' | base64 -d | passwd root"
	if web.commands[2] != wantStep3 {
		t.Fatalf("step 3 = %q, want %q", web.commands[2], wantStep3)
	}
	if len(result.AttemptedMACs) != 3 || len(web.deleted) != 3 || !result.SSHVerified || result.Status != app.StatusSucceeded {
		t.Fatalf("result = %+v, deleted = %v", result, web.deleted)
	}
	if *calls < 2 {
		t.Fatalf("SSH calls = %d, want precheck and final verification", *calls)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "root-pass") || strings.Contains(string(encoded), base64.StdEncoding.EncodeToString([]byte(plain))) {
		t.Fatalf("result contains password material: %s", encoded)
	}
}

func TestEnableSSHFailureCleansOnlyAttemptedMACs(t *testing.T) {
	t.Parallel()

	web := &fakeWebSession{failAddAt: 1, addErr: &device.Error{Code: device.CodeRemoteAPIRejected}, deleteErrAt: -1}
	prober, _ := newEnableTestProber(web, false)
	password, _ := secret.NewPassword("admin")
	rootPassword, _ := secret.NewPassword("root")
	result, err := prober.EnableSSH(t.Context(), device.DefaultTarget(), password, rootPassword, hostkey.Policy{})
	assertEnableCode(t, err, device.CodeRemoteAPIRejected)
	if len(web.commands) != 2 || len(web.deleted) != 2 || result.AcceptedSteps != 1 || result.Status != app.StatusPartial {
		t.Fatalf("result = %+v, commands = %d, deleted = %d", result, len(web.commands), len(web.deleted))
	}
}

func TestEnableSSHDoesNotRetryUnknownAdd(t *testing.T) {
	t.Parallel()

	web := &fakeWebSession{failAddAt: 0, addErr: &device.Error{Code: device.CodeRemoteResultUnknown}, deleteErrAt: -1}
	prober, _ := newEnableTestProber(web, false)
	password, _ := secret.NewPassword("admin")
	rootPassword, _ := secret.NewPassword("root")
	result, err := prober.EnableSSH(t.Context(), device.DefaultTarget(), password, rootPassword, hostkey.Policy{})
	assertEnableCode(t, err, device.CodeRemoteResultUnknown)
	if web.addCalls != 1 || result.Status != app.StatusUnknown || len(web.deleted) != 1 {
		t.Fatalf("result = %+v, add calls = %d, deleted = %d", result, web.addCalls, len(web.deleted))
	}
}

func TestEnableSSHRootSuccessWithCleanupFailureIsPartial(t *testing.T) {
	t.Parallel()

	web := &fakeWebSession{failAddAt: -1, deleteErrAt: 1, deleteErrRepeat: true, deleteErr: &device.Error{Code: device.CodeRemoteResultUnknown}}
	prober, _ := newEnableTestProber(web, false)
	password, _ := secret.NewPassword("admin")
	rootPassword, _ := secret.NewPassword("root")
	result, err := prober.EnableSSH(t.Context(), device.DefaultTarget(), password, rootPassword, hostkey.Policy{})
	assertEnableCode(t, err, device.CodeMACCleanupFailed)
	if !result.SSHVerified || result.Status != app.StatusPartial || len(result.Cleanup) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if web.deleteCalls != 4 {
		t.Fatalf("delete calls = %d, want one retry for the failed delete", web.deleteCalls)
	}
}

func TestEnableSSHAlreadyVerifiedDoesNotWrite(t *testing.T) {
	t.Parallel()

	web := &fakeWebSession{failAddAt: -1, deleteErrAt: -1}
	prober, _ := newEnableTestProber(web, true)
	password, _ := secret.NewPassword("admin")
	rootPassword, _ := secret.NewPassword("root")
	result, err := prober.EnableSSH(t.Context(), device.DefaultTarget(), password, rootPassword, hostkey.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != app.OutcomeAlreadyVerified || web.addCalls != 0 || web.deleteCalls != 0 {
		t.Fatalf("result = %+v, writes = %d/%d", result, web.addCalls, web.deleteCalls)
	}
}

func newEnableTestProber(web *fakeWebSession, already bool) (*Prober, *int) {
	prober := NewProber(nil)
	prober.webLogin = func(context.Context, device.Target, secret.Password) (WebSession, error) { return web, nil }
	prober.macGenerator = func() ([3]string, error) {
		return [3]string{"0A:11:22:33:44:10", "0A:11:22:33:44:11", "0A:11:22:33:44:12"}, nil
	}
	prober.sleep = func(time.Duration) {}
	calls := new(int)
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		*calls++
		if *calls == 1 && !already {
			return nil, &device.Error{Code: device.CodeSSHAuthFailed}
		}
		return &rootFakeShell{uid: "0\n", bootID: "00000000-0000-0000-0000-000000000001\n"}, nil
	}
	return prober, calls
}

func assertEnableCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != want {
		t.Fatalf("error = %T %v, want %s", err, err, want)
	}
}
