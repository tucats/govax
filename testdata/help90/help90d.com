$ ! HELP90D.COM - part D of round 1 of the Phase 90 probe: HELP's
$ ! prompts answered from a procedure's data lines, and from a file.
$ ! Written by testdata/help90/gen.go; HELP90.COM runs it. If HELP prompts
$ ! at the terminal instead, answer each prompt with Ctrl/Z, and say so.
$ SET NOON
$ SET VERIFY
$ HP := HELP/PROMPT/NOUSERLIBRARY/LIBRARY=H90DIR:H90A
$ HP ALPHA
Parameters

?
Qualifiers
/BRIEF



$ SHOW SYMBOL $STATUS
$ HP
ALPHA
Description
Details





$ HP
DEEP
L2
L3
NOSUCH
?
$ ! HELP's input ended at the $ line.
$ SHOW SYMBOL $STATUS
$ HP NOSUCH
ALPHA


$ HP
@H90DIR:H90B
BONLY



$ HP ALPHA
*
...
ALPHA



$ HP NOSUB


$ HP MANY
M05


$ DEFINE/USER_MODE SYS$INPUT H90DIR:RESP.TXT
$ HP ALPHA
$ HP/OUTPUT=H90P.LIS ALPHA
Parameters



$ TYPE H90P.LIS
$ EXIT
