! EXCHANGE3.CMD - builds the exchange volume for Phase 49's probe, round
! 3 (testdata/probe49/README.md). Run from the repository root:
!
!     govax console < testdata/probe49/exchange3.cmd
!
INITIALIZE/CONTAINER "testdata/disks/probe49c.dsk" /DEVICE=RD53 PROBE49C
MOUNT/WRITE DUA1 "testdata/disks/probe49c.dsk"
COPY "testdata/probe49/probe6c.mar"/HOST DUA1:[000000]PROBE6C.MAR
COPY "testdata/probe49/probe6c.com"/HOST DUA1:[000000]PROBE6C.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
