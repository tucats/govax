! EXCHANGE.CMD - builds Phase 98's probe exchange volume with govax.
! Written by testdata/bat98/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/bat98-exchange.dsk" /DEVICE=RD53 BAT98
MOUNT/WRITE DUA5 "testdata/disks/bat98-exchange.dsk"
CREATE/DIRECTORY DUA5:[BAT98LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[BAT98SCR]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[BAT98]
CREATE/DIRECTORY DUA5:[BAT98O]
CREATE/DIRECTORY DUA5:[BAT98W]
COPY "testdata/bat98/bat98.com"/HOST DUA5:[000000]BAT98.COM
COPY "testdata/bat98/bat98acct.com"/HOST DUA5:[000000]BAT98ACCT.COM
COPY "testdata/bat98/bat98drop.com"/HOST DUA5:[000000]BAT98DROP.COM
COPY "testdata/bat98/bat98z.com"/HOST DUA5:[000000]BAT98Z.COM
COPY "testdata/bat98/bat98j.com"/HOST DUA5:[000000]BAT98J.COM
COPY "testdata/bat98/bat98p.com"/HOST DUA5:[000000]BAT98P.COM
COPY "testdata/bat98/bat98ok.com"/HOST DUA5:[000000]BAT98OK.COM
COPY "testdata/bat98/bat98wrn.com"/HOST DUA5:[000000]BAT98WRN.COM
COPY "testdata/bat98/bat98err.com"/HOST DUA5:[000000]BAT98ERR.COM
COPY "testdata/bat98/bat98fat.com"/HOST DUA5:[000000]BAT98FAT.COM
COPY "testdata/bat98/bat98bad.com"/HOST DUA5:[000000]BAT98BAD.COM
COPY "testdata/bat98/bat98w.com"/HOST DUA5:[000000]BAT98W.COM
COPY "testdata/bat98/bat98c.com"/HOST DUA5:[000000]BAT98C.COM
COPY "testdata/bat98/bat98o.com"/HOST DUA5:[000000]BAT98O.COM
COPY "testdata/bat98/bat98r.com"/HOST DUA5:[000000]BAT98R.COM
COPY "testdata/bat98/bat98q.mar"/HOST DUA5:[000000]BAT98Q.MAR
COPY "testdata/bat98/def_sjc.mar"/HOST DUA5:[000000]DEF_SJC.MAR
COPY "testdata/bat98/def_qui.mar"/HOST DUA5:[000000]DEF_QUI.MAR
COPY "testdata/bat98/def_jbc.mar"/HOST DUA5:[000000]DEF_JBC.MAR
COPY "testdata/bat98/login_bat98.com"/HOST DUA5:[BAT98]LOGIN.COM
COPY "testdata/bat98/login_bat98o.com"/HOST DUA5:[BAT98O]LOGIN.COM
COPY "testdata/bat98/login_bat98w.com"/HOST DUA5:[BAT98W]LOGIN.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000...]
DISMOUNT DUA5
