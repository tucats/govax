! EXCHANGE.CMD - builds Phase 113's probe round volume and its scratch disk with
! govax (testdata/err113's README). Run from the repository root:
!
!     govax --profile tbprobe --system-file none console < testdata/err113/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/err113-scratch.dsk" /DEVICE=RD51 SCR113
INITIALIZE/CONTAINER "testdata/disks/err113-exchange.dsk" /DEVICE=RD54 ERR113
MOUNT/WRITE DUA5: "testdata/disks/err113-exchange.dsk"
SET DEFAULT DUA5:[000000]
COPY/QUIET "testdata/err113/err113.com"/HOST ERR113.COM
CREATE/DIRECTORY DUA5:[ERR113]
SET DEFAULT DUA5:[ERR113]
COPY/QUIET "testdata/str113/str113.mar"/HOST STR113.MAR
COPY/QUIET "testdata/svc113/svc113.mar"/HOST SVC113.MAR
COPY/QUIET "testdata/svc113/def_dib.mar"/HOST DEF_DIB.MAR
COPY/QUIET "testdata/err113/ini113.mar"/HOST INI113.MAR
COPY/QUIET "testdata/err113/ini113.opt"/HOST INI113.OPT
COPY/QUIET "testdata/err113/ini113s.mar"/HOST INI113S.MAR
COPY/QUIET "testdata/err113/ini113s.opt"/HOST INI113S.OPT
COPY/QUIET "testdata/err113/mac113a.mar"/HOST MAC113A.MAR
COPY/QUIET "testdata/err113/mac113b.mar"/HOST MAC113B.MAR
COPY/QUIET "testdata/err113/mac113c1.mar"/HOST MAC113C1.MAR
COPY/QUIET "testdata/err113/mac113c2.mar"/HOST MAC113C2.MAR
COPY/QUIET "testdata/err113/mac113d1.mar"/HOST MAC113D1.MAR
COPY/QUIET "testdata/err113/mac113d2.mar"/HOST MAC113D2.MAR
COPY/QUIET "testdata/err113/mac113e.mar"/HOST MAC113E.MAR
COPY/QUIET "testdata/err113/nocc.fdl"/HOST NOCC.FDL
CREATE/DIRECTORY DUA5:[FOR81G]
SET DEFAULT DUA5:[FOR81G]
COPY/QUIET "testdata/for81/for81g.mar"/HOST FOR81G.MAR
COPY/QUIET "testdata/for81/for81t.mar"/HOST FOR81T.MAR
COPY/QUIET "testdata/for81/for81.opt"/HOST FOR81.OPT
COPY/QUIET "testdata/for81/for81g.com"/HOST FOR81G.COM
COPY/QUIET "testdata/for81/for81h.com"/HOST FOR81H.COM
SET DEFAULT DUA5:[000000]
DISMOUNT DUA5:
