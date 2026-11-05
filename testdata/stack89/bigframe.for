C     BIGFRAME: a recursive routine whose frame is 800,000 bytes (1,563
C     pages), every page of it written before the routine recurses and
C     read back after, at depths 1 to 20 in turn. Each depth's line says
C     how many of its values came back wrong (none). With paging off the
C     user stack is a fixed 2 MB, and depth 3 doesn't fit; with paging on
C     it grows to the table area's room, about 10 MB, and depth 13
C     doesn't. Either way the image ends with no message and $STATUS
C     %X1000000C: an access violation with no stack to report it on
C     (docs/PHASE-89 - reentrant FORTRAN.md, subtask 9).
      PROGRAM BIGFRAME
      INTEGER DEPTH, WRONG
      DO DEPTH = 1, 20
         WRONG = 0
         CALL DIVE(DEPTH, WRONG)
         PRINT 10, DEPTH, WRONG
      END DO
10    FORMAT (' DEPTH', I3, ' WRONG', I6)
      END

      RECURSIVE SUBROUTINE DIVE(N, WRONG)
      INTEGER N, WRONG, I
      REAL*8 A(100000)
      DO I = 1, 100000, 64
         A(I) = N * 1000000 + I
      END DO
      IF (N .GT. 1) CALL DIVE(N - 1, WRONG)
      DO I = 1, 100000, 64
         IF (A(I) .NE. N * 1000000 + I) WRONG = WRONG + 1
      END DO
      END
