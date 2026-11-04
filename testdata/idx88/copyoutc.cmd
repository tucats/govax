! COPYOUTC.CMD - copies round 3's results off its exchange volume. Written by
! hand. Run from the repository root:
!
!     mkdir -p testdata/disks/idx88c-help
!     govax console < testdata/idx88/copyoutc.cmd
!
! IDX88CM.LOG, MACRO's own output, stays on the volume until the author has
! read it (the clean-room notes). The MSG*.TXT files are HELP/MESSAGE's text,
! DIGITAL's, so they go to testdata/disks, which is never committed.
MOUNT DUA5 "testdata/disks/idx88c-exchange.dsk"
COPY DUA5:[000000]IDX88C.LOG "testdata/idx88/vax/idx88c.log"/HOST/QUIET
COPY DUA5:[000000]IDX88CBCK.TXT "testdata/idx88/vax/idx88cbck.txt"/HOST/QUIET
COPY DUA5:[000000]IDX88C.BCK "testdata/idx88/vax/idx88c.bck"/HOST/BINARY/QUIET
COPY DUA5:[000000]MSGCONV.TXT "testdata/disks/idx88c-help/msgconv.txt"/HOST/QUIET
COPY DUA5:[000000]MSGCONV2.TXT "testdata/disks/idx88c-help/msgconv2.txt"/HOST/QUIET
COPY DUA5:[000000]MSGFDL.TXT "testdata/disks/idx88c-help/msgfdl.txt"/HOST/QUIET
COPY DUA5:[000000]MSGCRFDL.TXT "testdata/disks/idx88c-help/msgcrfdl.txt"/HOST/QUIET
COPY DUA5:[000000]MSGANL.TXT "testdata/disks/idx88c-help/msganl.txt"/HOST/QUIET
COPY DUA5:[000000]MSGANL2.TXT "testdata/disks/idx88c-help/msganl2.txt"/HOST/QUIET
DISMOUNT DUA5
