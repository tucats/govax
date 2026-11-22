! COPYOUT.CMD - copies VMS's objects and log off the exchange volume. Written by
! testdata/tir111/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/tir111/vax
!     govax console < testdata/tir111/copyout.cmd
!
MOUNT/NOWRITE DUA5: "testdata/disks/tir111-exchange.dsk"
COPY/QUIET DUA5:[000000]TIR111.LOG "testdata/tir111/vax/tir111.log"/HOST
COPY/QUIET DUA5:[000000]TIRA.OBJ "testdata/tir111/vax/tira.obj"/HOST
COPY/QUIET DUA5:[000000]TIRB.OBJ "testdata/tir111/vax/tirb.obj"/HOST
COPY/QUIET DUA5:[000000]TIRC1.OBJ "testdata/tir111/vax/tirc1.obj"/HOST
COPY/QUIET DUA5:[000000]TIRC2.OBJ "testdata/tir111/vax/tirc2.obj"/HOST
COPY/QUIET DUA5:[000000]TIRC3.OBJ "testdata/tir111/vax/tirc3.obj"/HOST
COPY/QUIET DUA5:[000000]TIRC4.OBJ "testdata/tir111/vax/tirc4.obj"/HOST
COPY/QUIET DUA5:[000000]TIRC8.OBJ "testdata/tir111/vax/tirc8.obj"/HOST
COPY/QUIET DUA5:[000000]TIRD.OBJ "testdata/tir111/vax/tird.obj"/HOST
COPY/QUIET DUA5:[000000]TIRE.OBJ "testdata/tir111/vax/tire.obj"/HOST
COPY/QUIET DUA5:[000000]TIRF.OBJ "testdata/tir111/vax/tirf.obj"/HOST
COPY/QUIET DUA5:[000000]TIRG.OBJ "testdata/tir111/vax/tirg.obj"/HOST
COPY/QUIET DUA5:[000000]TIRR.OBJ "testdata/tir111/vax/tirr.obj"/HOST
COPY/QUIET DUA5:[000000]TIRX.OBJ "testdata/tir111/vax/tirx.obj"/HOST
DISMOUNT DUA5:
EXIT
