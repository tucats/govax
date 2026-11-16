$ ! BAT98OK.COM - Phase 98's probe: a procedure that succeeds. Written by testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98OK RAN, P1=", P1, ", P2=", P2, ", DEPTH ", F$ENVIRONMENT("DEPTH")
$ EXIT 1
