package debugger_test

import (
	"strings"
	"testing"
)

// These tests are docs/PHASE-42.md's subtask 20: the debugger's built-in
// symbols (the VMS 5.5 Debugger Manual's appendix D).

// TestLocationSymbols: %CURLOC (and "."), %NEXTLOC, and %PREVLOC (and "^")
// are the location EXAMINE, DEPOSIT, or EVALUATE/ADDRESS last referred to,
// the one after it, and the one before, each as big as what was shown
// there.
func TestLocationSymbols(t *testing.T) {
	c := dataSession(t)

	say(t, c, "EXAMINE WATCHL")
	expect(t, "EXAMINE %CURLOC", say(t, c, "EXAMINE %CURLOC"), "DBGCMD\\WATCHL:  00000000\n")
	expect(t, "EVALUATE .", say(t, c, "EVALUATE ."), "00000200\n")

	// WATCHL is a longword, so the next location is WATCHB, a byte.
	expect(t, "EXAMINE %NEXTLOC", say(t, c, "EXAMINE %NEXTLOC"), "DBGCMD\\WATCHB:  00\n")

	// WATCHB is a byte, so the location before it is one byte back.
	expect(t, "EVALUATE %PREVLOC", say(t, c, "EVALUATE %PREVLOC"), "00000203\n")
	expect(t, "EVALUATE ^", say(t, c, "EVALUATE ^"), "00000203\n")
	expect(t, "EVALUATE .+1", say(t, c, "EVALUATE .+1"), "00000205\n")

	// EVALUATE/ADDRESS moves the current location; DEPOSIT does too.
	say(t, c, "EVALUATE/ADDRESS SOURCE")
	expect(t, "EXAMINE/ASCII:4 .", say(t, c, "EXAMINE/ASCII:4 ."), "DBGCMD\\SOURCE:  '0123'\n")

	say(t, c, "DEPOSIT WATCHL = 9")
	expect(t, "EXAMINE .", say(t, c, "EXAMINE ."), "DBGCMD\\WATCHL:  00000009\n")
}

// TestCurrentValue: %CURVAL (and "\") is the value EXAMINE or EVALUATE last
// showed, or DEPOSIT last stored; EVALUATE/ADDRESS doesn't change it.
func TestCurrentValue(t *testing.T) {
	c := dataSession(t)

	if _, err := sayErr(c, "EVALUATE %CURVAL"); err == nil || !strings.Contains(err.Error(), "no value") {
		t.Errorf("EVALUATE %%CURVAL before any value: %v", err)
	}

	say(t, c, "DEPOSIT WATCHL = 5")
	expect(t, `EVALUATE \`, say(t, c, `EVALUATE \`), "00000005\n")
	expect(t, "EVALUATE %CURVAL+1", say(t, c, "EVALUATE %CURVAL+1"), "00000006\n")

	// That EVALUATE's value, 6, is now the current value.
	say(t, c, "EVALUATE/ADDRESS SOURCE")
	expect(t, "EVALUATE %CURVAL", say(t, c, "EVALUATE %CURVAL"), "00000006\n")

	say(t, c, "DEPOSIT/BYTE WATCHB = 7F")
	say(t, c, "EXAMINE WATCHB")
	expect(t, `EVALUATE \`, say(t, c, `EVALUATE \`), "0000007F\n")

	// A register examined is a value shown too.
	say(t, c, "DEPOSIT R5 = 1234")
	say(t, c, "EXAMINE R5")
	expect(t, `EVALUATE \`, say(t, c, `EVALUATE \`), "00001234\n")
}

// TestRadixSymbols: %BIN, %DEC, %HEX, and %OCT read the numbers of the
// operand after them in their radix (the manual's examples).
func TestRadixSymbols(t *testing.T) {
	c := dataSession(t)

	for _, s := range []struct{ command, want string }{
		{"EVALUATE/DEC %HEX 10", "16\n"},
		{"EVALUATE/DEC %HEX (10 + 10)", "32\n"},
		{"EVALUATE/DEC %BIN 10", "2\n"},
		{"EVALUATE/DEC %OCT (10 + 10)", "16\n"},
		{"EVALUATE/HEX %DEC 10", "0000000A\n"},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}

	say(t, c, "SET RADIX DECIMAL")

	for _, s := range []struct{ command, want string }{
		{"EVALUATE %HEX 20 + 33", "65\n"},
		{"EVALUATE %HEX (20+33)", "83\n"},
		{"EVALUATE %HEX (20+ %OCT 10 +33)", "91\n"},
		{"EVALUATE %DECWINDOWS", "0\n"},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}
}

// TestNameSymbols: %NAME 'string' names a symbol by a string, and a
// leading backslash names a global symbol.
func TestNameSymbols(t *testing.T) {
	c := dataSession(t)

	expect(t, "EVALUATE/ADDRESS %NAME 'WATCHB'", say(t, c, "EVALUATE/ADDRESS %NAME 'WATCHB'"), "00000204\n")
	expect(t, `EVALUATE/ADDRESS \WATCHB`, say(t, c, `EVALUATE/ADDRESS \WATCHB`), "00000204\n")
	expect(t, "EXAMINE %NAME 'WATCHL'", say(t, c, "EXAMINE %NAME 'WATCHL'"), "DBGCMD\\WATCHL:  00000000\n")

	if _, err := sayErr(c, "EVALUATE %NAME 'NO.SUCH'"); err == nil || !strings.Contains(err.Error(), "NO.SUCH") {
		t.Errorf("EVALUATE %%NAME of an unknown name: %v", err)
	}
}

// TestExceptionSymbols: the %EXC_ symbols describe the exception the
// program last stopped for, and a WHEN clause may test them: the first
// SS$_ENDOFFILE signaled is handled (CATCH), the second isn't.
func TestExceptionSymbols(t *testing.T) {
	c := probeSession(t)

	if _, err := sayErr(c, "EVALUATE %EXC_NUMBER"); err == nil || !strings.Contains(err.Error(), "no current exception") {
		t.Errorf("EVALUATE %%EXC_NUMBER with no exception: %v", err)
	}

	say(t, c, "SET BREAK/EXCEPTION")
	say(t, c, "GO")

	for _, s := range []struct{ command, want string }{
		{"EVALUATE %EXC_NUMBER", "00000870\n"},
		{"EVALUATE %EXC_SEVERITY", "00000000\n"},
		{"EVALUATE %EXC_NAME", "'ENDOFFILE'\n"},
		{"EVALUATE %EXC_FACILITY", "'SYSTEM'\n"},
		{"EVALUATE/CONDITION_VALUE %EXC_NUMBER", "%SYSTEM-W-ENDOFFILE, end of file\n"},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}

	if _, err := sayErr(c, "EVALUATE %EXC_NAME+1"); err == nil {
		t.Errorf("EVALUATE %%EXC_NAME+1 succeeded")
	}

	// A WHEN clause on the exception's name: the break no longer fires for
	// ENDOFFILE, so the next stop is the unhandled one.
	say(t, c, `SET BREAK/EXCEPTION WHEN (%EXC_NAME NEQ "ENDOFFILE")`)

	out := say(t, c, "GO")
	if !strings.Contains(out, "break on unhandled exception") || strings.Contains(out, "break on exception") {
		t.Errorf("GO with WHEN (%%EXC_NAME NEQ ...):\n%s", out)
	}

	expect(t, "EVALUATE %EXC_NAME", say(t, c, "EVALUATE %EXC_NAME"), "'ENDOFFILE'\n")

	// Running again forgets the exception.
	say(t, c, "STEP/INSTRUCTION")

	if _, err := sayErr(c, "EVALUATE %EXC_NUMBER"); err == nil {
		t.Errorf("EVALUATE %%EXC_NUMBER after a STEP succeeded")
	}
}
