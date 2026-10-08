! EXCHANGE.CMD - builds probe 3's exchange volume with govax, for running
! it again by itself (README.md). Run from the repository root:
!
!     govax console < testdata/mp/probe3/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-probe3.dsk" /DEVICE=RD53 MPPROBE3
MOUNT/WRITE DUA1 "testdata/disks/mp-probe3.dsk"
COPY "testdata/mp/probe3/probe3.mar"/HOST DUA1:[000000]PROBE3.MAR
COPY "testdata/mp/probe3/probe3c.mar"/HOST DUA1:[000000]PROBE3C.MAR
COPY "testdata/mp/probe3/probe3.com"/HOST DUA1:[000000]PROBE3.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
