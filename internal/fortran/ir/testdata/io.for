      PROGRAM IO
C     Every form of I/O statement this phase compiles (Phase 80,
C     subtask 3): formats of every kind, units, internal files,
C     namelists, specifiers, implied-DO lists, ENCODE and DECODE.
      INTEGER I, J, M, K(10), IOS
      INTEGER*2 IOS2
      REAL X, A(5)
      CHARACTER*20 LINE, PAGE(3)
      CHARACTER*8 FMT
      NAMELIST /G/ I, X, A
      ASSIGN 100 TO J
   10 FORMAT (I5, 2X, F8.2)
      PRINT 10, I, X
      PRINT J, I
      PRINT FMT, X
      PRINT '(1X,I3)', I
      PRINT K, I
      PRINT K(2), I
      TYPE 10, I, X
      ACCEPT 10, I, X
      READ 10, I, X
      READ *, I, (A(M), M = 1, 5)
      READ (5, 10, END=200, ERR=300, IOSTAT=IOS) I, X
      WRITE (6, 10, ERR=300, IOSTAT=IOS2) I, X
      WRITE (*, 10) ((K(M), M = 1, 10, 2), I, J = 1, 2), I
      WRITE (I + 1, *) X
      READ (LINE, 10) I, X
      WRITE (PAGE, *) I
      WRITE (LINE(2:5), '(I3)') I
      WRITE (6, G)
      READ (5, NML=G)
      PRINT G
      ENCODE (20, 10, LINE) I, X
      DECODE (12, 10, K) I
      WRITE (6, *) 'DONE', X + 1.0
  100 FORMAT ('X=', F8.2)
  200 CONTINUE
  300 END
