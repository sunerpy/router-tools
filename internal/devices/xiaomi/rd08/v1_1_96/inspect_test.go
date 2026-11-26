package rd08v1196

import (
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
)

func TestInspectSoftReportsEveryStateReadOnly(t *testing.T) {
	t.Parallel()

	managed := map[string]string{
		"firewall.auto_ssh":         "include",
		"firewall.auto_ssh.type":    "script",
		"firewall.auto_ssh.path":    autoSSHPath,
		"firewall.auto_ssh.enabled": "1",
	}
	with := func(changes map[string]string) map[string]string {
		values := map[string]string{}
		for k, v := range managed {
			values[k] = v
		}
		for k, v := range changes {
			if v == "" {
				delete(values, k)
			} else {
				values[k] = v
			}
		}
		return values
	}
	cases := []struct {
		name       string
		uci        map[string]string
		fileExists bool
		hash       string
		executable bool
		want       app.SoftState
	}{
		{"not installed", map[string]string{}, false, "", false, app.SoftNotInstalled},
		{"installed and verified", managed, true, autoSSHHash, true, app.SoftInstalledVerified},
		{"file content drifted", managed, true, strings.Repeat("0", 64), true, app.SoftFileDrift},
		{"file not executable", managed, true, autoSSHHash, false, app.SoftModeDrift},
		{"managed path with other values", with(map[string]string{"firewall.auto_ssh.enabled": "0"}), true, autoSSHHash, true, app.SoftUCIDrift},
		{"foreign path", with(map[string]string{"firewall.auto_ssh.path": "/user/custom.sh"}), true, autoSSHHash, true, app.SoftConflict},
		{"partial foreign config", map[string]string{"firewall.auto_ssh": "include"}, false, "", false, app.SoftConflict},
		{"orphan file", map[string]string{}, true, autoSSHHash, true, app.SoftOrphanFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := &fakeCommandShell{handler: func(command string) commandFixture {
				switch {
				case strings.HasPrefix(command, "uci -q get "):
					if value, ok := tc.uci[strings.TrimPrefix(command, "uci -q get ")]; ok {
						return commandFixture{stdout: value + "\n"}
					}
					return commandFixture{code: 1}
				case command == "test -e "+autoSSHPath:
					if tc.fileExists {
						return commandFixture{}
					}
					return commandFixture{code: 1}
				case command == "test -x "+autoSSHPath:
					if tc.executable {
						return commandFixture{}
					}
					return commandFixture{code: 1}
				case command == "sh -n "+autoSSHPath:
					return commandFixture{}
				case command == "sha256sum "+autoSSHPath:
					return commandFixture{stdout: tc.hash + "  " + autoSSHPath + "\n"}
				}
				return commandFixture{code: 1}
			}}
			got, err := inspectSoft(t.Context(), remote)
			if err != nil {
				t.Fatalf("inspectSoft() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("inspectSoft() = %q, want %q", got, tc.want)
			}
			for _, command := range remote.commands {
				if !readOnlyInspection(command) {
					t.Fatalf("inspection ran a non-read-only command %q", command)
				}
			}
		})
	}
}

func TestInspectDeepFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		values map[string]string
		want   app.DeepFlags
	}{
		{"all set", map[string]string{"ssh_en": "1", "telnet_en": "1", "uart_en": "1", "boot_wait": "on"}, app.DeepFlagsAllSet},
		{"none", map[string]string{"ssh_en": "0", "telnet_en": "0", "uart_en": "0", "boot_wait": "off"}, app.DeepFlagsNone},
		{"mixed", map[string]string{"ssh_en": "1", "telnet_en": "1", "uart_en": "0", "boot_wait": "off"}, app.DeepFlagsMixed},
		{"unset keys", map[string]string{}, app.DeepFlagsNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := &fakeCommandShell{handler: func(command string) commandFixture {
				for _, source := range []string{"bdata get ", "nvram get "} {
					if strings.HasPrefix(command, source) {
						value, ok := tc.values[strings.TrimPrefix(command, source)]
						if !ok {
							return commandFixture{code: 1}
						}
						return commandFixture{stdout: value + "\n"}
					}
				}
				return commandFixture{code: 1}
			}}
			got, err := inspectDeepFlags(t.Context(), remote)
			if err != nil || got != tc.want {
				t.Fatalf("inspectDeepFlags() = %q, %v; want %q", got, err, tc.want)
			}
			for _, command := range remote.commands {
				if !strings.HasPrefix(command, "bdata get ") && !strings.HasPrefix(command, "nvram get ") {
					t.Fatalf("deep inspection ran %q", command)
				}
			}
		})
	}
}

// readOnlyInspection lists the fixed read-only commands soft inspection may run.
func readOnlyInspection(command string) bool {
	for _, prefix := range []string{"uci -q get firewall.auto_ssh", "test -e " + autoSSHPath, "test -x " + autoSSHPath, "sh -n " + autoSSHPath, "sha256sum " + autoSSHPath} {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

// Deep inspection also reports whether the crash partition the flow writes is listed and readable, using
// only the read-only commands the deep precheck already runs.
func TestInspectDeepReportsTheCrashPartition(t *testing.T) {
	t.Parallel()

	const withCrash = "dev:    size   erasesize  name\nmtd0: 00080000 00020000 \"boot\"\nmtd7: 00040000 00020000 \"crash\"\n"
	cases := []struct {
		name      string
		mtd       string
		readable  bool
		listed    bool
		canRead   bool
		wantError bool
	}{
		{name: "listed and readable", mtd: withCrash, readable: true, listed: true, canRead: true},
		{name: "listed but not readable", mtd: withCrash, listed: true},
		{name: "missing", mtd: "dev:    size   erasesize  name\nmtd0: 00080000 00020000 \"boot\"\n"},
		{name: "unparsable layout", mtd: "garbage\n", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			remote := &fakeCommandShell{handler: func(command string) commandFixture {
				switch {
				case command == "cat /proc/mtd":
					return commandFixture{stdout: tc.mtd}
				case command == "test -r /dev/mtd7":
					if tc.readable {
						return commandFixture{}
					}
					return commandFixture{code: 1}
				case strings.HasPrefix(command, "bdata get "), strings.HasPrefix(command, "nvram get "):
					return commandFixture{code: 1}
				}
				return commandFixture{code: 127}
			}}
			got, err := inspectDeep(t.Context(), remote)
			if tc.wantError {
				if err == nil {
					t.Fatalf("inspectDeep() = %+v, want an error for an unparsable layout", got)
				}
				return
			}
			if err != nil || got.Flags != app.DeepFlagsNone || got.CrashPartition != tc.listed || got.CrashReadable != tc.canRead {
				t.Fatalf("inspectDeep() = %+v, %v", got, err)
			}
			for _, command := range remote.commands {
				if command != "cat /proc/mtd" && !strings.HasPrefix(command, "test -r /dev/mtd") &&
					!strings.HasPrefix(command, "bdata get ") && !strings.HasPrefix(command, "nvram get ") {
					t.Fatalf("deep inspection ran %q", command)
				}
			}
		})
	}
}
