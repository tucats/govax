$ ! PROBE50G.COM - PROBE50F's nested procedure (section F7): it leaves a
$ ! file open as it ends, and its last READ of SYS$INPUT is at its end of
$ ! file.
$ OPEN/WRITE KEEP PROBE50FD
$ WRITE KEEP "from PROBE50G"
$ L = "-"
$ GOTO G_READ
$ G_EOF:
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL L
$ EXIT
$ G_READ:
$ READ/END_OF_FILE=G_EOF SYS$INPUT L
