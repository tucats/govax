C     Timer ASTs landing all over statements (Phase 83, subtask 7): every statement's
C     IOSTAT is its own, the interrupted statement's and the AST's. Before FTNRTL held
C     ASTs off, an AST's statement could take the context of one just ended and give it
C     its own status. Where they land doesn't show in the output.
      PROGRAM ASTRACE
      EXTERNAL TICK
      INTEGER SYS$SETIMR, SYS$CANTIM
      INTEGER DELTA(2), IOS, BAD, I, J
      CHARACTER*12 BUF, WRONG
      COMMON /FLAGS/ NTICK, NBAD, NARMED
      DATA DELTA /-1000, -1/, WRONG /'12X4'/
C     The main line sets each timer, not the AST: an AST longer than the interval
C     would otherwise always find its next one expired, and nothing else would run.
      DO 100 J = 1, 2000
          IF (NARMED .EQ. 0) THEN
              NARMED = 1
              DELTA(1) = -1000 - MOD(NTICK, 97) * 100
              I = SYS$SETIMR(, DELTA, TICK,)
          END IF
          WRITE (BUF, '(I6)', IOSTAT=IOS) J
          IF (IOS .NE. 0) BAD = BAD + 1
          READ (WRONG, '(I4)', IOSTAT=IOS) I
          IF (IOS .EQ. 0) BAD = BAD + 1
100   CONTINUE
      I = SYS$CANTIM(,)
      PRINT *, 'WRONG', BAD, NBAD
      PRINT *, 'ANY TICKS', NTICK .GT. 0
      END

C     Reads a field it can't and writes one it can.
      SUBROUTINE TICK
      INTEGER IOS, K
      CHARACTER*12 TEXT, WRONG
      COMMON /FLAGS/ NTICK, NBAD, NARMED
      DATA WRONG /'9Q'/
      NTICK = NTICK + 1
      READ (WRONG, '(I2)', IOSTAT=IOS) K
      IF (IOS .EQ. 0) NBAD = NBAD + 1
      WRITE (TEXT, '(I6)', IOSTAT=IOS) NTICK
      IF (IOS .NE. 0) NBAD = NBAD + 1
      NARMED = 0
      END
