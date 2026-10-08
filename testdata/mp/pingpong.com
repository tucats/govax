$ ! PINGPONG.COM - Phase 46's mailbox ping-pong (testdata/mp/README.md).
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @PINGPONG/OUTPUT=PINGPONG.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST MBXPONG
$ MACRO/NOLIST MBXPINGPONG
$ LINK MBXPONG
$ LINK MBXPINGPONG
$ here = F$ENVIRONMENT("DEFAULT")
$ MBXPINGPONG :== $'here'MBXPINGPONG.EXE
$ MBXPINGPONG 'here'MBXPONG.EXE
$ SET NOVERIFY
$ EXIT
