$ ! PROBE4.COM - probe 4 (testdata/mp/probe4/README.md): LIB$SPAWN and
$ ! DCL's SPAWN. Run it, from SYSTEM, with the exchange volume as the
$ ! default directory:
$ !
$ !     @PROBE4/OUTPUT=PROBE4.LOG
$ !
$ SET NOON
$ MACRO P4CHILD, P4NOCLI, PROBE4
$ LINK P4CHILD
$ LINK P4NOCLI
$ LINK PROBE4
$ P4SYM == "a global symbol"
$ RUN PROBE4
$ ! The spawned processes' own output, step by step.
$ TYPE P4_*.LOG
$ ! DCL's SPAWN: its messages, with and without waiting.
$ SPAWN/OUTPUT=P4_W.LOG RUN P4CHILD
$ SHOW SYMBOL $STATUS
$ SPAWN/NOWAIT/PROCESS=P4_NOWAIT/OUTPUT=P4_N.LOG RUN P4CHILD
$ WAIT 00:00:05
$ TYPE P4_W.LOG, P4_N.LOG
$ EXIT
