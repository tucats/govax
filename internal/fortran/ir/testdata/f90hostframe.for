      OPTIONS /F90
C     A host in its stack frame (Phase 103, subtask 4): a RECURSIVE host's
C     variables and the cell holding its dummy's address are in its frame, so
C     each call of an internal procedure passes the host's FP as a hidden last
C     argument; the internal procedure, reaching that frame (HostFrame), passes
C     the FP it was given on to its sibling.
      RECURSIVE SUBROUTINE WALK(N)
      INTEGER N, MINE
      MINE = N
      IF (N .GT. 1) CALL WALK(N - 1)
      CALL ADD
      CONTAINS
      SUBROUTINE ADD
      MINE = MINE + N
      CALL SHOW(MINE)
      END SUBROUTINE ADD
      SUBROUTINE SHOW(V)
      INTEGER V
      V = V + 1
      END SUBROUTINE SHOW
      END SUBROUTINE WALK
