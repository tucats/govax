C     Fortran 90's INTEGER(1) (Phase 100, subtask 5): INTEGER*1 and
C     INTEGER(1), operations of two bytes in a byte, a byte beside a wider
C     type in that type (a constant is a longword), the generic functions
C     through INTEGER*2 and back, INT by the default integer, a specific
C     name's own result, and a power.
      PROGRAM F90BYTES
      INTEGER*1 I
      INTEGER(1) :: J = -5_1, K
      INTEGER*2 W
      LOGICAL*1 L
      K = I + J
      K = I * J - 3_1
      W = I + W
      W = I + 5
      K = ABS(J)
      K = MAX(I, J)
      K = IAND(I, J)
      W = INT(J)
      W = IIABS(J)
      K = J ** I
      K = L + I
      IF (I .LT. J) K = -K
      PRINT *, I, J, K
      END
