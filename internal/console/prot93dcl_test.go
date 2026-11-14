package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 93's subtask 5 (docs/planning/PHASE-93 - file protection.md): the
// process's default protection and privileges at DCL, held to the probe's
// VMS 7.1 logs (testdata/prot93/vax).

// prot93Log returns the lines of one of the probe's VMS logs.
func prot93Log(t *testing.T, name string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "prot93", "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
}

// logSection returns the lines of log from the one that is first to the
// one before the first that starts with end, after it.
func logSection(t *testing.T, log []string, first, end string) []string {
	t.Helper()

	for i, line := range log {
		if line != first {
			continue
		}

		for j := i + 1; j < len(log); j++ {
			if strings.HasPrefix(log[j], end) {
				return log[i:j]
			}
		}
	}

	t.Fatalf("no section %q to %q", first, end)

	return nil
}

// TestDefaultProtectionOracle replays the probe's "DEFAULT PROTECTION"
// section: each SET PROTECTION/DEFAULT, SHOW PROTECTION, and
// F$ENVIRONMENT("PROTECTION") of it, and its malformed code, each giving
// the lines VMS 7.1 gave. (The section's CREATE and DIRECTORY lines are
// new files' profiles, subtask 7's.)
func TestDefaultProtectionOracle(t *testing.T) {
	log := prot93Log(t, "prot93.log")
	lines := logSection(t, log, "######## DEFAULT PROTECTION", "$ CREATE/DIRECTORY")
	lines = append(lines, logSection(t, log, "$ SET PROTECTION=(S:Z)/DEFAULT", "$ COPY")...)

	d, _, buf := newCommandDispatcher(t)
	commands := 0

	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok || !(strings.HasPrefix(command, "SET PROTECTION") || strings.HasPrefix(command, "SHOW PROTECTION") ||
			strings.Contains(command, `F$ENVIRONMENT("PROTECTION")`)) {
			continue
		}

		var want []string
		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$ ") {
			i++
			want = append(want, lines[i])
		}

		buf.Reset()

		if err := d.Dispatch(command); err != nil {
			buf.WriteString(dclMessage(err) + "\n")
		}

		if got := strings.TrimSuffix(buf.String(), "\n"); got != strings.Join(want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", command, got, strings.Join(want, "\n"))
		}

		commands++
	}

	if commands != 25 {
		t.Errorf("%d commands replayed, want 25", commands)
	}
}

// TestProtectionGrammarOracle replays the lines of the probe's
// "MALFORMED AND ODD VALUES" that govax's commands take today (Phase 93's
// subtask 6, the $PROTECTION and $UIC grammar types): CREATE/DIRECTORY's
// bad /OWNER_UIC and /PROTECTION, SET PROTECTION/DEFAULT's bad letters,
// and an unknown category in SET PROTECTION=(code) of a file, which DCL
// refuses before the command runs. Each one's messages must be VMS 7.1's.
func TestProtectionGrammarOracle(t *testing.T) {
	lines := logSection(t, prot93Log(t, "prot93.log"), "######## MALFORMED AND ODD VALUES", "######## ACLS")

	d, _, buf := newCommandDispatcher(t)
	commands := 0

	for i := 0; i < len(lines); i++ {
		command, ok := strings.CutPrefix(lines[i], "$ ")
		if !ok || !(strings.HasPrefix(command, "CREATE/DIRECTORY/") || strings.HasPrefix(command, "SET PROTECTION=")) {
			continue
		}

		var want []string
		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$ ") {
			i++
			want = append(want, lines[i])
		}

		buf.Reset()

		if err := d.Dispatch(command); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		if got := strings.TrimSuffix(buf.String(), "\n"); got != strings.Join(want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", command, got, strings.Join(want, "\n"))
		}

		commands++
	}

	if commands != 4 {
		t.Errorf("%d commands replayed, want 4", commands)
	}
}

// dclMessage is the message DCL shows for err, a command's failure.
func dclMessage(err error) string { return "%" + err.Error() }

// TestDefaultProtectionCommands: the code is merged into the process's own
// default protection, SET PROTECTION=(Q:R)/DEFAULT is DCL's IVKEYW, and
// SET PROTECTION of a file waits for subtask 11.
func TestDefaultProtectionCommands(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	p := c.RTL.Process

	if p.DefaultProtection != 0xFA00 {
		t.Errorf("process 1's default protection %#04x, want RMS_FILEPROT's %%XFA00", p.DefaultProtection)
	}

	if err := d.Dispatch("SET PROTECTION=(G,W:RE)/DEFAULT"); err != nil {
		t.Fatal(err)
	}

	if p.DefaultProtection != 0xAF00 {
		t.Errorf("default protection %#04x, want %%XAF00", p.DefaultProtection)
	}

	for _, tc := range []struct{ line, want string }{
		{"SET PROTECTION=(Q:R)/DEFAULT", "%DCL-W-IVKEYW, unrecognized keyword - check validity and spelling\n \\Q\\"},
		{"SET PROTECTION=(S:R,S:W)/DEFAULT", "%DCL-W-IVPROT, invalid protection code"},
		{"SET PROTECTION=(W:R) X.DAT", "%DCL-E-PROTNOTIMPL, SET PROTECTION of a file is not implemented yet; SET PROTECTION/DEFAULT is"},
	} {
		if err := d.Dispatch(tc.line); err == nil || dclMessage(err) != tc.want {
			t.Errorf("%s: %v, want %q", tc.line, err, tc.want)
		}
	}

	if p.DefaultProtection != 0xAF00 {
		t.Errorf("a refused code changed the default protection to %#04x", p.DefaultProtection)
	}
}

// TestShowProcessPrivilegesOracle: SHOW PROCESS/PRIVILEGES after its
// heading, as SYSTEM in the probe's prot93.log and as PROT93A, [200,1]
// authorized for NETMBX and TMPMBX, in prot93u_prot93a.log; and SET
// PROCESS/PRIVILEGES=BYPASS for PROT93A, %SYSTEM-W-NOTALLPRIV. VMS's
// SYSTEM held NET$MANAGE, and both held the SYS$NODE_ system right,
// which govax's processes don't, so those lines are left out.
func TestShowProcessPrivilegesOracle(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	report := func() []string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch("SHOW PROCESS/PRIVILEGES"); err != nil {
			t.Fatal(err)
		}

		return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")[3:]
	}

	vmsReport := func(log []string) []string {
		section := logSection(t, log, "Authorized privileges:", "System rights:")
		want := []string{" "}

		for _, line := range section {
			if strings.TrimSpace(line) != "NET$MANAGE" {
				want = append(want, line)
			}
		}

		return want[:len(want)-1] // the space before "System rights:"
	}

	if got, want := report(), vmsReport(prot93Log(t, "prot93.log")); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("SYSTEM:\n got %q\nwant %q", got, want)
	}

	priv := func(name string) uint64 { return 1 << vmsdef.Symbols["PRV$V_"+name] }
	p := c.RTL.Process
	p.UIC = uint32(security.NewUIC(0o200, 1))
	p.AuthorizedPrivileges = priv("NETMBX") | priv("TMPMBX")
	p.ProcessPrivileges, p.CurrentPrivileges = p.AuthorizedPrivileges, p.AuthorizedPrivileges

	accountLog := prot93Log(t, "prot93u_prot93a.log")
	if got, want := report(), vmsReport(accountLog); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("PROT93A:\n got %q\nwant %q", got, want)
	}

	section := logSection(t, accountLog, "$ SET PROCESS/PRIVILEGES=BYPASS", "$ IF")

	buf.Reset()

	if err := d.Dispatch("SET PROCESS/PRIVILEGES=BYPASS"); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSuffix(buf.String(), "\n"); got != section[1] {
		t.Errorf("SET PROCESS/PRIVILEGES=BYPASS: %q, want %q", got, section[1])
	}

	if p.CurrentPrivileges&priv("BYPASS") != 0 {
		t.Error("BYPASS was enabled for a process not authorized for it")
	}
}

// TestSetProcessPrivileges: SYSTEM turns off the four privileges that
// bear on protection, as the probe's passes did, permanently, so they stay
// off past an image's rundown; they come back, ALL and NOALL are every
// privilege, and a name that isn't a privilege is DCL's IVKEYW.
func TestSetProcessPrivileges(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	p := c.RTL.Process
	four := security.PrivBYPASS | security.PrivSYSPRV | security.PrivGRPPRV | security.PrivREADALL

	if err := d.Dispatch("SET PROCESS/PRIVILEGES=(NOREADALL,NOGRPPRV,NOSYSPRV,NOBYPASS)"); err != nil {
		t.Fatal(err)
	}

	if p.CurrentPrivileges&four != 0 || p.ProcessPrivileges&four != 0 {
		t.Errorf("privileges %#x (permanent %#x) still hold the four", p.CurrentPrivileges, p.ProcessPrivileges)
	}

	if creds := c.RTL.Credentials(); creds.Privileges&four != 0 || !creds.InSystemGroup() {
		t.Errorf("credentials %+v: want none of the four, [1,4] still a system UIC", creds)
	}

	c.RTL.ImageRundown()

	if p.CurrentPrivileges&four != 0 {
		t.Error("the privileges came back at an image's rundown")
	}

	if err := d.Dispatch("SET PROCESS/PRIVILEGES=(READALL,GRPPRV,SYSPRV,BYPASS)"); err != nil || p.CurrentPrivileges&four != four {
		t.Errorf("%v, privileges %#x: want the four back", err, p.CurrentPrivileges)
	}

	if err := d.Dispatch("SET PROCESS/PRIVILEGES=NOALL"); err != nil || p.CurrentPrivileges != 0 {
		t.Errorf("NOALL: %v, privileges %#x", err, p.CurrentPrivileges)
	}

	if err := d.Dispatch("SET PROCESS/PRIVILEGES=ALL"); err != nil || p.CurrentPrivileges != security.AllPrivileges {
		t.Errorf("ALL: %v, privileges %#x", err, p.CurrentPrivileges)
	}

	want := "%DCL-W-IVKEYW, unrecognized keyword - check validity and spelling\n \\NOSUCH\\"
	if err := d.Dispatch("SET PROCESS/PRIVILEGES=(NOBYPASS,NOSUCH)"); err == nil || dclMessage(err) != want {
		t.Errorf("NOSUCH: %v", err)
	}

	if p.CurrentPrivileges&security.PrivBYPASS == 0 {
		t.Error("a refused list changed the privileges")
	}
}

// TestCreateDirectoryOwnerOracle: an account without privileges may not
// give a new directory another's UIC, by /OWNER_UIC=uic or =PARENT under
// another's directory, and nothing is made (subtask 7; round 2's PROT93C,
// [300,1], under PROT93A's [PROT93.OWN], which gives everyone RWE, as the
// probe's did).
func TestCreateDirectoryOwnerOracle(t *testing.T) {
	log := prot93Log(t, filepath.Join("round2", "prot93v_prot93c.log"))

	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA1")

	for _, line := range []string{"DEFINE PROT93$DISK DUA1:", "CREATE/DIRECTORY/OWNER_UIC=[200,1]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) DUA1:[PROT93.OWN]"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	p := c.RTL.Process
	p.UIC, p.CurrentPrivileges = uint32(security.NewUIC(0o300, 1)), 0

	replayed := 0

	for i, line := range log {
		command, ok := strings.CutPrefix(line, "$ ")
		if !ok || !strings.HasPrefix(command, "CREATE/DIRECTORY/OWNER_UIC=") || !strings.Contains(command, "[PROT93.OWN.") {
			continue
		}

		var want []string
		for j := i + 1; j < len(log) && !strings.HasPrefix(log[j], "$ "); j++ {
			want = append(want, log[j])
		}

		buf.Reset()

		if err := d.Dispatch(command); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		if got := strings.TrimSuffix(buf.String(), "\n"); got != strings.Join(want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", command, got, strings.Join(want, "\n"))
		}

		replayed++
	}

	if replayed != 2 {
		t.Errorf("%d commands replayed, want 2", replayed)
	}

	// Its own UIC it may give.
	buf.Reset()

	if err := d.Dispatch("CREATE/DIRECTORY/OWNER_UIC=[300,1] PROT93$DISK:[PROT93.OWN.MINE]"); err != nil {
		t.Errorf("its own UIC: %v (%s)", err, buf.String())
	}
}
