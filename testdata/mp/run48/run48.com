$ ! RUN48.COM - every VAX run of Phase 48, in one go
$ ! (testdata/mp/run48/README.md). Run it, from the SYSTEM account, with
$ ! the exchange volume as the default directory:
$ !
$ !     @RUN48
$ !
$ ! Each part writes its own log.
$ !
$ SET NOON
$ @DEFS/OUTPUT=DEFS.LOG
$ @MILESTONE/OUTPUT=MILESTONE.LOG
$ @PROBE4/OUTPUT=PROBE4.LOG
$ DIRECTORY/SIZE *.LOG
$ EXIT
