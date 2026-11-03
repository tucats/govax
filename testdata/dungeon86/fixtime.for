C     FIXTIME - IDATE and ITIME at a fixed moment, for a repeatable game.
C     DUNGEON seeds its random numbers from both. Compiled with GAME.F, as
C     FORT GAME.F+FIXTIME.FOR, its IDATE is a unit of that compilation's,
C     which wins over the system subroutine of the name. subr.f's ITIME is
C     then this one too, as a global routine.
C     Written for govax's milestone (docs/PHASE-86 - FORTRAN milestone.md).
C
      SUBROUTINE IDATE(I, J, K)
      INTEGER I, J, K
      I = 3
      J = 14
      K = 86
      END
C
      SUBROUTINE ITIME(H, M, S)
      INTEGER H, M, S
      H = 12
      M = 30
      S = 15
      END
