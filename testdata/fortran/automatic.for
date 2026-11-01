C     AUTOMATIC, STATIC, and VOLATILE (Phase 82, subtask 6): variables in the
C     stack, whatever kind of statement uses them.
      PROGRAM AUTOS
      EXTERNAL IFACT, ISFACT
      AUTOMATIC IMAIN
      CHARACTER*12 TEXT
      COMMON /SHARED/ IC, ID
      VOLATILE /SHARED/, V
      STATIC SV
      IMAIN = 11
      CALL KINDS
      CALL ENTRYA(4)
      CALL ENTRYB(5, 6)
      CALL ENTRYA(7)
      PRINT *, 'Frames:', IFACT(5, IFACT), ISFACT(5, ISFACT)
      CALL WIDTHS(3)
      CALL OVERLAY
      CALL BIG(30000)
      CALL ONSTACK(TEXT)
      PRINT *, TEXT
      IC = 2
      ID = IC + 1
      V = ID * 2.0
      SV = V + 1
      PRINT *, IMAIN, IC, ID, V, SV
      END
C     Every form of reference.
      SUBROUTINE KINDS
      STRUCTURE /PAIR/
        INTEGER*2 A
        INTEGER*4 B(3)
        CHARACTER*3 C
      END STRUCTURE
      AUTOMATIC I, X, D, K(0:4), M(2,3), S, SS(3), R, RR(2), J, L, IL, Z
      AUTOMATIC N(3), Q, B, W
      REAL*8 D
      CHARACTER*6 S, SS
      RECORD /PAIR/ R, RR, Q
      COMPLEX Z
      LOGICAL*1 B
      INTEGER*2 W
      F(Y) = Y + X
      I = 7
      X = I / 2.0
      D = X * 2.0D0
      B = .TRUE.
      W = -3
      PRINT *, I, X, D, B, W
      DO 10 J = 0, 4
        K(J) = J * J
   10 CONTINUE
      PRINT *, K(3), K
      DO 20 J = 1, 2
        DO 20 L = 1, 3
   20 M(J, L) = 10 * J + L
      PRINT *, M(2, 3), M
      S = 'ABCDEF'
      SS(1) = S(2:3) // 'XY'
      SS(2) = SS(1)(3:) // S
      SS(3) = S
      PRINT *, S, ' ', SS(1), ' ', SS(2), ' ', SS
      R.A = 1
      R.B(2) = 22
      R.C = 'RST'
      RR(2) = R
      Q = RR(2)
      J = 2
      RR(J).B(J) = Q.B(2) + 1
      PRINT *, R.A, RR(2).A, RR(2).B(2), R.B(2), RR(2).C
      Z = (1.0, 2.0)
      Z = Z * Z
      PRINT *, Z, F(1.0)
      CALL SET(N, 3)
      PRINT *, N
      PRINT *, ISUM(N, 3), ISUM(N(2), 2), ILEN(S), ILEN(SS(2)(1:2))
      PRINT *, ICOUNT(SS, 3), %LOC(N(2)) - %LOC(N(1)), SIZEOF(N)
      WRITE (S, '(I6)') I * 100
      PRINT *, S
      READ (S, '(I6)') IL
      PRINT *, IL + 1
      ASSIGN 30 TO L
      GO TO L, (30, 40)
   40 PRINT *, 'WRONG'
   30 PRINT *, 'ASSIGNED'
      END
      SUBROUTINE SET(N, K)
      DIMENSION N(K)
      DO 10 I = 1, K
   10 N(I) = I * 5
      END
      INTEGER FUNCTION ISUM(N, K)
      DIMENSION N(K)
      ISUM = 0
      DO 10 I = 1, K
   10 ISUM = ISUM + N(I)
      END
      INTEGER FUNCTION ILEN(S)
      CHARACTER*(*) S
      ILEN = LEN(S)
      END
      INTEGER FUNCTION ICOUNT(S, K)
      CHARACTER*(*) S(K)
      ICOUNT = K * LEN(S(1))
      END
C     Each entry point reserves the area, with no effect on the arguments.
      SUBROUTINE ENTRYA(N)
      AUTOMATIC T, A
      DIMENSION A(2)
      T = N
      A(2) = T * 2
      PRINT *, 'A', N, T, A(2)
      RETURN
      ENTRY ENTRYB(M, N)
      T = M + N
      A(1) = T + 1
      PRINT *, 'B', M, N, T, A(1)
      END
C     Each call has its own: IFACT calls itself through its argument, the only
C     way there is, and M has its value after the call. ISFACT's M is static,
C     so the innermost call's value is everyone's.
      INTEGER FUNCTION IFACT(N, F)
      EXTERNAL F
      INTEGER F
      AUTOMATIC M
      M = N
      IF (M .LE. 1) THEN
        IFACT = 1
      ELSE
        IFACT = F(M - 1, F)
        IFACT = M * IFACT
      END IF
      END
      INTEGER FUNCTION ISFACT(N, F)
      EXTERNAL F
      INTEGER F
      STATIC M
      M = N
      IF (M .LE. 1) THEN
        ISFACT = 1
      ELSE
        ISFACT = F(M - 1, F)
        ISFACT = M * ISFACT
      END IF
      END
C     A variable format expression runs from FTNRTL, with another FP.
      SUBROUTINE WIDTHS(N)
      AUTOMATIC IW, T, TA
      DIMENSION TA(2)
      CHARACTER*2 T
      IW = N + 2
      TA(2) = 4
      T = '12'
      WRITE (6, 100) N, N
  100 FORMAT (1X, I<IW>, I<TA(2) + IW>)
      WRITE (6, 110) N
  110 FORMAT (1X, I<ILEN(T) + N>)
      END
C     EQUIVALENCE puts a group in the frame together.
      SUBROUTINE OVERLAY
      AUTOMATIC I
      INTEGER*2 W(2)
      CHARACTER*4 C
      EQUIVALENCE (I, W), (W(1), C)
      I = 16705
      I = I + 65536 * 16962
      PRINT *, W(1), W(2), C
      W(2) = 17219
      PRINT *, I, C
      END
C     A frame of many pages, its last page first touched far below the stack.
      SUBROUTINE BIG(N)
      AUTOMATIC IA
      DIMENSION IA(30000)
      DO 10 I = 1, N, 1000
   10 IA(I) = I
      IA(N) = -1
      ISUM2 = 0
      DO 20 I = 1, N, 1000
   20 ISUM2 = ISUM2 + IA(I)
      PRINT *, ISUM2, IA(N)
      END
C     A CHARACTER variable in the frame, moved to the caller's.
      SUBROUTINE ONSTACK(OUT)
      CHARACTER*(*) OUT
      AUTOMATIC LOCAL
      CHARACTER*5 LOCAL
      LOCAL = 'HELLO'
      OUT = LOCAL // ' ' // LOCAL(1:2)
      END
