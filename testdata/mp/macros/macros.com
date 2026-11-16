$ ! MACROS.COM - the system service macro probes, round 10
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Rounds 5 to 9 have run (MACROS5.COM to MACROS9.COM, logs in vax/);
$ ! round 10 writes MACROS10.LOG. (Phase 63's probe, testdata/pg63, ran
$ ! MACROS8.COM itself, and Phase 93's round 3, testdata/prot93's
$ ! SEC93.COM, ran MACROS9.COM.)
$ !
$ @MACROS10/OUTPUT=MACROS10.LOG
$ EXIT
