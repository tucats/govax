! EXCHANGE2.CMD - builds the Phase 32 oracle's second exchange volume with
! govax. Written by testdata/mar/rms/gen.go. Run from the repository root:
!
!     govax console < testdata/mar/rms/exchange2.cmd
!
INITIALIZE/CONTAINER "testdata/disks/rms2-exchange.dsk" /DEVICE=RD53 RMSXCHG2
MOUNT/WRITE DUA1 "testdata/disks/rms2-exchange.dsk"
COPY "testdata/mar/rms/r2_dvi_def.mar"/HOST DUA1:[000000]R2_DVI_DEF.MAR
COPY "testdata/mar/rms/r2_dvi_imm.mar"/HOST DUA1:[000000]R2_DVI_IMM.MAR
COPY "testdata/mar/rms/r2_dvi_reg.mar"/HOST DUA1:[000000]R2_DVI_REG.MAR
COPY "testdata/mar/rms/r2_order_fab.mar"/HOST DUA1:[000000]R2_ORDER_FAB.MAR
COPY "testdata/mar/rms/r2_order_nam.mar"/HOST DUA1:[000000]R2_ORDER_NAM.MAR
COPY "testdata/mar/rms/r2_order_rab.mar"/HOST DUA1:[000000]R2_ORDER_RAB.MAR
COPY "testdata/mar/rms/r2_order_xaball.mar"/HOST DUA1:[000000]R2_ORDER_XABALL.MAR
COPY "testdata/mar/rms/r2_order_xabdat.mar"/HOST DUA1:[000000]R2_ORDER_XABDAT.MAR
COPY "testdata/mar/rms/r2_order_xabkey.mar"/HOST DUA1:[000000]R2_ORDER_XABKEY.MAR
COPY "testdata/mar/rms/r2_order_xabpro.mar"/HOST DUA1:[000000]R2_ORDER_XABPRO.MAR
COPY "testdata/mar/rms/r2_order_xabrdt.mar"/HOST DUA1:[000000]R2_ORDER_XABRDT.MAR
COPY "testdata/mar/rms/r2_order_xabtrm.mar"/HOST DUA1:[000000]R2_ORDER_XABTRM.MAR
COPY "testdata/mar/rms/r2_pro_all.mar"/HOST DUA1:[000000]R2_PRO_ALL.MAR
COPY "testdata/mar/rms/r2_pro_one.mar"/HOST DUA1:[000000]R2_PRO_ONE.MAR
COPY "testdata/mar/rms/r2_pro_sym.mar"/HOST DUA1:[000000]R2_PRO_SYM.MAR
COPY "testdata/mar/rms/r2_uic_init.mar"/HOST DUA1:[000000]R2_UIC_INIT.MAR
COPY "testdata/mar/rms/r2_uic_one.mar"/HOST DUA1:[000000]R2_UIC_ONE.MAR
COPY "testdata/mar/rms/rms2.com"/HOST DUA1:[000000]RMS2.COM
COPY "testdata/mar/rms/rmserr2.com"/HOST DUA1:[000000]RMSERR2.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
