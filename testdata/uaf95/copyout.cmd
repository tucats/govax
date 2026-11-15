! COPYOUT.CMD - copies Phase 95's probe results off uaf95-exchange.dsk
! into testdata/uaf95/vax (README.md). Written by testdata/uaf95/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/copyout.cmd
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/uaf95-exchange.dsk"
DIRECTORY DUA5:[UAF95LOG],[UAF95SCR]
COPY DUA5:[UAF95LOG]UAF95.LOG "testdata/uaf95/vax/uaf95.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95DROP.LOG "testdata/uaf95/vax/uaf95drop.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95J_UAF95.LOG "testdata/uaf95/vax/uaf95j_uaf95.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95J_UAF95L.LOG "testdata/uaf95/vax/uaf95j_uaf95l.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95J_UAF95D.LOG "testdata/uaf95/vax/uaf95j_uaf95d.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95I_UAF95_BATCH.LOG "testdata/uaf95/vax/uaf95i_uaf95_batch.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95I_UAF95L_BATCH.LOG "testdata/uaf95/vax/uaf95i_uaf95l_batch.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95I_UAF95_INTERACTIVE.LOG "testdata/uaf95/vax/uaf95i_uaf95_interactive.log"/HOST/QUIET
COPY DUA5:[UAF95LOG]UAF95I_UAF95L_INTERACTIVE.LOG "testdata/uaf95/vax/uaf95i_uaf95l_interactive.log"/HOST/QUIET
COPY DUA5:[UAF95SCR]SYSUAF.LIS;1 "testdata/uaf95/vax/sysuaf_1.lis"/HOST/QUIET
COPY DUA5:[UAF95SCR]SYSUAF.LIS;2 "testdata/uaf95/vax/sysuaf_2.lis"/HOST/QUIET
COPY DUA5:[UAF95SCR]SYSUAF.LIS;3 "testdata/uaf95/vax/sysuaf_3.lis"/HOST/QUIET
COPY DUA5:[UAF95SCR]RIGHTSLIST.LIS;1 "testdata/uaf95/vax/rightslist_1.lis"/HOST/QUIET
COPY DUA5:[UAF95SCR]RIGHTSLIST.LIS;2 "testdata/uaf95/vax/rightslist_2.lis"/HOST/QUIET
COPY DUA5:[UAF95SCR]RIGHTSLIST.LIS;3 "testdata/uaf95/vax/rightslist_3.lis"/HOST/QUIET
DISMOUNT DUA5
