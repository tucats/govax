! EXCHANGE.CMD - builds Phase 91's probe exchange volume with govax.
! Written by testdata/lnm91/gen.go. Run from the repository root:
!
!     govax console < testdata/lnm91/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lnm91-exchange.dsk" /DEVICE=RD51 LNM91
MOUNT/WRITE DUA5 "testdata/disks/lnm91-exchange.dsk"
COPY "testdata/lnm91/lnm91.mar"/HOST DUA5:[000000]LNM91.MAR
COPY "testdata/lnm91/lnm91.com"/HOST DUA5:[000000]LNM91.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
