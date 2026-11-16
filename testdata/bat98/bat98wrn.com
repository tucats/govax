$ ! BAT98WRN.COM - Phase 98's probe: a procedure that ends with a warning. Written by testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98WRN RAN, P1=", P1, ", P2=", P2, ", DEPTH ", F$ENVIRONMENT("DEPTH")
$ EXIT %X10000000
