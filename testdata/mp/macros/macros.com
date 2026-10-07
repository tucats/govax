$ ! MACROS.COM - Phase 45's system service macro probes
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS/OUTPUT=MACROS.LOG
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST R5_MISC
$ ANALYZE/OBJECT/OUTPUT=R5_MISC.ANL R5_MISC.OBJ
$ SET NOVERIFY
$ EXIT
