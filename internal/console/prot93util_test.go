package console

import (
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Phase 93's subtask 9 (docs/planning/PHASE-93 - file protection.md): the
// DCL utilities checked against the process's identity, held to the
// probe's Part B (testdata/prot93/prot93b.com), which SYSTEM without its
// four privileges (M0) and each account without privileges (A0, B0, C0)
// ran on VMS 7.1.
//
// The commands are replayed from the logs, where SET VERIFY shows each
// after substitution, on a copy of the exchange volume as VMS left it
// (local only, as TestProt93ProgramOracle's). A command that changes the
// volume is compared only where VMS refused it: what it did when allowed
// is on the volume already.

// prot93UtilRun is one identity's DCL in a log: the log, the tag, the
// user, and the sections of its commands replayed, each from a line to
// the one before a line that starts with its end.
type prot93UtilRun struct {
	log, tag, user string
	sections       [][2]string
}

// prot93UtilSections are the sections an account's run of PROT93U.COM
// replays: Part B, the ACL'd files typed, the changes to profiles that
// need control (subtask 11), and new files in others' directories.
func prot93UtilSections(tag string) [][2]string {
	return [][2]string{
		{"$ @PROT93$DISK:[000000]PROT93B " + tag, "$ EXIT"},
		{`$ WRITE SYS$OUTPUT "######## ACL-GRANTED AND -DENIED FILES"`, `$ WRITE SYS$OUTPUT "######## NEW FILES AND DIRECTORIES IN OTHERS' DIRECTORIES"`},
		{`$ WRITE SYS$OUTPUT "######## NEW FILES AND DIRECTORIES IN OTHERS' DIRECTORIES"`, `$ WRITE SYS$OUTPUT "######## END"`},
	}
}

// prot93OwnersSection is round 2's OWNERS section of an account's run of
// PROT93V.COM: owners given and refused (subtask 11).
var prot93OwnersSection = [][2]string{
	{`$ WRITE SYS$OUTPUT "######## OWNERS"`, `$ WRITE SYS$OUTPUT "######## NEW FILES' ACLS"`},
}

// prot93UtilRuns are the runs: SYSTEM's Part B, and each account's, and
// each account's round 2 owners.
var prot93UtilRuns = []prot93UtilRun{
	{"prot93.log", "M0", "SYSTEM", [][2]string{{"$ @PROT93$DISK:[000000]PROT93B M0", "$ EXIT"}}},
	{"prot93u_prot93a.log", "A0", "PROT93A", prot93UtilSections("A0")},
	{"prot93u_prot93b.log", "B0", "PROT93B", prot93UtilSections("B0")},
	{"prot93u_prot93c.log", "C0", "PROT93C", prot93UtilSections("C0")},
	{"round2/prot93v_prot93a.log", "A0", "PROT93A", prot93OwnersSection},
	{"round2/prot93v_prot93b.log", "B0", "PROT93B", prot93OwnersSection},
	{"round2/prot93v_prot93c.log", "C0", "PROT93C", prot93OwnersSection},
}

// prot93UtilLater are the commands a later subtask brings (showing
// profiles, 12) and those govax hasn't (SEARCH, DIFFERENCES, APPEND):
// their lines are run, if govax has them, but not compared.
var prot93UtilLater = []string{
	"SHOW SECURITY", "DIRECTORY/SECURITY",
	"SEARCH ", "DIFFERENCES ", "APPEND ", "DUMP/HEADER",
}

// prot93UtilChanging are the commands that change the volume.
var prot93UtilChanging = []string{
	"DELETE ", "PURGE ", "RENAME ", "COPY ", "OPEN/WRITE ", "OPEN/APPEND ", "CREATE ", "CREATE/DIRECTORY ",
	"CONVERT ", "BACKUP ", "MACRO/", "LINK/", "WRITE FU", "CLOSE FU",
	"SET SECURITY", "SET FILE", "SET PROTECTION=(W", "SET ACL", "CREATE/DIRECTORY/",
}

// prot93Times are a date and time as DUMP and LIBRARY/LIST show the
// moment they ran.
var prot93Times = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d(:\d\d(\.\d\d)?)?`)

// prot93UtilCommand is one command of a run: its line and VMS's output.
type prot93UtilCommand struct {
	line   string
	output []string
}

// prot93UtilCommands reads a run's commands from its log.
func prot93UtilCommands(t *testing.T, r prot93UtilRun) []prot93UtilCommand {
	t.Helper()

	log := prot93Log(t, r.log)

	var commands []prot93UtilCommand

	for _, bounds := range r.sections {
		section := logSection(t, log, bounds[0], bounds[1])

		for _, l := range section {
			if line, ok := strings.CutPrefix(l, "$ "); ok {
				commands = append(commands, prot93UtilCommand{line: line})
			} else if len(commands) > 0 {
				commands[len(commands)-1].output = append(commands[len(commands)-1].output, l)
			}
		}
	}

	return commands
}

// prot93UtilNames are the probe's UICs by the names VMS's rights
// database gave them: each is compared as its numbers, since govax names
// a process's own UIC by its username alone where VMS's database named
// some by group and member.
var prot93UtilNames = strings.NewReplacer(
	"[PROT93,PROT93A]", "[200,1]", "[PROT93,PROT93B]", "[200,2]", "[PROT93C]", "[300,1]",
	"[PROT93,*]", "[200,*]", "[PROT93A]", "[200,1]", "[PROT93B]", "[200,2]",
)

// prot93UtilNormal is output as compared: times masked, the probe's
// identifier names as numbers, trailing blanks and blank lines at the
// end gone.
func prot93UtilNormal(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = prot93UtilNames.Replace(prot93Times.ReplaceAllString(l, "<time>"))
		out = append(out, strings.TrimRight(l, " "))
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// prot93UtilDifferences are the commands whose output differs from VMS's
// for a reason, by run tag and line (the tag as "@"), with the reason.
// Each must still differ.
var prot93UtilDifferences = map[string]string{
	// PROT93A owns [.UDN] and lists it; the files the passes after A0's
	// made there (N_M0.DAT, SM0.DIR, ...) are on the volume now.
	"A0 DIRECTORY PROT93$DISK:[PROT93.UDN]": "later passes' files in [.UDN]",
	"A0 DIRECTORY":                          "later passes' files in [.UDN], the default directory",

}

// TestProt93UtilitiesOracle replays Part B as each identity.
func TestProt93UtilitiesOracle(t *testing.T) {
	c, run := prot93Console(t, "prot93-exchange.dsk", false)
	p := c.RTL.Process

	// The commands name the probe's identifiers, as AUTHORIZE made them.
	for _, id := range sec93Identifiers {
		if status := c.RTL.AddIdentifier(id.name, id.value, 0); status != 1 {
			t.Fatalf("AddIdentifier %s: %#x", id.name, status)
		}
	}

	compared := 0
	differed := map[string]bool{}

	for _, r := range prot93UtilRuns {
		account := prot93Accounts[r.user]
		privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")

		if r.user == "SYSTEM" {
			privileges = security.AllPrivileges &^ (prot93Privilege("READALL") | prot93Privilege("GRPPRV") |
				prot93Privilege("SYSPRV") | prot93Privilege("BYPASS"))
		}

		// Each run was a new process on VMS: no symbols from the one before.
		run("DELETE/SYMBOL/ALL")
		run("SET DEFAULT PROT93$DISK:[PROT93LOG]")
		run(`P1 = "` + r.tag + `"`)

		// The process's permanent privileges too: image rundown sets the
		// current ones from them.
		p.UIC, p.Username, p.Rights = uint32(account.uic), r.user, append(prot93InteractiveRights(), account.rights...)
		p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges

		for _, cmd := range prot93UtilCommands(t, r) {
			if strings.HasPrefix(cmd.line, "!") || cmd.line == "EXIT" || strings.HasPrefix(cmd.line, "@PROT93$DISK:[000000]PROT93B") {
				continue
			}

			// A CREATE VMS allowed read its data line (the log's
			// "RECORD"): it isn't run, there being no data here for it.
			if prot93CreateAllowed(cmd.line, cmd.output) {
				continue
			}

			got := strings.Split(run(cmd.line), "\n")

			if prot93UtilSkipped(cmd) {
				continue
			}

			key := r.tag + " " + strings.ReplaceAll(cmd.line, r.tag, "@")
			gl, wl := prot93UtilNormal(got), prot93UtilNormal(prot93CreateWanted(cmd.output))
			compared++

			if why, ok := prot93UtilDifferences[key]; ok {
				if gl != wl {
					differed[key] = true

					t.Logf("%s: differs as expected (%s)", key, why)
				}

				continue
			}

			if gl != wl {
				t.Errorf("%s %s: $ %s\n got:\n%s\nwant:\n%s", r.log, r.tag, cmd.line, gl, wl)
			}
		}

		p.UIC, p.Username, p.Rights = uint32(security.SystemUIC), "SYSTEM", prot93InteractiveRights()
		p.ProcessPrivileges, p.CurrentPrivileges = security.AllPrivileges, security.AllPrivileges
	}

	t.Logf("%d commands compared", compared)

	for key := range prot93UtilDifferences {
		if !differed[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from prot93UtilDifferences", key)
		}
	}
}

// prot93UtilSkipped reports whether a command isn't compared: a later
// subtask's, or one that changes the volume and VMS allowed.
func prot93UtilSkipped(cmd prot93UtilCommand) bool {
	for _, v := range prot93UtilLater {
		if strings.HasPrefix(cmd.line, v) {
			return true
		}
	}

	for _, v := range prot93UtilChanging {
		if strings.HasPrefix(cmd.line, v) && !prot93UtilRefused(cmd) {
			return true
		}
	}

	return false
}

// prot93UtilRefused reports whether VMS's output for cmd has a message:
// a refusal, for the commands the probe made to be refused.
func prot93UtilRefused(cmd prot93UtilCommand) bool {
	return strings.Contains(strings.Join(cmd.output, "\n"), "%")
}
