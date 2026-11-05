C     GONE89.FOR - FTNRTL's one known difference from FORRTL after an unwind
C     (docs/DEVIATIONS.md; Phase 83's for83GoneSameFrame), which recursion makes
C     the usual case: going down again from where a handler unwound to gives
C     each activation the frame it had, and a statement on the gone statement's
C     unit from deeper than it, with no statement at its depth or above between,
C     is RECIO_OPE (40) on FTNRTL, where FORRTL's (for83g.mar's cases 2 and 7)
C     goes on. One at the gone statement's depth is fine, and so is any after it.
C     Run by TestRTL89SameFrames.
      PROGRAM GONE89
      COMMON /G/ KSIG, NOSIG, IOS
      OPEN (UNIT=8, STATUS='SCRATCH')
      KSIG = 10
      NOSIG = 0
      CALL DOWN(1)
      KSIG = 11
      NOSIG = 1
      CALL DOWN(1)
      PRINT *, 'DEEPER', IOS
      KSIG = 10
      CALL DOWN(1)
      PRINT *, 'AFTER', IOS
      CLOSE (8)
      END

C     Down to KSIG, which writes unit 8 with an item that signals unless NOSIG is
C     set; activation 3's handler unwinds to activation 2.
      RECURSIVE SUBROUTINE DOWN(N)
      EXTERNAL UNWIND
      INTEGER N, BOOM
      COMMON /G/ KSIG, NOSIG, IOS
      IF (N .EQ. 3) CALL LIB$ESTABLISH(UNWIND)
      IF (N .EQ. KSIG) THEN
         WRITE (8, '(I4)', IOSTAT=IOS) BOOM(N)
      ELSE
         CALL DOWN(N+1)
      END IF
      END

      INTEGER FUNCTION BOOM(N)
      INTEGER N
      COMMON /G/ KSIG, NOSIG, IOS
      IF (NOSIG .EQ. 0) CALL LIB$SIGNAL(%VAL(44))
      BOOM = N
      END

      INTEGER FUNCTION UNWIND(SIG, MECH)
      INCLUDE '($SSDEF)'
      INTEGER SIG(*), MECH(*)
      INTEGER SYS$UNWIND
      UNWIND = SS$_RESIGNAL
      IF (SIG(2) .NE. SS$_UNWIND) UNWIND = SYS$UNWIND(,)
      END
