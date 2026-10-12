package dclshell

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestParseProcedureCommand: the file name, /OUTPUT only where it touches
// the file name, and the parameters by DCL's rules (User's Manual, 14.2).
func TestParseProcedureCommand(t *testing.T) {
	for _, tc := range []struct {
		text   string
		file   string
		output string
		params []string
	}{
		{"setd", "SETD", "", nil},
		{" WORK:[MAINT]SETD.COM", "WORK:[MAINT]SETD.COM", "", nil},
		{`"/tmp/My Proc.com"`, "/tmp/My Proc.com", "", nil},
		{"setd/output=results.txt", "SETD", "RESULTS.TXT", nil},
		{"setd/out=r a b", "SETD", "R", []string{"A", "B"}},
		{"setd/output = r", "SETD", "R", nil},
		{"setd /output=x", "SETD", "", []string{"/OUTPUT=X"}},
		{`data "Paul Cramer" 24 "(555) 111-1111"`, "DATA", "", []string{"Paul Cramer", "24", "(555) 111-1111"}},
		{`data paul cramer`, "DATA", "", []string{"PAUL", "CRAMER"}},
		{`data "" "Paul Cramer"`, "DATA", "", []string{"", "Paul Cramer"}},
		{`data a"b c"d  "say ""hi"""`, "DATA", "", []string{"Ab cD", `say "hi"`}},
		{"data a\tb ! a comment", "DATA", "", []string{"A", "B"}},
		{`data "!" x`, "DATA", "", []string{"!", "X"}},
		{"sum 1 2 3 4 5 6 7 8", "SUM", "", []string{"1", "2", "3", "4", "5", "6", "7", "8"}},
	} {
		cmd, err := parseProcedureCommand(tc.text)
		if err != nil {
			t.Errorf("parseProcedureCommand(%q): %v", tc.text, err)

			continue
		}

		if cmd.file != tc.file || cmd.output != tc.output || cmd.hasOutput != (tc.output != "") ||
			!reflect.DeepEqual(cmd.params, tc.params) {
			t.Errorf("parseProcedureCommand(%q) = %q, %q, %q; want %q, %q, %q",
				tc.text, cmd.file, cmd.output, cmd.params, tc.file, tc.output, tc.params)
		}
	}

	for _, tc := range []struct {
		text   string
		status uint32
	}{
		{"", vmserrors.CLI_MISSINGPARAMETER},
		{"  ! just a comment", vmserrors.CLI_MISSINGPARAMETER},
		{"sum 1 2 3 4 5 6 7 8 9", vmserrors.CLI_DEFOVF},
		{"setd/log", vmserrors.CLI_UNRECOGNIZED},
		{"setd/output", vmserrors.CLI_NEEDQUALIFIERVALUE},
	} {
		if _, err := parseProcedureCommand(tc.text); !errors.Is(err, vmserrors.New(tc.status)) {
			t.Errorf("parseProcedureCommand(%q): %v, want %v", tc.text, err, vmserrors.New(tc.status))
		}
	}
}

// TestProcedureSource: in a DCL procedure, a command line starts with
// "$", a "$!" line is a comment, continuation lines are joined, and any
// other record is a data line; in a debugger procedure, every line is a
// command. Any line ending will do. The cursor is what labels and GOTO
// will move.
func TestProcedureSource(t *testing.T) {
	text := "$! A DCL comment\r\n" +
		"$ SHOW SYMBOL P1\r\n" +
		"\r\n" +
		"$!\r" +
		"PRINT \"data\"\n" +
		"$ COPY A -\n" +
		"     /LOG -   ! a comment after the hyphen\n" +
		"     B\n" +
		"$ PRINT \"not - continued\" ! -x\n" +
		"$PRINT \"in quotes -\"\n" +
		"  $ INDENTED\n" +
		"$\n" +
		"$ LAST -"

	type line struct {
		text string
		data bool
	}

	want := []line{
		{"SHOW SYMBOL P1", false},
		{"", true},
		{`PRINT "data"`, true},
		{"COPY A /LOG B", false},
		{`PRINT "not - continued" ! -x`, false},
		{`PRINT "in quotes -"`, false},
		{"  $ INDENTED", true},
		{"LAST -", false},
	}

	read := func(p *procedureSource) []line {
		var got []line

		for {
			text, data, ok := p.Read()
			if !ok {
				return got
			}

			got = append(got, line{text, data})
		}
	}

	p := newProcedureSource("TEST.COM", text)
	if got := read(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines:\n%+v\nwant:\n%+v", got, want)
	}

	p.Rewind()

	first, _, _ := p.Read()
	mark := p.Position()
	second, _, _ := p.Read()
	p.Read()
	p.Seek(mark)

	again, _, _ := p.Read()
	if first != want[0].text || second != want[1].text || again != want[1].text {
		t.Errorf("after Rewind and Seek: %q, %q, %q; want %q, %q, %q", first, second, again, want[0].text, want[1].text, want[1].text)
	}

	debug := newProcedureSource("DBG.COM", "! a comment\nSET BREAK A\n\n  EXAMINE -\n  R0\n")
	debug.plain = true

	if got := read(debug); !reflect.DeepEqual(got, []line{{"SET BREAK A", false}, {"EXAMINE R0", false}}) {
		t.Errorf("a debugger procedure's lines: %+v", got)
	}

	if empty := newProcedureSource("EMPTY.COM", ""); len(empty.records) != 0 {
		t.Errorf("an empty file has %d records", len(empty.records))
	}
}
