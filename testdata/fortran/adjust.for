C     Adjustable, assumed-size, and passed-length arrays (sections 6.1.1.1
C     to 6.1.1.3): the manual's SUM and its X(-I/2:I/2,J), bounds in a
C     common block, lower bounds of every sign, an ENTRY's own list, every
C     element type, whole arrays written and passed on, and SIZEOF.
      PROGRAM ADJUST
      DIMENSION A1(10,35), A2(3,56), ARRAY(9,5), B(10)
      INTEGER M(3,4), V(12)
      EQUIVALENCE (M, V)
      REAL*8 D(4)
      COMPLEX C(2,2)
      COMPLEX*16 CD(3)
      INTEGER*2 W(6)
      CHARACTER*3 NAMES(2,3)
      CHARACTER*2 SHORT(4)
      CHARACTER*3 PICK
      COMMON /SHAPE/ NROWS, NCOLS
C
C     The manual's sample calls on SUM: the same array, two shapes.
      DO 10 J = 1, 35
      DO 10 I = 1, 10
10    A1(I,J) = I + 100*J
      DO 12 J = 1, 56
      DO 12 I = 1, 3
12    A2(I,J) = 1.0
      PRINT *, SUM(A1,10,35), SUM(A2,3,56), SUM(A1,10,10)
C
C     X is X(-4:4,5), whatever SUB does to I and J afterwards.
      L = 9
      K = 5
      DO 14 J = 1, 5
      DO 14 I = 1, 9
14    ARRAY(I,J) = 0
      CALL SUB(ARRAY,L,K)
      PRINT *, L, K, ARRAY(9,5), ARRAY(1,5), ARRAY(9,1)
C
C     A vector seen as matrices of two shapes, by column.
      DO 20 I = 1, 12
20    V(I) = I
      CALL SHOW(M,3,4)
      CALL SHOW(M,4,3)
      CALL SHOW(M,6,2)
C
C     Bounds in a common block, set before the call.
      NROWS = 2
      NCOLS = 6
      CALL FROMC(V)
      NROWS = 6
      NCOLS = 2
      CALL FROMC(V)
C
C     Lower bounds, and an array that starts with a bound of its own.
      CALL LOWER(V,-1,1,2,3)
      CALL LOWERC(V,2)
C
C     An ENTRY's own list (the manual's S and S1).
      DO 30 I = 1, 10
30    B(I) = 0
      CALL S(B,2,3.0)
      CALL S1(5,B,3,2)
      PRINT *, B(2), B(5)
      CALL S2(4.0)
C
C     Falling into an ENTRY statement isn't entering: the bounds stay.
      N = 12
      CALL FALL(V,N)
      PRINT *, N
      N = 2
      CALL FALL2(V,N)
C
C     Every element type, its subscripts' scale the element's size.
      D(1) = 1.5D0
      D(2) = 2.5D0
      D(3) = 3.5D0
      D(4) = 4.5D0
      CALL DBLS(D,2,2)
      C(1,1) = (1.0,2.0)
      C(2,1) = (3.0,4.0)
      C(1,2) = (5.0,6.0)
      C(2,2) = (7.0,8.0)
      CALL CPLX(C,2)
      CD(1) = (1.0D0,-1.0D0)
      CD(2) = (2.0D0,-2.0D0)
      CD(3) = (3.0D0,-3.0D0)
      CALL CPLXD(CD,3)
      PRINT *, CD(2)
      W(4) = 7
      CALL SHORTS(W,2,3)
      PRINT *, W(4), W(6)
C
C     CHARACTER arrays: a declared length and passed lengths.
      NAMES(1,1) = 'ONE'
      NAMES(2,1) = 'TWO'
      NAMES(1,2) = 'SIX'
      NAMES(2,2) = 'TEN'
      NAMES(1,3) = 'FOX'
      NAMES(2,3) = 'DOG'
      CALL CHARS(NAMES,2,3)
      CALL PASSED(NAMES,2,3)
      CALL PASSED(NAMES,3,2)
      CALL ASSUME(NAMES,2)
      SHORT(1) = 'AB'
      SHORT(2) = 'CD'
      SHORT(3) = 'EF'
      SHORT(4) = 'GH'
      CALL PASSED(SHORT,2,2)
      CALL LENGTHS(SHORT,4)
C
C     A whole array passed on, and an element as the array's start.
      CALL OUTER(V,3,4)
      CALL SHOW(V(4),3,3)
C
C     An array's size in bytes, and the empty array.
      CALL SIZES(V,3,4)
      CALL SIZES(V,0,4)
C
C     An entry point that defines an array the header's list doesn't, and
C     a function's hidden argument moving the dummies along by one.
      CALL LATE
      CALL LATE1(B,2)
      PRINT *, PICK(NAMES,2,3)
      END

      SUBROUTINE LATE
      DIMENSION A(N)
      PRINT *, 'LATE'
      RETURN
      ENTRY LATE1(A,N)
      PRINT *, A(N)
      END

      CHARACTER*3 FUNCTION PICK(A,M,N)
      CHARACTER*3 A(M,N)
      PICK = A(M,N)
      END

      FUNCTION SUM(A,M,N)
      DIMENSION A(M,N)
      SUM = 0.0
      DO 10 J = 1, N
      DO 10 I = 1, M
10    SUM = SUM + A(I,J)
      RETURN
      END

      SUBROUTINE SUB(X,I,J)
      DIMENSION X(-I/2:I/2,J)
      X(I/2,J) = 999
      J = 1
      I = 2
      X(-4,5) = 111
      X(4,1) = 222
      END

      SUBROUTINE SHOW(A,M,N)
      INTEGER A(M,N)
      PRINT *, 'A', M, N, A(1,1), A(M,1), A(1,N), A(M,N)
      PRINT *, A
      END

      SUBROUTINE FROMC(A)
      COMMON /SHAPE/ NROWS, NCOLS
      INTEGER A(NROWS,NCOLS)
      PRINT *, A(2,2), A(NROWS,NCOLS)
      END

      SUBROUTINE LOWER(A,L1,U1,L2,U2)
      INTEGER L1, U1, L2, U2
      INTEGER A(L1:U1,L2:U2,0:1)
      PRINT *, A(-1,2,0), A(1,2,0), A(0,3,0), A(-1,2,1), A(1,3,1)
      PRINT *, A(L1+1,L2,0), A(U1,U2,1)
      END

      SUBROUTINE LOWERC(A,N)
      INTEGER A(0:N,-1:*)
      PRINT *, A(0,-1), A(2,-1), A(0,0), A(1,1), A(N,2)
      END

      SUBROUTINE S(A,I,X)
      DIMENSION A(I)
      A(I) = X
      RETURN
      ENTRY S1(I,A,K,L)
      A(I) = A(I) + 1.0
      RETURN
      ENTRY S2(X)
      PRINT *, X
      END

      SUBROUTINE FALL(A,N)
      INTEGER A(N)
      N = 1
      ENTRY FALL2(A,N)
      PRINT *, A
      END

      SUBROUTINE DBLS(A,M,N)
      REAL*8 A(M,N)
      PRINT *, A(1,1), A(2,1), A(1,2), A(2,2)
      PRINT *, A
      END

      SUBROUTINE CPLX(A,N)
      COMPLEX A(N,N)
      PRINT *, A(2,1), A(1,2)
      PRINT *, A(2,2)*A(1,1)
      END

      SUBROUTINE CPLXD(A,N)
      COMPLEX*16 A(N)
      PRINT *, A(N)
      A(2) = A(1) + A(3)
      END

      SUBROUTINE SHORTS(A,M,N)
      INTEGER*2 A(M,N)
      A(2,3) = A(2,2) + 1
      END

      SUBROUTINE CHARS(A,M,N)
      CHARACTER*3 A(M,N)
      PRINT *, A(1,1), A(M,N), A(2,2)(2:3)
      PRINT *, A
      CALL PASSED(A,M,N)
      END

      SUBROUTINE PASSED(A,M,N)
      CHARACTER*(*) A(M,N)
      PRINT *, LEN(A(1,1)), A(M,N), ' ', A(1,N)(2:)
      PRINT *, A
      A(1,1) = A(1,1)
      END

      SUBROUTINE ASSUME(A,M)
      CHARACTER*(*) A(M,*)
      PRINT *, A(1,2), A(2,3), LEN(A(1,1))
      END

      SUBROUTINE LENGTHS(A,N)
      CHARACTER*(*) A(0:N-1)
      CHARACTER*1 C
      PRINT *, A(0), A(N-1), LEN(A(0))
      C = A(2)
      PRINT *, C
      END

      SUBROUTINE OUTER(A,M,N)
      INTEGER A(M,N)
      CALL SHOW(A,M,N)
      CALL SHOW(A(1,2),M,N-1)
      END

      SUBROUTINE SIZES(A,M,N)
      INTEGER A(M,N)
      PRINT *, SIZEOF(A)
      END
