$ ! DEFS.COM - the definition probes (testdata/mp/defs/README.md): Phase
$ ! 48's, for $CLIDEF and $LIBDEF. (Phase 45's, DEF_ACC, DEF_MSG, DEF_PQL,
$ ! and DEF_PRC, ran on 2026-10-07, and Phase 46's, DEF_SEC, DEF_LCK,
$ ! DEF_LKI, DEF_PSL, and DEF_DC, on 2026-10-08; their results are in
$ ! vax/.) Run it with the exchange volume as the default directory:
$ !
$ !     @DEFS/OUTPUT=DEFS.LOG
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST DEF_CLI
$ ANALYZE/OBJECT/OUTPUT=DEF_CLI.ANL DEF_CLI.OBJ
$ MACRO/NOLIST DEF_LIB
$ ANALYZE/OBJECT/OUTPUT=DEF_LIB.ANL DEF_LIB.OBJ
$ SET NOVERIFY
$ EXIT
