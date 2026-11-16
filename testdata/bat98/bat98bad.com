$ ! BAT98BAD.COM - Phase 98's probe: a procedure with a command DCL
$ ! doesn't know, under DCL's default ON action. Written by
$ ! testdata/bat98/gen.go.
$ WRITE SYS$OUTPUT "BAT98BAD RAN"
$ NOSUCHVERB98
$ WRITE SYS$OUTPUT "BAT98BAD WENT ON"
$ EXIT 1
