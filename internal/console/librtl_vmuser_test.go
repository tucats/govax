package console

import "testing"

// TestLibrtlUserZone: the 5.0 LIB$ manual's Example 5-1, a user-defined
// zone that passes each operation on to a real zone (its user argument
// the real zone's ID) and counts them, with LIB$DELETE_VM_ZONE itself
// as the delete routine. LIB$GET_VM and LIB$FREE_VM of the user zone
// reach the real zone through the program's routines, a second free's
// LIB$_BADBLOADR is the routine's status passed back, RESET goes
// through, DELETE deletes the real zone (deleting it again is
// LIB$_BADBLOADR), and the user zone is gone too. It ends with 1 when
// all of that held, or the number of the check that failed, or the
// failing status.
func TestLibrtlUserZone(t *testing.T) {
	r0, _ := runMacroProgram(t, `	.PSECT	DATA,NOEXE,WRT,LONG
REAL:	.LONG	0
USER:	.LONG	0
SIZE:	.LONG	40
ADDR:	.LONG	0
GETS:	.LONG	0
FREES:	.LONG	0
RESETS:	.LONG	0
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	GET_RTN,^M<>
	INCL	GETS
	CALLG	(AP),G^LIB$GET_VM
	RET
	.ENTRY	FREE_RTN,^M<>
	INCL	FREES
	CALLG	(AP),G^LIB$FREE_VM
	RET
	.ENTRY	RESET_RTN,^M<>
	INCL	RESETS
	CALLG	(AP),G^LIB$RESET_VM_ZONE
	RET
	.ENTRY	START,^M<>
	PUSHAL	REAL
	CALLS	#1,G^LIB$CREATE_VM_ZONE
	BLBS	R0,1$
	RET
1$:	PUSHAL	G^LIB$DELETE_VM_ZONE
	PUSHAL	RESET_RTN
	PUSHAL	FREE_RTN
	PUSHAL	GET_RTN
	PUSHAL	REAL
	PUSHAL	USER
	CALLS	#6,G^LIB$CREATE_USER_VM_ZONE
	BLBS	R0,2$
	RET
2$:	PUSHAL	USER
	PUSHAL	ADDR
	PUSHAL	SIZE
	CALLS	#3,G^LIB$GET_VM
	BLBS	R0,3$
	RET
3$:	MOVL	#10,R0
	CMPL	GETS,#1
	BEQL	51$
	RET
51$:	PUSHAL	USER
	PUSHAL	ADDR
	PUSHAL	SIZE
	CALLS	#3,G^LIB$FREE_VM
	BLBS	R0,4$
	RET
4$:	PUSHAL	USER
	PUSHAL	ADDR
	PUSHAL	SIZE
	CALLS	#3,G^LIB$FREE_VM
	MOVL	#20,R1
	CMPL	R0,#^X158264
	BEQL	59$
	MOVL	R1,R0
	RET
59$:	MOVL	#30,R0
	CMPL	FREES,#2
	BEQL	52$
	RET
52$:	PUSHAL	USER
	CALLS	#1,G^LIB$RESET_VM_ZONE
	BLBS	R0,5$
	RET
5$:	MOVL	#40,R0
	CMPL	RESETS,#1
	BEQL	53$
	RET
53$:	PUSHAL	USER
	CALLS	#1,G^LIB$DELETE_VM_ZONE
	BLBS	R0,6$
	RET
6$:	PUSHAL	REAL
	CALLS	#1,G^LIB$DELETE_VM_ZONE
	MOVL	#50,R1
	CMPL	R0,#^X158264
	BEQL	60$
	MOVL	R1,R0
	RET
60$:	PUSHAL	USER
	PUSHAL	ADDR
	PUSHAL	SIZE
	CALLS	#3,G^LIB$GET_VM
	MOVL	#60,R1
	CMPL	R0,#^X158264
	BEQL	61$
	MOVL	R1,R0
	RET
61$:	MOVL	#1,R0
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (10: GET_RTN not called once, 20: the second free's status, 30: FREE_RTN not called twice, "+
			"40: RESET_RTN not called, 50: the real zone not deleted, 60: the user zone not gone; otherwise the failing status)", r0)
	}
}

// TestLibrtlZonePageRoutines: a zone made with get-page and free-page
// routines of the program's (LIB$CREATE_VM_ZONE's twelfth and thirteenth
// arguments, the VSI manual's places), which count their calls and pass
// them on to LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE. As on VMS 7.1
// (testdata/lib59), the zone's control block's page is a get-page call,
// and its one-page initial area another; a small block fits there, a
// 2000-byte block takes a third call (a new area); with the routine
// failing (status ^X1234), a block too large for both areas is
// LIB$_INSVIRMEM; and LIB$DELETE_VM_ZONE gives both areas and the
// control block's page back, a free-page call each. It ends with 1 when
// all of that held, or the number of the check that failed, or the
// failing status.
func TestLibrtlZonePageRoutines(t *testing.T) {
	r0, _ := runMacroProgram(t, `	.PSECT	DATA,NOEXE,WRT,LONG
ZONE:	.LONG	0
INIT:	.LONG	1
SMALL:	.LONG	8
BIG:	.LONG	2000
HUGE:	.LONG	20000
ADDR:	.LONG	0
GETP:	.LONG	0
FREEP:	.LONG	0
FAIL:	.LONG	0
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	GETPAGE,^M<>
	INCL	GETP
	TSTL	FAIL
	BEQL	1$
	MOVL	#^X1234,R0
	RET
1$:	CALLG	(AP),G^LIB$GET_VM_PAGE
	RET
	.ENTRY	FREEPAGE,^M<>
	INCL	FREEP
	CALLG	(AP),G^LIB$FREE_VM_PAGE
	RET
	.ENTRY	START,^M<>
	PUSHAL	FREEPAGE
	PUSHAL	GETPAGE
	CLRQ	-(SP)
	CLRQ	-(SP)
	CLRL	-(SP)
	PUSHAL	INIT
	CLRQ	-(SP)
	CLRQ	-(SP)
	PUSHAL	ZONE
	CALLS	#13,G^LIB$CREATE_VM_ZONE
	BLBS	R0,2$
	RET
2$:	MOVL	#10,R0
	CMPL	GETP,#2
	BEQL	54$
	RET
54$:	PUSHAL	ZONE
	PUSHAL	ADDR
	PUSHAL	SMALL
	CALLS	#3,G^LIB$GET_VM
	BLBS	R0,3$
	RET
3$:	MOVL	#20,R0
	CMPL	GETP,#2
	BEQL	55$
	RET
55$:	PUSHAL	ZONE
	PUSHAL	ADDR
	PUSHAL	BIG
	CALLS	#3,G^LIB$GET_VM
	BLBS	R0,4$
	RET
4$:	MOVL	#30,R0
	CMPL	GETP,#3
	BEQL	56$
	RET
56$:	MOVL	#1,FAIL
	PUSHAL	ZONE
	PUSHAL	ADDR
	PUSHAL	HUGE
	CALLS	#3,G^LIB$GET_VM
	MOVL	#40,R1
	CMPL	R0,#^X158214
	BEQL	62$
	MOVL	R1,R0
	RET
62$:	MOVL	#50,R0
	CMPL	GETP,#4
	BEQL	57$
	RET
57$:	CLRL	FAIL
	PUSHAL	ZONE
	CALLS	#1,G^LIB$DELETE_VM_ZONE
	BLBS	R0,5$
	RET
5$:	MOVL	#60,R0
	CMPL	FREEP,#3
	BEQL	58$
	RET
58$:	MOVL	#1,R0
	RET
	.END	START
`)

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1 (10: not a get-page call for the control block and one for the initial area, 20: one for a block that fit, "+
			"30: none for a new area, 40: not LIB$_INSVIRMEM when the routine failed, 50: it wasn't called, "+
			"60: not a free-page call per area and the control block; otherwise the failing status)", r0)
	}
}
