$ ! PROBE50U.COM - round 5's nested cases (testdata/dcl50/README.md),
$ ! one per P1, run by PROBE50T.COM. Each case ends with EXIT (or ends
$ ! the procedure some other way, which is what it asks about).
$ GOTO 'P1'
$ !
$ ! T1: EXIT with no status after a success: is bit 28 set?
$ EXITOK:
$ X = 1
$ EXIT
$ !
$ ! T1: $STATUS at an ON ERROR THEN GOTO label.
$ ONGOTO:
$ ON ERROR THEN GOTO ONGOTO_L
$ OPEN/READ F NOSUCH.DAT
$ WRITE SYS$OUTPUT "ONGOTO: not here"
$ ONGOTO_L:
$ SHOW SYMBOL $STATUS
$ EXIT
$ !
$ ! T1: ON's command: substituted when ON is read, or when it runs?
$ ONSUBST:
$ V = "first"
$ ON WARNING THEN WRITE SYS$OUTPUT "ONSUBST: V is ''V'"
$ V = "second"
$ FROBNICATE
$ EXIT
$ !
$ ! T1: F$ENVIRONMENT("ON_SEVERITY") for each ON, and with SET NOON and
$ ! ON CONTROL_Y together.
$ ONSEV:
$ WRITE SYS$OUTPUT "ONSEV default: ", F$ENVIRONMENT("ON_SEVERITY")
$ ON WARNING THEN CONTINUE
$ WRITE SYS$OUTPUT "ONSEV warning: ", F$ENVIRONMENT("ON_SEVERITY")
$ ON SEVERE_ERROR THEN CONTINUE
$ WRITE SYS$OUTPUT "ONSEV severe: ", F$ENVIRONMENT("ON_SEVERITY")
$ SET NOON
$ ON CONTROL_Y THEN EXIT
$ WRITE SYS$OUTPUT "ONSEV noon, control_y: ", F$ENVIRONMENT("ON_SEVERITY"), " ", F$ENVIRONMENT("ON_CONTROL_Y")
$ OPEN/READ F NOSUCH.DAT
$ WRITE SYS$OUTPUT "ONSEV: still here after an error"
$ ON ERROR THEN CONTINUE
$ WRITE SYS$OUTPUT "ONSEV after ON ERROR: ", F$ENVIRONMENT("ON_SEVERITY")
$ EXIT
$ !
$ ! T1: does SKPDAT's warning take ON WARNING's action?
$ SKPDAT:
$ ON WARNING THEN GOTO SKPDAT_L
$ DIRECTORY PROBE50U.COM
skipped data
$ WRITE SYS$OUTPUT "SKPDAT: no action"
$ EXIT
$ SKPDAT_L:
$ WRITE SYS$OUTPUT "SKPDAT: action taken"
$ SHOW SYMBOL $STATUS
$ EXIT
$ !
$ ! T1: EXIT with two values.
$ EXIT34:
$ EXIT 3 4
$ WRITE SYS$OUTPUT "EXIT34: still here"
$ EXIT
$ !
$ ! T1: EXIT n's message, the procedure run with /OUTPUT.
$ EXITMSG:
$ WRITE SYS$OUTPUT "EXITMSG: in the log"
$ EXIT 44
$ !
$ ! T2: does a GOTO's forward search record the labels it passes? If
$ ! so, the GOTO back finds the first FWD_A; if not, the second.
$ FWDREC:
$ GOTO FWD_B
$ FWD_A: WRITE SYS$OUTPUT "FWDREC: first FWD_A"
$ EXIT
$ FWD_B:
$ GOTO FWD_A
$ FWD_A: WRITE SYS$OUTPUT "FWDREC: second FWD_A"
$ EXIT
$ !
$ ! T2: IF expr THEN, with nothing after THEN.
$ THENEMPTY:
$ IF 1 THEN
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "THENEMPTY: after"
$ EXIT
$ !
$ ! T2: a command other than THEN after a block's IF.
$ NOTTHEN:
$ IF 1
$ WRITE SYS$OUTPUT "NOTTHEN: the command after IF"
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "NOTTHEN: after"
$ ENDIF
$ SHOW SYMBOL $STATUS
$ EXIT
$ !
$ ! T2: THEN, ELSE, ENDIF, GOSUB, a label, and RETURN: do they change
$ ! $STATUS (IVVERB's, set first)?
$ FLOWSTATUS:
$ FROBNICATE
$ IF 1
$ THEN
$   SHOW SYMBOL $STATUS
$ ELSE
$ ENDIF
$ SHOW SYMBOL $STATUS
$ FS_LABEL:
$ SHOW SYMBOL $STATUS
$ GOSUB FS_SUB
$ SHOW SYMBOL $STATUS
$ EXIT
$ FS_SUB:
$ SHOW SYMBOL $STATUS
$ RETURN
$ !
$ ! T2: RETURN with a failing status: its message, and the ON action.
$ RETURNN:
$ ON WARNING THEN WRITE SYS$OUTPUT "RETURNN: on action"
$ GOSUB RN_SUB
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "RETURNN: after"
$ EXIT
$ RN_SUB:
$ RETURN 44
$ !
$ ! T2: a GOSUB to a label that isn't there.
$ GOSUBMISS:
$ GOSUB NOSUCH_LABEL
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "GOSUBMISS: after"
$ EXIT
$ !
$ ! T2: a CALL of a subroutine that isn't there, and of a plain label.
$ CALLMISS:
$ CALL NOSUCH_SUB
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "CALLMISS: after the missing one"
$ CALL CM_PLAIN
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "CALLMISS: after the plain label"
$ EXIT
$ CM_PLAIN:
$ WRITE SYS$OUTPUT "CALLMISS: at the plain label"
$ EXIT
$ !
$ ! T5: STOP in a subprocess's procedure, and EXIT n from one.
$ SUBSTOP:
$ WRITE SYS$OUTPUT "SUBSTOP: before"
$ STOP
$ WRITE SYS$OUTPUT "SUBSTOP: after"
$ EXIT
$ SUBEXIT:
$ WRITE SYS$OUTPUT "SUBEXIT: depth ", F$ENVIRONMENT("DEPTH")
$ EXIT 3
$ !
$ ! T2: ENDI and ENDS for ENDIF and ENDSUBROUTINE. (Last in the file:
$ ! if ENDS doesn't end the subroutine, it runs to the end of the file.)
$ ABBREV:
$ IF 0
$ THEN
$   WRITE SYS$OUTPUT "ABBREV: not here"
$ ELSE
$   WRITE SYS$OUTPUT "ABBREV: else branch"
$ ENDI
$ WRITE SYS$OUTPUT "ABBREV: after ENDI"
$ SHOW SYMBOL $STATUS
$ CALL AB_SUB
$ WRITE SYS$OUTPUT "ABBREV: after CALL"
$ SHOW SYMBOL $STATUS
$ EXIT
$ AB_SUB: SUBROUTINE
$ WRITE SYS$OUTPUT "ABBREV: in the subroutine"
$ ENDS
$ WRITE SYS$OUTPUT "ABBREV: past ENDS"
