$ ! PROBE3.COM - Phase 46's probe 3 (testdata/mp/probe3/README.md).
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @PROBE3/OUTPUT=PROBE3.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST PROBE3C
$ MACRO/NOLIST PROBE3
$ LINK PROBE3C
$ LINK PROBE3
$ here = F$ENVIRONMENT("DEFAULT")
$ PROBE3 :== $'here'PROBE3.EXE
$ PROBE3 'here'PROBE3C.EXE
$ SET NOVERIFY
$ EXIT
