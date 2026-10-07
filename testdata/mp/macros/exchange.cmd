! EXCHANGE.CMD - builds the system service macro probes' exchange volume
! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from
! the repository root:
!
!     govax console < testdata/mp/macros/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-macros.dsk" /DEVICE=RD53 MPMACROS
MOUNT/WRITE DUA1 "testdata/disks/mp-macros.dsk"
COPY "testdata/mp/macros/r4_adjstk.mar"/HOST DUA1:[000000]R4_ADJSTK.MAR
COPY "testdata/mp/macros/r4_adjwsl.mar"/HOST DUA1:[000000]R4_ADJWSL.MAR
COPY "testdata/mp/macros/r4_alloc.mar"/HOST DUA1:[000000]R4_ALLOC.MAR
COPY "testdata/mp/macros/r4_dalloc.mar"/HOST DUA1:[000000]R4_DALLOC.MAR
COPY "testdata/mp/macros/r4_ascefc.mar"/HOST DUA1:[000000]R4_ASCEFC.MAR
COPY "testdata/mp/macros/r4_dacefc.mar"/HOST DUA1:[000000]R4_DACEFC.MAR
COPY "testdata/mp/macros/r4_dlcefc.mar"/HOST DUA1:[000000]R4_DLCEFC.MAR
COPY "testdata/mp/macros/r4_wfland.mar"/HOST DUA1:[000000]R4_WFLAND.MAR
COPY "testdata/mp/macros/r4_wflor.mar"/HOST DUA1:[000000]R4_WFLOR.MAR
COPY "testdata/mp/macros/r4_synch.mar"/HOST DUA1:[000000]R4_SYNCH.MAR
COPY "testdata/mp/macros/r4_cancel.mar"/HOST DUA1:[000000]R4_CANCEL.MAR
COPY "testdata/mp/macros/r4_setast.mar"/HOST DUA1:[000000]R4_SETAST.MAR
COPY "testdata/mp/macros/r4_dclast.mar"/HOST DUA1:[000000]R4_DCLAST.MAR
COPY "testdata/mp/macros/r4_dclexh.mar"/HOST DUA1:[000000]R4_DCLEXH.MAR
COPY "testdata/mp/macros/r4_canexh.mar"/HOST DUA1:[000000]R4_CANEXH.MAR
COPY "testdata/mp/macros/r4_setexv.mar"/HOST DUA1:[000000]R4_SETEXV.MAR
COPY "testdata/mp/macros/r4_setprv.mar"/HOST DUA1:[000000]R4_SETPRV.MAR
COPY "testdata/mp/macros/r4_cmkrnl.mar"/HOST DUA1:[000000]R4_CMKRNL.MAR
COPY "testdata/mp/macros/r4_cmexec.mar"/HOST DUA1:[000000]R4_CMEXEC.MAR
COPY "testdata/mp/macros/r4_asctim.mar"/HOST DUA1:[000000]R4_ASCTIM.MAR
COPY "testdata/mp/macros/r4_bintim.mar"/HOST DUA1:[000000]R4_BINTIM.MAR
COPY "testdata/mp/macros/r4_gettim.mar"/HOST DUA1:[000000]R4_GETTIM.MAR
COPY "testdata/mp/macros/r4_numtim.mar"/HOST DUA1:[000000]R4_NUMTIM.MAR
COPY "testdata/mp/macros/r4_faol.mar"/HOST DUA1:[000000]R4_FAOL.MAR
COPY "testdata/mp/macros/r4_fao.mar"/HOST DUA1:[000000]R4_FAO.MAR
COPY "testdata/mp/macros/r4_putmsg.mar"/HOST DUA1:[000000]R4_PUTMSG.MAR
COPY "testdata/mp/macros/r4_getmsg.mar"/HOST DUA1:[000000]R4_GETMSG.MAR
COPY "testdata/mp/macros/r4_crelnm.mar"/HOST DUA1:[000000]R4_CRELNM.MAR
COPY "testdata/mp/macros/r4_dellnm.mar"/HOST DUA1:[000000]R4_DELLNM.MAR
COPY "testdata/mp/macros/r4_trnlnm.mar"/HOST DUA1:[000000]R4_TRNLNM.MAR
COPY "testdata/mp/macros/r4_crelnt.mar"/HOST DUA1:[000000]R4_CRELNT.MAR
COPY "testdata/mp/macros/r4_crelog.mar"/HOST DUA1:[000000]R4_CRELOG.MAR
COPY "testdata/mp/macros/r4_dellog.mar"/HOST DUA1:[000000]R4_DELLOG.MAR
COPY "testdata/mp/macros/r4_trnlog.mar"/HOST DUA1:[000000]R4_TRNLOG.MAR
COPY "testdata/mp/macros/r4_sndopr.mar"/HOST DUA1:[000000]R4_SNDOPR.MAR
COPY "testdata/mp/macros/r4_expreg.mar"/HOST DUA1:[000000]R4_EXPREG.MAR
COPY "testdata/mp/macros/r4_cntreg.mar"/HOST DUA1:[000000]R4_CNTREG.MAR
COPY "testdata/mp/macros/r4_cretva.mar"/HOST DUA1:[000000]R4_CRETVA.MAR
COPY "testdata/mp/macros/r4_deltva.mar"/HOST DUA1:[000000]R4_DELTVA.MAR
COPY "testdata/mp/macros/r4_lckpag.mar"/HOST DUA1:[000000]R4_LCKPAG.MAR
COPY "testdata/mp/macros/r4_ulkpag.mar"/HOST DUA1:[000000]R4_ULKPAG.MAR
COPY "testdata/mp/macros/r4_lkwset.mar"/HOST DUA1:[000000]R4_LKWSET.MAR
COPY "testdata/mp/macros/r4_ulwset.mar"/HOST DUA1:[000000]R4_ULWSET.MAR
COPY "testdata/mp/macros/r4_setprt.mar"/HOST DUA1:[000000]R4_SETPRT.MAR
COPY "testdata/mp/macros/r4_setrwm.mar"/HOST DUA1:[000000]R4_SETRWM.MAR
COPY "testdata/mp/macros/r4_getsyi.mar"/HOST DUA1:[000000]R4_GETSYI.MAR
COPY "testdata/mp/macros/r4_getsyiw.mar"/HOST DUA1:[000000]R4_GETSYIW.MAR
COPY "testdata/mp/macros/r4_idtoasc.mar"/HOST DUA1:[000000]R4_IDTOASC.MAR
COPY "testdata/mp/macros/r4_asctoid.mar"/HOST DUA1:[000000]R4_ASCTOID.MAR
COPY "testdata/mp/macros/r4_unwind.mar"/HOST DUA1:[000000]R4_UNWIND.MAR
COPY "testdata/mp/macros/r4_brkthru.mar"/HOST DUA1:[000000]R4_BRKTHRU.MAR
COPY "testdata/mp/macros/r4_brkthruw.mar"/HOST DUA1:[000000]R4_BRKTHRUW.MAR
COPY "testdata/mp/macros/r4_creprc.mar"/HOST DUA1:[000000]R4_CREPRC.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
