! COPYOUTF.CMD - copies the rerun's output off its exchange volume.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/copyoutf.cmd
!
MOUNT DUA2 "testdata/disks/for81f-exchange.dsk"
COPY DUA2:[000000]FOR81F-VAX.TXT "testdata/for81/vax/for81f-vax.txt"/HOST
COPY DUA2:[000000]FOR81F.LOG "testdata/for81/vax/for81f.log"/HOST
DISMOUNT DUA2
