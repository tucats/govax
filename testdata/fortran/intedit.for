      PROGRAM INTEDIT
C     Integer and logical editing, Phase 80's subtask 5: I, O, Z, and L
C     with widths, minimum digits, signs, and the default widths of
C     Table 8-4; a field too narrow is asterisks, and IOSTAT gets
C     OUTCONERR (63) while the statement goes on; I of a REAL edits its
C     bits and fails with FORVARMIS (61) at the statement's end, its
C     record written.
      INTEGER I, IOS
      INTEGER*2 K
      BYTE B
      LOGICAL L, M
      REAL X
   10 FORMAT (1X, I3.0, I5, I5.3, SP, I5, SS, I5)
   20 FORMAT (1X, O8, Z8, O4.4, Z6.4)
   30 FORMAT (1X, L1, L3, 2X, 2L2)
   40 FORMAT (1X, 'DEFAULT', I, I, O, Z, L)
      I = 284
      K = -284
      B = 127
      L = .TRUE.
      M = .FALSE.
      X = 1.0
      WRITE (6, 10) 0, I, I, I, K
      WRITE (6, 20) I, I, 7, 2708
      WRITE (6, 20) K, K, B, B
      WRITE (6, 30) L, M, L, M
      WRITE (6, 40) I, K, B, X, L
C     Z of a REAL is its bytes: 1.0 is 4080.
      WRITE (6, '(1X,Z8,O8)') X, X
      WRITE (6, '(1X,I2,I4)', IOSTAT=IOS) 3244, 42
      PRINT *, 'OUTCONERR', IOS
      WRITE (6, '(1X,I8)', IOSTAT=IOS) X
      PRINT *, 'FORVARMIS', IOS
      END
