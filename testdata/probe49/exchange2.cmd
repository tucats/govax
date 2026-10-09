! EXCHANGE2.CMD - builds the exchange volume for Phase 49's probe, round
! 2 (testdata/probe49/README.md). Run from the repository root:
!
!     govax console < testdata/probe49/exchange2.cmd
!
INITIALIZE/CONTAINER "testdata/disks/probe49b.dsk" /DEVICE=RD53 PROBE49B
MOUNT/WRITE DUA1 "testdata/disks/probe49b.dsk"
COPY "testdata/probe49/probe6b.mar"/HOST DUA1:[000000]PROBE6B.MAR
COPY "testdata/probe49/probe6b.com"/HOST DUA1:[000000]PROBE6B.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
