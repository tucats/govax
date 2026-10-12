# Phase 50's probe

What Phase 50's DCL work chose without a manual to settle it
(`docs/PHASE-50 - DCL command procedures.md`, and DEVIATIONS.md's
"[Phase 50]" entries). Round 1 covers subtasks 8 and 9: symbol
substitution and expressions. It also covers some of what comes next:
the statuses that subtask 10's `$STATUS` and `$SEVERITY` will report, and
two of the phase's open questions about `@`.

| File | What it holds |
| ---- | ------------- |
| `probe50.com` | The probe: sections A to D below |
| `probe50s.com` | Shows its P1 (section C) |
| `probe50r.com` | Runs itself one level deeper, to level 35 (section C) |
| `probe50e.com` | `EXIT 'P1'` (section D) |
| `probe50v.com` | Round 2, verification (subtask 15): sections V1 to V10 below |
| `probe50w.com` | Round 2's nested procedure (section V9) |
| `probe50i.com` | Round 3, SYS$INPUT, DECK, and EOD (subtask 16): sections I1 to I9 below |
| `probe50j.com` | Round 3's nested procedure, whose first record is a data line (I1, I2) |
| `probe50k.com` | Round 3's DECK/DOLLARS cases, one per P1 (I3, I5) |
| `probe50f.com` | Round 4, DCL's file I/O (subtask 17): sections F1 to F11 below |
| `probe50g.com` | Round 4's nested procedure, which leaves a file open (F7) |
| `probe50q.com` | Round 4's interactive part, run at the terminal: INQUIRE and READ (Q1 to Q11) |
| `probe50p.com` | Round 4's procedure run with /OUTPUT from `probe50q.com` (Q11) |
| `probe50t.com` | Round 5, what subtasks 10 to 19 chose: sections T1 to T5 below |
| `probe50u.com` | Round 5's nested cases, one per P1 |
| `probe50y.com` | Round 5's interactive part, STOP and Ctrl/Y: cases Y1 to Y16 below |
| `probe50z.com` | Round 5's interactive nested procedure, one case per P1 |
| `probe50x.mar` | Round 5's long-running image, for Ctrl/Y (Y13, Y14) |
| `exchange.cmd`, `copyout.cmd` | Make the exchange volume and copy the log back |
| `vax/` | The VMS run's log, `probe50.log` (2026-10-09) |

`probe50.com` runs with SET NOON and SET VERIFY, so every line is echoed
and no failure ends it. Each case sets X to "-", runs one command, and
shows `$STATUS` and X:

    $ X = "-"
    $ X = 7 / 0
    $ SHOW SYMBOL $STATUS
    $ SHOW SYMBOL X

## The sections

- **A, expressions:** arithmetic, radixes, overflow, division by zero, bad
  digits; string concatenation and reduction; how strings convert to
  integers (blanks, signs, `%X`, T and Y); the comparisons and logical
  operators, their precedence, and operators DCL doesn't have; malformed
  expressions; symbols and abbreviations; lexical function calls (blanks
  before the parenthesis, abbreviated names, missing and extra
  arguments, no parentheses). Each case's message, its segment line (the
  ` \TEXT\` under it), and its status.
- **B, substitution:** apostrophes outside and inside quotes, undefined
  symbols, a missing closing apostrophe, iteration, lexical functions
  between apostrophes, case after `'X'` and `&X`, `&` after a non-delimiter
  and inside quotes, comments, a symbol whose value names itself; DEFINE
  with `'X'` and `&X` (the User's Manual's 13.18.3 example) and `&X` whose
  value has a blank; a substituted verb, an alias whose value has
  apostrophes (12.13.4's EXEC example), an alias of an alias.
- **C, procedures:** `'NAME'`, `"''NAME'"`, and `&NAME` as a parameter;
  nine parameters (CLI$_DEFOVF or CLI$_MAXPARM?); `/OUTPUT=.LOG` with no
  name (where the file goes); nesting past 32 levels (CLI$_STKOVF?).
- **D, statuses:** `$STATUS` and `$SEVERITY` after an unknown verb, SHOW
  SYMBOL and DELETE/SYMBOL of an undefined symbol, TYPE and `@` of a file
  that isn't there, and `EXIT n` from a procedure for several n.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/dcl50/exchange.cmd

   This makes `testdata/disks/dcl50.dsk` (RD53 size, label DCL50,
   gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account:

       @PROBE50/OUTPUT=PROBE50.LOG

   It takes a few seconds; nothing waits.
3. Dismount, copy the container back, and run:

       govax console < testdata/dcl50/copyout.cmd

   The log goes to `vax/probe50.log`.

Round 2 is run the same way, with `@PROBE50V/OUTPUT=PROBE50V.LOG`, and
round 3 with `@PROBE50I/OUTPUT=PROBE50I.LOG`. Round 4 has two parts (see
"Round 4" below); `copyout.cmd` now copies round 4's logs, to
`vax/probe50f.log` and `vax/probe50p.log` (the author kept the
interactive session as `vax/probe50q.log`).

## Round 2: verification

`probe50v.com` runs with SET NOON and SET VERIFY. Each section starts by
showing a marker symbol M, so the log can be split even where
verification is off:

- **V1:** a continued command with and without apostrophe substitution
  (shown record by record, or joined?).
- **V2:** indentation, labels, a label alone, a blank `$` line, comments.
- **V3:** IF blocks: which lines of the branch that doesn't run are
  shown (its comments, its ELSE and ENDIF).
- **V4:** GOTO, GOSUB/RETURN, CALL/SUBROUTINE/ENDSUBROUTINE: the lines
  passed over and the lines that end each.
- **V5:** F$VERIFY in comments (alone, abbreviated, without its closing
  apostrophe, after another apostrophe pair, in a command's comment) and
  in commands.
- **V6:** image verification of the data lines TYPE SYS$INPUT reads, and
  of data lines nothing reads.
- **V7:** SET PREFIX: the time (`!5%T`), directives that take arguments,
  a malformed control string, a continued command, a data line,
  F$ENVIRONMENT("VERIFY_PREFIX"), SET NOPREFIX.
- **V8:** whether ON's action and messages are shown.
- **V9:** SET VERIFY's keyword forms, and whether a nested procedure's
  SET NOVERIFY (or `'F$VERIFY(0)'`) lasts after it returns
  (`probe50w.com`).
- **V10:** F$VERIFY's arguments and values, and keyword and verb
  abbreviations (`PROCE`, `IMA`, `SET NOVERI=IMAGE`, `SET VERI`).

## What VMS answered (2026-10-09)

`TestProbe50Oracle` (`internal/console/probe50_test.go`) replays sections
A and B under govax and compares each case's messages and value with the
log; every case matches. The answers that changed govax:

- **Messages:** DCL's, shown as `%DCL-`. A segment line follows UNDSYM,
  IVCHAR, IVOPER, IVFNAM, ABFNAM, IVVERB, and EXPSYN for a symbol that
  names itself (`\'SELF\`); not EXPSYN otherwise, SYMDEL, ARGREQ, NOCOMD,
  DEFOVF, STKOVF, or UNDSYM from SHOW SYMBOL and DELETE/SYMBOL. All are
  warnings. IVFNAM's and ABFNAM's segment is the name and its `(`.
- **Expressions:** `7 / 0` is 2147483647, with no message. A quoted string
  with no closing quote runs to the end of the line. `""quoted""` is the
  string QUOTED: a string token goes on through quotes and letters. An
  operator's closing dot may be left out (`1 .EQ 1`), but not shortened
  (`.E.` is IVOPER). `.NOT.` may follow another operator (`1 .EQ. .NOT. 0`
  is 0). `1 + 2)` assigns 3 and then gives SYMDEL. `F$LENGTH` without
  parentheses is an undefined symbol; `F$L(` is ambiguous (VMS's full set
  of names counts); `F$LENGTH(,)` is ARGREQ and `F$LENGTH("a","b")`
  SYMDEL. Strings convert to numbers with blanks, signs, and `%X`
  prefixes, as govax already did.
- **Substitution:** the closing apostrophe may be left out (`'NAME`), and
  blanks may follow the opening one (`' NAME'`). A symbol whose value
  names itself is EXPSYN. `&` isn't substituted in a `:=` assignment or
  an `@` parameter; in DEFINE, `&TWO` ("a b") is one value, its case kept.
  An alias's value isn't looked up as a symbol again (an alias of an
  alias is IVVERB), and one that starts with an apostrophe is NOCOMD.
- **`@`:** a ninth parameter is DEFOVF; the @ that would make the 32nd
  level, the terminal's among them, is STKOVF; each level goes on after
  either (both warnings). `/OUTPUT=.LOG` makes a file named `.LOG` in the
  default directory, which govax still refuses (open).
- **Statuses** (section D, and `$STATUS` throughout): subtask 10 follows
  them; `TestProbe50Oracle` compares each case's `$STATUS` too, and
  `TestProbe50Statuses` replays section D.
  `$STATUS` is a string, `"%X00030001"` after success; IVVERB's is
  `%X00038090`; a status a message was already shown for has bit 28 set
  (`%X10951238` after TYPE's failure); EXIT's status is shown as a
  message unless it's odd (success or informational) or has bit 28 set,
  and `EXIT` with no status keeps the last one.

Also seen: SET VERIFY shows a command after apostrophe substitution
(for subtask 15), and before `@PROBE50`'s SET VERIFY nothing is echoed.

Still open, for a later round: the quotient of a negative number divided
by zero; whether `''NAME` inside quotes needs its closing apostrophe; a
foreign command's `&`; and `/OUTPUT=` with no name.

## What VMS answered in round 2 (2026-10-11)

`TestProbe50VOracle` (`internal/console/probe50_test.go`) runs
`probe50v.com` under govax and compares its output with
`vax/probe50v.log` section by section; every section matches (V6, and
V7's one TYPE SYS$INPUT, since subtask 16). The answers
(DEVIATIONS.md's "[Phase 50] Verification"):

- **V1:** a continued command is shown a record at a time, each record
  with its own substitutions (`$ A := MYFILE -`, `  .DAT`).
- **V2:** indentation, labels, a label alone, a blank `$`, and comments
  are shown as they are.
- **V3:** the THEN of a false block IF isn't shown; its ELSE and ENDIF
  are, and nothing of the branch that doesn't run.
- **V4:** a CALL shows `$ V4S: SUBROUTINE`, the body, and
  `$ ENDSUBROUTINE`; GOSUB shows its label line and RETURN.
- **V5:** `'F$VERIFY(n)'` in a comment, and in a command's comment, is
  performed and shown replaced by its value (`$ ! 0 - this one should
  be`); without its closing apostrophe it is still performed.
- **V6:** with image verification, each data line TYPE SYS$INPUT reads
  is shown, then typed; a data line after SET VERIFY was shown and read
  as a command: `%DCL-W-IVVERB` with `\DATA\`.
- **V7:** `"[!UL|!AS|!XL] "` and `"!%D !"` are `%DCL-E-INVPREFIX`, and the
  prefix stays; data lines get no prefix.
- **V8:** ON's action isn't shown.
- **V9:** SET NOVERIFY with keywords is `%DCL-W-CONFLICT` with
  `\NOVERIFY\`; a nested procedure's SET NOVERIFY or `'F$VERIFY(0)'`
  lasts after it returns.
- **V10:** F$VERIFY(1,0) turns image verification off, F$VERIFY(,1)
  changes nothing, F$VERIFY("YES") is on; PROCE and IMA abbreviate
  SET VERIFY's keywords.

## Round 3: SYS$INPUT, DECK, and EOD

`probe50i.com` checks what subtask 16 chose without VMS (DEVIATIONS.md's
"[Phase 50] SYS$INPUT from a procedure"). It runs with SET NOON and SET
VERIFY, and each section starts by showing a marker symbol M. Most data
lines are WRITE commands without a `$`: if DCL reads one as a command,
its text is written; if DCL skips it, `%DCL-W-SKPDAT` is.

- **I1:** a data line after each kind of command: assignments, SHOW
  SYMBOL (and a failing one), SHOW DEFAULT, SHOW TRANSLATION, SHOW
  LOGICAL, SET DEFAULT, SET PROMPT, SET [NO]ON, SET VERIFY, SET SYMBOL,
  DEFINE, ASSIGN, DEASSIGN, IF (false, and true with SHOW SYMBOL and
  with DIRECTORY), CONTINUE, WRITE, WAIT, DIRECTORY, a failing TYPE, an
  unknown verb, GOTO and a label, GOSUB's RETURN, CALL, and @. Which are
  DCL's own commands, whose data is read as a command, and which images?
- **I2:** a data line as a procedure's first record (`probe50j.com`).
- **I3:** DECK and EOD as commands with nothing reading: is a deck's
  data skipped, `$` lines too? Their effect on `$STATUS`. DECK/DOLLARS
  as a command (`probe50k.com COMMAND`).
- **I4:** TYPE SYS$INPUT through a DECK and EOD (are they shown?), the
  data after an EOD, and `$EOD` with no blank.
- **I5:** DECK/DOLLARS's string unquoted and quoted (its case), a record
  that only starts with it, `/DOLLARS` with no string, and the string
  after blanks (`probe50k.com`, one case per P1, so a deck that never
  ends stops at that file's end).
- **I6:** READ SYS$INPUT (for subtask 17): the record after the READ, a
  READ at a `$` line and at an EOD, and the record after the one READ
  took.
- **I7:** TYPE of a logical name that translates to SYS$INPUT, directly
  and through another.
- **I8:** which records end the data: `$` after blanks or a tab, `$`
  alone, and an empty record.
- **I9:** SET VERIFY=(PROCEDURE,NOIMAGE) with a deck, and a data line
  read as a command with procedure verification off.

## What VMS answered in round 3 (2026-10-11)

`TestProbe50IOracle` (`internal/console/probe50_test.go`) runs
`probe50i.com` under govax and compares its output with
`vax/probe50i.log`: sections I2, I3, I4, I5, I7, and I8 line for line
and in I1 whether each data line was read as a command or skipped.
Until subtask 17 gave govax WRITE and READ, WRITE was emulated from
govax's IVVERB for it, and I6 (READ) and I9 (a WRITE not echoed) waited;
since subtask 17 every section but I1 is compared line for line. The
answers (DEVIATIONS.md's "[Phase 50] SYS$INPUT from a procedure"):

- **I1, I2:** after SHOW LOGICAL, DIRECTORY, and a failing TYPE (images)
  the data was skipped with SKPDAT; after DCL's own commands, an unknown
  verb, GOTO, GOSUB's RETURN, CALL, and @, and as a procedure's first
  record, it was read as a command.
- **I3:** DECK read by DCL is `%DCL-W-IVVALU` (with /DOLLARS too) and
  starts no deck; EOD leaves `$STATUS` alone; `%%` read as a command is
  IVVERB `\%%\`. WRITE leaves `$STATUS` at `%X00010001`.
- **I4:** DECK and EOD records aren't shown; data after an EOD is skipped;
  `$EOD` works without a blank.
- **I5:** /DOLLARS's string is uppercased unless quoted, and matched
  case and all at the start of a record; bare /DOLLARS means EOD.
- **I6:** READ SYS$INPUT takes the next record whatever it is, `$` lines
  and EOD included.
- **I7:** TYPE through a logical name for SYS$INPUT, and through two,
  types the data.
- **I8:** `$` after blanks or a tab is data, an empty record is data, and
  a lone `$` ends the data and the skipping: the data after it was read
  as a command.
- **I9:** with image verification off the data isn't shown; a data line
  read as a command with procedure verification off isn't shown either.

## Round 4: DCL's file I/O

`probe50f.com` checks what subtask 17 chose for OPEN, READ, WRITE, and
CLOSE without VMS (DEVIATIONS.md's "[Phase 50] DCL's file I/O"). It runs
with SET NOON and SET VERIFY, makes files named `PROBE50F*.DAT` (deleting
any left from an earlier run), and reads nothing from the terminal. Each
section starts by showing a marker symbol M, and most commands are
followed by `SHOW SYMBOL $STATUS`.

- **F1:** OPEN/WRITE: `$STATUS` after OPEN, WRITE, and CLOSE; the logical
  name (SHOW LOGICAL/FULL: its mode, table, and value); WRITE's items
  (integers, an empty string, quotes); the file's attributes
  (DIRECTORY/FULL) and its records with their fixed control (DUMP/RECORD);
  the next OPEN/WRITE's version.
- **F2:** OPEN/READ and READ: each record as read, a READ into a name
  that is a global symbol (is the local one new?), the end of the file
  with /END_OF_FILE, with no label, and with /ERROR (the message, the
  status, and whether the symbol changes); TYPE of the logical name while
  the file is open.
- **F3:** OPEN's failures: a missing file, /ERROR (the status at the
  label), a missing directory, /APPEND and /READ/WRITE of a missing file
  (OPENIN or OPENOUT?), a wildcard, /WRITE with /APPEND, no file, a bad
  /SHARE keyword; OPEN of a logical name that already has a file (the
  same file, then another: which does READ read?), and CLOSE twice.
- **F4:** READ, WRITE, and CLOSE of a name never opened (CLI$_UNDFIL?),
  CLOSE/NOLOG and CLOSE/ERROR of one, WRITE to a file opened to read (and
  /ERROR), READ of a file opened to write, a logical name DCL didn't
  define, a bad symbol name, WRITE with no data.
- **F5:** WRITE/UPDATE of a file opened /WRITE; /APPEND; /READ/WRITE:
  WRITE/UPDATE before any READ, of the same length and of another, WRITE
  before the end of the file and at it; /READ/APPEND (where does READ
  start?).
- **F6:** WRITE to SYS$OUTPUT and SYS$ERROR (does SYS$ERROR's go to the
  /OUTPUT log?), an integer expression, a trailing comma, two items with
  no comma, /SYMBOL with symbols and with a quoted string, a logical name
  for SYS$OUTPUT, `SYS$OUTPUT:`, and SYS$INPUT.
- **F7:** a file a nested procedure (`probe50g.com`) leaves open, used
  after it ends; READ SYS$INPUT at the nested procedure's end of file.
- **F8:** a data line after OPEN, READ, and CLOSE: read as a command, or
  SKPDAT?
- **F9:** sharing: OPEN of a file DCL is writing, without /SHARE and with
  it; what a reader sees of records the writer hasn't closed; TYPE of
  such a file; two /SHARE=READ readers and an /APPEND.
- **F10:** long records: WRITE and WRITE/SYMBOL of 600 characters, and
  READ of them back.
- **F11:** the logical name OPEN defined, deassigned while the file is
  open (user mode, then supervisor mode): can READ and CLOSE still reach
  the file?

`probe50q.com` is interactive, for INQUIRE and READ at the terminal,
whose prompts don't go to an /OUTPUT log. Run it at the terminal, with no
/OUTPUT:

    @PROBE50Q

and at each prompt type what the comment before it asks for (the
verification shows the comment). Keep the whole terminal session, prompts
and replies included, as `vax/probe50q.log` (simh's `SET CONSOLE
LOG=file`, or a copy of the screen):

- **Q1 to Q5:** INQUIRE with a prompt (the reply's case, blanks, quotes,
  and `'X'`), with none (the symbol's name as the prompt?),
  /NOPUNCTUATION, Ctrl/Z (the symbol and `$STATUS`), and an empty reply
  with /GLOBAL.
- **Q6 to Q9:** READ SYS$COMMAND with the default prompt (exactly what
  is shown: `DATA:`? a blank after it?), with /PROMPT, and Ctrl/Z with
  /END_OF_FILE and with no label.
- **Q10:** WRITE to SYS$COMMAND and SYS$ERROR.
- **Q11:** `probe50p.com`, run with /OUTPUT=PROBE50P.LOG: does INQUIRE's
  prompt go to the terminal or the log, and WRITE SYS$ERROR and
  SYS$COMMAND? Its log is typed after, and `copyout.cmd` copies it to
  `vax/probe50p.log`.

Then, at the `$` prompt itself (command level 0), type

    READ SYS$INPUT X
    SHOW SYMBOL X

replying `hello` to READ's prompt, and keep that too: READ SYS$INPUT at
the terminal.

## What VMS answered in round 4 (2026-10-11)

`TestProbe50FOracle` (`internal/console/probe50_test.go`) runs
`probe50f.com` under govax, with /OUTPUT as on VMS, on DUA1 of a node
called SIMVAX, and compares its log with `vax/probe50f.log` section by
section; every section matches, but for DELETE's, DIRECTORY's, and
DUMP's output (their own layouts) and the extra blank SHOW LOGICAL puts
before a process-permanent file's name. `TestProbe50QOracle` runs
`probe50q.com` with what the author typed and compares each symbol,
`$STATUS`, and message with `vax/probe50q.log`, and `PROBE50P.LOG` with
`vax/probe50p.log`; all match. The answers (DEVIATIONS.md's "[Phase 50]
DCL's file I/O"):

- **F1:** OPEN leaves %X00000001; WRITE and CLOSE RMS$_NORMAL. The
  logical name is `[super,confine] = "_SIMVAX$DUA1" [terminal]`. The new
  file is VFC, 2 bytes of control, print-file carriage control.
- **F2:** TYPE of the logical name typed the file and used up its
  records: every READ after it was at the end of the file. READ at the
  end shows `%RMS-E-EOF`, `$STATUS` %X0001827A (no bit 28), the symbol
  unchanged, no local symbol made; /END_OF_FILE and /ERROR branch with
  the same status.
- **F3:** OPEN's failures are OPENIN (OPENOUT only for a new file), with
  `-SYSTEM-W-NOSUCHFILE` under RMS$_DNF, and bit 28 in `$STATUS` even at
  the /ERROR label. OPEN/WRITE/APPEND is CONFLICT `\WRITE\`. A name
  already open is passed over with CLI$_NORMAL, and READ reads the first
  file.
- **F4:** UNDFIL for a name never opened; CLOSE/NOLOG of one is silent
  and CLI$_NORMAL. WRITE to a file opened to read is RMS$_FAC (no bit
  28); READ of a file opened /WRITE is RMS$_EOF.
- **F5:** WRITE/UPDATE with no record read is RMS$_CUR (on a /WRITE file
  too), of another length RMS$_RSZ; WRITE before the end of a
  /READ/WRITE file RMS$_NEF; /READ/APPEND reads from the end.
- **F6:** WRITE SYS$ERROR doesn't go to the /OUTPUT log; `SYS$OUTPUT:` is
  UNDFIL; WRITE SYS$INPUT RMS$_FAC; a trailing comma, or items without
  one, EXPSYN.
- **F7:** a file a procedure leaves open stays open; after an
  /END_OF_FILE branch the procedure's EXIT gave %X10030001.
- **F8:** data lines after OPEN, READ, and CLOSE are read as commands.
- **F9:** a file OPEN/WRITE has open keeps out a reader, a /SHARE reader,
  and TYPE (RMS$_FLK); two /SHARE=READ readers share, and keep /APPEND
  out.
- **F10:** a WRITE item over 255 characters is TKNOVF, its segment line
  cut at 255; /SYMBOL and READ take 600; READ leaves CLI$_NORMAL.
- **F11:** DEASSIGN of the name (supervisor mode by default) leaves
  %X00000001, and READ and CLOSE are then UNDFIL.
- **Q1 to Q11:** INQUIRE leaves %X00000001, Ctrl/Z gives ""; with
  verification, the substituted reply is shown on the procedure's output
  (the /OUTPUT log). READ's prompt is "Data:" and a tab; Ctrl/Z is
  RMS$_EOF. WRITE SYS$COMMAND is RMS$_NORMAL; SYS$ERROR and SYS$COMMAND,
  and INQUIRE's prompt, go to the terminal under /OUTPUT.

Still open: the fixed control bytes DCL writes (DUMP/BLOCK would show
them; govax writes 01 8D), and why SHOW LOGICAL indents a
process-permanent name one blank further.

## Round 5: statuses, flow, lexicals, subprocesses, STOP, and Ctrl/Y

Round 5 asks about what subtasks 10 to 19 chose without VMS (DEVIATIONS.md's
"[Phase 50]" entries: statuses and ON; labels, IF blocks, GOSUB, and CALL;
the lexical functions; STOP and Ctrl/Y; the subprocess's DCL). It has two
parts. Build the exchange volume as before (`exchange.cmd` now copies
round 5's files), and run from SYSTEM with the volume's `[000000]` as the
default directory.

### Part 1: `@PROBE50T/OUTPUT=PROBE50T.LOG`

Nothing waits. It runs with SET NOON and SET VERIFY; each section starts
by showing a marker symbol M, most cases are followed by `SHOW SYMBOL
$STATUS`, and each lexical case sets X to "-" first.

- **T1, statuses and ON:** EXIT with no status after a success (bit
  28?); `$STATUS` at an ON ERROR THEN GOTO label; whether ON's command is
  substituted when ON is read or when it runs; F$ENVIRONMENT
  ("ON_SEVERITY") after each ON, with SET NOON and ON CONTROL_Y; whether
  SKPDAT takes ON WARNING's action; `ON FOO`; `EXIT 3 4`; `EXIT 44`'s
  message from a procedure run with /OUTPUT (its log is PROBE50N.LOG);
  DIRECTORY's and TYPE's success status; `-7 / 0`; `''X` in quotes with
  no closing apostrophe.
- **T2, flow:** whether a GOTO's forward search records the labels it
  passes (FWDREC); `IF 1 THEN` with nothing after; a command other than
  THEN after a block's IF, and the ENDIF after it; whether THEN, ELSE,
  ENDIF, a label, GOSUB, and RETURN change `$STATUS`; `RETURN 44`'s
  message and the ON action; a GOSUB to a missing label (does the
  procedure go on?); CALL of a missing subroutine and of a plain label;
  ENDI and ENDS as ENDIF and ENDSUBROUTINE.
- **T3, lexical functions:** keyword abbreviations (F$EDIT's COMP,
  F$ENVIRONMENT's DEP, F$GETSYI's NODEN, F$GETJPI's PRCN, F$PARSE's NAM,
  F$CVTIME's ABS); F$EDIT's order and quoted text; the range errors of
  F$EXTRACT, F$ELEMENT, F$CVSI, F$CVUI; F$ELEMENT past the end; F$LOCATE
  of ""; F$TYPE of " 12 ", "%X1F", "+5", "1 2", and of a string that isn't
  a name; F$FAO with a string for !UL and an integer for !AS;
  F$ENVIRONMENT's CONTROL, NOCONTROL, ON_SEVERITY, PROTECTION; F$GETJPI's
  USERNAME, UIC, MODE, JOBTYPE, STATE, CURPRIV, and F$USER; F$SEARCH of a
  name without wildcards three times, and without its type; F$PARSE of a
  missing device and directory, SYNTAX_ONLY, a default's version;
  F$MESSAGE of CREATE's OPENOUT and of 2; F$CVTIME's formats, fields, a
  delta, combinations, and DCL's `date:time` form.
- **T5, subprocesses:** SPAWN of a command (F$ENVIRONMENT("DEPTH") in the
  subprocess), of a procedure that STOPs, of one that EXITs 3, and of an
  unknown verb: what each prints, and `$STATUS` after each SPAWN.

### Part 2: `PROBE50Y`, at the terminal

Run each case at the terminal, without /OUTPUT, and keep the whole
session, prompts and keys included, as `vax/probe50y.log`. After each
case (when the `$` prompt is back) type

    SHOW SYMBOL $STATUS
    WRITE SYS$OUTPUT F$ENVIRONMENT("DEPTH")

The first case builds PROBE50X.EXE (MACRO and LINK). When a case says
"Press Ctrl/Y now", press it within the 20 seconds of the WAIT; where a
step says to type commands, they are typed at the prompt Ctrl/Y gives.

| Case | Type | Then |
| ---- | ---- | ---- |
| Y1 | `@PROBE50Y STOP` | (nothing waits: STOP in a nested procedure) |
| Y2 | `@PROBE50Y ONSTOP` | (nothing waits: ON ERROR THEN STOP in a nested one) |
| Y3 | `@PROBE50Y WAIT` | Ctrl/Y, then `SHOW SYMBOL X`, `WRITE SYS$OUTPUT F$ENVIRONMENT("DEPTH")`, `X = "changed"`, `SHOW SYMBOL $STATUS`, `CONTINUE` |
| Y4 | `@PROBE50Y WAIT` | Ctrl/Y, then `EXIT` |
| Y5 | `@PROBE50Y WAIT` | Ctrl/Y, then `STOP` |
| Y6 | `@PROBE50Y WAIT` | Ctrl/Y, then `DIRECTORY PROBE50Y.COM` (an image), then `CONTINUE` |
| Y7 | `@PROBE50Y WAIT` | Ctrl/Y, then Ctrl/Y again at the prompt, then `CONTINUE` |
| Y8 | `@PROBE50Y ONGOTO` | Ctrl/Y, and Ctrl/Y again when asked |
| Y9 | `@PROBE50Y ONCONT` | Ctrl/Y |
| Y10 | `@PROBE50Y NESTED` | Ctrl/Y during the nested procedure's wait (if a prompt comes, `CONTINUE`) |
| Y11 | `@PROBE50Y NESTED0` | the same |
| Y12 | `@PROBE50Y NOCONTROL` | Ctrl/Y during the 10-second wait (it should do nothing) |
| Y13 | `@PROBE50Y IMAGE` | Ctrl/Y while PROBE50X runs, then `SHOW SYMBOL $STATUS`, `CONTINUE`, and wait for "spun" |
| Y14 | `@PROBE50Y IMAGEON` | Ctrl/Y while PROBE50X runs |
| Y15 | `@PROBE50Y WAIT` | Ctrl/C instead of Ctrl/Y, then `CONTINUE` |
| Y16 | at the `$` prompt: `L: SHOW SYMBOL $STATUS`, `GOTO L`, `GOSUB L`, `RETURN`, `CALL L`, `SUBROUTINE`, `ENDSUBROUTINE`, each followed by `SHOW SYMBOL $STATUS` | labels and flow commands at the terminal |

(If PROBE50X ends before you can press Ctrl/Y, note it: its count is in
`probe50x.mar`.) Then run `copyout.cmd`, which copies PROBE50T.LOG to
`vax/probe50t.log` and PROBE50N.LOG to `vax/probe50n.log`.

## What VMS answered in round 5 (2026-10-11)

`TestProbe50TOracle` (`internal/console/probe50t_test.go`) runs
`probe50t.com` on a booted machine, with /OUTPUT, on DUA1 of a node
called SIMVAX, with Ctrl/T enabled as SYSTEM's login had it, and
compares its log with `vax/probe50t.log` section by section; every
section matches, but for DIRECTORY's layout and two F$FAO cases where
VMS's answer was garbage (a number from an address, an access
violation). The interactive part, `vax/probe50y.log`, was run by hand;
its answers are in govax's tests (`dclcontroly_test.go`,
`controlyimage_test.go`). The OPCOM message in it is the VAX's own. The
answers that changed govax (DEVIATIONS.md's "[Phase 50] Round 5 of the
probe"):

- **T1:** EXIT with no status sets bit 28 after a success too
  (%X10030001); ON's command is substituted when ON is read; SKPDAT
  takes ON WARNING's action; `ON FOO` is `%DCL-W-ONERR` with ` \ FOO\`;
  `EXIT 3 4` is EXPSYN and the procedure goes on; EXIT 44's message goes
  to the caller's output; DIRECTORY ends with %X00000001 and TYPE with
  %X10000001; `-7 / 0` is -2147483648; `''X` in quotes needs no closing
  apostrophe.
- **T2:** a forward search records the labels it passes; `IF 1 THEN`
  alone does nothing; after a block's IF, commands are passed over
  silently until THEN, and an ENDIF before it is `%DCL-E-INVIFNEST`
  (an error); a search that passes such a block shows INVIFNEST and goes
  on; THEN, ELSE, ENDIF, labels, and GOSUB leave `$STATUS`, and RETURN
  sets %X10000001; `RETURN 44` shows its message and takes the ON
  action; a missing GOSUB or CALL target shows USGOSUB or USCALL and
  ends the procedure with USGOTO's status (%X10038148); ENDI and ENDS
  work.
- **T3:** keywords aren't abbreviated (COMP, DEP, NODEN, PRCN, NAM,
  ABS); F$EXTRACT with a negative offset is "", with a negative length
  the rest; F$ELEMENT's long delimiter is `%DCL-E-STRTOOLNG`; F$TYPE of a
  string is ARGREQ; JOBTYPE is 3; the privilege is ACNT; F$SEARCH
  without wildcards finds the file every time, as `DUA1:[000000]...`;
  F$MESSAGE knows CREATE's OPENOUT; F$CVTIME has no DAYOFYEAR, a delta
  `1-02:03:04.5` is `1 02:03:04.5`, and a bad one is IVDTIME with the
  time as its segment.
- **T5:** a spawned subprocess has its parent's SET VERIFY; SPAWN's
  `$STATUS` is the subprocess's final status, and a failure's message is
  shown again (`%DCL-W-IVVERB`); STOP in a spawned procedure ends the
  process with `$STATUS` as it was.
- **Y1 to Y16:** STOP leaves `$STATUS`; Ctrl/Y's prompt reads commands
  in the procedure's context (its symbols, DEPTH 1); CONTINUE resumes
  (an image too); an image command at the prompt ends the procedure
  (CONTINUE after it does nothing); Ctrl/Y at the prompt gives a new
  one; ON CONTROL_Y stays in effect; with ON CONTROL_Y only in the
  caller, the nested procedure ends with `$STATUS` marked as shown and
  the caller goes on (its action not taken); with none anywhere, the
  prompt comes at any depth (Y11); SET NOCONTROL=Y cancels ON CONTROL_Y,
  and Ctrl/Y only echoes; Ctrl/C acts as Ctrl/Y; at the terminal a label
  is NOLBLS (`\L:\`), GOTO is silent (%X10038148), GOSUB INVGOSUB,
  RETURN silent, CALL `%DCL-E-INVCALL`.

Still open: what WAIT leaves in `$STATUS` when it runs to its end;
RETURN's status at the terminal; ELSE before a block's THEN; and a CALL
of a plain label.
