$ ! MACROS8.COM - round 8 of the system service macro probes
$ ! (testdata/mp/macros/README.md). Written by gen.go. MACROS.COM runs it.
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST R8_PURGWS
$ ANALYZE/OBJECT/OUTPUT=R8_PURGWS.ANL R8_PURGWS.OBJ
$ SET NOVERIFY
$ EXIT
