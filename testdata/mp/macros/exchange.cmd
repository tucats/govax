! EXCHANGE.CMD - builds the system service macro probes' exchange volume
! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from
! the repository root:
!
!     govax console < testdata/mp/macros/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-macros.dsk" /DEVICE=RD53 MPMACROS
MOUNT/WRITE DUA1 "testdata/disks/mp-macros.dsk"
COPY "testdata/mp/macros/lst_creprc.mar"/HOST DUA1:[000000]LST_CREPRC.MAR
COPY "testdata/mp/macros/lst_delprc.mar"/HOST DUA1:[000000]LST_DELPRC.MAR
COPY "testdata/mp/macros/lst_wake.mar"/HOST DUA1:[000000]LST_WAKE.MAR
COPY "testdata/mp/macros/lst_hiber.mar"/HOST DUA1:[000000]LST_HIBER.MAR
COPY "testdata/mp/macros/lst_schdwk.mar"/HOST DUA1:[000000]LST_SCHDWK.MAR
COPY "testdata/mp/macros/lst_canwak.mar"/HOST DUA1:[000000]LST_CANWAK.MAR
COPY "testdata/mp/macros/lst_forcex.mar"/HOST DUA1:[000000]LST_FORCEX.MAR
COPY "testdata/mp/macros/lst_suspnd.mar"/HOST DUA1:[000000]LST_SUSPND.MAR
COPY "testdata/mp/macros/lst_resume.mar"/HOST DUA1:[000000]LST_RESUME.MAR
COPY "testdata/mp/macros/lst_setpri.mar"/HOST DUA1:[000000]LST_SETPRI.MAR
COPY "testdata/mp/macros/lst_setprn.mar"/HOST DUA1:[000000]LST_SETPRN.MAR
COPY "testdata/mp/macros/lst_getjpi.mar"/HOST DUA1:[000000]LST_GETJPI.MAR
COPY "testdata/mp/macros/lst_getjpiw.mar"/HOST DUA1:[000000]LST_GETJPIW.MAR
COPY "testdata/mp/macros/lst_getdvi.mar"/HOST DUA1:[000000]LST_GETDVI.MAR
COPY "testdata/mp/macros/lst_getdviw.mar"/HOST DUA1:[000000]LST_GETDVIW.MAR
COPY "testdata/mp/macros/lst_crembx.mar"/HOST DUA1:[000000]LST_CREMBX.MAR
COPY "testdata/mp/macros/lst_delmbx.mar"/HOST DUA1:[000000]LST_DELMBX.MAR
COPY "testdata/mp/macros/lst_setimr.mar"/HOST DUA1:[000000]LST_SETIMR.MAR
COPY "testdata/mp/macros/lst_cantim.mar"/HOST DUA1:[000000]LST_CANTIM.MAR
COPY "testdata/mp/macros/lst_waitfr.mar"/HOST DUA1:[000000]LST_WAITFR.MAR
COPY "testdata/mp/macros/lst_setef.mar"/HOST DUA1:[000000]LST_SETEF.MAR
COPY "testdata/mp/macros/lst_clref.mar"/HOST DUA1:[000000]LST_CLREF.MAR
COPY "testdata/mp/macros/lst_readef.mar"/HOST DUA1:[000000]LST_READEF.MAR
COPY "testdata/mp/macros/ext_adjstk.mar"/HOST DUA1:[000000]EXT_ADJSTK.MAR
COPY "testdata/mp/macros/ext_adjwsl.mar"/HOST DUA1:[000000]EXT_ADJWSL.MAR
COPY "testdata/mp/macros/ext_alloc.mar"/HOST DUA1:[000000]EXT_ALLOC.MAR
COPY "testdata/mp/macros/ext_dalloc.mar"/HOST DUA1:[000000]EXT_DALLOC.MAR
COPY "testdata/mp/macros/ext_ascefc.mar"/HOST DUA1:[000000]EXT_ASCEFC.MAR
COPY "testdata/mp/macros/ext_dacefc.mar"/HOST DUA1:[000000]EXT_DACEFC.MAR
COPY "testdata/mp/macros/ext_dlcefc.mar"/HOST DUA1:[000000]EXT_DLCEFC.MAR
COPY "testdata/mp/macros/ext_wfland.mar"/HOST DUA1:[000000]EXT_WFLAND.MAR
COPY "testdata/mp/macros/ext_wflor.mar"/HOST DUA1:[000000]EXT_WFLOR.MAR
COPY "testdata/mp/macros/ext_synch.mar"/HOST DUA1:[000000]EXT_SYNCH.MAR
COPY "testdata/mp/macros/ext_cancel.mar"/HOST DUA1:[000000]EXT_CANCEL.MAR
COPY "testdata/mp/macros/ext_setast.mar"/HOST DUA1:[000000]EXT_SETAST.MAR
COPY "testdata/mp/macros/ext_dclast.mar"/HOST DUA1:[000000]EXT_DCLAST.MAR
COPY "testdata/mp/macros/ext_dclexh.mar"/HOST DUA1:[000000]EXT_DCLEXH.MAR
COPY "testdata/mp/macros/ext_canexh.mar"/HOST DUA1:[000000]EXT_CANEXH.MAR
COPY "testdata/mp/macros/ext_setexv.mar"/HOST DUA1:[000000]EXT_SETEXV.MAR
COPY "testdata/mp/macros/ext_setprv.mar"/HOST DUA1:[000000]EXT_SETPRV.MAR
COPY "testdata/mp/macros/ext_cmkrnl.mar"/HOST DUA1:[000000]EXT_CMKRNL.MAR
COPY "testdata/mp/macros/ext_cmexec.mar"/HOST DUA1:[000000]EXT_CMEXEC.MAR
COPY "testdata/mp/macros/ext_asctim.mar"/HOST DUA1:[000000]EXT_ASCTIM.MAR
COPY "testdata/mp/macros/ext_bintim.mar"/HOST DUA1:[000000]EXT_BINTIM.MAR
COPY "testdata/mp/macros/ext_gettim.mar"/HOST DUA1:[000000]EXT_GETTIM.MAR
COPY "testdata/mp/macros/ext_numtim.mar"/HOST DUA1:[000000]EXT_NUMTIM.MAR
COPY "testdata/mp/macros/ext_faol.mar"/HOST DUA1:[000000]EXT_FAOL.MAR
COPY "testdata/mp/macros/ext_fao.mar"/HOST DUA1:[000000]EXT_FAO.MAR
COPY "testdata/mp/macros/ext_putmsg.mar"/HOST DUA1:[000000]EXT_PUTMSG.MAR
COPY "testdata/mp/macros/ext_getmsg.mar"/HOST DUA1:[000000]EXT_GETMSG.MAR
COPY "testdata/mp/macros/ext_crelnm.mar"/HOST DUA1:[000000]EXT_CRELNM.MAR
COPY "testdata/mp/macros/ext_dellnm.mar"/HOST DUA1:[000000]EXT_DELLNM.MAR
COPY "testdata/mp/macros/ext_trnlnm.mar"/HOST DUA1:[000000]EXT_TRNLNM.MAR
COPY "testdata/mp/macros/ext_crelnt.mar"/HOST DUA1:[000000]EXT_CRELNT.MAR
COPY "testdata/mp/macros/ext_crelog.mar"/HOST DUA1:[000000]EXT_CRELOG.MAR
COPY "testdata/mp/macros/ext_trnlog.mar"/HOST DUA1:[000000]EXT_TRNLOG.MAR
COPY "testdata/mp/macros/ext_dellog.mar"/HOST DUA1:[000000]EXT_DELLOG.MAR
COPY "testdata/mp/macros/ext_sndopr.mar"/HOST DUA1:[000000]EXT_SNDOPR.MAR
COPY "testdata/mp/macros/ext_expreg.mar"/HOST DUA1:[000000]EXT_EXPREG.MAR
COPY "testdata/mp/macros/ext_cntreg.mar"/HOST DUA1:[000000]EXT_CNTREG.MAR
COPY "testdata/mp/macros/ext_cretva.mar"/HOST DUA1:[000000]EXT_CRETVA.MAR
COPY "testdata/mp/macros/ext_deltva.mar"/HOST DUA1:[000000]EXT_DELTVA.MAR
COPY "testdata/mp/macros/ext_lckpag.mar"/HOST DUA1:[000000]EXT_LCKPAG.MAR
COPY "testdata/mp/macros/ext_ulkpag.mar"/HOST DUA1:[000000]EXT_ULKPAG.MAR
COPY "testdata/mp/macros/ext_lkwset.mar"/HOST DUA1:[000000]EXT_LKWSET.MAR
COPY "testdata/mp/macros/ext_ulwset.mar"/HOST DUA1:[000000]EXT_ULWSET.MAR
COPY "testdata/mp/macros/ext_setprt.mar"/HOST DUA1:[000000]EXT_SETPRT.MAR
COPY "testdata/mp/macros/ext_setrwm.mar"/HOST DUA1:[000000]EXT_SETRWM.MAR
COPY "testdata/mp/macros/ext_getsyi.mar"/HOST DUA1:[000000]EXT_GETSYI.MAR
COPY "testdata/mp/macros/ext_getsyiw.mar"/HOST DUA1:[000000]EXT_GETSYIW.MAR
COPY "testdata/mp/macros/ext_idtoasc.mar"/HOST DUA1:[000000]EXT_IDTOASC.MAR
COPY "testdata/mp/macros/ext_asctoid.mar"/HOST DUA1:[000000]EXT_ASCTOID.MAR
COPY "testdata/mp/macros/ext_unwind.mar"/HOST DUA1:[000000]EXT_UNWIND.MAR
COPY "testdata/mp/macros/ext_extra.mar"/HOST DUA1:[000000]EXT_EXTRA.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
