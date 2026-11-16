package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestAuthorizeFromDCL runs AUTHORIZE every way DCL reaches it
// (dclshell/authorize.go): the verb with one command, MCR AUTHORIZE
// reading a session's commands from the procedure's data lines, RUN
// SYS$SYSTEM:AUTHORIZE, and a foreign command; each leaves AUTHORIZE's
// $STATUS, and what they did is in the System's authorization file.
func TestAuthorizeFromDCL(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "auth.com",
		"$ SET NOON",
		"$ AUTHORIZE ADD SMITH/UIC=[200,1]/FLAGS=NODISUSER",
		"$ SHOW SYMBOL $STATUS",
		"$ AUTHORIZE ADD SMITH",
		"$ SHOW SYMBOL $STATUS",
		"$ MCR AUTHORIZE",
		"MODIFY SMITH/ASTLM=60",
		"FROB",
		"EXIT",
		"$ SHOW SYMBOL $STATUS",
		"$ RUN SYS$SYSTEM:AUTHORIZE",
		"COPY SMITH JONES/UIC=[200,2]",
		"$ UAF := $SYS$SYSTEM:AUTHORIZE.EXE",
		"$ UAF RENAME JONES BROWN",
		`$ PRINT "done"`,
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@auth: %v", err)
	}

	want := `%UAF-I-ADDMSG, user record successfully added
%UAF-I-RDBADDMSGU, identifier SMITH value [000200,000001] added to rights database
  $STATUS == "%X00000001"
%UAF-E-UAEERR, invalid user name, user name already exists
  $STATUS == "%X13F1943A"
%UAF-I-MDFYMSG, user record(s) updated
%CLI-W-IVVERB, unrecognized command verb - check validity and spelling
 \FROB\
%UAF-I-DONEMSG, system authorization file modified
%UAF-I-RDBNOMODS, no modifications made to rights database
  $STATUS == "%X00000001"
%UAF-I-COPMSG, user record copied
%UAF-W-DEFPWD, copied or renamed records must receive new password
%UAF-I-RDBADDMSGU, identifier JONES value [000200,000002] added to rights database
%UAF-I-DONEMSG, system authorization file modified
%UAF-I-RDBDONEMSG, rights database modified
%UAF-I-RENMSG, user record renamed
%UAF-W-DEFPWD, copied or renamed records must receive new password
%UAF-I-RDBMDFYMSG, identifier JONES modified
done
`
	if got := buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	f, err := c.RTL.System.Authorization()
	if err != nil {
		t.Fatal(err)
	}

	smith, _ := f.Find("SMITH")
	brown, ok := f.Find("BROWN")

	if smith == nil || smith.Quotas.ASTLM != 60 || !ok || brown.UIC != uaf.NewUIC(0o200, 2) {
		t.Errorf("accounts: SMITH %+v, BROWN %+v", smith, brown)
	}

	if id, ok := f.IdentifierNamed("BROWN"); !ok || id.Value != uaf.IdentifierValue(uaf.NewUIC(0o200, 2)) {
		t.Errorf("identifier BROWN: %v", id)
	}
}

// TestAuthorizeNeedsAccess: a process that isn't in the system category
// and lacks SYSPRV gets NAOFIL, as VMS 7.1 told the probe's UAF95.
func TestAuthorizeNeedsAccess(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	c.RTL.Process.UIC = uint32(uaf.NewUIC(0o200, 5))
	c.RTL.Process.CurrentPrivileges = 0

	mustDispatch(t, d, "AUTHORIZE ADD SMITH")

	if got := buf.String(); !strings.HasPrefix(got, "%UAF-E-NAOFIL") || !strings.Contains(got, "-RMS-E-PRV") {
		t.Errorf("got\n%s", got)
	}
}

// TestAuthorizeListAndRights: LIST writes SYSUAF.LIS and RIGHTSLIST.LIS in
// the default directory, a volume's here, and an identifier granted to an
// account's UIC is among the rights its login gives (Decision 11).
func TestAuthorizeListAndRights(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA1:")

	for _, line := range []string{
		"SET DEFAULT DUA1:[000000]",
		"AUTHORIZE ADD SMITH/UIC=[200,1]/OWNER=\"JOE SMITH\"/FLAGS=NODISUSER",
		"AUTHORIZE ADD/IDENTIFIER PROJX",
		"AUTHORIZE GRANT/IDENTIFIER PROJX SMITH",
		"AUTHORIZE LIST SMITH",
		"AUTHORIZE LIST/RIGHTS SMITH",
	} {
		mustDispatch(t, d, line)
	}

	buf.Reset()
	mustDispatch(t, d, "TYPE SYSUAF.LIS")
	mustDispatch(t, d, "TYPE RIGHTSLIST.LIS")

	want := `       Owner         Username           UIC       Account  Privs Pri Directory

JOE SMITH            SMITH           [200,1]               Normal  4 [USER]
Identifier                         Value           Attributes
  PROJX                            %X80010000      
`
	if got := buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	l, err := c.RTL.System.BeginLogin("SMITH", true)
	if err != nil {
		t.Fatal(err)
	}

	if len(l.Rights) != 1 || l.Rights[0].Value != uaf.FirstGeneral {
		t.Errorf("SMITH's login holds %+v", l.Rights)
	}
}

// TestAuthorizeHelp: AUTHORIZE's HELP reads UAFHELP.HLB (govax's own copy
// without a system volume), as one command and in a session, where its
// prompts read the procedure's data lines and Return goes back to UAF>.
func TestAuthorizeHelp(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	mustDispatch(t, d, "AUTHORIZE HELP SHOW /BRIEF")

	if out := buf.String(); !strings.Contains(out, "SHOW") || !strings.Contains(out, "A line for each account") {
		t.Errorf("AUTHORIZE HELP SHOW /BRIEF:\n%s", out)
	}

	proc := writeProcedure(t, t.TempDir(), "uafhelp.com",
		"$ AUTHORIZE", "HELP", "GRANT", "", "", "EXIT")

	buf.Reset()
	mustDispatch(t, d, "@"+proc)

	out := buf.String()
	for _, want := range []string{"Additional information available:", "REVOKE", "GRANT/IDENTIFIER lets", "%UAF-I-NOMODS"} {
		if !strings.Contains(out, want) {
			t.Errorf("the session lacks %q:\n%s", want, out)
		}
	}
}

// TestHelpAuthAbbreviated: HELP AUTH is AUTHORIZE's topic alone, since the
// topic that told who wrote govax is GOVAX now, with its credits.
func TestHelpAuthAbbreviated(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	mustDispatch(t, d, "HELP/NOPROMPT AUTH")

	if out := buf.String(); !strings.Contains(out, "Makes and changes the accounts") || strings.Contains(out, "Forest Edge") {
		t.Errorf("HELP AUTH:\n%s", out)
	}

	buf.Reset()
	mustDispatch(t, d, "HELP/NOPROMPT GOVAX CREDITS")

	if out := buf.String(); !strings.Contains(out, "Forest Edge Software") {
		t.Errorf("HELP GOVAX CREDITS:\n%s", out)
	}
}
