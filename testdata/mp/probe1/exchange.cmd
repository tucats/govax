! EXCHANGE.CMD - builds probe 1's exchange volume with govax
! (testdata/mp/probe1/README.md). Run from the repository root:
!
!     govax console < testdata/mp/probe1/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-probe1.dsk" /DEVICE=RD53 MPPROBE1
MOUNT/WRITE DUA1 "testdata/disks/mp-probe1.dsk"
COPY "testdata/mp/child.mar"/HOST DUA1:[000000]CHILD.MAR
COPY "testdata/mp/probe1/info.mar"/HOST DUA1:[000000]INFO.MAR
COPY "testdata/mp/probe1/probe1.mar"/HOST DUA1:[000000]PROBE1.MAR
COPY "testdata/mp/probe1/probe1.com"/HOST DUA1:[000000]PROBE1.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
