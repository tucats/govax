$ ! PROBE2.COM - Phase 45's probe 2 (testdata/mp/probe2/README.md).
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @PROBE2/OUTPUT=PROBE2.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST CHILD
$ MACRO/NOLIST PROBE2
$ LINK CHILD
$ LINK PROBE2
$ here = F$ENVIRONMENT("DEFAULT")
$ PROBE2 :== $'here'PROBE2.EXE
$ PROBE2 'here'CHILD.EXE
$ ! The quotas a process gets by default and at least (SYSGEN's PQL_ parameters).
$ MCR SYSGEN
SHOW/PQL
EXIT
$ ! The null device.
$ SHOW DEVICE/FULL NLA0:
$ WRITE SYS$OUTPUT "DEVCLASS ", F$GETDVI("NLA0:","DEVCLASS"), " DEVTYPE ", F$GETDVI("NL:","DEVTYPE")
$ ! Some defaults of the running system.
$ MCR SYSGEN
SHOW MAXPROCESSCNT
SHOW PQL_DPRCLM
SHOW DEFPRI
SHOW QUANTUM
EXIT
$ SET NOVERIFY
$ EXIT
