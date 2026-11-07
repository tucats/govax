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
  whatever rounds 1–2 leave open.

## Subtasks

1. The survey and plan (this document), reviewed by the author (done
   2026-11-07).
2. The probe: `testdata/help90`'s modules, `gen.go`, procedures, and
   `lbr90.mar`; rounds 1 and 2 run on VMS; "What the probe settled" written
   into this doc.
3. LIBRARY/HELP: `internal/lbr`'s help modules (input rules, key length,
   DCX) and the command's `/HELP`, held to VMS's libraries and messages
   (`TestHelp90LibraryOracle`).
4. The engine's tree, lookup, and display: `internal/help` rewritten
   (`ParseModule`, `Lookup`, `Render`), held to round 1's B part
   (`TestHelp90Oracle`), and to G's local log when it's present.
5. The interactive session: prompts and responses, held to round 2's
   terminal log and round 1's D part.
6. DCL's HELP: the grammar's qualifiers, the library search and
   `HLP$LIBRARY`, `/OUTPUT`, `/PAGE`, HELP in procedures and subprocesses;
   round 1's C part.
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
