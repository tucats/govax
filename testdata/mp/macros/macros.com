$ ! MACROS.COM - the system service macro probes, round 9
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Rounds 5 to 8 have run (MACROS5.COM to MACROS8.COM, logs in vax/);
$ ! round 9 writes MACROS9.LOG. (Phase 63's probe, testdata/pg63, ran
$ ! MACROS8.COM itself, and Phase 93's round 3, testdata/prot93's
$ ! SEC93.COM, runs MACROS9.COM.)
$ !
$ @MACROS9/OUTPUT=MACROS9.LOG
$ EXIT
