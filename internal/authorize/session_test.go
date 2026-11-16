package authorize

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
)

// session runs a UAF> session reading lines, and returns what it wrote
// and the prompts it gave.
func session(t *testing.T, store Store, creds security.Credentials, lines ...string) (string, int) {
	t.Helper()

	var out strings.Builder

	prompts := 0
	u := &Utility{
		Store: store, Credentials: creds, Out: &out,
		Read: func(p string) (string, bool) {
			if p != "UAF> " {
				t.Errorf("prompt %q", p)
			}

			prompts++

			if len(lines) == 0 {
				return "", false
			}

			line := lines[0]
			lines = lines[1:]

			return line, true
		},
	}

	if status := u.Run(""); status != ssNormal {
		t.Errorf("session's status %%X%08X", status)
	}

	return out.String(), prompts
}

// TestSession is the probe's "A SESSION" (testdata/uaf95's log): a SHOW
// matching nothing, an unknown verb, an ADD, a SHOW/BRIEF, and a REMOVE,
// then EXIT, and both files said to have changed.
func TestSession(t *testing.T) {
	out, _ := session(t, &memoryStore{f: uaf.Default()}, security.SystemCredentials(),
		"SHOW NOSUCH", "FROB", "ADD UAFS/UIC=[300,40]", "SHOW UAFS/BRIEF", "REMOVE UAFS", "EXIT")

	want := `%UAF-W-BADSPC, no user matches specification
%CLI-W-IVVERB, unrecognized command verb - check validity and spelling
 \FROB\
%UAF-I-ADDMSG, user record successfully added
%UAF-I-RDBADDMSGU, identifier UAFS value [000300,000040] added to rights database
       Owner         Username           UIC       Account  Privs Pri Directory

                     UAFS            [300,40]              Normal  4 Disuser
%UAF-I-REMMSG, record removed from system authorization file
%UAF-I-RDBREMMSGU, identifier UAFS value [000300,000040] removed from rights database
%UAF-I-DONEMSG, system authorization file modified
%UAF-I-RDBDONEMSG, rights database modified
`
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

// TestSessionEndOfInput: a session ends at the end of its input, as EXIT
// ends it, and one that changed nothing says so for both files (the
// probe's second session, with its SHOW SYSTEM/BRIEF).
func TestSessionEndOfInput(t *testing.T) {
	out, prompts := session(t, &memoryStore{f: uaf.Default()}, security.SystemCredentials(), "SHOW SYSTEM/BRIEF", "! a comment")

	want := "       Owner         Username           UIC       Account  Privs Pri Directory\n\n" +
		"SYSTEM MANAGER       SYSTEM            [1,4]      SYSTEM   All     4 SYS$SYSROOT:[SYSMGR]\n" +
		"%UAF-I-NOMODS, no modifications made to system authorization file\n" +
		"%UAF-I-RDBNOMODS, no modifications made to rights database\n"
	if out != want || prompts != 3 {
		t.Errorf("got %d prompts and\n%s", prompts, out)
	}
}

// TestSessionChangesAccountsOnly: a change to an account alone modifies
// the file but not the rights database.
func TestSessionChangesAccountsOnly(t *testing.T) {
	out, _ := session(t, &memoryStore{f: uaf.Default()}, security.SystemCredentials(), "MODIFY SYSTEM/ASTLM=60", "EXI")

	if !strings.HasSuffix(out, "%UAF-I-DONEMSG, system authorization file modified\n"+
		"%UAF-I-RDBNOMODS, no modifications made to rights database\n") {
		t.Errorf("got\n%s", out)
	}
}

// TestNoAccess: a process outside the system category without SYSPRV
// may not open the file: NAOFIL with RMS's PRV, as VMS 7.1 told the
// probe's UAF95 account, $STATUS SS$_NORMAL, and the session never
// starts.
func TestNoAccess(t *testing.T) {
	store := &memoryStore{f: uaf.Default()}
	user := security.Credentials{UIC: security.NewUIC(0o200, 5)}

	var out strings.Builder

	u := &Utility{Store: store, Credentials: user, Out: &out}
	if status := u.Run("ADD SMITH/UIC=[200,1]"); status != ssNormal {
		t.Errorf("status %%X%08X", status)
	}

	want := "%UAF-E-NAOFIL, unable to open system authorization file (SYSUAF.DAT)\n" +
		"-RMS-E-PRV, insufficient privilege or file protection violation\n"
	if out.String() != want {
		t.Errorf("got\n%s", out.String())
	}

	if _, ok := store.f.Find("SMITH"); ok {
		t.Error("SMITH was added")
	}

	text, prompts := session(t, store, user, "EXIT")
	if text != want || prompts != 0 {
		t.Errorf("session: %d prompts and\n%s", prompts, text)
	}

	// SYSPRV, or a system group's UIC, gives access.
	sysprv := user
	sysprv.Privileges = security.PrivSYSPRV

	out.Reset()

	u = &Utility{Store: store, Credentials: sysprv, Out: &out}
	if u.Run("ADD SMITH/UIC=[200,1]"); !strings.Contains(out.String(), "ADDMSG") {
		t.Errorf("with SYSPRV:\n%s", out.String())
	}
}

// TestModifyFields: the fields the probe's cases set, read back from the
// file, and the password kept as uaf's hash.
func TestModifyFields(t *testing.T) {
	store := &memoryStore{f: uaf.Default()}
	run := func(command string) string {
		var out strings.Builder

		(&Utility{Store: store, Credentials: security.SystemCredentials(), Out: &out}).Run(command)

		return out.String()
	}

	run(`ADD SMITH/UIC=[200,1]/OWNER="John Smith"/PASSWORD=Secret1/FLAGS=NODISUSER/PRIVILEGES=(NOALL,TMPMBX)`)

	r, _ := store.f.Find("SMITH")
	if r.Owner != "John Smith" || r.Flags != 0 || strings.Join(r.Privileges.Names(), ",") != "TMPMBX" {
		t.Errorf("SMITH is %+v", *r)
	}

	if ok, err := uaf.CheckPassword(r.Password, "secret1"); !ok || err != nil {
		t.Errorf("the password %q doesn't check: %v", r.Password, err)
	}

	if out := run("MODIFY SMITH/CPUTIME=1-02:03:04.5/PRIORITY=%X1F/PRIVILEGES=ALL"); out != "%UAF-I-MDFYMSG, user record(s) updated\n" {
		t.Errorf("MODIFY said\n%s", out)
	}

	r, _ = store.f.Find("SMITH")
	if r.Quotas.CPUTIME != ((26*60+3)*60+4)*100+50 || r.Priority != 31 || r.Privileges != uaf.AllPrivileges {
		t.Errorf("SMITH is %+v", *r)
	}

	for command, want := range map[string]string{
		"MODIFY SMITH/CPUTIME=25:00":       "%UAF-E-BADVALUE, error in value specification \\25:00\\\n",
		"MODIFY SMITH/ACCOUNT=TOOLONGACC":  "%UAF-E-VALTOOBIG, value too large for field \\TOOLONGACC\\\n",
		"MODIFY SMITH/NOPASSWORD":          "%UAF-I-PWDLESSMIN, new password is shorter than minimum password length\n%UAF-I-MDFYMSG, user record(s) updated\n",
		"MODIFY SMITH/PRIVILEGES=NONESUCH": "%CLI-W-IVKEYW, unrecognized keyword - check validity and spelling\n \\NONESUCH\\\n",
	} {
		if out := run(command); out != want {
			t.Errorf("%s said\n%s\nwant\n%s", command, out, want)
		}
	}

	if r, _ = store.f.Find("SMITH"); r.Password != "" {
		t.Errorf("SMITH still has a password")
	}
}

// TestWildcardMatch: % is one character, * any run.
func TestWildcardMatch(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"UAF%", "UAFA", true}, {"UAF%", "UAF", false}, {"UAF%", "UAFAB", false},
		{"UAF*", "UAF", true}, {"UAF*", "UAFAB", true}, {"*", "SYSTEM", true},
		{"S*M", "SYSTEM", true}, {"S*M", "SYSTEMS", false}, {"SYSTEM", "SYSTEM", true},
	} {
		if got := wildcardMatch(c.pattern, c.name); got != c.want {
			t.Errorf("%s matching %s: %v", c.pattern, c.name, got)
		}
	}
}

// TestPrivilegeCategory: the brief report's word for each category, the
// most powerful one an account has.
func TestPrivilegeCategory(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "None"}, {[]string{"TMPMBX", "NETMBX"}, "Normal"}, {[]string{"TMPMBX", "GRPPRV"}, "Group"},
		{[]string{"GROUP", "EXQUOTA"}, "Devour"}, {[]string{"OPER", "TMPMBX"}, "System"},
		{[]string{"READALL"}, "Objects"}, {[]string{"SYSPRV", "TMPMBX"}, "All"}, {[]string{"ALL"}, "All"},
	} {
		p, err := uaf.ParsePrivileges(c.names)
		if err != nil {
			t.Fatal(err)
		}

		if got := privilegeCategory(p); got != c.want {
			t.Errorf("%v: %s, want %s", c.names, got, c.want)
		}
	}
}

// TestHoldersByUIC: a holder record is a UIC's, so every account with the
// UIC holds the identifier (SHOW/FULL lists it), a UIC identifier's new
// value takes its holder records along ($MOD_IDENT), and removing the
// account's identifier drops them ($REM_IDENT).
func TestHoldersByUIC(t *testing.T) {
	store := &memoryStore{f: uaf.Default()}
	run := func(command string) string {
		var out strings.Builder

		(&Utility{Store: store, Credentials: security.SystemCredentials(), Out: &out}).Run(command)

		return out.String()
	}

	run("ADD SMITH/UIC=[300,1]")
	run("COPY SMITH JONES")
	run("ADD/IDENTIFIER PROJX/ATTRIBUTES=RESOURCE")

	if out := run("GRANT/IDENTIFIER/ATTRIBUTES=(RESOURCE,DYNAMIC) PROJX SMITH"); out != "%UAF-I-GRANTMSG, identifier PROJX granted to SMITH\n" {
		t.Errorf("GRANT said\n%s", out)
	}

	if out := run("SHOW JONES/FULL"); !strings.HasSuffix(out, "Identifier                         Value           Attributes\n"+
		"  PROJX                            %X80010000      RESOURCE \n") {
		t.Errorf("JONES/FULL, holding PROJX by SMITH's UIC with RESOURCE alone:\n%s", out)
	}

	run("MODIFY SMITH/UIC=[300,2]")

	if out := run("SHOW/RIGHTS SMITH"); !strings.Contains(out, "PROJX") {
		t.Errorf("SMITH's rights after MODIFY/UIC:\n%s", out)
	}

	if out := run("SHOW JONES/FULL"); strings.Contains(out, "PROJX") {
		t.Errorf("JONES, still [300,1], holds PROJX:\n%s", out)
	}

	run("REMOVE SMITH")

	if id, _ := store.f.IdentifierNamed("PROJX"); len(id.Holders) != 0 {
		t.Errorf("PROJX's holders after REMOVE SMITH: %+v", id.Holders)
	}

	// In order: each depends on the one before.
	for _, c := range []struct{ command, want string }{
		{"ADD/IDENTIFIER INTERACTIVE", "%UAF-E-RDBADDERRU, unable to add INTERACTIVE value [000000,000000] to rights database\n-SYSTEM-F-DUPLNAM, duplicate name\n"},
		{"REMOVE/IDENTIFIER BATCH", "%UAF-E-RDBREMERR, unable to remove BATCH from rights database\n-SYSTEM-F-NOPRIV, insufficient privilege or object protection violation\n"},
		{"MODIFY/IDENTIFIER PROJX/NAME=PROJY/ATTRIBUTES=NORESOURCE", "%UAF-I-RDBMDFYMSG, identifier PROJX modified\n"},
		{"RENAME/IDENTIFIER PROJY PROJZ", "%UAF-I-RDBMDFYMSG, identifier PROJY modified\n"},
		{"SHOW/IDENTIFIER/VALUE=UIC=[300,*]", "%UAF-E-SHOWERR, unable to complete SHOW command\n-SYSTEM-F-NOSUCHID, unknown rights identifier\n"},
		{"ADD/IDENTIFIER GRP300/VALUE=UIC=[300,*]", "%UAF-I-RDBADDMSGU, identifier GRP300 value [000300,177777] added to rights database\n"},
		{"SHOW/IDENTIFIER/VALUE=UIC=[300,*]", "  Name                             Value           Attributes\n  GRP300                           [000300,177777] \n"},
		{"ADD/IDENTIFIER BIG/VALUE=IDENTIFIER=%X10000000", "%UAF-E-BADVALUE, error in value specification \\268435456\\\n"},
		{"ADD/IDENTIFIER TAKEN/VALUE=IDENTIFIER=%X10000", "%UAF-E-RDBADDERRU, unable to add TAKEN value %X80010000 to rights database\n-SYSTEM-F-DUPIDENT, duplicate identifier\n"},
		{"ADD/IDENTIFIER OK/VALUE=IDENTIFIER=%X10005", "%UAF-I-RDBADDMSG, identifier OK value %X80010005 added to rights database\n"},
		{"SHOW/IDENTIFIER PROJZ", "  Name                             Value           Attributes\n  PROJZ                            %X80010000      \n"},
	} {
		if out := run(c.command); out != c.want {
			t.Errorf("%s said\n%s\nwant\n%s", c.command, out, c.want)
		}
	}
}
