package debugger_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/vax"
)

// dataSession is the probe's session stopped at its fourth FACT call's
// return (as exam.dlg's), where the data is at known addresses (WATCHL at
// 200, WATCHB 204, BUFFER 208 for 16 bytes, SOURCE 218).
func dataSession(t *testing.T) *console.Console {
	t.Helper()

	c := stepSession(t)

	say(t, c, "SET BREAK/AFTER:3 BACK")
	say(t, c, "GO")
	say(t, c, "CANCEL BREAK/ALL")

	return c
}

// TestPSLTable: the processor status longword is shown as the table of
// its fields exam.dlg shows (a user-mode PSL of 03C00000), and others'
// fields land under their names.
func TestPSLTable(t *testing.T) {
	c := dataSession(t)
	c.CPU.SetPSL(vax.PSL(0x03C00000))

	want := `DBGCMD\FACT\%PSL:       
        CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C
         0   0  0   0  USER   USER    0  0  0  0 0 0 0 0 0
`
	expect(t, "EXAMINE PSL", say(t, c, "EXAMINE PSL"), want)
	expect(t, "EXAMINE/PSL PSL", say(t, c, "EXAMINE/PSL PSL"), want)

	// Kernel mode, IPL 1F, the interrupt stack, and every condition code:
	// each field's text lands in the row under its name.
	c.CPU.SetPSL(vax.PSL(0x041F000F))

	rows := strings.Split(say(t, c, "EXAMINE PSL"), "\n")
	if len(rows) < 3 || !strings.HasSuffix(rows[2], "KERNEL KERNEL  1F  0  0  0 0 1 1 1 1") || //nolint:dupword
		!strings.Contains(rows[2], " 1 KERNEL") {
		t.Errorf("EXAMINE PSL of a kernel-mode PSL:\n%s", strings.Join(rows, "\n"))
	}
}

// TestEvaluateErrors: EVALUATE of the contents of a data label, as exam.dlg
// shows: .WATCHL is what is at the address WATCHL holds (0), which can't
// be read, and so is .R2 (R2 is 4).
func TestEvaluateErrors(t *testing.T) {
	c := dataSession(t)

	for command, want := range map[string]string{
		"EVALUATE .WATCHL": "no read access to address 00000000",
		"EVALUATE .R2":     "no read access to address 00000004",
	} {
		if _, err := sayErr(c, command); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", command, err, want)
		}
	}
}

// TestEvaluateRadix: the output radix and the qualifiers choose how a value
// is written, and the input radix how it is read (exam.dlg's radix block).
func TestEvaluateRadix(t *testing.T) {
	c := dataSession(t)

	for _, s := range []struct{ command, want string }{
		{"EVALUATE 255", "00000255\n"},
		{"EVALUATE/DECIMAL 100", "256\n"},
		{"EVALUATE/OCTAL 10", "00000000020\n"},
		{"EVALUATE/BINARY 5", "0000000000000000 0000000000000101\n"},
		{"EVALUATE 0FFFF+1", "00010000\n"},
		{"EVALUATE NOT 0", "0FFFFFFFF\n"},
		{"EVALUATE 7 MOD 3", "00000001\n"},
		{"EVALUATE 1@4", "00000010\n"},
		{"EVALUATE 6 AND 3", "00000002\n"},
		{"EVALUATE 6 OR 1", "00000007\n"},
		{"EVALUATE 6 XOR 3", "00000005\n"},
		{"EVALUATE 5 GTR 3", "00000001\n"},
		{"EVALUATE 5 LSS 3", "00000000\n"},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}
}

// TestDepositForms: numbers are in the input radix and sized by the
// label's type, a type qualifier overrides it, a string goes in with
// /ASCII (cut or padded to its count), and a register takes a longword.
func TestDepositForms(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT WATCHL = 7")
	say(t, c, "DEPOSIT/BYTE WATCHB = 42")
	say(t, c, `DEPOSIT/ASCII:4 BUFFER = "WXYZ"`)
	say(t, c, "DEPOSIT R3 = 99")
	say(t, c, "DEPOSIT/WORD BUFFER[8] = 1234")

	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  00000007\n")
	expect(t, "EXAMINE WATCHB", say(t, c, "EXAMINE WATCHB"), "DBGCMD\\WATCHB:  42\n")
	expect(t, "EXAMINE R3", say(t, c, "EXAMINE R3"), "DBGCMD\\FACT\\%R3:        00000099\n")
	expect(t, "EXAMINE/WORD BUFFER[8]", say(t, c, "EXAMINE/WORD BUFFER[8]"), "DBGCMD\\BUFFER[8]:       1234\n")
	expect(t, "EXAMINE/ASCII:4 BUFFER", say(t, c, "EXAMINE/ASCII:4 BUFFER"), "DBGCMD\\BUFFER[0:15]:    'WXYZ'\n")

	// A deposit with no value, or no location, is an error.
	for _, bad := range []string{"DEPOSIT WATCHL", "DEPOSIT = 5"} {
		if _, err := sayErr(c, bad); err == nil {
			t.Errorf("%s succeeded", bad)
		}
	}
}

// TestExamineRadixQualifiers: a value is shown in the radix the command or
// the session's output radix gives, padded to its size, and a byte's, word's,
// and quadword's widths follow their sizes.
func TestExamineRadixQualifiers(t *testing.T) {
	c := dataSession(t)

	say(t, c, "DEPOSIT WATCHL = 0A5")

	for _, s := range []struct{ command, want string }{
		{"EXAMINE/DECIMAL WATCHL", "DBGCMD\\WATCHL:  165\n"},
		{"EXAMINE/OCTAL/BYTE WATCHL", "DBGCMD\\WATCHL:  245\n"},
		{"EXAMINE/BINARY/BYTE WATCHL", "DBGCMD\\WATCHL:  10100101\n"},
		{"EXAMINE/BINARY/WORD WATCHL", "DBGCMD\\WATCHL:  0000000010100101\n"},
		{"EXAMINE/QUADWORD WATCHL", "DBGCMD\\WATCHL:  00000000000000A5\n"},
		{"EXAMINE/WORD WATCHL", "DBGCMD\\WATCHL:  00A5\n"},
	} {
		expect(t, s.command, say(t, c, s.command), s.want)
	}

	say(t, c, "SET RADIX/OUTPUT DECIMAL")
	expect(t, "EXAMINE WATCHL", say(t, c, "EXAMINE WATCHL"), "DBGCMD\\WATCHL:  165\n")
}

// TestExamineText: a string label shows its text in quotes and a
// non-printing byte as a period, MSG (.ASCID) is reached through its
// descriptor, and /ASCII:n takes n characters in the input radix (so 16 is
// 22 characters, as the probe's log shows).
func TestExamineText(t *testing.T) {
	c := dataSession(t)

	expect(t, "EXAMINE MSG", say(t, c, "EXAMINE MSG"), "DBGCMD\\MSG:     'DBGCMD: done'\n")
	expect(t, "EXAMINE/ASCII:5 SOURCE", say(t, c, "EXAMINE/ASCII:5 SOURCE"), "DBGCMD\\SOURCE:  '01234'\n")

	// BUFFER holds 16 bytes and SOURCE follows it: /ASCII:16 is 0x16 = 22
	// characters, the 16 zero bytes (periods) and SOURCE's first six.
	expect(t, "EXAMINE/ASCII:16 BUFFER", say(t, c, "EXAMINE/ASCII:16 BUFFER"),
		"DBGCMD\\BUFFER[0:15]:    '"+strings.Repeat(".", 16)+"012345'\n")
}

// TestExamineErrors: an unreadable address is NOACCESSR, a bad type
// combination is refused, and EXAMINE of an unknown name says so.
func TestExamineErrors(t *testing.T) {
	c := dataSession(t)

	for command, want := range map[string]string{
		"EXAMINE 0":                  "no read access to address 00000000",
		"EXAMINE/BYTE/WORD WATCHL":   "BYTE",
		"EXAMINE/DECIMAL/OCTAL R2":   "DECIMAL",
		"EXAMINE NOSUCHSYMBOL":       "NOSUCHSYMBOL",
		"EXAMINE BUFFER[16]":         "BUFFER",
		"SYMBOLIZE NOSUCHSYMBOL":     "NOSUCHSYMBOL",
		"EVALUATE":                   "",
		"EVALUATE 1 +":               "",
		"EXAMINE WATCHL,":            "",
		"EVALUATE/DECIMAL/OCTAL 1":   "DECIMAL",
		"DEPOSIT/BYTE/WORD WATCHL=1": "BYTE",
	} {
		_, err := sayErr(c, command)
		if err == nil {
			t.Errorf("%s succeeded", command)

			continue
		}

		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q, want it to mention %q", command, err, want)
		}
	}
}
