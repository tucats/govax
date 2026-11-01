C     Calling LIBRTL as the LIB$ manual's FORTRAN examples do (Phase 83, subtask
C     5): the routines that make an instruction callable, with quadwords as
C     two-element arrays and results by reference; and a function's status from
C     a condition, through LIB$SIG_TO_RET, LIB$MATCH_COND, $UNWIND, and $PUTMSG.
      PROGRAM RTLCALLS
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INTEGER*4 A(2), B(2), C(2), MUL1, MUL2, ADDEND, DIVISOR, Q, R, ST
      INTEGER*4 STATES(4)
      COMMON /SHARED/ STATES
      REAL*4 X, COEFF(3), RESULT
      INTEGER*2 DEGREE
      INTEGER*4 LIB$ADDX, LIB$SUBX, LIB$EMUL, LIB$EDIV, LIB$POLYF
      INTEGER*4 LIB$SKPC, LIB$LOCC, LIB$INDEX, LIB$LEN, LIB$LP_LINES
      LOGICAL*4 LIB$BBSSI, LIB$BBCCI
      INTEGER*4 DIVIDE, OPENIT, EXTRA
      COMMON /UNWIND/ EXTRA
      REAL*4 FLOATS(2)
      DATA COEFF /1.0, 2.0, 3.0/, FLOATS /2.0, 0.0/
C
C     Multiple-precision arithmetic: 2**32 - 1 and 1 make 2**32, and back.
      A(1) = -1
      A(2) = 0
      B(1) = 1
      B(2) = 0
      ST = LIB$ADDX(A, B, C)
      PRINT *, ST .EQ. SS$_NORMAL, C(1), C(2)
      ST = LIB$SUBX(C, B, A)
      PRINT *, ST .EQ. SS$_NORMAL, A(1), A(2)
C
C     2**16 times 2**16, plus 5, in a quadword, and divided back.
      MUL1 = 65536
      MUL2 = 65536
      ADDEND = 5
      ST = LIB$EMUL(MUL1, MUL2, ADDEND, C)
      PRINT *, ST .EQ. SS$_NORMAL, C(1), C(2)
      DIVISOR = 65536
      ST = LIB$EDIV(DIVISOR, C, Q, R)
      PRINT *, ST .EQ. SS$_NORMAL, Q, R
C
C     X**2 + 2X + 3 at 2: a word's degree, and a REAL's result.
      X = 2.0
      DEGREE = 2
      ST = LIB$POLYF(X, DEGREE, COEFF, RESULT)
      PRINT *, ST .EQ. SS$_NORMAL, RESULT
C
C     Interlocked bits: the old value comes back, and bit 42 is in the second
C     longword. A name declared by a module may still be CALLed: the answer is
C     thrown away.
      PRINT *, LIB$BBSSI(42, STATES), LIB$BBSSI(42, STATES), STATES(2)
      PRINT *, LIB$BBCCI(42, STATES), LIB$BBCCI(42, STATES), STATES(2)
      CALL SHAREDBIT
      PRINT *, STATES(1)
C
C     Strings by descriptor, and a function with no arguments.
      PRINT *, LIB$SKPC(' ', '  AB'), LIB$SKPC('A', 'AAA'),
     1         LIB$LOCC('B', 'ABC'), LIB$LOCC('Z', 'ABC'),
     2         LIB$INDEX('HELLO', 'L')
      PRINT *, LIB$LP_LINES() .GT. 0
C
C     A function that turns any condition into its status.
      PRINT *, DIVIDE(7, 2), DIVIDE(7, 0) .EQ. SS$_INTDIV
      CALL HALVES(FLOATS)
C
C     A handler that knows the failure, and unwinds out of the OPEN: by the
C     mechanism array's depth to the frame that established it, past the OPEN,
C     and one further to that frame's caller, with the mechanism array's R0.
      EXTRA = 0
      ST = OPENIT('NOSUCH.DAT')
      PRINT *, 'BACK', ST
      EXTRA = 1
      ST = OPENIT('NOSUCH.DAT')
      PRINT *, 'BACK', ST
C
C     $PUTMSG calling a routine with each line, which it keeps from the terminal.
      CALL MESSAGES
      PRINT *, 'DONE'
      END

      SUBROUTINE SHAREDBIT
      INTEGER*4 STATES(4)
      COMMON /SHARED/ STATES
      LOGICAL*4 LIB$BBSSI
      CALL LIB$BBSSI(0, STATES)
      END

C     LIB$SIG_TO_RET as the handler: the condition, here the hardware's, is the
C     value of the function the frame belongs to.
      INTEGER*4 FUNCTION DIVIDE(N, M)
      EXTERNAL LIB$SIG_TO_RET
      CALL LIB$ESTABLISH(LIB$SIG_TO_RET)
      DIVIDE = N / M
      END

      SUBROUTINE HALVES(FLOATS)
      INCLUDE '($SSDEF)'
      REAL*4 FLOATS(2)
      INTEGER*4 RECIP
      PRINT *, RECIP(FLOATS(1)) .EQ. 1,
     1         RECIP(FLOATS(2)) .EQ. SS$_FLTDIV_F
      END

      INTEGER*4 FUNCTION RECIP(V)
      EXTERNAL LIB$SIG_TO_RET
      CALL LIB$ESTABLISH(LIB$SIG_TO_RET)
      V = 1.0 / V
      RECIP = 1
      END

C     The established handler stays for the call of FTNRTL, whose error reaches it.
      INTEGER*4 FUNCTION OPENIT(NAME)
      CHARACTER*(*) NAME
      EXTERNAL FINDER
      OPENIT = 77
      CALL LIB$ESTABLISH(FINDER)
      OPEN (UNIT=1, FILE=NAME, STATUS='OLD')
      PRINT *, 'PAST THE OPEN'
      END

      INTEGER*4 FUNCTION FINDER(SIGARGS, MECHARGS)
      INCLUDE '($SSDEF)'
      INCLUDE '($FORDEF)'
      INTEGER*4 SIGARGS(*), MECHARGS(*), WHICH, LIB$MATCH_COND
      INTEGER*4 EXTRA, DEPTH
      COMMON /UNWIND/ EXTRA
      FINDER = SS$_RESIGNAL
      WHICH = LIB$MATCH_COND(SIGARGS(2), FOR$_NO_SUCDEV, FOR$_FILNOTFOU)
C     WHICH is 0 when the unwind calls this handler itself, with SS$_UNWIND, its
C     own frame being one of those removed.
      PRINT *, 'WHICH', WHICH
      IF (WHICH .EQ. 2) THEN
         MECHARGS(4) = 55
         DEPTH = MECHARGS(3) + EXTRA
         CALL SYS$UNWIND(DEPTH,)
         PRINT *, 'UNWINDING'
      END IF
      END

      SUBROUTINE MESSAGES
      INCLUDE '($SSDEF)'
      EXTERNAL KEEPER
      CALL LIB$ESTABLISH(KEEPER)
      CALL LIB$SIGNAL(%VAL(SS$_ABORT))
      PRINT *, 'CONTINUED'
      END

      INTEGER*4 FUNCTION KEEPER(SIGARGS, MECHARGS)
      INCLUDE '($SSDEF)'
      INTEGER*4 SIGARGS(*), MECHARGS(*)
      EXTERNAL SHOWLINE
C     The signal's PC and PSL aren't part of the message.
      SIGARGS(1) = SIGARGS(1) - 2
      CALL SYS$PUTMSG(SIGARGS, SHOWLINE,)
      SIGARGS(1) = SIGARGS(1) + 2
      KEEPER = SS$_CONTINUE
      END

      LOGICAL*4 FUNCTION SHOWLINE(LINE)
      CHARACTER*(*) LINE
      PRINT *, 'LINE: ', LINE
      SHOWLINE = .FALSE.
      END
