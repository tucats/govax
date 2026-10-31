C     A FORTRAN routine as the rest of VMS sees it (Phase 79, subtask 11, the
C     calling standard): a condition handler LIB$ESTABLISH names, called with the
C     signal and mechanism arrays, its value in R0 saying to continue; an AST
C     routine SYS$DCLAST names, its parameter by value, seen by %LOC; a function and
C     a subroutine LIB$CALLG calls with a list made by hand.
      PROGRAM CBACK
      EXTERNAL HANDLR, NOTE, SUMTWO, ADDTO
      INTEGER HANDLR, SUMTWO, LIB$CALLG, SYS$DCLAST, LIST(3), I, J, K
      CALL LIB$ESTABLISH(HANDLR)
      CALL LIB$SIGNAL(%VAL(1200))
      PRINT *, 'CONTINUED'
      K = SYS$DCLAST(NOTE, %VAL(7), %VAL(0))
      PRINT *, K
      I = 40
      J = 2
      LIST(1) = 2
      LIST(2) = %LOC(I)
      LIST(3) = %LOC(J)
      PRINT *, LIB$CALLG(LIST, SUMTWO)
      K = LIB$CALLG(LIST, ADDTO)
      PRINT *, I
      END

      INTEGER FUNCTION HANDLR(SIG, MECH)
      INTEGER SIG(*), MECH(*)
      PRINT *, 'SIGNAL', SIG(1), SIG(2)
      HANDLR = 1
      END

      SUBROUTINE NOTE(P)
      PRINT *, 'AST', %LOC(P)
      END

      INTEGER FUNCTION SUMTWO(A, B)
      INTEGER A, B
      SUMTWO = A + B
      END

      SUBROUTINE ADDTO(A, B)
      INTEGER A, B
      A = A + B
      END
