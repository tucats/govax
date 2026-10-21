# Phase 71 — process-permanent files

**Status:** planned 2026-10-21. Subtask 1 (the survey, and the VMS probe,
`testdata/ppf71`) is done; Part B's decisions await the author's review,
and subtask 2 the probe's VAX run.

## Goal

govax handles process-permanent files as VMS does, in place of the
special cases it has used instead since Phase 25:

1. **The equivalence strings.** SYS$INPUT, SYS$OUTPUT, SYS$ERROR,
   SYS$COMMAND, and each name DCL's OPEN defines translate to VMS's
   process-permanent file string: ESC, a zero byte, the internal file
   index (IFI) as a word, and then the device name. `$TRNLNM` and
   F$TRNLNM return it whole; SHOW LOGICAL and SHOW TRANSLATION, `$ASSIGN`,
   `$GETDVI`, and F$GETDVI look past the first four bytes.
2. **One table of open files that outlive images**, kept by RMS for each
   process: the four process-permanent files LOGINOUT (govax's process
   startup) opens, a command procedure's input and `/OUTPUT` file, the
   files DCL's OPEN opens, and a subprocess's input and output files.
3. **Indirect access.** An image's `$OPEN` or `$CREATE` of such a name
   reaches the open file instead of opening another, under the rules of
   the *Guide to OpenVMS File Applications*, 6.6.
4. **The special cases go**: RMS's TTA0: console case (except for a
   direct open of the terminal), corevms's output files rewritten whole
   for each line (`outfile.go`), the input swapped in Go during a
   procedure line (`RedirectInput`), and DCL's name walk and its map of
   OPEN's files compared by equivalence string.

Out of scope: process-permanent files of other organizations reached
indirectly (the guide: only sequential files may be), statistics, and
the process I/O segment's size (PIOPAGES).

## Part A — the survey (2026-10-21)

### Sources

- *Guide to OpenVMS File Applications* (7.3; the text is
  `~/Documents/Technical Doc/VMS/vms-file-applications-7.3.txt`):
  - 6.6, "Using Process-Permanent Files": opened with FAB$V_PPF, the
    structures in P1, open across image activations. LOGINOUT opens
    SYS$COMMAND, SYS$INPUT, SYS$OUTPUT, and SYS$ERROR this way, and DCL's
    OPEN opens its files this way. User-mode code reaches them only
    indirectly, through a logical name whose equivalence string "has a
    special format" (the System Services Reference's logical name
    services describe it). For the indirect accessor: a Create is an
    Open, with the FAB written as Open writes it; Open and Create do no
    I/O; any number of them; **one position context for the file**; no
    random access and no `$REWIND` if it was opened with FAB$V_SQO (as
    the four are); NFS, PPF, and UFO in FOP are errors, and some options
    are ignored (SPL, SCF, DLT on Close; RAB$V_ASY; MBC, MBF); a NAM's
    expanded and resultant strings are "solely of the process logical
    name followed by a colon (such as SYS$INPUT:)"; FAB$B_FAC is ignored,
    operations checked against the original open's; the record
    attributes are saved at each Open and Connect, and a print file maps
    carriage control for each Put; no `$ERASE`; `$DECK`, `$EOD`, and
    other `$` records checked on SYS$INPUT when it is a file, not when
    it is a terminal or mailbox; at image exit, the rundown ends the
    indirect I/O but doesn't close the files.
  - 6.2 (logical name translation in a file specification): if an
    equivalence string "refers to a process-permanent file, defaults are
    not needed so RMS stops processing the file specification and copies
    the logical name to the expanded string."
  - 1.x (around line 2754): DCL's OPEN puts RMS's structures in the
    process I/O segment, of PIOPAGES pages, not expanded.
- **Real VMS output already on hand** (VMS 7.3, `testdata/dcl50/vax`):
  - `probe50f.log`: after `OPEN/WRITE OUT` and `OPEN/READ IN`, SHOW
    LOGICAL/FULL shows `"OUT" [super,confine] = "_SIMVAX$DUA1"
    [terminal] (LNM$PROCESS_TABLE)`: supervisor mode, CONFINE and
    TERMINAL, and the device name, with no colon, as what SHOW shows
    after the prefix. A name left after its file closed (`KEEP`) shows
    the same text without the attributes.
  - `probe50i.log`: in a procedure, SHOW TRANSLATION and SHOW LOGICAL of
    SYS$INPUT show `"_SIMVAX$DUA1:"`, the procedure's device, with a
    colon.
- The DCL Dictionary (SPAWN): "the process-permanent open files and any
  image or procedure context are not copied" to a subprocess.

### What govax does today

| Where | What | The difference |
| ----- | ---- | -------------- |
| `internal/lnm/database.go`, `DefineProcessNames` | SYS$INPUT, SYS$OUTPUT, SYS$ERROR, SYS$COMMAND = `_TTA0:` (executive, TERMINAL) | No prefix; nothing open behind them. |
| `internal/rms/create.go`, `open.go` (`consoleDeviceName`) | A spec whose device is TTA0 gets a `FileHandle` writing to the console | Each open is a separate stream; no indirect access, no NAM `SYS$OUTPUT:`. |
| `internal/corevms/outfile.go` | SYS$OUTPUT or SYS$ERROR naming a file (a SPAWN's log, `/USER_MODE` before RUN): the file made at the first line and rewritten whole for every line, a "permanent" file and an image's apart, guessed with `held` | Cost grows with the square of the lines; an image's own `$CREATE` of SYS$OUTPUT opens a different stream. |
| `internal/corevms/input.go`, `RedirectInput`; `internal/dclshell/dclinput.go` | A procedure's data lines reach the image through a Go reader swapped in for each line | No SYS$INPUT entry an image could open; DCL decides what is SYS$INPUT by following names (`processPermanentFile`), not by their value. |
| `internal/dclshell/dclfile.go`, OPEN | Defines the name as the device's display text (`_GOVAX$DUA1`, CONFINE, TERMINAL, supervisor), and keeps the file in a map by name, "orphaned" when the name's value changes | RMS can't reach the file through the name. |

### Side findings: the dry run (2026-10-21)

`testdata/ppf71` runs on govax (`@PPF71/OUTPUT=PPF71.LOG` on a copy of
the exchange volume): everything assembles, links, and runs to the end.
What it shows, before VMS's answers:

1. **An image's `$CREATE` of a name DCL opened makes a new file.**
   F9's `$CREATE PPF71_OUT` created `DUA1:[000000]_GOVAX$DUA1.;1` (the
   name's text taken as a file name) and wrote the image's records into
   it; F2's `$OPEN PPF71_IN` was RMS$_FNF for `_GOVAX$DUA1.;`. VMS gives
   both indirect access to DCL's files.
2. **`@file/OUTPUT=` doesn't change SYS$OUTPUT's translation**: under
   `/OUTPUT=PPF71.LOG`, SYS$OUTPUT still translated to `_TTA0:` (only
   the executive-mode name exists), though the image's output went to
   the log. `$GETDVI` and `$ASSIGN` of SYS$OUTPUT reached the terminal.
3. **In the subprocess**, SYS$OUTPUT translates to the log's file name,
   and `$GETDVI` and `$ASSIGN` of it fail (SS$_IVDEVNAM; VMS gives the
   disk), as do those of SYS$ERROR (also the log) and SYS$COMMAND
   (SS$_NOSUCHDEV); SYS$INPUT, NL:, works. The parent's OPEN names didn't come
   with it (READ was CLI$_UNDFIL), which may be VMS's rule too (the
   names are CONFINE).
4. **Indirect opens don't share a position**: on SYS$INPUT they do, by
   accident (one Go reader), but `$REWIND` and `$FIND` succeed where
   VMS should refuse them, `$PUT` to SYS$INPUT succeeds, FOP's PPF and
   NFS are accepted, the NAM's expanded string is `_TTA0:[000000].;`
   (VMS: `SYS$OUTPUT:`), and FAB$L_DEV is 0.
5. **`DEFINE PPF71_DCLCOPY "''X'"`** copies `_TTA0:` (nothing to copy),
   and DCL's WRITE to it is CLI$_UNDFIL, as WRITE to `PPF71_ALIAS`
   (defined as `SYS$OUTPUT`) is not: DCL follows the names.

## Part B — decisions and questions

### Decisions (proposed; for the author's review)

1. **RMS keeps each process's process-permanent files**, in Go, as
   govax keeps its other per-process structures (the PFN database, the
   working set list): a table of entries, each an open stream (a
   terminal, a volume or host file, a mailbox or NL:, or a command
   procedure's input), with the FAC, SHR, and FOP (SQO) and record
   attributes of the open that made it. It lives for the process's
   life; image rundown ends the image's indirect accesses only.
2. **The equivalence string is VMS's**, byte for byte as the probe
   shows: ESC, 0, the IFI as a word, then the device name (with or
   without its colon, as the probe shows for each kind). `lnm` stores
   it as given (a Go string holds any bytes), with a helper to make and
   read one; nothing in `lnm` treats it specially but file-spec
   translation, which stops at it (the guide's 6.2).
3. **Who looks past the prefix**: SHOW LOGICAL and SHOW TRANSLATION
   (they show the rest, as `probe50f.log` shows), the device lookups
   (`$ASSIGN`, `$GETDVI`, F$GETDVI, `deviceName`), and RMS's name
   processing (which takes the IFI instead). `$TRNLNM`, F$TRNLNM, and
   `$CRELNM` of a copied string keep it.
4. **Indirect access is RMS's**, by the guide's list: `$OPEN`/`$CREATE`
   find the IFI in the string, return an IFI marked indirect (the
   probe shows how), fill the FAB from the entry, write the NAM's
   strings as `NAME:`; `$CONNECT` gives an ISI on the entry's one
   stream; the refused operations get the statuses the probe shows.
5. **DCL and corevms go through the table**: process startup opens the
   four for the terminal (or a subprocess's files); a procedure's input
   and `/OUTPUT` file are entries DCL opens at the level's start and
   closes at its end, defining the supervisor-mode names; OPEN makes an
   entry and CLOSE closes it; LIB$PUT_OUTPUT, LIB$GET_INPUT, `$PUTMSG`,
   and DCL's READ and WRITE use the entries' streams. That replaces
   `outfile.go`, `RedirectInput`, `processPermanentFile`, and the
   `dclFiles` map's comparison of strings (a name names an open file by
   its IFI).
6. **The TTA0: case stays only for a direct open of the terminal**
   (`$CREATE TTA0:`), as a terminal record stream, which is what VMS
   does with it.

### Questions for the author

1. **IFI values.** VMS's numbers depend on its process I/O segment;
   the probe shows their form (which bits mark a process-permanent or
   indirect IFI). Proposed: govax's own numbering in VMS's form, with
   the oracle masking the numbers, as earlier oracles mask addresses.
2. **`$DECK`/`$EOD` checks.** The guide puts them in RMS, for SYS$INPUT
   from a file; govax's DCL does them in its procedure reader
   (`procedureSource.readData`). Proposed: keep the reader as the
   SYS$INPUT entry's stream, so the checks stay where they are tested.
3. **A subprocess's names.** Whether the parent's OPEN names reach a
   subprocess (they're CONFINE, and the DCL Dictionary says its open
   files aren't copied) is the probe's question; proposed: follow it.

### The probe's cases (subtask 1; `testdata/ppf71`)

`PPF71.COM` runs at level 1 under `/OUTPUT`. DCL's view first: a `LEX`
subroutine shows, for a name, F$TRNLNM's length, first four bytes (two
bytes and a word), and the rest, its ACCESS_MODE, TERMINAL, CONFINE,
LENGTH, and TABLE_NAME, the same at executive mode, F$PARSE, and
F$GETDVI; for SYS$INPUT, SYS$OUTPUT, SYS$ERROR, SYS$COMMAND, TT, and a
name defined as `SYS$OUTPUT`; SHOW LOGICAL/FULL, SHOW TRANSLATION, SHOW
LOGICAL/PROCESS; F$PARSE and F$SEARCH of SYS$OUTPUT; DCL's WRITE to the
alias; and F$TRNLNM's string copied by DEFINE. Then the program's parts:

- **T**: `$TRNLNM` of the four at each of executive, supervisor, and
  user mode (status, length, attributes, mode, table, bytes, text), and
  of TT, SYS$DISK, the OPEN names, the copies, and `SYS$OUTPUT:`.
- **D**: `$GETDVIW` (DEVNAM, FULLDEVNAM, DEVCLASS, DEVCHAR) and
  `$ASSIGN` of the same names, and `$GETDVIW` of the channel.
- **O**: SYS$OUTPUT: `$OPEN` then `$CONNECT`, `$PUT`, `$DISPLAY`,
  `$REWIND`, `$FIND`, `$GET`, `$ERASE`, `$CLOSE` (O1); `$CREATE` (O2);
  two opens interleaved (O3); FAC=GET then `$PUT` (O4); FOP PPF, UFO,
  NFS (O5–O7); `SYS$OUTPUT:`, `SYS$OUTPUT:PPF71X.DAT`, `_SYS$OUTPUT:`,
  the alias (O8–O11); `$PARSE` and `$SEARCH` (O12); SYS$OUTPUT's string
  copied by `$CRELNM` to PPF71_COPY, then translated, assigned, opened,
  and written (O13); SYS$ERROR (O14); SYS$COMMAND, open only (O15).
  Every FAB report shows STS, STV, IFI, FOP, DEV, SDC, FAC, SHR, ORG,
  RFM, RAT, MRS, ALQ, FSZ, and the NAM's ESA, RSA, FNB, DVI, and FID;
  every RAB report STS, STV, ISI, RSZ, and RFA.
- **I**: SYS$INPUT from eight data lines: two `$GET`s, a second open's
  `$GET`, the first's again (I1–I3); `$REWIND`, `$FIND` (I4, I5);
  `$DISPLAY`; the rest to the end through the second open, the first
  after the end (I7, I8); FAC=PUT and a `$PUT` (I9); `$CREATE` (I10).
- **F**: after DCL's `OPEN/READ PPF71_IN` (a file of eight records, one
  READ) and `OPEN/WRITE PPF71_OUT` (one WRITE): the names; two indirect
  opens of PPF71_IN reading in turn, `$REWIND`, `$FIND` (F2–F6); FAC=PUT
  and a `$PUT` (F7); `PPF71_IN:PPF71X.DAT` (F8); `$CREATE` and a second
  open of PPF71_OUT writing in turn (F9); FAC=GET of PPF71_OUT and a
  `$GET` (F10). Then DCL READs and WRITEs again, and the file is typed.
- The **subprocess** (`PPF71P.COM`, spawned with `/OUTPUT` and
  `/INPUT=NL:` while the OPEN files are open): DCL's view, parts T and
  D, and a READ of the parent's PPF71_IN.
- The **nested procedure** (`@PPF71S/OUTPUT=PPF71S.LOG`): DCL's view of
  SYS$INPUT and SYS$OUTPUT, parts T and O writing into its log.
- **X**: SYS$OUTPUT opened, a record put, no `$CLOSE`; then DCL writes.

The README's optional step 3 runs T, D, and O at the terminal, where no
`/OUTPUT` stands between the program and the terminal's strings.

## Part C — subtasks

1. **The survey and the probe** (this doc; `testdata/ppf71`, run on
   govax). Done 2026-10-21.
2. **The probe's VAX run**: the answers in Part A, Part B revised by
   them.
3. **The strings** (`lnm`): making and reading a process-permanent file
   string; file-spec translation stopping at one; SHOW LOGICAL and SHOW
   TRANSLATION; the device lookups (`$ASSIGN`, `$GETDVI`, F$GETDVI).
4. **RMS's table and indirect access**: the entries, `$OPEN`/`$CREATE`
   by IFI, `$CONNECT`, `$GET`/`$PUT` on the one stream, the refusals,
   the NAM's strings, the FAB from the entry, rundown.
5. **Process startup's four files**: the terminal entries, the TTA0:
   case narrowed to a direct open, LIB$PUT_OUTPUT, LIB$GET_INPUT, and
   `$PUTMSG` through the entries.
6. **Command procedures**: SYS$INPUT an entry on the procedure's
   reader (replacing `RedirectInput`), `/OUTPUT` an entry on a file, the
   supervisor-mode names at each level.
7. **DCL's OPEN, READ, WRITE, CLOSE** through entries (replacing the
   map's string comparison and `processPermanentFile`); images reaching
   OPEN's files.
8. **Subprocesses**: their input and output files as entries
   (replacing `outfile.go`), and the copying rule the probe shows.
9. **The oracle**: `TestPPF71Oracle` runs the probe on govax and holds
   it to VMS's logs; the other oracles' expectations follow (SHOW
   LOGICAL, the traceback's SYS$OUTPUT and SYS$ERROR, SPAWN's logs).
10. **The close-out**: DEVIATIONS.md, MANUAL.md, CLAUDE.md.

## Progress log

### 2026-10-21 — Planned; subtask 1, the survey and the probe

Revisited at the author's request: Phase 25 left the ESC/IFI prefix out
"until RMS models process-permanent files", and since Phase 50 DCL keeps
files open indefinitely. The survey above found four mechanisms standing
in for process-permanent files, and the guide's 6.6 the rules to replace
them with. Wrote `testdata/ppf71` (the program, three procedures, the
exchange and copy-out scripts, the README) and ran it on govax: it runs
to the end, and its answers are the side findings above. Waiting for the
author's review of Part B and the VAX run.
