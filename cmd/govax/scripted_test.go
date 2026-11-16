package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/uaf"
)

// addUserScript is a session that runs SYS$MANAGER:ADDUSER.COM with its
// answers on the lines after it, as typed ahead or sent down a pipe, then
// @ of a logical name that names a procedure without its type, and shows
// what both did.
var addUserScript = strings.Join([]string{
	"@SYS$MANAGER:ADDUSER",
	"smith",      // username
	"John Smith", // owner
	"",           // group 200
	"",           // the first free member, 1
	"Secret",     // password
	"Secret",     // again
	"",           // the home device
	"",           // [SMITH]
	"",           // a plain user
	"DEFINE TELLME SYS$MANAGER:TELL",
	"@TELLME",
	"SHOW SYMBOL/GLOBAL TOLD",
	"EXIT",
}, "\n") + "\n"

// scriptedSystem gives the test a system volume and an authorization file
// of its own, with SYS$MANAGER:TELL.COM on the volume.
func scriptedSystem(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	authorization := filepath.Join(dir, "sysuaf.json")
	withAuthorizationFile(t, authorization)
	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})

	tell := filepath.Join(dir, "tell.com")
	if err := os.WriteFile(tell, []byte("$ TOLD == \"YES\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oneShot(t, `COPY "`+tell+`"/HOST SYS$MANAGER:TELL.COM`)

	return authorization
}

// checkScripted checks a session of addUserScript: ADDUSER read its answers
// from the lines after it, none of which DCL took for a command, and
// @TELLME found TELL.COM.
func checkScripted(t *testing.T, authorization, out string) {
	t.Helper()

	for _, want := range []string{
		"%UAF-I-ADDMSG, user record successfully added",
		"Added SMITH, [200,1]",
		`TOLD == "YES"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the session's output lacks %q", want)
		}
	}

	for _, unwanted := range []string{"IVVERB", "ADDUSER-I-STOPPED", "Secret"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("the session's output has %q", unwanted)
		}
	}

	f, err := uaf.Load(authorization)
	if err != nil {
		t.Fatal(err)
	}

	if rec, ok := f.Find("SMITH"); !ok || rec.Owner != "John Smith" {
		t.Errorf("SMITH's record: %+v, %v", rec, ok)
	}

	if t.Failed() {
		t.Logf("session:\n%s", out)
	}
}

// TestRun_scriptedAnswers: a session's input given to run (a test's
// reader) is shared by the command lines and a procedure's INQUIREs, so
// ADDUSER's questions take the lines that follow @ADDUSER, not end of file
// (readline's buffered reader once took the whole script).
func TestRun_scriptedAnswers(t *testing.T) {
	authorization := scriptedSystem(t)

	out, err := session(t, addUserScript)
	if err != nil {
		t.Fatal(err)
	}

	checkScripted(t, authorization, out)
}

// TestRun_pipedAnswers is TestRun_scriptedAnswers with the script on
// govax's standard input, a pipe, as `govax < script` gives it.
func TestRun_pipedAnswers(t *testing.T) {
	authorization := scriptedSystem(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString(addUserScript)
		_ = w.Close()
	}()

	saved := os.Stdin
	os.Stdin = r

	t.Cleanup(func() {
		os.Stdin = saved
		_ = r.Close()
	})

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, nil, nil); err != nil {
		t.Fatal(err)
	}

	checkScripted(t, authorization, buf.String())
}
