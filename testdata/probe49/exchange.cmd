! EXCHANGE.CMD - builds the exchange volume for Phase 49's probe
! (testdata/probe49/README.md). Run from the repository root:
!
!     govax console < testdata/probe49/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/probe49.dsk" /DEVICE=RD53 PROBE49
MOUNT/WRITE DUA1 "testdata/disks/probe49.dsk"
COPY "testdata/probe49/probe6.mar"/HOST DUA1:[000000]PROBE6.MAR
COPY "testdata/probe49/probe6.com"/HOST DUA1:[000000]PROBE6.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
