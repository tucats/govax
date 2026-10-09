! EXCHANGE.CMD - builds the exchange volume for Phase 50's probe
! (testdata/dcl50/README.md). Run from the repository root:
!
!     govax console < testdata/dcl50/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/dcl50.dsk" /DEVICE=RD53 DCL50
MOUNT/WRITE DUA1 "testdata/disks/dcl50.dsk"
COPY "testdata/dcl50/probe50.com"/HOST DUA1:[000000]PROBE50.COM
COPY "testdata/dcl50/probe50s.com"/HOST DUA1:[000000]PROBE50S.COM
COPY "testdata/dcl50/probe50r.com"/HOST DUA1:[000000]PROBE50R.COM
COPY "testdata/dcl50/probe50e.com"/HOST DUA1:[000000]PROBE50E.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
