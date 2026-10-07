! EXCHANGE.CMD - builds the Phase 45 definition probes' exchange volume
! with govax (testdata/mp/defs/README.md). Run from the repository root:
!
!     govax console < testdata/mp/defs/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-defs.dsk" /DEVICE=RD53 MPDEFS
MOUNT/WRITE DUA1 "testdata/disks/mp-defs.dsk"
COPY "testdata/mp/defs/def_acc.mar"/HOST DUA1:[000000]DEF_ACC.MAR
COPY "testdata/mp/defs/def_msg.mar"/HOST DUA1:[000000]DEF_MSG.MAR
COPY "testdata/mp/defs/def_pql.mar"/HOST DUA1:[000000]DEF_PQL.MAR
COPY "testdata/mp/defs/def_prc.mar"/HOST DUA1:[000000]DEF_PRC.MAR
COPY "testdata/mp/defs/defs.com"/HOST DUA1:[000000]DEFS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
