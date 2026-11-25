! EXCHANGEB.CMD - builds Phase 113's probe round 2's volume (testdata/err113's
! README, "Round 2") with govax. Run from the repository root:
!
!     govax --profile tbprobe --system-file none console < testdata/err113/exchangeb.cmd
!
! The round's scratch disk is round 1's, testdata/disks/err113-scratch.dsk, which
! ERR113B.COM initializes again.
!
INITIALIZE/CONTAINER "testdata/disks/err113b-exchange.dsk" /DEVICE=RD54 ERR113B
MOUNT/WRITE DUA5: "testdata/disks/err113b-exchange.dsk"
SET DEFAULT DUA5:[000000]
COPY/QUIET "testdata/err113/err113b.com"/HOST ERR113B.COM
CREATE/DIRECTORY DUA5:[ERR113]
SET DEFAULT DUA5:[ERR113]
COPY/QUIET "testdata/err113/lnk113l.mar"/HOST LNK113L.MAR
COPY/QUIET "testdata/err113/lnk113l.opt"/HOST LNK113L.OPT
COPY/QUIET "testdata/err113/lnk113d.com"/HOST LNK113D.COM
COPY/QUIET "testdata/err113/lnk113d00.mar"/HOST LNK113D00.MAR
COPY/QUIET "testdata/err113/lnk113d01.mar"/HOST LNK113D01.MAR
COPY/QUIET "testdata/err113/lnk113d02.mar"/HOST LNK113D02.MAR
COPY/QUIET "testdata/err113/lnk113d03.mar"/HOST LNK113D03.MAR
COPY/QUIET "testdata/err113/lnk113d04.mar"/HOST LNK113D04.MAR
COPY/QUIET "testdata/err113/lnk113d05.mar"/HOST LNK113D05.MAR
COPY/QUIET "testdata/err113/lnk113d06.mar"/HOST LNK113D06.MAR
COPY/QUIET "testdata/err113/lnk113d07.mar"/HOST LNK113D07.MAR
COPY/QUIET "testdata/err113/lnk113d08.mar"/HOST LNK113D08.MAR
COPY/QUIET "testdata/err113/lnk113d09.mar"/HOST LNK113D09.MAR
COPY/QUIET "testdata/err113/lnk113d10.mar"/HOST LNK113D10.MAR
COPY/QUIET "testdata/err113/lnk113d11.mar"/HOST LNK113D11.MAR
COPY/QUIET "testdata/err113/lnk113d12.mar"/HOST LNK113D12.MAR
COPY/QUIET "testdata/err113/lnk113d13.mar"/HOST LNK113D13.MAR
COPY/QUIET "testdata/err113/lnk113d14.mar"/HOST LNK113D14.MAR
COPY/QUIET "testdata/err113/lnk113d15.mar"/HOST LNK113D15.MAR
COPY/QUIET "testdata/err113/mac113f.mar"/HOST MAC113F.MAR
COPY/QUIET "testdata/err113/fix8.fdl"/HOST FIX8.FDL
COPY/QUIET "testdata/err113/fix7.fdl"/HOST FIX7.FDL
COPY/QUIET "testdata/err113/fix1.fdl"/HOST FIX1.FDL
CREATE/DIRECTORY DUA5:[FOR81M]
SET DEFAULT DUA5:[FOR81M]
COPY/QUIET "testdata/for81/for81m.mar"/HOST FOR81M.MAR
COPY/QUIET "testdata/for81/for81w.mar"/HOST FOR81W.MAR
COPY/QUIET "testdata/for81/for81.opt"/HOST FOR81.OPT
COPY/QUIET "testdata/err113/for81m.com"/HOST FOR81M.COM
COPY/QUIET "testdata/err113/for81n.com"/HOST FOR81N.COM
SET DEFAULT DUA5:[000000]
DISMOUNT DUA5:
