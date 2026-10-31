      PROGRAM STRINGS
C     CHARACTER arguments and concatenation: variables, constants,
C     substrings, elements, and expressions passed by descriptor;
C     passed-length dummies and one of a declared length; a dummy
C     changed, and a parenthesized argument's copy; arrays passed whole,
C     from an element, passed on, and seen with shorter elements;
C     concatenations of known length in a temporary, one that overlaps
C     its target, and ones of a passed-length dummy in a dynamic string.
      CHARACTER*8 S, T*3, NAME*12, LIST(3)*4, W*6
      INTEGER I
C     Variables, constants, substrings, and elements passed.
      S = 'ABCDEFGH'
      CALL SHOW(S)
      CALL SHOW('LITERAL')
      CALL SHOW(S(3:5))
      I = 2
      CALL SHOW(S(I:I+3))
      LIST(1) = 'ONE'
      LIST(2) = 'TWO'
      LIST(3) = 'SIX'
      CALL SHOW(LIST(I))
C     A dummy of a declared length has the first characters.
      CALL FIRST3(S)
C     Concatenation.
      T = 'XYZ'
      NAME = S(1:2) // '-' // T
      PRINT *, '[', NAME, ']'
      CALL SHOW(T // T)
      CALL SHOW(S(:I) // '+' // S(I+5:))
      PRINT *, T // '.' // S(6:), '|'
C     A concatenation that overlaps its target.
      S = 'ABCDEFGH'
      S = S(2:) // S(1:1)
      PRINT *, S
      S = 'ABCD' // S
      PRINT *, S
C     A dummy changed: the caller's variable, a substring, an element.
      W = 'ABCDEF'
      CALL FILL(W, '*')
      PRINT *, W
      W = 'ABCDEF'
      CALL FILL(W(2:4), '#')
      PRINT *, W
      CALL FILL(LIST(2), '?')
      PRINT *, LIST, '|'
C     A parenthesized argument is a copy, which the routine changes.
      W = 'ABCDEF'
      CALL FILL((W), '!')
      PRINT *, W
C     Arrays passed whole and from an element.
      LIST(2) = 'TWO'
      CALL PAIR(LIST)
      CALL PAIR(LIST(2))
      CALL WHOLE(LIST)
      CALL SHORT(LIST)
C     Passed on, and a passed-length dummy concatenated.
      CALL WRAP('MIDDLE')
      CALL WRAP(S(1:I))
      END

      SUBROUTINE SHOW(X)
      CHARACTER*(*) X
      PRINT *, '[', X, ']'
      END

      SUBROUTINE FIRST3(X)
      CHARACTER*3 X
      PRINT *, '[', X, ']'
      CALL SHOW(X)
      END

      SUBROUTINE FILL(X, C)
C     Each character of X made C's.
      CHARACTER*(*) X, C*1
      X = C // C // C // C // C // C // C // C
      END

      SUBROUTINE PAIR(A)
C     A passed-length array: whole, its elements, a substring of one.
      CHARACTER*(*) A(2)
      PRINT *, A, '|'
      PRINT *, A(2), A(1)(2:3), '|'
      A(2)(1:1) = '>'
      END

      SUBROUTINE WHOLE(A)
C     An assumed-size passed-length array, passed on whole.
      CHARACTER*(*) A(*)
      PRINT *, A(1), A(3), '|'
      CALL PAIR(A)
      END

      SUBROUTINE SHORT(B)
C     An array of shorter elements over the caller's.
      CHARACTER*2 B(3)
      PRINT *, B, '|'
      CALL SHOW(B(2))
      END

      SUBROUTINE WRAP(X)
      CHARACTER*(*) X
      CHARACTER*20 R
      CALL SHOW(X)
      CALL SHOW(X(2:))
      R = '<' // X // '>'
      PRINT *, '[', R, ']'
      PRINT *, '<' // X // '>', '|'
      CALL SHOW(X // X)
      IF (X // 'E' .EQ. 'MIDDLEE') PRINT *, 'MIDDLE seen'
      IF (X(1:1) // X .NE. 'MMIDDLE') THEN
         PRINT *, 'not MIDDLE'
      END IF
      END
