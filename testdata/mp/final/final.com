$ ! FINAL.COM - the multiprocessing program's last VAX run
$ ! (testdata/mp/final/README.md). Run it, from the SYSTEM account, with
$ ! the exchange volume as the default directory:
$ !
$ !     @FINAL
$ !
$ ! Each part writes its own log.
$ !
$ SET NOON
$ @MACROS7/OUTPUT=MACROS7.LOG
$ @PROBE5/OUTPUT=PROBE5.LOG
$ DIRECTORY/SIZE *.LOG
$ EXIT
