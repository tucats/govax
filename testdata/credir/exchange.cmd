! EXCHANGE.CMD - builds the Phase 34 oracle's exchange volume with govax.
! Run from the repository root:
!
!     govax console < testdata/credir/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/credir-exchange.dsk" /DEVICE=RD51 CREDIR
MOUNT/WRITE DUA1 "testdata/disks/credir-exchange.dsk"
COPY "testdata/credir/credir.com"/HOST DUA1:[000000]CREDIR.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
