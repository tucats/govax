$ ! PG66.COM - Phase 66's probe of image activation
$ ! (testdata/pg66/README.md). Run it, from SYSTEM, with the exchange
$ ! volume's [000000] as the default directory:
$ !
$ !     @PG66/OUTPUT=PG66.LOG
$ !
$ ! PG66.LOG gets this procedure's log; PG66x.OUT each run's lines;
$ ! PG66_BUILD.LOG MACRO's and LINK's messages.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ RUN SYS$SYSTEM:SYSGEN
USE ACTIVE
SHOW PFCDEFAULT
SHOW MINWSCNT
SHOW WSMAX
SHOW GROWLIM
SHOW FREELIM
SHOW FREEGOAL
SHOW DZRO_MIN
EXIT
$ SHOW WORKING_SET
$ !
$ ! The program, assembled, and linked twice: PG66.EXE with every
$ ! section's default cluster factor, PG66C.EXE with BIG's 8.
$ !
$ DEFINE/USER_MODE SYS$OUTPUT PG66_BUILD.LOG
$ DEFINE/USER_MODE SYS$ERROR PG66_BUILD.LOG
$ MACRO/NOLIST PG66
$ DEFINE/USER_MODE SYS$OUTPUT PG66L1.LOG
$ DEFINE/USER_MODE SYS$ERROR PG66L1.LOG
$ LINK/NOTRACEBACK/MAP=PG66/FULL PG66
$ DEFINE/USER_MODE SYS$OUTPUT PG66L2.LOG
$ DEFINE/USER_MODE SYS$ERROR PG66L2.LOG
$ LINK/NOTRACEBACK/MAP=PG66C/FULL/EXECUTABLE=PG66C PG66,PG66C/OPTIONS
$ DIRECTORY/SIZE=ALL PG66*.EXE
$ !
$ ! Part A: the activation, from each image, PG66.EXE twice in a row.
$ !
$ CALL RUNA PG66 R1
$ CALL RUNA PG66 R2
$ CALL RUNA PG66C R3
$ !
$ ! Part W: BIG at a small working set.
$ !
$ DEFINE PG66_PART W
$ DEFINE/USER_MODE SYS$OUTPUT PG66W.OUT
$ RUN PG66
$ SHOW SYMBOL $STATUS
$ !
$ ! Part D: the image's file deleted while it runs, in a subprocess.
$ ! The image makes PG66D.RDY once it runs, waits ten seconds, reads
$ ! BIG, and makes PG66D.END.
$ !
$ COPY PG66.EXE PG66D.EXE
$ IF F$SEARCH("PG66D.RDY") .NES. "" THEN DELETE PG66D.RDY;*
$ IF F$SEARCH("PG66D.END") .NES. "" THEN DELETE PG66D.END;*
$ DEFINE PG66_PART D
$ SPAWN/NOWAIT/PROCESS=PG66DEL/OUTPUT=PG66D.OUT RUN PG66D
$ n = 0
$ WAITRDY:
$ IF F$SEARCH("PG66D.RDY") .NES. "" THEN GOTO READY
$ WAIT 00:00:01
$ n = n + 1
$ IF n .LT. 60 THEN GOTO WAITRDY
$ WRITE SYS$OUTPUT "D the image never made PG66D.RDY"
$ READY:
$ DELETE PG66D.EXE;*
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "D after DELETE: ", F$SEARCH("PG66D.EXE")
$ n = 0
$ WAITEND:
$ IF F$SEARCH("PG66D.END") .NES. "" THEN GOTO ENDED
$ WAIT 00:00:01
$ n = n + 1
$ IF n .LT. 60 THEN GOTO WAITEND
$ WRITE SYS$OUTPUT "D the image never made PG66D.END"
$ ENDED:
$ WAIT 00:00:02
$ WRITE SYS$OUTPUT "D after the image: ", F$SEARCH("PG66D.EXE")
$ DEASSIGN PG66_PART
$ TYPE PG66R1.OUT
$ TYPE PG66R2.OUT
$ TYPE PG66R3.OUT
$ TYPE PG66W.OUT
$ TYPE PG66D.OUT
$ SET NOVERIFY
$ EXIT
$ !
$ ! RUNA image id: part A from image (PG66 or PG66C), its lines to
$ ! PG66id.OUT, with the process's counts before RUN in the logical names
$ ! the image reads, and after it the counts' changes across the RUN.
$ !
$ RUNA: SUBROUTINE
$ DEFINE PG66_PART A
$ DEFINE PG66_IMAGE 'P1'.EXE
$ f0 = F$GETJPI("","PAGEFLTS")
$ c0 = F$GETJPI("","PAGFILCNT")
$ g0 = F$GETJPI("","PPGCNT")
$ DEFINE PG66_FLTS 'f0'
$ DEFINE PG66_CNT 'c0'
$ DEFINE PG66_PPG 'g0'
$ DEFINE/USER_MODE SYS$OUTPUT PG66'P2'.OUT
$ RUN 'P1'
$ s = $STATUS
$ f1 = F$GETJPI("","PAGEFLTS")
$ c1 = F$GETJPI("","PAGFILCNT")
$ g1 = F$GETJPI("","PPGCNT")
$ WRITE SYS$OUTPUT P2, " ", P1, " around RUN: status=", s, " flts=", f1 - f0, -
	" charged=", c0 - c1, " ppg=", g1 - g0
$ ENDSUBROUTINE
