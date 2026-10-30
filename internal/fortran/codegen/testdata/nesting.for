C     Blocks inside blocks, loops sharing a terminal statement, jumps.
      PROGRAM NEST
      INTEGER I, J, K, M(3,4)
      LOGICAL L
      L = .TRUE.
      DO 20 I = 1, 3
         DO 20 J = 1, 4
            M(I,J) = I + J
            IF (M(I,J) .GT. 5) GO TO 20
            M(I,J) = -M(I,J)
   20 CONTINUE
      K = 0
      IF (L) THEN
         DO 30 X = 0.5, 2.5, 0.5
            IF (X .EQ. 1.5) THEN
               K = K + 1
            ELSE IF (X .GT. 2) THEN
               GO TO 40
            END IF
   30    CONTINUE
   40 END IF
      IF (K) PRINT *, 'ODD', K
      IF (.NOT. L .OR. K .EQ. 2 * 3 - 5) K = 7 / 2
      L = K .AND. 6
      END
