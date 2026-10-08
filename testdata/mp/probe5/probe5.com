$ ! PROBE5.COM - the multiprocessing program's last probe
$ ! (testdata/mp/probe5/README.md). Run it, from SYSTEM, with the exchange
$ ! volume as the default directory:
$ !
$ !     @PROBE5/OUTPUT=PROBE5.LOG
$ !
$ SET NOON
$ MACRO P5SLEEP, P5LOCK, P5INFO, PROBE5
$ LINK P5SLEEP
$ LINK P5LOCK
$ LINK P5INFO
$ LINK PROBE5
$ RUN PROBE5
$ TYPE P5_INFO.LOG
$ !
$ ! DCL's side. Verify is on, so each command is in the log before what
$ ! it does.
$ SET VERIFY
$ ! 9: verb abbreviations: R for RUN, MC for MCR, EXI and LO in a
$ ! subprocess, and the status each leaves.
$ R P5INFO
$ SHOW SYMBOL $STATUS
$ MC P5INFO
$ SHOW SYMBOL $STATUS
$ SPAWN/OUTPUT=P5_EXI.LOG EXI 5
$ SHOW SYMBOL $STATUS
$ SPAWN/OUTPUT=P5_LO.LOG LO
$ SHOW SYMBOL $STATUS
$ ! 10: SHOW LOGICAL of a name with no translation: its status.
$ SHOW LOGICAL P5_NO_SUCH_NAME
$ SHOW SYMBOL $STATUS
$ ! 11: RUN of a file that isn't an image: the messages and the status.
$ RUN PROBE5.MAR
$ SHOW SYMBOL $STATUS
$ ! 12: the access modes of the process logical names a subprocess gets.
$ DEFINE/SUPERVISOR P5_SUPER X
$ DEFINE/EXECUTIVE P5_EXEC X
$ SPAWN/OUTPUT=P5_LNM.LOG SHOW LOGICAL/FULL/PROCESS P5_*
$ ! 13: the console terminal's and the null device's characteristics,
$ ! SHOW DEVICE/FULL of each and of a mailbox, and SHOW DEVICE of a
$ ! prefix and of no device.
$ WRITE SYS$OUTPUT F$FAO("OPA0: DEVCHAR !XL DEVTYPE !UL DEVBUFSIZ !UL", -
	F$GETDVI("OPA0:","DEVCHAR"), F$GETDVI("OPA0:","DEVTYPE"), -
	F$GETDVI("OPA0:","DEVBUFSIZ"))
$ WRITE SYS$OUTPUT F$FAO("NLA0: DEVCHAR !XL DEVTYPE !UL DEVBUFSIZ !UL", -
	F$GETDVI("NLA0:","DEVCHAR"), F$GETDVI("NLA0:","DEVTYPE"), -
	F$GETDVI("NLA0:","DEVBUFSIZ"))
$ SHOW DEVICE/FULL OPA0:
$ SHOW DEVICE/FULL NLA0:
$ SHOW DEVICE/FULL MBA1:
$ SHOW DEVICE MB
$ SHOW DEVICE XYZ
$ SHOW SYMBOL $STATUS
$ SET NOVERIFY
$ ! The subprocesses' logs.
$ TYPE P5_EXI.LOG, P5_LO.LOG, P5_LNM.LOG
$ EXIT
