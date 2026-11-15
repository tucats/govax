package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestSetTerminal: SET TERMINAL changes the characteristics its qualifiers
// name and no others, as F$GETDVI then reads them; /PERMANENT without
// LOG_IO or PHY_IO changes nothing; a device that isn't a terminal is
// NOTSET (docs/planning/PHASE-94 - privileges.md, subtask 6).
func TestSetTerminal(t *testing.T) {
	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r", ""))
	}

	dvi := func(item string) string {
		t.Helper()

		return dispatch(`WRITE SYS$OUTPUT F$GETDVI("TT:","` + item + `")`)
	}

	steps := []struct {
		command string
		item    string
		want    string
	}{
		{"SET TERMINAL/WIDTH=132", "DEVBUFSIZ", "132"},
		{"SET TERMINAL/PAGE=48", "TT_PAGE", "48"},
		{"SET TERMINAL/NOECHO", "TT_NOECHO", "TRUE"},
		{"SET TERMINAL/ECHO", "TT_NOECHO", "FALSE"},
		{"SET TERMINAL/UPPERCASE", "TT_LOWER", "FALSE"},
		{"SET TERMINAL/LOWERCASE", "TT_LOWER", "TRUE"},
		{"SET TERMINAL/ANSI_CRT", "TT_ANSICRT", "TRUE"},
		{"SET TERMINAL/NOANSI_CRT TT:", "TT_ANSICRT", "FALSE"},
		{"SET TERMINAL/DEVICE_TYPE=VT52", "DEVTYPE", "64"},
		{"SET TERMINAL/DEVICE_TYPE=VT100/PERMANENT", "DEVTYPE", "96"},
		{"SET TERMINAL/WIDTH=80 SYS$COMMAND", "DEVBUFSIZ", "80"},
	}

	for _, s := range steps {
		if out := dispatch(s.command); out != "" {
			t.Errorf("%s: %s", s.command, out)
		}

		if got := dvi(s.item); got != s.want {
			t.Errorf("after %s, %s is %s, want %s", s.command, s.item, got, s.want)
		}
	}

	// The page length and width survived the other changes.
	if got := dvi("TT_PAGE"); got != "48" {
		t.Errorf("TT_PAGE %s, want 48", got)
	}

	dispatch(`X = F$SETPRV("NOLOG_IO,NOPHY_IO")`)

	out := dispatch("SET TERMINAL/PERMANENT/WIDTH=100")
	if !strings.Contains(out, "%SET-W-NOTSET, error modifying TTA0:") || !strings.Contains(out, "-SYSTEM-F-NOPRIV") {
		t.Errorf("SET TERMINAL/PERMANENT without LOG_IO: %q", out)
	}

	if got := dvi("DEVBUFSIZ"); got != "80" {
		t.Errorf("a refused /PERMANENT changed the width to %s", got)
	}

	if out := dispatch("SET TERMINAL/WIDTH=100"); out != "" {
		t.Errorf("SET TERMINAL without LOG_IO: %s", out)
	}

	if out := dispatch("SET TERMINAL NLA0:"); !strings.Contains(out, "%SET-W-NOTSET, error modifying NLA0:") {
		t.Errorf("SET TERMINAL NLA0: %q", out)
	}
}
