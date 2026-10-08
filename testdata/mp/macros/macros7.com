$ ! MACROS7.COM - round 7 of the system service macro probes
$ ! (testdata/mp/macros/README.md). Written by gen.go. MACROS.COM runs it.
$ !
$ ! Each program is assembled with /NOLIST, so no listing of a macro
$ ! expansion is made, and its object is analyzed.
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST R7_LOCK
$ ANALYZE/OBJECT/OUTPUT=R7_LOCK.ANL R7_LOCK.OBJ
$ SET NOVERIFY
$ EXIT
