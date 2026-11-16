$ ! BAT98O.COM - Phase 98's probe: a job that writes its name, P1, at
$ ! the end of BAT98$DISK:[BAT98SCR]ORDER.TXT. Written by
$ ! testdata/bat98/gen.go.
$ OPEN/APPEND ORDER BAT98$DISK:[BAT98SCR]ORDER.TXT
$ WRITE ORDER P1, " ", F$TIME()
$ CLOSE ORDER
$ EXIT
