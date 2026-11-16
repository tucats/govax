package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestHostFileNeedsSYSPRV runs the key end to end (docs/planning/PHASE-97
// - host file access.md, subtask 2): with vax.host.sysprv on, TYPE of a
// host file, by its path and by /HOST, is refused to PRIV94, who doesn't
// hold SYSPRV, and allowed once it turns SYSPRV on; with the key off, as
// before, it is allowed without it.
func TestHostFileNeedsSYSPRV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")

	if err := os.WriteFile(path, []byte("host text\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, on := range []bool{true, false} {
		name := "off"
		if on {
			name = "on"
		}

		t.Run(name, func(t *testing.T) {
			setSetting(t, corevms.HostSYSPRVSetting, map[bool]string{true: "true", false: "false"}[on])

			c, buf := scheduledConsole(t, longQuantum, brbSelf)
			d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
			dispatch := func(line string) string {
				t.Helper()
				buf.Reset()

				if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
					buf.WriteString("%" + err.Error() + "\n")
				}

				return strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r", ""))
			}

			priv94Account.become(c)

			for _, line := range []string{`TYPE "` + path + `"`, `TYPE/HOST "` + path + `"`} {
				got := dispatch(line)
				t.Logf("%s:\n%s", line, got)

				if on && (strings.Contains(got, "host text") || !strings.Contains(got, "-RMS-E-PRV")) {
					t.Errorf("%s without SYSPRV:\n%s", line, got)
				}

				if !on && got != "host text" {
					t.Errorf("%s, key off:\n%s", line, got)
				}
			}

			if got := dispatch("SET PROCESS/PRIVILEGES=SYSPRV"); got != "" {
				t.Fatalf("SET PROCESS/PRIVILEGES=SYSPRV: %s", got)
			}

			if got := dispatch(`TYPE "` + path + `"`); got != "host text" {
				t.Errorf("with SYSPRV:\n%s", got)
			}
		})
	}
}
