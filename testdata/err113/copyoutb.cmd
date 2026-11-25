! COPYOUTB.CMD - copies VMS's results off Phase 113's probe round 2's volume
! (testdata/err113's README, "Round 2"). Run from the repository root, after the
! VAX has dismounted it:
!
!     mkdir -p testdata/err113/vax/round2
!     govax --profile tbprobe --system-file none console < testdata/err113/copyoutb.cmd
!
MOUNT/NOWRITE DUA5: "testdata/disks/err113b-exchange.dsk"
COPY/QUIET DUA5:[000000]ERR113B.LOG "testdata/err113/vax/round2/err113b.log"/HOST
COPY/QUIET DUA5:[ERR113]LNK113L.MAP "testdata/err113/vax/round2/lnk113l.map"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113L.EXE "testdata/err113/vax/round2/lnk113l.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113L.OBJ "testdata/err113/vax/round2/lnk113l.obj"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D00.EXE "testdata/err113/vax/round2/lnk113d00.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D01.EXE "testdata/err113/vax/round2/lnk113d01.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D02.EXE "testdata/err113/vax/round2/lnk113d02.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D03.EXE "testdata/err113/vax/round2/lnk113d03.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D04.EXE "testdata/err113/vax/round2/lnk113d04.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D05.EXE "testdata/err113/vax/round2/lnk113d05.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D06.EXE "testdata/err113/vax/round2/lnk113d06.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D07.EXE "testdata/err113/vax/round2/lnk113d07.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D08.EXE "testdata/err113/vax/round2/lnk113d08.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D09.EXE "testdata/err113/vax/round2/lnk113d09.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D10.EXE "testdata/err113/vax/round2/lnk113d10.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D11.EXE "testdata/err113/vax/round2/lnk113d11.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D12.EXE "testdata/err113/vax/round2/lnk113d12.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D13.EXE "testdata/err113/vax/round2/lnk113d13.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D14.EXE "testdata/err113/vax/round2/lnk113d14.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]LNK113D15.EXE "testdata/err113/vax/round2/lnk113d15.exe"/HOST
COPY/QUIET DUA5:[ERR113]MAC113F.LIS "testdata/err113/vax/round2/mac113f.lis"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]MAC113F.OBJ "testdata/err113/vax/round2/mac113f.obj"/HOST
COPY/QUIET DUA5:[FOR81M]FOR81M-VAX.TXT "testdata/for81/vax/for81m-vax.txt"/HOST
COPY/QUIET DUA5:[FOR81M]FOR81M.LOG "testdata/for81/vax/for81m.log"/HOST
DISMOUNT DUA5:
