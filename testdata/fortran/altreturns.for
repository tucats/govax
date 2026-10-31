C     Alternate returns (sections 5.9 and 6.1.1.5): the manual's CHECK, whose
C     arguments after the *s still reach it; RETURN i past the range and below
C     it, a REAL i truncated, an i that's an expression; * and &; a specifier
C     in a logical IF's CALL and a CALL ending a loop; out of a loop; a return
C     passed on through two subroutines; an ENTRY with its *s in other places;
C     several specifiers to one label; a dynamic string's freeing, which mustn't
C     lose the number; and a subroutine with RETURN i and no *s.
      PROGRAM ALTRET
      REAL A, B, C
      INTEGER I, N
      CHARACTER*2 S, T
      A = 1.0
      B = 2.0
      C = -1.0
      CALL CHECK(A, B, *10, *20, C)
      PRINT *, 'VALUE LESS THAN ZERO'
      GO TO 30
10    PRINT *, 'VALUE EQUALS ZERO'
      GO TO 30
20    PRINT *, 'VALUE MORE THAN ZERO'
30    C = 0.0
      CALL CHECK(A, B, *40, *50, C)
      PRINT *, 'VALUE LESS THAN ZERO'
      GO TO 60
40    PRINT *, 'VALUE EQUALS ZERO'
      GO TO 60
50    PRINT *, 'VALUE MORE THAN ZERO'
60    C = 5.0
      CALL CHECK(A, B, &70, &80, C)
      PRINT *, 'VALUE LESS THAN ZERO'
      GO TO 90
70    PRINT *, 'VALUE EQUALS ZERO'
      GO TO 90
80    PRINT *, 'VALUE MORE THAN ZERO'
C     The number chooses: 0, -1, and 3 are past the two specifiers.
90    DO 120 I = -1, 3
         CALL PICK(I, *100, *110)
         PRINT *, I, ' goes on'
         GO TO 120
100      PRINT *, I, ' first'
         GO TO 120
110      PRINT *, I, ' second'
120   CONTINUE
C     A REAL number is truncated, an expression computed.
      CALL ROUGH(2.9, *130, *140)
      PRINT *, 'ROUGH goes on'
      GO TO 150
130   PRINT *, 'ROUGH first'
      GO TO 150
140   PRINT *, 'ROUGH second'
150   N = 0
C     In a logical IF: taken, then not, then the statement not run.
      IF (N .EQ. 0) CALL PICK(1, *160, *170)
      PRINT *, 'IF goes on'
      GO TO 180
160   PRINT *, 'IF first'
180   IF (N .EQ. 0) CALL PICK(0, *170, *170)
      PRINT *, 'IF goes on'
      IF (N .NE. 0) CALL PICK(1, *170, *170)
      PRINT *, 'IF not run'
      GO TO 190
170   PRINT *, 'IF wrong'
C     A CALL ending a loop, going on with the loop until one returns: the last
C     pass's specifier leaves it.
190   DO 200 I = 1, 5
         N = N + 1
200   CALL STOPAT(I, 3, *210)
      PRINT *, 'Loop ended'
210   PRINT *, 'Left the loop at', I, N
C     Passed on: OUTER's second return when INNER takes its only one, its
C     first when INNER goes on.
      CALL OUTER(1, *230, *220)
      PRINT *, 'OUTER goes on'
220   CALL OUTER(0, *240, *230)
      PRINT *, 'OUTER goes on'
      GO TO 240
230   PRINT *, 'OUTER wrong'
240   CONTINUE
C     Entry points: the header's list has none, the others' are in other places.
      CALL GIVE(3.5, 2.5)
      PRINT *, 'GIVE goes on'
      CALL FIRST(*250, 4.5, *260, 1.5)
      PRINT *, 'FIRST goes on'
      GO TO 260
250   PRINT *, 'FIRST first'
260   CALL LAST(6.5, 7.5, *270)
      PRINT *, 'LAST goes on'
      GO TO 280
270   PRINT *, 'LAST first'
C     Several places, one label.
280   CALL PICK(2, *290, *290)
      PRINT *, 'Wrong: PICK goes on'
290   PRINT *, 'To one label'
C     A concatenation of passed lengths, freed before the jump.
      S = 'AB'
      T = 'CD'
      CALL JOIN(S, T)
C     RETURN i with no specifiers in the routine's list or the CALL.
      CALL BARE(2)
      PRINT *, 'BARE goes on'
      END

      SUBROUTINE CHECK(X, Y, *, *, Q)
      Z = (X - Y) * Q
      Z = -Z
      IF (Z) 60, 70, 80
60    RETURN
70    RETURN 1
80    RETURN 2
      END

      SUBROUTINE PICK(I, *, *)
      RETURN I
      END

      SUBROUTINE ROUGH(X, *, *)
      RETURN X - 1.0
      END

      SUBROUTINE STOPAT(I, N, *)
      IF (I .EQ. N) RETURN 1
      END

      SUBROUTINE OUTER(I, *, *)
      CALL INNER(I, *10)
      PRINT *, 'INNER went on'
      RETURN 1
10    PRINT *, 'INNER returned'
      RETURN 2
      END

      SUBROUTINE INNER(I, *)
      RETURN I
      END

      SUBROUTINE GIVE(X, Y)
      PRINT *, 'GIVE', X, Y
      GO TO 10
      ENTRY FIRST(*, Y, *, X)
      PRINT *, 'FIRST', X, Y
      RETURN 1
      ENTRY LAST(X, Y, *)
      PRINT *, 'LAST', X, Y
      RETURN 1
10    END

      SUBROUTINE JOIN(S, T)
      CHARACTER*(*) S, T
      CALL SEE(S // T, *10, *20)
      PRINT *, 'JOIN goes on'
      RETURN
10    PRINT *, 'JOIN first'
      RETURN
20    PRINT *, 'JOIN second'
      END

      SUBROUTINE SEE(TEXT, *, *)
      CHARACTER*(*) TEXT
      PRINT *, TEXT, LEN(TEXT)
      RETURN LEN(TEXT) - 2
      END

      SUBROUTINE BARE(I)
      RETURN I
      END
