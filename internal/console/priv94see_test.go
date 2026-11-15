package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// priv94WalkProcedure is PRIV94D.COM's WALK subroutine as a procedure of
// its own: F$PID's walk of the processes, saying which of the holders and
// itself it saw, and how many processes in all.
const priv94WalkProcedure = `$ CTX = ""
$ N = 0
$ SEEN = ""
$ ME = F$GETJPI("","PID")
$ WALK_LOOP:
$ PID = F$PID(CTX)
$ IF PID .EQS. "" THEN GOTO WALK_DONE
$ N = N + 1
$ IF PID .EQS. GPID THEN SEEN = SEEN + " G"
$ IF PID .EQS. WPID THEN SEEN = SEEN + " W"
$ IF PID .EQS. ME THEN SEEN = SEEN + " SELF"
$ IF N .LT. 500 THEN GOTO WALK_LOOP
$ WALK_DONE:
$ WRITE SYS$OUTPUT "F$PID saw''SEEN'"
$ WRITE SYS$OUTPUT "  ~ F$PID count ", N
`

// priv94Walks makes each "GOSUB WALK" of the sections, whose subroutine's
// lines VMS's log shows as commands of their own, one command running
// priv94WalkProcedure from a file (written in dir), with what the
// subroutine's lines wrote as its output.
func priv94Walks(t *testing.T, sections []priv94DCLSection, dir string) []priv94DCLSection {
	t.Helper()

	file := filepath.Join(dir, "walk.com")
	if err := os.WriteFile(file, []byte(priv94WalkProcedure), 0o644); err != nil {
		t.Fatal(err)
	}

	out := make([]priv94DCLSection, len(sections))

	for i, s := range sections {
		out[i].heading = s.heading

		for j := 0; j < len(s.commands); j++ {
			cmd := s.commands[j]
			if cmd.line != "GOSUB WALK" {
				out[i].commands = append(out[i].commands, cmd)

				continue
			}

			walk := priv94DCLCommand{line: `@"` + file + `"`}

			for j++; j < len(s.commands) && s.commands[j].line != "RETURN"; j++ {
				walk.out = append(walk.out, s.commands[j].out...)
			}

			out[i].commands = append(out[i].commands, walk)
		}
	}

	return out
}

// priv94SeeMask leaves out of a SEE section's lines what two systems can't
// share: SHOW SYSTEM's title and every process but the holders (VMS's
// system processes, govax's own), and of the holders' lines only their PIDs
// and names; the time SHOW PROCESS shows, and its node; a process's
// current priority, which each scheduler boosts in its own way; F$PID's
// count of processes (a "~" line), and the order it found them in, which
// is the order of their PIDs, process 1's first on govax.
func priv94SeeMask(line string, out []string) []string {
	var kept []string

	if line == "SHOW SYSTEM" {
		for _, l := range out {
			if f := strings.Fields(l); len(f) > 1 && (f[1] == "PRIV94_G" || f[1] == "PRIV94_W") {
				kept = append(kept, f[0]+" "+f[1])
			}
		}

		return kept
	}

	for _, l := range out {
		switch {
		case strings.HasPrefix(l, "  ~"):
			continue
		case strings.Contains(line, `"PRI")`) && strings.Trim(l, "0123456789") == "":
			l = "(a priority)"
		case strings.HasPrefix(l, "F$PID saw"):
			seen := strings.Fields(l)[2:]
			slices.Sort(seen)
			l = strings.Join(append([]string{"F$PID", "saw"}, seen...), " ")
		case strings.Contains(l, "   User: "):
			l = l[strings.Index(l, "User: "):]
		case strings.Contains(l, "Node: "):
			l = l[strings.Index(l, "Process name: "):]
		}

		kept = append(kept, l)
	}

	return kept
}

// TestPriv94See replays the probe's SEE sections (the phase doc's subtask
// 8) as SYSTEM, PRIV94, and PRIV94N, with no privilege, GROUP, and WORLD:
// SHOW SYSTEM, SHOW PROCESS/IDENTIFICATION, F$GETJPI, and F$PID of the
// holders, PRIV94_G in group 200 and PRIV94_W in group 300, and SET
// PROCESS/PRIORITY, /SUSPEND, and /RESUME of them. SHOW SYSTEM shows every
// process to everyone (VMS's SHOW is installed with WORLD); the rest follow
// $GETJPI's rule, so SYSTEM, in group 1, needs WORLD for both.
func TestPriv94See(t *testing.T) {
	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94System, "priv94.log"},
		{priv94Account, "priv94u_priv94.log"},
		{priv94Plain, "priv94u_priv94n.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			sections := priv94Walks(t, priv94DCLSections(t, run.file, "SEE"), t.TempDir())
			replayPriv94DCLMasked(t, run.id, sections, nil, priv94Holders(t, run.file), priv94SeeMask)
		})
	}
}

// TestPriv94Priority replays round 2's PRIORITY sections (PRIV94R.COM's and
// PRIV94V.COM's) as SYSTEM and PRIV94: SET PROCESS/PRIORITY of the process
// itself above its base priority is refused without ALTPRI, by DCL before
// $SETPRI is asked, and done with it. Each line of F$GETJPI's PRI and PRIB
// is compared by PRIB alone: the current priority is the scheduler's.
func TestPriv94Priority(t *testing.T) {
	mask := func(line string, out []string) []string {
		if !strings.Contains(line, `"PRIB")`) {
			return out
		}

		var kept []string

		for _, l := range out {
			if f := strings.Fields(l); len(f) == 2 {
				l = "base " + f[1]
			}

			kept = append(kept, l)
		}

		return kept
	}

	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94System, "round2/priv94r.log"},
		{priv94Account, "round2/priv94v.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			replayPriv94DCLMasked(t, run.id, priv94DCLSections(t, run.file, "PRIORITY"), nil, nil, mask)
		})
	}
}
