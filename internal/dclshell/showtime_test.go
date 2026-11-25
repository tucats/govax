package dclshell

import (
	"regexp"
	"strings"
	"testing"
)

// TestShowTime checks SHOW TIME and SHOW DAYTIME: the system time after
// two blanks, to the second, as VMS's DCL shows it ("  25-NOV-2026
// 12:30:58"), the same time F$TIME() gives.
func TestShowTime(t *testing.T) {
	c, buf := newTestInterpreter(t)
	g := loadEvaxGrammar(t)
	c.Grammar = g
	c.BindDCL(g)

	d := &testDispatcher{Console: c}
	c.SetRoute(d.Dispatch)

	form := regexp.MustCompile(`^  [ 0-9]\d-[A-Z]{3}-\d{4} \d{2}:\d{2}:\d{2}$`)

	for _, line := range []string{"SHOW TIME", "SHOW DAYTIME", "SHOW TI"} {
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		got := strings.TrimSuffix(buf.String(), "\n")
		if !form.MatchString(got) {
			t.Errorf("%s printed %q, not VMS's form", line, got)
		}

		want := timeText(c.systemTime())
		if !strings.HasPrefix(want, got[2:]) {
			t.Errorf("%s printed %q; F$TIME is %q", line, got, want)
		}
	}
}
