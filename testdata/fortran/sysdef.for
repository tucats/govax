C     The definition modules built into govax (Phase 83, subtask 4): constants
C     from several modules, services declared by $SYSSRVNAM, the condition
C     handler's structures, and IOSTAT's numbers. Each unit uses a few of the
C     names it includes, and the second includes $SYSSRVNAM and calls nothing.
      PROGRAM SYSDEF
      IMPLICIT NONE
      INCLUDE '($SSDEF)'
      INCLUDE '($FORIOSDEF)'
      INCLUDE '($SYSSRVNAM)'
      INCLUDE '($JPIDEF)'
      INCLUDE '($PSLDEF)'
      INTEGER*4 STATUS, TIME(2), IOS, VALUE
      CHARACTER*23 TEXT
      EXTERNAL HANDLER
      PRINT *, SS$_NORMAL, SS$_ABORT, JPI$_PID, PSL$C_USER
C
C     Services called as functions, by the names $SYSSRVNAM declares.
      STATUS = SYS$BINTIM('19-OCT-1987 13:30:15.25', TIME)
      PRINT *, STATUS .EQ. SS$_NORMAL
      STATUS = SYS$ASCTIM(%VAL(0), TEXT, TIME, %VAL(0))
      PRINT *, STATUS .EQ. SS$_NORMAL, ' ', TEXT
C
C     IOSTAT's numbers: none, no such file, and no such unit.
      READ (UNIT=TEXT(1:2), FMT='(I2)', IOSTAT=IOS) VALUE
      PRINT *, IOS, VALUE
      OPEN (UNIT=1, FILE='NOSUCH.DAT', STATUS='OLD', IOSTAT=IOS)
      PRINT *, IOS .EQ. FOR$IOS_FILNOTFOU, IOS
      OPEN (UNIT=999, IOSTAT=IOS)
      PRINT *, IOS .EQ. FOR$IOS_INVLOGUNI
C
C     A handler reading $CHFDEF's structures.
      CALL LIB$ESTABLISH(HANDLER)
      CALL LIB$SIGNAL(%VAL(SS$_ABORT), %VAL(1), %VAL(77))
      PRINT *, 'CONTINUED'
      CALL NOTHING
      END

C     The handler's arguments are the arrays, by reference, as records.
      INTEGER*4 FUNCTION HANDLER(SIGARGS, MECHARGS)
      INCLUDE '($SSDEF)'
      INCLUDE '($CHFDEF)'
      RECORD /CHFDEF1/ SIGARGS
      RECORD /CHFDEF2/ MECHARGS
      PRINT *, SIGARGS.CHF$L_SIG_ARGS,
     1         SIGARGS.CHF$L_SIG_NAME .EQ. SS$_ABORT
      PRINT *, SIGARGS.CHF$L_SIG_ARG1, MECHARGS.CHF$L_MCH_ARGS,
     1         MECHARGS.CHF$L_MCH_DEPTH
      PRINT *, CHF$L_MCH_DEPTH, CHF$S_CHFDEF2
      HANDLER = SS$_CONTINUE
      END

      SUBROUTINE NOTHING
      INCLUDE '($SYSSRVNAM)'
      PRINT *, 'NOTHING'
      END
