# Phase 24: `.RMSDEF`/`.FAB`/`.RAB` assembler pseudo-ops

## Goal

Give `internal/asm` real `$FABDEF`/`$RABDEF`/`$RMSDEF`-equivalent support, so a
MACRO-32 fixture that drives RMS (`testdata/asm/rms_roundtrip.asm`, and whatever
comes next per the design note below) can build a FAB/RAB by keyword
(`.FAB FAC=FAB$V_PUT, ORG=FAB$C_SEQ, RFM=FAB$C_FIX, MRS=4, FNA=fspec, FNS=13`)
instead of hand-laying-out `.BLKB`/`.BYTE`/`.WORD`/`.LONG` blocks at byte offsets
a programmer has to already know, with the symbolic VMS names (`FAB$V_PUT`,
`FAB$C_SEQ`, ...) only ever present as an unchecked `;` comment today. Three new
pseudo-ops:

- `.RMSDEF` — defines every real FAB$/RAB$/RMS$ symbol (field offsets, bitmask
  flags, named code values, and RMS$_ completion-status codes) as an assembler
  symbol, the same role the real `$FABDEF`/`$RABDEF`/`$RMSDEF` MACRO-32 library
  macros play, combined into one pseudo-op per the user's own request. Not
  defined until `.RMSDEF` is actually given, so an ordinary program that never
  touches RMS doesn't carry ~450 unused symbols in its symbol table.
- `.FAB` — builds an 80-byte FAB instance at the current deposit location,
  field values supplied as `KEYWORD=value` (numeric, radix-prefixed, or a
  symbol — including a `.RMSDEF`-defined one) parameters, matching the real
  `$FAB` macro's own calling convention.
- `.RAB` — same for a 68-byte RAB, matching `$RAB`.

**Status: planning — no subtasks started yet.**

## Why this phase looks different

Like Phase 22/23, this phase has no `reference/eVAX` counterpart: confirmed by
grepping `reference/eVAX/eVAX/Source/Assembler/asm_pseudo.c`'s own `pseudos[]`
table (30 entries) for any `FAB`/`RAB`/`RMSDEF` name — there is none, because
`eVAX` never implemented RMS at all (its own `rms.c` is a stub that always
returns `SS$_NORMAL`; see `internal/rms/status.go`'s own doc comment). This
phase's correctness reference is instead `reference/vms/{fabdef,rabdef,rmsdef}.h`
— real VAX/VMS 7.3 SDL-generated C headers (not part of `reference/eVAX`) that
already served as `internal/rms/fab.go`/`rab.go`'s own cross-check during Phase
22, alongside a real VMS 7.3 system. There is no C pseudo-op behavior to port
for fidelity; this is free design within real VMS's own documented field
layout/symbol set, the same freedom Phase 22/23 had within `ods2`'s API and
real VMS DCL conventions respectively.

`docs/PHASE-22.md`'s own subtask 14 explicitly flagged this as deliberately
deferred, future work — not something this phase is inventing a need for:

> FAB/RAB field values in the test program: hand-encoded literals rather than a
> general `$FABDEF`/`$RABDEF` `.INCLUDE` macro-expansion facility (out of scope
> this phase — no such `.asm` fixture exists yet; building one is plausible
> future `internal/asm` work, not needed for this acceptance test).

That note imagined an `.INCLUDE`-based macro-expansion facility (closer to how
real MACRO-32's own `$FABDEF` works: it expands to a pile of `.EQU`-style offset
symbols, not a struct-building pseudo-op). This phase instead makes `.FAB`/`.RAB`
themselves build the struct directly, keeping `.RMSDEF` as the symbol-definition
half — matching the user's own explicit description (`.FAB`/`.RAB` as an
analogue of the `$FAB`/`$RAB` macros, `.RMSDEF` as the analogue of `$FABDEF`/
`$RABDEF`/`$RMSDEF`) rather than the `.INCLUDE`-macro shape PHASE-22.md
originally sketched.

## Scope: build the complete field/symbol set now, not just what RMS reads today

`internal/rms/fab.go`/`rab.go` today only define the ~10 (of 33) FAB fields and
~9 (of 26) RAB fields this project's own SYS$CREATE/CONNECT/PUT/GET/CLOSE/OPEN
handlers actually read or write — private, unexported constants, verified
against a real VMS 7.3 system and by hand-walking `fabdef.h`/`rabdef.h`'s C
struct declarations (see those files' own doc comments). **Per the user's
explicit direction while this phase was being planned**, `.FAB`/`.RAB`/
`.RMSDEF` build the *complete* real field/symbol set from day one — every named
FAB/RAB field (not just the subset RMS currently consumes), every `FAB$M_`/
`RAB$M_` bitmask flag, every `FAB$C_`/`RAB$C_` named code value, and every
`RMS$_` completion-status code — because RMS support is expanding next (more
record organizations, more FAB/RAB-driven behavior), and those additional
fields/symbols will be needed soon. Scoping this pass down to "just what today's
handlers use" would mean redoing this work in the very next phase.

## Design decisions

### Shared data lives in `internal/vmsdef` (consolidated from `internal/p1vector`), not a new one-off package

Both `internal/asm` (to place a `.FAB`/`.RAB` keyword's value at the right byte
offset, and to define `.RMSDEF`'s symbols) and `internal/rms` (which already has
its own, currently-private, partial offset constants) need the same field-
offset/symbol data. Per the same reasoning `docs/PHASE-11.md`'s `.P1VECTOR` work
landed on (see that doc's own progress log): neither package is naturally
"below" the other here, and hand-duplicating ~450 symbols in two places would be
a real drift risk.

This phase's own planning originally sketched a second, FAB/RAB-only leaf
package (`internal/rmsdef`) mirroring `internal/p1vector`'s existing shape —
but the user flagged mid-planning that this asm/RTL-shared-static-VMS-data
need is going to keep recurring as more of the VMS system-service library
gets ported (more record organizations, more FAB/RAB-driven behavior, and
whatever comes after that), and a fresh micro-package per topic doesn't scale.
Resolved by consolidating instead of adding a sibling: `internal/p1vector`
was renamed to **`internal/vmsdef`** (`P1VectorEntry`/`P1VectorTable`, same
content, just renamed to avoid colliding with this phase's own `Field`/
`Constant` types once they land in the same package) and this phase's FAB/RAB/
RMS-status data becomes new files inside that same package rather than a new
import path:

- `internal/vmsdef/p1vector.go` — unchanged content, existing `.P1VECTOR` data.
- `internal/vmsdef/fab.go`/`rab.go` (this phase, subtask 1) —
  `FABFields`/`RABFields []Field{Symbol, Keyword string; Offset, Size uint32}`,
  the complete, doubly-verified struct-offset tables (33 FAB fields to 80
  bytes, 26 RAB fields to 68 bytes; see "Verifying the offset tables" below).
- `internal/vmsdef/constants_generated.go` (this phase, subtask 1, generated —
  see "Generator, not hand-transcription" below) — every `FAB$M_`/`FAB$C_`/
  `RAB$M_`/`RAB$C_`/`RMS$_` symbol and its real numeric value.

One package, not one per topic: a caller needing several of these tables at
once (as `internal/asm`'s `.RMSDEF` already will, for both FAB and RAB data
together) imports one path, and adding the next VMS-definitions table this
project needs is "add a file here," not "stand up and wire in another
near-identical micro-package." No interfaces, registries, or subpackages —
still just flat Go data, organized by file within one package.

`internal/rms/fab.go`/`rab.go` are updated to read their field offsets from
`internal/vmsdef` instead of maintaining their own private literals (exactly
how `internal/rtl/p1vector.go` was already cut over to build its dispatch
index over `vmsdef.P1VectorTable` instead of owning its own copy) — a nice
side effect of this phase: it deletes the last hand-duplicated-constant risk
`fab.go`'s own doc comment already worried about (the `fabFNS`/48-vs-52 bug it
documents was exactly this kind of drift, caught the hard way once already).

### Generator, not hand-transcription, for the flat constant tables

`reference/vms/fabdef.h`/`rabdef.h`/`rmsdef.h` together carry ~450 flat
`#define NAME value` constant lines (80 + 45 + 267, per this phase's planning
research, plus the two `$K_BLN`/`$C_BLN` struct-length pairs) — far too many to
safely hand-transcribe (the exact mistake `internal/cpu/gen/main.go`'s own doc
comment cites as the reason *it* exists, for `instruction_table.h`'s ~284-entry
table: "rather than hand-transcribing ... entries"). This phase adds a sibling
generator, `internal/vmsdef/gen/main.go`, run via `go generate` from
`internal/vmsdef` (matching `internal/cpu/instruction.go`'s own `go:generate`
directive precedent exactly), parsing each header's `#define <PREFIX>$<NAME>
<value>` lines with a simple regex (far simpler than `instruction_table.h`'s
multi-field struct-literal entries — this is a flat, one-line-per-symbol
format) and skipping the C-only field-access aliasing macros each header ends
with (e.g. `#define fab$w_ifi fab$r_ifi_overlay.fab$w_ifi` — recognizable
because the "value" isn't a bare integer literal).

### Verifying the offset tables: hand-derived twice, not generated

Unlike the flat constant lists, `fabdef.h`/`rabdef.h` don't carry `#define`
lines for field *offsets* at all (real MACRO-32's own `$FABDEF` generates those
separately from the struct; this C header only declares `struct fabdef { ... }`
and lets a C compiler compute layout). Deriving them means walking the struct
declaration applying VAX C's natural alignment rules (1/2/4-byte types align to
their own size; a `union` takes its largest member's size/alignment) — the same
method `fab.go`/`rab.go`'s own doc comments describe using originally. This
phase's own planning already did that derivation twice, independently (once by
hand while planning, once via a research agent working from the same headers,
neither aware of the other's numbers), and both land exactly on `FAB$K_BLN`
(80) and `RAB$K_BLN` (68) and agree on every field offset — including
reproducing `fab.go`/`rab.go`'s existing, already-independently-verified subset
exactly. Given only 33 + 26 = 59 fields total (small enough to review by eye,
unlike the ~450-entry flat constant lists above), the offset tables are
hand-transcribed into `internal/vmsdef` with both derivations' agreement
recorded as their provenance — not machine-generated, since a struct-layout
parser for just two small, one-off structs would be more code than the data it
produces, and the flat-constant generator already eliminates the actual
high-risk transcription (450 symbols, not 59).

### `.RMSDEF` symbol shape, and why it doesn't gate `.FAB`/`.RAB`

`.RMSDEF` defines each field's real MACRO-32 offset symbol (`FAB$L_STS`,
`FAB$B_FAC`, `RAB$L_RBF`, ...) plus every `FAB$M_`/`FAB$C_`/`RAB$M_`/`RAB$C_`/
`RMS$_` constant — so a program can address a field the real MACRO-32 way
(`fab+FAB$L_STS`) once `.RMSDEF` has run, exactly like a real assembly that
opens with `$FABDEF`. `.FAB`/`.RAB` themselves do **not** require `.RMSDEF` to
have run first: their own keyword table (`FAC=`, `ORG=`, ...) is internal Go
data driving field placement directly, independent of whether the symbolic
constants exist as assembler symbols. `.RMSDEF` only matters once a `.FAB`/
`.RAB` keyword's *value* is written as a symbolic name (`ORG=FAB$C_SEQ` instead
of `ORG=0`) — at that point ordinary forward-reference/undefined-symbol
handling already covers "used before `.RMSDEF`", no special-case enforcement
needed, the same way a real MACRO-32 program simply gets `%FAB$C_SEQ IS
UNDEFINED` if it references `$FABDEF`'s own symbols without including it first.

Matches `.P1VECTOR`'s own fixed idempotency bug (`docs/PHASE-11.md`'s
same-day fix): `.RMSDEF` must tolerate being assembled more than once in the
same session without a duplicate-symbol error — `unique=false` on every
`setSymbol` call, from the start this time, not a fix bolted on after the fact.

### `.FAB`/`.RAB` keyword parsing and sub-field addressing

Reuses `pseudoSet`'s existing `NAME` `=`-or-`,` `value` separator handling
(`internal/asm/pseudo.go`) as the per-keyword loop shape, and `exprValue` (not
`exprNoForward`) for each field's value so a forward reference works exactly
like `.LONG` already supports (`FNA=fspec` where `fspec` is a label defined
later in the file). Every field not named in the parameter list is left as
zero, matching `.BLKB`'s own existing "never written reads back as zero"
convention (`internal/asm/image.go`) — no explicit zero-fill pass needed, and
matching real VMS's own expectation that a calling program supplies a
zero-initialized block to begin with.

Still open (see "Open questions"): whether `.FAB`/`.RAB` additionally define
per-field sub-symbols (`<label>_FAC`, `<label>_STS`, ...) the way
`rms_roundtrip.asm`'s current hand-rolled layout does today, or leave a program
to address fields the real-MACRO-32 way (`<label>+FAB$L_FAC`) once `.RMSDEF`
has defined the offset symbols — to be resolved during implementation once
there's a concrete second fixture (per the design note) to judge which reads
better in practice.

## Scope / field-and-symbol inventory

(Derived during planning from `reference/vms/fabdef.h`/`rabdef.h`; both
independently reproduce `internal/rms/fab.go`/`rab.go`'s existing verified
subset exactly.)

- **FAB** — 80 bytes, 33 named fields (`fab$b_bid`/`bln` through
  `fabdef$$_fill_9`), 80 `FAB$` `#define` constants (struct-length pair,
  ~14 `FAB$C_` code values, ~66 `FAB$M_` bitmask flags across `fop`/`fac`/
  `shr`/`org`/`rat`/`journal`/`rcf`).
- **RAB** — 68 bytes, 26 named fields, 45 `RAB$` `#define` constants
  (struct-length pair, 6 `RAB$C_` code values, ~37 `RAB$M_` bitmask flags,
  almost all within the single `rab$l_rop` longword).
- **RMS$_** — 267 completion-status codes (`rmsdef.h`), no struct of its own —
  pure status-code symbol definitions, the largest single piece of `.RMSDEF`'s
  own symbol table. `internal/rms/status.go` already has 18 of these as
  private, verified Go constants; this phase's `internal/vmsdef.Constants`
  becomes the complete superset.

## Subtasks

1. `internal/vmsdef` (consolidated package, see "Shared data lives in
   `internal/vmsdef`" above): `fab.go`/`rab.go` (`Field` type, `FABFields`/
   `RABFields`) and the `internal/vmsdef/gen` generator (`go generate` over
   `reference/vms/{fabdef,rabdef,rmsdef}.h`) producing `Constants`, plus the
   hand-transcribed, doubly-derived FAB/RAB offset tables. Migrate
   `internal/rms/fab.go`/`rab.go` to read their offsets from this package
   instead of their own private constants (behavior-preserving — existing
   `internal/rms` tests must pass unchanged).
2. `.RMSDEF` pseudo-op: defines every `internal/vmsdef.Constants` entry plus
   each field's own `FAB$_`/`RAB$_` offset symbol, permanent, idempotent
   (`unique=false` from the start).
3. `.FAB` pseudo-op: `KEYWORD=value` parsing over the full 33-field table,
   80-byte deposit, forward-reference-capable field values.
4. `.RAB` pseudo-op: same for the 26-field, 68-byte RAB.
5. Update `testdata/asm/rms_roundtrip.asm` to build its FAB/RAB via
   `.RMSDEF`/`.FAB`/`.RAB` instead of hand-laid-out `.BLKB`/`.BYTE`/`.WORD`/
   `.LONG` blocks and bare numeric literals — the concrete acceptance check
   for this phase, matching how Phase 22 subtask 14's own fixture already
   exercises the real SYS$ call path end to end.
6. Docs: this file's progress log; `docs/PLAN.md` phase-table row + narrative
   paragraph; `docs/DEVIATIONS.md` only if a genuine FAB/RAB field-value
   ambiguity turns up while double-checking against `reference/vms` (not
   expected — these are real VMS headers, not a from-scratch reimplementation
   the way Phase 22's own RMS semantics work was).

## Open questions

- Exact `.FAB`/`.RAB` sub-field addressing convention (auto-generated
  `<label>_<FIELD>` sub-symbols vs. real-MACRO-32-style `<label>+FAB$L_<FIELD>`
  addressing via `.RMSDEF`'s own offset symbols) — deferred to subtask 3/4,
  once there's a second real fixture (per the design note's "more record
  types" direction) to judge against, not just subtask 5's rewrite of the one
  existing fixture.
- Whether `.RMSDEF` should also expose the `FAB$V_`-style single-bit-position
  aliases some real VMS macros define alongside their `FAB$M_` mask
  siblings (e.g. `FAB$V_PUT` as a bit index vs. `FAB$M_PUT`'s already-shifted
  mask value) — `fabdef.h`'s own header doesn't appear to define `FAB$V_`
  constants directly (bit positions only exist implicitly as C bitfield
  declarations), so this may be moot; confirm during subtask 1's generator
  work.

## Progress Log

### 2026-09-23 — Planning

- Requested by the user, directly following `docs/PHASE-11.md`'s own
  `.P1VECTOR` work landing: "similar need for the p1vector shared storage
  area between asm and rtl" for `.RMSDEF`/`.FAB`/`.RAB`, which `internal/rms`
  and `internal/asm` would otherwise need independently.
- Delegated research (subagent, ~224s/123K tokens): confirmed `reference/eVAX`
  has no FAB/RAB/RMSDEF pseudo-op precedent at all (grepped `asm_pseudo.c`'s
  own 30-entry `pseudos[]` table); catalogued `internal/rms/fab.go`/`rab.go`'s
  existing ~10/~9-field private offset subsets and their own verification
  provenance; extracted the full `reference/vms/fabdef.h`/`rabdef.h` symbol
  counts (80/45 `#define`s) and hand-derived both structs' complete field-
  offset tables by C alignment rules, landing exactly on `FAB$K_BLN`=80/
  `RAB$K_BLN`=68 and reproducing `fab.go`/`rab.go`'s existing values exactly;
  confirmed `rmsdef.h` is 267 flat `RMS$_` status-code `#define`s with no
  struct of its own; identified `internal/asm/pseudo.go`'s existing
  `pseudoShim`/`pseudoP1Vector` (symbol+byte-deposit shape) and `pseudoSet`
  (`KEYWORD=value` parsing shape) as the closest reusable precedent, and
  `internal/cpu/gen/main.go` as the established `go:generate`-a-Go-table-
  from-a-reference-header precedent this phase's own `internal/vmsdef/gen`
  will mirror; confirmed `docs/PHASE-22.md` subtask 14's own scope-cut note
  ("plausible future `internal/asm` work") anticipated exactly this. Ran
  an independent hand-derivation of both offset tables in parallel with the
  agent (neither aware of the other's numbers) as a second cross-check — both
  agree on every field, matching `fab.go`/`rab.go`'s existing verified subset.
- User clarified mid-planning, unprompted, a forward-looking scope decision:
  build the *complete* FAB/RAB field and symbol set now (not just the ~10/~9
  fields today's RMS handlers read), since RMS support is expanding next
  (more record types, etc.) and this data will be needed again soon — see
  "Scope: build the complete field/symbol set now, not just what RMS reads
  today" above. This settled what would otherwise have been an open scoping
  question (start narrow and widen later vs. build complete now).
- No code written yet; this document is the planning deliverable requested.
  Implementation starts at subtask 1 in a future session.

### 2026-09-23 — Subtask 1, part 1: consolidate `internal/p1vector` into `internal/vmsdef`

- User flagged, after this doc's own initial planning pass sketched a second,
  FAB/RAB-only `internal/rmsdef` leaf package: the asm/RTL-shared-static-VMS-
  data need is going to keep recurring as more of the VMS system-service
  library gets ported, so a fresh micro-package per topic doesn't scale —
  explicitly left the call on whether to act on it now to this session's own
  judgment ("this may introduce unneeded complexity, but ... we probably
  will be at this point again").
- Decided to consolidate rather than add a sibling: renamed
  `internal/p1vector` to `internal/vmsdef` (`p1vector.go`'s own content
  unchanged, `Entry`/`Table` renamed to `P1VectorEntry`/`P1VectorTable` to
  leave room for this phase's own `Field`/`Constant` types in the same
  package without a name collision), updated every consumer
  (`internal/rtl/p1vector.go`, `internal/rtl/rtl_test.go`,
  `internal/asm/pseudo.go`, `internal/asm/p1vector_test.go`) and every code
  comment naming the old import path. This phase's own FAB/RAB/RMS-status
  data (rest of subtask 1) lands as new files in this same package rather
  than a new one — see "Shared data lives in `internal/vmsdef`" above, which
  replaced this doc's own original `internal/rmsdef` design section
  in-place rather than leaving both versions around to go stale against
  each other.
- `docs/PHASE-11.md`'s own `.P1VECTOR` progress-log entry (which predates
  this rename) got one short addendum note pointing at this rename rather
  than being rewritten — its own historical text still says
  `internal/p1vector`, accurate as of when it was written.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean immediately
  after the rename (behavior-preserving, no `.P1VECTOR` semantics touched).
  Subtask 1's actual new content (FAB/RAB field tables, the constant
  generator, `internal/rms` migration) continues from here.
