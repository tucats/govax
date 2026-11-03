package diag

import (
	"strings"
	"testing"
)

// TestMessageString checks the printed form: the VMS-style first line,
// where the message is, the quoted line, and the pointer.
func TestMessageString(t *testing.T) {
	cases := []struct {
		name string
		m    Message
		want string
	}{
		{
			"with a column",
			Message{Error, "NOTIMPL", "ENCODE is not implemented yet",
				Position{File: "DUA0:[WORK]A.FOR;1", Line: 7, Column: 7, Text: "      ENCODE (8, 10, BUF) X"}},
			"%FORT-E-NOTIMPL, ENCODE is not implemented yet\n" +
				"  at line 7 of DUA0:[WORK]A.FOR;1\n" +
				"        ENCODE (8, 10, BUF) X\n" +
				"  ------^",
		},
		{
			"a whole line",
			Message{Warning, "X", "text", Position{File: "a.for", Line: 2, Text: "      CONTINUE"}},
			"%FORT-W-X, text\n  at line 2 of a.for\n        CONTINUE",
		},
		{
			"a whole file",
			Message{Fatal, "X", "text", Position{File: "a.for"}},
			"%FORT-F-X, text\n  in a.for",
		},
		{
			"nowhere",
			Message{Informational, "X", "text", Position{}},
			"%FORT-I-X, text",
		},
	}

	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

// TestPointerKeepsTabs checks that the pointer copies the source line's
// tabs, so it lines up under a tab-format line however a terminal
// expands tabs.
func TestPointerKeepsTabs(t *testing.T) {
	if got := pointer("10\tGO TO 20", 10); got != "--\t------^" {
		t.Errorf("pointer = %q", got)
	}

	if got := pointer("X", 1); got != "^" {
		t.Errorf("pointer at column 1 = %q", got)
	}

	// A column past the end of the line (an error at its end) still
	// gets a pointer.
	if got := pointer("AB", 4); got != "---^" {
		t.Errorf("pointer past the end = %q", got)
	}
}

// TestListSeverities checks the List's counts, its worst severity, and
// when a compilation has failed.
func TestListSeverities(t *testing.T) {
	saved := messages
	defer func() { messages = saved }()

	messages = map[string]kind{
		"INFO": {Informational, "info %d"},
		"WARN": {Warning, "warn"},
		"ERR":  {Error, "err %s"},
	}

	var l List

	if _, reported := l.Worst(); reported || l.Failed() {
		t.Error("an empty List has a worst severity, or failed")
	}

	m := l.Report("INFO", Position{}, 3)
	if m.Text != "info 3" || m.Severity != Informational {
		t.Errorf("Report = %+v", m)
	}

	l.Report("WARN", Position{})
	l.Report("WARN", Position{})

	if worst, _ := l.Worst(); worst != Warning || l.Failed() {
		t.Errorf("after warnings: worst %v, failed %v", worst, l.Failed())
	}

	l.Report("ERR", Position{}, "x")

	if worst, _ := l.Worst(); worst != Error || !l.Failed() {
		t.Errorf("after an error: worst %v, failed %v", worst, l.Failed())
	}

	if l.Count(Warning) != 2 || l.Count(Error) != 1 || len(l.Messages()) != 4 {
		t.Errorf("counts: %d warnings, %d errors, %d messages", l.Count(Warning), l.Count(Error), len(l.Messages()))
	}
}

// TestReportUnknownPanics checks that naming a message the table lacks
// is caught as the compiler's own mistake.
func TestReportUnknownPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Report of an unknown message didn't panic")
		}
	}()

	var l List
	l.Report("NO_SUCH_MESSAGE", Position{})
}

// TestTable checks every entry of the real table: an identifier of
// capitals and digits, as VMS's are, and lowercase text.
func TestTable(t *testing.T) {
	for ident, k := range messages {
		if ident == "" || strings.Trim(ident, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
			t.Errorf("identifier %q isn't capitals and digits", ident)
		}

		if k.Format == "" || (k.Format[0] >= 'A' && k.Format[0] <= 'Z') {
			t.Errorf("%s: text %q should start in lowercase", ident, k.Format)
		}

		if sev, ok := Known(ident); !ok || sev != k.Severity {
			t.Errorf("Known(%s) = %v, %v", ident, sev, ok)
		}
	}
}

// TestSeverityStatus checks the VMS severity codes.
func TestSeverityStatus(t *testing.T) {
	want := map[Severity]uint32{Warning: 0, Error: 2, Informational: 3, Fatal: 4}
	for s, code := range want {
		if s.Status() != code {
			t.Errorf("%v.Status() = %d, want %d", s, s.Status(), code)
		}
	}
}

// TestWarningsKeywords: keywords apply in order to the setting, cut short if you like,
// and what isn't one is refused by name.
func TestWarningsKeywords(t *testing.T) {
	cases := []struct {
		keywords []string
		want     Warnings
	}{
		{nil, Warnings{Declarations: true, Unused: true, Alignment: true}},
		{[]string{"NONE"}, Warnings{NoGeneral: true}},
		{[]string{"ALL", "NONE"}, Warnings{NoGeneral: true}},
		{[]string{"NONE", "ALL"}, Warnings{Declarations: true, Unused: true, Alignment: true}},
		{[]string{"unused"}, Warnings{Unused: true}},
		{[]string{"DECL", "NOGEN", "ALI"}, Warnings{Declarations: true, NoGeneral: true, Alignment: true}},
		{[]string{"ALL", "NODECLARATIONS", "NOUNUSED", "NOALIGNMENT"}, Warnings{}},
		{[]string{"NON"}, Warnings{NoGeneral: true}},
	}

	for _, c := range cases {
		got, err := Warnings{}.With(c.keywords)
		if err != nil {
			t.Errorf("%v: %v", c.keywords, err)
		} else if got != c.want {
			t.Errorf("%v: got %+v, want %+v", c.keywords, got, c.want)
		}
	}

	if _, err := (Warnings{}).With([]string{"BOGUS"}); err == nil || !strings.Contains(err.Error(), "no keyword BOGUS") {
		t.Errorf("BOGUS: %v", err)
	}

	if _, err := (Warnings{}).With([]string{"NO"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("NO: %v", err)
	}
}

// TestWarningsHeld: NOGENERAL holds back what's less than an error and asked for by no
// keyword, and Drop takes it out of a list.
func TestWarningsHeld(t *testing.T) {
	var l List

	l.Report("SUBSCRIPTRANGE", Position{}, 7, "A", "upper", 3)
	l.Report("NOLISTING", Position{}, "/SHOW")
	l.Report("UNUSEDNAME", Position{}, "X")
	l.Report("ALIGNFIELD", Position{}, "F", 1, 4)
	l.Report("UNDEFLBL", Position{}, 10)

	l.Drop(Warnings{}.Held)

	if got := len(l.Messages()); got != 5 {
		t.Errorf("the default held back %d", 5-got)
	}

	l.Drop(Warnings{NoGeneral: true}.Held)

	idents := []string{}
	for _, m := range l.Messages() {
		idents = append(idents, m.Ident)
	}

	if got := strings.Join(idents, " "); got != "UNUSEDNAME ALIGNFIELD UNDEFLBL" {
		t.Errorf("NOGENERAL left %s", got)
	}
}
