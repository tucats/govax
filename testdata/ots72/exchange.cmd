! EXCHANGE.CMD - builds the Phase 72 OTS$ probe's exchange volume with
! govax. Written by testdata/ots72/gen.go. Run from the repository root:
!
!     govax console < testdata/ots72/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/ots72-exchange.dsk" /DEVICE=RD51 OTS72
MOUNT/WRITE DUA2 "testdata/disks/ots72-exchange.dsk"
COPY "testdata/ots72/ots72.mar"/HOST DUA2:[000000]OTS72.MAR
COPY "testdata/ots72/ots72t.mar"/HOST DUA2:[000000]OTS72T.MAR
COPY "testdata/ots72/ots72.com"/HOST DUA2:[000000]OTS72.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
