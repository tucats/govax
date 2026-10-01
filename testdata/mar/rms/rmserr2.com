$ ! RMSERR2.COM - the Phase 32 oracle's the second round's probes that may be
$ ! errors. Their messages go to ERRORS2.LOG, which the author audits
$ ! for macro text before Claude reads it.
$ ! Written by testdata/mar/rms/gen.go. Run it with the exchange volume as
$ ! the default directory:
$ !
$ !     @RMSERR2/OUTPUT=ERRORS2.LOG
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST R2_DVI_DEF
$ ANALYZE/OBJECT/OUTPUT=R2_DVI_DEF.ANL R2_DVI_DEF.OBJ
$ MACRO/NOLIST R2_DVI_IMM
$ ANALYZE/OBJECT/OUTPUT=R2_DVI_IMM.ANL R2_DVI_IMM.OBJ
$ MACRO/NOLIST R2_DVI_REG
$ ANALYZE/OBJECT/OUTPUT=R2_DVI_REG.ANL R2_DVI_REG.OBJ
$ MACRO/NOLIST R2_PRO_ALL
$ ANALYZE/OBJECT/OUTPUT=R2_PRO_ALL.ANL R2_PRO_ALL.OBJ
$ MACRO/NOLIST R2_PRO_ONE
$ ANALYZE/OBJECT/OUTPUT=R2_PRO_ONE.ANL R2_PRO_ONE.OBJ
$ MACRO/NOLIST R2_PRO_SYM
$ ANALYZE/OBJECT/OUTPUT=R2_PRO_SYM.ANL R2_PRO_SYM.OBJ
$ MACRO/NOLIST R2_UIC_INIT
$ ANALYZE/OBJECT/OUTPUT=R2_UIC_INIT.ANL R2_UIC_INIT.OBJ
$ MACRO/NOLIST R2_UIC_ONE
$ ANALYZE/OBJECT/OUTPUT=R2_UIC_ONE.ANL R2_UIC_ONE.OBJ
$ DIRECTORY/SIZE=ALL *.OBJ
