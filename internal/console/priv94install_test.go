package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// priv94NamedSection reads the section headed exactly heading ("########
// INSTALL") from the VMS log file: its commands and what each wrote, up to
// the section's "$ !", the next heading, or its EXIT.
func priv94NamedSection(t *testing.T, file, heading string) priv94DCLSection {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "priv94", "vax", file))
	if err != nil {
		t.Fatal(err)
	}

	var (
		s  = priv94DCLSection{heading: heading}
		in bool
	)

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		switch {
		case line == heading:
			in = true

			continue
		case !in:
			continue
		case line == "$ !" || line == "$ EXIT" || strings.HasPrefix(line, "######## "):
			return s
		}

		if cmd, ok := strings.CutPrefix(line, "$ "); ok {
			s.commands = append(s.commands, priv94DCLCommand{line: cmd})
		} else if n := len(s.commands); n > 0 {
			s.commands[n-1].out = append(s.commands[n-1].out, line)
		}
	}

	if len(s.commands) == 0 {
		t.Fatalf("%s has no section %q", file, heading)
	}

	return s
}

// priv94InstallMask leaves SHOW PROCESS/PRIVILEGES out of the comparison:
// its time, PID, and rights are the VAX's. The masks it would show are
// what PRIV94I prints, which is compared.
func priv94InstallMask(line string, out []string) []string {
	if strings.HasPrefix(line, "SHOW PROCESS") {
		return nil
	}

	return out
}

// TestPriv94Install replays the probe's installed image sections (the
// phase doc's subtask 9, Decision 9): SYSTEM runs PRIV94I plain, installs it
// /PRIVILEGED=(SYSNAM,OPER), lists it, and runs it with every privilege and
// then with only TMPMBX and NETMBX; PRIV94 and PRIV94N run it installed,
// list it, and are refused INSTALL ADD without CMKRNL; the end removes it,
// whose LIST is then NOKFEFND, and runs it plain again. PRIV94I prints the
// four masks and whether $CRELNM in the system table worked.
func TestPriv94Install(t *testing.T) {
	build := func(install bool) func(*console.Console, func(string) string) map[string]string {
		return func(_ *console.Console, dispatch func(string) string) map[string]string {
			buildPriv94(t, dispatch, []string{"PRIV94I", "PRIV94H"})

			if install {
				line := "INSTALL ADD PRIV94$DISK:[000000]PRIV94I/OPEN/HEADER_RESIDENT/PRIVILEGED=(SYSNAM,OPER)"
				if out := dispatch(line); out != "" {
					t.Fatalf("%s: %s", line, out)
				}
			}

			return nil
		}
	}

	t.Run("SYSTEM", func(t *testing.T) {
		replayPriv94DCLMasked(t, priv94System,
			[]priv94DCLSection{priv94NamedSection(t, "priv94.log", "######## INSTALL")},
			nil, build(false), priv94InstallMask)
	})

	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94Account, "priv94u_priv94.log"},
		{priv94Plain, "priv94u_priv94n.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			replayPriv94DCLMasked(t, run.id,
				[]priv94DCLSection{priv94NamedSection(t, run.file, "######## INSTALLED "+run.id.name)},
				nil, build(true), priv94InstallMask)
		})
	}

	t.Run("end", func(t *testing.T) {
		replayPriv94DCLMasked(t, priv94System,
			[]priv94DCLSection{priv94NamedSection(t, "priv94z.log", "######## INSTALL")},
			nil, build(true), priv94InstallMask)
	})
}
