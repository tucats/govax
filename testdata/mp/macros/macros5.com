$ ! MACROS5.COM - round 5 of the system service macro probes
$ ! (testdata/mp/macros/README.md). Written by gen.go. MACROS.COM runs it.
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
