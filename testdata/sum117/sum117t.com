$ ! SUM117T.COM - Phase 117's probe: SUMSLP at a terminal.
$ ! Written by testdata/sum117/gen.go. Run it at a terminal, from the
$ ! exchange volume's [SUM117] directory, and keep everything the
$ ! terminal shows (README.md, "The terminal"):
$ !
$ !     $ SET DEFAULT DUA1:[SUM117]
$ !     $ @SUM117T
$ !
$ SET NOON
$ SET VERIFY
$ SET DEFAULT [.IN]
$ ! The listing and the output at the terminal.
$ EDIT/SUM/OUTPUT=TT:/LISTING=TT: S10.SRC/UPDATE=C01
$ SHOW SYMBOL $STATUS
$ EDIT/SUM/NOOUTPUT/LISTING=SYS$OUTPUT S10.SRC/UPDATE=G15A
$ ! No parameter: answer the prompt with S10.SRC/UPDATE=C01
$ EDIT/SUM/OUTPUT=TT:
$ SHOW SYMBOL $STATUS
$ ! The update typed at the terminal: type these lines, then Ctrl/Z
$ ! (and a second time without the "/", to see what Ctrl/Z alone does):
$ !     -3
$ !     typed after line 3
$ !     /
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=SYS$COMMAND
$ SHOW SYMBOL $STATUS
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=SYS$COMMAND
$ SHOW SYMBOL $STATUS
$ ! An error's messages at the terminal.
$ EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=Y08
$ SHOW SYMBOL $STATUS
$ SET DEFAULT [-]
$ SET NOVERIFY
