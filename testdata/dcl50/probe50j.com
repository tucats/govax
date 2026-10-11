WRITE SYS$OUTPUT "I2 a data line first in a procedure: read"
$ ! PROBE50J.COM - PROBE50I's nested procedure (sections I1 and I2): its
$ ! first record is a data line. Is it read as a command?
$ EXIT
