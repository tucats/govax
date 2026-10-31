! EXCHANGE.CMD - builds the Phase 81 probe's exchange volume with govax.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for81-exchange.dsk" /DEVICE=RD54 FOR81
MOUNT/WRITE DUA2 "testdata/disks/for81-exchange.dsk"
COPY "testdata/for81/for81.mar"/HOST DUA2:[000000]FOR81.MAR
COPY "testdata/for81/for81r.mar"/HOST DUA2:[000000]FOR81R.MAR
COPY "testdata/for81/for81.opt"/HOST DUA2:[000000]FOR81.OPT
COPY "testdata/for81/for81.com"/HOST DUA2:[000000]FOR81.COM
COPY "testdata/for81/for81b.com"/HOST DUA2:[000000]FOR81B.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
