! COPYOUT.CMD - copies VMS's results off Phase 113's probe round volume
! (testdata/err113's README). Run from the repository root, after the VAX has
! dismounted it:
!
!     mkdir -p testdata/err113/vax testdata/str113/vax testdata/svc113/vax
!     govax --profile tbprobe --system-file none console < testdata/err113/copyout.cmd
!
! The scratch disk stays in testdata/disks, as the VAX left it.
!
MOUNT/NOWRITE DUA5: "testdata/disks/err113-exchange.dsk"
COPY/QUIET DUA5:[000000]ERR113.LOG "testdata/err113/vax/err113.log"/HOST
COPY/QUIET DUA5:[ERR113]STR113.TXT "testdata/str113/vax/str113-vax.txt"/HOST
COPY/QUIET DUA5:[ERR113]SVC113.TXT "testdata/svc113/vax/svc113-vax.txt"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]DEF_DIB.OBJ "testdata/svc113/vax/def_dib.obj"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]INI113.EXE "testdata/err113/vax/ini113.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]INI113S.EXE "testdata/err113/vax/ini113s.exe"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]INI113NT.EXE "testdata/err113/vax/ini113nt.exe"/HOST
COPY/QUIET DUA5:[ERR113]MAC113A.LIS "testdata/err113/vax/mac113a.lis"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]MAC113A.OBJ "testdata/err113/vax/mac113a.obj"/HOST
COPY/QUIET DUA5:[ERR113]MAC113B.LIS "testdata/err113/vax/mac113b.lis"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]MAC113B.OBJ "testdata/err113/vax/mac113b.obj"/HOST
COPY/QUIET DUA5:[ERR113]MAC113C.LIS "testdata/err113/vax/mac113c.lis"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]MAC113C.OBJ "testdata/err113/vax/mac113c.obj"/HOST
COPY/QUIET DUA5:[ERR113]MAC113D.LIS "testdata/err113/vax/mac113d.lis"/HOST
COPY/QUIET/BINARY DUA5:[ERR113]MAC113D.OBJ "testdata/err113/vax/mac113d.obj"/HOST
COPY/QUIET DUA5:[FOR81G]FOR81G-VAX.TXT "testdata/for81/vax/for81g-vax.txt"/HOST
COPY/QUIET DUA5:[FOR81G]FOR81G.LOG "testdata/for81/vax/for81g.log"/HOST
DISMOUNT DUA5:
