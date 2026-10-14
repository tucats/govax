package console

import (
	"regexp"
	"testing"
)

// TestLibrtlVMZoneDebugging: a linked program that makes a zone named
// PROBE, walks the zones with LIB$FIND_VM_ZONE (the manual's example,
// LIB$SHOW_VM_ZONE of each at level 0), then shows its zone at level 1
// through an action routine that writes each line and returns the user
// argument (a success, so every line comes), and through one that
// returns a failure (so the report stops after its first line, with
// that status), and verifies the zone. It ends with 1, or the number of
// the check that failed.
func TestLibrtlVMZoneDebugging(t *testing.T) {
	r0, out := runMacroProgram(t, `	.PSECT	DATA,NOEXE,WRT,LONG
CTX:	.LONG	0
ZONE:	.LONG	0
MINE:	.LONG	0
ONE:	.LONG	1
NAME:	.ASCID	/PROBE/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	ACTION,^M<>
	PUSHL	4(AP)
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	8(AP),R0
	RET
	.ENTRY	FAIL,^M<>
	PUSHL	4(AP)
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#^X10,R0
	RET
	.ENTRY	START,^M<>
	PUSHAQ	NAME
	.REPEAT	9
	PUSHL	#0
	.ENDR
	PUSHAL	MINE
	CALLS	#11,G^LIB$CREATE_VM_ZONE
	BLBS	R0,1$
	RET
1$:	PUSHAL	ZONE
	PUSHAL	CTX
	CALLS	#2,G^LIB$FIND_VM_ZONE
	BLBC	R0,2$
	PUSHAL	ZONE
	CALLS	#1,G^LIB$SHOW_VM_ZONE
	BLBS	R0,1$
	RET
2$:	MOVL	#10,R1
	CMPL	R0,#^X158274
	BEQL	3$
	MOVL	R1,R0
	RET
3$:	PUSHL	#^X12345671
	PUSHAL	ACTION
	PUSHAL	ONE
	PUSHAL	MINE
	CALLS	#4,G^LIB$SHOW_VM_ZONE
	MOVL	#20,R1
	CMPL	R0,#1
	BEQL	4$
	MOVL	R1,R0
	RET
4$:	PUSHL	#0
	PUSHAL	FAIL
	PUSHAL	ONE
	PUSHAL	MINE
	CALLS	#4,G^LIB$SHOW_VM_ZONE
	MOVL	#30,R1
	CMPL	R0,#^X10
	BEQL	5$
	MOVL	R1,R0
	RET
5$:	PUSHAL	MINE
	CALLS	#1,G^LIB$VERIFY_VM_ZONE
	MOVL	#40,R1
	CMPL	R0,#1
	BEQL	6$
	MOVL	R1,R0
6$:	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %d, want 1", r0)
	}

	// The zone IDs are the image's addresses: masked.
	got := regexp.MustCompile(`Id = [0-9A-F]{8}`).ReplaceAllString(out, "Id = XXXXXXXX")

	const want = "Zone Id = XXXXXXXX,  Zone name = \"DEFAULT_ZONE\"\n" +
		"Zone Id = XXXXXXXX,  Zone name = \"STRING_ZONE\"\n" +
		"Zone Id = XXXXXXXX,  Zone name = \"PROBE\"\n" +
		"Zone Id = XXXXXXXX,  Zone name = \"PROBE\"\n" +
		"\n" +
		"\tAlgorithm = LIB$K_VM_FIRST_FIT\n" +
		"\n" +
		"\tFlags = 00000000\n" +
		"\n" +
		"\tInitial size =      16 pages     Current size = 0 pages in 0 areas\n" +
		"\tExtend size  =      16 pages     Page limit   = None\n" +
		"\n" +
		"\tRequests are rounded up to a multiple of 8 bytes,\n" +
		"\tnaturally aligned on 8 byte boundaries\n" +
		"\n" +
		"\t0 bytes have been freed and not yet reallocated\n" +
		"\n" +
		"\t88 bytes are used for zone and area control blocks, or 100.0% overhead\n" +
		"\n" +
		"Zone Id = XXXXXXXX,  Zone name = \"PROBE\"\n"

	if got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}
