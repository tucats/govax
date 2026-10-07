$ ! DEFS.COM - the Phase 45 definition probes (testdata/mp/defs/README.md).
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @DEFS/OUTPUT=DEFS.LOG
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST DEF_ACC
$ ANALYZE/OBJECT/OUTPUT=DEF_ACC.ANL DEF_ACC.OBJ
$ MACRO/NOLIST DEF_MSG
$ ANALYZE/OBJECT/OUTPUT=DEF_MSG.ANL DEF_MSG.OBJ
$ MACRO/NOLIST DEF_PQL
$ ANALYZE/OBJECT/OUTPUT=DEF_PQL.ANL DEF_PQL.OBJ
$ MACRO/NOLIST DEF_PRC
$ ANALYZE/OBJECT/OUTPUT=DEF_PRC.ANL DEF_PRC.OBJ
$ SET NOVERIFY
$ EXIT
