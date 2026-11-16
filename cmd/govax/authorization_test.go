package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/uaf"
)

// withAuthorizationFile gives one test an authorization file of its own,
// put back when it ends.
func withAuthorizationFile(t *testing.T, path string) {
	t.Helper()

	saved := authorizationFile
	authorizationFile = path

	t.Cleanup(func() { authorizationFile = saved })
}

// TestRun_authorizationFile: the first start makes the file with SYSTEM
// and DEFAULT, mode 0600, and the System reads it; a file govax can't
// read stops startup with the file's name and the line.
func TestRun_authorizationFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "sysuaf.json")
	withAuthorizationFile(t, path)

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	f, err := uaf.Load(path)
	if err != nil {
		t.Fatalf("the file wasn't made: %v", err)
	}

	if _, ok := f.Find("SYSTEM"); !ok {
		t.Error("no SYSTEM")
	}

	if _, ok := f.Find("DEFAULT"); !ok {
		t.Error("no DEFAULT")
	}

	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}

	data, _ := os.ReadFile(path)
	bad := strings.Replace(string(data), `"TMPMBX"`, `"NOSUCHPRIV"`, 1)

	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	err = run(nil, 0, 0, &buf, emptyStdin(), nil)

	var pe *uaf.ParseError
	if !errors.As(err, &pe) || pe.Line == 0 || !strings.Contains(err.Error(), path) {
		t.Errorf("run on a bad file = %v, want a ParseError naming %s and a line", err, path)
	}

	if strings.Contains(buf.String(), "STARTUP") {
		t.Errorf("startup went on:\n%s", buf.String())
	}
}

// withUser gives one test --user's account, put back when it ends.
func withUser(t *testing.T, name string) {
	t.Helper()

	saved := loginUser
	loginUser = name

	t.Cleanup(func() { loginUser = saved })
}

// oneShot runs command as govax's one-shot command and returns its
// output, failing the test if it fails.
func oneShot(t *testing.T, command string) string {
	t.Helper()

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), []string{command}); err != nil {
		t.Fatalf("%s: %v\n%s", command, err, buf.String())
	}

	return buf.String()
}

// TestRun_logIn: with a system volume SYSTEM's home is SYS$MANAGER,
// SYS$LOGIN and its kin in the job table and the default directory
// there; --user logs in as another account, with its UIC and privileges
// and the time of the login before; a disabled or missing account stops
// govax with LOGINOUT's message; and with no system volume SYSTEM stays
// where it was, as before accounts.
func TestRun_logIn(t *testing.T) {
	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))

	// No system volume (TestMain's): the host's directory, no SYS$LOGIN.
	if out := oneShot(t, "SHOW LOGICAL SYS$LOGIN"); !strings.Contains(out, "NOTRAN") {
		t.Errorf("SYS$LOGIN with no home:\n%s", out)
	}

	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})

	if out := oneShot(t, "SHOW DEFAULT"); !strings.Contains(out, "SYS$SYSROOT:[SYSMGR]") {
		t.Errorf("SYSTEM's default directory:\n%s", out)
	}

	for name, want := range map[string]string{
		"SYS$LOGIN":        `"SYS$LOGIN" [exec] = "SYS$SYSROOT:[SYSMGR]" (LNM$JOB_`,
		"SYS$LOGIN_DEVICE": `"SYS$LOGIN_DEVICE" [exec] = "SYS$SYSROOT:" (LNM$JOB_`,
		"SYS$SCRATCH":      `"SYS$SCRATCH" [exec] = "SYS$SYSROOT:[SYSMGR]" (LNM$JOB_`,
		"SYS$DISK":         `"SYS$DISK" [exec] = "SYS$SYSROOT:" (LNM$PROCESS_TABLE)`,
	} {
		if out := oneShot(t, "SHOW LOGICAL/FULL "+name); !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}

	var out string

	err := uaf.Update(authorizationFile, func(f *uaf.File) error {
		smith := uaf.DefaultRecord()
		smith.Username, smith.Flags, smith.Account = "SMITH", 0, "GOVAX"
		smith.UIC = uaf.NewUIC(0o300, 1)
		smith.Device, smith.Directory = "SYS$SYSROOT:", "[SYSMGR]"
		f.Put(smith)

		disabled := uaf.DefaultRecord()
		disabled.Username = "OFF"
		f.Put(disabled)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	withUser(t, "smith")

	out = oneShot(t, `WRITE SYS$OUTPUT F$GETJPI("","USERNAME"),"/",F$GETJPI("","UIC"),"/",F$GETJPI("","ACCOUNT"),"/",F$GETJPI("","CURPRIV"),"/",F$GETJPI("","PRCLM")`)
	if !strings.Contains(out, "SMITH       /[SMITH]/GOVAX   /TMPMBX,NETMBX/2") {
		t.Errorf("SMITH's process:\n%s", out)
	}

	// The first login was recorded: the second reports it.
	out = oneShot(t, `WRITE SYS$OUTPUT F$GETJPI("","LAST_LOGIN_I")`)
	if strings.Contains(out, "1858") {
		t.Errorf("the last login wasn't recorded:\n%s", out)
	}

	f, err := uaf.Load(authorizationFile)
	if err != nil {
		t.Fatal(err)
	}

	if r, _ := f.Find("SMITH"); r.LastInteractive.IsZero() {
		t.Error("no login time in SMITH's record")
	}

	for user, want := range map[string]string{"OFF": "%LOGIN-F-DISUSER, account is disabled", "NOBODY": "%LOGIN-F-NOSUCHUSER"} {
		withUser(t, user)

		var buf bytes.Buffer

		err := run(nil, 0, 0, &buf, emptyStdin(), []string{"SHOW DEFAULT"})
		if err == nil || !strings.Contains(buf.String(), want) || strings.Contains(buf.String(), "SYSROOT") {
			t.Errorf("--user %s: %v\n%s", user, err, buf.String())
		}

		if reportFailure(&buf, err) != 1 {
			t.Errorf("--user %s's exit status", user)
		}
	}
}

// TestRun_startupProcedureAsSystem: vax.startup.procedure is part of the
// system's startup, run as SYSTEM before process 1 logs in as --user's
// account, so a privileged command in it works whoever logs in.
func TestRun_startupProcedureAsSystem(t *testing.T) {
	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))

	err := uaf.Update(authorizationFile, func(f *uaf.File) error {
		smith := uaf.DefaultRecord()
		smith.Username, smith.Flags, smith.UIC = "SMITH", 0, uaf.NewUIC(0o300, 1)
		f.Put(smith)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	proc := filepath.Join(dir, "mystart.com")
	if err := os.WriteFile(proc, []byte(`$ WRITE SYS$OUTPUT "STARTUP AS ",F$GETJPI("","USERNAME"),F$PRIVILEGE("CMKRNL")`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	setConfig(t, "vax.startup.procedure", proc)
	withUser(t, "smith")

	out := oneShot(t, `WRITE SYS$OUTPUT "THEN ",F$GETJPI("","USERNAME"),F$PRIVILEGE("CMKRNL")`)

	if !strings.Contains(out, "STARTUP AS SYSTEM      TRUE") {
		t.Errorf("the startup procedure didn't run as SYSTEM:\n%s", out)
	}

	if !strings.Contains(out, "THEN SMITH       FALSE") {
		t.Errorf("the command didn't run as SMITH:\n%s", out)
	}
}

// session runs govax as an interactive session whose input is typed (not
// a terminal) and returns its output.
func session(t *testing.T, typed string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	err := run(nil, 0, 0, &buf, io.NopCloser(strings.NewReader(typed)), nil)

	return buf.String(), err
}

// TestRun_loginProcedures: a session runs the login procedures after
// the login, SYS$SYLOGIN and then SYS$LOGIN:LOGIN.COM; a one-shot command
// runs neither (docs/planning/PHASE-95 - authentication.md, Decision 8).
func TestRun_loginProcedures(t *testing.T) {
	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))
	withSystemVolume(t, systemFlags{file: filepath.Join(dir, "govax-system.dsk")})

	for name, text := range map[string]string{
		"sylogin.com": "$ LOGGED == \"SYLOGIN \" + F$STRING(F$ENVIRONMENT(\"DEPTH\"))\n",
		"login.com":   "$ LOGGED == LOGGED + \", LOGIN \" + F$STRING(F$ENVIRONMENT(\"DEPTH\"))\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}

		oneShot(t, `COPY "`+path+`"/HOST SYS$MANAGER:`+strings.ToUpper(name))
	}

	out, err := session(t, "SHOW SYMBOL/GLOBAL LOGGED\n")
	if err != nil || !strings.Contains(out, `LOGGED == "SYLOGIN 2, LOGIN 1"`) {
		t.Errorf("the session's login procedures: %v\n%s", err, out)
	}

	var buf bytes.Buffer

	err = run(nil, 0, 0, &buf, emptyStdin(), []string{"SHOW SYMBOL/GLOBAL LOGGED"})
	if err == nil || !strings.Contains(err.Error()+buf.String(), "UNDSYM") {
		t.Errorf("a one-shot command ran the login procedures: %v\n%s", err, buf.String())
	}
}

// TestRun_password: an account with a password logs in only when it is
// given: GOVAX_PASSWORD when standard input isn't a terminal, refused
// with LOGINOUT's message when wrong, and a message saying what to do
// when it isn't set. Its case doesn't matter, as on VMS.
func TestRun_password(t *testing.T) {
	dir := t.TempDir()
	withAuthorizationFile(t, filepath.Join(dir, "sysuaf.json"))

	hash, err := uaf.HashPassword("Secret")
	if err != nil {
		t.Fatal(err)
	}

	err = uaf.Update(authorizationFile, func(f *uaf.File) error {
		smith := uaf.DefaultRecord()
		smith.Username, smith.Flags, smith.UIC, smith.Password = "SMITH", 0, uaf.NewUIC(0o300, 1), hash
		f.Put(smith)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	withUser(t, "smith")

	for _, tc := range []struct {
		set, value, want string
	}{
		{"", "", "%GOVAX-F-PASSWORD, account SMITH has a password"},
		{"set", "wrong", "User authorization failure"},
	} {
		if tc.set != "" {
			t.Setenv(passwordVariable, tc.value)
		}

		var buf bytes.Buffer

		err := run(nil, 0, 0, &buf, emptyStdin(), []string{"SHOW DEFAULT"})
		if err == nil || !strings.Contains(buf.String(), tc.want) || reportFailure(&buf, err) != 1 {
			t.Errorf("%q: %v\n%s", tc.value, err, buf.String())
		}
	}

	t.Setenv(passwordVariable, "SECRET")

	if out := oneShot(t, `WRITE SYS$OUTPUT F$GETJPI("","USERNAME")`); !strings.Contains(out, "SMITH") {
		t.Errorf("with the password:\n%s", out)
	}
}

// TestCheckPassword_terminal: at a terminal govax asks three times, each
// wrong answer LOGINOUT's message, and never looks at GOVAX_PASSWORD.
func TestCheckPassword_terminal(t *testing.T) {
	hash, err := uaf.HashPassword("SECRET")
	if err != nil {
		t.Fatal(err)
	}

	r := uaf.DefaultRecord()
	r.Username, r.Password = "SMITH", hash

	t.Setenv(passwordVariable, "SECRET")

	for _, tc := range []struct {
		typed    []string
		failures int
		ok       bool
	}{
		{[]string{"secret"}, 0, true},
		{[]string{"no", "nope", "secret"}, 2, true},
		{[]string{"no", "nope", "never", "secret"}, 3, false},
	} {
		typed := tc.typed

		saved := passwordInput
		passwordInput = func(io.Writer) (passwordReader, bool) {
			return func() (string, error) {
				next := typed[0]
				typed = typed[1:]

				return next, nil
			}, true
		}

		var buf bytes.Buffer

		err := checkPassword(r, &buf, true)
		passwordInput = saved

		out := buf.String()
		if (err == nil) != tc.ok || strings.Count(out, "User authorization failure") != tc.failures ||
			strings.Count(out, "Password: ") != min(len(tc.typed), passwordTries) {
			t.Errorf("%v: %v\n%s", tc.typed, err, out)
		}
	}

	// An account with no password is asked for nothing.
	r.Password = ""

	var buf bytes.Buffer
	if err := checkPassword(r, &buf, true); err != nil || buf.Len() != 0 {
		t.Errorf("no password: %v, %q", err, buf.String())
	}
}
