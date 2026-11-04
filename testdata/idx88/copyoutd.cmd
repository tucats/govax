! COPYOUTD.CMD - copies round 4's log off its exchange volume. Written by
! hand. Run from the repository root:
!
!     govax console < testdata/idx88/copyoutd.cmd
!
! IDX88DM.LOG, MACRO's own output, stays on the volume until the author has
! read it (the clean-room notes).
MOUNT DUA5 "testdata/disks/idx88d-exchange.dsk"
COPY DUA5:[000000]IDX88D.LOG "testdata/idx88/vax/idx88d.log"/HOST/QUIET
DISMOUNT DUA5
