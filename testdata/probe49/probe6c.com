$ ! PROBE6C.COM - Phase 49's probe, round 3 (testdata/probe49/README.md).
$ ! Run it, from SYSTEM, with the exchange volume as the default
$ ! directory:
$ !
$ !     @PROBE6C
$ !
$ ! MACRO's output goes to P6C_BUILD.LOG (to be audited before anything
$ ! reads it), the program's report to PROBE6C.LOG. LINK writes nothing
$ ! when it succeeds.
$ SET NOON
$ DEFINE/USER_MODE SYS$OUTPUT P6C_BUILD.LOG
$ DEFINE/USER_MODE SYS$ERROR P6C_BUILD.LOG
$ MACRO/NOLIST PROBE6C
$ LINK/NOMAP PROBE6C
$ DEFINE/USER_MODE SYS$OUTPUT PROBE6C.LOG
$ RUN PROBE6C
$ TYPE PROBE6C.LOG
$ EXIT
