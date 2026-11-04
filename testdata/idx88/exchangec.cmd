! EXCHANGEC.CMD - builds round 3's exchange volume (the utilities) with govax.
! Written by hand. Run from the repository root:
!
!     govax console < testdata/idx88/exchangec.cmd
!
! Round 1's files, the inputs, are restored from its save set into [IN].
INITIALIZE/CONTAINER "testdata/disks/idx88c-exchange.dsk" /DEVICE=RD54 IDX88C
MOUNT/WRITE DUA5 "testdata/disks/idx88c-exchange.dsk"
COPY "testdata/idx88/idx88c.com"/HOST DUA5:[000000]IDX88C.COM
COPY "testdata/idx88/idx88p.mar"/HOST DUA5:[000000]IDX88P.MAR
CREATE/DIRECTORY DUA5:[IN]
BACKUP "testdata/idx88/vax/idx88.bck"/SAVE_SET DUA5:[IN]
DIRECTORY DUA5:[000000]
DIRECTORY/SIZE DUA5:[IN]
DISMOUNT DUA5
