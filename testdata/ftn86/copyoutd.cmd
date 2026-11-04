! COPYOUTD.CMD - copies DUNGEON's log off the exchange volume, to the gitignored
! testdata/disks: it holds the game's text. Written by testdata/ftn86/gen.go. Run from the
! repository root, after the VAX has dismounted the volume:
!
!     govax console < testdata/ftn86/copyoutd.cmd
!
MOUNT/NOWRITE DUA5: "testdata/disks/ftn86-exchange.dsk"
COPY/QUIET DUA5:[000000]FTN86D.LOG "testdata/disks/ftn86d-round4.log"/HOST
DISMOUNT DUA5:
QUIT
