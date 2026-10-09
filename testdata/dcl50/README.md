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
| `exchange.cmd`, `copyout.cmd` | Make the exchange volume and copy the log back |
| `vax/` | The VMS run's log, `probe50.log` (once it's run) |

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

## govax's answers (2026-10-09)

What govax does now, to set beside VMS's:

- Messages are DCL's CLI$_ texts (from `vmsdef.Messages`), each with a
  segment line, shown as `%CLI-W-...`, as govax shows its other DCL
  messages. UNDSYM, EXPSYN, IVOPER, IVFNAM, ABFNAM, ARGREQ, NOPAREN,
  IVCHAR, and MAXPARM (too many lexical arguments) are warnings, so a
  procedure goes on. Division by zero is govax's CLI_DIVZERO (an error),
  an unterminated string CLI_UNTERMSTR (an error), and a symbol whose
  value names itself CLI_SYMDEPTH (an error).
- Strings convert to integers with blanks around the number ignored,
  with a sign, and with `%X`, `%O`, or `%D`: `" 12"`, `"12 "`, and `"+5"`
  are numbers, and `"%X10"` is 16.
- `1.5` is IVOPER (a `.` that starts no operator); `.NOT.` after a
  comparison operator (`1 .EQ. .NOT. 0`) is EXPSYN.
- An apostrophe with no closing one after the name, or with no name
  after it, is kept as it is. A comment isn't scanned.
- `&` substitution keeps the value's case everywhere; in an `@`
  parameter, a value with blanks stays one parameter.
- An alias of an alias works (the second is looked up again), to 16
  levels.
- Section C: nine parameters are CLI_MAXPARM, a 33rd level govax's
  CLI_MAXDEPTH; `/OUTPUT=.LOG` is refused (CLI_BADFILESPEC: a new file
  needs a name).
- Section D needs subtask 10 (`$STATUS`, SET NOON, SET VERIFY, WRITE,
  DCL's IF); until then govax can't run the probe whole.
