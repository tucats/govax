! EXCHANGE.CMD - builds the exchange volume for Phase 69's probe
! (testdata/backup69/README.md). Run from the repository root:
!
!     govax console < testdata/backup69/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/backup69-exchange.dsk" /DEVICE=RD54 BACKUP69
MOUNT/WRITE DUA1 "testdata/disks/backup69-exchange.dsk"
COPY "testdata/backup69/backup69.com"/HOST DUA1:[000000]BACKUP69.COM
COPY "testdata/backup69/sscopy.mar"/HOST DUA1:[000000]SSCOPY.MAR
COPY "testdata/mar/pi.mar"/HOST DUA1:[000000]PI.MAR
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
