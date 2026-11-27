package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmserrors"
)

// bat98Files are testdata/bat98's procedures and sources, and where the
// exchange volume has them (exchange.cmd).
var bat98Files = []struct{ name, dir string }{
	{"bat98.com", "[000000]"}, {"bat98b.com", "[000000]"}, {"bat98acct.com", "[000000]"}, {"bat98drop.com", "[000000]"},
	{"bat98z.com", "[000000]"}, {"bat98j.com", "[000000]"}, {"bat98p.com", "[000000]"},
	{"bat98ok.com", "[000000]"}, {"bat98wrn.com", "[000000]"}, {"bat98err.com", "[000000]"},
	{"bat98fat.com", "[000000]"}, {"bat98bad.com", "[000000]"}, {"bat98w.com", "[000000]"},
	{"bat98c.com", "[000000]"}, {"bat98o.com", "[000000]"}, {"bat98r.com", "[000000]"},
	{"bat98q.mar", "[000000]"}, {"def_sjc.mar", "[000000]"}, {"def_qui.mar", "[000000]"},
	{"def_jbc.mar", "[000000]"},
	{"login_bat98.com", "[BAT98]"}, {"login_bat98o.com", "[BAT98O]"}, {"login_bat98w.com", "[BAT98W]"},
}

// TestBat98Probe rehearses Phase 98's probe (testdata/bat98) on govax, so
// that a VAX round isn't lost to a mistake in its own DCL: the job's view
// of itself, the protection matrix's procedure, and SYSTEM's whole run.
// govax has no queues yet, so every queue command is a message here; what
// is checked is that each procedure reaches its end, and that its own
// lines (symbols, labels, substitution) are right. BAT98_DUMP names a
// file to write SYSTEM's log to, for reading.
func TestBat98Probe(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	
	if testing.Short() {
		t.Skip("the probe's whole run is long")
	}

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

	dsk := filepath.Join(t.TempDir(), "bat98.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 BAT98`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	for _, dir := range []string{"[BAT98LOG]", "[BAT98SCR]", "[BAT98]", "[BAT98O]", "[BAT98W]"} {
		dispatch("CREATE/DIRECTORY DUA1:" + dir)
	}

	for _, f := range bat98Files {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "bat98", f.name))
		if err != nil {
			t.Fatal(err)
		}

		to := strings.ToUpper(f.name)
		if strings.HasPrefix(to, "LOGIN_") {
			to = "LOGIN.COM"
		}

		if out := dispatch(`COPY "` + src + `"/HOST DUA1:` + f.dir + to + "/QUIET"); out != "" {
			t.Fatalf("COPY %s: %s", f.name, out)
		}
	}

	dispatch("DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:")
	dispatch("SET DEFAULT DUA1:[000000]")

	// A batch job's view of itself, run here at the terminal.
	out := dispatch(`@BAT98$DISK:[000000]BAT98J ALPHA "two words" 3 "" E F G "H h"`)
	for _, want := range []string{
		"######## BAT98J SYSTEM",
		"P2 [two words] 9",
		"P8 [H h] 3",
		"  DEPTH = 1",
		"  USERNAME = SYSTEM",
		"BAT98J FINISHED",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BAT98J's output lacks %q:\n%s", want, out)
		}
	}

	// The matrix's procedure as SYSTEM: one pass, every command reached.
	out = dispatch(`@BAT98$DISK:[000000]BAT98P A "12,13" 11`)
	for _, want := range []string{
		"######## PASS NONE AS SYSTEM STATE A HELD 12 RETAINED 11 QUEUE BAT98_D1",
		"== NONE 31",
		"BAT98P FINISHED",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BAT98P's output lacks %q:\n%s", want, out)
		}
	}

	dispatch("SET PROCESS/PRIVILEGES=ALL")

	// SYSTEM's run, to its end; its log is on the volume.
	out = dispatch("@BAT98$DISK:[000000]BAT98")
	if !strings.Contains(out, "BAT98 finished; the log is") {
		t.Fatalf("BAT98 didn't finish:\n%s", out)
	}

	log := dispatch("TYPE BAT98$DISK:[BAT98LOG]BAT98.LOG")

	if name := os.Getenv("BAT98_DUMP"); name != "" {
		if err := os.WriteFile(name, []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(log, "BAT98 FINISHED") {
		t.Errorf("BAT98.LOG doesn't reach its end:\n%s", log)
	}

	// BAT98ACCT.COM's AUTHORIZE commands are govax's as well as VMS's, so
	// the later subtasks' oracle tests can make the same accounts.
	for _, want := range []string{
		"identifier BAT98O value [000200,000011] added to rights database",
		"identifier BAT98W value [000300,000010] added to rights database",
		"identifier BAT98_ID granted to BAT98",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("BAT98.LOG lacks %q", want)
		}
	}

	bat98Clean(t, "BAT98.LOG", log)

	// Round 2, after round 1, as on the VAX.
	out = dispatch("@BAT98$DISK:[000000]BAT98B")
	if !strings.Contains(out, "BAT98B finished; the log is") {
		t.Fatalf("BAT98B didn't finish:\n%s", out)
	}

	// BAT98B.COM deassigns BAT98$DISK as it ends.
	log = dispatch("TYPE DUA1:[BAT98LOG]BAT98B.LOG")
	if !strings.Contains(log, "BAT98B FINISHED") {
		t.Errorf("BAT98B.LOG doesn't reach its end:\n%s", log)
	}

	bat98Clean(t, "BAT98B.LOG", log)
}

// bat98Clean checks the probe's own lines in a log: no symbol it uses is
// undefined, no label it goes to is missing, and no line is too long.
func bat98Clean(t *testing.T, name, log string) {
	t.Helper()

	// Case 83's /PARAMETERS value of 260 characters is DCL's TKNOVF on
	// purpose, as it was on VMS 7.1 (govax gives it since subtask 7).
	if i := strings.Index(log, strings.Repeat("X", 260)+") "); i >= 0 {
		if j := strings.Index(log[i:], "TKNOVF"); j >= 0 && j < 400 {
			log = log[:i+j] + log[i+j+len("TKNOVF"):]
		}
	}

	for _, bad := range []string{"UNDSYM", "USGOTO", "NOLBLS", "TKNOVF", "RTB,"} {
		if strings.Contains(log, bad) {
			t.Errorf("%s has %s:\n%s", name, bad, around(log, bad))
		}
	}
}

// around is the text near the first occurrence of what in s.
func around(s, what string) string {
	i := strings.Index(s, what)
	if i < 0 {
		return ""
	}

	from, to := max(0, i-600), min(len(s), i+300)

	return s[from:to]
}
