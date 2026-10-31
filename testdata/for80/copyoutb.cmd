! COPYOUTB.CMD - copies the VAX's second run off its exchange volume.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/copyoutb.cmd
!
MOUNT DUA2 "testdata/disks/for80b-exchange.dsk"
COPY DUA2:[000000]FOR80B-VAX.TXT "testdata/for80/vax/for80b-vax.txt"/HOST
COPY DUA2:[000000]FOR80B.LOG "testdata/for80/vax/for80b.log"/HOST
DISMOUNT DUA2
