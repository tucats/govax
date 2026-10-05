package console

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// dlgBlock returns the output lines the debugger logged for command in
// the session log name (testdata/dbg/vax), the first time it was given:
// the lines after "! command" up to the next command, each without the
// log's leading "!", and without any error message.
func dlgBlock(t *testing.T, name, command string) []string {
	t.Helper()

	data, err := os.ReadFile(dbgImagePath(t, name))
	if err != nil {
		t.Fatal(err)
	}

	var (
		lines []string
		in    bool
	)

	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		// A command is "! " and its text; a CASE table's entries are
		// "!" and blanks.
		if (strings.HasPrefix(l, "! ") && !strings.HasPrefix(l, "!  ")) || l == "!" {
			if in {
				break
			}

			in = l == "! "+command

			continue
		}

		// A %DEBUG message is a later command's, whose own line the log
		// leaves out.
		if in && !strings.HasPrefix(l, "!%") {
			lines = append(lines, strings.TrimPrefix(l, "!"))
		}
	}

	if len(lines) == 0 {
		t.Fatalf("%s: no output logged for %q", name, command)
	}

	return lines
}

// dispatchStepped loads image as RUN/STEP does and returns a dispatcher
// for its console, the output so far cleared.
func dispatchStepped(t *testing.T, image string) (*Dispatcher, *bytes.Buffer) {
	t.Helper()

	c, buf := runStepped(t, dbgImagePath(t, image))

	return NewDispatcher(c, loadEvaxGrammar(t), nil), buf
}

// TestDisassembleSymbolic: DISASSEMBLE (symbolic by default) prints what
// the VMS debugger's EXAMINE/INSTRUCTION printed for the same range, in
// each image's session: real LINK's image (DBGDIS) and govax's link of
// the same objects (GVDBGDIS): locations, operands, a CASE table, ranges given by routine,
// label, line, and path, and the decimal radix.
func TestDisassembleSymbolic(t *testing.T) {
	cases := []struct {
		command string
		session string // the debugger's command in dbgdis.dlg
		radix   int
	}{
		{"DISASSEMBLE START LAST", "EXAMINE/INSTRUCTION START:LAST", 16},
		{"DISASSEMBLE LOCALR LOCEND", "EXAMINE/INSTRUCTION LOCALR:LOCEND", 16},
		{`DIS DBGDIS\LOCALR\JSBRTN JSBEND`, "EXAMINE/INSTRUCTION JSBRTN:JSBEND", 16},
		{"DISASSEMBLE %LINE 47 %LINE 56", "EXAMINE/INSTRUCTION %LINE 47:%LINE 56", 16},
		{"DISASSEMBLE %LINE 85", "EXAMINE/INSTRUCTION %LINE 85", 16},
		{`DISASSEMBLE DBGSUB\%LINE 14`, `EXAMINE/INSTRUCTION DBGSUB\%LINE 14`, 16},
		{"DISASSEMBLE/SYMBOLIC START+2", "EXAMINE/INSTRUCTION START+2", 16},
		{"DISASSEMBLE SUB2", "EXAMINE/INSTRUCTION SUB2", 16},
	}

	for _, image := range []string{"dbgdis.exe", "gvdbgdis.exe"} {
		d, buf := dispatchStepped(t, image)

		for _, tc := range cases {
			buf.Reset()
			d.Console.Radix = tc.radix

			if err := d.Dispatch(tc.command); err != nil {
				t.Errorf("%s: %s: %v", image, tc.command, err)

				continue
			}

			got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			want := dlgBlock(t, strings.TrimSuffix(image, ".exe")+".dlg", tc.session)

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("%s: %s:\ngot:\n%s\nwant:\n%s", image, tc.command, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		}
	}
}

// TestDisassembleSymbolicDecimal: SET RADIX DECIMAL's offsets, from the
// session's second %LINE 47 to 56 range.
func TestDisassembleSymbolicDecimal(t *testing.T) {
	d, buf := dispatchStepped(t, "dbgdis.exe")
	d.Console.Radix = 10

	if err := d.Dispatch("DISASSEMBLE %LINE 55 %LINE 56"); err != nil {
		t.Fatal(err)
	}

	want := `DBGDIS\START\%LINE 55:  MOVAB    L^GLIMIT+589,R0
DBGDIS\START\%LINE 56:  MOVAL    L^GLIMIT+605,R0
`
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestDisassembleQualifiers: /NOSYMBOLIC is the console's own layout;
// /CONSTANTS and /SHAREABLE name what the debugger doesn't.
func TestDisassembleQualifiers(t *testing.T) {
	d, buf := dispatchStepped(t, "dbgdis.exe")

	cases := []struct {
		command, want string
	}{
		{"DISASSEMBLE/NOSYMBOLIC %LINE 42", "00000402: MOVL S^#10,R2\n"},
		{"DISASSEMBLE %LINE 42", `DBGDIS\START\%LINE 42:  MOVL     S^#0A,R2` + "\n"},
		{"DISASSEMBLE/CONSTANTS %LINE 42", `DBGDIS\START\%LINE 42:  MOVL     S^#DBGDIS\LIMIT,R2` + "\n"},
		{"DISASSEMBLE %LINE 93", `DBGDIS\START\%LINE 93:  CALLS    S^#01,@L^SUB2+0F0` + "\n"},
		{"DISASSEMBLE/SHAREABLE %LINE 93", `DBGDIS\START\%LINE 93:  CALLS    S^#01,G^LIB$PUT_OUTPUT` + "\n"},
		{"DISASSEMBLE/NOSYMBOLIC/SHAREABLE %LINE 93", "0000050E: CALLS S^#1,G^LIB$PUT_OUTPUT\n"},
	}

	for _, tc := range cases {
		buf.Reset()

		if err := d.Dispatch(tc.command); err != nil {
			t.Errorf("%s: %v", tc.command, err)

			continue
		}

		if got := buf.String(); got != tc.want {
			t.Errorf("%s:\ngot  %q\nwant %q", tc.command, got, tc.want)
		}
	}
}

// TestEvaluateDebugNames: the expression evaluator finds names and lines
// in a loaded image's debug symbol table, after the console's own.
func TestEvaluateDebugNames(t *testing.T) {
	c, _ := runStepped(t, dbgImagePath(t, "dbgdis.exe"))

	cases := []struct {
		expr string
		want uint32
	}{
		{"START", 0x400},
		{`DBGDIS\START\LOOP`, 0x4DD},
		{`dbgdis\start\loop`, 0x4DD},
		{"JSBRTN+3", 0x53E},
		{`DBGSUB\SUBDATA`, 0x268},
		{"GLIMIT", 3},                // a global constant
		{"%LINE 47", 0x419},          // the debugger's EVALUATE/ADDRESS %LINE 47
		{`DBGSUB\%LINE 14`, 0x546},   // ... and DBGSUB\%LINE 14
		{`DBGDIS\START\%LINE 42+3`, 0x405},
		{"(%LINE 47)-2", 0x417},
	}

	for _, tc := range cases {
		v, rest, err := c.Evaluator().Eval(tc.expr)
		if err != nil || strings.TrimSpace(rest) != "" || v != tc.want {
			t.Errorf("Eval(%q) = %X, %q, %v; want %X", tc.expr, v, rest, err, tc.want)
		}
	}

	for _, bad := range []string{`DBGDIS\NOSUCH`, `DBGSUB\SUBEND`, "%LINE 9999", "%LINE", `NOSUCH\%LINE 1`} {
		if _, _, err := c.Evaluator().Eval(bad); err == nil {
			t.Errorf("Eval(%q) succeeded; want an error", bad)
		}
	}
}
