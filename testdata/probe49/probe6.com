$ ! PROBE6.COM - Phase 49's probe (testdata/probe49/README.md). Run it,
$ ! from SYSTEM, with the exchange volume as the default directory:
$ !
$ !     @PROBE6
$ !
$ ! MACRO's and LINK's output goes to P6BUILD.LOG (to be audited before
$ ! anything reads it: a MACRO error can quote a macro's expansion), the
$ ! program's report to PROBE6.LOG.
$ SET NOON
$ DEFINE/USER_MODE SYS$OUTPUT P6BUILD.LOG
$ DEFINE/USER_MODE SYS$ERROR P6BUILD.LOG
$ MACRO/NOLIST PROBE6
$ DEFINE/USER_MODE SYS$OUTPUT P6LINK.LOG
$ DEFINE/USER_MODE SYS$ERROR P6LINK.LOG
$ LINK/NOMAP PROBE6
$ DEFINE/USER_MODE SYS$OUTPUT PROBE6.LOG
$ RUN PROBE6
$ TYPE PROBE6.LOG
$ EXIT
