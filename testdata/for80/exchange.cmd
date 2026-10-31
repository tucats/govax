! EXCHANGE.CMD - builds the Phase 80 probe's exchange volume with govax.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for80-exchange.dsk" /DEVICE=RD51 FOR80
MOUNT/WRITE DUA2 "testdata/disks/for80-exchange.dsk"
COPY "testdata/for80/for80.mar"/HOST DUA2:[000000]FOR80.MAR
COPY "testdata/for80/for80d.mar"/HOST DUA2:[000000]FOR80D.MAR
COPY "testdata/for80/for80e.mar"/HOST DUA2:[000000]FOR80E.MAR
COPY "testdata/for80/for80t.mar"/HOST DUA2:[000000]FOR80T.MAR
COPY "testdata/for80/for80.opt"/HOST DUA2:[000000]FOR80.OPT
COPY "testdata/for80/for80r.com"/HOST DUA2:[000000]FOR80R.COM
COPY "testdata/for80/for80.com"/HOST DUA2:[000000]FOR80.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
