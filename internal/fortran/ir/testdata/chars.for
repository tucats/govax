C     CHARACTER values' instructions (Phase 78, subtask 5).
      PROGRAM CHARS
      CHARACTER*8 S, T*3, NAMES(2:4)*5
      LOGICAL L
      S = 'ABC'
      T = S
      S = T
      NAMES(I) = S(2:6)
      S(I:J) = T
      S(I+1:) = NAMES(3)(:J)
      L = S .EQ. T
      L = S .LT. 'X' .AND. L
      IF (S .GT. NAMES(I)) I = 1
      IF (T .EQ. 'ABC' .OR. .NOT. S(I:I) .NE. 'Q') I = 2
      IF (L .AND. T .LE. S) I = 3
      PRINT *, S, T(2:3), NAMES(I), S(I:J), NAMES
      END
