C     The third round of testdata/for83 (for83g.mar) on FTNRTL: what's left of a
C     statement a handler unwound from, numbered as for83g.mar's cases, so that
C     TestFor83GoneOnFTNRTL can hold them to FORRTL's run.
      PROGRAM FOR83Y
      INTEGER N, XBLK(4), XST
      CHARACTER*8 PAR
      CHARACTER*6 ITEM
      EXTERNAL XHAND, QUIET
      COMMON /CASE/ NCASE
      CALL LIB$GET_FOREIGN(PAR)
      READ (PAR, '(BN,I8)') N
      NCASE = N
      CALL SHOW('#!UL', N, 0)
      IF (N .EQ. 9) THEN
          XBLK(2) = %LOC(XHAND)
          XBLK(3) = 1
          XBLK(4) = %LOC(XST)
          WRITE (6, '(A)') ' on 6'
          CALL SYS$DCLEXH(XBLK)
          OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
          WRITE (12, '(A)') 'from the program'
          GO TO 900
      END IF
      OPEN (12, FILE='FOR83W.DAT', STATUS='NEW')
      IF (N .EQ. 1) THEN
          CALL BEGUN(12)
          CALL SAY(' =back')
          CALL DEEP(12)
          CLOSE (12)
      ELSE IF (N .EQ. 2 .OR. N .EQ. 7) THEN
          CALL AGAIN(0)
          IF (N .EQ. 2) CALL SAY(' =first call')
          CALL AGAIN(1)
          CALL SAY(' =second call')
          CLOSE (12)
      ELSE IF (N .EQ. 3) THEN
          CALL BEGUN(12)
          CALL SAY(' =back')
          WRITE (12, '(A)') 'above'
          CALL SAY(' =above')
          CLOSE (12)
      ELSE IF (N .EQ. 4 .OR. N .EQ. 5 .OR. N .EQ. 8) THEN
          WRITE (12, '(A,A)') 'outer', ITEM(N)
          CALL SAY(' =outer''s end')
          CLOSE (12)
          CLOSE (13)
      ELSE IF (N .EQ. 6) THEN
          CALL LIB$ESTABLISH(QUIET)
          WRITE (12, '(A,A)') 'outer', ITEM(N)
      END IF
900   END

C     An item of the statement on unit 12: what the case does in the middle of it.
      CHARACTER*6 FUNCTION ITEM(N)
      EXTERNAL GONEA
      IF (N .EQ. 4 .OR. N .EQ. 5) THEN
          CALL BEGUN(13)
          CALL SAY(' =back')
      END IF
      IF (N .EQ. 5) CALL DEEP(13)
      IF (N .EQ. 6) CALL DEEP(12)
      IF (N .EQ. 8) THEN
          CALL SYS$DCLAST(GONEA,,)
          CALL SAY(' =after')
      END IF
      ITEM = 'second'
      END

      SUBROUTINE GONEA
      CALL BEGUN(13)
      CALL SAY(' =back')
      END

C     A statement on unit U, an item in, that a signal leaves.
      SUBROUTINE BEGUN(U)
      INTEGER U
      EXTERNAL OUT
      CHARACTER*4 GONE
      CALL LIB$ESTABLISH(OUT)
      WRITE (U, '(A,A)') 'gone', GONE()
      CALL SAY('NOT REACHED')
      END

      CHARACTER*4 FUNCTION GONE()
      CALL LIB$SIGNAL(%VAL('08018003'X))
      GONE = ' '
      END

C     Called twice from one place, so that the second call's frame is the first's.
      SUBROUTINE AGAIN(TURN)
      INTEGER TURN
      EXTERNAL OUT
      CHARACTER*4 GONE
      CALL LIB$ESTABLISH(OUT)
      IF (TURN .EQ. 0) WRITE (12, '(A,A)') 'gone', GONE()
      IF (TURN .NE. 0) CALL DEEP(12)
      END

      SUBROUTINE DEEP(U)
      INTEGER U
      CALL DEEP2(U)
      END

      SUBROUTINE DEEP2(U)
      INTEGER U
      CALL DEEP3(U)
      END

      SUBROUTINE DEEP3(U)
      INTEGER U
      WRITE (U, '(A)') 'deep'
      CALL SAY(' =deep statement')
      END

      INTEGER FUNCTION OUT(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      INTEGER SYS$UNWIND
      OUT = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_UNWIND) THEN
          CALL SHOW(' =c n=!UL cond=!XL', SIG(1), SIG(2))
          CALL SHOW(' =m n=!UL depth=!UL', MECH(1), 0)
          OUT = SYS$UNWIND(,)
      END IF
      END

      INTEGER FUNCTION QUIET(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      CALL SHOW(' =i n=!UL cond=!XL', SIG(1), SIG(2))
      QUIET = SS$_CONTINUE
      END

      SUBROUTINE XHAND(ST)
      INTEGER ST
      CALL SHOW(' =exit handler status=!XL', ST, 0)
      WRITE (12, '(A)') 'from the exit handler'
      END

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
