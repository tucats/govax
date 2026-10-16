$ ! MACROS.COM - the system service macro probes, round 8
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Rounds 5 to 7 have run (MACROS5.COM to MACROS7.COM, logs in vax/);
$ ! round 8 writes MACROS8.LOG. (Phase 63's probe, testdata/pg63, runs
$ ! MACROS8.COM itself.)
$ !
$ @MACROS8/OUTPUT=MACROS8.LOG
$ EXIT
