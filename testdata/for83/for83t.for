C     An AST at every instruction of a statement (Phase 83, subtask 7): the program
C     traces itself (for83u.mar keeps PSW<T> set), and the trace handler declares an
C     AST wherever none is waiting, so an AST's statements run between every two
C     instructions of the main line's that ASTs aren't held off for. Each statement's
C     IOSTAT must be its own, and a unit's records right. Run by TestFor83ASTEverywhere.
      PROGRAM FOR83T
      EXTERNAL TRACER
      INTEGER IOS1, IOS2, IOS3, IOS4, K
      CHARACTER*12 BUF, WRONG, BACK
      COMMON /ASTC/ NPEND, NAST, NBAD, NTRACE
      DATA WRONG /'12X4'/
      CALL LIB$ESTABLISH(TRACER)
      OPEN (12, FILE='FOR83T.DAT', STATUS='NEW')
      CALL TRACEON
      WRITE (BUF, '(I6)', IOSTAT=IOS1) 42
      READ (WRONG, '(I4)', IOSTAT=IOS2) K
      WRITE (12, '(A,I6)', IOSTAT=IOS3) 'R', 7
      WRITE (12, *, IOSTAT=IOS4) 'LIST', 8
      CALL TRACEOFF
      REWIND 12
      READ (12, '(A)') BACK
      PRINT *, 'FIRST ', BACK
      READ (12, '(A)') BACK
      PRINT *, 'SECOND ', BACK
      CLOSE (12, DISPOSE='DELETE')
      PRINT *, 'IOSTAT', IOS1, IOS2, IOS3, IOS4
      PRINT *, 'BUF ', BUF
      PRINT *, 'AST BAD', NBAD
      PRINT *, 'MANY', NTRACE .GT. 1000, NAST .GT. 1000
      END

      INTEGER FUNCTION TRACER(SIG, MECH)
      INCLUDE '($SSDEF)'
      EXTERNAL ASTS
      INTEGER SIG(*), MECH(*)
      COMMON /ASTC/ NPEND, NAST, NBAD, NTRACE
      TRACER = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_TBIT) RETURN
      NTRACE = NTRACE + 1
      CALL KEEPT(%VAL(MECH(2)))
      IF (NPEND .EQ. 0) THEN
          NPEND = 1
          CALL SYS$DCLAST(ASTS,,)
      END IF
      TRACER = SS$_CONTINUE
      END

      SUBROUTINE ASTS
      INTEGER IOS, K
      CHARACTER*12 TEXT, WRONG
      COMMON /ASTC/ NPEND, NAST, NBAD, NTRACE
      DATA WRONG /'9Q'/
      NAST = NAST + 1
      READ (WRONG, '(I2)', IOSTAT=IOS) K
      IF (IOS .EQ. 0) NBAD = NBAD + 1
      WRITE (TEXT, '(I6)', IOSTAT=IOS) NAST
      IF (IOS .NE. 0) NBAD = NBAD + 1
      NPEND = 0
      END
