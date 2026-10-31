C     Procedures passed between files: a routine of another file, an entry point,
C     a CHARACTER function of the file that calls it through a dummy
      PROGRAM PROCS
      EXTERNAL TWICE, HALF, ANOTE, NAMER
      CHARACTER*4 NAMER
      PRINT *, CALLF(TWICE, 4.0), CALLF(HALF, 4.0)
      CALL CALLS(ANOTE, 6)
      CALL CALLCH(NAMER)
      CALL CALLS(%VAL(%LOC(ANOTE)), 7)
      END

      SUBROUTINE ANOTE(I)
      PRINT *, 'ANOTE', I
      END
