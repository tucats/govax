$ ! PROBE50Q.COM - Phase 50's interactive probe for subtask 17: INQUIRE
$ ! and READ at the terminal (testdata/dcl50/README.md). Run it at the
$ ! terminal from SYSTEM, WITHOUT /OUTPUT, with the exchange volume as the
$ ! default directory:
$ !
$ !     @PROBE50Q
$ !
$ ! and type, at each prompt, what the comment before it asks for. Keep
$ ! the whole session (prompts, replies, output) for vax/probe50q.log.
$ SET NOON
$ SET VERIFY
$ X = "there"
$ ! Q1: INQUIRE with a prompt. Type:   hello  'X'   "Mixed  Case"
$ INQUIRE ANSWER "Your answer"
$ SHOW SYMBOL ANSWER
$ SHOW SYMBOL $STATUS
$ ! Q2: INQUIRE with no prompt. Type:   yes
$ INQUIRE FLAG
$ SHOW SYMBOL FLAG
$ ! Q3: INQUIRE/NOPUNCTUATION. Type:   raw
$ INQUIRE/NOPUNCTUATION LAST "> "
$ SHOW SYMBOL LAST
$ ! Q4: INQUIRE. Type Ctrl/Z.
$ CZ = "-"
$ INQUIRE CZ "Type Ctrl/Z"
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL CZ
$ ! Q5: INQUIRE/GLOBAL. Type just Return.
$ INQUIRE/GLOBAL EMPTY "Type Return"
$ SHOW SYMBOL EMPTY
$ ! Q6: READ SYS$COMMAND, the default prompt. Type:   Some  Text 'X'
$ READ SYS$COMMAND R
$ SHOW SYMBOL R
$ SHOW SYMBOL $STATUS
$ ! Q7: READ/PROMPT. Type:   abc
$ READ/PROMPT="Say: " SYS$COMMAND R
$ SHOW SYMBOL R
$ ! Q8: READ/END_OF_FILE of SYS$COMMAND. Type Ctrl/Z.
$ READ/END_OF_FILE=Q8E SYS$COMMAND R
$ WRITE SYS$OUTPUT "Q8 Ctrl/Z didn't branch"
$ Q8E:
$ SHOW SYMBOL $STATUS
$ ! Q9: READ SYS$COMMAND with no label. Type Ctrl/Z.
$ READ SYS$COMMAND R
$ SHOW SYMBOL $STATUS
$ ! Q10: WRITE to SYS$COMMAND and SYS$ERROR.
$ WRITE SYS$COMMAND "to SYS$COMMAND"
$ SHOW SYMBOL $STATUS
$ WRITE SYS$ERROR "to SYS$ERROR"
$ ! Q11: a procedure run with /OUTPUT: where INQUIRE's prompt and
$ ! WRITE SYS$ERROR go (the terminal, the log, or both). Type:   nested
$ @PROBE50P/OUTPUT=PROBE50P.LOG
$ TYPE PROBE50P.LOG
$ EXIT
