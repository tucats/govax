C     AUTOMATIC variables reached through FP, descriptors of them filled in at
C     run time, each entry point reserving the frame, a variable format expression
C     reaching them through a cell; VOLATILE places marked (Phase 82, subtask 6).
      SUBROUTINE LOCALS(ARG, N, TEXT)
      STRUCTURE /PAIR/
        INTEGER*2 A
        REAL*4 B(2)
      END STRUCTURE
      AUTOMATIC I, K(0:4), T, R, RA
      RECORD /PAIR/ R, RA(2)
      CHARACTER*7 T
      CHARACTER*(*) TEXT
      DIMENSION ARG(N)
      VOLATILE I, ARG, TEXT, V, R
      I = N
      K(I) = K(2) + 1
      T = 'FRAME'
      R.A = I
      RA(I).B(2) = 2.0
      RA(1) = R
      ARG(I) = V
      TEXT = T
      PRINT *, I, K, T, K(I), R.A
      CALL S(I, K, T, R)
      WRITE (T, '(I7)') I
      ENTRY OTHER(N, ARG)
      WRITE (6, 100) I
  100 FORMAT (I<I + K(2)>, A<LEN(T)>)
      END
