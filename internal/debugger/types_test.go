package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// These tests are docs/PHASE-42.md's subtask 18: EXAMINE's and DEPOSIT's
// type qualifiers, and SET TYPE, SHOW TYPE, and CANCEL TYPE. They run on
// the probe's program (dataSession: WATCHL at 200, WATCHB 204, BUFFER 208
// for 16 bytes, SOURCE 218, MSG an .ASCID).

// TestFloatTypes: a floating value is deposited from a literal and shown
// as the manual's examples show each format: fixed notation with 7
// significant digits for F_floating and 15 for D_floating, and exponent
// notation, with one more digit, outside 0.1 to 10 to that power.
func TestFloatTypes(t *testing.T) {
	c := dataSession(t)

	for _, s := range []struct{ deposit, examine, want string }{
		{"DEPOSIT/FLOAT WATCHL = 1.5", "EXAMINE/FLOAT WATCHL", "DBGCMD\\WATCHL:  1.500000\n"},
		{"DEPOSIT/FLOAT WATCHL = 5.35E3", "EXAMINE/FLOAT WATCHL", "DBGCMD\\WATCHL:  5350.000\n"},
		{"DEPOSIT/FLOAT WATCHL = 0.2407524", "EXAMINE/FLOAT WATCHL", "DBGCMD\\WATCHL:  0.2407524\n"},
		{"DEPOSIT/FLOAT WATCHL = 3.0E10", "EXAMINE/FLOAT WATCHL", "DBGCMD\\WATCHL:  3.0000001E+10\n"},
		{"DEPOSIT/FLOAT WATCHL = -2.5E-3", "EXAMINE/F_FLOAT WATCHL", "DBGCMD\\WATCHL:  -2.4999999E-03\n"},
		{"DEPOSIT/FLOAT WATCHL = 0", "EXAMINE/FLOAT WATCHL", "DBGCMD\\WATCHL:  0.0000000\n"},
		{"DEPOSIT/D_FLOAT BUFFER = 2.356", "EXAMINE/D_FLOAT BUFFER", "DBGCMD\\BUFFER[0]:       2.35600000000000\n"},
		{"DEPOSIT/G_FLOAT BUFFER = 1.5E-300", "EXAMINE/G_FLOAT BUFFER", "DBGCMD\\BUFFER[0]:       1.500000000000000E-300\n"},
		{"DEPOSIT/H_FLOAT BUFFER = 0.5", "EXAMINE/H_FLOAT BUFFER", "DBGCMD\\BUFFER[0]:       0." + strings.Repeat("5", 1) + strings.Repeat("0", 32) + "\n"},
	} {
		say(t, c, s.deposit)
		expect(t, s.examine, say(t, c, s.examine), s.want)
	}

	// F_floating 1.5 is the longword 000040C0: the word with the sign and
	// exponent comes first in memory.
	say(t, c, "DEPOSIT/FLOAT WATCHL = 1.5")
	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  000040C0\n")

	// An integer expression is converted.
	say(t, c, "DEPOSIT/FLOAT WATCHL = 0A")
	expect(t, "EXAMINE/FLOAT WATCHL", say(t, c, "EXAMINE/FLOAT WATCHL"), "DBGCMD\\WATCHL:  10.00000\n")

	// A reserved operand (sign set, exponent zero) is named.
	say(t, c, "DEPOSIT WATCHL = 8000")
	expect(t, "EXAMINE/FLOAT WATCHL", say(t, c, "EXAMINE/FLOAT WATCHL"), "DBGCMD\\WATCHL:  Reserved operand\n")
}

// TestFloatDatum: a label of floating data (DBGDIS's FVAL: .F_FLOATING 1.5)
// is shown as its type, without a qualifier.
func TestFloatDatum(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	if got := examineOutput(t, c, "EXAMINE FVAL"); got != "DBGDIS\\FVAL:    1.500000\n" {
		t.Errorf("EXAMINE FVAL: %q", got)
	}

	dispatch(t, c, "DEPOSIT FVAL = 2.25")

	if got := examineOutput(t, c, "EXAMINE FVAL"); got != "DBGDIS\\FVAL:    2.250000\n" {
		t.Errorf("EXAMINE FVAL after DEPOSIT: %q", got)
	}
}

// TestPackedType: a packed decimal string of n digits is n/2+1 bytes, two
// digits a byte and the sign in the last nibble.
func TestPackedType(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT/PACKED:5 BUFFER = -^D123")
	expect(t, "EXAMINE/PACKED:5 BUFFER", say(t, c, "EXAMINE/PACKED:5 BUFFER"), "DBGCMD\\BUFFER[0]:       -123\n")
	expect(t, "EXAMINE/BYTE BUFFER[0]", say(t, c, "EXAMINE/BYTE BUFFER[0]:BUFFER[2]"),
		"DBGCMD\\BUFFER[0]:       00\nDBGCMD\\BUFFER[1]:       12\nDBGCMD\\BUFFER[2]:       3D\n")

	say(t, c, "DEPOSIT/PACKED:4 BUFFER = ^D9876")
	expect(t, "EXAMINE/PACKED:4 BUFFER", say(t, c, "EXAMINE/PACKED:4 BUFFER"), "DBGCMD\\BUFFER[0]:       9876\n")

	if _, err := sayErr(c, "DEPOSIT/PACKED:2 BUFFER = ^D123"); err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Errorf("DEPOSIT/PACKED:2 of 3 digits: %v", err)
	}
}

// TestTextTypes: the counted, zero-terminated, and descriptor strings.
func TestTextTypes(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT/ASCIC BUFFER = 'HELLO'")
	expect(t, "EXAMINE/ASCIC BUFFER", say(t, c, "EXAMINE/ASCIC BUFFER"), "DBGCMD\\BUFFER[0]:       'HELLO'\n")
	expect(t, "EXAMINE/AC BUFFER", say(t, c, "EXAMINE/AC BUFFER"), "DBGCMD\\BUFFER[0]:       'HELLO'\n")
	expect(t, "EXAMINE/BYTE BUFFER", say(t, c, "EXAMINE/BYTE BUFFER"), "DBGCMD\\BUFFER[0]:       05\n")

	say(t, c, `DEPOSIT/ASCIW BUFFER = "AB"`)
	expect(t, "EXAMINE/ASCIW BUFFER", say(t, c, "EXAMINE/ASCIW BUFFER"), "DBGCMD\\BUFFER[0]:       'AB'\n")
	expect(t, "EXAMINE/WORD BUFFER", say(t, c, "EXAMINE/WORD BUFFER"), "DBGCMD\\BUFFER[0]:       0002\n")

	say(t, c, "DEPOSIT/ASCIZ BUFFER = 'xyz'")
	expect(t, "EXAMINE/ASCIZ BUFFER", say(t, c, "EXAMINE/ASCIZ BUFFER"), "DBGCMD\\BUFFER[0]:       'xyz'\n")

	expect(t, "EXAMINE/ASCID MSG", say(t, c, "EXAMINE/ASCID MSG"), "DBGCMD\\MSG:     'DBGCMD: done'\n")

	// A string deposited through a descriptor is padded to its length.
	say(t, c, "DEPOSIT/ASCID MSG = 'ok'")
	expect(t, "EXAMINE/ASCID MSG", say(t, c, "EXAMINE/AD MSG"), "DBGCMD\\MSG:     'ok          '\n")

	// A quoted string deposited at a text label fills its length.
	say(t, c, "DEPOSIT SOURCE = 'abc'")
	expect(t, "EXAMINE/ASCII:5 SOURCE", say(t, c, "EXAMINE/ASCII:5 SOURCE"), "DBGCMD\\SOURCE:  'abc  '\n")
}

// TestOtherTypes: octawords, times, condition values, and the PSW.
func TestOtherTypes(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT/OCTAWORD BUFFER = 12345678")
	expect(t, "EXAMINE/OCTAWORD BUFFER", say(t, c, "EXAMINE/OCTAWORD BUFFER"),
		"DBGCMD\\BUFFER[0]:       "+strings.Repeat("0", 24)+"12345678\n")

	say(t, c, `DEPOSIT/DATE_TIME BUFFER = "11-OCT-2026 14:03:22.17"`)
	expect(t, "EXAMINE/DATE_TIME BUFFER", say(t, c, "EXAMINE/DATE_TIME BUFFER"),
		"DBGCMD\\BUFFER[0]:       11-OCT-2026 14:03:22.17\n")

	if _, err := sayErr(c, `DEPOSIT/DATE_TIME BUFFER = "NOT A TIME"`); err == nil {
		t.Error("DEPOSIT/DATE_TIME of a bad time succeeded")
	}

	say(t, c, "DEPOSIT WATCHL = 870")
	expect(t, "EXAMINE/CONDITION_VALUE WATCHL", say(t, c, "EXAMINE/CONDITION_VALUE WATCHL"),
		"DBGCMD\\WATCHL:  %SYSTEM-W-ENDOFFILE, end of file\n")
	expect(t, "EVALUATE/CONDITION_VALUE 1", say(t, c, "EVALUATE/CONDITION_VALUE 1"),
		"%SYSTEM-S-NORMAL, normal successful completion\n")

	c.CPU.SetPSL(vax.PSL(0x03C0000D))
	expect(t, "EXAMINE/PSW PSL", say(t, c, "EXAMINE/PSW PSL"),
		"DBGCMD\\FACT\\%PSL:       \n        DV FU IV T N Z V C\n         0  0  0 0 1 1 0 1\n")
}

// TestRegisterTypes: a register takes a type as memory does; a value wider
// than a longword takes the registers after it.
func TestRegisterTypes(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT/D_FLOAT R0 = 1.5")
	expect(t, "EXAMINE/D_FLOAT R0", say(t, c, "EXAMINE/D_FLOAT R0"), "DBGCMD\\FACT\\%R0:        1.50000000000000\n")
	expect(t, "EXAMINE R0", say(t, c, "EXAMINE R0"), "DBGCMD\\FACT\\%R0:        000040C0\n")
	expect(t, "EXAMINE R1", say(t, c, "EXAMINE R1"), "DBGCMD\\FACT\\%R1:        00000000\n")

	say(t, c, "DEPOSIT R0 = 1")
	expect(t, "EXAMINE/CONDITION_VALUE R0", say(t, c, "EXAMINE/CONDITION_VALUE R0"),
		"DBGCMD\\FACT\\%R0:        %SYSTEM-S-NORMAL, normal successful completion\n")

	for _, bad := range []string{"EXAMINE/ASCIC R0", "EXAMINE/OCTAWORD PC", "DEPOSIT/ASCIZ R0 = 'A'"} {
		if _, err := sayErr(c, bad); err == nil || !strings.Contains(err.Error(), "not allowed on given data type") {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

// TestSetType: SET TYPE types the locations no symbol types, SET
// TYPE/OVERRIDE every location, and SHOW TYPE says which; a qualifier wins
// over both.
func TestSetType(t *testing.T) {
	c := dataSession(t)

	expect(t, "SHOW TYPE", say(t, c, "SHOW TYPE"), "type: long integer\n")
	expect(t, "SHOW TYPE/OVERRIDE", say(t, c, "SHOW TYPE/OVERRIDE"), "type/override: none\n")

	// 300 is past the data, in no image's symbols.
	say(t, c, "DEPOSIT 300 = 11223344")
	expect(t, "EXAMINE 300", say(t, c, "EXAMINE 300"), "00000300:       11223344\n")

	say(t, c, "SET TYPE BYTE")
	expect(t, "SHOW TYPE", say(t, c, "SHOW TYPE"), "type: byte integer\n")
	expect(t, "EXAMINE 300", say(t, c, "EXAMINE 300"), "00000300:       44\n")

	// A label keeps its own type.
	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  00000000\n")

	say(t, c, "SET TYPE/OVERRIDE WORD")
	expect(t, "SHOW TYPE/OVERRIDE", say(t, c, "SHOW TYPE/OVERRIDE"), "type/override: word integer\n")
	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  0000\n")
	expect(t, "EXAMINE/LONG WATCHL", say(t, c, "EXAMINE/LONGWORD WATCHL"), "DBGCMD\\WATCHL:  00000000\n")

	say(t, c, "CANCEL TYPE/OVERRIDE")
	expect(t, "SHOW TYPE/OVERRIDE", say(t, c, "SHOW TYPE/OVERRIDE"), "type/override: none\n")

	say(t, c, "SET TYPE ASCII:3")
	expect(t, "EXAMINE 300", say(t, c, "EXAMINE 300"), "00000300:       'D3\"'\n")

	say(t, c, "SET TYPE PACKED:3")
	expect(t, "SHOW TYPE", say(t, c, "SHOW TYPE"), "type: packed decimal, 3 digits\n")

	say(t, c, "SET TYPE LONG")
	expect(t, "SHOW TYPE", say(t, c, "SHOW TYPE"), "type: long integer\n")

	if _, err := sayErr(c, "SET TYPE NOSUCH"); err == nil {
		t.Error("SET TYPE NOSUCH succeeded")
	}
}

// TestTypeQualifier: /TYPE=(name) shows a location as the datum name
// labels is typed.
func TestTypeQualifier(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT WATCHL = 0A5")
	expect(t, "EXAMINE/TYPE=(WATCHB) WATCHL", say(t, c, "EXAMINE/TYPE=(WATCHB) WATCHL"), "DBGCMD\\WATCHL:  A5\n")

	say(t, c, "SET TYPE/OVERRIDE TYPE=(WATCHB)")
	expect(t, "SHOW TYPE/OVERRIDE", say(t, c, "SHOW TYPE/OVERRIDE"), "type/override: type of WATCHB\n")
	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  A5\n")
	say(t, c, "CANCEL TYPE/OVERRIDE")

	if _, err := sayErr(c, "EXAMINE/TYPE=(FACT) WATCHL"); err == nil {
		t.Error("EXAMINE/TYPE= of a routine succeeded")
	}
}

// TestTypeMiscellany: /NOSYMBOLIC names a location typed as a number by
// its address, a range
// steps by its type's size, and SET TYPE INSTRUCTION shows an untyped
// location as an instruction.
func TestTypeMiscellany(t *testing.T) {
	c := dataSession(t)

	expect(t, "EXAMINE/NOSYMBOLIC 200", say(t, c, "EXAMINE/NOSYMBOLIC 200"), "00000200:       00000000\n")
	expect(t, "EXAMINE/NOSYMBOLIC WATCHL", say(t, c, "EXAMINE/NOSYMBOLIC WATCHL"), "DBGCMD\\WATCHL:  00000000\n")

	say(t, c, "DEPOSIT/FLOAT BUFFER[0] = 1")
	say(t, c, "DEPOSIT/FLOAT BUFFER[4] = 2")
	expect(t, "EXAMINE/FLOAT BUFFER[0]:BUFFER[4]", say(t, c, "EXAMINE/FLOAT BUFFER[0]:BUFFER[4]"),
		"DBGCMD\\BUFFER[0]:       1.000000\nDBGCMD\\BUFFER[4]:       2.000000\n")

	// 301 is past the data, in no image's symbols: an instruction's bytes
	// once deposited there (01 is NOP).
	say(t, c, "DEPOSIT/BYTE 301 = 1")
	say(t, c, "SET TYPE INSTRUCTION")
	expect(t, "SHOW TYPE", say(t, c, "SHOW TYPE"), "type: instruction\n")

	if got := say(t, c, "EXAMINE 301"); !strings.Contains(got, "NOP") {
		t.Errorf("EXAMINE 301 with SET TYPE INSTRUCTION: %q", got)
	}
}
