! EXCHANGE.CMD - builds round 2's exchange volume with govax.
! Written by testdata/for88/gen.go. Run from the repository root:
!
!     govax console < testdata/for88/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for88-exchange.dsk" /DEVICE=RD54 FOR88
MOUNT/WRITE DUA5 "testdata/disks/for88-exchange.dsk"
COPY "testdata/for88/for88.mar"/HOST DUA5:[000000]FOR88.MAR
COPY "testdata/for88/for88.opt"/HOST DUA5:[000000]FOR88.OPT
COPY "testdata/for88/for88.com"/HOST DUA5:[000000]FOR88.COM
COPY "testdata/for88/for88b.com"/HOST DUA5:[000000]FOR88B.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
