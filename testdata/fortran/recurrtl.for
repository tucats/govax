C RECURRTL.FOR - FTNRTL under recursion (Phase 89, subtask 8): statements
C nested 300 deep on internal files, and one under way on each of 50 units at
C once; RECIO_OPE at every level below a statement on the same unit; and a
C handler that unwinds dozens of activations out of their statements, after
C which the statement the establisher's caller was in goes on, the unit a gone
C statement had is free to a statement at its depth or above, and the gone
C statements' contexts are used again rather than piling up.
      PROGRAM RECURRTL
      INTEGER NEST, CHAIN, SAME, DIVE, LIB$STAT_VM
      INTEGER I, A, B, BAD, R, FIRST, LAST, ISTAT, NREC
      CHARACTER*8 LINE
      COMMON /LEVELS/ IOSV(5)
      COMMON /UNW/ KEST, KSIG, NOSIG, BACKW

C     300 internal WRITEs under way at once, under a WRITE to unit 6.
      WRITE (6, 100) 'NEST', NEST(300)
  100 FORMAT (1X, A, I6)

C     A WRITE under way on each of units 20 to 69, each from its own activation.
      DO 10 I = 20, 69
         OPEN (UNIT=I, STATUS='SCRATCH')
   10 CONTINUE
      R = CHAIN(1)
      BAD = 0
      DO 20 I = 1, 50
         REWIND (19+I)
         READ (19+I, '(2I6)') A, B
         IF (A .NE. I .OR. B .NE. 1275 - I*(I+1)/2) BAD = BAD + 1
         CLOSE (19+I)
   20 CONTINUE
      WRITE (6, 110) R, BAD
  110 FORMAT (' CHAIN', I6, ' BAD', I3)

C     Unit 7 from five activations: only the outermost's statement is allowed.
      OPEN (UNIT=7, STATUS='SCRATCH')
      R = SAME(5)
      WRITE (6, 120) IOSV
  120 FORMAT (' SAME IOSTAT', 5I4)
      REWIND (7)
      NREC = 0
   30 READ (7, '(A)', END=40) LINE
      NREC = NREC + 1
      WRITE (6, 130) LINE
  130 FORMAT (' SAME RECORD "', A, '"')
      GO TO 30
   40 CLOSE (7)
      WRITE (6, 140) NREC
  140 FORMAT (' SAME RECORDS', I3)

C     Activations 3 to 59 unwound out of their internal WRITEs, 21 times; the
C     heap mustn't grow after the first.
      KEST = 3
      KSIG = 60
      NOSIG = 0
      R = DIVE(1)
      ISTAT = LIB$STAT_VM(3, FIRST)
      DO 50 I = 1, 20
         R = DIVE(1)
   50 CONTINUE
      ISTAT = LIB$STAT_VM(3, LAST)
      WRITE (6, 150) R, LAST - FIRST
  150 FORMAT (' DIVE', I6, ' GROWTH', I6)

C     A WRITE to unit 8 at depth 30 unwound: then a statement on unit 8 from the
C     establisher's caller; from the same depth in the same frames again; and
C     from the main program.
      OPEN (UNIT=8, STATUS='SCRATCH')
      KSIG = 30
      BACKW = 1
      CALL DOWN(1)
      BACKW = 0
      CALL DOWN(1)
      NOSIG = 1
      CALL DOWN(1)
      NOSIG = 0
      CALL DOWN(1)
      WRITE (8, '(A)') 'MAIN'
      REWIND (8)
   60 READ (8, '(A)', END=70) LINE
      WRITE (6, 160) LINE
  160 FORMAT (' DOWN "', A, '"')
      GO TO 60
   70 CLOSE (8)
      END

C NEST(N) is N, by way of N internal WRITEs, each under way while the next
C activation's runs.
      RECURSIVE INTEGER FUNCTION NEST(N) RESULT(V)
      INTEGER N
      CHARACTER*8 TEXT
      IF (N .EQ. 0) THEN
         V = 0
      ELSE
         WRITE (TEXT, '(I8)') NEST(N-1) + 1
         READ (TEXT, '(I8)') V
      END IF
      END

C CHAIN(N) is N + ... + 50; each activation writes N and the rest's sum to unit
C 19+N while the deeper activations write theirs.
      RECURSIVE INTEGER FUNCTION CHAIN(N) RESULT(S)
      INTEGER N
      COMMON /LAST/ INNER
      IF (N .GT. 50) THEN
         S = 0
      ELSE
         WRITE (19+N, '(2I6)') N, CHAIN(N+1)
         S = N + INNER
      END IF
      INNER = S
      END

C SAME(N) writes unit 7 from inside a statement on unit 7, but for the
C outermost: RECIO_OPE (40), each statement's own.
      RECURSIVE INTEGER FUNCTION SAME(N)
      INTEGER N, IOS
      COMMON /LEVELS/ IOSV(5)
      IF (N .EQ. 0) THEN
         SAME = 0
         RETURN
      END IF
      WRITE (7, '(2I4)', IOSTAT=IOS) N, SAME(N-1)
      IOSV(N) = IOS
      SAME = N
      END

C DIVE(N) is 60-N, but activation KSIG signals, and the handler activation KEST
C established unwinds to its caller with -100 as DIVE(KEST): DIVE(1) is then
C -100+KEST-1.
      RECURSIVE INTEGER FUNCTION DIVE(N)
      EXTERNAL UNWIND
      INTEGER N
      CHARACTER*6 TEXT
      COMMON /UNW/ KEST, KSIG, NOSIG, BACKW
      IF (N .EQ. KEST) CALL LIB$ESTABLISH(UNWIND)
      IF (N .EQ. KSIG) CALL LIB$SIGNAL(%VAL(44))
      WRITE (TEXT, '(I6)') DIVE(N+1)
      READ (TEXT, '(I6)') DIVE
      DIVE = DIVE + 1
      END

C DOWN(N) goes down to KSIG, which writes unit 8 with an item that signals
C unless NOSIG is set; the handler of activation KEST unwinds to KEST-1, which
C writes unit 8 if BACKW is set.
      RECURSIVE SUBROUTINE DOWN(N)
      EXTERNAL UNWIND
      INTEGER N, BOOM
      COMMON /UNW/ KEST, KSIG, NOSIG, BACKW
      IF (N .EQ. KEST) CALL LIB$ESTABLISH(UNWIND)
      IF (N .EQ. KSIG) THEN
         WRITE (8, '(A,I4)') 'AT', BOOM(N)
      ELSE
         CALL DOWN(N+1)
         IF (N .EQ. KEST-1 .AND. BACKW .NE. 0)
     1      WRITE (8, '(A,I4)') 'BACK', N
      END IF
      END

      INTEGER FUNCTION BOOM(N)
      INTEGER N
      COMMON /UNW/ KEST, KSIG, NOSIG, BACKW
      IF (NOSIG .EQ. 0) CALL LIB$SIGNAL(%VAL(44))
      BOOM = N
      END

C Unwinds to the caller of the routine that established it, with R0 -100.
      INTEGER FUNCTION UNWIND(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      INTEGER SYS$UNWIND
      UNWIND = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_UNWIND) THEN
         MECH(4) = -100
         UNWIND = SYS$UNWIND(,)
      END IF
      END
