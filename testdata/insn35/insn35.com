$ ! INSN35.COM - assembles, links, and runs the Phase 35 instruction
$ ! probes. Written by testdata/insn35/gen.go. Run it with the exchange
$ ! volume's [000000] as the default directory:
$ !
$ !     @INSN35/OUTPUT=INSN35.LOG
$ !
$ ! or, to run some of the probes, name them:
$ !
$ !     @INSN35/OUTPUT=INSN35.LOG P35G,P35H
$ !
$ ! Each probe writes its own .DMP file; running a probe again writes a
$ ! new version of it.
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ SHOW SYSTEM/NOPROCESS
$ WRITE SYS$OUTPUT F$GETSYI("HW_NAME")
$ LIST = P1
$ IF LIST .EQS. "" THEN LIST = "P35FD,P35G,P35H,P35O,P35P"
$ I = 0
$ LOOP:
$ NAME = F$ELEMENT(I,",",LIST)
$ IF NAME .EQS. "," THEN GOTO DONE
$ MACRO/NOLIST 'NAME'
$ LINK 'NAME'
$ RUN 'NAME'
$ I = I + 1
$ GOTO LOOP
$ DONE:
$ DIRECTORY/SIZE/DATE *.DMP
