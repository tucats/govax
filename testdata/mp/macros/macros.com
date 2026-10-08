$ ! MACROS.COM - the system service macro probes, rounds 5 and 6
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Each round writes its own log, MACROS5.LOG and MACROS6.LOG.
$ !
$ @MACROS5/OUTPUT=MACROS5.LOG
$ @MACROS6/OUTPUT=MACROS6.LOG
$ EXIT
