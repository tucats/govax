C     Condition handlers written in FORTRAN (Phase 83, subtask 6): which of the
C     library's errors a handler can go on from, the arrays it's given, $UNWIND,
C     and LIB$'s tools, as testdata/for83's runs on VMS have them. The last thing
C     it does is continue a LIB$STOP, which ends the image.
      PROGRAM HANDLE
      INCLUDE '($SSDEF)'
      EXTERNAL GOON, SETR0, LIB$FIXUP_FLT, NEWOP, SHOW
      INTEGER GOON, SETR0, NEWOP, SHOW, LIB$SIGNAL, LIB$MATCH_COND
      INTEGER LIB$INT_OVER
      INTEGER I, J, K, OLD, TOSTAT, UNWOUND, UNDER
      CHARACTER*8 TEXT
      REAL X, Y
      INTEGER XBITS
      EQUIVALENCE (X, XBITS)
      COMMON /COUNTS/ NCOND
      DATA TEXT /'12X4  77'/
C
C     A field that can't be read: the handler is called at the item, it goes on,
C     and the statement reads its next item.
      CALL LIB$ESTABLISH(GOON)
      I = -1
      J = -1
      READ (TEXT, '(I4,1X,I4)') I, J
      PRINT *, 'I', I, J, NCOND
      I = -1
      J = -1
      READ (TEXT, '(O4,1X,I4)') I, J
      PRINT *, 'O', I, J, NCOND
C
C     An item of the wrong type: its field first, and the type at the end.
      X = 3.0
      READ (TEXT, '(I4)') X
      PRINT *, 'WRONG TYPE', NCOND
C
C     A field too small is signaled after the record, whose field is stars.
      NCOND = 0
      WRITE (TEXT, '(I2)') 12345
      PRINT *, TEXT, NCOND
C
C     LIB$MATCH_COND: the position in its list.
      PRINT *, LIB$MATCH_COND(SS$_ACCVIO, 1, SS$_ROPRAND, SS$_ACCVIO)
C
C     A handler sets the R0 LIB$SIGNAL returns.
      CALL LIB$ESTABLISH(SETR0)
      PRINT *, LIB$SIGNAL(%VAL(SS$_ENDOFFILE))
C
C     An error FORTRAN stops for, unwound from by a routine's own handler, which
C     sets what the routine returns; the statement is gone, and FORTRAN goes on.
      PRINT *, UNWOUND()
      OPEN (UNIT=12, STATUS='SCRATCH')
      WRITE (12, *) 'FINE'
      CLOSE (12)
C
C     LIB$SIG_TO_RET as a routine's handler.
      PRINT *, TOSTAT()
C
C     LIB$FIXUP_FLT, as the handler: the reserved operand becomes zero.
      CALL LIB$ESTABLISH(LIB$FIXUP_FLT)
      XBITS = '8000'X
      Y = X + 1.0
      PRINT *, Y, XBITS
C
C     And called by a handler, with a value for it.
      CALL LIB$ESTABLISH(NEWOP)
      XBITS = '8000'X
      Y = X + 1.0
      PRINT *, Y, X
C
C     Integer overflow traps only once it's asked for, and goes on.
      CALL LIB$ESTABLISH(SHOW)
      I = 2147483647
      K = I + 1
      OLD = LIB$INT_OVER(1)
      K = I + 1
      PRINT *, OLD, LIB$INT_OVER(0), K
C
C     And floating underflow, a fault this handler can't go on from.
      PRINT *, UNDER()
C
C     Continuing a LIB$STOP ends the image, with the condition's message.
      CALL LIB$ESTABLISH(GOON)
      CALL LIB$STOP(%VAL(SS$_ENDOFFILE))
      PRINT *, 'NOT HERE'
      END

C     Says which condition it was, and continues it.
      INTEGER FUNCTION GOON(SIG, MECH)
      INCLUDE '($SSDEF)'
      INCLUDE '($FORDEF)'
      INCLUDE '($CHFDEF)'
      RECORD /CHFDEF1/ SIG
      RECORD /CHFDEF2/ MECH
      INTEGER LIB$MATCH_COND
      COMMON /COUNTS/ NCOND
      NCOND = NCOND + 1
      I = LIB$MATCH_COND(SIG.CHF$L_SIG_NAME, FOR$_INPCONERR,
     1  FOR$_FORVARMIS, FOR$_OUTCONERR, SS$_ENDOFFILE)
      PRINT *, 'GOON', I, MECH.CHF$L_MCH_ARGS
      GOON = SS$_CONTINUE
      END

      INTEGER FUNCTION SETR0(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      MECH(4) = 77
      SETR0 = SS$_CONTINUE
      END

C     A handler that unwinds to the caller of the routine that established it,
C     which then sees the mechanism array's R0 as the routine's value.
      INTEGER FUNCTION UNWOUND()
      EXTERNAL OUT
      INTEGER OUT
      CALL LIB$ESTABLISH(OUT)
      OPEN (UNIT=13, FILE='NOSUCH.DAT', STATUS='OLD')
      UNWOUND = 1
      END

      INTEGER FUNCTION OUT(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      INTEGER SYS$UNWIND
      OUT = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_UNWIND) THEN
          MECH(4) = 55
          OUT = SYS$UNWIND(,)
      END IF
      END

      INTEGER FUNCTION TOSTAT()
      INCLUDE '($SSDEF)'
      EXTERNAL LIB$SIG_TO_RET
      CALL LIB$ESTABLISH(LIB$SIG_TO_RET)
      CALL LIB$SIGNAL(%VAL(SS$_ENDOFFILE))
      TOSTAT = 0
      END

C     Gives a reserved operand the value 2.5.
      INTEGER FUNCTION NEWOP(SIG, MECH)
      INTEGER SIG(*), MECH(*)
      INTEGER LIB$FIXUP_FLT
      NEWOP = LIB$FIXUP_FLT(SIG, MECH, 2.5)
      END

      INTEGER FUNCTION SHOW(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      PRINT *, 'INTOVF', SIG(2) .EQ. SS$_INTOVF
      SHOW = SS$_CONTINUE
      END

      INTEGER FUNCTION UNDER()
      EXTERNAL OUT
      INTEGER LIB$FLT_UNDER, OLD
      REAL T, Z
      DATA T /1.0E-30/
      CALL LIB$ESTABLISH(OUT)
      Z = T * T
      OLD = LIB$FLT_UNDER(1)
      Z = T * T
      UNDER = 1
      END
