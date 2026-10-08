$ ! RUN46.COM - every VAX run waiting at the end of Phase 46, in one go
$ ! (testdata/mp/run46/README.md). Run it, from the SYSTEM account, with
$ ! the exchange volume as the default directory:
$ !
$ !     @RUN46
$ !
$ ! Each part writes its own log.
$ !
$ SET NOON
$ @MACROS
$ @DEFS/OUTPUT=DEFS.LOG
$ @PROBE3/OUTPUT=PROBE3.LOG
$ @PINGPONG/OUTPUT=PINGPONG.LOG
$ DIRECTORY/SIZE *.LOG
$ EXIT
