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
COPY "testdata/dcl50/probe50v.com"/HOST DUA1:[000000]PROBE50V.COM
COPY "testdata/dcl50/probe50w.com"/HOST DUA1:[000000]PROBE50W.COM
COPY "testdata/dcl50/probe50i.com"/HOST DUA1:[000000]PROBE50I.COM
COPY "testdata/dcl50/probe50j.com"/HOST DUA1:[000000]PROBE50J.COM
COPY "testdata/dcl50/probe50k.com"/HOST DUA1:[000000]PROBE50K.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
