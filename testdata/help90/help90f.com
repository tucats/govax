$ ! HELP90F.COM - part F of the Phase 90 probe (round 1b): the debugger's
$ ! HELP, with DBG$HELP naming the probe's library, then with nothing
$ ! defined. Written by testdata/help90/gen.go. The second session shows
$ ! VMS's own help, so this log stays out of the repository; DBGH.DLG, the
$ ! first session's, holds only the probe's. Run it after HELP90.COM:
$ !
$ !     @HELP90F/OUTPUT=HELP90F.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ MACRO/DEBUG DBGT
$ LINK/DEBUG DBGT
$ DEFINE/USER_MODE DBG$INPUT HELP90D.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ DEFINE/USER_MODE DBG$HELP H90DIR:H90D
$ RUN DBGT
$ TYPE DBGH.DLG
$ DEFINE/USER_MODE DBG$INPUT HELP90V.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ RUN DBGT
$ EXIT
