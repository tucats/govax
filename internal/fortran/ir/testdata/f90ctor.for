      OPTIONS /F90
C     Array constants and constructors (Phase 102, subtask 7): a constructor
C     of constants and an array PARAMETER read from their data, a section of
C     one folded, an element with a subscript not constant, RESHAPE, a
C     constructor of values filled straight into its target, one filled
C     into a temporary, with an implied DO, and initial values.
      PROGRAM F90CTR
      INTEGER, PARAMETER :: P(4) = (/ 2, 3, 5, 7 /)
      REAL, PARAMETER :: M(2, 2) = RESHAPE((/ 1.0, 2.0, 3.0, 4.0 /),
     1  (/ 2, 2 /))
      INTEGER, PARAMETER :: N2 = SIZE(M), Q(2) = P(2:3)
      CHARACTER*3, PARAMETER :: W(2) = (/ 'ONE', 'TWO' /)
      REAL :: A(4), B(2, 2), X
      INTEGER :: K(4) = (/ (I * I, I = 1, 4) /), I, J
      CHARACTER*3 C(2)
      A = (/ 1.0, 2.0, 3.0, 4.0 /)
      K = P + Q(1)
      J = P(I)
      B = M
      A = (/ X, (I * X, I = 1, 2), A(1) /)
      A(1:2) = (/ X, X + 1.0 /) + A(3:4)
      A(1:J) = (/ (X * I, I = 1, J) /)
      C = W
      END
