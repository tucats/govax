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
