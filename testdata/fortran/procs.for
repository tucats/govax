C     Procedures as arguments (sections 4.7, 4.9, 6.1.1), null arguments,
C     Hollerith constants, and the built-in functions (section 6.1.2).
      PROGRAM PROCS
      EXTERNAL DOUBLE, SHOW, IPLUS, QHALF, ZSWAP, ADDEM, STARS, LETTER
      EXTERNAL ENTRY2, ALTS, SUBS, SHOWN, SHOWV
      INTRINSIC SIN, ABS, MOD, CHAR, LEN, DSQRT, QSQRT
      CHARACTER*5 STARS
      CHARACTER*1 LETTER
      CHARACTER*12 T
      CHARACTER*8 S
      REAL*16 QHALF, Q
      COMPLEX*16 ZSWAP, Z
      REAL*8 D
      LOGICAL*1 L
      BYTE B
      INTEGER*2 N
      DIMENSION A(4)
      CHARACTER*2 CA(3)
C     A function, a subroutine, and intrinsics, through dummies
      PRINT *, APPLY(DOUBLE, 3.0), APPLY(ABS, -4.0)
      PRINT *, APPLY(SIN, 0.0)
      CALL CALLIT(SHOW, 7)
      PRINT *, IAPPLY(IPLUS, 2, 5), IAPPLY(MOD, 17, 5)
C     Passed on through a second routine, EXTERNAL or called before
      PRINT *, ONWARD(DOUBLE, 8.0), ONWARD2(DOUBLE, 9.0)
C     Through hidden arguments: REAL*16 and COMPLEX*16
      Q = 5
      CALL QAPPLY(QHALF, Q)
      CALL QAPPLY(QSQRT, Q)
      Z = (1D0, 2D0)
      CALL ZAPPLY(ZSWAP, Z)
      D = 16D0
      CALL DAPPLY(DSQRT, D)
C     CHARACTER functions: the length this unit declares goes with them
      CALL TEXTS(STARS, 2)
      T = 'BEFORE'
      CALL TEXTS2(STARS, T)
      PRINT *, T
      CALL ONE(LETTER)
      CALL ONE(CHAR)
      PRINT *, LENOF(LEN)
C     An entry point, and a dummy procedure reached through ENTRY's cells
      CALL CALLIT(ENTRY2, 11)
      CALL VIA1(1.0)
      CALL VIA2(SHOW, 12)
      CALL VIA2(ENTRY2, 13)
C     Alternate returns from a dummy procedure
      CALL CHOOSE(ALTS, 1)
      CALL CHOOSE(ALTS, 2)
      CALL CHOOSE(ALTS, 3)
C     Null arguments
      CALL NULLS(X, , 3)
      CALL NULLS(, Y, )
      CALL NULLS(, , )
C     Hollerith and character constants for numbers; the manual's example,
C     where the routine is a dummy, so the character constant stays one
      CALL NUMBER(4HABCD)
      CALL NUMBER('ABCD')
      CALL NUMBER(%REF('WXYZ'))
      CALL TEXTLEN('ABCD')
      CALL TEXTLEN(%DESCR('ABCDEF'))
      CALL STEXT(SUBS, SHOWN, A, 2.5)
C     %VAL: the routine finds the value where an address would be
      I = 300
      N = -2
      L = .TRUE.
      B = 200
      CALL SHOWV(%VAL(I), %VAL(N), %VAL(L), %VAL(B))
      CALL SHOWV(%VAL(5), %VAL(I+1), %VAL(1.5), %VAL(ZEXT(N)))
      CALL SHOWV(%VAL(.FALSE.), %VAL(0), %VAL(%LOC(I)-%LOC(I)), %VAL(2))
      X = 1.5
      CALL BITSOF(%VAL(X))
C     %VAL(%LOC(X)) is what passing X does
      A(2) = 6.5
      CALL SETIT(%VAL(%LOC(A(2))))
      PRINT *, A(2)
C     %REF: the same as the default for numbers, and the address of the text
      S = 'ABCDEFGH'
      CALL SETIT(%REF(A(2)))
      PRINT *, A(2)
      CALL NUMBER(%REF(S))
      CALL NUMBER(%REF(S(5:8)))
      CALL NUMBER(%REF(S//'!'))
      CALL ADDRESS(%REF(S(2:3)), %LOC(S(2:3)))
      CALL ADDRESS(%REF(A), %LOC(A))
      CALL ADDRESS(%REF(CA), %LOC(CA))
      PRINT *, %LOC(S(2:3)) - %LOC(S), %LOC(A(2)) - %LOC(A)
      X = 1.5
      CALL SETIT(%REF((X)))
      PRINT *, X
      CALL NUMBER(%REF((S)))
C     %DESCR: a CHARACTER*(*) dummy finds the length and the address
      D = 2D0
      CALL DESC(%DESCR(I), %LOC(I))
      CALL DESC(%DESCR(D), %LOC(D))
      CALL DESC(%DESCR(N), %LOC(N))
      CALL DESC(%DESCR(A), %LOC(A))
      CALL DESC(%DESCR(A(3)), %LOC(A(3)))
      CALL DESC(%DESCR(S), %LOC(S))
      CALL DESC(%DESCR(CA), %LOC(CA))
      CALL DESC(%DESCR((I)), %LOC(I))
      CALL ASSUM(A)
      CALL ADJ(A, 4)
C     A procedure in each form: its address however it is reached
      CALL PROCAD(DOUBLE, %REF(DOUBLE), %VAL(%LOC(DOUBLE)))
      CALL ZEMS(%DESCR(DOUBLE), %DESCR(STARS), STARS, %LOC(DOUBLE),
     1  %LOC(STARS))
      CALL PASSED(DOUBLE, STARS)
      END

C     The routines passed
      REAL FUNCTION DOUBLE(X)
      DOUBLE = 2.0 * X
      END

      SUBROUTINE SHOW(I)
      PRINT *, 'SHOW', I
      RETURN
      ENTRY ENTRY2(I)
      PRINT *, 'ENTRY2', I
      END

      INTEGER FUNCTION IPLUS(I, J)
      IPLUS = I + J
      END

      REAL*16 FUNCTION QHALF(Q)
      REAL*16 Q
      QHALF = Q / 2
      END

      COMPLEX*16 FUNCTION ZSWAP(Z)
      COMPLEX*16 Z
      ZSWAP = DCMPLX(DIMAG(Z), DBLE(Z))
      END

      CHARACTER*(*) FUNCTION STARS(N)
      STARS = '*****'
      STARS(N+1:) = 'ABCDE'
      END

      CHARACTER*1 FUNCTION LETTER(I)
      LETTER = 'L'
      END

      REAL FUNCTION ADDEM(X, Y)
      ADDEM = X + Y
      END

      SUBROUTINE ALTS(K, *, *)
      RETURN K - 1
      END

C     The routines given them
      REAL FUNCTION APPLY(F, X)
      APPLY = F(X)
      END

      SUBROUTINE CALLIT(S, I)
      CALL S(I)
      END

      INTEGER FUNCTION IAPPLY(F, I, J)
      INTEGER F
      IAPPLY = F(I, J)
      END

      REAL FUNCTION ONWARD(F, X)
      EXTERNAL F
      ONWARD = APPLY(F, X)
      END

      REAL FUNCTION ONWARD2(F, X)
      Y = F(X)
      ONWARD2 = APPLY(F, Y)
      END

      SUBROUTINE QAPPLY(F, Q)
      REAL*16 F, Q
      PRINT *, F(Q)
      END

      SUBROUTINE ZAPPLY(F, Z)
      COMPLEX*16 F, Z, W
      W = F(Z)
      PRINT *, W
      END

      SUBROUTINE DAPPLY(F, D)
      REAL*8 F, D
      PRINT *, F(D)
      END

C     The length comes from the descriptor it was passed with
      SUBROUTINE TEXTS(F, N)
      CHARACTER*(*) F
      PRINT *, F(N), LEN(F(N))
      PRINT *, F(N)//'!'
      END

      SUBROUTINE TEXTS2(F, T)
      CHARACTER*(*) F, T
      EXTERNAL F
      T = F(3)
      CALL TEXTS(F, 4)
      END

      SUBROUTINE ONE(F)
      CHARACTER*1 F
      PRINT *, F(65)
      END

      INTEGER FUNCTION LENOF(F)
      INTEGER F
      LENOF = F('SEVEN')
      END

      SUBROUTINE VIA1(X)
      PRINT *, 'VIA1', X
      RETURN
      ENTRY VIA2(P, I)
      CALL P(I)
      END

      SUBROUTINE CHOOSE(P, K)
      CALL P(K, *10, *20)
      PRINT *, 'NO ALTERNATE RETURN'
      RETURN
10    PRINT *, 'FIRST'
      RETURN
20    PRINT *, 'SECOND'
      END

      SUBROUTINE NULLS(A, B, C)
      PRINT *, %LOC(A) .EQ. 0, %LOC(B) .EQ. 0, %LOC(C) .EQ. 0
      END

      SUBROUTINE NUMBER(I)
      PRINT *, I
      END

      SUBROUTINE TEXTLEN(S)
      CHARACTER*(*) S
      PRINT *, S, LEN(S)
      END

C     The manual's example (6.1.1.4)
      SUBROUTINE STEXT(CHARSUB, HOLLSUB, A, B)
      EXTERNAL CHARSUB, HOLLSUB
      CALL CHARSUB(A, 'STRING')
      CALL HOLLSUB(B, 6HSTRING)
      END

      SUBROUTINE SUBS(A, S)
      CHARACTER*(*) S
      PRINT *, 'CHARSUB ', S
      END

      SUBROUTINE SHOWN(B, H)
      INTEGER H
      PRINT *, 'HOLLSUB', B, H
      END

      SUBROUTINE SHOWV(A, B, C, D)
      PRINT *, %LOC(A), %LOC(B), %LOC(C), %LOC(D)
      END

      SUBROUTINE BITSOF(A)
      PRINT *, %LOC(A)
      END

      SUBROUTINE SETIT(X)
      X = X + 1.0
      END

      SUBROUTINE ADDRESS(X, I)
      PRINT *, %LOC(X) .EQ. I
      END

C     An assumed-size array by its first element, an adjustable one whole
      SUBROUTINE ASSUM(A)
      DIMENSION A(*)
      CALL DESC(%DESCR(A), %LOC(A))
      END

      SUBROUTINE ADJ(A, N)
      DIMENSION A(N)
      CALL DESC(%DESCR(A), %LOC(A))
      CALL ADDRESS(%REF(A), %LOC(A))
      END

      SUBROUTINE DESC(S, I)
      CHARACTER*(*) S
      PRINT *, LEN(S), %LOC(S) .EQ. I
      END

      SUBROUTINE PROCAD(F, G, H)
      EXTERNAL F, G, H
      PRINT *, %LOC(F) .EQ. %LOC(G), %LOC(F) .EQ. %LOC(H)
      END

C     A numeric procedure's descriptor is of the length 0, a CHARACTER one's
C     of the unit's; %LOC takes the address back out of a CHARACTER dummy
C     procedure, and the dummy procedure's %DESCR is a new one
      SUBROUTINE ZEMS(D1, D2, F, I, J)
      CHARACTER*(*) D1, D2, F
      EXTERNAL F
      PRINT *, LEN(D1), %LOC(D1) .EQ. I, LEN(D2), %LOC(D2) .EQ. J,
     1  %LOC(F) .EQ. J
      END

      SUBROUTINE PASSED(F, G)
      EXTERNAL F
      CHARACTER*(*) G
      EXTERNAL G
      CALL ZEMS2(%DESCR(F), %DESCR(G), %REF(F), %REF(G))
      END

      SUBROUTINE ZEMS2(D1, D2, P, Q)
      CHARACTER*(*) D1, D2
      PRINT *, LEN(D1), %LOC(D1) .EQ. %LOC(P), LEN(D2),
     1  %LOC(D2) .EQ. %LOC(Q)
      END
