$ ! LIB57S.COM - the Phase 57 probe's nested procedure: runs LIB57's
$ ! group P1 at command level 2, then shows what it changed there.
$ LIB57 'P1'
$ SHOW SYMBOL $STATUS
$ GOTO 'P1'
$ SYMOUT:
$ SHOW SYMBOL S_INNER
$ SHOW SYMBOL S_INNERG
$ SHOW SYMBOL LOCOUT
$ SHOW SYMBOL LOCDEL
$ EXIT
$ DOLOCAL:
$ SHOW SYMBOL CHAINED_LOCAL
$ EXIT
$ DOEXIT:
$ WRITE SYS$OUTPUT "LIB57S goes on after the chained EXIT"
$ EXIT
