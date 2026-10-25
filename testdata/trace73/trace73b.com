$ ! TRACE73B.COM - round 2 of Phase 73's probe of the trace trap
$ ! (testdata/trace73/README.md): what part G of round 1 left open. Run
$ ! it with the exchange volume's [000000] as the default directory:
$ !
$ !     @TRACE73B/OUTPUT=TRACE73B.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO TRACE73B
$ LINK/MAP=TRACE73B/FULL TRACE73B
$ SHOW SYMBOL $STATUS
$ LIST = "S,G,L,M"
$ IF P1 .NES. "" THEN LIST = P1
$ I = 0
$ LOOP:
$   P = F$ELEMENT(I, ",", LIST)
$   IF P .EQS. "," THEN GOTO DONE
$   DEFINE TRACE73B_PART 'P'
$   RUN TRACE73B
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO LOOP
$ DONE:
$ EXIT
