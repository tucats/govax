$ ! MACROS.COM - the system service macro probes, round 7
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Rounds 5 and 6 have run (MACROS5.COM and MACROS6.COM, logs in vax/);
$ ! round 7 writes MACROS7.LOG.
$ !
$ @MACROS7/OUTPUT=MACROS7.LOG
$ EXIT
