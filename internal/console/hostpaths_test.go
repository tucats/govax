package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// hostNOPRIV is the message of a command with no protection refusal of
// its own (docs/planning/PHASE-97 - host file access.md, Decision 5).
const hostNOPRIV = "%SYSTEM-F-NOPRIV, insufficient privilege or object protection violation"

// TestHostPathsNeedSYSPRV runs the host paths that don't go through
// rms.Session.Locate (the phase doc's A.3, subtask 4) three ways, as
// TestHostFileCommands runs Locate's: with vax.host.sysprv on, as PRIV94
// without SYSPRV, each refused with SS$_NOPRIV and nothing made on the
// host; with SYSPRV turned on, and with the key off, each allowed.
// PRIV94 turns on SYSNAM and VOLPRO first, so MOUNT and
// INITIALIZE/CONTAINER of an existing container are refused for SYSPRV
// alone. EXE$OPEN is corevms's TestExeOpenHostAccess.
func TestHostPathsNeedSYSPRV(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}

	for _, pass := range []struct {
		name        string
		key, sysprv bool
	}{
		{"on", true, false},
		{"on-sysprv", true, true},
		{"off", false, false},
	} {
		t.Run(pass.name, func(t *testing.T) {
			host, dispatch := hostCommandConsole(t, pass.key, true)
			refused := pass.key && !pass.sysprv
			h := func(name string) string { return `"` + filepath.Join(host, name) + `"` }

			// A container to mount, made while SYSPRV is on.
			for _, line := range []string{
				"SET PROCESS/PRIVILEGES=(SYSNAM,VOLPRO)",
				"INITIALIZE/CONTAINER " + h("old.dsk") + " /DEVICE=RD51 OLD97",
			} {
				if out := dispatch(line); out != "" {
					t.Fatalf("setup: %s:\n%s", line, out)
				}
			}

			if !pass.sysprv {
				if out := dispatch("SET PROCESS/PRIVILEGES=NOSYSPRV"); out != "" {
					t.Fatalf("SET PROCESS/PRIVILEGES=NOSYSPRV: %s", out)
				}
			}

			before := hostFiles(t, host)

			for _, c := range []struct {
				line, allowed string // allowed: a line the command shows, "" for none
			}{
				{"INITIALIZE/CONTAINER " + h("new.dsk") + " /DEVICE=RD51 NEW97", ""},
				{"INITIALIZE/CONTAINER " + h("old.dsk") + " /DEVICE=RD51 OLD97", ""},
				{"MOUNT/NOLOG DUA2 " + h("old.dsk"), ""},
				{"SPAWN/HOST echo from-host", "from-host"},
			} {
				got := dispatch(c.line)
				t.Logf("%s:\n%s", c.line, got)

				if refused {
					if got != hostNOPRIV {
						t.Errorf("%s:\n got %q\nwant %q", c.line, got, hostNOPRIV)
					}

					continue
				}

				if strings.Contains(got, "NOPRIV") || (c.allowed != "" && !strings.Contains(got, c.allowed)) ||
					(c.allowed == "" && got != "") {
					t.Errorf("%s:\n%s", c.line, got)
				}
			}

			if after := hostFiles(t, host); refused && !slices.Equal(before, after) {
				t.Errorf("host files made while refused: %v, were %v", after, before)
			}

			if !refused {
				for _, name := range []string{"new.dsk"} {
					if _, err := os.Stat(filepath.Join(host, name)); err != nil {
						t.Errorf("%s not made: %v", name, err)
					}
				}

				if out := dispatch("DISMOUNT DUA2"); out != "" {
					t.Errorf("DISMOUNT DUA2: %s", out)
				}
			}
		})
	}
}
