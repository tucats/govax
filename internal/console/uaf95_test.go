package console_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestUAF95Probe rehearses Phase 95's probe (testdata/uaf95) on govax, so
// the VAX round can't be lost to a mistake in the probe's own DCL: the
// login procedures record what they see in global symbols, and UAF95I.COM
// shows them, the process, and every F$GETJPI item, a long value in
// 100-character lines. Here SYSTEM runs them by @, without logging in
// (TestUAF95LoginOracle logs in as the probe's accounts); govax has no
// SUBMIT, and those lines fail with messages and the procedures go on.
func TestUAF95Probe(t *testing.T) {
	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.ReplaceAll(buf.String(), "\r", "")
	}

	dsk := filepath.Join(t.TempDir(), "uaf95.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 UAF95`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("DEFINE/SYSTEM/EXECUTIVE_MODE UAF95$DISK DUA1:")
	dispatch("CREATE/DIRECTORY DUA1:[UAF95LOG]")
	dispatch("CREATE/DIRECTORY DUA1:[UAF95]")
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, f := range []struct{ name, dir string }{
		{"uaf95i.com", "[000000]"}, {"uaf95s.com", "[000000]"},
		{"login.com", "[UAF95]"}, {"uaf95x.com", "[UAF95]"},
	} {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "uaf95", f.name))
		if err != nil {
			t.Fatal(err)
		}

		if out := dispatch(`COPY "` + src + `"/HOST DUA1:` + f.dir + strings.ToUpper(f.name) + "/QUIET"); out != "" {
			t.Fatalf("COPY %s: %s", f.name, out)
		}
	}

	// As a login would run them: SYLOGIN (which, for SYSTEM, isn't one of
	// the probe's accounts, records nothing), then LOGIN.COM, then the
	// LGICMD, which fails partway.
	dispatch("@UAF95$DISK:[000000]UAF95S")
	dispatch("@UAF95$DISK:[UAF95]LOGIN")
	dispatch("@UAF95$DISK:[UAF95]UAF95X")

	out := dispatch("@UAF95$DISK:[000000]UAF95I NOLOG")

	for _, want := range []string{
		`UAF95_ORDER == ",LOGIN,LGICMD"`,
		`UAF95_LOGIN_MODE == "INTERACTIVE"`,
		`UAF95_LGICMD_BEFORE == "YES"`,
		`UAF95_LGICMD_AFTER_WARNING == "YES"`,
		"  USERNAME = SYSTEM",
		"  UIC = [SYSTEM]",
		"  AUTHPRIV = CMKRNL,CMEXEC,",
		"    ETPRV,TMPMBX,",
		"  WSSIZE = ",
		"UAF95I FINISHED",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("UAF95I's output lacks %q:\n%s", want, out)
		}
	}

	// A warning let the LGICMD procedure go on, and an error ended it, as
	// DCL's default ON action does.
	if strings.Contains(out, "UAF95_LGICMD_AFTER_ERROR") {
		t.Errorf("UAF95X.COM went on after its failing command:\n%s", out)
	}

	if strings.Contains(out, "TKNOVF") {
		t.Errorf("a value was too long to write:\n%s", out)
	}
}
