C     Part B's cases on FTNRTL (testdata/for83's README): I/O from a handler and from
C     an AST during a statement, the ways of ending an image with a unit open (CALL
C     EXIT's forms among them), and
C     exit handlers, numbered as for83b.mar's, so TestFor83PartBOnFTNRTL can hold
C     them to VMS's run. A number this program doesn't have just prints itself.
      PROGRAM FOR83X
      INTEGER N, XBLK(4), XST
      INTEGER*2 IW
      CHARACTER*8 PAR
      CHARACTER*6 INLIST
      EXTERNAL ASTH, XHAND, COUNTH
      COMMON /CASE/ NHOOK, NVIA, NXMODE
      CALL LIB$GET_FOREIGN(PAR)
      READ (PAR, '(BN,I8)') N
      CALL NUMBER(N)
      XBLK(1) = 0
      XBLK(2) = %LOC(XHAND)
      XBLK(3) = 1
      XBLK(4) = %LOC(XST)
      IF (N .GE. 36 .AND. N .LE. 45) THEN
          NHOOK = (N - 36) / 2 + 1
          NVIA = MOD(N - 36, 2) + 1
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          WRITE (12, '(A,A)') 'first', INLIST()
          CALL SAY(' =IO_END')
          CALL SAY(' =back')
          CLOSE (12)
      ELSE IF (N .EQ. 46) THEN
          NHOOK = 1
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          CALL SYS$DCLAST(ASTH,,)
          CALL SAY(' =after')
          CALL SAY(' =back')
          CLOSE (12)
      ELSE IF (N .GE. 47 .AND. N .LE. 54) THEN
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          WRITE (12, '(A)') 'record'
          IF (N .EQ. 47) CALL EXIT
          IF (N .EQ. 48) CALL EXIT(3)
          IF (N .EQ. 49) CALL EXIT(2)
          IF (N .EQ. 50) CALL EXIT(0)
          IF (N .EQ. 51) THEN
C             The compiler refuses a null status: the routine's own answer.
              CALL LIB$ESTABLISH(COUNTH)
              CALL FTN$EXIT(%VAL(0))
          END IF
          IW = 3
          IF (N .EQ. 53) IW = -1
          IF (N .EQ. 52 .OR. N .EQ. 53) CALL EXIT(IW)
          CALL FTN$EXIT_W
      ELSE IF (N .GE. 55 .AND. N .LE. 57) THEN
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          WRITE (12, '(A)') 'record'
          IF (N .EQ. 55) STOP
          IF (N .EQ. 57) CALL SYS$EXIT(%VAL(3))
          CALL SAY(' =returned')
          CALL SAY(' =back')
      ELSE IF (N .GE. 58 .AND. N .LE. 63) THEN
          NXMODE = MOD(N - 58, 3) + 1
          IF (N .LE. 60) CALL SYS$DCLEXH(XBLK)
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          WRITE (12, '(A)') 'from the program'
          CALL SAY(' =back')
          IF (N .GE. 61) CALL SYS$DCLEXH(XBLK)
      ELSE IF (N .EQ. 64) THEN
          NXMODE = 2
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          CALL SYS$DCLEXH(XBLK)
          OPEN (14, FILE='NOSUCH.DAT', STATUS='OLD')
      END IF
      END

C     An item of the statement in progress: a condition or an AST, with the case's
C     I/O in it, then the rest of the statement.
      CHARACTER*6 FUNCTION INLIST()
      EXTERNAL HAND, ASTH
      COMMON /CASE/ NHOOK, NVIA, NXMODE
      CALL LIB$ESTABLISH(HAND)
      IF (NVIA .EQ. 1) THEN
          CALL LIB$SIGNAL(%VAL('08018003'X))
      ELSE
          CALL SYS$DCLAST(ASTH,,)
      END IF
      CALL SAY(' =after')
      INLIST = 'second'
      END

      SUBROUTINE ASTH
      CALL HOOK
      END

      INTEGER FUNCTION HAND(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      CALL SHOW(' =c n=!UL cond=!XL', SIG(1), SIG(2))
      CALL SHOW(' =m n=!UL depth=!UL', MECH(1), MECH(3))
      CALL HOOK
      HAND = SS$_CONTINUE
      END

C     Continues a condition that comes back, until the program stops itself.
      INTEGER FUNCTION COUNTH(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      DATA NCALLS /0/
      CALL SHOW(' =c n=!UL cond=!XL', SIG(1), SIG(2))
      CALL SHOW(' =m n=!UL depth=!UL', MECH(1), MECH(3))
      NCALLS = NCALLS + 1
      IF (NCALLS .GE. 12) THEN
          CALL SAY(' =too many conditions; the program stops itself')
          CALL SYS$EXIT(%VAL(3))
      END IF
      COUNTH = SS$_CONTINUE
      END

C     The I/O: on the same unit, another, an internal file, unit 6, or a CLOSE.
      SUBROUTINE HOOK
      EXTERNAL INNER
      CHARACTER*10 IFILE
      COMMON /CASE/ NHOOK, NVIA, NXMODE
      CALL LIB$ESTABLISH(INNER)
      GO TO (10, 20, 30, 40, 50), NHOOK
10    WRITE (12, '(A)') 'inner'
      GO TO 60
20    WRITE (13, '(A)') 'inner'
      CLOSE (13)
      GO TO 60
30    WRITE (IFILE, '(A)') 'inner'
      GO TO 60
40    WRITE (6, '(A)') 'inner on 6'
      GO TO 60
50    CLOSE (12)
60    CALL SAY(' =hook''s statement')
      END

      INTEGER FUNCTION INNER(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      CALL SHOW(' =i n=!UL cond=!XL', SIG(1), SIG(2))
      INNER = SS$_CONTINUE
      END

      SUBROUTINE XHAND(ST)
      INTEGER ST
      EXTERNAL INNER
      COMMON /CASE/ NHOOK, NVIA, NXMODE
      CALL LIB$ESTABLISH(INNER)
      CALL SHOW(' =exit handler status=!XL', ST, 0)
      IF (NXMODE .EQ. 1) WRITE (12, '(A)') 'from the exit handler'
      IF (NXMODE .EQ. 2) WRITE (15, '(A)') 'from the exit handler'
      END

      SUBROUTINE NUMBER(N)
      CALL SHOW('#!UL', N, 0)
      END

C     SHOW and SAY use $FAO and LIB$PUT_OUTPUT, so that they are none of FTNRTL's I/O.
      SUBROUTINE SHOW(CTL, A, B)
      CHARACTER*(*) CTL
      INTEGER A, B
      CHARACTER*80 LINE
      INTEGER*2 L
      CALL SYS$FAO(CTL, L, LINE, %VAL(A), %VAL(B))
      CALL LIB$PUT_OUTPUT(LINE(1:L))
      END

      SUBROUTINE SAY(TEXT)
      CHARACTER*(*) TEXT
      CALL LIB$PUT_OUTPUT(TEXT)
      END
