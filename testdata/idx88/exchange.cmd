! EXCHANGE.CMD - builds the Phase 88 probe's exchange volume with govax.
! Written by testdata/idx88/gen.go. Run from the repository root:
!
!     govax console < testdata/idx88/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/idx88-exchange.dsk" /DEVICE=RD54 IDX88
MOUNT/WRITE DUA5 "testdata/disks/idx88-exchange.dsk"
COPY "testdata/idx88/idx88.mar"/HOST DUA5:[000000]IDX88.MAR
COPY "testdata/idx88/idx88.com"/HOST DUA5:[000000]IDX88.COM
COPY "testdata/idx88/idx88w.dat"/HOST DUA5:[000000]IDX88W.DAT
COPY "testdata/idx88/def_bkt.mar"/HOST DUA5:[000000]DEF_BKT.MAR
COPY "testdata/idx88/def_irc.mar"/HOST DUA5:[000000]DEF_IRC.MAR
COPY "testdata/idx88/def_key.mar"/HOST DUA5:[000000]DEF_KEY.MAR
COPY "testdata/idx88/def_area.mar"/HOST DUA5:[000000]DEF_AREA.MAR
COPY "testdata/idx88/def_plg.mar"/HOST DUA5:[000000]DEF_PLG.MAR
DIRECTORY DUA5:[000000]
DISMOUNT DUA5
