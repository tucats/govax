package console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Phase 93's subtask 11b (docs/planning/PHASE-93 - file protection.md):
// CREATE's /PROTECTION and /OWNER_UIC and COPY's /PROTECTION, held to
// SYSTEM's run of the probe's "PROFILES OF NEW FILES" section
// (prot93.log) on a new volume made as PROT93.COM made the VAX's
// [PROT93.F], [PROT93.OWN], [PROT93.DP], and [PROT93.PRF].
//
// DIRECTORY/SECURITY and SHOW SECURITY are subtask 12's: until then their
// reports are made in Go from each file's profile (prot93DirectoryLines,
// prot93Profiles). govax has no APPEND, so the section's P6.DAT, made by
// APPEND/NEW_VERSION, is left out of VMS's listing.

// prot93ProfileSetup is what PROT93.COM's TREE section did that the
// section depends on.
var prot93ProfileSetup = []string{
	"CREATE/DIRECTORY PROT93$DISK:[PROT93.F]",
	"CREATE/DIRECTORY PROT93$DISK:[PROT93.OWN]",
	"CREATE/DIRECTORY PROT93$DISK:[PROT93.DP]",
	"CREATE/DIRECTORY PROT93$DISK:[PROT93.PRF]",
	"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.F]FROT1.DAT",
	"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:R,O:W,G:E,W:D) PROT93$DISK:[PROT93.F]FROT1.DAT",
	"SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PROT93$DISK:[PROT93LOG]T.DAT",
	"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[PROT93]F.DIR",
	"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[PROT93]OWN.DIR",
	"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[PROT93]DP.DIR",
	"SET SECURITY/ACL=((DEFAULT_PROTECTION,S:RWED,O:RWED,G:R,W:E),(IDENTIFIER=PROT93_TEAM,OPTIONS=DEFAULT,ACCESS=READ)," +
		"(IDENTIFIER=PROT93B,OPTIONS=DEFAULT+NOPROPAGATE,ACCESS=READ+WRITE),(IDENTIFIER=PROT93A,ACCESS=READ)) PROT93$DISK:[PROT93]DP.DIR",
	"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[PROT93]PRF.DIR",
}

// prot93DirectoryLines is DIRECTORY/SECURITY's listing of spec as VMS 7.1
// laid it out: the heading, a line per file (its name and version, owner,
// and protection in columns), and the total. DIRECTORY shows every
// version, so a spec with no version is given ";*".
func prot93DirectoryLines(t *testing.T, c *Console, spec string) []string {
	t.Helper()

	full := spec
	if !strings.Contains(full[strings.LastIndexByte(full, ']')+1:], ";") {
		if strings.HasSuffix(full, "]") {
			full += "*.*"
		}

		full += ";*"
	}

	objects, err := c.RTL.Session.SecurityObjects(full)
	if err != nil {
		t.Fatalf("SecurityObjects %s: %v", full, err)
	}

	names := consoleNames{c}
	heading := ""

	var out []string

	for _, o := range objects {
		dir, name := o.Name[:strings.LastIndexByte(o.Name, ']')+1], o.Name[strings.LastIndexByte(o.Name, ']')+1:]
		if dir != heading {
			heading = dir
			out = append(out, "", "Directory "+dir, "")
		}

		p := o.Profile()
		out = append(out, strings.TrimRight(fmt.Sprintf("%-21s%-33s%s", name,
			security.IdentifierText(uint32(p.Owner), names), prot93ShortProtection(p.Protection)), " "))
	}

	files := "files"
	if len(objects) == 1 {
		files = "file"
	}

	return append(out, "", fmt.Sprintf("Total of %d %s.", len(objects), files))
}

// prot93ShortProtection is a protection code as DIRECTORY/SECURITY shows
// it: "(RWED,RWED,RE,)".
func prot93ShortProtection(p security.Protection) string {
	parts := make([]string, 4)
	for c := range parts {
		parts[c] = p.Allows(security.Category(c)).String()
	}

	return "(" + strings.Join(parts, ",") + ")"
}

// prot93CreateWanted is VMS's output for a CREATE (or COPY) as compared:
// the data line its log shows ("RECORD"), and what DCL said of that line
// after a refusal (SKPDAT, or INSFPRM after a refused value), are the
// procedure's, not the command's.
func prot93CreateWanted(want []string) []string {
	var out []string

	for _, l := range want {
		if l == "RECORD" || strings.HasPrefix(l, "%DCL-W-SKPDAT") || strings.HasPrefix(l, "%DCL-W-INSFPRM") {
			continue
		}

		out = append(out, l)
	}

	return out
}

// prot93CreateRun runs a CREATE of a file that reads a data line: as a
// procedure holding the command and the line, so SYS$INPUT has it.
func prot93CreateRun(t *testing.T, run func(string) string, command string) string {
	t.Helper()

	proc := writeProcedure(t, t.TempDir(), "create.com", "$ "+command, "RECORD")

	return run("@" + proc)
}

// prot93CreateAllowed reports whether a command is a CREATE of a file
// that VMS let read its data (its output had no message).
func prot93CreateAllowed(command string, want []string) bool {
	if !strings.HasPrefix(command, "CREATE ") && !strings.HasPrefix(command, "CREATE/") || strings.HasPrefix(command, "CREATE/DIRECTORY") {
		return false
	}

	return !strings.Contains(strings.Join(want, "\n"), "%")
}

// TestCreateProfilesOracle replays the probe's "PROFILES OF NEW FILES"
// section as SYSTEM.
func TestCreateProfilesOracle(t *testing.T) {
	c, run := prot93SetConsole(t)

	for _, line := range prot93ProfileSetup {
		if out := run(line); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") || strings.Contains(out, "-W-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	lines := logSection(t, prot93Log(t, "prot93.log"),
		`$ WRITE SYS$OUTPUT "######## PROFILES OF NEW FILES"`, `$ WRITE SYS$OUTPUT "######## DEFAULT PROTECTION"`)

	compared := 0

	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok || strings.HasPrefix(command, "!") || strings.HasPrefix(command, "WRITE ") {
			continue
		}

		var want []string
		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$ ") {
			i++
			want = append(want, lines[i])
		}

		var got []string

		switch {
		case strings.HasPrefix(command, "APPEND"):
			continue // govax has no APPEND
		case strings.HasPrefix(command, "DIRECTORY/SECURITY "):
			got = prot93DirectoryLines(t, c, strings.TrimPrefix(command, "DIRECTORY/SECURITY "))

			var kept []string
			for _, l := range want {
				if !strings.HasPrefix(l, "P6.DAT") {
					kept = append(kept, l)
				}
			}

			if len(kept) < len(want) && kept[len(kept)-1] == "Total of 6 files." {
				kept[len(kept)-1] = "Total of 5 files."
			}

			want = kept
		case strings.HasPrefix(command, "SHOW SECURITY "):
			if strings.Contains(command, ",") {
				continue // a list, DCL's NOLIST: subtask 12's
			}

			got = prot93Profiles(t, c, command[strings.LastIndexByte(command, ' ')+1:])
		default:
			out := ""
			if prot93CreateAllowed(command, want) {
				out = prot93CreateRun(t, run, command)
			} else {
				out = run(command)
			}

			out = strings.TrimSuffix(out, "\n")

			// govax's COPY says what it copied, where VMS's says nothing
			// without /LOG.
			if strings.HasPrefix(command, "COPY") && strings.HasPrefix(out, "%COPY-S-COPIED") && !strings.Contains(out, "\n") {
				out = ""
			}

			if out != "" {
				got = strings.Split(out, "\n")
			}

			want = prot93CreateWanted(want)
		}

		compared++

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("$ %s\n got:\n%s\nwant:\n%s", command, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	t.Logf("%d commands compared", compared)

	if compared < 30 {
		t.Errorf("%d commands compared, want the section's all", compared)
	}
}

// TestCreateOwnerAndProtection: CREATE/OWNER_UIC of another's UIC by an
// account without privilege is refused before its data is read, its own
// allowed (A.4, round 2); a code giving nobody access is taken as none
// (the DCL Dictionary); BACKUP's /PROTECTION is merged into the save
// set's; and the refusals of bad values.
func TestCreateOwnerAndProtection(t *testing.T) {
	c, run := prot93SetConsole(t)
	p := c.RTL.Process

	for _, line := range []string{
		"CREATE/DIRECTORY/OWNER_UIC=[200,1] PROT93$DISK:[PROT93.OWN]",
		"SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[PROT93]OWN.DIR",
	} {
		if out := run(line); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	for _, tc := range []struct{ command, want string }{
		{"CREATE/PROTECTION=(S,O,G,W) PROT93$DISK:[PROT93.SEC]NONE.DAT", ""},
		{"CREATE/PROTECTION=(S:Z) PROT93$DISK:[PROT93.SEC]M1.DAT", "%CREATE-F-SYNTAX, error parsing 'Z'"},
		{"CREATE/OWNER_UIC=[NOSUCH] PROT93$DISK:[PROT93.SEC]M1.DAT",
			"%CREATE-F-SYNTAX, error parsing '[NOSUCH]'\n-SYSTEM-F-NOSUCHID, unknown rights identifier"},
		{"COPY/PROTECTION=(S:Z) PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]M2.DAT", "%COPY-F-SYNTAX, error parsing 'Z'"},
		{"COPY/PROTECTION=(Q:R) PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]M2.DAT",
			"%DCL-W-IVKEYW, unrecognized keyword - check validity and spelling\n \\Q\\"},
		{"BACKUP PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]S.BCK/SAVE_SET/PROTECTION=(S:Z)",
			"%BACKUP-F-SYNTAX, error parsing 'Z'"},
		{"BACKUP PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]S.BCK/SAVE_SET/PROTECTION=(G:RWED,W:R)", ""},
	} {
		out := ""
		if strings.HasPrefix(tc.command, "CREATE") && tc.want == "" {
			out = prot93CreateRun(t, run, tc.command)
		} else {
			out = run(tc.command)
		}

		if got := strings.TrimSuffix(out, "\n"); got != tc.want {
			t.Errorf("$ %s\n got:\n%s\nwant:\n%s", tc.command, got, tc.want)
		}
	}

	for _, tc := range []struct{ spec, want string }{
		{"PROT93$DISK:[PROT93.SEC]NONE.DAT", "(RWED,RWED,RE,)"},
		{"PROT93$DISK:[PROT93.SEC]S.BCK", "(RWED,RWED,RWED,R)"},
	} {
		lines := prot93DirectoryLines(t, c, tc.spec)
		if len(lines) < 4 || !strings.HasSuffix(lines[3], tc.want) {
			t.Errorf("%s: %q, want its protection %s", tc.spec, lines, tc.want)
		}
	}

	// PROT93C ([300,1], no privileges) may name itself, not another.
	privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")
	p.UIC, p.Username = uint32(security.NewUIC(0o300, 1)), "PROT93C"
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges

	if got := strings.TrimSuffix(prot93CreateRun(t, run, "CREATE/OWNER_UIC=[PROT93C] PROT93$DISK:[PROT93.OWN]CM.DAT"), "\n"); got != "" {
		t.Errorf("its own UIC: %s", got)
	}

	want := "%CREATE-E-OPENOUT, error opening DUA1:[PROT93.OWN]CA.DAT; as output\n" +
		"-RMS-E-PRV, insufficient privilege or file protection violation"
	if got := strings.TrimSuffix(run("CREATE/OWNER_UIC=[200,1] PROT93$DISK:[PROT93.OWN]CA.DAT"), "\n"); got != want {
		t.Errorf("another's UIC:\n got:\n%s\nwant:\n%s", got, want)
	}

	p.UIC, p.Username = uint32(security.SystemUIC), "SYSTEM"
	p.ProcessPrivileges, p.CurrentPrivileges = security.AllPrivileges, security.AllPrivileges

	got := strings.Join(prot93DirectoryLines(t, c, "PROT93$DISK:[PROT93.OWN]C*.DAT"), "\n")
	if !strings.Contains(got, "CM.DAT;1             [PROT93C]") || strings.Contains(got, "CA.DAT") {
		t.Errorf("[PROT93.OWN]:\n%s", got)
	}
}
