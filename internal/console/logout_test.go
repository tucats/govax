package console

import (
	"strings"
	"testing"
)

// TestLogout_processOne: LOGOUT in the console's own process ends govax,
// and the session's end writes its report: by default (and /BRIEF) the one
// line, with /FULL the accounting information too. /BRIEF and /FULL are
// one setting, as on VMS: the last given counts, /NOFULL is /BRIEF, and
// /NOBRIEF is /FULL. EXIT, and a LOGOUT taken back (Resume, a No to
// ending batch jobs), write nothing.
func TestLogout_processOne(t *testing.T) {
	cases := []struct {
		command string
		full    bool
		report  bool
	}{
		{"LOGOUT", false, true},
		{"LOGOUT/BRIEF", false, true},
		{"LOGOUT/FULL", true, true},
		{"LO/FULL", true, true},
		{"LOGOUT/BRIEF/FULL", true, true},
		{"LO/FULL/BR", false, true}, // LO*GOUT := LOGOUT/FULL, then LO/BR
		{"LOGOUT/NOFULL", false, true},
		{"LOGOUT/NOBRIEF", true, true},
		{"EXIT", false, false},
	}

	for _, tc := range cases {
		d, c, buf := newCommandDispatcher(t)

		if err := d.Dispatch(tc.command); err != nil {
			t.Fatalf("%s: %v", tc.command, err)
		}

		if c.Running() || buf.Len() != 0 {
			t.Errorf("%s: running %v, output %q; want ended, nothing yet", tc.command, c.Running(), buf.String())
		}

		c.EndSession()

		got := buf.String()
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")

		switch {
		case !tc.report:
			if strings.Contains(got, "logged out") {
				t.Errorf("%s: %q, want no report", tc.command, got)
			}

		case !strings.HasPrefix(got, "  SYSTEM       logged out at "):
			t.Errorf("%s: %q, want SYSTEM's logged out line", tc.command, got)

		case tc.full && (len(lines) != 7 || lines[2] != "  Accounting information:" ||
			!strings.HasPrefix(lines[6], "  Charged CPU time:")):
			t.Errorf("%s: %q, want the line and the accounting information", tc.command, got)

		case !tc.full && len(lines) != 1:
			t.Errorf("%s: %q, want the line alone", tc.command, got)
		}
	}

	d, c, buf := newCommandDispatcher(t)

	if err := d.Dispatch("LOGOUT/FULL"); err != nil {
		t.Fatal(err)
	}

	c.Resume()
	c.EndSession()

	if strings.Contains(buf.String(), "logged out") {
		t.Errorf("LOGOUT taken back: %q, want no report", buf.String())
	}
}
