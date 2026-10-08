$ ! DEFS.COM - the definition probes (testdata/mp/defs/README.md): Phase
$ ! 46's, for $SECDEF, $LCKDEF, $LKIDEF, $PSLDEF, and $DCDEF. (Phase 45's,
$ ! DEF_ACC, DEF_MSG, DEF_PQL, and DEF_PRC, ran on 2026-10-07; their results
$ ! are in vax/.) Run it with the exchange volume as the default directory:
$ !
$ !     @DEFS/OUTPUT=DEFS.LOG
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST DEF_SEC
$ ANALYZE/OBJECT/OUTPUT=DEF_SEC.ANL DEF_SEC.OBJ
$ MACRO/NOLIST DEF_LCK
$ ANALYZE/OBJECT/OUTPUT=DEF_LCK.ANL DEF_LCK.OBJ
$ MACRO/NOLIST DEF_LKI
$ ANALYZE/OBJECT/OUTPUT=DEF_LKI.ANL DEF_LKI.OBJ
$ MACRO/NOLIST DEF_PSL
$ ANALYZE/OBJECT/OUTPUT=DEF_PSL.ANL DEF_PSL.OBJ
$ MACRO/NOLIST DEF_DC
$ ANALYZE/OBJECT/OUTPUT=DEF_DC.ANL DEF_DC.OBJ
$ SET NOVERIFY
$ EXIT
