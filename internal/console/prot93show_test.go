package console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Phase 93's subtask 12 (docs/planning/PHASE-93 - file protection.md):
// DIRECTORY/SECURITY, /ACL, and /FULL, and SHOW SECURITY and SHOW ACL, the
// commands that show a file's profile. TestSetSecurityOracle and
// TestCreateProfilesOracle replay them on volumes they make, and
// TestProt93UtilitiesOracle as each account; this file holds them to VMS's
// listings of the files VMS made, on round 1's volume as VMS left it
// (local only, as TestProt93ActivationOracle's).

// prot93TreeCommands are the listings PROT93Z.COM made at round 1's end,
// as SYSTEM: every file of the tree, its owner, protection, and ACL.
var prot93TreeCommands = []string{
	"DIRECTORY/SECURITY PROT93$DISK:[PROT93...]*.*;*",
	"DIRECTORY/SECURITY PROT93$DISK:[PROT93LOG]*.*;*",
	"DIRECTORY/SECURITY PROT93$DISK:[000000]*.*;*",
	"DIRECTORY/SECURITY PROT93$DISK:[PROT93TOP...]*.*;*,[300001...]*.*;*,[200002...]*.*;*",
}

// prot93FCommands are SYSTEM's reports of [PROT93.F] and of the volume in
// prot93.log. The
// volume is as round 1 ended, so its files were revised after them (each
// /FULL entry's "Revised:" line, masked), and FACLCTL.DAT gives the world
// RE, as PROT93A's run set it after (prot93LaterChanges).
var prot93FCommands = []string{
	"DIRECTORY/SECURITY PROT93$DISK:[PROT93.F]",
	"DIRECTORY/ACL PROT93$DISK:[PROT93.F]",
	"DIRECTORY/FULL PROT93$DISK:[PROT93.F]",
	"SHOW SECURITY PROT93$DISK:[PROT93.F]*.*",
	"SHOW SECURITY/CLASS=VOLUME PROT93$DISK:",
}

// TestProt93TreeOracle replays PROT93Z.COM's listings on round 1's volume.
func TestProt93TreeOracle(t *testing.T) {
	c, run := prot93Console(t, "prot93-round1.dsk", false)

	for _, id := range sec93Identifiers {
		if status := c.RTL.AddIdentifier(id.name, id.value, 0); status != 1 {
			t.Fatalf("AddIdentifier %s: %#x", id.name, status)
		}
	}

	lines := 0

	for log, commands := range map[string][]string{"prot93z.log": prot93TreeCommands, "prot93.log": prot93FCommands} {
		for _, command := range commands {
			want := logSection(t, prot93Log(t, log), "$ "+command, "$ ")[1:]
			got := prot93Lines(run, command)

			if log == "prot93.log" {
				want, got = prot93LaterChanges(want), prot93MaskRevised(got)
			}
			lines += len(want)

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("$ %s\n%s", command, prot93FirstDifference(got, want))
			}
		}
	}

	t.Logf("%d lines compared", lines)
}

// prot93FirstDifference says where two listings first differ, with the
// lines around it.
func prot93FirstDifference(got, want []string) string {
	for i := 0; i < max(len(got), len(want)); i++ {
		g, w := "<end>", "<end>"
		if i < len(got) {
			g = got[i]
		}

		if i < len(want) {
			w = want[i]
		}

		if g != w {
			return fmt.Sprintf("line %d:", i+1) + "\n got: " + g + "\nwant: " + w
		}
	}

	return "(no difference)"
}

// prot93LaterChanges is a report of [PROT93.F] made before PROT93A's run
// gave FACLCTL.DAT world RE, as if after it, with its "Revised:" lines
// masked.
func prot93LaterChanges(lines []string) []string {
	out := prot93MaskRevised(lines)
	inFACLCTL := false

	for i, l := range out {
		switch {
		case strings.HasPrefix(l, "FACLCTL.DAT;1"), strings.HasPrefix(l, "DUA1:[PROT93.F]FACLCTL.DAT;1 "):
			inFACLCTL = true
		case strings.HasPrefix(l, "FACL"), strings.HasPrefix(l, "DUA1:[PROT93.F]"):
			inFACLCTL = false
		}

		if !inFACLCTL {
			continue
		}

		l = strings.Replace(l, "(,,,)", "(,,,RE)", 1)
		l = strings.Replace(l, "World:", "World:RE", 1)
		out[i] = strings.Replace(l, "Group, World)", "Group, World: RE)", 1)
	}

	return out
}

// prot93MaskRevised is lines with each /FULL "Revised:" line's date and
// revision count masked.
func prot93MaskRevised(lines []string) []string {
	out := append([]string(nil), lines...)

	for i, l := range out {
		if strings.HasPrefix(l, "Revised:") {
			out[i] = "Revised:   <masked>"
		}
	}

	return out
}

// TestShowSecurityCases: what the probe's logs leave to govax's choice
// (each unconfirmed in the phase doc) and the forms they show, on a new
// volume: a missing file, an unknown class, a list, a file the process
// may not read (VMS's READERR), SHOW ACL with entries and without, and
// F$FILE_ATTRIBUTES' UIC by the rights database's names.
func TestShowSecurityCases(t *testing.T) {
	c, run := prot93SetConsole(t)
	p := c.RTL.Process

	for _, line := range []string{
		"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]A.DAT",
		"COPY PROT93$DISK:[PROT93LOG]T.DAT PROT93$DISK:[PROT93.SEC]B.DAT",
		"SET SECURITY/OWNER=[200,1]/PROTECTION=(S,O:RWED,G,W)/ACL=(IDENTIFIER=PROT93_TEAM,ACCESS=READ) PROT93$DISK:[PROT93.SEC]A.DAT",
	} {
		run(line)
	}

	for _, tc := range []struct{ command, want string }{
		{"SHOW SECURITY PROT93$DISK:[PROT93.SEC]NOSUCH.DAT",
			"%SHOW-E-OPENIN, error opening DUA1:[PROT93.SEC]NOSUCH.DAT; as input\n-RMS-E-FNF, file not found"},
		{"SHOW SECURITY/CLASS=QUEUE PROT93$DISK:[PROT93.SEC]A.DAT",
			"%DCL-W-IVKEYW, unrecognized keyword - check validity and spelling\n \\QUEUE\\"},
		{"SHOW ACL PROT93$DISK:[PROT93.SEC]A.DAT,B.DAT",
			"%DCL-W-NOLIST, list of parameter values not allowed - check use of comma (,)\n \\PROT93$DISK:[PROT93.SEC]A.DAT,B\\"},
		{"SHOW ACL PROT93$DISK:[PROT93.SEC]A.DAT",
			"Object type: FILE,  Object name: DUA1:[PROT93.SEC]A.DAT;1,  on <time>\n          (IDENTIFIER=PROT93_TEAM,ACCESS=READ)"},
		{"SHOW ACL PROT93$DISK:[PROT93.SEC]B.DAT",
			"Object type: FILE,  Object name: DUA1:[PROT93.SEC]B.DAT;1,  on <time>"},
		{"SHOW SECURITY PROT93$DISK:[PROT93.SEC]A.DAT",
			"\nDUA1:[PROT93.SEC]A.DAT;1 object of class FILE\n     Owner: [PROT93,PROT93A]\n" +
				"     Protection: (System, Owner: RWED, Group, World)\n     Access Control List:\n" +
				"          (IDENTIFIER=PROT93_TEAM,ACCESS=READ)"},
		{"DIRECTORY/ACL PROT93$DISK:[PROT93.SEC]*.DAT",
			"\nDirectory DUA1:[PROT93.SEC]\n\nA.DAT;1            \n          (IDENTIFIER=PROT93_TEAM,ACCESS=READ)\n" +
				"B.DAT;1            \n\nTotal of 2 files."},
		{`WRITE SYS$OUTPUT F$FILE_ATTRIBUTES("PROT93$DISK:[PROT93.SEC]A.DAT","UIC")`, "[PROT93,PROT93A]"},
	} {
		got := strings.Join(prot93MaskTimes(prot93Lines(run, tc.command)), "\n")
		if got != tc.want {
			t.Errorf("$ %s\n got:\n%s\nwant:\n%s", tc.command, got, tc.want)
		}
	}

	// PROT93C ([300,1], no privileges) may neither read A.DAT nor control
	// it: READERR, and the files after it in a wildcard aren't shown.
	privileges := prot93Privilege("NETMBX") | prot93Privilege("TMPMBX")
	p.UIC, p.Username = uint32(security.NewUIC(0o300, 1)), "PROT93C"
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges

	want := "%SHOW-F-READERR, error reading DUA1:[PROT93.SEC]A.DAT;1\n" +
		"-SYSTEM-F-NOPRIV, insufficient privilege or object protection violation"
	if got := strings.Join(prot93Lines(run, "SHOW SECURITY PROT93$DISK:[PROT93.SEC]*.DAT"), "\n"); got != want {
		t.Errorf("as PROT93C:\n got:\n%s\nwant:\n%s", got, want)
	}

	p.UIC, p.Username = uint32(security.SystemUIC), "SYSTEM"
	p.ProcessPrivileges, p.CurrentPrivileges = security.AllPrivileges, security.AllPrivileges
}
