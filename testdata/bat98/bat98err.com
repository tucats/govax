$ ! BAT98ERR.COM - Phase 98's probe: a procedure that ends with an error (RMS-E-FNF). Written by testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98ERR RAN, P1=", P1, ", P2=", P2, ", DEPTH ", F$ENVIRONMENT("DEPTH")
$ EXIT %X00018292
