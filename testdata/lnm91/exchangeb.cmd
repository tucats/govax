! EXCHANGEB.CMD - builds round 2's exchange volume with govax. Written by
! testdata/lnm91/gen.go. Run from the repository root:
!
!     govax console < testdata/lnm91/exchangeb.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lnm91b-exchange.dsk" /DEVICE=RD51 LNM91B
MOUNT/WRITE DUA5 "testdata/disks/lnm91b-exchange.dsk"
COPY "testdata/lnm91/lnm91.mar"/HOST DUA5:[000000]LNM91.MAR
COPY "testdata/lnm91/lnm91b.com"/HOST DUA5:[000000]LNM91B.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
