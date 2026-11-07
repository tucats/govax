# Phase 90 — VMS help

**Status:** planned 2026-11-07; the survey and plan reviewed and accepted
by the author the same day ("The review", after Part B). Needs nothing
beyond what Phases 1–89 left in place.

## Goal

HELP as VMS has it. govax's help is now two text files in a format of its
own (`$KEY,KEY` lines, four-character tokens). It can't list a topic's
subtopics, prompt for more, take abbreviations, or go down a command's tree
of parameters, qualifiers, and examples. When this phase is done:

- govax's help is **VMS help libraries**: `SYS$HELP:HELPLIB.HLB` for DCL,
  and the debugger's own library beside it, built from `.HLP` modules in
  VMS's help format during govax's build, as LIBRARY/HELP would build them.
  They go on the system volume like its other libraries, and a user can add
  modules of their own with `LIBRARY/HELP`.
- **HELP is VMS's HELP**: the level-1 topic list (and HELP with no topic or
  `HELP HELP`), each topic's text indented by its level, "additional
  information available" with the subtopics in columns, abbreviations,
  ambiguous matches, wildcards and `...`, `/qualifier` keys, the `Topic?`
  and `Subtopic?` prompts and how `?`, Return, and Ctrl/Z answer them,
  `@library`, the `HLP$LIBRARY` user libraries, and HELP's qualifiers.
  Each of these is held to VMS 7.1's output by a probe.
- The **debugger's HELP** reads its own library the way VMS 7.1's debugger
  does.
- **LBRSHR's routines** a VAX program needs to read libraries are there:
  `LBR$OUTPUT_HELP` and `LBR$GET_HELP`, and the routines that open a
  library and read a text, help, or macro module's records. They run on the
  same Go code DCL's HELP uses.
- govax's **console and debugger help are rewritten** as VMS-style modules,
  in govax's own words, with tests that every command and qualifier the
  grammars have is documented.

## Part A — the survey

### A.1 What govax does today

- **`internal/help`** (159 lines) parses a text file in which a line
  `$SHOW,LOGI` names a topic by up to as many words as HELP was given, each
  cut or padded to four characters. Several `$` lines in a row share one
  body. `Show` prints the body for the key `helpKey(words)` builds, or "No
  help available for that topic". It has no tree. A topic doesn't know its
  subtopics, so nothing lists them. An abbreviation shorter than four
  characters works only if the file spells it out (`$SH  ,LOGI`). Nothing
  prompts.
- **The files** are `internal/bootdata/files/console.help` (5,536 lines,
  176 `$` keys) and `debug.help` (2,253 lines, 279 keys). The system volume
  holds them as text files, `[SYSTEM.SYSHLP]CONSOLE.HLP` and `DEBUG.HLP`
  (`internal/sysvol/files.go`, Supplied). `cmd/govax/grammarfiles.go`'s
  `help` reads them through the resolver (volume, then built in) when govax
  starts. The console's `Dispatcher.Help` and the debugger's are parsed
  `*help.File`s.
- **The grammars**: `console.dcl`'s `verb help/id=1630` and `debug.dcl`'s
  `verb help/id=3` take the rest of the line, so HELP has no qualifiers.
- **Tests** that rest on the format: `internal/console/help_test.go` (keys
  resolve; Phase 23's and 68's topics are there; `HELP X/Y` without a
  space), `internal/debugger/help_test.go` (`TestEveryVerbHasHelp`, its
  topics, unique keys), and `cmd/govax/main_test.go`'s help-from-embedded
  file test. MANUAL.md mentions HELP in a few places, and its section 5
  table names CONSOLE.HLP and DEBUG.HLP.
- **`internal/lbr`** already reads help libraries, DCX-reduced ones too:
  LIBRARY/LIST/FULL of VMS's HELPLIB.HLB lists its 122 modules, and
  LIBRARY/EXTRACT writes a module's text. The Builder writes object, macro,
  shareable, and text libraries, but **LIBRARY has no `/HELP`**, so it can't
  make a help library or insert a module into one. `library.go`'s
  `extract` already handles `lbr.TypeHelp`.
- **There are no LBR$ routines.** `vmsdef` has no LBRSHR symbols, LBR$
  messages, or `$LBRDEF`/`$HLPDEF` values. VMS 7.1's
  `[VMS$COMMON.SYSLIB]LBRSHR.EXE` (70 blocks) is on the local system disk,
  so `internal/vmsdef/gen -image` can take its transfer vector from its GST,
  as Phase 77 did for MTHRTL.

### A.2 VMS's help libraries, as the system disk has them

The local VMS 7.1 system disk (`testdata/disks/rq0-ra92.dsk`, `DUA2:` when
mounted) has 33 `.HLB` files in `[VMS$COMMON.SYSHLP]`. I looked at the
headers of two, and at the shape of three DCL modules and the debugger's
module, by extracting them with govax's own LIBRARY to a scratch directory
outside the repository. No VMS text is in this document or the
repository.

**HELPLIB.HLB**: library format 3.0, created by Librarian T09-20, 122
modules, a key length of 15, DCX data-reduced, 5,869 blocks. Module names
keep their case (`DCL_Tips`, `DECthreads`), and some are punctuation (`:=`,
`=`, `@`).

**The debugger's library is `DBG$HELP.HLB`**, not `DEBUG.HLB`, and it is
built differently. It has **one module, `DEBUG`**, a key length of 80, no
DCX, 31,438 lines. Every command and concept is a level-2 topic under that
one level-1 key: 65 of them (concept topics such as `Address_Expressions`,
`Built_in_Symbols`, `Path_Names`, then the commands from
`@_(Execute_Procedure)` to `WHILE`). Its keys go down to level 6
(1,046 at level 3, 387 at 4, 42 at 5, 5 at 6), with 437 `/qualifier` lines.
`SET` and `SHOW` have a level-3 topic for each keyword (`SET BREAK`,
`SET EDITOR`, ...) with Parameters, Qualifiers, Description, and Examples at
level 4 below it. Long topics are split into `Description` and
`Description,_Continued...`. Keys use underscores for spaces.

**The module format**, as these modules show it (the probe confirms each
point, and settles what they leave open):

1. A **key line** is a level digit 1–9 in column 1, one or more blanks
   (both one and two occur), and the key. A module starts with its level-1
   key line, which is the module's name in the index. Text follows each key
   until the next key line.
2. A **key may start with `/`.** EXCHANGE has `2 /NETWORK`, a qualifier of
   the command, beside its subcommands. HELP's own module has
   `2 /INSTRUCTIONS`, `2 /LIBRARY`, and so on.
3. **Under a topic named `Qualifiers`** (and under ANALYZE /MEDIA's
   `Command_Qualifiers`), a line with `/` in column 1 and no digit starts a
   subtopic one level down: SUBMIT's `2 Qualifiers` is followed by
   `/AFTER`, `/BACKUP`, ..., each with its text. Is the rule the topic's
   name, or any `/` in column 1? The probe settles it.
4. **Under `Parameter` or `Parameters`**, each parameter's name is a line
   in column 1 (SUBMIT's `filespec[,...]`) with no digit. It looks like
   plain text. The probe settles whether HELP treats it as a key.
5. **Text** is indented in the source (three blanks in the DCL modules), and
   lines are at most 78 characters. The manual says HELP adds two blanks per
   key level when it displays a topic. A key's text often ends with a line
   holding one blank, then an empty line. The probe shows whether that line
   means anything.

**Three shapes of command**, which govax's modules will copy:

- **A simple command** (SUBMIT): level 1 is the description and Format, and
  level 2 is `Parameter`, `Qualifiers` (with `/` lines), and `Examples`.
- **A utility with subcommands** (EXCHANGE): level 1 and its `Description`
  and `Parameters`; a level-2 topic for each subcommand (COPY, DELETE,
  DIRECTORY, ..., TYPE), each with level-3 `Parameters`, `Description`,
  `Qualifiers`, and `Examples`; and `2 /NETWORK` for the command's own
  qualifier.
- **One verb, several commands** (ANALYZE): level 1 says what ANALYZE is,
  and each form is a level-2 topic keyed by its qualifier (`/AUDIT`,
  `/CRASH_DUMP`, `/DISK_STRUCTURE`, `/ERROR_LOG`, `/IMAGE`, `/MEDIA`, ...).
  Each form has its own `Parameter`, `Description`, `Qualifiers` (with `/`
  lines), and `Examples` at level 3. Some forms put each qualifier at
  level 3 as a numbered key with its own `Examples` at level 4
  (`/AUDIT`'s `/BEFORE`). `/AUDIT /SELECT` has a level-4 topic for each of
  its keywords. So a qualifier is a `/` line when it is a leaf, and a
  numbered `/KEY` when it has subtopics. govax's ANALYZE (`/OBJECT`,
  `/IMAGE`, `/RMS_FILE`) and other verbs whose qualifier picks the syntax
  (INITIALIZE `/CONTAINER`, CREATE `/DIRECTORY`, `/FDL`, `/NAME_TABLE`)
  take this shape.

### A.3 HELP's behavior, as the manuals give it

The *OpenVMS DCL Dictionary* (7.3-1, HELP) and the *Utility Routines
Manual* (7.3-1, LBR$OUTPUT_HELP and LBR$GET_HELP) describe:

- **Requests.** Keys are separated by blanks or `/`. No key gives HELP's
  instructions and the list of level-1 topics (`/NOINSTRUCTIONS` gives only
  the list). `HELP` gives the HELP topic and its subtopics. Any key may be
  abbreviated, and an ambiguous abbreviation shows every match. `*` in
  place of a key shows every topic at that level. `...` after a level-1 key
  shows the topic and all its subtopics. `*` and `%` work inside a key.
  `@filespec` as the first key makes that library the root.
- **Prompts.** There are four: `Topic?`; `@library Topic?` when the root
  isn't the main library; `keyword... Subtopic?` while in a topic; and the
  last two combined. At each one, keys search the libraries (or the current
  topic's subtopics), `?` shows the list again, Return goes up a level (or
  ends HELP at the top), and Ctrl/Z ends HELP. A topic with subtopics
  prompts for them. One without prompts for the current topic's other
  subtopics.
- **"Not found."** The manual's example shows "Sorry, no documentation on
  X", then the list of additional information available.
- **Libraries.** The main library is `SYS$HELP:HELPLIB.HLB`, and
  `/LIBRARY=` names another (default `SYS$HELP:`, type `.HLB`). The user
  libraries are `HLP$LIBRARY`, `HLP$LIBRARY_1`, ... (numbered with no gaps)
  in the process, group, and system tables, in that order
  (`/USERLIBRARY=`). A miss searches root, main, process, group, and system
  libraries, then the root again so the context returns to it.
  `/[NO]LIBLIST` lists the user libraries with the topics.
- **Qualifiers.** `/[NO]PROMPT`, `/[NO]INSTRUCTIONS`, `/[NO]LIBLIST`,
  `/LIBRARY`, `/NOLIBRARY`, `/USERLIBRARY`, `/OUTPUT` (default
  `HELP.LIS`), `/PAGE[=CLEAR_SCREEN|SCROLL|SAVE[=n]]`, `/SEARCH`, `/EXACT`,
  `/HIGHLIGHT`, `/WRAP`, `/MESSAGE`.
- **LBR$OUTPUT_HELP** (output routine, width default 80, the request line,
  library, flags `HLP$M_PROMPT`, `PROCESS`, `GROUP`, `SYSTEM`, `LIBLIST`,
  `HELP`, and the input routine) is the whole interactive HELP in one call.
  **LBR$GET_HELP** (an open library, width, an action routine called with
  the line, flags `HLP$M_NOHLPTXT`, `KEYNAMLIN`, `OTHERINFO`, user data, and
  the key level, then up to ten keys) returns a topic's text "in the same
  format as the output returned by the DCL command HELP", indented two
  blanks per level. Statuses: `LBR$_ILLINROU`, `ILLOUTROU`, `NOHLPLIS`,
  `TOOMNYARG`, `USRINPERR`, `ILLCTL`, `LIBNOTOPN`, `NOTHLPLIB`.

The *Command Definition, Librarian, and Message Utilities Manual*, which
defines the help module format and LIBRARY/HELP, isn't in the local
library. The format's rules come from VMS's own modules (A.2) and from what
VMS's LIBRARY and HELP do with modules we write (the probe).

### A.4 What the manuals and modules don't settle

The probe's questions:

- **The exact layout** of a display: blank lines, each level's indent, the
  header lines that repeat the keys of the topic shown, the "Additional
  information available:" line, the column width and count of the subtopic
  list (fixed columns, or by the longest key; at 80 columns, at other
  terminal widths, and with `/OUTPUT`), and HELP's own instructions, which
  come from the HELP module's text.
- **Matching**: whether an abbreviation that exactly matches one key and
  starts another is still ambiguous; what case does; underscores; `%` and
  `*`; keys with blanks, commas, or parentheses (`@_(Execute_Procedure)`);
  `/` with and without blanks around it (`HELP SUBMIT/AFTER`,
  `HELP SUBMIT /AFTER`, `HELP SUBMIT Qualifiers /AFTER`); whether a `/`
  line under Qualifiers is found one level up, as `HELP SUBMIT /AFTER`
  suggests; parameter names under Parameters (A.2, 4).
- **The prompts**: the keys shown before `Subtopic?`; `?`; Return at each
  depth; Ctrl/Z; an unknown subtopic; a topic with no subtopics; `@lib` at
  each prompt.
- **Where prompts read** when HELP runs in a command procedure or with
  SYS$INPUT not a terminal (does it prompt, and on SYS$INPUT or
  SYS$COMMAND?), and with `/OUTPUT`.
- **The user libraries**: their order, `/LIBLIST`'s layout, a gap in the
  numbers, a library that can't be opened, and the message when no library
  can be.
- **LIBRARY/HELP**: the default key length (HELPLIB has 15, DBG$HELP 80);
  how a `.HLP` file with several level-1 keys becomes modules; case; text
  before the first key; a level that skips (1 then 3); a key longer than the
  key length; `/DATA=REDUCE`; `/LIST/FULL`; `/EXTRACT`. The libraries VMS
  makes are kept, so govax's can be held to them byte for byte, as its
  macro and text libraries were.
- **The debugger's HELP**: where it finds its library (a `DBG$HELP` logical
  name, or a fixed `SYS$HELP:DBG$HELP.HLB`), whether `HELP X` is looked up
  as `DEBUG X`, whether it prompts, and its "not found".
- **The LBR$ routines**: each one's statuses and arguments in the cases a
  program meets, GET_HELP's action routine calls (flags and levels per
  line), and OUTPUT_HELP's prompting through a program's own input routine.

### A.5 Where the real difficulties are

- **The clean room.** VMS's help text is DIGITAL's prose, and none of it may
  reach the build. The probe makes this easy. Its libraries are `.HLP`
  files we write, made into libraries by VMS's LIBRARY, and every question
  in A.4 is asked of them. So VMS's logs hold only our own text and HELP's
  framing, and can be committed like other probes' logs. The runs on VMS's
  EXCHANGE, SUBMIT, and ANALYZE modules, which you suggested, are a check
  that real modules behave as ours do. Their logs are DIGITAL's text, so
  they stay local (Decision 9).
- **Rewriting the help** is most of the work: 7,800 lines in about 450 keys
  become a tree of modules for some 90 DCL verbs and govax's topics, plus
  the debugger's ~60 commands. MANUAL.md and the grammars are the sources.
  It should be split into several subtasks, and tests should check it
  against the grammars as it goes.
- **One engine for three callers.** The console's HELP, the debugger's
  HELP, and LBR$OUTPUT_HELP all need the same lookup, display, and
  prompting. OUTPUT_HELP calls a VAX program's output and input routines,
  which is Phase 58's callback mechanism.
- **User modules and the system volume's refresh.** If a user inserts a
  module into HELPLIB.HLB, the file is no longer govax's copy, so Phase
  87's refresh keeps it with `%MOUNT-I-SYSKEPT` and govax's later help never
  reaches that user. A help library should be refreshed by module instead
  (Decision 7 and Question 2).

## Part B — decisions

Proposed with the survey, and accepted in the review below, with its
answers written in.

1. **govax's help sources are `.HLP` modules in VMS's format, in govax's
   own words**, one file per level-1 topic, in
   `internal/bootdata/files/help/` (the DCL library) and
   `internal/bootdata/files/dbghelp/` (the debugger's). A new
   `go generate` step builds the two libraries with `internal/lbr`, as
   LIBRARY/CREATE/HELP would and reproducibly (times fixed, as FTNRTL's
   build does). They are committed and embedded, and a test checks they're
   current (like `TestCommittedImage`). Text no reader of govax's help would
   miss isn't copied from MANUAL.md, and nothing comes from VMS's modules
   but the shapes in A.2.
2. **The libraries are `SYS$HELP:HELPLIB.HLB` and `SYS$HELP:DBG$HELP.HLB`**
   (`[SYSTEM.SYSHLP]`, Supplied, Binary), VMS's names, with the debugger's
   in VMS's shape: one `DEBUG` module with the commands at level 2 (the
   review's answer 1). The `.HLP` sources go on
   the system volume too, in `[SYSTEM.SYSHLP.SOURCES]`, with
   `SYS$UPDATE:BUILDHELP.COM` to rebuild the libraries from them with
   LIBRARY/HELP, as BUILDSTARLET.COM rebuilds STARLET.MLB. CONSOLE.HLP and
   DEBUG.HLP go away.
3. **`internal/help` is rewritten as the engine**, plain Go, knowing
   nothing of RMS or the console:
   - `ParseModule` turns a module's records into a tree of topics. Each
     topic has its key, level, text lines, and subtopics, with A.2's rules
     for `/` lines.
   - A `Library` interface gives a module by name and the names in order,
     over `lbr.Library`.
   - `Lookup` applies VMS's matching rules (abbreviations, ambiguity, `*`,
     `%`, `...`, `/`).
   - `Render` writes a topic in HELP's layout at a given width.
   - `Session` is the interactive HELP: libraries, prompts, and the
     responses, over an output function and an input function (which
     returns end of file for Ctrl/Z).

   The console's HELP, the debugger's HELP, and LBR$OUTPUT_HELP are three
   thin callers.
4. **DCL's HELP takes VMS's qualifiers**: `/[NO]PROMPT`,
   `/[NO]INSTRUCTIONS`, `/[NO]LIBLIST`, `/LIBRARY`, `/NOLIBRARY`,
   `/USERLIBRARY`, `/OUTPUT`, and `/PAGE` in its simple form (stop after
   each screenful, `CLEAR_SCREEN` and `SCROLL`). `/PAGE=SAVE`'s screen
   navigation, `/SEARCH`, `/EXACT`, `/HIGHLIGHT`, and `/WRAP` wait for a
   later phase, as does `HELP/MESSAGE`, which needs the MSGHLP database and
   messages of govax's own (the review's answer 3). Those qualifiers are accepted and report
   NOTIMPL. The topic is still the rest of the line, with qualifiers after
   the verb only (`HELP/NOPROMPT COPY /LOG` asks about `/LOG`). The probe
   decides whether VMS reads it the same way.
5. **The library search is VMS's**: root, main, the `HLP$LIBRARY` names in
   the process, group, and system tables (govax has those tables), then the
   root again. Names are found through RMS with default `SYS$HELP:.HLB`, so
   a volume library, a host file by `--path`, and a user's library in their
   own directory all work. A system volume that has no HELPLIB.HLB falls
   back to govax's built-in copy, as the grammars do now.
6. **LIBRARY/HELP** creates, inserts, replaces, deletes, lists, and
   extracts help modules, by the probe's rules for splitting a `.HLP` file
   into modules, key length, and case. It is held to VMS's libraries byte
   for byte, and to VMS's LIBRARY messages for bad modules.
7. **The refresh treats a help library by module.** A HELPLIB.HLB or
   DBG$HELP.HLB that differs from govax's gets govax's new modules, and
   modules the user inserted are kept. A module both supply is kept as the
   user's, with SYSKEPT naming the module. The manifest records each module
   govax supplied, so the refresh can tell which modules the user changed.
   (The review's answer 2.)
8. **LBRSHR.EXE is emulated in a package of its own, `internal/lbrrtl`**,
   like `internal/mthrtl`. Its vector is read from VMS 7.1's LBRSHR GST
   into `vmsdef` (`gen -image`), with LBR$ messages and `$LBRDEF`,
   `$HLPDEF`, `$CREDEF`, `$MHDDEF` values added by `gen` as earlier
   definitions were. RUN's locator finds it as an emulated image, and LINK
   resolves `LBR$` names from govax's tables. A library's control index
   names Go state, as LIB$'s zones do. The routines in this phase:
   - `LBR$INI_CONTROL`, `LBR$OPEN`, `LBR$CLOSE`
   - `LBR$GET_HEADER`, `LBR$SET_INDEX`, `LBR$LOOKUP_KEY`, `LBR$FIND`
   - `LBR$GET_RECORD` (move and locate modes, `LBR$SET_MOVE`,
     `LBR$SET_LOCATE`), `LBR$GET_INDEX` (an action routine for each key)
   - `LBR$GET_HELP`, `LBR$OUTPUT_HELP`, `LBR$RET_RMSSTV`

   The writing routines (`LBR$PUT_RECORD`, `LBR$PUT_END`,
   `LBR$INSERT_KEY`, `LBR$REPLACE_KEY`, `LBR$DELETE_KEY`,
   `LBR$DELETE_DATA`, `LBR$PUT_HISTORY`) are this phase's last subtask
   before the close-out (the review's answer 4). Every other vector entry
   (`LBR$SEARCH`, `LBR$MAP_MODULE`, ...) is a stub that signals NOTIMPL
   with its name, until a later phase needs it. The routines read through `internal/lbr`,
   and the help routines through the engine.
9. **The probe's VMS logs of our own modules are committed; its runs on
   VMS's modules are not.** `testdata/help90` holds our `.HLP` files, the
   procedures, and VMS's logs and libraries made from our modules. The
   EXCHANGE, SUBMIT, and ANALYZE runs go to a local, gitignored
   `testdata/disks/help90-vms/`, and their tests skip without it, as
   DUNGEON's do. The extracts made for this survey are in a scratch
   directory and will be deleted.

   **This is deliberately conservative** (the review's answer 5). Logs of
   HELP displaying our own modules hold no DIGITAL text, and a log of
   VMS's modules would likely do no harm either. But a log of VMS's help
   is DIGITAL's prose word for word, and keeping it out of the repository
   means it can't reach govax's help, its tests' expected output, or a
   commit by accident. govax's help is written without those logs open:
   what they teach about HELP's layout and matching is taken from the logs
   of our own modules, which show the same rules.
10. **Tests hold the help to the grammars**:
    - every console verb and debugger command has a topic;
    - every qualifier a verb's grammar takes appears as a `/` line or
      `/KEY` under its command or form (with a short list of exceptions,
      each with a reason);
    - every configuration key in `validConfigs` is under CONFIG KEYS;
    - every module parses, with no level skipped and no key repeated among
      siblings.

    The old tests that read the `$` format are replaced by these.
11. **govax's own topics** (CONFIG, KEYS, STARTUP, SYSTEM_VOLUME, and the
    machine's commands such as VMINIT and DEBUG) become level-1 modules
    beside the verbs. Where VMS has a module for the same subject, govax's
    uses VMS's name and shape: `Lexicals` with a level-2 topic per function
    (now LEXICAL), `Symbol_Assign`, `Specify`, and `INSTRUCTIONS` and
    `HINTS` (HELP's own topics).

### Questions for the review

1. **The debugger's library name.** VMS 7.1's is `SYS$HELP:DBG$HELP.HLB`,
   with one module `DEBUG`. Use VMS's name and shape (recommended), or
   `DEBUG.HLB` as you suggested?
2. **User modules in HELPLIB.HLB.** When govax's help changes, should the
   refresh merge by module, keeping the user's modules (Decision 7,
   recommended)? Or keep the user's whole file with SYSKEPT, as for other
   files, and recommend `HLP$LIBRARY` for a user's own help, which never
   conflicts? VMS's own upgrades simply replaced HELPLIB.HLB.
3. **What waits.** May `/PAGE=SAVE` and its companions, and `HELP/MESSAGE`,
   wait for a later phase (Decision 4)?
4. **LBR$ scope.** Is Decision 8's list enough: reading every library type
   and both help routines, with the writing routines (`LBR$PUT_RECORD`,
   `LBR$INSERT_KEY`, `LBR$REPLACE_KEY`, `LBR$DELETE_*`, `LBR$PUT_END`) as
   NOTIMPL stubs? Writing could come in this phase as a last subtask if you
   want programs to build libraries.
5. **The VMS-module runs** (Decision 9): keep their logs local, as
   recommended, or commit them as other probes' VMS logs are?

### The review

On 2026-11-07 the author accepted the plan, answering the questions:

1. The debugger's library takes VMS's name and shape,
   `SYS$HELP:DBG$HELP.HLB` with one `DEBUG` module.
2. The refresh treats a help library module by module (Decision 7).
3. `/PAGE=SAVE`, `/SEARCH`, `/EXACT`, `/HIGHLIGHT`, `/WRAP`, and
   `HELP/MESSAGE` wait for a later phase.
4. Writing libraries through the LBR$ routines is a subtask of this phase,
   the last before the close-out.
5. The logs of VMS's own modules stay local, a conservative choice made to
   rule out contaminating govax's help (Decision 9).

## The probe

`testdata/help90`, run on the VMS 7.1 VAX through an exchange volume as
the earlier probes were (`exchange.cmd`, `copyout.cmd`). The rounds are
planned now, and each round's findings may change the next:

- **Round 1, batch** (`help90.com`, logged):
  - **A. LIBRARY/HELP** on our modules: `/CREATE` with and without key
    lengths and `/DATA=REDUCE`, `/INSERT`, `/REPLACE`, `/DELETE`,
    `/LIST/FULL`, `/EXTRACT`, a file with several level-1 keys, and the
    malformed cases (A.4). The libraries VMS makes are kept.
  - **B. HELP/NOPROMPT/LIBRARY=** each probe library, through a matrix of
    requests written by `gen.go`: no key, HELP, each level, abbreviations,
    ambiguity, case, wildcards, `*`, `...`, `/` forms, Qualifiers' and
    Parameters' column-1 lines, deep keys, long and odd keys, misses, and
    `/NOINSTRUCTIONS`. Each request runs both with `/OUTPUT` and to the
    terminal at widths 80 and 132 (`SET TERMINAL/WIDTH`).
  - **C. User libraries**: `HLP$LIBRARY` chains in each table, a gap,
    `/USERLIBRARY=`, `/LIBLIST`, `@lib`, a missing library.
  - **D. HELP in a procedure** with `/PROMPT`, the responses as data
    lines, and with SYS$INPUT a file.
  - **E. The LBR$ routines**: `lbr90.mar` calls each routine of Decision 8
    on the probe's libraries and a text and macro library, printing
    statuses, headers, records, the GET_INDEX keys, the GET_HELP action
    routine's every call (line, flags, level), and an OUTPUT_HELP session
    driven by its own input routine from a script.
  - **F. The debugger's HELP**: DEBUG on a small image, `SET OUTPUT LOG`,
    and a series of HELP requests, including a miss, with `DBG$HELP`
    defined and not.
  - **G. VMS's modules** (local log, Decision 9): HELP/NOPROMPT on
    EXCHANGE, SUBMIT, and ANALYZE, at every level and with `...`.
- **Round 2, at the terminal**: a session you type from a table in the
  README, logged on the console: `Topic?` and `Subtopic?` at each depth,
  `?`, Return, Ctrl/Z, an unknown subtopic, `@lib` at each prompt, `/PAGE`,
  and the same in the debugger.
- **Round 3**: the LBR$ writing routines (subtask 11), a program making
  and changing libraries whose files come back for comparison; and
  whatever rounds 1–2 leave open. Subtask 3 left these: `$STATUS` after
  each of LIBRARIAN's help messages (DUPMODULE, ILLKEYLVL, KEYNAMLNG,
  NOMTCHFOU, RECTYP); a level-1 key of several words; a help file whose
  first key line isn't level 1; a bad key line after a module that was
  good in the same file; `/EXTRACT` of a missing name and of a wildcard
  matching nothing in a text library; and `/DATA=EXPAND`.

## What the probe settled

### Round 1 (2026-11-07)

VMS V7.1 on a VAX 8600 (simh). Logs: `testdata/help90/vax/round1/`
(checked for VMS text first, Decision 9). Parts of the run were spoiled by
the probe's own mistakes (its README lists them); round 1b redoes them.
What the logs settle, in our modules' terms:

**LIBRARY**

- A library's type comes from `/HELP` (or `/TEXT`, `/MACRO`, ...), never
  from the file type. Without one, LIBRARY works on `.OLB`, and
  `LIBRARY/CREATE H90Q.HLB H90C.HLP` made an *object* library named
  `.HLB`, with `%LIBRAR-E-RECTYP, illegal record type (49) in module  in
  file` (two blanks, no module name).
- **Any line whose first character is a digit is a key line.** `10 apples`
  is key `apples` at level 10: `%LIBRAR-E-ILLKEYLVL, illegal key level 10;
  key apples in file; expected 1`. A skipped level (1, then 3) is the same
  message, also "expected 1", so the "expected" number isn't the next legal
  level. The error ends the command. The modules before the bad one are in
  the library; the bad one and those after it aren't.
- A level-1 key longer than the key length is `%LIBRAR-E-KEYNAMLNG, key
  KEY name length illegal in file`, and nothing is inserted.
  `/CREATE=(KEYSIZE:40)` takes it. An object library's default key length
  is 31 (H90Q).
- Text before the first key, a key line with no blank or a tab after the
  digit, a bare digit, and a lowercase level-1 key draw no message. Round
  1b shows how they're stored.
- `/INSERT` of modules already there gives one `%LIBRAR-W-DUPMODULE, module
  M from file F already in L` per module.
- `/DATA=REDUCE` with `/CREATE` is DCL's MAXPARM: data reduction is a
  syntax of its own, with one parameter.
- A module's records are its lines as written ("1 ALPHA", "2 Parameters"),
  but a line holding only blanks is stored as an empty record.

**HELP's layout** (each display starts with an empty line)

- A topic's path is shown first: the level-1 key at column 1, and each
  deeper key on a line of its own, indented 2 blanks per level below 1. A
  topic's text is shifted right by 2 blanks per level: a level-1 line
  written with 3 blanks shows at 5, a level-2 line at 7, and a level-2 line
  written at column 1 (a parameter's name) at 5.
- After the text: two empty lines, `Additional information available:`
  indented as the topic's text, an empty line, and the subtopics, indented
  the same. Subtopics are in the module's order, not sorted, but the
  level-1 topics come in the index's (alphabetical) order. Keys go in cells
  of 11 columns, a longer key taking two or more cells, and a line takes as
  many as the output's width allows: 7 at an 80-column terminal, 10 to 12
  in a file. The exact rule, as the oracle settles it, is subtask 4's.
- **A `/` line in column 1 starts a subtopic** of the topic it's in. This
  holds under any topic, not only `Qualifiers`. The same key is also
  listed and found one level up: ALPHA's list is `Parameters Qualifiers`,
  then Qualifiers' `/BRIEF /COUNT /NOCONFIRM` on a line of their own, then
  `/DIRECT Description`, then Description's `/NOTQUAL` on its own line, then
  `Examples`. `HELP ALPHA /BRIEF`, `HELP ALPHA/BRIEF`, `HELP ALPHA /B`, and
  `HELP ALPHA Qualifiers /BRIEF` all show it, indented for the path used.
  In its own topic's display a `/` line is just text, and no subtopic list
  follows.
- A parameter's name under Parameters (`filespec`) is text, not a key:
  `HELP ALPHA filespec` isn't found.
- **No topic** shows the root library's own HELP module's level-1 text,
  then the root's level-1 topics (not HELP's subtopics). A root with no
  HELP module says `Sorry, no documentation on HELP`, then the list.
  `HELP HELP` is the HELP module with its own subtopics.

**Matching**

- Case doesn't matter. An exact key wins over longer ones it abbreviates
  (ALPHA shows only ALPHA, not ALPHABET). Otherwise every key a word
  abbreviates is shown, one after another (ALP, AL, and A show ALPHA,
  ALPHABET, and ALPS). `*` and `%` match within a key, and `*` alone
  matches every key at its level.
- `KEY...` shows the topic and everything under it.
- `ALPHA,Parameters` is `%LBR-E-INVKEY, invalid key`.
- **Not found:** `Sorry, no documentation on KEYS`, the words in uppercase
  as far as they went, indented one level below the last key found. Then
  the list of what is at that level, or no list where that level has no
  numbered subtopics.

**Libraries**

- `/LIBRARY=H90A` is `SYS$HELP:H90A.HLB` (`SYS$COMMON:[SYSHLP]`), and
  `[]H90A` is `SYS$COMMON:[000000]H90A.HLB`. A library that can't be opened
  is `%HELP-E-OPENIN, error opening FILE as input` and `-RMS-E-FNF, file not
  found`. `/NOLIBRARY/NOUSERLIBRARY` is `%LBR-E-NOHLPLIBS, no default help
  libraries can be opened`.
- The user libraries are listed after the topics as `Additional help
  libraries available (type @name for topics):`, then their file names
  without type, two blanks apart, in process, group, and system order.
  `@H90` chose H90B, the first that begins with it.

**Prompting**

- The prompts are `Topic?` and `ALPHA Subtopic?` (the keys of the
  topic). After a subtopic with no subtopics, the prompt stays at its
  parent. `?` shows the parent again, or at `Topic?` the HELP text and the
  topic list. Return goes up one level, and Return at `Topic?` ends HELP.
- In a procedure, HELP takes its answers from the procedure's data lines
  (SYS$INPUT). Each answer appears in the output, the prompts don't, and
  lines left when HELP ends are SKPDAT.
- At the terminal, a run of empty lines comes before each display. Round 2
  asks whether that is paging or screen clearing.

**The debugger** finds its library through `DBG$HELP`, and looks up the
words under `DEBUG`. It prompts, reading the debugger's own input (from
DBG$INPUT's file, too), so a procedure must answer its prompts.

**LBR$** (round 1's E part, `vax/round1/help90.log` from `#1`): the
values of `LBR$C_`, `HLP$M_`, and the `LBR$_` statuses; INI_CONTROL
returns LBR$_NORMAL and LOOKUP_KEY SS$_NORMAL; the first index is 1;
GET_HEADER's array; records as stored. The messages are LBR's own
(`%LBR-E-KEYNOTFND, key not found`). `LBR$_NOHLPLIS`, the manual's name,
isn't defined (round 1b found `LBR$_NOHLPLIBS` isn't either).

### Round 1b (2026-11-07)

The same procedure with round 1's mistakes fixed, run on DUA1. Logs:
`testdata/help90/vax/round1b/` (checked; `help90f.log` and `dbgv.dlg`
local). Two of round 1's findings are corrected here. One mistake was left:
the DEEP module nests each level under the sibling before it, so its deep
cases found nothing past L2. Round 1c has a chain that is nested correctly.

**LIBRARY**

- **A key line is a level number, then a blank.** `2Nospace`, a bare `2`,
  and `2`+tab+key are stored as text lines of the topic before them.
  `0 zero` is text too. `10 apples` is a key at level 10 (round 1). This
  corrects round 1's "any line whose first character is a digit".
- Text before the first key line is dropped without a message (BADTEXT).
- A help library's default key length is **15** (Max. key length: 15),
  not 31 as for an object library. A longer level-1 key is `KEYNAMLNG`, and
  `/CREATE=(KEYSIZE:40)` takes it. A text library's default is 39, a macro
  library's 31.
- The module's name keeps the case of its key (`MixedCase`, `lower`),
  and LIST shows it so. `/EXTRACT=alpha` and `=MIXEDCASE` both find it
  (case is ignored).
- **The error doesn't end the command** (this corrects round 1). After
  SKIP's ILLKEYLVL, `LIBRARY/CREATE/HELP H90X8 H90C,SKIP,H90G` went on to
  H90G. VMS 7.1 then failed to insert GONLY: an `INSERTERR` message with
  garbage for the name, `-RMS-E-EOF`, and a library LIST can't read past
  (`MHDERR`, `INVRFA`). govax will go on to the next file *correctly*.
  That VMS failure is a VMS bug, not behavior to copy, so it is unconfirmed
  how a correct librarian would do it.
- `/INSERT` of modules already there is `DUPMODULE`, one per module, and
  leaves them. `/REPLACE` replaces or adds. `/DELETE=NOSUCH` is
  `LOOKUPERR` with `-LBR-E-KEYNOTFND`. Each changing command adds to
  "Library history records". LIST/FULL says "inserted" with the date for a
  replaced module too.
- `/EXTRACT` writes each module's lines as stored: key lines, empty
  lines, tabs, and long lines kept.
- `LIBRARY/DATA=REDUCE/HELP` makes a new version, which says "Library is
  in DCX data reduced format". HELP reads it as it reads any other library.

**HELP's matching**

- **An exact key wins only at level 1** (the library's index). Within a
  topic, a word matches every subtopic it abbreviates, so `QUALS Qual`
  shows both Qual and Qualifiers, and `SAME Example` both Example and
  Examples.
- A topic list shows keys in file order and in the case of the file. A
  level-1 topic's heading is its key in uppercase (`MIXEDCASE`). A key
  longer than 10 characters takes 22 columns (two cells). Round 2
  refines the rule: a cell is the key and one blank, rounded up to a
  multiple of 11. Output to a file is 132 columns wide (12 cells).
- The request is split into words at blanks and before each `/`.
  `ODD Two Words` matches `Two Words` by its first word, then looks for
  WORDS under it. `ODD Slash/Inside` looks for `/INSIDE` under
  `Slash/Inside`. The "Sorry" line shows the words in uppercase and the
  slash split off (`SLASH /INSIDE`).
- `KEY...` must touch its key: `ALPHA ...` is "Sorry, no documentation on
  ALPHA ...". `DEEP L2...`, on a topic with no subtopics, is also
  "Sorry" (round 1c asks again).
- `/[NO]FLAG` is matched only as written. `/FLAG` and `/NOFLAG` aren't
  found.
- A `/` line indented by a blank is text (`/INDENTED`).
- A subtopic with no text and no subtopics (`EMPTY Child`) displays its
  heading, then `Sorry, no documentation on EMPTY CHILD`, then the *root*
  library's topic list.
- A word that matches nothing at level 1 shows "Sorry" with no heading,
  then the topic list. A word that matches nothing below a topic shows the
  topic's heading, then "Sorry", then the topic's subtopics.
- Text is shifted right by 2 blanks for each level, including lines
  stored in column 1. Tabs are kept.

**Prompting**

- With SYS$INPUT a file (`DEFINE/USER_MODE SYS$INPUT H90DIR:RESP.TXT`),
  HELP takes its answers from the file and shows no prompts. Its end of
  file ends HELP.
- **At the terminal** (the session typed after round 1b; the README's
  table went wrong at row 11):
  - `Qualifiers` at `ALPHA Subtopic?` shows the Qualifiers topic with each
    `/` subtopic's text inline. The prompt stays `ALPHA Subtopic?`, since
    the `/` keys are listed and prompted for at ALPHA's level.
  - An unknown subtopic shows ALPHA's heading, the "Sorry" line, and
    ALPHA's list, and the prompt stays.
  - Return at `ALPHA Subtopic?` goes to `Topic?`, and Return there ends
    HELP, as round 1 found.
  - The runs of empty lines before each display were 27 to 46 lines long.
    Round 2 asks why.

**The debugger** (`dbgh.dlg`): the prompts don't reach the log. Topic
cells follow HELP's rule (`Probe_Concept` takes 22 columns). `HELP EX`
shows both EXAMINE and EXIT. `HELP DEBUG` and `HELP EXTRA` are "Sorry,
no documentation on DEBUG DEBUG" and "on DEBUG EXTRA": the debugger
looks up the words *under* DEBUG only, and EXTRA, a second level-1 module,
is never reached.

**LBR$** (`#1` to `#79`, all run): the values, the statuses, and what each
routine returned, for subtask 10 to read in full. Neither `LBR$_NOHLPLIS` nor
`LBR$_NOHLPLIBS` is a symbol VMS 7.1's LINK finds (`UDFSYM`), and no case
returned such a status (#61, with libraries defined, returned
SS$_NORMAL). govax won't define the status until a VMS run shows one.

### Round 1c and round 2 (2026-11-07)

Logs: `testdata/help90/vax/round2/` (checked). `help90c.log` is round 1c's
procedure. `help90-term.log` is the terminal session, as simh logged it
(escape sequences and all), cut before simh's own help text at its end.

**Levels and wildcards** (DEEPER, a chain to 9):

- All nine levels work, each heading indented 2 more, and text 3 past its
  heading. A tenth word is VMS 7.1's quirk: `DEEPER L2 ... L9 L10` (or
  `... L9 *`) is "Sorry, no documentation on DEEPER L2 L3 L4 L5 L6 L7 L8
  L9 DEEPER". The tenth word is reported as the *first* one, as though HELP
  kept nine words and the tenth wrapped around. govax will copy the
  message (unconfirmed why).
- **`...` is taken only on the first word** (the level-1 topic).
  `DEEPER...` shows DEEPER and every subtopic below it in file order, each
  with its whole heading path and no topic lists. `NOSUB...` is just
  NOSUB. On any later word `...` is part of the word, so `DEEPER L2...`,
  `S2...`, `L2 L3 ... L8...`, `L2 *...`, and `ALPHA Parameters...` are
  "Sorry" at the level where the word failed. This settles round 1b's
  question.
- `*` matches any key at its level (`DEEPER * L3` works). `%` matches
  exactly one character, so `DEEPER % L3` fails (the keys are `L2` and
  `S2`) and `DEEPER L% L%` works. A shared abbreviation (`DEEPER L`, `L2
  L`) that matches one key shows just that key.
- In a procedure, `HELP/PROMPT` takes each data line as an answer and
  echoes it in the output, followed by an empty line. Unread lines are
  SKPDAT.

**The terminal** (`SHOW TERMINAL`: OPA0:, a VT100, page 52, ANSI_CRT,
Scope):

- **The empty lines were a screen clear.** On a video terminal with
  `/PAGE` (the default there), HELP starts by sending `ESC ) 0`, a
  scrolling region of the page's height (`ESC [1;52r`), and `ESC [;H
  ESC [2J` (home, clear). It clears again before each display, and sends
  `ESC [m` at the end. `/NOPAGE` sends none of these; each display just
  starts with an empty line.
- **The prompts name the whole path:** `DEEPER L2 L3 L4 Subtopic?`.
  Return goes up one level at a time to `Topic?`, then ends HELP.
  Starting partway (`HY DEEPER L2 L3`) prompts at that topic and climbs
  the same way. After a leaf (S5) the prompt stays at its parent.
- **`?` is read at once, with no Return** (the author saw this at the
  terminal; the log shows no line end after it). It redisplays the
  prompt's topic, with an extra empty line at the top.
- **Ctrl/Z** at a subtopic prompt shows ` Exit ` in reverse video (`ESC 7
  ESC [7m Exit ESC [m ESC 8`) and returns to `$` at once.
- **`@lib` at `Topic?`** goes into that library. H90B has no HELP module,
  so it shows "Sorry, no documentation on HELP", then H90B's topics, with
  the prompt `@H90B Topic?`. Its subtopic prompts are `@H90B BONLY
  Subtopic?`. Return at `@H90B Topic?` returns to the first library's
  `Topic?`, and Return there ends HELP.
- An unknown topic at the start (`HP NOSUCH`) shows "Sorry" and the topic
  list, then `Topic?`. An unknown subtopic shows its parent's heading,
  "Sorry", and the parent's list, and the prompt stays.
- `/PAGE` on TEXTY, a topic shorter than the page, didn't stop for more.
  Its 104-character line was wrapped by the terminal, not by HELP. HELP
  doesn't wrap text.
- **Cells at 132** (WIDE): a cell is the key plus one blank, rounded up
  to a multiple of 11 (a 10-character key 11 columns, 11 to 21 characters
  22, 22 to 32 characters 33). A key goes on the line if its *text*, not
  its cell, fits in the width: the first line ends with `Lxxxxxxxxxx` (11
  characters) at column 123, and the second with an 18-character key
  ending at column 130.

**The debugger at the terminal:**

- It doesn't clear the screen.
- Its subtopic prompts start with DEBUG (`DEBUG SET BREAK Subtopic?`) and
  climb to `Topic?`.
- A miss at `DBG> HELP` (`HELP EST`) shows the "Sorry" under DEBUG and
  DEBUG's list, then prompts `Topic?`. **At that `Topic?` the debugger
  looks at the library's level-1 keys** (`BREAK` was "Sorry", with the
  list `DEBUG EXTRA`), not under DEBUG.

## Subtasks

1. The survey and plan (this document), reviewed by the author (done
   2026-11-07).
2. The probe: `testdata/help90`'s modules, `gen.go`, procedures, and
   `lbr90.mar`; rounds 1 and 2 run on VMS; "What the probe settled" written
   into this doc (done 2026-11-07; round 3 belongs to subtask 11).
3. LIBRARY/HELP: `internal/lbr`'s help modules (input rules, key length,
   DCX) and the command's `/HELP`, held to VMS's libraries and messages
   (`TestHelp90LibraryOracle`) (done 2026-11-07; making a data-reduced
   library is left for later, see the log).
4. The engine's tree, lookup, and display: `internal/help` rewritten
   (`ParseModule`, `Lookup`, `Render`), held to round 1's B part
   (`TestHelp90Oracle`), and to G's local log when it's present
   (done 2026-11-07).
5. The interactive session: prompts and responses, held to round 2's
   terminal log and round 1's D part (done 2026-11-07).
6. DCL's HELP: the grammar's qualifiers, the library search and
   `HLP$LIBRARY`, `/OUTPUT`, `/PAGE`, HELP in procedures and subprocesses;
   round 1's C part (done 2026-11-07).
7. The libraries in the build: `go generate`, the committed and embedded
   HELPLIB.HLB and DBG$HELP.HLB, the system volume's entries, sources, and
   BUILDHELP.COM, the refresh by module (Decision 7), and Decision 10's
   tests. The help at this point is a mechanical conversion of today's
   files, so nothing is lost while subtasks 8 and 9 rewrite it.
8. The DCL help rewritten as modules, in parts: (a) the file commands; (b)
   DCL itself (symbols, procedures, flow, lexicals, file I/O); (c) the
   programming tools (MACRO, LINK, LIBRARY, ANALYZE, FORTRAN, RUN); (d)
   govax's own topics and the machine's commands, HELP, INSTRUCTIONS, and
   HINTS.
9. The debugger's help rewritten as the DEBUG module, and the debugger's
   HELP on it, held to round 1's F part.
10. LBRSHR: `vmsdef`'s vector, messages, and definitions;
    `internal/lbrrtl`'s reading routines; LINK and RUN finding them; held
    to round 1's E part (`TestLBR90Oracle`), in both paging modes.
11. LBRSHR's writing routines (Decision 8): a program creating a library,
    inserting and replacing modules and keys, deleting them, and writing
    history records, through `internal/lbr`'s Builder; held to a probe
    round of their own (round 3), the libraries VMS's to the byte.
12. The close-out: MANUAL.md (HELP, LIBRARY/HELP, the system volume's
    table, a section on writing help modules), FORTRAN-INTERNALS.md and
    other docs that mention HELP, CLAUDE.md, PLAN.md, and this doc's "Where
    things stand".

## Progress log

- **2026-11-07** — Survey and plan written (subtask 1): govax's help and
  LIBRARY today; VMS 7.1's HELPLIB.HLB and DBG$HELP.HLB headers and the
  shapes of the EXCHANGE, SUBMIT, ANALYZE, and DEBUG modules (extracted to a
  scratch directory with govax's LIBRARY, none in the repository); HELP and
  the LBR$ help routines from the DCL Dictionary and Utility Routines
  Manual.
- **2026-11-07** — The author reviewed and accepted the plan ("The
  review"): VMS's DBG$HELP.HLB; the refresh by module; HELP's screen and
  message qualifiers later; LBR$'s writing routines as subtask 11; VMS's
  modules' logs local. Subtask 1 done.
- **2026-11-07** — Subtask 2, the probe prepared: `testdata/help90`
  (README.md). `gen.go` writes the probe's modules (our own text, each line
  naming its topic), `help90.com` (parts A, B with 126 HELP requests, B2,
  C, E, then D in `help90d.com`), `help90v.com` (F and G, local only),
  `lbr90.mar` (79 cases of the LBR$ routines), and the exchange scripts.
  `copyout.cmd` brings everything to `testdata/disks/help90-vms/`, and a
  log goes to `vax/` only after a check that it holds none of VMS's text
  (Decision 9). `lbr90.mar` was rehearsed on govax against stand-in LBR$
  routines: it assembles with govax's MACRO, runs every case, and its
  handler reports a fault and goes on. The README has round 2's terminal
  session. Waiting for the VAX run.
- **2026-11-07** — Round 1 run on VMS V7.1. The logs were checked for VMS
  text by a script listing every line not from our modules, commands, or
  messages (HELP's framing only). `help90.log`, `h90.bck`, `dbgh.dlg`, and
  the terminal session's start are in `testdata/help90/vax/round1/`;
  `help90v.log` and `dbgv.dlg` are local. "What the probe settled" has
  round 1's findings. Four mistakes of the probe's own spoiled the LIBRARY
  part and the modules after TEXTY, and the debugger sessions. Round 1b
  (`gen.go` fixed, `help90f.com` for part F) and round 2's terminal table
  are ready.
- **2026-11-07** — Round 1b run on VMS V7.1 (on DUA1). Every part ran.
  The logs were checked as round 1's were, and `help90.log`, `h90.bck`, and
  `dbgh.dlg` are in `testdata/help90/vax/round1b/`. "What the probe
  settled" has round 1b's findings, which correct two of round 1's (the
  key line's form, and LIBRARY going on after a bad file). DEEP was nested
  wrongly, and the README's terminal table ended HELP before row 11. Round
  1c (`deeper.hlp`, `help90c.com`, a new exchange volume) and a revised
  round 2 table, each row starting at `$`, are ready.
- **2026-11-07** — Round 1c and round 2 run on VMS V7.1. Both logs were
  checked and are in `testdata/help90/vax/round2/` (the terminal log cut
  before simh's own help text). "What the probe settled" has their
  findings. The empty lines at the terminal were VT100 screen clears
  from `/PAGE`. `...` is taken only on the first word. The prompts name
  the whole path, `?` is read without a Return, and the debugger's
  `Topic?` looks at the library's level-1 keys. Subtask 2 done. Round 3
  (the LBR$ writing routines) belongs to subtask 11.
- **2026-11-07** — Subtask 3, LIBRARY/HELP. `internal/lbr/help.go` is the
  help file's rules: `HelpKeyLine` (a level number, a blank, and the key,
  which is the first word after the level: ILLKEYLVL names `apples` for
  `10 apples is a line ...`), and `Builder.HelpModules`, which splits a
  file into modules (text before the first key dropped, a line of blanks
  an empty record) and stops at a bad key line with a `HelpError` (the
  modules before it, and the module it was in so far). The Builder now
  keeps its modules by their names as the index compares them (`fold`:
  upper-cased in a help library, so ALPHA and alpha are one module), its
  data chain as a list of entries, and `Abandon`: VMS writes a module's
  records as it reads them, so a bad key line leaves the module's header
  (no references) and its records before that line in the data, with no
  end of module and no index entry, which Bytes writes too (H90X2, H90X6).
  `TestHelp90NewLibraries` makes each of the probe's sixteen one-command
  help libraries and matches VMS's byte for byte (H90A, H90B to H90S, H90D,
  H90L and H90K with their key lengths, the malformed modules' H90X1 to
  H90X7); `TestHelp90OtherLibraries` matches H90Q (an empty object
  library), H90T, and H90M; `TestHelp90DataReduced` reads H90R;2.
  DCL's LIBRARY takes `/HELP` (grammar id 1423; the type `.HLB`, input
  `.HLP`), with LIBRARIAN's messages for a help library: ILLKEYLVL (always
  "expected 1", as VMS 7.1 says) and KEYNAMLNG end that file's reading but
  not the command, as VMS 7.1 went on (`library.report`, the command's
  status the worst message's); DUPMODULE; LOOKUPERR with KEYNOTFND for
  `/DELETE` of a missing name; NOMTCHFOU for an `/EXTRACT` wildcard
  matching nothing. An object library given a file of text is RECTYP,
  naming the record's type and no module, and goes on to write the empty
  library (H90Q). `/EXTRACT`'s `/OUTPUT=SYS$OUTPUT` shows a text module's
  records. `TestHelp90LibraryOracle` runs part A of `help90.com` on a
  volume like the exchange volume and holds its log to VMS's line for
  line, with for83's masks (times, the librarian's name, the count of
  history records), but for three commands: `LIBRARY/DATA=REDUCE` (govax
  reads a data-reduced library and doesn't make one: matching VMS's DCX
  map would need its tree-building rules, which only its output could
  teach, so it waits for a phase that needs it), H90X8 (VMS 7.1's failure
  to insert GONLY after SKIP is a librarian bug; `TestHelp90LibraryGoesOn`
  holds govax's own way, both modules in), and DIRECTORY/SIZE=ALL (H90E,
  changed five times, is laid out afresh by govax's LIBRARY). Unconfirmed,
  and asked of round 3: the condition values of these messages (LIBRAR's
  facility and severity, with no message number, `librarWarning` and
  `librarError`), a level-1 key of several words (its first word names the
  module), a first key line that isn't level 1 (ILLKEYLVL, no module
  abandoned), and NOMTCHFOU in libraries other than help ones. The
  one-shot `govax library` has no `--help` (the name is the command line's
  own); it comes with subtask 7's work on the libraries in the build if it's
  wanted. MANUAL.md's section 10 and HELP LIBRARY describe `/HELP`.
  Subtask 3 done.
- **2026-11-07** — Subtask 4, the engine. `internal/help` is VMS's HELP
  beside the old `$`-format reader (`oldformat.go`, which the console and
  debugger use until subtask 7 removes it). `module.go`'s `ParseModule`
  makes a module's tree of `Topic`s: a key line is a level, a blank *or a
  tab*, and the rest of the line (HELP takes the tab LIBRARY doesn't:
  NOBLANK's Tabbed), and a `/` line in column 1 is a qualifier line's
  topic (`Slashes`) inside the topic whose text it's in, its text running
  to the next `/` line or key line. A topic lists its numbered subtopics,
  each followed by the `/` lines in its text (`list`); a word starting
  with `/` also names the topic's own `/` lines and its subtopics'
  (`candidates`). `library.go` is the `Library` interface over
  `lbr.Library` (`NewLBRLibrary`) and help text in memory
  (`NewMemoryLibrary`); `request.go` is `Split` and the matching
  (abbreviations, `*`, `%`, case); `lookup.go` is `Lookup`, a list of
  `Display`s; `render.go` is `Render` at a width. `TestHelp90Oracle`
  holds all 167 HELP requests of round 1b's part B and round 1c to VMS's
  logs line for line (the three that name a library that isn't there or
  isn't a help library are subtask 6's). `TestHelp90VMSModules` holds the
  28 requests of round 1's part G on VMS's own EXCHANGE, SUBMIT, and
  ANALYZE modules to its local log, reading HELPLIB.HLB copied (COPY/BINARY)
  from the local system disk into `testdata/disks/help90-vms/`; it skips
  without them. The VAX that ran the probe has modules that disk's
  HELPLIB.HLB lacks (DECnet-Plus, ON, RECALL, ...), so the two requests
  listing every module are compared up to their lists. What the logs
  settled beyond "What the probe settled":
  - **The list's margin.** A key goes on a line if its text ends by column
    width-3: at 132 a key ending at 129 stays and one ending at 130 or
    later starts a line (ANALYZE /AUDIT /SELECT's list at depth 3, and
    WIDE). That it's width-3 at 80 columns is unconfirmed.
  - **Trailing blanks** are dropped from a text line, but not its last
    character: VMS's modules have records of one blank, shown as the
    shift and a blank.
  - **The display's shape**: each heading after an empty line; then an
    empty line and the text; then, with subtopics, two empty lines, the
    "Additional information available:" line, an empty line, and the
    list; then an empty line. "Sorry" follows the last heading with no
    empty line between. A topic found with no text and no subtopics (EMPTY
    Child) is shown, then "Sorry" with the request's words and the root
    library's list, as if nothing had been found.
  - **Where a qualifier line is shown**: one level below the numbered
    topics last matched, which VMS keeps from one match to the next, so in
    `HELP ALPHA /*` /NOTQUAL (in Description) is shown under /DIRECT,
    matched just before it. govax copies this.
  - **The level-1 heading** is the word upper-cased when it names a module
    exactly (MIXEDCASE, LOWER), and the module's name otherwise (`*`, an
    abbreviation).
  - **The words** of the "Sorry" line are all the request's, as split,
    upper-cased; "..." is taken off the first word; past nine words each
    word is the one nine before it (only the tenth is in the logs).

  Unconfirmed, and chosen: a word that is only `(` is dropped (`ODD (`
  showed ODD); a first word with a comma is INVKEY, and which other
  characters are is unknown; where several wildcard paths fail, the
  "Sorry" is the deepest, the first of equals; a request going on past a
  qualifier line's topic is "Sorry" below it; qualifier lines two levels
  down aren't found. Subtask 4 done.
- **2026-11-07** — Subtask 5, the interactive session. `session.go`'s
  `Session` answers a request, then prompts and answers each response
  over a `Read` and a `Write` its caller gives it (a `BeforeDisplay` hook
  for `/PAGE`'s screen clears, an `Open` for `@library`). Its place is a
  library and a path of topics (`place`, `step`); the prompt names the
  path (`DEEPER L2 L3 Subtopic?`, `@H90B BONLY Subtopic?`). Keys at a
  `Subtopic?` prompt are looked up below its topic (`lookupFrom`, which
  `Lookup` now uses too: "..." is part of a word there, and "Sorry" names
  the path and the words); at `Topic?` they're a new request, `@spec`
  first moving to that library. `?` shows the prompt's topic again (at
  `Topic?`, HELP's own topic and the list); an empty response goes up a
  level, from an `@` library's `Topic?` back to the main library's, and
  from that out of HELP; end of input ends HELP at once. Where it
  prompts next (`next`, from the topics a `Display` now carries in
  `path`): at the topic shown if it has numbered subtopics, else at its
  parent (a qualifier line's too); after a miss where it was, but after a
  first request that misses at `Topic?` (VMS's debugger's `HELP EST`).
  The debugger is a Session whose words start with `DEBUG`.
  `TestHelp90Prompting` holds it to part D's nine HELP/PROMPT commands in
  round 1b's log and round 1c's chain, answered by the procedures' data
  lines (SET VERIFY's echo and SKPDAT played by the test) and by
  `resp.txt`; `TestHelp90Terminal` to round 2's ten sessions at the
  terminal, the debugger's two among them, with the escape sequences
  taken out and long lines wrapped as the terminal driver wraps them.
  What the terminal does is the caller's: the echo, `?` read without a
  Return, Ctrl/Z's ` Exit `, and `/PAGE`'s clears (subtask 6). Left to
  subtask 6: HELP/OUTPUT (part D's last command), the library search and
  its messages. Unconfirmed, and chosen: after several topics are shown
  HELP prompts at the topics their paths share (the probe's only case
  was `*` below ALPHA, which stayed at ALPHA; after `ALPHA...` this
  prompts at ALPHA); `@spec` at a `Subtopic?` prompt is a key, not a
  library; an empty response in a second `@` library goes back to the
  main one; INVKEY at `Topic?` leaves the prompt there; nine words'
  wrap isn't applied below a prompt's path. Subtask 5 done.
- **2026-11-07** — Subtask 6, DCL's HELP. `console.dcl`'s HELP takes
  VMS's qualifiers (ids 1612–1615, 1632–1639; `/LIBRARY` has a default so
  that `/NOLIBRARY` parses), and `internal/dclcmd/help.go` is the command:
  the main library (`/LIBRARY`, SYS$HELP:HELPLIB.HLB by default, its name
  parsed with SYS$HELP:.HLB through `rms.Session.FindImageFile`, or a host
  path), the user libraries (`HLP$LIBRARY`, `_1`, ... in the process,
  group, and system tables by `/USERLIBRARY`, each table's numbers ending
  at the first missing name, a library that can't be opened passed over),
  `/NOLIBRARY`'s first user library as the main one, `/OUTPUT` (no value
  is SYS$OUTPUT, `[]` is HELP.LIS, `/NOOUTPUT` nothing; a file is 132
  wide), `/PAGE` (the screen cleared before each display, `SCROLL`
  leaving it, a stop after a screenful), and OPENIN and NOHLPLIBS as
  VMS words them. `internal/help`'s Session gained `Users` and `LibList`:
  a first word that names no level-1 topic is looked for in the main
  library and then each user library (`searchOrder`), the first library
  that has it answering and the prompts going on in it until they climb
  back to `Topic?`; "@name" names a user library by the start of its name
  before `Open` is tried, and one that can't be found is a "Sorry" with
  every word; the user libraries' names follow a list of a library's
  topics (`Display.Libraries`, "Additional help libraries available").
  HELP's responses are SYS$INPUT's (`dclshell`'s `ReadHelpResponse`): a
  procedure's data lines, which SET VERIFY shows, a file SYS$INPUT names
  (`corevms.Environment.ReadInputRecord`, closed after HELP by
  `CloseInputFile`), or a line typed after the prompt; HELP is an image
  to DCL, so lines it leaves are SKPDAT and user-mode names go after it.
  `$STATUS` after HELP is SS$_NORMAL, as VMS 7.1's. `/SEARCH`, `/EXACT`,
  `/HIGHLIGHT`, `/WRAP`, `/MESSAGE`, and `/PAGE=SAVE` are
  `%DCL-W-HELPNOTIMPL`. Until subtask 7, HELP with no `/LIBRARY` and no
  SYS$HELP:HELPLIB.HLB shows the old built-in help (`ErrNoMainLibrary`).
  `TestHelp90DCLOracle` runs part B's library-name requests, B2, C, and D
  (HELP90D.COM whole, its data lines and RESP.TXT) on govax's DCL with
  part A's libraries, and holds every line to round 1b's log (file IDs
  and times masked) but `/LIBRARY=[]H90A`, which on VMS went to SYS$HELP's
  rooted directory; it runs the procedure at a 132-column terminal, since
  govax's DCL writes its messages at the terminal rather than in a
  procedure's `/OUTPUT` file. `TestHelp90DCLTerminal`, `...Messages`,
  `...Page`, `...BuiltIn`, and `...Subprocess` cover the prompts at the
  terminal and Ctrl/Z, the messages, `/PAGE`, the fallback, and HELP in a
  SPAWNed DCL with the parent's HLP$LIBRARY. Unconfirmed, and chosen:
  OPENIN's and NOHLPLIBS's `$STATUS` (the shared message's code with no
  facility, LBR's facility with no message number); OPENIN with LBR's
  NOTHLPLIB for a file that isn't a help library; the numbers going on
  past a user library that can't be opened; the library names not
  wrapped; whether a topic found in a user library prompts there; `/PAGE`'s
  stop prompt (paging's "Press RETURN to continue"), and leaving out the
  scrolling region and character set VMS's HELP sets; `?` read with a
  Return, and Ctrl/Z with no ` Exit `, at govax's terminal. Subtask 6
  done.
