package console

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 93's subtask 11 (docs/planning/PHASE-93 - file protection.md):
// SET SECURITY and its obsolete forms, SET FILE, SET PROTECTION file, and
// SET ACL, held to SYSTEM's run of the probe (prot93.log's "SET
// SECURITY", "MALFORMED AND ODD VALUES", and "ACLS" sections, and round
// 2's "OBSOLETE FORMS WITH NO VALUE") on a new volume made as PROT93.COM
// made the VAX's [PROT93.SEC] and [PROT93.ACE].
//
// The sections' SHOW SECURITY, SHOW ACL, DIRECTORY/SECURITY, and
// DIRECTORY/ACL (subtask 12) hold what the commands did to VMS's reports.

// prot93SetConsole is a console with a new volume on DUA1 holding the
// probe's [PROT93LOG]T.DAT, [PROT93.SEC], and [PROT93.ACE], the MFD and
// [PROT93] giving everyone RWE as PROT93.COM made them, and the probe's
// identifiers in the rights database.
func prot93SetConsole(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatalf("STARTUP.COM: %v", err)
	}

	run := func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		return buf.String()
	}

	for _, id := range sec93Identifiers {
		if status := c.RTL.AddIdentifier(id.name, id.value, 0); status != 1 {
			t.Fatalf("AddIdentifier %s: %#x", id.name, status)
		}
	}

	dir := t.TempDir()
	disk := filepath.Join(dir, "prot93.dsk")
	data := filepath.Join(dir, "t.dat")
	writeHostFile(t, data, "RECORD ONE OF THE TEMPLATE\n")

	for _, line := range []string{
		`INITIALIZE/CONTAINER "` + disk + `" /SIZE=8000 PROT93`,
		`MOUNT/WRITE DUA1 "` + disk + `"`,
		"DEFINE PROT93$DISK DUA1:",
		"SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) PROT93$DISK:[000000]000000.DIR",
		"CREATE/DIRECTORY PROT93$DISK:[PROT93LOG]",
		`COPY "` + data + `" PROT93$DISK:[PROT93LOG]T.DAT`,
		"CREATE/DIRECTORY PROT93$DISK:[PROT93]",
		"CREATE/DIRECTORY PROT93$DISK:[PROT93.ACE]",
		"CREATE/DIRECTORY PROT93$DISK:[PROT93.SEC]",
	} {
		if out := run(line); strings.Contains(out, "-W-") || strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	return c, run
}

// prot93Lines is a command's output as lines, the last newline dropped.
func prot93Lines(run func(string) string, command string) []string {
	out := strings.TrimSuffix(run(command), "\n")
	if out == "" {
		return nil
	}

	return strings.Split(out, "\n")
}

// prot93Profiles is SHOW SECURITY's report of the files spec names.
func prot93Profiles(run func(string) string, spec string) []string {
	return prot93Lines(run, "SHOW SECURITY "+spec)
}

// prot93SetCompared reports whether a command of the sections is one
// to run and compare.
func prot93SetCompared(line string) bool {
	switch {
	case strings.HasPrefix(line, "SHOW SECURITY "), strings.HasPrefix(line, "SHOW ACL "),
		strings.HasPrefix(line, "DIRECTORY/SECURITY "), strings.HasPrefix(line, "DIRECTORY/ACL "),
		strings.HasPrefix(line, "SET "), strings.HasPrefix(line, "SHOW PROTECTION"),
		strings.HasPrefix(line, "CREATE/"), strings.HasPrefix(line, "COPY/PROTECTION="),
		strings.HasPrefix(line, "COPY PROT93$DISK:[PROT93LOG]T.DAT "),
		strings.HasPrefix(line, "COPY PROT93$DISK:[PROT93.ACE]"),
		strings.HasPrefix(line, "WRITE SYS$OUTPUT ") && strings.Contains(line, "[PROT93.SEC]") && !prot93Unshown(line):
		return true
	}

	return false
}

// prot93Unshown reports whether a lexical function's item is one not
// compared: FID and DID are where the volume put the file.
func prot93Unshown(line string) bool {
	return strings.Contains(line, `"FID")`) || strings.Contains(line, `"DID")`)
}

// TestSetSecurityOracle replays the probe's sections as SYSTEM.
func TestSetSecurityOracle(t *testing.T) {
	t.Parallel()
	_, run := prot93SetConsole(t)

	log := prot93Log(t, "prot93.log")
	lines := logSection(t, log, `$ WRITE SYS$OUTPUT "######## SET SECURITY"`, `$ ! The ACLs'bytes`)
	lines = append(lines, logSection(t, prot93Log(t, "round2/prot93r.log"),
		`$ WRITE SYS$OUTPUT "######## OBSOLETE FORMS WITH NO VALUE"`, `$ SHOW SECURITY PROT93$DISK:[PROT93.ACE]L2.DAT`)...)

	compared, shown := 0, 0

	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok {
			continue
		}

		var want []string
		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$ ") {
			i++
			want = append(want, lines[i])
		}

		if !prot93SetCompared(command) {
			continue
		}

		out := strings.TrimSuffix(run(command), "\n")

		// govax's COPY says what it copied, where VMS's says nothing
		// without /LOG: a difference of COPY's, not this subtask's.
		if strings.HasPrefix(command, "COPY ") && strings.HasPrefix(out, "%COPY-S-COPIED") && !strings.Contains(out, "\n") {
			out = ""
		}

		var got []string
		if out != "" {
			got = strings.Split(out, "\n")
		}

		// SHOW ACL's heading has the time it ran.
		if strings.HasPrefix(command, "SHOW ACL ") {
			got, want = prot93MaskTimes(got), prot93MaskTimes(want)
		}

		if strings.HasPrefix(command, "SHOW SECURITY ") || strings.HasPrefix(command, "SHOW ACL ") {
			shown++
		}

		// The sections' CREATEs of files are all refused, so none reads
		// its data line.
		want = prot93CreateWanted(want)
		compared++

		// VMS's log has a NUL where DCL's segment ends with a blank
		// (NOPAREN's).
		for k := range want {
			want[k] = strings.ReplaceAll(want[k], "\x00", " ")
		}

		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("$ %s\n got:\n%s\nwant:\n%s", command, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	t.Logf("%d commands compared, %d profiles", compared, shown)

	if compared < 120 || shown < 40 {
		t.Errorf("%d commands and %d profiles compared, want the sections' all", compared, shown)
	}
}

// prot93MaskTimes is lines with each date and time made "<time>".
func prot93MaskTimes(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = prot93Times.ReplaceAllString(l, "<time>")
	}

	return out
}

// TestSetSecurityIdentities: what PROT93C ([300,1], no privileges) may
// change, as the probe's accounts found it (A.4, "Changing profiles"):
// a file whose ACL gives it CONTROL has its protection changed but not
// its owner (%SET-F-WRITEERR); one giving it nothing is refused by each
// form with that form's messages; its own file it may give its own UIC
// again but not another's.
func TestSetSecurityIdentities(t *testing.T) {
	t.Parallel()
	c, run := prot93SetConsole(t)
	p := c.RTL.Process

	for _, line := range []string{
		"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]CTL.DAT",
		"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]NONE.DAT",
		"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]MINE.DAT",
		"SET SECURITY/OWNER=[200,1]/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PROT93$DISK:[PROT93.SEC]*.DAT",
		"SET SECURITY/ACL=(IDENTIFIER=PROT93C,ACCESS=CONTROL) PROT93$DISK:[PROT93.SEC]CTL.DAT",
		"SET SECURITY/OWNER=[300,1] PROT93$DISK:[PROT93.SEC]MINE.DAT",
	} {
		if out := run(line); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
			t.Fatalf("%s: %s", line, out)
		}
	}

	privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")
	p.UIC, p.Username = uint32(security.NewUIC(0o300, 1)), "PROT93C"
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges

	const none = "DUA1:[PROT93.SEC]NONE.DAT;1"

	noPriv := "-SYSTEM-F-NOPRIV, insufficient privilege or object protection violation"

	for _, tc := range []struct{ line, want string }{
		{"SET SECURITY/PROTECTION=(W:RWED)/LOG PROT93$DISK:[PROT93.SEC]CTL.DAT",
			"%SET-I-MODIFIED, DUA1:[PROT93.SEC]CTL.DAT;1 modified"},
		{"SET SECURITY/OWNER=[300,1] PROT93$DISK:[PROT93.SEC]CTL.DAT",
			"%SET-F-WRITEERR, error writing DUA1:[PROT93.SEC]CTL.DAT;1\n" + noPriv},
		{"SET SECURITY/PROTECTION=(W:R) PROT93$DISK:[PROT93.SEC]NONE.DAT",
			"%SET-E-OPENIN, error opening " + none + " as input\n" + noPriv},
		{"SET SECURITY/ACL=(IDENTIFIER=PROT93C,ACCESS=READ) PROT93$DISK:[PROT93.SEC]NONE.DAT",
			"%SET-E-OPENIN, error opening " + none + " as input\n" + noPriv},
		{"SET FILE/PROTECTION=(W:R) PROT93$DISK:[PROT93.SEC]NONE.DAT",
			"%SET-E-PRONOTCHG, protection on " + none + " not changed\n" + noPriv +
				"\n%SET-E-PROERR, error protecting " + none + "\n" + noPriv},
		{"SET PROTECTION=(W:R) PROT93$DISK:[PROT93.SEC]NONE.DAT",
			"%SET-E-PRONOTCHG, protection on " + none + " not changed\n" + noPriv},
		{"SET SECURITY/OWNER=[300,1] PROT93$DISK:[PROT93.SEC]MINE.DAT", ""},
		{"SET SECURITY/OWNER=[200,2] PROT93$DISK:[PROT93.SEC]MINE.DAT",
			"%SET-F-WRITEERR, error writing DUA1:[PROT93.SEC]MINE.DAT;1\n" + noPriv},
	} {
		if got := strings.TrimSuffix(run(tc.line), "\n"); got != tc.want {
			t.Errorf("$ %s\n got:\n%s\nwant:\n%s", tc.line, got, tc.want)
		}
	}

	p.UIC, p.Username = uint32(security.SystemUIC), "SYSTEM"
	p.ProcessPrivileges, p.CurrentPrivileges = security.AllPrivileges, security.AllPrivileges

	got := prot93Profiles(run, "PROT93$DISK:[PROT93.SEC]*.DAT")
	want := []string{
		"", "DUA1:[PROT93.SEC]CTL.DAT;1 object of class FILE",
		"     Owner: [PROT93,PROT93A]",
		"     Protection: (System: RWED, Owner: RWED, Group: RE, World: RWED)",
		"     Access Control List:",
		"          (IDENTIFIER=[PROT93C],ACCESS=CONTROL)",
		"", "DUA1:[PROT93.SEC]MINE.DAT;1 object of class FILE",
		"     Owner: [PROT93C]",
		"     Protection: (System: RWED, Owner: RWED, Group: RE, World: RE)",
		"     Access Control List: <empty>",
		"", "DUA1:[PROT93.SEC]NONE.DAT;1 object of class FILE",
		"     Owner: [PROT93,PROT93A]",
		"     Protection: (System: RWED, Owner: RWED, Group: RE, World: RE)",
		"     Access Control List: <empty>",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("profiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
