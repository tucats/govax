$ ! PG67.COM - Phase 67's probe of the address space
$ ! (testdata/pg67/README.md). Run it, from SYSTEM, with the exchange
$ ! volume's [000000] as the default directory:
$ !
$ !     @PG67/OUTPUT=PG67.LOG
$ !
$ ! PG67.LOG gets this procedure's log; PG67x.OUT each run's lines;
$ ! PG67_BUILD.LOG MACRO's and LINK's messages.
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ RUN SYS$SYSTEM:SYSGEN
USE ACTIVE
SHOW VIRTUALPAGECNT
SHOW CTLPAGES
SHOW PQL_DPGFLQUOTA
SHOW PQL_MPGFLQUOTA
EXIT
$ SHOW PROCESS/QUOTA
$ !
$ ! The program, assembled, and linked twice: PG67.EXE with LINK's
$ ! default user stack, PG67S.EXE with STACK=50.
$ !
$ DEFINE/USER_MODE SYS$OUTPUT PG67_BUILD.LOG
$ DEFINE/USER_MODE SYS$ERROR PG67_BUILD.LOG
$ MACRO/NOLIST PG67
$ DEFINE/USER_MODE SYS$OUTPUT PG67L1.LOG
$ DEFINE/USER_MODE SYS$ERROR PG67L1.LOG
$ LINK/NOTRACEBACK/MAP=PG67/FULL PG67
$ DEFINE/USER_MODE SYS$OUTPUT PG67L2.LOG
$ DEFINE/USER_MODE SYS$ERROR PG67L2.LOG
$ LINK/NOTRACEBACK/MAP=PG67S/FULL/EXECUTABLE=PG67S PG67,PG67S/OPTIONS
$ DIRECTORY/SIZE=ALL PG67*.EXE
$ GOSUB VA
$ !
$ ! Part A: created space. Each run's items before and after it show
$ ! what image rundown gives back.
$ !
$ DEFINE PG67_PART A
$ DEFINE/USER_MODE SYS$OUTPUT PG67A.OUT
$ RUN PG67
$ SHOW SYMBOL $STATUS
$ GOSUB VA
$ !
$ ! Part S: the user stack grows by itself.
$ !
$ DEFINE PG67_PART S
$ DEFINE/USER_MODE SYS$OUTPUT PG67S.OUT
$ RUN PG67
$ SHOW SYMBOL $STATUS
$ GOSUB VA
$ !
$ ! Part T: the first instruction, with the default stack and STACK=50.
$ !
$ DEFINE PG67_PART T
$ DEFINE/USER_MODE SYS$OUTPUT PG67T.OUT
$ RUN PG67
$ SHOW SYMBOL $STATUS
$ DEFINE/USER_MODE SYS$OUTPUT PG67T50.OUT
$ RUN PG67S
$ SHOW SYMBOL $STATUS
$ GOSUB VA
$ !
$ ! Part C: part Q in the detached process PG67DET, with a small paging
$ ! file quota. The detached process sees only the system's logical
$ ! names, so the files' names are given with the device's full name.
$ !
$ DEV = F$GETDVI(F$PARSE("PG67.EXE",,,"DEVICE"),"FULLDEVNAM")
$ DIR = F$PARSE("PG67.EXE",,,"DIRECTORY")
$ DEFINE PG67_IMAGE 'DEV''DIR'PG67.EXE
$ DEFINE PG67_OUTQ 'DEV''DIR'PG67Q.OUT
$ SHOW LOGICAL PG67_*
$ DEFINE PG67_PART C
$ DEFINE/USER_MODE SYS$OUTPUT PG67C.OUT
$ RUN PG67
$ SHOW SYMBOL $STATUS
$ GOSUB VA
$ DEASSIGN PG67_PART
$ DEASSIGN PG67_IMAGE
$ DEASSIGN PG67_OUTQ
$ TYPE PG67A.OUT
$ TYPE PG67S.OUT
$ TYPE PG67T.OUT
$ TYPE PG67T50.OUT
$ TYPE PG67C.OUT
$ TYPE PG67Q.OUT
$ SET NOVERIFY
$ EXIT
$ !
$ ! VA: the process's address space items, from DCL.
$ !
$ VA:
$ WRITE SYS$OUTPUT "va frep0va=", F$GETJPI("","FREP0VA"), " frep1va=", F$GETJPI("","FREP1VA"), -
	" freptecnt=", F$GETJPI("","FREPTECNT"), " virtpeak=", F$GETJPI("","VIRTPEAK"), -
	" pagfilcnt=", F$GETJPI("","PAGFILCNT")
$ RETURN
