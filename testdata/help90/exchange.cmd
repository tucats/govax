! EXCHANGE.CMD - builds round 3b's exchange volume with govax.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/help90x-exchange.dsk" /DEVICE=RD54 HELP90
MOUNT/WRITE DUA5 "testdata/disks/help90x-exchange.dsk"
COPY "testdata/help90/lbr90x.mar"/HOST DUA5:[000000]LBR90X.MAR
COPY "testdata/help90/help90x.com"/HOST DUA5:[000000]HELP90X.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
