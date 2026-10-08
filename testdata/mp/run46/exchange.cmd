! EXCHANGE.CMD - builds the exchange volume for every VAX run waiting at
! the end of Phase 46 (testdata/mp/run46/README.md). Run from the
! repository root:
!
!     govax console < testdata/mp/run46/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-run46.dsk" /DEVICE=RD53 MPRUN46
MOUNT/WRITE DUA1 "testdata/disks/mp-run46.dsk"
COPY "testdata/mp/macros/r5_misc.mar"/HOST DUA1:[000000]R5_MISC.MAR
COPY "testdata/mp/macros/r6_crmpsc.mar"/HOST DUA1:[000000]R6_CRMPSC.MAR
COPY "testdata/mp/macros/r6_mgblsc.mar"/HOST DUA1:[000000]R6_MGBLSC.MAR
COPY "testdata/mp/macros/r6_dgblsc.mar"/HOST DUA1:[000000]R6_DGBLSC.MAR
COPY "testdata/mp/macros/r6_enq.mar"/HOST DUA1:[000000]R6_ENQ.MAR
COPY "testdata/mp/macros/r6_enqw.mar"/HOST DUA1:[000000]R6_ENQW.MAR
COPY "testdata/mp/macros/r6_deq.mar"/HOST DUA1:[000000]R6_DEQ.MAR
COPY "testdata/mp/macros/r6_getlki.mar"/HOST DUA1:[000000]R6_GETLKI.MAR
COPY "testdata/mp/macros/r6_getlkiw.mar"/HOST DUA1:[000000]R6_GETLKIW.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
COPY "testdata/mp/macros/macros5.com"/HOST DUA1:[000000]MACROS5.COM
COPY "testdata/mp/macros/macros6.com"/HOST DUA1:[000000]MACROS6.COM
COPY "testdata/mp/defs/def_sec.mar"/HOST DUA1:[000000]DEF_SEC.MAR
COPY "testdata/mp/defs/def_lck.mar"/HOST DUA1:[000000]DEF_LCK.MAR
COPY "testdata/mp/defs/def_lki.mar"/HOST DUA1:[000000]DEF_LKI.MAR
COPY "testdata/mp/defs/def_psl.mar"/HOST DUA1:[000000]DEF_PSL.MAR
COPY "testdata/mp/defs/def_dc.mar"/HOST DUA1:[000000]DEF_DC.MAR
COPY "testdata/mp/defs/defs.com"/HOST DUA1:[000000]DEFS.COM
COPY "testdata/mp/probe3/probe3.mar"/HOST DUA1:[000000]PROBE3.MAR
COPY "testdata/mp/probe3/probe3c.mar"/HOST DUA1:[000000]PROBE3C.MAR
COPY "testdata/mp/probe3/probe3.com"/HOST DUA1:[000000]PROBE3.COM
COPY "testdata/mp/mbxpingpong.mar"/HOST DUA1:[000000]MBXPINGPONG.MAR
COPY "testdata/mp/mbxpong.mar"/HOST DUA1:[000000]MBXPONG.MAR
COPY "testdata/mp/pingpong.com"/HOST DUA1:[000000]PINGPONG.COM
COPY "testdata/mp/run46/run46.com"/HOST DUA1:[000000]RUN46.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
