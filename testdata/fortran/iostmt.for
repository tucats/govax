      PROGRAM IOSTMT
C     The I/O statements as Phase 80's subtask 3 leaves them: every one
C     goes through FTNRTL's begin and end calls; list-directed output to
C     the default units and units 5 and 6 works; IOSTAT=, ERR=, and END=
C     take the errors FTNRTL finds so far; and a statement may run inside
C     another's item, on another unit (not on the same one).
      INTEGER IOS, INNER
      CHARACTER*20 LINE
   10 FORMAT (1X, I5)
      PRINT *, 'PRINT', 1
      WRITE (*,*) 'WRITE (*,*)', 2
      WRITE (6,*) 'WRITE (6,*)', 3
      TYPE *, 'TYPE', 4
      WRITE (5,*) 'WRITE (5,*)', 5
C     Integer editing (I5) is subtask 5's: NOTIMP, FOR number 2.
      WRITE (6, 10, IOSTAT=IOS) 6
      PRINT *, 'FORMATTED', IOS
      WRITE (6, 10, ERR=20) 7
      PRINT *, 'NOT HERE'
   20 PRINT *, 'ERR= TAKEN'
      READ (5, *, IOSTAT=IOS, END=30) I
      PRINT *, 'READ', IOS
   30 CONTINUE
C     Units: 200 is none (INVLOGUNI, 32); 7 is Phase 81's (NOTIMP); -1
C     is PRINT's; 6 can't be read (ERRDURREA, 39).
      WRITE (200, *, IOSTAT=IOS) 8
      PRINT *, 'UNIT 200', IOS
      WRITE (7, *, IOSTAT=IOS) 9
      PRINT *, 'UNIT 7', IOS
      WRITE (-1, *, IOSTAT=IOS) 'UNIT -1', 10
      PRINT *, 'UNIT -1', IOS
      READ (6, *, IOSTAT=IOS) I
      PRINT *, 'READ (6)', IOS
C     An internal file: NOTIMP so far.
      WRITE (LINE, *, IOSTAT=IOS) 11
      PRINT *, 'INTERNAL', IOS
C     Another unit's statement inside an item, which writes its record
C     first; then the same unit's, which is RECIO_OPE, 40.
      PRINT *, 'OUTER', INNER(6), 'END'
      WRITE (6, *) 'OUTER', INNER(6)
      END

      INTEGER FUNCTION INNER(IU)
      INTEGER IOS
      WRITE (IU, *, IOSTAT=IOS) 'INNER'
      INNER = IOS
      END
