$ ! HELP90X.COM - round 3b of the Phase 90 probe: binary keys, the index
$ ! blocks preallocated, and opening a library another opening has open
$ ! (LBR90X.MAR). Written by testdata/help90/gen.go. Run it with the
$ ! exchange volume's [000000] as the default directory:
$ !
$ !     @HELP90X/OUTPUT=HELP90X.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ MACRO LBR90X
$ LINK LBR90X
$ ! Each case is a run of its own, ended by the program's timer if it
$ ! runs past three minutes.
$ LBR90X := $'L'LBR90X
$ LBR90X 1
$ LBR90X 2
$ LBR90X 3
$ LBR90X 4
$ LBR90X 5
$ LBR90X 6
$ LBR90X 7
$ LBR90X 8
$ LBR90X 9
$ LBR90X 10
$ LBR90X 11
$ ! X12 holds WX open to update for 90 seconds in a subprocess, while
$ ! X13 opens it to update, then (a second holder) X14 to read.
$ SPAWN/NOWAIT/OUTPUT=HOLD1.LOG LBR90X 12
$ WAIT 00:00:15
$ LBR90X 13
$ WAIT 00:01:45
$ TYPE HOLD1.LOG
$ SPAWN/NOWAIT/OUTPUT=HOLD2.LOG LBR90X 12
$ WAIT 00:00:15
$ LBR90X 14
$ WAIT 00:01:45
$ TYPE HOLD2.LOG
$ LBR90X 15
$ LBR90X 16
$ LBR90X 17
$ SHOW SYMBOL $STATUS
$ !
$ ! The libraries as LIBRARY sees them.
$ !
$ DIRECTORY/SIZE=ALL X*.*LB;*,WX.TLB;*
$ LIBRARY/LIST/FULL X1.TLB
$ LIBRARY/LIST/FULL X2.ULB
$ LIBRARY/LIST/FULL X3.ULB
$ LIBRARY/LIST/FULL X4.ULB
$ LIBRARY/LIST/FULL X5.TLB
$ LIBRARY/LIST/FULL X6.TLB
$ LIBRARY/LIST/FULL X7.TLB
$ LIBRARY/LIST/FULL X8.TLB
$ LIBRARY/LIST/FULL X9.TLB
$ LIBRARY/LIST/FULL X10.TLB
$ LIBRARY/LIST/FULL WX.TLB
$ !
$ ! The libraries, for the host.
$ !
$ BACKUP X*.*LB;*,WX.TLB;* H90X.BCK/SAVE_SET/LOG
