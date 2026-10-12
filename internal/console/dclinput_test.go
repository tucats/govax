package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// skpdatLine and ivvaluLine are CLI$_SKPDAT's and CLI$_IVVALU's
// messages, as a procedure shows them.
const (
	skpdatLine = `%DCL-W-SKPDAT, image data (records not beginning with "$") ignored` + "\n"
	ivvaluLine = "%DCL-W-IVVALU, invalid value syntax - see command documentation\n"
)

// runInputProcedure writes lines as a command procedure, runs it, and
// returns what it wrote. A failure the procedure has already shown is
// not an error here.
func runInputProcedure(t *testing.T, lines ...string) string {
	t.Helper()

	d, _, buf := newCommandDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "input.com", lines...)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@input: %v", err)
	}

	return buf.String()
}

// TestProcedureInput_data: what reads a procedure's data lines, and what
// becomes of those nothing reads (dclinput.go).
func TestProcedureInput_data(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  string
	}{
		{
			// TYPE SYS$INPUT reads the data up to the next "$" line,
			// blank and indented lines as they are.
			name:  "type",
			lines: []string{"$ TYPE SYS$INPUT", "one", "", "  two", `$ PRINT "after"`},
			want:  "one\n\n  two\nafter\n",
		},
		{
			name:  "type with a colon, after IF's THEN",
			lines: []string{"$ IF 1 THEN TYPE SYS$INPUT:", "one", `$ PRINT "after"`},
			want:  "one\nafter\n",
		},
		{
			// A logical name that translates to SYS$INPUT, directly or
			// through another, is the input too (RMS's iterative
			// translation).
			name: "logical names",
			lines: []string{"$ DEFINE INFILE SYS$INPUT", "$ DEFINE OTHER INFILE:", "$ TYPE INFILE", "one",
				"$ TYPE OTHER", "two", `$ PRINT "after"`},
			want: "one\ntwo\nafter\n",
		},
		{
			// An EOD line ends the input and is used up; the data
			// after it, which TYPE doesn't read, is skipped.
			name:  "EOD",
			lines: []string{"$ TYPE SYS$INPUT", "one", "$ EOD", "two", `$ PRINT "after"`},
			want:  "one\n" + skpdatLine + "after\n",
		},
		{
			// DECK makes "$" lines data, up to EOD.
			name:  "DECK",
			lines: []string{"$ TYPE SYS$INPUT", "$ DECK", `$ PRINT "x"`, "plain", "$ EOD", `$ PRINT "after"`},
			want:  "$ PRINT \"x\"\nplain\nafter\n",
		},
		{
			// /DOLLARS's string ends the deck instead, and an EOD
			// line inside it is data.
			name:  "DECK/DOLLARS",
			lines: []string{"$ TYPE SYS$INPUT", `$ DECK/DOLL="%%"`, "$ EOD", "%% the end", `$ PRINT "after"`},
			want:  "$ EOD\nafter\n",
		},
		{
			// DECK read by DCL itself is IVVALU, a warning, and starts
			// no deck; EOD does nothing (VMS 7.3, testdata/dcl50's I3).
			name: "DECK as a command",
			lines: []string{`$ PRINT "a"`, "$ DECK", `$ PRINT "run"`, `PRINT "data"`, "$ EOD", `$ PRINT "b"`,
				`$ DECK/DOLLARS="%%"`, `PRINT "read"`},
			want: "a\n" + ivvaluLine + "run\n" + skpdatLine + "b\n" + ivvaluLine + "read\n",
		},
		{
			// A lone "$" ends the run of an image's data DCL skips: the
			// data after it is a command (VMS 7.3, I8); a comment too.
			name: "a lone dollar",
			lines: []string{"$ TYPE SYS$INPUT", "one", "$", `PRINT "read"`, `$ PRINT "a"`, "skipped",
				"$ ! a comment", `PRINT "read too"`},
			want: "one\nread\na\n" + skpdatLine + "read too\n",
		},
		{
			// A "$" after blanks or a tab is data (VMS 7.3, I8).
			name:  "indented dollars",
			lines: []string{"$ TYPE SYS$INPUT", "  $ blanks", "\t$ tab", `$ PRINT "after"`},
			want:  "  $ blanks\n\t$ tab\nafter\n",
		},
		{
			// After one of DCL's own commands, a data line is a command
			// (VMS 7.3, testdata/dcl50's V6).
			name:  "data after an assignment",
			lines: []string{"$ X = 1", `PRINT "a command"`, `$ PRINT "after"`},
			want:  "a command\nafter\n",
		},
		{
			name:  "data at the start",
			lines: []string{`PRINT "a command"`},
			want:  "a command\n",
		},
		{
			// After an image (PRINT counts as one), data is skipped.
			name:  "data after an image",
			lines: []string{`$ PRINT "a"`, `PRINT "data"`, "more", `$ PRINT "b"`},
			want:  "a\n" + skpdatLine + "b\n",
		},
		{
			// Image verification shows each line as it is read, before
			// TYPE types it; procedure verification is off.
			name:  "image verification",
			lines: []string{"$ SET VERIFY=(NOPROCEDURE,IMAGE)", "$ TYPE SYS$INPUT", "one", "two", "$ SET NOVERIFY"},
			want:  "one\none\ntwo\ntwo\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runInputProcedure(t, tc.lines...); got != tc.want {
				t.Errorf("output:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

// TestProcedureInput_nested: a nested procedure's TYPE SYS$INPUT reads
// its own data lines, and the outer procedure's input is its own again
// when it returns; a data line after the @ is a command, @ being DCL's.
func TestProcedureInput_nested(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com", "$ TYPE SYS$INPUT", "inner data")
	outer := writeProcedure(t, dir, "outer.com", "$ @"+inner, `PRINT "outer command"`,
		"$ TYPE SYS$INPUT", "outer data")

	if err := d.Dispatch("@" + outer); err != nil {
		t.Fatalf("@outer: %v", err)
	}

	if want := "inner data\nouter command\nouter data\n"; buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}
}

// TestProcedureInput_terminal: at the terminal, TYPE SYS$INPUT types
// what is typed, to the end of the input; DECK is IVVALU, and EOD does
// nothing, leaving $STATUS as it was (VMS 7.3, testdata/dcl50's I3).
func TestProcedureInput_terminal(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	c.In = strings.NewReader("typed one\ntyped two\n")

	if err := d.Dispatch("DECK"); !errors.Is(err, vmserrors.New(vmserrors.CLI_IVVALU)) {
		t.Errorf("DECK: %v, want IVVALU", err)
	}

	deckStatus := c.Status()
	// VMS 7.3 gave %X00038088, CLI$_IVVALU.
	if deckStatus&^stsInhibitMsg != 0x00038088 {
		t.Errorf("$STATUS after DECK: %08X, want %%X00038088", deckStatus)
	}

	for _, line := range []string{"EOD", "TYPE SYS$INPUT"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if line == "EOD" && c.Status() != deckStatus {
			t.Errorf("$STATUS after EOD: %08X, want DECK's", c.Status())
		}
	}

	if want := "typed one\ntyped two\n"; buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}
}


// TestIsInputStream: SYS$INPUT by name and through logical names, and
// names that lead elsewhere.
func TestIsInputStream(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	for _, line := range []string{
		"DEFINE INFILE SYS$INPUT", "DEFINE OTHER INFILE:", "DEFINE AFILE SYS$INPUT:[DIR]X.DAT",
		"DEFINE LOOP1 LOOP2", "DEFINE LOOP2 LOOP1",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	for spec, want := range map[string]bool{
		"SYS$INPUT": true, "sys$input:": true, "INFILE": true, "OTHER:": true,
		"AFILE": false, "SYS$INPUT:X.DAT": false, "LOOP1": false, "NOSUCH": false, "X.DAT": false,
	} {
		if got := c.IsInputStream(spec); got != want {
			t.Errorf("IsInputStream(%q) = %v, want %v", spec, got, want)
		}
	}
}
