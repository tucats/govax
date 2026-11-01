C     AUTOMATIC, STATIC, and VOLATILE (Phase 82, subtask 6): AUTOMATIC names in
C     the frame area, an EQUIVALENCE group's together; the others in $LOCAL and
C     common blocks as ever; VOLATILE spreading through blocks and groups.
      SUBROUTINE LOCALS(ARG, N)
      STRUCTURE /PAIR/
        INTEGER*2 A
        REAL*8 B(2)
      END STRUCTURE
      AUTOMATIC I, K(0:4), T, R, RA, EQ1
      RECORD /PAIR/ R, RA(2)
      CHARACTER*7 T
      LOGICAL*1 EQ1
      INTEGER EQ2
      STATIC S
      SAVE S
      COMMON /BLK/ C1, C2
      EQUIVALENCE (EQ1, EQ2)
      EQUIVALENCE (C1, P1), (P1, P4)
      VOLATILE /BLK/, V, ARG, K
      I = N
      K(I) = 1
      T = 'FRAME'
      R.A = 1
      RA(I).B(2) = 2.0
      EQ2 = 3
      S = I
      V = S
      ARG = V
      C2 = P4
      END
