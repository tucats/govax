      PROGRAM STRS
      CHARACTER*8 S, LIST(3)*4
      S = 'ABCDEFGH'
      I = 2
      CALL SHOW(S, 'LITERAL', S(I:I+3), LIST(I), (S))
      S = S(2:) // S(:I)
      CALL PAIR(LIST)
      END

      SUBROUTINE SHOW(X, Y, Z, U, V)
      CHARACTER*(*) X, Y, Z, V
      CHARACTER*3 U
      CHARACTER*20 R
      X(2:) = Y
      R = '<' // X // '>'
      PRINT *, X, U, X // X
      CALL PAIR(X)
      IF (Z // 'E' .EQ. U) RETURN
      END

      SUBROUTINE PAIR(A)
      CHARACTER*(*) A(2)
      PRINT *, A, A(2)(I:)
      END
