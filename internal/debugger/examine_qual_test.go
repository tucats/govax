package debugger_test

import "testing"

// TestExamineInstructionQualifiers: govax's /CONSTANTS and /SHAREABLE
// qualifiers of EXAMINE/INSTRUCTION name what the VMS debugger doesn't: a
// constant by the symbol that defines it, and a G^ reference (an address in
// a shareable image, such as a run-time library routine) by routine.
// They moved here with DISASSEMBLE, which had them (docs/PHASE-42.md,
// Decision 7), and SET MODE NOSYMBOLIC is the old /NOSYMBOLIC.
func TestExamineInstructionQualifiers(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	cases := []struct {
		command, want string
	}{
		{"EXAMINE/INSTRUCTION %LINE 42", `DBGDIS\START\%LINE 42:  MOVL     S^#0A,R2` + "\n"},
		{"EXAMINE/INSTRUCTION/CONSTANTS %LINE 42", `DBGDIS\START\%LINE 42:  MOVL     S^#DBGDIS\LIMIT,R2` + "\n"},
		{"EXAMINE/INSTRUCTION %LINE 93", `DBGDIS\START\%LINE 93:  CALLS    S^#01,@L^SUB2+0F0` + "\n"},
		{"EXAMINE/INSTRUCTION/SHAREABLE %LINE 93", `DBGDIS\START\%LINE 93:  CALLS    S^#01,G^LIB$PUT_OUTPUT` + "\n"},
	}

	for _, tc := range cases {
		if got := examineOutput(t, c, tc.command); got != tc.want {
			t.Errorf("%s:\ngot  %q\nwant %q", tc.command, got, tc.want)
		}
	}
}
