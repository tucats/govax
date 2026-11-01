! EXCHANGE.CMD - builds the Phase 83 probe's exchange volume with govax.
! Written by testdata/for83/gen.go. Run from the repository root:
!
!     govax console < testdata/for83/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for83-exchange.dsk" /DEVICE=RD54 FOR83
MOUNT/WRITE DUA2 "testdata/disks/for83-exchange.dsk"
COPY "testdata/for83/for83.mar"/HOST DUA2:[000000]FOR83.MAR
COPY "testdata/for83/for83b.mar"/HOST DUA2:[000000]FOR83B.MAR
COPY "testdata/for83/for83c.mar"/HOST DUA2:[000000]FOR83C.MAR
COPY "testdata/for83/lnk83.mar"/HOST DUA2:[000000]LNK83.MAR
COPY "testdata/for83/def_opc.mar"/HOST DUA2:[000000]DEF_OPC.MAR
COPY "testdata/for83/for83.opt"/HOST DUA2:[000000]FOR83.OPT
COPY "testdata/for83/for83.com"/HOST DUA2:[000000]FOR83.COM
COPY "testdata/for83/for83r.com"/HOST DUA2:[000000]FOR83R.COM
COPY "testdata/str83/str83.mar"/HOST DUA2:[000000]STR83.MAR
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
