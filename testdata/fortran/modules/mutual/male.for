C     M(0) = 0, M(N) = N - F(M(N-1)), recursive by OPTIONS.
      OPTIONS /RECURSIVE
      INTEGER FUNCTION MALE(N)
      INTEGER N, FEM
      IF (N .EQ. 0) THEN
         MALE = 0
      ELSE
         MALE = N - FEM(MALE(N - 1))
      END IF
      END

C     The discs above N out of the way, disc N's move, and the discs back on it.
C     Each move is its two pegs and a comma.
      RECURSIVE SUBROUTINE STEP(N, FROM, TO, VIA)
      INTEGER N, NMOVES
      CHARACTER*1 FROM, TO, VIA
      CHARACTER*80 MOVES
      COMMON /TOWER/ NMOVES
      COMMON /TOWERC/ MOVES
      CALL HANOI(N - 1, FROM, VIA, TO)
      MOVES(3*NMOVES+1:3*NMOVES+3) = FROM // TO // ','
      NMOVES = NMOVES + 1
      CALL HANOI(N - 1, VIA, TO, FROM)
      END
