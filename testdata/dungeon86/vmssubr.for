C     VMSSUBR - what DUNGEON's kit calls VMSSUBR.FOR, which its copy here
C     lacks: ITIME, which VAX FORTRAN doesn't have. It gives the time of day
C     as three integers, from the system subroutine TIME's 'hh:mm:ss'.
C     Written for govax's milestone (docs/PHASE-86 - FORTRAN milestone.md).
C
      SUBROUTINE ITIME(H, M, S)
      INTEGER H, M, S
      CHARACTER*8 NOW
C
      CALL TIME(NOW)
      READ (NOW, 10) H, M, S
   10 FORMAT (I2, 1X, I2, 1X, I2)
      RETURN
      END
