! EXCHANGE.CMD - builds the TIR probe's exchange volume with govax. Written by
! testdata/tir111/gen.go. Run from the repository root:
!
!     govax console < testdata/tir111/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/tir111-exchange.dsk" /SIZE=20000 TIR111
MOUNT/WRITE DUA5: "testdata/disks/tir111-exchange.dsk"
SET DEFAULT DUA5:[000000]
COPY/QUIET "testdata/tir111/tir111.com"/HOST TIR111.COM
COPY/QUIET "testdata/tir111/tira.mar"/HOST TIRA.MAR
COPY/QUIET "testdata/tir111/tirb.mar"/HOST TIRB.MAR
COPY/QUIET "testdata/tir111/tirc1.mar"/HOST TIRC1.MAR
COPY/QUIET "testdata/tir111/tirc2.mar"/HOST TIRC2.MAR
COPY/QUIET "testdata/tir111/tirc3.mar"/HOST TIRC3.MAR
COPY/QUIET "testdata/tir111/tirc4.mar"/HOST TIRC4.MAR
COPY/QUIET "testdata/tir111/tirc8.mar"/HOST TIRC8.MAR
COPY/QUIET "testdata/tir111/tird.mar"/HOST TIRD.MAR
COPY/QUIET "testdata/tir111/tire.mar"/HOST TIRE.MAR
COPY/QUIET "testdata/tir111/tirf.mar"/HOST TIRF.MAR
COPY/QUIET "testdata/tir111/tirg.mar"/HOST TIRG.MAR
COPY/QUIET "testdata/tir111/tirr.mar"/HOST TIRR.MAR
COPY/QUIET "testdata/tir111/tirx.mar"/HOST TIRX.MAR
SET DEFAULT DUA2:[000000]
DISMOUNT DUA5:
EXIT
