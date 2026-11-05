C     FMTRACE.FOR - a FORMAT's parsed form under ASTs (Phase 89, subtask 8). FTNRTL
C     parses a FORMAT statement at its first use and keeps the result in the
C     FORMAT's cell. Here the routine using it is entered again by an AST at
C     each Kth instruction of its first statement in turn (the program traces
C     itself, as testdata/for83's FOR83T does), the cell emptied before each,
C     so some AST comes between FTNRTL's test of the cell and its store: both
C     statements parse it (a race), and one copy must be freed, not lost. Run by
C     TestRTL89FormatCache.
      PROGRAM FMTRACE
      EXTERNAL TRACER
      INTEGER K, NRACE, NRUNS, GETS0, GETS1, B0, B1, ISTAT, LIB$STAT_VM
      COMMON /ASTC/ KAT, NTRACE, NFIRED, NBAD
      CALL LIB$ESTABLISH(TRACER)
      CALL WARM(3)
      ISTAT = LIB$STAT_VM(3, B0)
      NRACE = 0
      NRUNS = 0
      DO 10 K = 1, 5000, 3
         CALL SHOW(0, .TRUE.)
         KAT = K
         NTRACE = 0
         NFIRED = 0
         ISTAT = LIB$STAT_VM(1, GETS0)
         CALL TRACEON
         CALL SHOW(1, .FALSE.)
         CALL TRACEOFF
         ISTAT = LIB$STAT_VM(1, GETS1)
         IF (NFIRED .EQ. 0) GO TO 20
         NRUNS = NRUNS + 1
         IF (GETS1 - GETS0 .EQ. 2) NRACE = NRACE + 1
   10 CONTINUE
   20 CALL SHOW(0, .TRUE.)
      ISTAT = LIB$STAT_VM(3, B1)
      PRINT *, 'RUNS', NRUNS .GT. 100
      PRINT *, 'RACES', NRACE .GT. 0
      PRINT *, 'LOST', B1 - B0
      PRINT *, 'BAD', NBAD
      END

C     The statement, or with CLEAR its FORMAT's cell emptied.
      RECURSIVE SUBROUTINE SHOW(N, CLEAR)
      INTEGER N, IFMT
      LOGICAL CLEAR
      CHARACTER*12 TEXT
      COMMON /ASTC/ KAT, NTRACE, NFIRED, NBAD
      IF (CLEAR) THEN
          ASSIGN 100 TO IFMT
          CALL CLEARFMT(%VAL(IFMT))
          RETURN
      END IF
      WRITE (TEXT, 100) N
  100 FORMAT ('<', I3, '>')
      IF (TEXT .NE. '<  ' // CHAR(48+N) // '>') NBAD = NBAD + 1
      END

C     N statements under way at once, so FTNRTL has contexts enough on its free
C     list for the loop: what it allocates there is parsed formats.
      RECURSIVE SUBROUTINE WARM(N)
      INTEGER N, DEEPER
      CHARACTER*4 TEXT
      IF (N .GT. 0) WRITE (TEXT, '(I4)') DEEPER(N)
      END

      INTEGER FUNCTION DEEPER(N)
      INTEGER N
      CALL WARM(N-1)
      DEEPER = N
      END

C     The AST: the same routine, the same FORMAT.
      SUBROUTINE ASTS
      COMMON /ASTC/ KAT, NTRACE, NFIRED, NBAD
      NFIRED = NFIRED + 1
      CALL SHOW(2, .FALSE.)
      END

C     At the KATth trace, an AST; after it, no more tracing.
      INTEGER FUNCTION TRACER(SIG, MECH)
      INCLUDE '($SSDEF)'
      EXTERNAL ASTS
      INTEGER SIG(*), MECH(*), PSL
      COMMON /ASTC/ KAT, NTRACE, NFIRED, NBAD
      TRACER = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_TBIT) RETURN
      TRACER = SS$_CONTINUE
      NTRACE = NTRACE + 1
      PSL = SIG(1) + 1
      IF (NTRACE .GT. KAT) THEN
          SIG(PSL) = IAND(SIG(PSL), NOT(16))
          RETURN
      END IF
      CALL KEEPT(%VAL(MECH(2)))
      IF (NTRACE .EQ. KAT) CALL SYS$DCLAST(ASTS,,)
      END
