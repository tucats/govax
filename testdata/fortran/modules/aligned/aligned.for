C     /ALIGN across modules (Phase 82, subtask 9). This module's /BLK/ asks for an
C     octaword, for its COMPLEX; the other's, which stops before it, only for a
C     quadword. LINK keeps the largest alignment, whichever module comes first.
      PROGRAM ALIGNED
CDEC$ OPTIONS /ALIGN=COMMONS=NATURAL
      COMMON /BLK/ B, D, Z
      LOGICAL*1 B
      REAL*8 D
      COMPLEX Z
CDEC$ END OPTIONS
      D = 2.5
      CALL OTHER
      PRINT *, 'DMOD', MOD(%LOC(D), 8)
      PRINT *, 'ZMOD', MOD(%LOC(Z), 16)
      PRINT *, 'D', INT(D * 2.)
      END
