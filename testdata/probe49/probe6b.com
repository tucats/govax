$ ! PROBE6B.COM - Phase 49's probe, round 2 (testdata/probe49/README.md).
$ ! Run it, from SYSTEM, with the exchange volume as the default
$ ! directory:
$ !
$ !     @PROBE6B
$ !
$ ! MACRO's output goes to P6B_BUILD.LOG (to be audited before anything
$ ! reads it), the program's report to PROBE6B.LOG. LINK writes nothing
$ ! when it succeeds.
$ SET NOON
$ DEFINE/USER_MODE SYS$OUTPUT P6B_BUILD.LOG
$ DEFINE/USER_MODE SYS$ERROR P6B_BUILD.LOG
$ MACRO/NOLIST PROBE6B
$ LINK/NOMAP PROBE6B
$ DEFINE/USER_MODE SYS$OUTPUT PROBE6B.LOG
$ RUN PROBE6B
$ TYPE PROBE6B.LOG
$ EXIT
