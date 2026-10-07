$ ! PROBE1.COM - Phase 45's probe 1 (testdata/mp/probe1/README.md).
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @PROBE1/OUTPUT=PROBE1.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST CHILD
$ MACRO/NOLIST INFO
$ MACRO/NOLIST PROBE1
$ LINK CHILD
$ LINK INFO
$ LINK PROBE1
$ ! The parent takes the two images, with their directory, as its command
$ ! line, separated by a blank.
$ here = F$ENVIRONMENT("DEFAULT")
$ PROBE1 :== $'here'PROBE1.EXE
$ SHOW PROCESS
$ PROBE1 'here'CHILD.EXE 'here'INFO.EXE
$ SHOW SYSTEM
$ SET NOVERIFY
$ EXIT
