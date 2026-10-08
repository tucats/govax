! EXCHANGE.CMD - builds the definition probes' exchange volume with govax
! (testdata/mp/defs/README.md): Phase 46's five. Run from the repository
! root:
!
!     govax console < testdata/mp/defs/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-defs.dsk" /DEVICE=RD53 MPDEFS
MOUNT/WRITE DUA1 "testdata/disks/mp-defs.dsk"
COPY "testdata/mp/defs/def_sec.mar"/HOST DUA1:[000000]DEF_SEC.MAR
COPY "testdata/mp/defs/def_lck.mar"/HOST DUA1:[000000]DEF_LCK.MAR
COPY "testdata/mp/defs/def_lki.mar"/HOST DUA1:[000000]DEF_LKI.MAR
COPY "testdata/mp/defs/def_psl.mar"/HOST DUA1:[000000]DEF_PSL.MAR
COPY "testdata/mp/defs/def_dc.mar"/HOST DUA1:[000000]DEF_DC.MAR
COPY "testdata/mp/defs/defs.com"/HOST DUA1:[000000]DEFS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
