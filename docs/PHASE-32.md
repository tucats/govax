# Phase 32 — govax's own RMS macros, written clean-room

**Status:** planned (2026-09-30), not started.

## Goal

govax's own STARLET.MLB (`internal/bootdata/files/starlet.mar`) gains the
RMS macros MACRO-32 programs use, so that they assemble with no licensed
STARLET.MLB:

- the control block initialization macros (`$FAB`, `$RAB`, `$NAM`, and
  the XABs);
- their `_STORE` forms;
- the symbol definition macros (`$FABDEF`, `$RMSDEF`, ...);
- the RMS service macros (`$OPEN`, `$GET`, ...).

The macros are written from DIGITAL's published documentation and from
observing what real VAX MACRO produces. They are never written from
STARLET.MLB's macro text. The acceptance test is Phase 28's own RMS
fixtures: `rmscopy.mar` and `fabalign.mar`, assembled with govax's
STARLET, give the objects real MACRO gave (`testdata/mar/macros/vax/`),
byte for byte.

## The clean-room method

### What the macros may be written from

1. **The OpenVMS Record Management Services Reference Manual** (2001, the
   VMS 7.3 era; `~/Documents/Technical Doc/VMS/rms_manual.pdf`), plus the
   VAX-11 2.0 RMS manual for history.
   - Appendix A documents every macro's format, keywords, and values.
   - Appendix B gives the MACRO rules: argument forms, alignment, the store
     macros' R0 use and `MOVAx`/`MOVx` forms, and the service macros'
     `CALLG (AP)` and `PUSHAL ... CALLS` expansions.
   - Chapters 4 to 19 give each block's fields, sizes, and defaults.
2. **Values and layouts, which are facts:** govax's `vmsdef.Symbols`,
   `FABFields`, and `RABFields`, and the NAM$ and XAB$ values in
   STARLET.OLB's definition modules. Those are numbers captured by `gen`
   (Phase 31), not macro text.
3. **Real MACRO's output for govax-written sources** (subtask 3): objects
   and `ANALYZE/OBJECT` reports of test programs that call each macro. These
   settle what the manual doesn't, such as the order fields are stored in,
   or a field stored twice (as `overwrite.go` already handles for `$FAB`'s
   FNA). Observing a product's output to interoperate with it copies none
   of its text.

### What they may not be written from

- **STARLET.MLB's text:** the library itself, `LIBRARY/EXTRACT` of it, or
  anything printed from it.
- **Macro expansion text:** real MACRO listings of the macros'
  expansions. The oracle runs (subtask 3) make objects and analyses only,
  and any listing is made with `.NOSHOW ME,MEB`.

### The barrier

Permission rules alone don't stop a Claude session reading a file here:
`Bash` is allowed wholesale, so `cat` or `strings` would get around a
`Read` rule. So the barrier is a **PreToolUse hook**
(`.claude/hooks/cleanroom.sh`, set up in `.claude/settings.json` so it's
committed and reviewable). It refuses any Read, Grep, Glob, or Bash call
whose arguments name one of these:

- `starlet.mlb` (any case, any directory);
- `reference/vms/`;
- a `.ext` extract of a system library;
- a real-MACRO listing not yet audited (see subtask 0).

There is one exception: `go test`, `go build`, and `go vet` pass through,
because the local-only comparison tests read the real STARLET.MLB. Those
tests must report differences as **object data** (records, TIR commands,
bytes), never as expanded macro text. Subtask 0 audits the existing tests'
failure messages to make sure of that.

### Earlier exposure, recorded honestly

Phase 28 compared govax's macro facility against the real STARLET.MLB,
and some of what it learned is in the tree:

- names of STARLET's internal helper macros, and descriptions of how
  its definition and block macros are built;
- their alignment check and keyword-error behavior;
- one quoted source line in `fabalign.mar`'s comment.

The behavior is observable in real MACRO's output (the GENINFO message,
the error text, the double store in the object), so it may stay as
behavior. The quoted text goes (subtask 0). This phase's macros cite the
manual section or oracle fixture that each detail comes from.

## Scope

VAX VMS 7.3 only. `$RAB64` and `$NAML` are Alpha-only and are left out.

| Kind | Macros | Source of truth |
| ---- | ------ | --------------- |
| Initialization | `$FAB`, `$RAB`, `$NAM`, `$XABALL`, `$XABDAT`, `$XABFHC`, `$XABKEY`, `$XABPRO`, `$XABRDT`, `$XABSUM`, `$XABTRM`, and `$XABITM` if VAX 7.3 has it | Appendix A, chapters 4–19, oracle |
| Store | each of the above's `_STORE` form | Appendix A, B.2.3, oracle |
| Definition | `$FABDEF`, `$RABDEF`, `$NAMDEF`, `$XABDEF` and each `$XABxxxDEF`, `$RMSDEF`; also `$SSDEF`, `$IODEF`, `$JPIDEF`, `$DVIDEF`, `$SYIDEF`, `$LNMDEF`, and the other families `Symbols` holds | generated from `vmsdef.Symbols`; local vs. global form from the oracle |
| Service | `$CLOSE`, `$CONNECT`, `$CREATE`, `$DELETE`, `$DISCONNECT`, `$DISPLAY`, `$ENTER`, `$ERASE`, `$EXTEND`, `$FIND`, `$FLUSH`, `$FREE`, `$GET`, `$NXTVOL`, `$OPEN`, `$PARSE`, `$PUT`, `$READ`, `$RELEASE`, `$REMOVE`, `$RENAME`, `$REWIND`, `$SEARCH`, `$SPACE`, `$TRUNCATE`, `$UPDATE`, `$WAIT`, `$WRITE` | B.2.4, Part III, oracle |

This phase is about assembling. Running a program that uses NAM or XAB
blocks, or a service govax's RMS doesn't implement ($PARSE, $SEARCH,
$DISPLAY, ...), still fails at run time. That's the next phase.

## Subtasks

Each subtask ends with `go build`/`go vet`/`go test` and golangci-lint
clean, and a commit. `build -i` follows each one that changes behavior.

0. **Barrier and provenance.**
   - Add the hook and `.claude/settings.json`, and show that it refuses
     each kind of access, and that `go test` still runs.
   - Reword the tree's quotations of STARLET text (`fabalign.mar`'s
     comment; `conditional.go`, `message.go`, and `message_test.go`'s
     descriptions) as observed behavior.
   - The author audits the three real-MACRO listings that contain `$$`
     text (`rmscopy.lis`, `fabalign.lis`, `qiow.lis`). Claude doesn't read
     them. If they hold STARLET expansion lines, regenerate them with
     `.NOSHOW ME,MEB` or drop them.
   - Audit the comparison tests' failure messages.

   *Verify:* the hook's refusals, shown in the log; the tests pass.
1. **Values.** Use `gen -into symbols -olb starlet.olb` for the FAB$,
   RAB$, NAM$, and XAB$ families, so field offsets and codes join
   `Symbols`.
   - Cross-check the FAB$/RAB$ offsets against `FABFields`/`RABFields`.
   - Update `.RMSDEF`, which then gets its offsets from `Symbols` too, and
     `TestSymbolNames_rmsFamilies`.
   - STARLET's definition modules lack some codes, such as XAB$C_KEY,
     XAB$C_SUM, and XAB$C_ITM. Take them from the manual's tables and
     confirm them with the oracle.

   *Verify:* cross-check tests.
2. **The specification: `docs/RMS-MACROS.md`**, written from the manual.
   - For each macro: its keywords, and for each keyword the field, size,
     and encoding (which `FAB$M_`/`FAB$C_` value a keyword such as `GET`
     or `SEQ` names), the default, and the argument forms.
   - Each entry cites its manual section, and details the manual doesn't
     settle are marked for the oracle.

   The macros are written from this document. *Verify:* review.
3. **The oracle.** (Needs the author's VAX, as Phase 28's fixtures did.)
   - govax-written `.MAR` programs call every macro with each keyword and
     argument form:
     - defaults only;
     - every keyword;
     - multi-option fields;
     - addresses, values, and registers for the `_STORE` forms;
     - every service macro in both formats;
     - a misaligned block.
   - `exchange.cmd` builds the exchange volume. A `.COM` assembles each
     program with `/NOLIST`, runs `ANALYZE/OBJECT`, and links and runs
     `rmscopy`.
   - Results come back into `testdata/mar/rms/vax/`.

   This can run while subtask 2 is written. *Verify:* the fixtures and
   their results are committed, with a README.
4. **Definition macros.** `mkstarlet` generates each `$xxxDEF` macro from
   `Symbols` into `starlet.mar`, in the form the oracle shows: local
   definitions by default, and the global form if the macro takes one.
   *Verify:* assembling each against the oracle's objects; `rmscopy`'s
   `$RMSDEF`.
5. **Initialization macros.** Write `$FAB`, `$RAB`, `$NAM`, and the XABs in
   `starlet.mar` from `RMS-MACROS.md`, including keyword checking and the
   alignment message. *Verify:*
   - every oracle fixture's object matches byte for byte;
   - `rmscopy` and `fabalign` with govax's STARLET match real MACRO's
     objects (new `TestMacroFixtureObjects` cases with no licensed file);
   - the local-only comparison against the real STARLET.MLB (object data
     only).
6. **Store macros.** The `_STORE` forms, and their register (`Rn`) and
   run-time value forms. *Verify:* the oracle's objects.
7. **Service macros.** The 28 service macros in both formats, `$RENAME`'s
   OLDFAB/NEWFAB, and `$WAIT`'s RAB-only form. *Verify:* the oracle's
   objects; `rmscopy` links and runs under govax.
8. **Close-out.**
   - README's table: STARLET.MLB is needed only for macros govax doesn't
     have.
   - HELP.
   - Deviations for any detail where govax's object differs from real
     MACRO's, with the reason.
   - CLAUDE.md and PLAN.md.

## Decisions

- **Objects match byte for byte.** Wherever the oracle shows real MACRO's
  object, govax's must be the same, apart from timestamps and other
  variable data. A detail that can't be learned from output is logged in
  DEVIATIONS.md with govax's choice.
- **Running NAM/XAB programs is the next phase:** $PARSE/$SEARCH filling a
  NAM's result strings, and $OPEN/$DISPLAY filling XABDAT/XABFHC/XABPRO.
- **The cheap `$xxxDEF` families are in this phase:** $SSDEF, $IODEF,
  $JPIDEF, $DVIDEF, $SYIDEF, $LNMDEF, and the others `Symbols` holds, all
  from subtask 4's generator.
- **The hook is committed** in `.claude/settings.json`: the clean-room
  method is on the record with the rest of the project.

## Progress log

- 2026-09-30: Plan drafted for review. Decided: objects match byte
  for byte, NAM/XAB at run time is the next phase, the cheap `$xxxDEF`
  families are in, and the hook is committed.
- 2026-09-30: Subtask 0, the parts Claude can do:
  - **The barrier.** `.claude/hooks/cleanroom.sh` is a PreToolUse hook on
    Read, Grep, Glob, and Bash, installed by `.claude/settings.json`. It
    refuses any call whose input names:
    - the real macro library, or an extract or listing made from it;
    - `vmslib` as a directory name (so the tests' `vmsLibFile` helper
      passes), apart from the three files `gen` reads for their values;
    - `reference/vms/`;
    - the three unaudited real-MACRO listings, or their directory as a
      whole.

    `.claude/hooks/cleanroom_test.sh` has 16 cases, which all pass. The
    hook took effect in this session and refused a Bash call and an edit
    script that merely mentioned the library's file name. So docs that
    name it are edited with the Edit/Write tools, which the hook doesn't
    watch, and commit messages that name it go through `git commit -F`.
  - **Provenance.** This session read Phase 28's log, which described how
    STARLET's definition and block macros are built internally: helper
    macro names, internal symbols, the psect save and restore, and one
    quoted line. Those descriptions are redacted:
    - PHASE-28.md;
    - `conditional.go`, `message.go`, `cursor.go`, and `macros.go`;
    - `message_test.go`, `macros_test.go`, and `macrodir_test.go`, whose
      test macros and symbols are renamed generically;
    - `object.go`;
    - `fabalign.mar`'s comment.

    What remains is behavior real MACRO's output shows: the GENINFO
    message, the error text, and `$ABS$` in objects. Because this session
    has seen those descriptions, **subtasks 4–7 (writing the macros) should
    run in a fresh session** that starts with the hook active and reads
    only this document, `docs/RMS-MACROS.md`, the manuals, and the oracle's
    output.
  - **Test audit.** `TestGovaxStarletMatchesReal` reports object dumps.
    `TestStarletLoads` names macros. An assembly error inside an
    expansion names the offending token (an opcode, say), never the line.
    None prints macro text.
  - **For the author:** audit `rmscopy.lis`, `fabalign.lis`, and
    `qiow.lis` in `testdata/mar/macros/vax/` for STARLET expansion lines.
    Regenerate them with `.NOSHOW ME,MEB`, or delete them (no test reads
    them). Then remove them from the hook's list.
- 2026-09-30: Subtask 0 finished. The author audited `rmscopy.lis`,
  `fabalign.lis`, and `qiow.lis`. They hold no expansion text: a macro
  call appears, and the location advances by the bytes it generated,
  nothing more. The hook no longer refuses them. In their place, it
  refuses the Phase 32 oracle's error log (`testdata/mar/rms/vax/
  errors.log`) until the author has audited that too, since MACRO's error
  messages might echo an expansion line. `cleanroom_test.sh` has 18 cases.
- 2026-09-30: Subtask 1, mostly done.
  - **Captured.** `gen -into symbols` with `-prefix FAB$`, `RAB$`, `NAM$`,
    and `XAB$` against STARLET.OLB merged 550 names into `Symbols` (now
    4,339): 126 FAB$, 86 RAB$, 163 NAM$, and 175 XAB$. These are field
    offsets, bit numbers, sizes, and codes. Not one existing value
    changed.
  - **Cross-checked.** `TestFields_matchSymbols`: every field in
    `FABFields`/`RABFields`, derived by hand from fabdef.h and rabdef.h,
    is at STARLET's offset.
  - **`.RMSDEF`** now takes the offsets from `Symbols` and no longer
    defines them a second time from `FABFields`/`RABFields`. It defines
    the 153 FAB$/RAB$ names real `$FABDEF`/`$RABDEF` have that it lacked:
    the three `.RMSDEF` goldens gained exactly those names and nothing
    else. `TestSymbolNames_rmsFamilies` is now 605.
  - **Not yet:** XABKEY's, XABSUM's, and XABITM's fields and codes
    (XAB$C_KEY, XAB$W_POS0, ...). STARLET.OLB's definition modules don't
    have them, and the manual gives each field's name and size but not its
    offset. They need either an RMS definition file from the author's VMS
    source kit (merged with `gen -sdl`), or the oracle (subtask 3). A
    govax-written program that uses `$XABKEYDEF` and stores each field
    name, assembled by real MACRO, shows the values in its object.
- 2026-09-30: Subtask 2 done. `docs/RMS-MACROS.md` is the specification,
  written from the RMS Reference Manual's Appendices A and B, its field
  chapters, and Part III's calling formats.
  - **Contents:** the general rules; the keyword encodings (one choice →
    the `C_` code, options → the OR of the `M_` masks, SHR's `SHRxxx`
    names), checked against `Symbols` for every keyword the manual lists;
    each initialization macro's fields and defaults; the store macros'
    rules; the definition macros; and the service macros' argument
    lists.
  - **Open details:** 14 oracle questions (O1–O14), each with the fixture
    that answers it. Examples: field store order, RFM's default, what an
    omitted protection class stores, and the service macros' argument
    lists with ERR omitted.
  - **Not in VAX 7.3's values:** SHR=NQL, ROP_2's bits, and NOP's
    NO_SHORT_UPCASE are likely post-7.3 or Alpha-only (O5), and XABKEY,
    XABSUM, and XABITM still need values (O9).
- 2026-09-30: Subtask 3, the govax side. `testdata/mar/rms/gen.go` (a
  `go run` program) writes the oracle from govax's tables and the manual:
  - 73 probes: definition probes for 29 `$xxxDEF` macros; defaults,
    every-keyword, and one-option-at-a-time probes for the 12 block
    macros; store probes for the 12 `_STORE` macros; and every service
    in seven forms.
  - 10 error probes, assembled by a separate procedure into
    `ERRORS.LOG`, which the hook refuses until the author audits it.
  - `rms.com`/`rmserr.com` (`MACRO/NOLIST` and `ANALYZE/OBJECT`), and
    `exchange.cmd`.

  The definition probes' candidates are `Symbols`' names per family, plus
  the manual's XABKEY, XABSUM, and XABITM names. Names longer than
  MACRO's 31-character limit are dropped (one BLISS name).

  *Checked:* with scratch stub macros (Appendix A's keywords as formal
  arguments, empty bodies) prepended, all 73 probes assemble with govax.
  That caught one mistake of mine: `FNM=<A.DAT;1>`, where the `;` starts
  a comment.

  **Waiting on the author's VAX run** (`testdata/mar/rms/README.md`).
- 2026-09-30: VMS couldn't mount the first exchange volume. MOUNT
  reported `QUOTAFAIL` and `BADSECSYS`, both from `BADIRECTORY`: "bad
  directory file format" in the MFD.
  - **Diagnosis.** `ods2`'s AnalyzeDisk found the volume clean, and the
    MFD was one contiguous extent with sensible size fields. But with 95
    entries, its third block's records filled all 512 bytes, leaving no
    room for the 0xFFFF end-of-data marker. `ods2`'s encoder allowed that
    deliberately, since its own decoder stops by itself. Phase 28's
    smaller volume never filled a block exactly.
  - **Fix** (`ods2` cf18a63): records use at most 510 bytes, so every
    block keeps its marker. `TestDirectoryInsertKeepsBlockSentinels`
    makes 32-byte records, 16 of which filled a block, and fails with the
    old encoder. The decoder still reads an exactly-full block.
  - The volume, rebuilt, has every MFD block ending by byte 510.
    govax's tests pass.
- 2026-09-30: Subtask 3 done. The author ran the oracle on the VAX,
  audited both logs (no expansion text), and committed the results in
  `testdata/mar/rms/vax/`, 83 objects with their analyses. The hook no
  longer refuses anything there.
  - **`decode.go`** (`go run`) turns the definition probes' objects into
    `defined.txt`: for each `$xxxDEF`, every candidate name with its value
    or "undefined". It pairs each probe's `.LONG`s with the data stored
    in the DATA psect. Immediate data is gathered as one byte stream,
    because MACRO splits a run across records at any byte. Across 3,585
    definitions, no name has two values.
  - **`gen -values FILE`** merges "NAME = value" lines.
    - Additions: XABKEY's, XABSUM's, and XABITM's 81 values, and two TT$_
      codes.
    - Corrections, with `-replace`: five ATR$/FIB$ values whose listings
      are evidently a later version's. FIB$C_LENGTH is 92, not 96.
    - `Symbols` has 4,422 names.
  - **Answers** to O1–O9 and O13 are in `docs/RMS-MACROS.md`. Each
    initialization macro's store sequence (O3) is precise enough to build
    from. The rest are read from the objects as each macro is written.
  - The author asked for subtasks 4–7 to be done in this session, with a
    commit after each. This session read Phase 28's descriptions of
    STARLET's internals before redacting them (subtask 0). The macros are
    written from `docs/RMS-MACROS.md`, the manual, and the oracle's
    objects, which they're tested against.
- 2026-09-30: Subtask 4 done.
  - **The macros.** `internal/bootdata/mkdefs` generates
    `files/starletdef.mar`, 28 `$xxxDEF` macros, from
    `testdata/mar/rms/defined.txt`. Each defines exactly the names, and
    the values, real MACRO showed its namesake defining.
    - It does `.SAVE LOCAL_BLOCK` and `.PSECT $ABS$,ABS`, defines each
      name with `=`, or with `==` when the argument is `GLOBAL`, then sets
      a guard symbol of govax's own and does `.RESTORE`. A second call
      defines nothing.
    - `go generate ./internal/bootdata` runs `mkdefs`, then `mkstarlet`,
      which builds the library from both sources (`StarletSources`).
  - **More names.** `decode.go` now also takes every global symbol the
    global-form probes define. That added `SYSTEM$_FACILITY` to `$SSDEF`;
    the SS$_ candidates had missed it.
  - **Object records** (`internal/obj/builder.go`, govax's own Phase 27
    code). The objects matched real MACRO's except for where long runs of
    data were cut into records. govax filled records to 512 bytes in
    128-byte pieces. Real MACRO starts a new TIR record once one holds 461
    bytes, cutting a run of immediate data to end there (at least one
    byte of it), and a new GSD record once one holds 460.
    - Measured on every TIR record of the Phase 27–32 fixtures: most
      records closed by a cut run are exactly 461 bytes, and a 460-byte
      record that can take one more data byte does. GSD records of 460
      bytes are always followed by another.
    - Repacking the real objects' records under this rule reproduces 405
      of their 440 runs of TIR records. The rest are broken by other
      events, such as fix-ups, which govax models separately.
    - `packChunk` now follows the rule. Every existing fixture comparison
      still passes.
  - **Tests.** `TestOracleObjects` (`internal/asm/oracle_test.go`)
    assembles each oracle probe with govax's own STARLET.MLB and compares
    the object with real MACRO's, record for record, apart from traceback
    records. All 31 definition probes match, including `def_twice` and
    both global forms. `def_state` is left out: VAX 7.3 has no
    `$STATEDEF`.
- 2026-09-30: Subtask 5 done. The 12 initialization macros are in
  `starlet.mar`: `$FAB`, `$RAB`, `$NAM`, `$XABALL`, `$XABDAT`, `$XABFHC`,
  `$XABITM`, `$XABKEY`, `$XABPRO`, `$XABRDT`, `$XABSUM`, and `$XABTRM`.
  - **Sources.** Keywords and defaults come from the manual's appendix A.
    The statement sequence comes from the oracle's objects, decoded into
    block offsets: which fields are stored, in what order, where `. =`
    moves appear (zero moves included), and which stores are conditional
    (`$RAB`'s PBF/PSZ over KBF/KSZ, `$XABALL`'s ALN). Longword fields are
    stored with `.ADDRESS`; govax's assembler already encodes that as
    real MACRO does (shortest stack form, `STO_PIDR`).
  - **Shape of each macro.** It calls its `$xxxDEF`; the XAB macros also
    call `$XABDEF`, which defines the common XAB names and XAB$B_BKZ.
    Then it notes a misaligned block, stores the identification bytes,
    moves to the end and back, stores the fields, and returns to the end.
  - **`$FAB`'s FNM= and DNM=** go in psect `$RMSNAM`, with
    `.SAVE`/`.RESTORE` around them, and then FNA/FNS and DNA/DNS are
    stored.
  - **Helpers** of govax's own: `$$RMSBITS` and `$$RMSBIT` OR a field's
    option masks, trying an alternate prefix first (`FAB$M_SHR`), and
    report "UNDEFINED BIT VALUE CODE". `$$RMSCODE` handles a one-choice
    value and reports "UNDEFINED VALUE FOR FIELD". `$$RMSPRO` and
    `$$RMSCLASS` build XABPRO's protection word: four deny bits a class,
    cleared by R/W/E/D, and an omitted class denies all (^XF).
    `$$RMSCHK` displays real MACRO's alignment message.
  - **`$XABKEY`'s FLG default**: an alternate key gets DUP and CHG, and
    FLG=CHG on a primary key is real MACRO's "PRIMARY KEY MAY NOT
    CHANGE".
  - **Tested.** `TestOracleObjects` now covers 60 probes (29 more), all
    byte-identical to real MACRO's objects apart from traceback records.
    `TestMacroFixtureObjects` assembles `fabalign.mar` with govax's own
    STARLET and gets real MACRO's object. `TestRMSBlockAlignmentMessage`
    checks the message appears once, for the misaligned block.
    `TestRMSKeywordErrors` checks the error messages.
- 2026-09-30: Subtask 6 done. The 11 store macros are in `starlet.mar`:
  `$FAB_STORE`, `$RAB_STORE`, `$NAM_STORE`, and the eight XAB ones.
  - **Instruction forms**, decoded from the oracle's code:
    - The block address: a register is used as is, a label is loaded
      into R0 with MOVAL, and with neither R0 is used.
    - Keyword fields get `MOVx #mask` or `MOVB #code`, replacing the
      field rather than OR-ing into it (O12).
    - Address fields get MOVAL, and value fields MOVB/W/L/Q by the
      field's size, with the operand as given.
    - CHAN_MODE and LNM_MODE get INSV into FAB$B_ACMODES.
    - DID/FID/RFA/RFI take a register pair (MOVL Rn, MOVW Rn+1). A
      symbolic address is real MACRO's "ILLEGAL ADDRESSING MODE", and R12
      is "ILLEGAL USE OF REGISTER".
    - POS/SIZ lists get one move per element. XABPRO's PRO/UIC are
      constants from a list or a move from an address.
  - **The macros' structure.** Each calls its `$xxxDEF`(s), then passes
    its arguments by position to an inner macro of govax's own
    (`$$xxx_STO`) with the base register. A formal's name is substituted
    wherever it appears, so the outer macro can't pass keywords by name.
    The helpers are `$$RMSBASE`, `$$RMSOPT`, `$$RMSCHO`, `$$RMSPAIR`,
    `$$RMSLIST`, `$$RMSPROS`, and `$$RMSUICS`.
  - **Tested.** `TestOracleObjects` adds the seven clean store probes, all
    byte-identical (67 probes). `TestOracleStoreProbesWithErrors` covers
    the four with rejected lines:
    - each rejected line is an error for govax too, with real MACRO's
      message;
    - the probe without those lines assembles to real MACRO's code
      without theirs, compared as a stream of code bytes and relocations.
  - **Not settled by the oracle**, in DEVIATIONS.md: the store order in a
    call with several keywords (one data point), DVI= with a register, and
    a one-element PRO=/UIC= list.
