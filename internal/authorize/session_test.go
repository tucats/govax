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

// TestSession is the probe's "A SESSION" (testdata/uaf95's log) but for
// its SHOW commands, which are subtask 6's: an unknown verb, an ADD and a
// REMOVE, then EXIT, and both files said to have changed.
func TestSession(t *testing.T) {
	out, _ := session(t, &memoryStore{f: uaf.Default()}, security.SystemCredentials(),
		"FROB", "ADD UAFS/UIC=[300,40]", "REMOVE UAFS", "EXIT")

	want := `%CLI-W-IVVERB, unrecognized command verb - check validity and spelling
 \FROB\
%UAF-I-ADDMSG, user record successfully added
%UAF-I-RDBADDMSGU, identifier UAFS value [000300,000040] added to rights database
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
// probe's second session).
func TestSessionEndOfInput(t *testing.T) {
	out, prompts := session(t, &memoryStore{f: uaf.Default()}, security.SystemCredentials(), "", "! a comment")

	want := "%UAF-I-NOMODS, no modifications made to system authorization file\n" +
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
