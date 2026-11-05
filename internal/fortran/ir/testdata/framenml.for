C Phase 89, subtask 5: a NAMELIST group with a variable in the frame is a block
C the statement fills in, in a cell (in the frame, in a recursive unit), each
C entity's descriptor written whole; a group of static variables is still a
C datum, as a SAVE'd one shows.
      RECURSIVE SUBROUTINE R(N)
      INTEGER N, K, A(3), L
      SAVE L
      NAMELIST /G/ K, A
      NAMELIST /H/ L
      READ (1, G)
      IF (N .GT. 1) CALL R(N - 1)
      WRITE (6, G)
      WRITE (6, H)
      END
      SUBROUTINE U
      REAL Y
      AUTOMATIC Y
      NAMELIST /V/ Y
      Y = 1.0
      WRITE (6, V)
      END
