      PROGRAM OPTAS
C     A variable an AST routine changes (Phase 84's Decisions 7 and 9): section 6.2
C     of the manual lets no routine run twice at once, and an AST that does it
C     anyway, or changes a variable behind the code's back, must say so by VOLATILE.
C     Each case reads wrong if a value is kept past the AST.
      INTEGER SYS$SETIMR, SYS$DCLAST, DELTA(2), ST, SPINS
      EXTERNAL SETFL, BUMP
      COMMON /FLAGS/ NFLAG, NBUMP
      VOLATILE /FLAGS/
      DATA DELTA /-100000, -1/
C
C     A routine run again by an AST, changing a static variable no call is given.
      CALL REENT(1)
C
C     COMMON changed during a call that doesn't name it.
      NBUMP = 1
      ST = SYS$DCLAST(BUMP,,)
      KEPT = NBUMP
      PRINT *, 'BUMP', KEPT
C
C     A loop with no call in it, which only an AST can end. Without VOLATILE the
C     test would be moved out of the loop, and the program would never stop.
      NFLAG = 0
      SPINS = 0
      ST = SYS$SETIMR(, DELTA, SETFL,)
   10 IF (NFLAG .NE. 0) GO TO 20
      SPINS = SPINS + 1
      GO TO 10
   20 PRINT *, 'FLAG', NFLAG, SPINS .GT. 0
      END
C
      SUBROUTINE REENT(LEVEL)
      EXTERNAL AGAIN
      INTEGER SYS$DCLAST, STAT, I, K
      VOLATILE STAT
      STAT = LEVEL * 10
      IF (LEVEL .EQ. 1) THEN
         I = SYS$DCLAST(AGAIN,,)
         K = STAT
         PRINT *, 'REENT', K
      END IF
      END
C
      SUBROUTINE AGAIN
      CALL REENT(2)
      END
C
      SUBROUTINE BUMP
      COMMON /FLAGS/ NFLAG, NBUMP
      NBUMP = NBUMP + 1
      END
C
      SUBROUTINE SETFL
      COMMON /FLAGS/ NFLAG, NBUMP
      NFLAG = 7
      END
