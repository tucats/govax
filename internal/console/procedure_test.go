package console

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// writeProcedure writes a host command procedure of lines in dir, and
// returns its path quoted for an @ command (a host path has a "/" and
// lowercase letters, so DCL needs the quotes).
func writeProcedure(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	writeFile(t, path, strings.Join(lines, "\n")+"\n")

	return `"` + path + `"`
}

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

// TestProcedure_parameters: P1 to P8 are the procedure's local symbols,
// "" for those not given (User's Manual, 12.14's SHOW SYMBOL example).
func TestProcedure_parameters(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "params.com", "$ SHOW SYMBOL/LOCAL/ALL")

	if err := d.Dispatch("@" + proc + ` one "Two Words" "" x"y"z`); err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		`  P1 = "ONE"`,
		`  P2 = "Two Words"`,
		`  P3 = ""`,
		`  P4 = "XyZ"`,
		`  P5 = ""`,
		`  P6 = ""`,
		`  P7 = ""`,
		`  P8 = ""`,
	}, "\n") + "\n"

	if buf.String() != want {
		t.Errorf("SHOW SYMBOL/LOCAL/ALL in the procedure:\n%s\nwant:\n%s", buf.String(), want)
	}
}

// TestProcedure_symbolScope: a procedure's local symbols go when it ends,
// its global ones stay; it sees the levels outside it; and a nested
// procedure's P1 is its own (User's Manual, 12.10 and 14.4).
func TestProcedure_symbolScope(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com",
		"$ SHOW SYMBOL OUTER",
		"$ SHOW SYMBOL TERM",
		"$ SHOW SYMBOL P1",
		`$ INNER = "i"`,
		`$ INNER_GLOBAL == "ig"`,
	)

	outer := writeProcedure(t, dir, "outer.com",
		`$ OUTER = "o"`,
		`$ TERM = "changed"`,
		`$ OUTER_GLOBAL == "og"`,
		"$ @"+inner,
		"$ SHOW SYMBOL/LOCAL/ALL",
	)

	for _, line := range []string{`TERM = "t"`, "@" + outer + " first"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	for _, want := range []string{
		`  OUTER = "o"`,
		`  TERM = "changed"`,
		`  P1 = ""`,
		`  P1 = "FIRST"`,
	} {
		if !strings.Contains(buf.String(), want+"\n") {
			t.Errorf("output lacks %q:\n%s", want, buf.String())
		}
	}

	if strings.Contains(buf.String(), "INNER") {
		t.Errorf("the inner procedure's locals outlived it:\n%s", buf.String())
	}

	if c.CommandLevel() != 0 {
		t.Errorf("command level %d after the procedures, want 0", c.CommandLevel())
	}

	for name, want := range map[string]string{"TERM": "t", "OUTER_GLOBAL": "og", "INNER_GLOBAL": "ig"} {
		if got, ok := c.DCLSymbol(name); !ok || got != want {
			t.Errorf("%s = %q, %v at level 0; want %q", name, got, ok, want)
		}
	}

	for _, name := range []string{"OUTER", "INNER", "P1"} {
		if got, ok := c.DCLSymbol(name); ok {
			t.Errorf("%s = %q at level 0; want undefined", name, got)
		}
	}
}

// TestProcedure_exit: EXIT ends the procedure it's in, and the level
// above goes on; at the terminal it ends govax. QUIT ends govax from
// inside a procedure.
func TestProcedure_exit(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com", `$ PRINT "before"`, "$ EXIT", `$ PRINT "after"`)
	outer := writeProcedure(t, dir, "outer.com", "$ @"+inner, `$ PRINT "back"`)

	if err := d.Dispatch("@" + outer); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "before\nback\n" || !c.Running() {
		t.Errorf("output %q, running %v; want %q, still running", got, c.Running(), "before\nback\n")
	}

	quits := writeProcedure(t, dir, "quits.com", "$ QUIT", `$ PRINT "after"`)
	caller := writeProcedure(t, dir, "caller.com", "$ @"+quits, `$ PRINT "back"`)

	buf.Reset()

	if err := d.Dispatch("@" + caller); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 || c.Running() || c.CommandLevel() != 0 {
		t.Errorf("after QUIT in a procedure: output %q, running %v, level %d; want none, ended, 0",
			buf.String(), c.Running(), c.CommandLevel())
	}

	d2, c2, _ := newCommandDispatcher(t)
	if err := d2.Dispatch("EXIT"); err != nil || c2.Running() {
		t.Errorf("EXIT at the terminal: %v, running %v; want govax ended", err, c2.Running())
	}
}

// TestProcedure_errorEndsProcedure: by DCL's default action an error ends
// the procedure, and the procedures that called it, with its message shown
// once; a warning is shown and the procedure goes on (User's Manual, 13.8).
func TestProcedure_errorEndsProcedure(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com", `$ PRINT "one"`, "$ SET BOGUS", `$ PRINT "two"`)
	outer := writeProcedure(t, dir, "outer.com", "$ @"+inner, `$ PRINT "outer after"`)

	err := d.Dispatch("@" + outer)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_BADPARAMETER)) || !vmserrors.MessageInhibited(err) {
		t.Errorf("@outer: %v, want CLI_BADPARAMETER with its message shown", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "one\n%DCL-E-BADPARAMETER") || strings.Count(out, "BADPARAMETER") != 1 ||
		strings.Contains(out, "two") || strings.Contains(out, "outer after") {
		t.Errorf("output:\n%s\nwant one, then the message once, then nothing", out)
	}

	if c.CommandLevel() != 0 || !c.Running() {
		t.Errorf("level %d, running %v; want 0, running", c.CommandLevel(), c.Running())
	}

	warns := writeProcedure(t, dir, "warns.com", "$ @X 1 2 3 4 5 6 7 8 9", `$ PRINT "still here"`)

	buf.Reset()

	if err := d.Dispatch("@" + warns); err != nil {
		t.Errorf("a procedure with a warning: %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "%DCL-W-DEFOVF") || !strings.HasSuffix(out, "still here\n") {
		t.Errorf("output:\n%s\nwant DEFOVF's warning, then still here", out)
	}
}

// TestProcedure_dataLines: data lines (records without a "$") that
// nothing reads are skipped, with one SKPDAT warning for each run of them,
// and the procedure goes on (User's Manual, 13.3 and 13.8).
func TestProcedure_dataLines(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "data.com",
		`$ PRINT "a"`, "1993", "", "1994", `$ PRINT "b"`, "1995")

	// The procedure ends with SKPDAT's status, a warning whose message
	// has been shown.
	if err := d.Dispatch("@" + proc); !errors.Is(err, vmserrors.New(vmserrors.CLI_SKPDAT)) || !vmserrors.MessageInhibited(err) {
		t.Errorf("@data: %v, want SKPDAT, shown", err)
	}

	skpdat := `%DCL-W-SKPDAT, image data (records not beginning with "$") ignored` + "\n"
	if want := "a\n" + skpdat + "b\n" + skpdat; buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}
}

// TestProcedure_output: /OUTPUT sends the procedure's output to a file,
// .LIS by default, and an error message to the terminal too (User's
// Manual, 13.6.4); the console's output is the terminal again after it.
// NL: discards the output.
func TestProcedure_output(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com", `$ PRINT "from inner"`)
	proc := writeProcedure(t, dir, "proc.com", `$ PRINT "to the file"`, "$ @"+inner, "$ SET BOGUS")
	result := filepath.Join(dir, "result")

	if err := d.Dispatch("@" + proc + `/OUTPUT="` + result + `"`); err == nil {
		t.Error("the procedure's error wasn't returned")
	}

	terminal := buf.String()
	if !strings.HasPrefix(terminal, "%DCL-E-BADPARAMETER") || strings.Contains(terminal, "to the file") {
		t.Errorf("terminal:\n%s\nwant only the error message", terminal)
	}

	data, err := os.ReadFile(result + ".lis")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(data); got != "to the file\nfrom inner\n"+terminal {
		t.Errorf("result.lis:\n%s\nwant the output and the message", got)
	}

	buf.Reset()

	if err := d.Dispatch(`PRINT "terminal again"`); err != nil || buf.String() != "terminal again\n" {
		t.Errorf("after the procedure: %v, %q", err, buf.String())
	}

	buf.Reset()

	quiet := writeProcedure(t, dir, "quiet.com", `$ PRINT "discarded"`)
	if err := d.Dispatch("@" + quiet + "/OUTPUT=NL:"); err != nil || buf.Len() != 0 {
		t.Errorf("/OUTPUT=NL:: %v, terminal %q; want nothing shown", err, buf.String())
	}
}

// TestProcedure_volume: a procedure on a mounted volume, found through
// SET DEFAULT with .COM by default, its /OUTPUT file on the volume too.
func TestProcedure_volume(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatal(err)
	}

	records := [][]byte{[]byte(`$ PRINT "on the volume"`), []byte("$ SHOW SYMBOL P1")}
	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "PROC.COM"}, rms.TextRecords, records); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("@PROC/OUTPUT=RESULT hello"); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("terminal %q, want nothing", buf.String())
	}

	if err := c.Type("RESULT.LIS", false); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); !strings.Contains(got, "on the volume\n") || !strings.Contains(got, `P1 = "HELLO"`) {
		t.Errorf("RESULT.LIS:\n%s", got)
	}
}

// TestProcedure_hostNames: a host procedure is found with .COM added, and
// a name DCL uppercased is found in lowercase; a missing one is
// SS$_NOSUCHFILE.
func TestProcedure_hostNames(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "setr6.com"), "$ SET R6=7\n")

	if err := d.Dispatch("@setr6"); err != nil {
		t.Fatal(err)
	}

	if got := c.CPU.GPR(vax.R6); got != 7 {
		t.Errorf("R6 = %#x, want 7", got)
	}

	if err := d.Dispatch("@NOSUCH"); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("@NOSUCH: %v, want SS_NOSUCHFILE", err)
	}
}

// TestProcedure_depth: procedures nest at most 32 command levels deep,
// the terminal's among them; the @ that would make the 32nd fails with
// a warning, CLI_STKOVF, and each level goes on (VMS 7.3's run of
// testdata/dcl50).
func TestProcedure_depth(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "self.com")
	writeFile(t, path, `$ @"`+path+`"`+"\n"+`$ PRINT "after"`+"\n")

	if err := d.Dispatch(`@"` + path + `"`); err != nil {
		t.Errorf("a procedure that runs itself: %v", err)
	}

	if out := buf.String(); strings.Count(out, "STKOVF") != 1 || strings.Count(out, "after") != maxCommandLevels {
		t.Errorf("output:\n%s", out)
	}

	if c.CommandLevel() != 0 || len(c.dclSymbols.levels) != 1 {
		t.Errorf("level %d, %d symbol levels; want 0 and 1", c.CommandLevel(), len(c.dclSymbols.levels))
	}
}
