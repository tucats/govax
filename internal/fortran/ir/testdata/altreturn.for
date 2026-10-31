C     Alternate returns (sections 5.9 and 6.1.1.5): the specifiers aren't in
C     the argument list, a RETURN i of any arithmetic value, an ENTRY's own
C     places, and a specifier in a logical IF's CALL.
      PROGRAM ALT
      REAL A, B, C
      INTEGER K
      CALL CHECK(A, B, *10, *20, C)
      K = 1
10    K = 2
20    CALL NONE(*30)
      IF (K .GT. 0) CALL NONE(*10)
      CALL ONLY(&10, &20, &30)
      CALL OTHER(A, *30)
30    END

      SUBROUTINE CHECK(X, Y, *, *, Z)
      REAL X, Y, Z
      IF (X - Y) 60, 70, 80
60    RETURN
70    RETURN 1
80    RETURN Z + 1.0
      END

      SUBROUTINE NONE(*)
      RETURN 1
      ENTRY ONLY(*, *, *)
      ENTRY OTHER(Q, *)
      Q = 1.0
      END

      SUBROUTINE PLAIN(I)
      RETURN I + 1
      END

C     A dynamic string is freed before the jump, so R0 waits in a temporary.
      SUBROUTINE JOINED(S, T)
      CHARACTER*(*) S, T
      CALL TAKES(S // T, *10)
      S = 'A'
10    RETURN
      END
