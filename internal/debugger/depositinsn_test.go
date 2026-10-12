package debugger_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// These tests are docs/PHASE-42.md's subtask 19: DEPOSIT/INSTRUCTION.

// bytesAt returns n bytes of memory at addr, in hexadecimal.
func bytesAt(t *testing.T, c *console.Console, addr, n uint32) string {
	t.Helper()

	data, err := c.ReadBytes(addr, n)
	if err != nil {
		t.Fatalf("ReadBytes(%X): %v", addr, err)
	}

	return fmt.Sprintf("% X", data)
}

// TestDepositInstruction: an instruction is assembled where it goes, its
// numbers read in the input radix and its names the program's.
func TestDepositInstruction(t *testing.T) {
	c := dataSession(t)

	// 301 is past the data, in the writable data page.
	say(t, c, "DEPOSIT/INSTRUCTION 301 = 'MOVL #20A,R0'")
	expect(t, "bytes", bytesAt(t, c, 0x301, 7), "D0 8F 0A 02 00 00 50")

	if got := say(t, c, "EXAMINE/INSTRUCTION 301"); !strings.Contains(got, "MOVL") || !strings.Contains(got, "R0") {
		t.Errorf("EXAMINE/INSTRUCTION 301: %q", got)
	}

	// A small number is a short literal; a name is its address; a path
	// name too.
	say(t, c, `DEPOSIT/INSTRUCTION 301 = "ADDL2 #2,WATCHL"`)
	expect(t, "ADDL2 bytes", bytesAt(t, c, 0x301, 3), "C0 02 CF")

	say(t, c, `DEPOSIT/INSTRUCTION 301 = "MOVAL DBGCMD\WATCHL,R1"`)
	expect(t, "MOVAL bytes", bytesAt(t, c, 0x301, 2), "DE CF")

	// A branch's displacement is from where the instruction is: BRB to
	// 310 from 301 is 310-(301+2) = 0D.
	say(t, c, "DEPOSIT/INSTRUCTION 301 = 'BRB 310'")
	expect(t, "BRB bytes", bytesAt(t, c, 0x301, 2), "11 0D")

	// In the decimal radix, 10 is ten.
	say(t, c, "SET RADIX/INPUT DECIMAL")
	say(t, c, "DEPOSIT/INSTRUCTION 769 = 'MOVL #10,R0'")
	expect(t, "decimal bytes", bytesAt(t, c, 0x301, 3), "D0 0A 50")
	say(t, c, "SET RADIX HEXADECIMAL")

	// SET TYPE INSTRUCTION makes it the default for untyped locations.
	say(t, c, "SET TYPE INSTRUCTION")
	say(t, c, "DEPOSIT 301 = 'NOP'")
	expect(t, "NOP", bytesAt(t, c, 0x301, 1), "01")
	say(t, c, "SET TYPE LONGWORD")

	for _, bad := range []string{
		"DEPOSIT/INSTRUCTION 301 = NOP",              // not quoted
		"DEPOSIT/INSTRUCTION 301 = 'FROB R0'",        // no such instruction
		"DEPOSIT/INSTRUCTION R0 = 'NOP'",             // a register
		"DEPOSIT/INSTRUCTION 301 = 'MOVL NOSUCH,R0'", // an unknown name
	} {
		if _, err := sayErr(c, bad); err == nil {
			t.Errorf("%s succeeded", bad)
		}
	}
}

// TestDepositInstructionRuns: an instruction deposited at the PC is the
// one the next step runs.
func TestDepositInstructionRuns(t *testing.T) {
	c := dataSession(t)

	// The program is at BACK: MULL2 R2,R0, three bytes; MOVL #7,R0 is
	// three bytes too.
	say(t, c, "DEPOSIT/INSTRUCTION .PC = 'MOVL #7,R0'")
	say(t, c, "STEP/INSTRUCTION")
	expect(t, "EXAMINE R0", say(t, c, "EXAMINE R0"), "DBGCMD\\FACT\\%R0:        00000007\n")
}
