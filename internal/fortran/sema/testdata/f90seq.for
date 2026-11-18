      OPTIONS /F90
C     Sections and array expressions given to dummies that aren't
C     assumed-shape (Phase 104, subtask 6): a section known to be contiguous,
C     passed as its first element's address; one that isn't, copied into a
C     temporary and back, but not back to an INTENT(IN) dummy; an expression,
C     never copied back; a CHARACTER section by a class A descriptor; an
C     assumed-shape array, and a section of one, tested when the program runs;
C     an assumed-shape array as an internal file; and a function reference
C     whose copy back comes when its call returns.
      SUBROUTINE GIVE(A, N, S)
      INTEGER N
      REAL A(N, 3)
      CHARACTER*4 S(5)
      INTERFACE
        SUBROUTINE LOOK(V)
        REAL, INTENT(IN) :: V(2)
        END SUBROUTINE
      END INTERFACE
      CALL TAKE(A(:, 2))
      CALL TAKE(A(1, :))
      CALL LOOK(A(1, 1:2))
      CALL TAKE(A(:, 1) + 1.0)
      CALL WORDS(S(1:5:2))
      X = F(A(2, :)) + 1.0
      END
      SUBROUTINE SHAPED(W, C)
      REAL W(:, :)
      CHARACTER*8 C(:)
      CALL TAKE(W)
      CALL TAKE(W(:, 1))
      WRITE (C, '(A)') 'X', 'Y'
      END
