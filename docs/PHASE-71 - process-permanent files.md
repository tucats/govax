# Phase 71 — process-permanent files

**Status:** planned 2026-10-21; Part B's decisions and the proposed
answers to its questions accepted by the author the same day. Subtasks 1
(the survey, and the VMS probe, `testdata/ppf71`), 2 (the probe's
VAX run; Part A, "The probe's answers"), 3 (the strings), and 4 (RMS's
table and indirect access) are done.

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

### The probe's answers (2026-10-21)

OpenVMS V7.1 on simh's VAX 8600, the volume on DUA1, run from the
console terminal (OPA0:) as `@PPF71/OUTPUT=PPF71.LOG`, then parts T, D,
and O at the terminal (`vax/ppf71-term.log`; its blank lines are the
author's Returns, answering O1's `$FIND` and `$GET` of the terminal).
Everything ran. Statuses below by name: RMS$_PRV 1829A, RMS$_FOP 1853C,
RMS$_DEV 184C4, RMS$_IOP 18574, RMS$_FAC 18514, RMS$_IFI 18564.

**The strings.** An equivalence string of a process-permanent file is
ESC, 0, a word, then a device name, as the guide says. The word is
`%X8000` plus an index (the entry's number, from 1) plus `%X100`,
except for a file DCL's OPEN/READ opened (PPF71_IN: `%X8005`; its
OPEN/WRITE file, PPF71_OUT, was `%X8106`). What `%X100` means isn't
shown; govax will set it as observed. The device name is the node's
physical name, `_SIMVAX$OPA0:` or `_SIMVAX$DUA1:`, with a colon for the
files LOGINOUT and DCL's procedures open, without one for OPEN's.

| Name | Where | Mode | Attributes | Word | Device |
| ---- | ----- | ---- | ---------- | ---- | ------ |
| SYS$INPUT, SYS$COMMAND | the terminal | exec | concealed, terminal | 8101 | `_SIMVAX$OPA0:` |
| SYS$OUTPUT, SYS$ERROR | the terminal | exec | concealed, terminal | 8103 | `_SIMVAX$OPA0:` |
| SYS$OUTPUT | DCL, at level 0 | super | concealed, terminal, confine | 8103 | the same entry |
| SYS$INPUT | a procedure | super | terminal, confine | 8102 | `_SIMVAX$DUA1:` |
| SYS$OUTPUT | under `@/OUTPUT=` | super | terminal, confine | 8104 | `_SIMVAX$DUA1:` |
| SYS$INPUT | a nested procedure | super | terminal, confine | 8102 again | its own file |
| SYS$OUTPUT | a nested `@/OUTPUT=` | super | terminal, confine | 8105 | its log |
| PPF71_IN, PPF71_OUT | DCL's OPEN | super | terminal, confine | 8005, 8106 | `_SIMVAX$DUA1` |
| SYS$OUTPUT, SYS$ERROR | a subprocess, `/OUTPUT=` | exec | terminal | 8101 | its log |
| SYS$INPUT, SYS$COMMAND | a subprocess, `/INPUT=NL:` | exec | terminal | 8102 | `_NLA0:` |
| SYS$INPUT | its procedure | super | terminal, confine | 8103 | the procedure |
| SYS$OUTPUT | its DCL | super | terminal, confine | 8101 | the exec entry |

So SYS$ERROR shares SYS$OUTPUT's entry and SYS$COMMAND SYS$INPUT's when
they name the same device; at level 0 at the terminal there is no
supervisor SYS$INPUT (the executive name is found); a nested procedure's
input takes entry 2 again (DCL keeps one procedure file open), while
each `/OUTPUT` file takes a new entry. Only the terminal's executive
names are concealed. TT is not a process-permanent file: `OPA0:`
(concealed, terminal) at the console, `_NLA0:` in the subprocess.

**`$TRNLNM` and DCL.** `$TRNLNM` returns the whole string (length 17
for `_SIMVAX$OPA0:`). **F$TRNLNM leaves out the four bytes** (length 13)
though its LENGTH item says 17, so DCL never holds the prefix: `DEFINE
PPF71_DCLCOPY "''X'"` makes a plain name, which RMS and `$GETDVI` follow
to the disk and DCL's WRITE refuses (CLI$_UNDFIL). A copy made by
`$CRELNM` with the whole string is a working process-permanent name
(O13). SHOW LOGICAL and SHOW TRANSLATION show the string after the
prefix (`/FULL` adds `[super,confine]` or `[exec]` and
`[concealed,terminal]`). F$PARSE of SYS$OUTPUT is `SYS$OUTPUT:.;`, of
`X.DAT` with SYS$OUTPUT as default `SYS$OUTPUT:X.DAT;`; F$SEARCH is
empty. F$GETDVI, `$GETDVIW`, and `$ASSIGN` of the names reach the
device after the prefix. (The probe's `F$TRNLNM(P1,,,"EXECUTIVE")`
returned "" for every name, its empty table argument naming no table;
govax agrees.)

**A subprocess** gets its own entries (above); the parent's OPEN names
didn't come with it (they're CONFINE: F$TRNLNM empty, READ CLI$_UNDFIL),
its other names did.

**Indirect access.** `$OPEN` or `$CREATE` (identically) of a
process-permanent name:

- STS RMS$_NORMAL; STV a small number that differs by entry (%X20 to
  %X70; not modeled, masked); FAB$W_IFI `%XC080` plus the index
  (`%XC084` for entry 4); FOP as given; FAC as given (O4's GET stays in
  the FAB).
- FAB$L_SDC the device's characteristics, without DEV$M_IDV for an
  output stream (SYS$OUTPUT's log: `18CD4108`, the disk `1CCD4108`) or
  DEV$M_ODV for an input stream (`14CD4108`); FAB$L_DEV the same with
  RND, FOD, and DIR cleared and REC and CCL set (`08CD0103`). At the
  terminal both are the terminal's own (`0C040007`).
- ORG, RFM, RAT, MRS, FSZ, and ALQ from the file: the `/OUTPUT` logs
  and OPEN/WRITE's file are VFC (2-byte header) print files; a
  procedure is stream-LF with CR; ALQ the file's allocation now. At the
  terminal, `$OPEN` showed VAR, CR, FSZ 2, and `$DISPLAY` VFC, print
  (unconfirmed which `$OPEN` writes; govax will follow the log).
- The NAM: ESA and RSA `NAME:.;` (the name used, `SYS$OUTPUT:.;`,
  `PPF71_COPY:.;`; through an alias, the process-permanent name, not the
  alias), not the guide's `NAME:`; FNB NAM$M_PPF and EXP_DEV
  (`%X10080`); DVI the name as a counted string; FID 0. A file name
  after the colon is kept (`SYS$OUTPUT:PPF71X.DAT;`, FNB adding
  EXP_NAME and EXP_TYPE) and ignored.
- `$CONNECT` gives an ISI (`%XC08n`, numbered apart from the IFIs;
  masked).
- **One position**: two opens, and DCL's own READ, read one stream in
  turn (data records 1 to 8 in order across both opens; PPF71_IN's A1
  by DCL, A2 to A6 by the image, A7 and A8 by DCL again); writes from
  both opens and DCL interleave in order (PPF71B.DAT: B1 to B5).
- `$REWIND` RMS$_IOP, on every stream, OPEN's files too. `$FIND`
  (sequential) succeeds on an input stream (and the terminal's
  SYS$OUTPUT, whose open had GET and PUT), RMS$_FAC on an output-only
  file, as `$GET` is. `$PUT` to an input stream RMS$_FAC (I9, F7); to an
  output stream opened with FAC=GET, success (O4): the original open's
  FAC decides. `$ERASE` RMS$_IFI. `$DISPLAY` succeeds, writing the
  real file's RSA (`DUA1:[000000]PPF71.LOG;1`), DVI (`_SIMVAX$DUA1`),
  FID, the original FAC (a procedure's GET and EXE, `%X82`), and STV an
  address in P1 (masked). `$GET` at the end RMS$_EOF; OPEN/WRITE's file
  read through FAC=GET is at its end (RMS$_EOF), not refused.
- FOP=PPF RMS$_PRV, before parsing (the NAM untouched); UFO and NFS
  RMS$_FOP after it. `_SYS$OUTPUT:` RMS$_DEV, STV SS$_NOSUCHDEV, ESA
  `_SYS$OUTPUT:[000000].;`. `$PARSE` succeeds (ESA `SYS$OUTPUT:.;`, the
  same FNB and DVI, DEV and SDC set); `$SEARCH` after it RMS$_DEV,
  SS$_NOSUCHDEV.
- An image that ends with an indirect stream open leaves the file open
  and its records in it; DCL writes on (part X).

## Part B — decisions and questions

### Decisions (accepted by the author, 2026-10-21)

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

### Questions for the author (answered 2026-10-21: each as proposed)

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

### Revised by the probe (2026-10-21)

- Goal 1 and Decision 3: **F$TRNLNM leaves the prefix out**, as SHOW
  does; only `$TRNLNM` (and `$CRELNM` given it) carries it.
- Decision 2: the word is `%X8000` + `%X100` + the index (no `%X100`
  for OPEN/READ's file); the device text has a colon but for OPEN's
  files. (Subtask 4: the `%X100` is FAB$V_PPF_RAT, bits 6 to 13, holding
  PRN, the RAT of the open that made the file; an indirect IFI's `%X80`
  is the accessor's RAT, CR.) SYS$ERROR shares SYS$OUTPUT's entry, and SYS$COMMAND
  SYS$INPUT's, when they name the same device.
- Decision 4: the NAM's strings are `NAME:.;`, and a file name after the
  colon stays in them. FAB$W_IFI is `%XC080` + the index; STV, ISI, and
  `$DISPLAY`'s STV are govax's own, masked. `$FIND` and `$GET` are
  refused only by the original FAC; `$REWIND` always.
- Decision 5: DCL defines the supervisor-mode names: SYS$OUTPUT at level
  0 (the executive entry), a procedure's SYS$INPUT (one entry, reused
  for a nested procedure), each `/OUTPUT` file's SYS$OUTPUT (an entry
  each). The terminal's executive names are concealed; a subprocess's
  aren't.
- Question 3 settled: a subprocess doesn't get its parent's OPEN names.

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
   them. Done 2026-10-21.
3. **The strings** (`lnm`): making and reading a process-permanent file
   string; file-spec translation stopping at one; SHOW LOGICAL and SHOW
   TRANSLATION; the device lookups (`$ASSIGN`, `$GETDVI`, F$GETDVI).
   Done 2026-10-21.
4. **RMS's table and indirect access**: the entries, `$OPEN`/`$CREATE`
   by IFI, `$CONNECT`, `$GET`/`$PUT` on the one stream, the refusals,
   the NAM's strings, the FAB from the entry, rundown. Done 2026-10-21.
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

### 2026-10-21 — Plan accepted

The author accepted Part B's decisions and the proposed answer to each
question: govax's own IFI numbering in VMS's form, masked by the
oracle; the `$DECK`/`$EOD` checks staying in DCL's procedure reader,
which becomes the SYS$INPUT entry's stream; a subprocess's names by
what the probe shows. The author is running the probe on the VAX.

### 2026-10-21 — Subtask 2: the probe's VAX run

The author ran the probe on VMS V7.1 (simh's VAX 8600), the procedure
and the terminal parts; the logs, PPF71B.DAT, and the image are in
`testdata/ppf71/vax` (an interactive LINK writes no map, so copyout
copies none). Part A's "The probe's answers" has what they show, and
Part B's "Revised by the probe" what changes: chiefly that F$TRNLNM
drops the prefix, that the NAM shows `NAME:.;`, and the supervisor-mode
names DCL defines at each level. What stays unconfirmed (the word's
`%X100`, the indirect open's STV, the terminal's RFM after `$OPEN`) is
marked there and gets govax's own choice.

### 2026-10-21 — Subtask 3: the strings

`internal/lnm/ppf.go`: `PermanentFileString(ifi, device)` makes the
string (ESC, 0, the IFI low byte first, the device), `ParsePermanentFile`
reads one into a `PermanentFile` (IFI, Device), and `DisplayValue` is a
value as a person sees it (the device, or any other string unchanged).
The database stores the string whole, so `$TRNLNM` returns it and
`$CRELNM` of a copy makes a working name, with nothing more to do.

`TranslateFileSpec` stops at such a string, whatever its attributes (a
`$CRELNM` copy may have none): the result's `Permanent` is the file and
`PermanentName` the name whose string it was (through an alias, the
process-permanent name, which the NAM's strings will show), and its
`Spec` is the device, given a colon if it has none (OPEN's strings
don't), followed by the rest of the caller's spec. So the device
lookups (`rms.PhysicalDevice`, under corevms's `deviceName`: `$ASSIGN`,
`$GETDVI`, F$GETDVI) reach the device after the prefix unchanged, and
RMS, until subtask 4 takes `Permanent` for indirect access, opens the
device as before.

F$TRNLNM's value and F$LOGICAL drop the prefix (F$LOGICAL's unconfirmed,
following F$TRNLNM), while LENGTH counts it; SHOW LOGICAL (every value,
`/FULL` too) and SHOW TRANSLATION show the device, and SHOW LOGICAL
doesn't follow a process-permanent string as another name. Nothing
defines such strings yet: process startup's (subtask 5), the
procedures' (6), and OPEN's (7) come with their entries. Tests:
`TestPermanentFileString`, `TestTranslateFileSpec_permanentFile`,
`TestLexicalPermanentFile`, `TestShowLogical_permanentFile`, and
`TestServiceSysAssignAndGetdviPermanentFile` (with and without the
node's name and the colon, through an alias; `$TRNLNM` whole).

### 2026-10-21 — Subtask 4: RMS's table and indirect access

**The words explained.** VMS's definitions have FAB$V_PPIFI (bit 15),
FAB$V_PPF_IND (bit 14), and FAB$V_PPF_RAT (bits 6 to 13, mask
`%X3FC0`) in FAB$W_IFI, and the probe's words fit them exactly: the
string's `%X8101` is PPIFI, RAT=PRN (`4 << 6`), index 1; OPEN/READ's
`%X8005` has RAT 0; an indirect `%XC084` is PPIFI, PPF_IND, RAT=CR (the
accessor's FAB as given), index 4. So the `%X100` Part A left open is
the PRN of the open that made the file, and the "record attributes saved
at each Open and Connect" of the guide are saved in the IFI and the ISI,
which is how govax keeps them too: no per-accessor state at all. The
ISI's index is the entry's stream's (SYS$INPUT file 2, stream 3), and
`$DISPLAY`'s STV, `%X7FF9C000` plus the stream's number, fits all four
of the probe's.

`internal/rms/ppf.go`: each `FileTable` keeps its process-permanent files
apart from the image's (`permanent`, by index, so `Rundown` at image exit
leaves them), each a `PermanentEntry`: the open stream (a `FileHandle`:
the terminal, a volume file, a record device), the `PermanentOptions`
(the device text the string carries, the opener's RAT, SQO), and the
device characteristics (a volume file's without IDV if the open doesn't
read, without ODV if it doesn't write, as VMS 7.1's `18CD4108` and
`14CD4108`). `AddPermanent` makes an open stream an entry (arming it, at
the end of the file if it only writes), `MakePermanent` an open IFI's
file; `IFI` and `EquivalenceString` are what DCL will define names with;
`Permanent` finds an entry by any word naming it; `ClosePermanent` and
`ClosePermanentFiles` close them (corevms's `processRundown` calls the
second at process deletion). Entries and streams are numbered from 1,
lowest free first. `FileHandle.arm` is `$CONNECT`'s arming, now shared by
`armForFAC`, `DCLFile.arm`, and `AddPermanent`.

Name processing (`name.go`) stops at a process-permanent string: the
device is the logical name with a colon, the program's own name, type,
and version are kept, nothing else is merged, and FNB has NAM$M_PPF and
EXP_DEV (a default name's device leads there too, without EXP_DEV:
`permanentName`; unconfirmed beyond F$PARSE's answer). `ppfaccess.go` is
the services' side:

- `$OPEN` and `$CREATE` (`openIndirect`): FOP's PPF RMS$_PRV before the
  name is processed; UFO and NFS RMS$_FOP after it; the FAB's IFI the
  indirect word, DEV and SDC, a volume file's attributes (the terminal's:
  only FSZ, 2, as VMS 7.1 left the rest); the NAM's RSA the expanded
  string, DVI the logical name, FID and DID zero. STV is 0 (VMS's
  varied, masked by the oracle). `$CREATE`'s refusal of an access without
  PUT still comes first (unconfirmed for an indirect create).
- `$CONNECT`: the ISI, nothing armed or moved. `$CLOSE`: the IFI cleared,
  the file left open. `$DISPLAY` (`displayIndirect`): the original FAC,
  the file's RSA, FID, and DVI, and for the terminal VFC, PRN, FSZ 2
  and the device's DVI (VMS's RSA for it, `SYS$OUTPUT:[].LOG;`, isn't
  written). `$PARSE` (`parseIndirect`): DVI the logical name, DEV and
  SDC both the device's characteristics; `$SEARCH` after it was already
  RMS$_DEV, SS$_NOSUCHDEV.
- The RAB services find an indirect stream by `LookupStream`. `$GET` and
  `$FIND` need the original open's GET, `$PUT` its PUT (RMS$_FAC,
  `indirectAccess`), `$REWIND` is RMS$_IOP; `$ERASE` of an open FAB was
  already RMS$_IFI. A `$PUT` to a VFC print file takes carriage control
  from the accessor's RAT (CR: `01 8D`, as DCL's records; otherwise
  zeros; unconfirmed, `indirectControl`).
- Not modeled: the guide's "no random access" for SQO files (an indirect
  `$GET` by RFA goes through), and a name whose file has been closed is
  RMS$_IFI at `$OPEN` and RMS$_DEV at `$PARSE` (both unconfirmed).

Found on the way: `$CREATE` ignored FAB$B_FSZ, so a VFC file it made
had no fixed control; it now takes FSZ, 0 meaning 2 (RMS manual).

Tests (`internal/rms/ppf_test.go`): `TestPermanentWords` (the probe's
words), `TestPermanent_outputIndirect` (O1, O2, O4: the FAB and NAM,
`$DISPLAY`, the refusals, `$CLOSE`, rundown, the records with their
carriage control), `TestPermanent_twoOpensOneStream` (O3, I1 to I9: two
opens read one stream in turn; an input stream's characteristics),
`TestPermanent_names` (O5 to O13: FOP, a name after the colon, an
alias, `$PARSE` and `$SEARCH`, a default name, a copied string, a closed
file), and `TestPermanent_terminal`. Nothing makes entries yet but the
tests: process startup's (subtask 5), the procedures' (6), and OPEN's
(7) come next.
