! EXCHANGE.CMD - builds the Phase 32 oracle's exchange volume with govax
! (docs/PHASE-32.md, subtask 3). Written by testdata/mar/rms/gen.go. Run
! from the repository root:
!
!     govax console < testdata/mar/rms/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/rms-exchange.dsk" /DEVICE=RD53 RMSXCHG
MOUNT/WRITE DUA1 "testdata/disks/rms-exchange.dsk"
COPY "testdata/mar/rms/def_atr.mar"/HOST DUA1:[000000]DEF_ATR.MAR
COPY "testdata/mar/rms/def_brk.mar"/HOST DUA1:[000000]DEF_BRK.MAR
COPY "testdata/mar/rms/def_dev.mar"/HOST DUA1:[000000]DEF_DEV.MAR
COPY "testdata/mar/rms/def_dvi.mar"/HOST DUA1:[000000]DEF_DVI.MAR
COPY "testdata/mar/rms/def_fab.mar"/HOST DUA1:[000000]DEF_FAB.MAR
COPY "testdata/mar/rms/def_fib.mar"/HOST DUA1:[000000]DEF_FIB.MAR
COPY "testdata/mar/rms/def_io.mar"/HOST DUA1:[000000]DEF_IO.MAR
COPY "testdata/mar/rms/def_jpi.mar"/HOST DUA1:[000000]DEF_JPI.MAR
COPY "testdata/mar/rms/def_lnm.mar"/HOST DUA1:[000000]DEF_LNM.MAR
COPY "testdata/mar/rms/def_nam.mar"/HOST DUA1:[000000]DEF_NAM.MAR
COPY "testdata/mar/rms/def_prt.mar"/HOST DUA1:[000000]DEF_PRT.MAR
COPY "testdata/mar/rms/def_prv.mar"/HOST DUA1:[000000]DEF_PRV.MAR
COPY "testdata/mar/rms/def_rab.mar"/HOST DUA1:[000000]DEF_RAB.MAR
COPY "testdata/mar/rms/def_rms.mar"/HOST DUA1:[000000]DEF_RMS.MAR
COPY "testdata/mar/rms/def_ss.mar"/HOST DUA1:[000000]DEF_SS.MAR
COPY "testdata/mar/rms/def_state.mar"/HOST DUA1:[000000]DEF_STATE.MAR
COPY "testdata/mar/rms/def_syi.mar"/HOST DUA1:[000000]DEF_SYI.MAR
COPY "testdata/mar/rms/def_tt.mar"/HOST DUA1:[000000]DEF_TT.MAR
COPY "testdata/mar/rms/def_tt2.mar"/HOST DUA1:[000000]DEF_TT2.MAR
COPY "testdata/mar/rms/def_twice.mar"/HOST DUA1:[000000]DEF_TWICE.MAR
COPY "testdata/mar/rms/def_xab.mar"/HOST DUA1:[000000]DEF_XAB.MAR
COPY "testdata/mar/rms/def_xaball.mar"/HOST DUA1:[000000]DEF_XABALL.MAR
COPY "testdata/mar/rms/def_xabdat.mar"/HOST DUA1:[000000]DEF_XABDAT.MAR
COPY "testdata/mar/rms/def_xabfhc.mar"/HOST DUA1:[000000]DEF_XABFHC.MAR
COPY "testdata/mar/rms/def_xabitm.mar"/HOST DUA1:[000000]DEF_XABITM.MAR
COPY "testdata/mar/rms/def_xabkey.mar"/HOST DUA1:[000000]DEF_XABKEY.MAR
COPY "testdata/mar/rms/def_xabpro.mar"/HOST DUA1:[000000]DEF_XABPRO.MAR
COPY "testdata/mar/rms/def_xabrdt.mar"/HOST DUA1:[000000]DEF_XABRDT.MAR
COPY "testdata/mar/rms/def_xabsum.mar"/HOST DUA1:[000000]DEF_XABSUM.MAR
COPY "testdata/mar/rms/def_xabtrm.mar"/HOST DUA1:[000000]DEF_XABTRM.MAR
COPY "testdata/mar/rms/err_def_global.mar"/HOST DUA1:[000000]ERR_DEF_GLOBAL.MAR
COPY "testdata/mar/rms/err_fac.mar"/HOST DUA1:[000000]ERR_FAC.MAR
COPY "testdata/mar/rms/err_keyword.mar"/HOST DUA1:[000000]ERR_KEYWORD.MAR
COPY "testdata/mar/rms/err_nop.mar"/HOST DUA1:[000000]ERR_NOP.MAR
COPY "testdata/mar/rms/err_org.mar"/HOST DUA1:[000000]ERR_ORG.MAR
COPY "testdata/mar/rms/err_rop2.mar"/HOST DUA1:[000000]ERR_ROP2.MAR
COPY "testdata/mar/rms/err_shr_nql.mar"/HOST DUA1:[000000]ERR_SHR_NQL.MAR
COPY "testdata/mar/rms/err_ss_global.mar"/HOST DUA1:[000000]ERR_SS_GLOBAL.MAR
COPY "testdata/mar/rms/err_store_reg.mar"/HOST DUA1:[000000]ERR_STORE_REG.MAR
COPY "testdata/mar/rms/err_xabdat_edt.mar"/HOST DUA1:[000000]ERR_XABDAT_EDT.MAR
COPY "testdata/mar/rms/init_fab.mar"/HOST DUA1:[000000]INIT_FAB.MAR
COPY "testdata/mar/rms/init_fab_all.mar"/HOST DUA1:[000000]INIT_FAB_ALL.MAR
COPY "testdata/mar/rms/init_fab_opt.mar"/HOST DUA1:[000000]INIT_FAB_OPT.MAR
COPY "testdata/mar/rms/init_nam.mar"/HOST DUA1:[000000]INIT_NAM.MAR
COPY "testdata/mar/rms/init_nam_all.mar"/HOST DUA1:[000000]INIT_NAM_ALL.MAR
COPY "testdata/mar/rms/init_nam_opt.mar"/HOST DUA1:[000000]INIT_NAM_OPT.MAR
COPY "testdata/mar/rms/init_rab.mar"/HOST DUA1:[000000]INIT_RAB.MAR
COPY "testdata/mar/rms/init_rab_all.mar"/HOST DUA1:[000000]INIT_RAB_ALL.MAR
COPY "testdata/mar/rms/init_rab_opt.mar"/HOST DUA1:[000000]INIT_RAB_OPT.MAR
COPY "testdata/mar/rms/init_xaball.mar"/HOST DUA1:[000000]INIT_XABALL.MAR
COPY "testdata/mar/rms/init_xaball_all.mar"/HOST DUA1:[000000]INIT_XABALL_ALL.MAR
COPY "testdata/mar/rms/init_xaball_opt.mar"/HOST DUA1:[000000]INIT_XABALL_OPT.MAR
COPY "testdata/mar/rms/init_xabdat.mar"/HOST DUA1:[000000]INIT_XABDAT.MAR
COPY "testdata/mar/rms/init_xabdat_all.mar"/HOST DUA1:[000000]INIT_XABDAT_ALL.MAR
COPY "testdata/mar/rms/init_xabfhc.mar"/HOST DUA1:[000000]INIT_XABFHC.MAR
COPY "testdata/mar/rms/init_xabfhc_all.mar"/HOST DUA1:[000000]INIT_XABFHC_ALL.MAR
COPY "testdata/mar/rms/init_xabitm.mar"/HOST DUA1:[000000]INIT_XABITM.MAR
COPY "testdata/mar/rms/init_xabitm_all.mar"/HOST DUA1:[000000]INIT_XABITM_ALL.MAR
COPY "testdata/mar/rms/init_xabitm_opt.mar"/HOST DUA1:[000000]INIT_XABITM_OPT.MAR
COPY "testdata/mar/rms/init_xabkey.mar"/HOST DUA1:[000000]INIT_XABKEY.MAR
COPY "testdata/mar/rms/init_xabkey_all.mar"/HOST DUA1:[000000]INIT_XABKEY_ALL.MAR
COPY "testdata/mar/rms/init_xabkey_opt.mar"/HOST DUA1:[000000]INIT_XABKEY_OPT.MAR
COPY "testdata/mar/rms/init_xabpro.mar"/HOST DUA1:[000000]INIT_XABPRO.MAR
COPY "testdata/mar/rms/init_xabpro_all.mar"/HOST DUA1:[000000]INIT_XABPRO_ALL.MAR
COPY "testdata/mar/rms/init_xabpro_opt.mar"/HOST DUA1:[000000]INIT_XABPRO_OPT.MAR
COPY "testdata/mar/rms/init_xabrdt.mar"/HOST DUA1:[000000]INIT_XABRDT.MAR
COPY "testdata/mar/rms/init_xabrdt_all.mar"/HOST DUA1:[000000]INIT_XABRDT_ALL.MAR
COPY "testdata/mar/rms/init_xabsum.mar"/HOST DUA1:[000000]INIT_XABSUM.MAR
COPY "testdata/mar/rms/init_xabsum_all.mar"/HOST DUA1:[000000]INIT_XABSUM_ALL.MAR
COPY "testdata/mar/rms/init_xabtrm.mar"/HOST DUA1:[000000]INIT_XABTRM.MAR
COPY "testdata/mar/rms/init_xabtrm_all.mar"/HOST DUA1:[000000]INIT_XABTRM_ALL.MAR
COPY "testdata/mar/rms/services.mar"/HOST DUA1:[000000]SERVICES.MAR
COPY "testdata/mar/rms/store_fab.mar"/HOST DUA1:[000000]STORE_FAB.MAR
COPY "testdata/mar/rms/store_nam.mar"/HOST DUA1:[000000]STORE_NAM.MAR
COPY "testdata/mar/rms/store_rab.mar"/HOST DUA1:[000000]STORE_RAB.MAR
COPY "testdata/mar/rms/store_xaball.mar"/HOST DUA1:[000000]STORE_XABALL.MAR
COPY "testdata/mar/rms/store_xabdat.mar"/HOST DUA1:[000000]STORE_XABDAT.MAR
COPY "testdata/mar/rms/store_xabfhc.mar"/HOST DUA1:[000000]STORE_XABFHC.MAR
COPY "testdata/mar/rms/store_xabkey.mar"/HOST DUA1:[000000]STORE_XABKEY.MAR
COPY "testdata/mar/rms/store_xabpro.mar"/HOST DUA1:[000000]STORE_XABPRO.MAR
COPY "testdata/mar/rms/store_xabrdt.mar"/HOST DUA1:[000000]STORE_XABRDT.MAR
COPY "testdata/mar/rms/store_xabsum.mar"/HOST DUA1:[000000]STORE_XABSUM.MAR
COPY "testdata/mar/rms/store_xabtrm.mar"/HOST DUA1:[000000]STORE_XABTRM.MAR
COPY "testdata/mar/rms/rms.com"/HOST DUA1:[000000]RMS.COM
COPY "testdata/mar/rms/rmserr.com"/HOST DUA1:[000000]RMSERR.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
