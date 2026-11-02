C     OPTIONS /EXTEND_SOURCE (Phase 85, subtask 6): the statement field runs to column 132 for
C     the unit after the statement, to its END, and the next unit is as the command said: in
C     SHORT, columns 73 on are a sequence field, and a statement read past 72 wouldn't compile.
      OPTIONS /EXTEND_SOURCE
      PROGRAM EXTSRC
      INTEGER I, N
      I = 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 + 11 + 12 + 13 + 14 + 15 + 16 + 17 + 18 + 19 + 20 + 21 + 22 + 23 + 24 + 25 + 26
      PRINT *, 'WIDE', I
      N = LEN('XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX')
      PRINT *, 'LENGTH', N
      CALL SHORT
      CALL AGAIN
      END
      SUBROUTINE SHORT
      INTEGER I
      I = 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 + 11 + 12 + 13 + 14    XX000010
      PRINT *, 'SHORT', I                                               ) SEQ 20
      RETURN                                                            ( SEQ 30
      END
      OPTIONS /NOCHECK/EXTEND_SOURCE
      SUBROUTINE AGAIN
      INTEGER I
      I = 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 + 11 + 12 + 13 + 14 + 15 + 16 + 17 + 18 + 19 + 20 + 21 + 22
      PRINT *, 'AGAIN', I
      END
