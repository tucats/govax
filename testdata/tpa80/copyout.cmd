! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/tpa80/gen.go. Run from the repository root, after the VAX has
! dismounted the volume:
!
!     govax console < testdata/tpa80/copyout.cmd
!
MOUNT/NOWRITE DUA2 "testdata/disks/tpa80-exchange.dsk"
COPY DUA2:[000000]TPA80.OUT "testdata/tpa80/vax/tpa80-vax.txt"/HOST
COPY DUA2:[000000]TPA80.LOG "testdata/tpa80/vax/tpa80.log"/HOST
COPY DUA2:[000000]TPA80.MLG "testdata/tpa80/vax/tpa80.mlg"/HOST
COPY DUA2:[000000]TPA80B.MLG "testdata/tpa80/vax/tpa80b.mlg"/HOST
COPY DUA2:[000000]TPA80C.MLG "testdata/tpa80/vax/tpa80c.mlg"/HOST
COPY/BINARY DUA2:[000000]TPA80.OBJ "testdata/tpa80/vax/tpa80.obj"/HOST
COPY/BINARY DUA2:[000000]TPA80B.OBJ "testdata/tpa80/vax/tpa80b.obj"/HOST
COPY DUA2:[000000]TPA80.ANL "testdata/tpa80/vax/tpa80.anl"/HOST
COPY DUA2:[000000]TPA80B.ANL "testdata/tpa80/vax/tpa80b.anl"/HOST
COPY DUA2:[000000]TPA80.MAP "testdata/tpa80/vax/tpa80.map"/HOST
DISMOUNT DUA2
