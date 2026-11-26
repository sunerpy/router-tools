package rd08v1196

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

type deepJournalFake struct {
	stages      []string
	transitions int
}

func (j *deepJournalFake) Stage(stage string) error        { j.stages = append(j.stages, stage); return nil }
func (j *deepJournalFake) Transition(string, string) error { j.transitions++; return nil }
func (j *deepJournalFake) Path() string                    { return "journal.json" }

type deepFixture struct {
	bootIDs   []string
	shells    []*deepShell
	commands  []string
	values    map[string]string
	mtdText   string
	crashErr  error
	eraseErr  error
	onlineErr error
	badFlag   string
	badValue  string
}

type deepShell struct {
	fixture *deepFixture
	bootID  string
}

func (s *deepShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	s.fixture.commands = append(s.fixture.commands, command)
	result := shell.CommandResult{Started: true}
	switch command {
	case "id -u":
		result.Stdout = []byte("0\n")
	case "cat /proc/sys/kernel/random/boot_id":
		result.Stdout = []byte(s.bootID + "\n")
	case "cat /proc/mtd":
		result.Stdout = []byte(s.fixture.mtdText)
	case "zz=$(dd if=/dev/zero bs=1 count=2 2>/dev/null) ; printf '\\xA5\\x5A%c%c' $zz $zz | mtd write - crash":
		if s.fixture.crashErr != nil {
			return result, s.fixture.crashErr
		}
	case "nvram get ssh_en", "nvram get telnet_en", "nvram get uart_en":
		result.Stdout = []byte("1\n")
	case "nvram get boot_wait":
		result.Stdout = []byte("on\n")
	case "bdata get ssh_en", "bdata get telnet_en", "bdata get uart_en":
		result.Stdout = []byte("1\n")
	case "bdata get boot_wait":
		result.Stdout = []byte("on\n")
	}
	if command == s.fixture.badFlag {
		result.Stdout = []byte(s.fixture.badValue + "\n")
	}
	if command == "mtd erase crash" && s.fixture.eraseErr != nil {
		return result, s.fixture.eraseErr
	}
	if strings.HasPrefix(command, "uci -q get ") {
		key := strings.TrimPrefix(command, "uci -q get ")
		value, ok := s.fixture.values[key]
		if !ok {
			result.ExitCode = 1
			return result, &shell.RemoteCommandError{Result: result}
		}
		result.Stdout = []byte(value + "\n")
	} else if strings.HasPrefix(command, "uci set ") {
		for _, segment := range strings.Split(command, " && ") {
			assignment := strings.TrimPrefix(segment, "uci set ")
			parts := strings.SplitN(assignment, "=", 2)
			if len(parts) == 2 {
				s.fixture.values[parts[0]] = strings.Trim(parts[1], "'")
			}
		}
	} else if strings.HasPrefix(command, "sha256sum ") {
		result.Stdout = []byte(autoSSHHash + "  file\n")
	}
	return result, nil
}
func (s *deepShell) Close() error { return nil }

func TestDeepPersistSuccessExactlyThreeReboots(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	prober := newDeepProber(fixture)
	journal := &deepJournalFake{}
	result, err := prober.DeepPersist(t.Context(), deepParams(fixture, journal))
	if err != nil {
		t.Fatalf("DeepPersist() error = %v", err)
	}
	if result.Status != app.StatusSucceeded || !result.FinalSSHVerified || !result.SoftVerified || len(result.Transitions) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if countCommand(fixture.commands, "reboot") != 3 || countCommand(fixture.commands, "mtd write - crash") != 1 || countCommand(fixture.commands, "mtd erase crash") != 1 {
		t.Fatalf("deep command counts are wrong: %v", fixture.commands)
	}
	if journal.transitions != 3 || journal.stages[len(journal.stages)-1] != "SUCCEEDED" {
		t.Fatalf("journal = %+v", journal)
	}
}

func TestDeepPersistSecondRebootFailureNeverErasesCrash(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.onlineErr = &device.Error{Code: device.CodeSSHUnreachable}
	prober := newDeepProber(fixture)
	calls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		calls++
		if calls <= 2 {
			return fixture.shells[calls-1], nil
		}
		return nil, fixture.onlineErr
	}
	journal := &deepJournalFake{}
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, journal))
	if err == nil {
		t.Fatal("DeepPersist() error = nil")
	}
	for _, command := range fixture.commands {
		if command == "mtd erase crash" {
			t.Fatalf("crash was erased after second reboot failure: %v", fixture.commands)
		}
	}
}

func TestDeepPersistMissingCrashHasNoWrites(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.mtdText = "dev: size erasesize name\nmtd0: 00000004 00020000 \"boot\"\n"
	prober := newDeepProber(fixture)
	result, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() missing crash error = nil")
	}
	if result.WriteStarted {
		t.Fatal("a failure before the crash write reports a device write")
	}
	for _, command := range fixture.commands {
		if strings.Contains(command, "mtd write") || strings.HasPrefix(command, "nvram set") || strings.HasPrefix(command, "bdata set") || command == "reboot" {
			t.Fatalf("write occurred without crash: %q", command)
		}
	}
}

func TestDeepPersistUnknownCrashWriteIsNotRetried(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.crashErr = &device.Error{Code: device.CodeRemoteResultUnknown}
	prober := newDeepProber(fixture)
	result, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil || result.Status != app.StatusUnknown || !result.WriteStarted {
		t.Fatalf("result = %+v, error = %v; want unknown with write evidence", result, err)
	}
	if countCommand(fixture.commands, "mtd write - crash") != 1 || countCommand(fixture.commands, "reboot") != 0 {
		t.Fatalf("unknown crash write was retried or rebooted: %v", fixture.commands)
	}
}

func TestDeepPersistRejectsInvalidBootIDBeforeWrites(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.shells[0].bootID = "invalid"
	prober := newDeepProber(fixture)
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() invalid boot_id error = nil")
	}
	if countCommand(fixture.commands, "mtd write") != 0 || countCommand(fixture.commands, "nvram set") != 0 || countCommand(fixture.commands, "reboot") != 0 {
		t.Fatalf("invalid boot_id caused writes: %v", fixture.commands)
	}
}

func TestDeepPersistFirstDownFailureStopsBeforeFlags(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	prober := newDeepProber(fixture)
	prober.probePort = func(context.Context, string, int) device.PortState { return device.PortOpen }
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() no-down error = nil")
	}
	if countCommand(fixture.commands, "nvram set") != 0 || countCommand(fixture.commands, "bdata set") != 0 {
		t.Fatalf("flags were written without observed shutdown: %v", fixture.commands)
	}
}

func TestDeepPersistUnchangedBootIDStopsBeforeFlags(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.shells[1].bootID = fixture.bootIDs[0]
	prober := newDeepProber(fixture)
	calls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		calls++
		if calls == 1 {
			return fixture.shells[0], nil
		}
		return &deepShell{fixture: fixture, bootID: fixture.bootIDs[0]}, nil
	}
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() unchanged boot_id error = nil")
	}
	if countCommand(fixture.commands, "nvram set") != 0 || countCommand(fixture.commands, "bdata set") != 0 {
		t.Fatalf("flags were written with unchanged boot_id: %v", fixture.commands)
	}
}

func TestDeepPersistBadFlagPreventsSecondReboot(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.badFlag = "bdata get ssh_en"
	fixture.badValue = "0"
	prober := newDeepProber(fixture)
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() bad flag error = nil")
	}
	if countCommand(fixture.commands, "reboot") != 1 || countCommand(fixture.commands, "mtd erase crash") != 0 {
		t.Fatalf("bad flags did not stop sequence: %v", fixture.commands)
	}
}

func TestDeepPersistEraseFailurePreventsThirdReboot(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	fixture.eraseErr = &device.Error{Code: device.CodeRemoteCommandFailed}
	prober := newDeepProber(fixture)
	_, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil {
		t.Fatal("DeepPersist() erase error = nil")
	}
	if countCommand(fixture.commands, "reboot") != 2 {
		t.Fatalf("erase failure allowed third reboot: %v", fixture.commands)
	}
}

func TestDeepPersistFinalSSHFailureIsNotSuccess(t *testing.T) {
	t.Parallel()

	fixture := newDeepFixture()
	prober := newDeepProber(fixture)
	calls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		calls++
		if calls >= 5 {
			return nil, &device.Error{Code: device.CodeSSHAuthFailed}
		}
		return fixture.shells[calls-1], nil
	}
	result, err := prober.DeepPersist(t.Context(), deepParams(fixture, &deepJournalFake{}))
	if err == nil || result.Status == app.StatusSucceeded || result.FinalSSHVerified {
		t.Fatalf("final SSH failure was reported as success: %+v, %v", result, err)
	}
}

func newDeepFixture() *deepFixture {
	fixture := &deepFixture{
		bootIDs: []string{
			"00000000-0000-0000-0000-000000000001",
			"00000000-0000-0000-0000-000000000002",
			"00000000-0000-0000-0000-000000000003",
			"00000000-0000-0000-0000-000000000004",
			"00000000-0000-0000-0000-000000000004",
		},
		values:  map[string]string{},
		mtdText: "dev: size erasesize name\nmtd0: 00000004 00020000 \"boot\"\nmtd1: 00000002 00020000 \"crash\"\n",
	}
	for _, bootID := range fixture.bootIDs {
		fixture.shells = append(fixture.shells, &deepShell{fixture: fixture, bootID: bootID})
	}
	return fixture
}

func newDeepProber(fixture *deepFixture) *Prober {
	prober := NewProber(func(context.Context, string, int) device.PortState { return device.PortClosed })
	prober.sleep = func(time.Duration) {}
	prober.onlineTimeout = 5 * time.Millisecond
	prober.telnetDial = func(context.Context, device.Target, secret.Password) (shell.Shell, error) {
		return nil, &device.Error{Code: device.CodeTelnetUnreachable}
	}
	calls := 0
	prober.sshDial = func(context.Context, device.Target, secret.Password, hostkey.Policy) (shell.Shell, error) {
		index := calls
		calls++
		if index >= len(fixture.shells) {
			index = len(fixture.shells) - 1
		}
		return fixture.shells[index], nil
	}
	return prober
}

func deepParams(fixture *deepFixture, journal app.DeepJournal) app.DeepParams {
	observation := device.Observation{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96"}
	partitions := []backup.Partition{
		{Index: 0, Name: "boot", Size: 4, EraseSize: 0x20000},
		{Index: 1, Name: "crash", Size: 2, EraseSize: 0x20000},
	}
	if !strings.Contains(fixture.mtdText, "crash") {
		partitions = partitions[:1]
	}
	return app.DeepParams{
		Target:       device.DefaultTarget(),
		RootPassword: mustDeepPassword(),
		BackupDir:    "/backup",
		Manifest: backup.Manifest{
			Device:     backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96", Profile: profileID},
			BootID:     fixture.bootIDs[0],
			Partitions: partitions,
		},
		Observation: observation,
		ProfileID:   profileID,
		Journal:     journal,
	}
}

func mustDeepPassword() secret.Password {
	password, _ := secret.NewPassword("root")
	return password
}

func countCommand(commands []string, fragment string) int {
	count := 0
	for _, command := range commands {
		if strings.Contains(command, fragment) {
			count++
		}
	}
	return count
}
