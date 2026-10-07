! EXCHANGE.CMD - builds probe 1's exchange volume with govax
! (testdata/mp/probe2/README.md). Run from the repository root:
!
!     govax console < testdata/mp/probe2/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-probe2.dsk" /DEVICE=RD53 MPPROBE2
MOUNT/WRITE DUA1 "testdata/disks/mp-probe2.dsk"
COPY "testdata/mp/child.mar"/HOST DUA1:[000000]CHILD.MAR
COPY "testdata/mp/probe2/probe2.mar"/HOST DUA1:[000000]PROBE2.MAR
COPY "testdata/mp/probe2/probe2.com"/HOST DUA1:[000000]PROBE2.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
