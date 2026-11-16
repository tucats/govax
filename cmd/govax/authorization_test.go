package main

import (
	"bytes"
	"errors"
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
