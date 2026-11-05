C DBG89F.FOR - the FORTRAN half of Phase 89's debugger probe (README.md): a
C recursive function with an argument and an automatic local in each frame,
C and no I/O statement, so VMS's MACRO and LINK build its MACRO file with no
C run-time library. FACT(4) is 4*3*2*10 = 240.
      PROGRAM DBG89F
      INTEGER FACT, R
      R = FACT(4)
      END
      RECURSIVE INTEGER FUNCTION FACT(N)
      INTEGER N, K
      K = N * 10
      IF (N .LE. 1) GO TO 10
      FACT = N * FACT(N-1)
      RETURN
 10   FACT = K
      RETURN
      END
