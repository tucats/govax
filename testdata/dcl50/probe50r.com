$ ! PROBE50R.COM - PROBE50's procedure that runs itself one level deeper,
$ ! until DCL refuses the level past 32.
$ WRITE SYS$OUTPUT "LEVEL ", P1
$ N = P1 + 1
$ IF N .LE. 35 THEN @PROBE50R 'N'
$ SHOW SYMBOL $STATUS
