package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/uaf"
)

// addUserConsole is a console with a system volume, STARTUP.COM run, and
// a fresh volume on DUA1, process 1 still SYSTEM.
func addUserConsole(t *testing.T) (*Console, *Dispatcher, *bytes.Buffer) {
	t.Helper()

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "sys.dsk")})
	if !r.Mounted {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatalf("STARTUP.COM: %v", err)
	}

	mountFreshContainer(t, c, "DUA1:")

	return c, d, buf
}

// TestAddUser is Phase 95's end to end (docs/planning/PHASE-95 -
// authentication.md, Decision 13): SYSTEM runs SYS$MANAGER:ADDUSER.COM,
// answering its questions at the terminal, and then process 1, logged in
// as the new account, has its UIC, privileges, and home, runs its
// LOGIN.COM, may read the system volume's files but not change them, and
// may write in its home; and ADDUSER refuses it at once.
func TestAddUser(t *testing.T) {
	c, d, buf := addUserConsole(t)

	c.In = strings.NewReader(strings.Join([]string{
		"smith",      // username
		"John Smith", // owner
		"",           // group 200
		"",           // the first free member, 1
		"Secret",     // password
		"Secret",     // again
		"DUA1",       // the home device
		"",           // [SMITH]
		"",           // a plain user
	}, "\n") + "\n")

	buf.Reset()

	if err := d.Dispatch("@SYS$MANAGER:ADDUSER"); err != nil {
		t.Errorf("ADDUSER: %v", err)
	}

	mustDispatch(t, d, "DIRECTORY/SECURITY DUA1:[000000]SMITH.DIR,DUA1:[SMITH]LOGIN.COM")

	out := buf.String()
	t.Logf("ADDUSER:\n%s", out)

	for _, want := range []string{
		"Username: Owner (the user's full name): UIC group number, octal [200]: UIC member number, octal [1]: ",
		"%UAF-I-ADDMSG, user record successfully added",
		"Added SMITH, [200,1], at home in DUA1:[SMITH].",
		"govax --user smith",
		"SMITH.DIR;1          [SMITH]                          (RWE,RWE,,)",
		"LOGIN.COM;1          [SMITH]                          (RWED,RWED,,)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ADDUSER's output lacks %q", want)
		}
	}

	if strings.Contains(out, "Secret") {
		t.Error("the password was shown")
	}

	f, err := c.RTL.System.Authorization()
	if err != nil {
		t.Fatal(err)
	}

	rec, ok := f.Find("SMITH")
	if !ok || rec.UIC != uaf.NewUIC(0o200, 1) || rec.Owner != "John Smith" || rec.Device != "DUA1:" ||
		rec.Directory != "[SMITH]" || rec.Flags&uaf.FlagDISUSER != 0 {
		t.Fatalf("SMITH's record: %+v", rec)
	}

	if ok, err := uaf.CheckPassword(rec.Password, "secret"); !ok || err != nil {
		t.Errorf("SMITH's password: %v, %v", ok, err)
	}

	// SMITH's own LOGIN.COM, a new version of the starter, still SMITH's.
	login := filepath.Join(t.TempDir(), "login.com")
	if err := os.WriteFile(login, []byte("$ SMITH_LOGGED_IN == \"YES\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustDispatch(t, d, `COPY "`+login+`"/HOST DUA1:[SMITH]LOGIN.COM`, "SET SECURITY/OWNER=[200,1] DUA1:[SMITH]LOGIN.COM;*")

	l, err := c.LogIn("SMITH", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := c.RunLoginProcedures(l.Record.LGICMD, d.Dispatch); err != nil {
		t.Fatalf("login procedures: %v", err)
	}

	buf.Reset()

	for _, line := range []string{
		`WRITE SYS$OUTPUT F$GETJPI("","USERNAME"),"/",F$GETJPI("","UIC"),"/",F$GETJPI("","CURPRIV")`,
		"SHOW DEFAULT",
		"SHOW SYMBOL/GLOBAL SMITH_LOGGED_IN",
		"COPY SYS$MANAGER:SYLOGIN.COM []MINE.COM",
		"DIRECTORY/SECURITY MINE.COM",
		"DELETE SYS$MANAGER:STARTUP.COM;*",
		"@SYS$MANAGER:ADDUSER",
	} {
		if err := d.Dispatch(line); err != nil {
			buf.WriteString("%" + err.Error() + "\n")
		}
	}

	out = buf.String()
	t.Logf("SMITH:\n%s", out)

	for _, want := range []string{
		"SMITH       /[SMITH]/TMPMBX,NETMBX",
		"  DUA1:[SMITH]",
		`SMITH_LOGGED_IN == "YES"`,
		"MINE.COM;1           [SMITH]",
		"-RMS-E-PRV",
		"%ADDUSER-E-NOPRIV",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SMITH's session lacks %q", want)
		}
	}

	if strings.Contains(out, "Username:") {
		t.Error("ADDUSER asked SMITH for a username")
	}
}

// TestAddUserExistingHome: a home directory already there, nested, is
// given to the new account (its directory file's owner changed), its
// LOGIN.COM left alone; passwords that differ are asked for again; and a
// username already in use is asked for again.
func TestAddUserExistingHome(t *testing.T) {
	c, d, buf := addUserConsole(t)

	mustDispatch(t, d, "CREATE/DIRECTORY DUA1:[USERS.JONES]")

	c.In = strings.NewReader(strings.Join([]string{
		"system",     // in use
		"jones",      // username
		"",           // no owner
		"300",        // group
		"",           // member 1
		"one", "two", // passwords that differ
		"", "", // none
		"DUA1:",         // the home device
		"[USERS.JONES]", // the home directory
		"2",             // a list
		"tmpmbx, sysprv",
	}, "\n") + "\n")

	buf.Reset()

	if err := d.Dispatch("@SYS$MANAGER:ADDUSER"); err != nil {
		t.Errorf("ADDUSER: %v", err)
	}

	mustDispatch(t, d, "DIRECTORY/SECURITY DUA1:[USERS]JONES.DIR")

	out := buf.String()
	t.Logf("ADDUSER:\n%s", out)

	for _, want := range []string{
		"%ADDUSER-W-EXISTS, SYSTEM is already a username or an identifier",
		"%ADDUSER-W-MISMATCH",
		"%ADDUSER-I-EXISTING, DUA1:[USERS.JONES] was already there; it is now JONES's",
		"JONES.DIR;1          [JONES]",
		"Added JONES, [300,1], at home in DUA1:[USERS.JONES].",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ADDUSER's output lacks %q", want)
		}
	}

	f, err := c.RTL.System.Authorization()
	if err != nil {
		t.Fatal(err)
	}

	rec, ok := f.Find("JONES")
	if !ok || rec.Password != "" || rec.Directory != "[USERS.JONES]" || rec.UIC != uaf.NewUIC(0o300, 1) {
		t.Fatalf("JONES's record: %+v", rec)
	}

	if names := rec.Privileges.Names(); strings.Join(names, ",") != "TMPMBX,SYSPRV" {
		t.Errorf("JONES's privileges: %v", names)
	}
}

// TestTerminalNoEcho: while SET TERMINAL/NOECHO is in effect, DCL's
// reads of the terminal (INQUIRE, READ SYS$COMMAND) turn the host's echo
// off through TerminalEcho and back on after; with /ECHO they don't.
func TestTerminalNoEcho(t *testing.T) {
	c := bootedConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	var calls []string

	c.TerminalEcho = func(on bool) func() {
		calls = append(calls, "off")

		return func() { calls = append(calls, "on") }
	}

	c.In = strings.NewReader("one\ntwo\nthree\n")

	mustDispatch(t, d, "INQUIRE A", "SET TERMINAL/NOECHO", "READ SYS$COMMAND B", "SET TERMINAL/ECHO", "INQUIRE C")

	if got := strings.Join(calls, ","); got != "off,on" {
		t.Errorf("TerminalEcho calls %q, want one read without echo", got)
	}
}
