! EXCHANGED.CMD - builds round 4's exchange volume (DCL on indexed files) with
! govax. Written by hand. Run from the repository root:
!
!     govax console < testdata/idx88/exchanged.cmd
!
INITIALIZE/CONTAINER "testdata/disks/idx88d-exchange.dsk" /DEVICE=RD54 IDX88D
MOUNT/WRITE DUA5 "testdata/disks/idx88d-exchange.dsk"
COPY "testdata/idx88/idx88d.mar"/HOST DUA5:[000000]IDX88D.MAR
COPY "testdata/idx88/idx88d.com"/HOST DUA5:[000000]IDX88D.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
