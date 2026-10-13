$ ! LIB57C.COM - the Phase 57 probe's procedure that LIB$DO_COMMAND runs.
$ WRITE SYS$OUTPUT "LIB57C: P1=""", P1, """ P2=""", P2, """ depth ", F$ENVIRONMENT("DEPTH")
$ EXIT
