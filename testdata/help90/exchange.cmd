! EXCHANGE.CMD - builds round 3's exchange volume with govax.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/help90w-exchange.dsk" /DEVICE=RD54 HELP90
MOUNT/WRITE DUA5 "testdata/disks/help90w-exchange.dsk"
COPY "testdata/help90/t1.txt"/HOST DUA5:[000000]T1.TXT
COPY "testdata/help90/t2.txt"/HOST DUA5:[000000]T2.TXT
COPY "testdata/help90/one.txt"/HOST DUA5:[000000]ONE.TXT
COPY "testdata/help90/two.txt"/HOST DUA5:[000000]TWO.TXT
COPY "testdata/help90/w10.hlp"/HOST DUA5:[000000]W10.HLP
COPY "testdata/help90/h90o.mar"/HOST DUA5:[000000]H90O.MAR
COPY "testdata/help90/h90om.mar"/HOST DUA5:[000000]H90OM.MAR
COPY "testdata/help90/h90b.hlp"/HOST DUA5:[000000]H90B.HLP
COPY "testdata/help90/toolong.hlp"/HOST DUA5:[000000]TOOLONG.HLP
COPY "testdata/help90/digits.hlp"/HOST DUA5:[000000]DIGITS.HLP
COPY "testdata/help90/multi.hlp"/HOST DUA5:[000000]MULTI.HLP
COPY "testdata/help90/notfirst.hlp"/HOST DUA5:[000000]NOTFIRST.HLP
COPY "testdata/help90/goodbad.hlp"/HOST DUA5:[000000]GOODBAD.HLP
COPY "testdata/help90/lbr90w.mar"/HOST DUA5:[000000]LBR90W.MAR
COPY "testdata/help90/help90w.com"/HOST DUA5:[000000]HELP90W.COM
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
