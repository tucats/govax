$ ! LOGIN.COM - Phase 98's probe: BAT98W's login procedure. Written by
$ ! testdata/bat98/gen.go.
$ BAT98_LOGIN == "''F$ENVIRONMENT("DEPTH")' ''F$MODE()'"
$ WRITE SYS$OUTPUT "LOGIN.COM OF BAT98W AT DEPTH ", F$ENVIRONMENT("DEPTH"), ", MODE ", F$MODE()
$ EXIT
