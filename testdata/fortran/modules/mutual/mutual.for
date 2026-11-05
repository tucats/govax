C     Recursion across files (Phase 89, subtask 11): Hofstadter's female and
C     male sequences, each function in its own file calling the other and
C     itself, and the towers of Hanoi, HANOI and STEP calling each other from
C     two files. FEM is RECURSIVE with RESULT; MALE's file says OPTIONS
C     /RECURSIVE, and MALE's name with an argument list is a call. However
C     the modules are linked, and whether or not compiled /RECURSIVE, the
C     program prints the same.
      PROGRAM MUTUAL
      INTEGER FEM, MALE, I, NMOVES
      CHARACTER*80 MOVES
      COMMON /TOWER/ NMOVES
      COMMON /TOWERC/ MOVES
      WRITE (6, 100) (FEM(I), I = 0, 15)
  100 FORMAT (' F:', 16I3)
      WRITE (6, 110) (MALE(I), I = 0, 15)
  110 FORMAT (' M:', 16I3)
      NMOVES = 0
      MOVES = ' '
      CALL HANOI(4, 'A', 'C', 'B')
      WRITE (6, 120) NMOVES, MOVES(1:3*NMOVES-1)
  120 FORMAT (' HANOI', I3, ': ', A)
      END
