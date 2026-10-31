! EXCHANGE.CMD - builds the Phase 80 TPARSE probe's exchange volume with govax.
! Written by testdata/tpa80/gen.go. Run from the repository root:
!
!     govax console < testdata/tpa80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/tpa80-exchange.dsk" /DEVICE=RD51 TPA80
MOUNT/WRITE DUA2 "testdata/disks/tpa80-exchange.dsk"
COPY "testdata/tpa80/tpa80.mar"/HOST DUA2:[000000]TPA80.MAR
COPY "testdata/tpa80/tpa80b.mar"/HOST DUA2:[000000]TPA80B.MAR
COPY "testdata/tpa80/tpa80c.mar"/HOST DUA2:[000000]TPA80C.MAR
COPY "testdata/tpa80/tpa80.com"/HOST DUA2:[000000]TPA80.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
