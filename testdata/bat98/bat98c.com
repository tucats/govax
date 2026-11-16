$ ! BAT98C.COM - Phase 98's probe: a job that uses CPU time, 15 seconds
$ ! of it at most. Written by testdata/bat98/gen.go.
$ SET NOON
$ WRITE SYS$OUTPUT "BAT98C STARTED, CPULIM ", F$GETJPI("","CPULIM")
$ T0 = F$GETJPI("","CPUTIM")
$ I = 0
$ LOOP:
$ I = I + 1
$ IF (F$GETJPI("","CPUTIM") - T0 .LT. 1500) .AND. (I .LT. 2000000) THEN GOTO LOOP
$ WRITE SYS$OUTPUT "BAT98C DONE ", I
$ EXIT
