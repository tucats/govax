! EXCHANGE.CMD - builds rounds 1c and 2's exchange volume with govax.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/help90c-exchange.dsk" /DEVICE=RD54 HELP90
MOUNT/WRITE DUA5 "testdata/disks/help90c-exchange.dsk"
COPY "testdata/help90/h90a1.hlp"/HOST DUA5:[000000]H90A1.HLP
COPY "testdata/help90/h90a2.hlp"/HOST DUA5:[000000]H90A2.HLP
COPY "testdata/help90/alpha2.hlp"/HOST DUA5:[000000]ALPHA2.HLP
COPY "testdata/help90/h90b.hlp"/HOST DUA5:[000000]H90B.HLP
COPY "testdata/help90/h90c.hlp"/HOST DUA5:[000000]H90C.HLP
COPY "testdata/help90/h90g.hlp"/HOST DUA5:[000000]H90G.HLP
COPY "testdata/help90/h90s.hlp"/HOST DUA5:[000000]H90S.HLP
COPY "testdata/help90/h90d.hlp"/HOST DUA5:[000000]H90D.HLP
COPY "testdata/help90/badtext.hlp"/HOST DUA5:[000000]BADTEXT.HLP
COPY "testdata/help90/skip.hlp"/HOST DUA5:[000000]SKIP.HLP
COPY "testdata/help90/noblank.hlp"/HOST DUA5:[000000]NOBLANK.HLP
COPY "testdata/help90/toolong.hlp"/HOST DUA5:[000000]TOOLONG.HLP
COPY "testdata/help90/lower.hlp"/HOST DUA5:[000000]LOWER.HLP
COPY "testdata/help90/digits.hlp"/HOST DUA5:[000000]DIGITS.HLP
COPY "testdata/help90/zero.hlp"/HOST DUA5:[000000]ZERO.HLP
COPY "testdata/help90/t1.txt"/HOST DUA5:[000000]T1.TXT
COPY "testdata/help90/t2.txt"/HOST DUA5:[000000]T2.TXT
COPY "testdata/help90/h90m.mar"/HOST DUA5:[000000]H90M.MAR
COPY "testdata/help90/resp.txt"/HOST DUA5:[000000]RESP.TXT
COPY "testdata/help90/help90.com"/HOST DUA5:[000000]HELP90.COM
COPY "testdata/help90/help90d.com"/HOST DUA5:[000000]HELP90D.COM
COPY "testdata/help90/help90v.com"/HOST DUA5:[000000]HELP90V.COM
COPY "testdata/help90/help90f.com"/HOST DUA5:[000000]HELP90F.COM
COPY "testdata/help90/help90d.dbg"/HOST DUA5:[000000]HELP90D.DBG
COPY "testdata/help90/help90v.dbg"/HOST DUA5:[000000]HELP90V.DBG
COPY "testdata/help90/dbgt.mar"/HOST DUA5:[000000]DBGT.MAR
COPY "testdata/help90/lbr90.mar"/HOST DUA5:[000000]LBR90.MAR
COPY "testdata/help90/deeper.hlp"/HOST DUA5:[000000]DEEPER.HLP
COPY "testdata/help90/help90c.com"/HOST DUA5:[000000]HELP90C.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
