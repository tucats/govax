$ ! BAT98FAT.COM - Phase 98's probe: a procedure that ends with a severe error (SS$_ABORT). Written by testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98FAT RAN, P1=", P1, ", P2=", P2, ", DEPTH ", F$ENVIRONMENT("DEPTH")
$ EXIT %X0000002C
