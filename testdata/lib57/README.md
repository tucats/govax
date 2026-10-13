# The Phase 57 probe: LIBRTL's CLI routines

These fixtures are `docs/PHASE-57 - LIB CLI access.md`'s subtask 10. Most
of what the phase chose without a manual to settle it is what DCL does
around an image, so the probe is a command procedure: `lib57.com` runs a
program, `lib57.mar`, once per group of cases (a foreign command's
parameter names the group: `LIB57 SYM`), and shows DCL's state after each
run (SHOW SYMBOL, `$STATUS`, F$ENVIRONMENT, SHOW LOGICAL). With SET
VERIFY, its log interleaves the program's lines with DCL's.
`TestLib57Oracle` (`internal/console/lib57_test.go`) runs it on govax and
compares the log with VMS's (`vax/lib57.log`) section by section, S1 to
S9.

`gen.go` writes everything here but this README and the output files:

    go run testdata/lib57/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib57.mar` | The program: 181 cases in 42 groups |
| `lib57b.mar` | The image LIB$RUN_PROGRAM runs: it shows what LIB$GET_FOREIGN and LIB$GET_COMMON give it, and runs itself once more when the common area starts with `AGAIN` |
| `lib57.com` | The probe, sections S1 to S9 below |
| `lib57s.com` | Runs a group at command level 2 (S2, S5), then shows DCL's state there |
| `lib57c.com` | The procedure LIB$DO_COMMAND runs with `@` (S5) |
| `lib57p.com` | The spawned subprocess's procedure: the control keys there (S7) |
| `lib57i.com` | The interactive part's procedure, one case per P1 (below) |
| `def_libcli.mar` | `$LIBCLIDEF GLOBAL`, as `testdata/mp/defs`'s probes (VMS V7.1 has no such macro: MACRO's UNRECSTMT) |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib57.log` | `LIB57.COM`'s log on VMS |
| `vax/def_libcli.obj`, `vax/def_libcli.anl` | The `$LIBCLIDEF` probe's object and analysis |
| `vax/lib57-term.log` | The interactive session, kept by hand |

## How the program works

As Phase 55's: each case runs in a routine of its own, whose condition
handler records the condition it's given, then unwinds to the case's
caller with the condition in R0. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex (a word, such as a
resultant length, in a longword whose high half is 0; `5555` or
`55555555` is the preset value, not written), and a string's length and
text (a fixed string's whole buffer, preset to dashes). A chaining case
(LIB$DO_COMMAND, LIB$RUN_PROGRAM) that works ends the image, so it
prints nothing: what DCL does next is in the log.

The NOCLI group makes a process with `$CREPRC`, which has no CLI, running
the same image (named from the DCL symbol LIB57_DIR, which `lib57.com`
defines) with no command text, so it runs the NOCLIRUN group; its
SYS$OUTPUT is LIB57N.OUT, its SYS$INPUT LIB57N.DAT (for GET_COMMAND), its
SYS$ERROR LIB57N.ERR. The parent waits for its termination message and
prints its final status; `lib57.com` then types LIB57N.OUT.

## The sections

- **S1, symbols at command level 1:** GET_SYMBOL of a global, a local,
  lowercase, trailing and leading blanks, abbreviations (and a name
  longer than the full one), an integer, a name in both tables,
  `$STATUS`, `$SEVERITY`, P1, P9, the name rules (empty, `1X`, `_X`,
  `-X`, `$X`, `*`, `X*Y`, 255 and 256 characters), a short fixed string,
  2 and 1 arguments, a dynamic string; SET_SYMBOL with and without a
  table, tables 0 to 3, the name rules again, a blank and a dash inside
  the name, values of 255, 256, 1024, and 4097 characters, `$STATUS`,
  an abbreviation overlapping another (AMBSYMDEF), a full name replacing
  an abbreviated one, an empty value, 1 argument; DELETE_SYMBOL from
  each table and the wrong one, an abbreviation, a missing name, tables
  0 and 3, an empty name, lowercase. Then SHOW SYMBOL of each.
- **S2, symbols at command level 2** (`@LIB57S SYMOUT`): GET of a local
  of level 1, P1 and P2; SET of a local, a global, and of level 1's
  local's name; DELETE of level 1's local. SHOW SYMBOL at level 2, then
  at level 1.
- **S3, logical names:** SET_LOGICAL again (SUPERSEDE), lowercase and
  trailing blank names, the job and group tables, no such table, a
  lowercase table name, an item list of two values, a value and an item
  list, neither, each attribute and a bad one, attributes with an item
  list, an empty name, an empty value, 255- and 256-character values;
  DELETE_LOGICAL of a supervisor name, of a user-mode one, of none, an
  empty name, from the job table, lowercase. Then SHOW LOGICAL/FULL of
  the process, job, and group tables.
- **S4, the common area:** GET before any PUT, PUT and GET of 0, 252,
  and 253 characters, a 10-character fixed GET, PUT and GET with one
  argument; the next image's GET (`LIB57 COM2`), LIB57B's, and a spawned
  subprocess's.
- **S5, LIB$DO_COMMAND:** SHOW SYMBOL, `@` a procedure with parameters,
  an empty and a blank command, a failing one, WRITE, 255 and 256
  characters (with a comment), after an exit handler, a chain of two
  (`LIB57 DOSHOW`), EXIT 3 from a nested procedure, after output, an
  assignment at level 2 (`@LIB57S DOLOCAL`), under SET NOVERIFY, and an
  unknown group (the program's own line).
- **S6, LIB$RUN_PROGRAM:** LIB57B with the common area set, a chain of
  two (LIB57B runs itself), after an exit handler, a full file name, a
  missing image, an empty name, a name with text after it, 256
  characters.
- **S7, the control keys:** with SET CONTROL=(Y,T) and ON CONTROL_Y set,
  each mask (0, T, Y, both, bits that aren't T or Y) to each routine,
  old-mask shown, and F$ENVIRONMENT's CONTROL, NOCONTROL, and
  ON_CONTROL_Y after each; a spawned subprocess's own settings (SET
  NOCONTROL=Y there, and the routines), then the parent's.
- **S8, LIB$ATTACH's refusals:** this process, a PID that doesn't exist,
  and 0.
- **S9, no CLI:** every routine of the phase in a `$CREPRC` process, and
  LIB$GET_FOREIGN.

What the probe leaves out: the longword at SYS$GL_COMMON (the phase keeps
the common area in Go; linking against SYS$GL_COMMON would need the
system's symbol table), and GET_COMMAND outside a `$CREPRC` process
(it reads the terminal: the interactive part).

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/lib57/gen.go
       govax console < testdata/lib57/exchange.cmd

   This makes `testdata/disks/lib57-exchange.dsk` (RD51 size, label
   LIB57, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account:

       @LIB57/OUTPUT=LIB57.LOG

   Nothing waits. LIB57.LOG gets the log, DEF_LIBCLI.ANL the `$LIBCLIDEF`
   probe's analysis (DEF_LIBCLI.OBJ its object). MACRO, LINK, and
   ANALYZE should show nothing.
3. Then the interactive part (below), with the terminal session logged.
4. Copy LIB57.LOG back as `vax/lib57.log`, DEF_LIBCLI.OBJ and
   DEF_LIBCLI.ANL as `vax/def_libcli.obj` and `vax/def_libcli.anl`, and
   the session as `vax/lib57-term.log`.

## The interactive part

After `@LIB57/OUTPUT=LIB57.LOG` (which defines the LIB57 foreign command
and leaves the images built), at the `$` prompt, in order. Where a case
says "at the prompt", it's the prompt LIB$PAUSE or DCL gives.

| Case | Type | Then |
| ---- | ---- | ---- |
| I1 | `LIB57 PAUSE` | at the prompt: `SHOW SYMBOL $STATUS`, `CONTINUE`; then `SHOW SYMBOL $STATUS` |
| I2 | `LIB57 PAUSE` | at the prompt: `STOP`; then `SHOW SYMBOL $STATUS` |
| I3 | `LIB57 PAUSE` | at the prompt: `EXIT`; then `SHOW SYMBOL $STATUS` |
| I4 | `LIB57 PAUSE` | at the prompt: `DIRECTORY LIB57.COM` (an image), then `CONTINUE` |
| I5 | `@LIB57I PAUSE` | at the prompt: `SHOW SYMBOL X`, `WRITE SYS$OUTPUT F$ENVIRONMENT("DEPTH")`, `CONTINUE` |
| I6 | `@LIB57I PAUSE` | at the prompt: `STOP`; then `SHOW SYMBOL $STATUS` |
| I7 | `SET NOCONTROL=Y`, then `LIB57 PAUSE` | at the prompt (if one comes): `CONTINUE`; then `SET CONTROL=Y` |
| I8 | `LIB57 GETCMD` | at `Command:` type `typed line`; at the next read (no prompt: LIB$GET_INPUT) `second line`; at `Short:` `longer than five`; at the last (no prompt) `third` |
| I9 | `@LIB57I GETCMD` | the same four lines (LIB$GET_INPUT should get the procedure's data line instead, so only three are asked for) |
| I10 | `@LIB57I CTLY` | Ctrl/Y during the first 10-second wait (Ctrl/Y is disabled), then again during the second (enabled again) |
| I11 | `LIB57 DOSHORT` | if DCL asks `_What:`, type `TIME` |
| I12 | `SPAWN` | in the subprocess: `ATTACH SYSTEM`; back in the parent: `ATTACH SYSTEM_1`; in the subprocess: `LOGOUT` |
| I13 | `ATTACH NOSUCH_PROCESS`, `ATTACH SYSTEM`, `ATTACH/IDENTIFICATION=7FFFFFFF` | each followed by `SHOW SYMBOL $STATUS` |
| I14 | `SPAWN/NOWAIT WAIT 00:00:30`, then `ATTACH SYSTEM_1` (the name SPAWNED showed) | `SHOW SYMBOL $STATUS` |
| I15 | `SPAWN` | in the subprocess: `LIB57_PID = F$GETJPI("","OWNER")`, `LIB57 ATTACH`; back in the parent: `ATTACH SYSTEM_1` (the subprocess's image prints its R0); in the subprocess: `LOGOUT` |

(The process names are SYSTEM's: use the names SPAWN's messages show.)

## The run

2026-10-13, OpenVMS V7.1 on simh's VAX 8600, from the SYSTEM account.
The probe ran twice in one login; `vax/lib57.log` is the second run's,
in which two things are the first run's doing (the common area's first
text, and LIB57N.OUT written twice), which `TestLib57Oracle` takes out
(`lib57RunHistory`). `$LIBCLIDEF` doesn't exist there: DEF_LIBCLI's
object defines nothing. govax matches the log in every section, with the
job and group tables' SHOW LOGICAL/FULL headers masked. What VMS answered
is in `docs/PHASE-57`'s progress log (subtask 11) and docs/DEVIATIONS.md.
