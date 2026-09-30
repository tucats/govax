# Phase 31 — Build without licensed VMS material

**Status:** in progress (2026-09-30).

## Goal

A fresh clone of the repository, with nothing but the Go toolchain and the
sibling `ods2` module, builds (`build`, which runs `go generate ./...`), vets,
and passes its tests. Nothing that DIGITAL/Compaq/HP/VSI hold copyright on is
committed. A user who has a VMS kit can put STARLET.MLB, STARLET.OLB,
IMAGELIB.OLB, LIBRTL.EXE, and so on in the `vax.library` directory to get full
fidelity. They can also use the `vmsdef/gen` tool to add definitions from
their own copies of VMS's SDL, BLISS, or message listings to govax's
symbol and message tables.

`reference/vms/` stays on the development machine as a working reference, but
it's no longer tracked by git and nothing in the build or tests reads it.
`reference/eVAX/` is the author's own work and stays.

## Inventory (what depends on or commits licensed material)

| Item | Tracked? | Build depends on it? | Disposition |
| ---- | -------- | -------------------- | ----------- |
| `reference/vms/*.h, *.sdl, *.txt` (20 files: fabdef.h, ssdef.txt, sysmsg.txt, objfmt.sdl, ...) | yes | **yes**: `internal/vmsdef/constants.go`'s `//go:generate`, which the `build` script runs | untrack; drop the generate step (subtasks 2, 6) |
| `reference/vms/disk-devices.md` | yes | no (cited by `internal/io/device.go`) | author's own chart: move to `docs/` (subtask 5) |
| `reference/vax_instr_set.pdf` (DEC's VAX Architecture manual) | yes | no | untrack; CLAUDE.md already points at the local copy (subtask 6) |
| `internal/vmsdef/constants_generated.go` (16 maps, 3,464 names) | yes | compiled in | keep as committed data, merged into one table (subtask 1). Numeric values are facts, not expression. |
| `internal/vmsdef/messages_generated.go` (sysmsg.txt's texts) | yes | compiled in | keep as committed data; no longer regenerated (subtask 3). The texts are a VMS system's ordinary output (a .COM file could print every one), not licensed material. |
| `internal/vmsdef/fab.go`, `rab.go` offsets hand-derived from fabdef.h/rabdef.h | yes | compiled in | facts; keep. Reword the provenance comments (subtask 5). |
| `internal/bootdata/files/starlet.mar` / `starlet.mlb` | yes | `go generate` (mkstarlet) | govax's own, written from the System Services manual's argument lists; keep |
| `internal/bootdata/files/ssdef.asm` | yes | embedded | values only; keep |
| `internal/bootdata/files/vax.help` ("Copyright 1997 Forest Edge Software") | yes | embedded | Forest Edge Software is the author's LLC; keep |
| `internal/vmsdef/gen/*_test.go` inline snippets | yes | tests | short, synthetic excerpts showing each format's shape; keep |
| `testdata/**` VAX binaries, objects, listings | yes | tests | the author's own sources and outputs; keep |
| `internal/console/linksource.go` `sharedImages`, `shim.go` `shimTable` offsets | yes | compiled in | hand-entered facts (LIBRTL's ident, page count, and the offsets of shimmed routines). Without IMAGELIB.OLB, LINK resolves only shimmed routines. Subtask 4 replaces this with a captured GST table. |
| `testdata/vmslib/`, `testdata/disks/`, `/starlet.req`, `/rmsdef.h` | no (gitignored) | tests skip without them | already correct; recheck that each test skips (subtask 6) |
| Git **history** of `reference/vms/` and the PDF | yes, in 482 commits | no | out of scope: the repository is private, and the author will collapse or purge its history before making it public. |

## Design

### One master symbol table

The 16 maps (`Constants`, `SSConstants`, `LNMConstants`, ..., `OBJConstants`)
become one `vmsdef.Symbols map[string]uint32`. There are no name collisions
across them today (3,464 names, 3,464 unique), so this is safe. The prefixes
already identify the facility. The one reason given for keeping them apart is
that `.RMSDEF` (`internal/asm/pseudo.go`) defines every `Constants` entry as an
assembler symbol and mustn't start defining `SS$`/`LNM$`/... names. It will
filter `Symbols` by the prefixes `FAB$`, `RAB$`, and `RMS$` instead. The
per-map doc comments carry real knowledge, such as LNM$_CHAIN being -1, the
OBJ$V_PSC_ bits all being 0, and DEVCHAR and DEVCHAR2 both numbering their
bits from 0. That knowledge moves to one package doc comment, organized by
prefix.

About 250 references are rewritten mechanically (`vmsdef.SSConstants[` becomes
`vmsdef.Symbols[`, and so on). No compatibility aliases are kept.

### The generator becomes an augmenter

`internal/vmsdef/gen` stops being a full rebuild from a fixed list of 19
required files. Instead:

- It **imports `internal/vmsdef`** to read the current `Symbols`, `Messages`,
  and a generated `SymbolSources` list (each source merged so far, by name
  and description, with no paths).
- It takes any number of repeatable, typed inputs: `-h FILE` (C `#define`
  headers), `-sdl FILE` (SDL modules, with `-sdl-stop MODULE` generalizing
  today's objfmt.sdl `$EOBJRECDEF` cut), `-bliss FILE [-prefix P]` (BLISS/VEST
  literal listings), and `-msg FILE` (message listings).
- It **merges** new names into the existing table. A name that is already
  present with the same value is a no-op. One with a different value is an
  error that lists every conflict, unless `-replace` is given. `-n` (dry run)
  reports what would be added or changed.
- It rewrites `symbols_generated.go` and/or `messages_generated.go` in the same
  sorted, `gofmt`ed form, so a merge shows up as a readable diff.

The `//go:generate` directive in `constants.go` is removed. Regenerating is a
deliberate, documented act (`go run ./internal/vmsdef/gen -sdl ~/vms/foo.sdl`),
never part of a build. The `bootdata` mkstarlet generate step stays, since it
reads only govax's own `starlet.mar`.

### Captured shareable-image symbol tables

VMS's files needn't be kept for their symbol values to be used. Gen gains
two more inputs: `-image FILE.EXE`, which reads one shareable image's GST
through `link.ReadShareableImage`, and `-imagelib IMAGELIB.OLB`, which reads
every image's GST that the library holds. The result is written to
govax-private generated tables in `vmsdef`. `vmsdef` imports nothing, so
these are plain types, not `link` types:

- `SharedImages map[string]SharedImage`: each image's page count, major and
  minor ID, and match control. This replaces the hand-entered
  `sharedImages`.
- `ImageSymbols map[string]ImageSymbol`: each universal symbol's image and
  offset, or its absolute value, plus its flags and entry mask.

The merge, conflict, and dry-run rules are the same as for `Symbols`.
`govaxSymbols()` (`internal/console/linksource.go`) builds its
`link.TableSource` from these tables. Shims still come from `shimTable`, but a
test checks that each shim's offset equals the captured one, so the two can't
drift apart.

As a result, LINK with no VMS files resolves every captured routine to its
real image and offset, and writes the same image it would with IMAGELIB.OLB
present. What the table can't provide:

- **Running** a routine that has no shim still needs the real image in
  `vax.library`, since RUN resolves against the real image or a shim.
- **STARLET.OLB modules that carry code**, since a module is copied into the
  link. Absolute symbols that STARLET.OLB defines can be captured the same
  way (`-olb FILE`, which records absolute definitions only), but whether
  that's worth doing depends on what the table-driven sources don't already
  cover. Subtask 4 measures this.

Recommended storage: generated Go map literals (as today), not an embedded
text file parsed at init. This keeps compile-time tables and the existing
access pattern. The cost is that gen can't run if the generated file doesn't
compile, and the fix for that is `git restore`.

## Subtasks

Each subtask ends with `go build ./... && go vet ./... && go test ./...` and
golangci-lint clean, then a commit (and `build -i` where it changes behavior).

1. **Merge the maps into `vmsdef.Symbols`.** Have the current gen emit one
   map. Rewrite the consumers. Filter `.RMSDEF` by prefix. Move the per-map
   notes into a doc comment. *Verify:* the existing tests pass unchanged apart
   from the renames. A new test checks that `.RMSDEF` defines exactly the same
   symbol set as before (the FAB$/RAB$/RMS$ subset, compared by count and by
   content).
2. **Symbol augmentation mode for gen.** Add the typed repeatable flags, the
   `SymbolSources` bookkeeping, the merge and conflict policy, `-replace`, and
   `-n`. Remove the `//go:generate` line. *Verify:* unit tests for the merge
   (add, same-value no-op, conflict error, `-replace`), plus a round trip.
   Running gen with no inputs must leave `symbols_generated.go` byte-identical,
   and re-merging each original reference/vms file (locally) must add nothing.
   `go generate ./...` succeeds with `reference/vms` moved aside.
3. **Message augmentation mode.** `-msg FILE` merges facilities and messages
   into `Messages` under the same conflict and dry-run rules. *Verify:* the
   same tests as subtask 2, applied to messages, and `$GETMSG`/`$PUTMSG`
   goldens unchanged.
4. **Captured shareable-image GSTs.** Add `-image`/`-imagelib` to gen, and
   the `SharedImages`/`ImageSymbols` tables. Capture them from the local
   `testdata/vmslib` copies. Build `govaxSymbols()` from the tables, and check
   the shims against them. Measure which STARLET.OLB symbols are still missing
   when linking the fixtures with `/NOSYSLIB`-equivalent sources, and capture
   absolute ones with `-olb` if that's worthwhile. *Verify:* each
   `testdata/link` and `testdata/mar` fixture links byte-identically with
   `vax.library` pointing to an empty directory. Today these tests need the
   real files or /NOSYSLIB. Every shim offset matches the captured table.
5. **Scrub path references.** Reword every Go comment and generated header
   that cites a `reference/vms/...` path to name the VMS definition instead
   (for example "VMS 7.3's $SSDEF"). There are about 16 files. Move
   `disk-devices.md` to `docs/`. Update CLAUDE.md, PLAN.md, and the phase docs
   that tell readers to look in `reference/vms`. *Verify:*
   `git grep reference/vms -- ':!docs/PHASE-*.md'` is empty; historical
   progress logs keep their wording.
6. **Untrack the licensed files.** Run `git rm --cached -r reference/vms
   reference/vax_instr_set.pdf` (local copies stay), add both to `.gitignore`
   with a comment, and note in CLAUDE.md that they're local-only. *Verify:* in
   a scratch clone with a `go.work` pointing at `../ods2`, and with no
   `reference/vms`, `testdata/vmslib`, or `testdata/disks`, check that `build`
   (with its `go generate`), `go vet`, and `go test ./...` all pass, and that
   tests needing licensed files skip with a message rather than fail.
7. **User-facing docs.** Add a README section, "Optional VMS files", covering
   what goes in `vax.library` (STARLET.MLB, STARLET.OLB, IMAGELIB.OLB,
   LIBRTL.EXE and other shareable images), what govax falls back to without
   each one, and how to extend the symbol and message tables with gen. Also
   add a HELP topic if one fits. *Verify:* doc review.

## Decisions

- **Message texts** stay committed. They're what any VMS system prints (a
  .COM file could produce every one), and there's no license restriction on
  a VMS system's output.
- **History** is out of scope. The repository is private, and the author will
  collapse or purge its history before it's made public. This phase stops
  licensed files from being committed from now on.
- **Captured images** (subtask 4): LIBRTL, plus each image govax has shims
  for. More are added later through `gen -image` as they're needed. For that
  reason `-imagelib` takes an image-name filter (`-images LIBRTL,...`) rather
  than capturing everything IMAGELIB.OLB lists.

## Progress log

- 2026-09-30: Plan drafted for review. Open questions settled (see
  Decisions): keep the message texts, leave history alone, and capture only
  LIBRTL and the shimmed images.
- 2026-09-30: Subtask 1 done. The 16 generated maps are one table,
  `vmsdef.Symbols` (`symbols_generated.go`, 3,464 names). Before the merge,
  a check confirmed that no name appears in two maps, and that the old
  `Constants` map was exactly the 393 FAB$/RAB$/RMS$ names, which no other
  map uses. `vmsdef.SymbolNames(prefixes...)` returns a family's names,
  sorted, and `.RMSDEF` uses it for FAB$/RAB$/RMS$. `symbols.go` documents
  each prefix, taking over the old per-map doc comments. The consumers
  (about 250 references in 48 files) were renamed mechanically. New tests:
  `TestSymbolNames_rmsFamilies` pins the 393-name set, and the `.RMSDEF`
  test checks that SS$/LNM$/IO$ names stay out. For now gen still
  rebuilds everything from `reference/vms`; subtask 2 changes that.
- 2026-09-30: Subtask 2 done. `internal/vmsdef/gen` now merges into the
  existing table instead of rebuilding it. It imports `vmsdef` to read
  `Symbols` and a new generated `SymbolSources` list (the file names merged
  so far, with no paths), takes repeatable `-h`/`-sdl`/`-bliss` inputs in
  command-line order, and rewrites `symbols_generated.go`. `-prefix` and
  `-sdl-stop` apply to the inputs after them, so objfmt.sdl's `$EOBJRECDEF`
  cut is no longer a special case. The C header parser (`header.go`) takes
  any upper-case `XXX$` name, filtered by `-prefix`, rather than only
  FAB$/RAB$/RMS$. The merge rules:
  - a name already present with the same value is left alone;
  - a different value is a conflict, and all conflicts are reported
    together;
  - `-replace` overwrites conflicting values;
  - `-n` is a dry run that lists what would be added or changed.

  The `//go:generate` directive and `constants.go` are gone.
  *Verified:*
  - gen with no inputs rewrites the file byte for byte (and
    `TestGenerateSymbols_roundTrip` checks this).
  - Re-merging all 18 original `reference/vms` files with their old prefix
    filters reports 0 added and 0 changed for each. Without the prefixes,
    only `SYSTEM$_FACILITY` would be new.
  - With `reference/vms` moved aside, `go generate ./... && go build ./...`
    succeeds.

  The message texts aren't regenerated any more either; subtask 3 gives
  them the same merge mode.
- 2026-09-30: Subtask 3 done. `gen -msg FILE` merges a message listing's
  facilities (by number) and messages (by masked condition value) into
  `Messages` and `MessageFacilities`, under the same rules as symbols: the
  same entry is left alone, a different one is a conflict unless
  `-replace`, and `-n` is a dry run. A message counts as the same only if
  its facility, ident, text, and $FAO count all match.
  `messages_generated.go` has a `MessageSources` list, and gen rewrites
  both generated files on every run. *Verified:* the first rewrite changed
  only the file's header comments, not one message; gen with no inputs
  reproduces both files (`TestGenerateMessages_roundTrip`); re-merging
  `sysmsg.txt` finds all 1,426 messages and 6 facilities already present;
  the full suite, `$GETMSG`/`$PUTMSG` goldens included, passes.
- 2026-09-30: Subtask 4 done, with two changes to the plan:
  - **No `-imagelib`.** IMAGELIB.OLB only says which image defines a
    symbol; the offsets come from the image itself, as real LINK reads
    them. So the capture is `gen -image FILE.EXE`, and only LIBRTL.EXE is
    on hand locally. Its 305 symbols and header facts (264 pages, ident
    1.14, match LEQ) went into `vmsdef.SharedImages`/`ImageSymbols`
    (`images_generated.go`). The header facts agree with the old
    hand-entered `sharedImages`, which is gone, and the capture adds the
    symbol, psect, and section counts a map reports. DECC$SHR and
    CMA$TIS_SHR aren't captured (no .EXE here), so their shims still give
    their offsets.
  - **`-olb` was worth doing.** With no STARLET.OLB, a program that leaves
    `SS$_NORMAL` for LINK to resolve got it undefined. `gen -olb FILE`
    captures the absolute symbols of an object library's definition
    modules into `vmsdef.LibrarySymbols` (`library_generated.go`). A
    definition module is one that adds nothing to an image: only header,
    GSD, and EOM records, plus text records that only set the relocation
    base; empty psects; absolute definitions; no references. STARLET has
    23 such modules and 7,292 symbols (SS$, IO$, SYI$, RMS$, SYS$, SMG$
    and BAS$ codes, ...). Each value comes from the module the library's
    global symbol index names, as LINK would add it: SYS$P1_VECTOR and
    SYS$VECTOR both define SYS$CONNECT, at different addresses.

  `govaxSymbols()` now builds LINK's own source in this order: captured
  image symbols, then library symbols where no image defines the name,
  then the P1 vector (overriding), then shims for images not captured.
  *Verified:*
  - `TestLink_capturedRoutineWithoutLibraries` links LIB$GET_INPUT (no
    shim) and SS$_NORMAL with an empty `vax.library` and no undefined
    symbols, and the image equals the one linked with the real IMAGELIB,
    STARLET, and LIBRTL. With empty tables, the same test fails
    (LIB$GET_INPUT undefined).
  - `TestShimOffsetsMatchCapturedImages` (no VMS files needed) and
    `TestCapturedLIBRTLMatchesImage` (with librtl.exe) pass.
  - `TestLibrarySymbolsMatchP1Vector`: 311 of 312 services match; the
    spare slot differs.
  - Re-merging librtl.exe and starlet.olb adds nothing.

  Finding: STARLET's values expose five wrong SS$_ codes in `Symbols`
  (ssdef.txt's; `sysmsg.txt`, eVAX's `ssdef.asm`, and STARLET all agree
  on others), plus build-dependent JPI$/SYI$ "last" markers; logged in
  docs/DEVIATIONS.md and pinned by `TestSymbols_matchLibrarySymbols`.
  Deferred for the author's decision on how to correct `Symbols`.
- 2026-09-30: Subtask 5 done. Every Go comment that cited a
  `reference/vms/...` path now names the VMS 7.3 definition file instead
  (12 files). `disk-devices.md`, the author's own chart, moved to
  `docs/DISK-DEVICES.md`. PLAN.md's two historical mentions say the files
  are kept outside the repository. CLAUDE.md gained an `internal/vmsdef`
  entry saying the generated tables are committed data that
  `internal/vmsdef/gen` merges into. Citations of `vax_instr_set.pdf` name
  the manual, not a repository path, so they stay. *Verified:*
  `git grep reference/vms -- ':!docs/PHASE-*.md'` finds nothing but this
  phase's own plan.
