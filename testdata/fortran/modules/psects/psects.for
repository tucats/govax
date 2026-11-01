CDEC$ IDENT 'PSECTS'
C     Common blocks with CDEC$ PSECT's attributes, seen from two files. One
C     image has one cluster here, so even the LCL block is one section.
      PROGRAM PSECTS
      COMMON /PAGED/ X /LOCAL/ L /FIXED/ K
      INTEGER L, K
CDEC$ PSECT /PAGED/ ALIGN=9
CDEC$ PSECT /LOCAL/ LCL, NOSHR
CDEC$ PSECT /FIXED/ NOWRT
      X = 2.5
      L = 11
      CALL OTHER
      PRINT *, 'MAIN', MOD(%LOC(X), 512), X, L, K
      END
