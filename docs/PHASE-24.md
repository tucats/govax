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

**Status: in progress — subtasks 1-5 done, subtask 6 (docs) remaining.**

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

**Resolved during subtask 5 planning:** `.FAB`/`.RAB` do *not* define
per-field sub-symbols (`<label>_FAC`, `<label>_STS`, ...) the way
`rms_roundtrip.asm`'s pre-Phase-24 hand-rolled layout did — a program
addresses a field at runtime the real-MACRO-32 way instead,
`<label>+FAB$B_FAC`, once `.RMSDEF` has defined the offset symbol. Confirmed
working directly (a throwaway probe test assembled `movb #1,
@#fab+FAB$B_FAC` and decoded the emitted absolute-mode operand address to
confirm it resolved to the FAB's base plus the real offset, exactly) before
committing to it for subtask 5's rewrite — ordinary `label+symbol` expression
arithmetic already supported this with zero new code, since `.FAB`/`.RAB`'s
own field values only ever need to be *set* by keyword at assembly time, not
also independently *addressed* by keyword at runtime.

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

1. **Done.** `internal/vmsdef` (consolidated package, see "Shared data lives
   in `internal/vmsdef`" above): `fab.go`/`rab.go` (`Field` type, `FABFields`/
   `RABFields`) and the `internal/vmsdef/gen` generator (`go generate` over
   `reference/vms/{fabdef,rabdef,rmsdef}.h`) producing `Constants`, plus the
   hand-transcribed, doubly-derived FAB/RAB offset tables. Migrated
   `internal/rms/fab.go`/`rab.go`/`status.go` to read their offsets/constants
   from this package instead of their own private literals (behavior-
   preserving — existing `internal/rms` tests pass unchanged).
2. **Done.** `.RMSDEF` pseudo-op: defines every `internal/vmsdef.Constants`
   entry plus each field's own `FAB$_`/`RAB$_` offset symbol, permanent,
   idempotent (`unique=false` from the start). Not gated on `.MICROKERNEL`
   (deposits no bytes, unlike `.P1VECTOR`/`.SHIM`/`.SCB`/`.REGION`).
3. **Done.** `.FAB` pseudo-op: `KEYWORD=value` parsing over the full
   33-field table, 80-byte deposit, forward-reference-capable field values.
4. **Done.** `.RAB` pseudo-op: same for the 26-field, 68-byte RAB — landed
   together with subtask 3 via a shared `buildControlBlock` implementation
   (identical shape, differing only in field table/block size/BID-BLN
   constant names), not worth a separate change.
5. **Done.** Update `testdata/asm/rms_roundtrip.asm` to build its FAB/RAB via
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

- ~~Exact `.FAB`/`.RAB` sub-field addressing convention~~ **Resolved** — see
  the design section's own "Resolved during subtask 5 planning" note above:
  real-MACRO-32-style `<label>+FAB$L_<FIELD>` addressing, no sub-symbols.
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

### 2026-09-23 — Subtask 1, part 2: FAB/RAB field tables, constant generator, `internal/rms` migration — subtask complete

- `internal/vmsdef/fab.go`/`rab.go` (new): the `Field` type
  (`Symbol`/`Keyword`/`Offset`/`Size`) and the complete, hand-transcribed
  `FABFields` (33 entries, 80 bytes) / `RABFields` (26 entries, 68 bytes)
  tables — both `KBF`/`PBF`, `KSZ`/`PSZ`, and `BKT`/`DCT` real union-member
  aliases included per this doc's own design note. `RAB$W_RFA` (offset 16,
  size 6) is the one field with no single-value keyword form in real `$RAB`
  either — included for `.RMSDEF`'s own offset-symbol definition, but
  `Field.Size`'s own doc comment flags it as not placeable via `.FAB`/`.RAB`'s
  1/2/4-byte keyword mechanism.
- `internal/vmsdef/fields_test.go` (new): `TestFABFields_tileEightyBytes`/
  `TestRABFields_tileSixtyEightBytes` confirm every field fits within its
  struct's real size with no unexpected byte-range overlaps (the three known
  alias pairs above are explicitly allowed; anything else overlapping would
  be a real transcription bug), and `TestFieldTables_keywordsUnique` confirms
  no duplicate keyword within either table (`.FAB`/`.RAB`'s own future
  keyword lookup depends on this). Caught one real mistake immediately:
  the first draft asserted FAB's highest field must end exactly at byte 80,
  which is wrong — the struct's own trailing 4-byte reserved padding
  (deliberately omitted, having no real field name) means the last *named*
  field (`RCF`) ends at 76, not 80; fixed the test's own expectation rather
  than the (correct) table.
- `internal/vmsdef/gen/main.go` (new) + `internal/vmsdef/constants.go`'s
  `go:generate` directive: parses all three `reference/vms/*.h` headers'
  flat `#define <PREFIX>$<NAME> <value>` lines (case alone reliably
  separates real upper-case value constants from each header's own
  lower-case C-only field-access aliasing macros — see the generator's own
  doc comment) into `internal/vmsdef/constants_generated.go`'s `Constants
  map[string]uint32` — 393 entries (80 FAB$, 45 RAB$, 268 RMS$, the last
  including `RMS$V_STVSTATUS` alongside the 267 `RMS$_` status codes proper).
  Spot-checked generated values against this phase's own planning research
  (`FAB$C_SEQ`=0, `FAB$C_FIX`=1, `FAB$K_BLN`=0x50=80, `RAB$K_BLN`=0x44=68,
  `RMS$_NORMAL`=0x10001=65537, `RMS$_EOF`=0x1827a=98938) — all match.
- Migrated `internal/rms/fab.go`/`rab.go`/`status.go`: their own
  private `const` blocks (offsets, `FAB$M_`/`FAB$C_`/`RAB$C_` bit/code
  values, all 17 `RMS$_` status codes this package's handlers use) became
  `var` blocks initialized from `vmsdef.FABFields`/`RABFields`/`Constants`
  via two small panic-on-miss lookup helpers (`fabOffset`/`rabOffset` in
  fab.go/rab.go, `vmsConst` in fab.go, shared across all three files).
  Cross-checked every one of the 17 `RMS$_` values against
  `constants_generated.go` before touching status.go — all matched
  status.go's own pre-migration literals exactly, confirming the migration
  is value-preserving, not just build-preserving.
- Two small type fallout fixes, both mechanics of switching `const` to `var`
  rather than behavior changes: `facPut`/`facGet`/`facUpd`/`orgSeq`/`orgRel`/
  `orgIdx`/`orgHsh`/`racSeq`/`racKey`/`racRFA` needed an explicit `byte(...)`
  conversion (`vmsConst` returns `uint32`; these are compared against
  `byte`-loaded FAB/RAB field values, and Go's untyped-constant-adapts-
  automatically rule no longer applied once they became typed vars) — a
  compile error caught this immediately, not a latent bug. `status_test.go`'s
  own `TestRMSStatusValuesAreDistinctAndPositive` needed its `map[string]int`/
  `map[int]string` changed to `uint32`, same reason.
- No production behavior changed anywhere in this subtask — every value
  placed into emulated VAX memory or compared against one is identical
  before and after, confirmed by the full existing `internal/rms` suite
  passing unchanged (`fab_test.go`'s own offset/size table in particular,
  which independently re-asserts every offset this migration now sources
  from `vmsdef.FABFields` instead of a literal).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output for any touched
  file), `go test ./...` all clean across the whole module (including the
  peer `ods2` module, reachable via `go.work`).
- Subtask 1 complete. Subtask 2 (`.RMSDEF` pseudo-op) is next.

### 2026-09-23 — Subtask 2: `.RMSDEF` pseudo-op

- `internal/asm/pseudo.go`'s new `pseudoRMSDEF`: iterates
  `vmsdef.Constants` (sorted by name, for deterministic assembly-error
  ordering — map iteration order isn't otherwise meaningful here) plus
  `vmsdef.FABFields`/`RABFields`, defining each as a permanent symbol via
  `setSymbol(..., SymPermanent, false)`. The `unique=false` was chosen
  deliberately from the start this time (not discovered as a bug after the
  fact the way `.P1VECTOR`'s was — see `docs/PHASE-11.md`'s own progress
  log): the design section above already worked out that real `$FABDEF`/
  `$RABDEF`/`$RMSDEF`'s own `set_symbol_direct` calls never raise
  `ASM_UNIQUE`, so a real `.INCLUDE`d-twice library macro is naturally
  idempotent, and `.RMSDEF` should be too.
- Deliberately **not** gated on `.MICROKERNEL`, unlike every other
  pseudo-op that touches `internal/vmsdef` data (`.P1VECTOR`) or deposits
  bytes at all (`.SHIM`/`.SCB`/`.REGION`): `.RMSDEF` deposits nothing into
  the image, purely defines symbols, matching how a real `$FABDEF`
  `.INCLUDE` works in any ordinary user-mode assembly regardless of
  microkernel context — confirmed by a dedicated test
  (`TestPseudoRMSDEFNoMicrokernelRequired`) that assembling a bare
  `.RMSDEF` with no preceding `.MICROKERNEL` statement succeeds.
- New tests (`internal/asm/rmsdef_test.go`):
  `TestPseudoRMSDEFNoMicrokernelRequired`;
  `TestPseudoRMSDEFDefinesEveryConstantAndOffsetSymbol` (a representative
  spot-check sample plus a full sweep over every single
  `vmsdef.Constants`/`FABFields`/`RABFields` entry, confirming each
  resolves to its real value as an assembler symbol);
  `TestPseudoRMSDEFIsIdempotent` (both within one source and across two
  separate top-level `Assemble` calls on the same `Assembler` — the shape
  a persistent console session's own `asmSession` actually produces);
  `TestPseudoRMSDEFSymbolsAreNoBytes` (confirms `Bytes()` is unchanged
  before/after, i.e. genuinely zero image footprint).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no new findings),
  `go test ./...` all clean across the whole module (including the peer
  `ods2` module, reachable via `go.work`).
- Subtask 2 complete. Subtask 3 (`.FAB` pseudo-op) is next.

### 2026-09-23 — Subtasks 3-4: `.FAB`/`.RAB` pseudo-ops

- `internal/asm/pseudo.go`'s new `buildControlBlock` is the shared
  implementation `pseudoFAB`/`pseudoRAB` both call (parametrized by field
  table, block size, and the real `FAB$C_BID`/`FAB$K_BLN` vs.
  `RAB$C_BID`/`RAB$K_BLN` constant names) — subtasks 3 and 4 turned out
  identical in shape once the field-table abstraction from subtask 1 was in
  place, so they landed together rather than as two near-duplicate
  functions. Matches a real `$FAB`/`$RAB` macro's own expansion: writes the
  block's `BID`/`BLN` identification bytes unconditionally first (read
  directly from `vmsdef.Constants` in Go, not through the assembler's own
  symbol table — so `.FAB`/`.RAB` need no preceding `.RMSDEF` to produce a
  correctly self-identifying block), then parses zero or more
  comma-separated `KEYWORD=value` parameters (`scanSetName` plus `=`/`:`/`,`
  separator handling, reusing `.SET`'s own established pattern) against the
  field table, placing each value at its real offset via the existing
  `exprValue`/`addrFixup`/`storeScaled` machinery `.BYTE`/`.WORD`/`.LONG`
  already use — so a keyword's value can be a symbolic constant
  (`ORG=FAB$C_SEQ`, once `.RMSDEF` has defined it) or a forward-referenced
  label (`FNA=fspec`), exactly like `.LONG` already supports. A field whose
  own `Size` isn't 1/2/4 (`RAB$W_RFA`, see `vmsdef.Field.Size`'s own doc
  comment) fails through `storeScaled`'s existing `VAX_BADSCALE` error
  rather than a bespoke one — not worth a new error code for the single
  known case. An unrecognized keyword reuses `VAX_UNDEFSYM` (a keyword
  really is a name lookup that failed, the same shape as an undefined
  symbol reference) rather than inventing a new status.
- Every field not named in the parameter list is left at zero — no
  explicit zero-fill pass needed, matching `.BLKB`'s own existing "never
  written reads back as zero" convention (`internal/asm/image.go`'s sparse
  map).
- New tests (`internal/asm/fabrab_test.go`): `TestPseudoFAB_autoBIDBLN`/
  `TestPseudoRAB_autoBIDBLN` (auto header bytes, correct block-size deposit
  advance); `TestPseudoFAB_keywordsPlaceValues` (several real keywords —
  `FAC`/`ORG`/`RFM`/`MRS`/`FNS` — landing at their real offsets, including a
  symbolic constant value resolved through a preceding `.RMSDEF`);
  `TestPseudoFAB_forwardReferenceValue` (`FNA=fspec` where `fspec` is
  defined later in the same file); `TestPseudoFAB_unknownKeywordErrors`;
  `TestPseudoFAB_unsettableFieldErrors` (`.RAB RFA=1`); and
  `TestPseudoFAB_unwrittenFieldsAreZero`. One test bug caught immediately
  by its own first run (not a bug in the pseudo-op): the `FNS=13` case
  needed rewriting to `FNS=^D13`, since this assembler's default radix is
  hex (`docs/PHASE-11.md`) — a bare `13` means `0x13` (19), the exact
  radix pitfall `testdata/asm/rms_roundtrip.asm`'s own existing `^D13` use
  already documents; the test now cites that same precedent inline.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no new findings),
  `go test ./...` all clean across the whole module (including the peer
  `ods2` module, reachable via `go.work`).
- Subtasks 3-4 complete. Subtask 5 (rewrite `rms_roundtrip.asm`) is next.

### 2026-09-23 — Subtask 5: rewrite `rms_roundtrip.asm` via `.RMSDEF`/`.FAB`/`.RAB`

- Resolved the design section's own open sub-field-addressing question
  concretely, by trying it: a throwaway probe (`movb #1, @#fab+FAB$B_FAC`
  after `fab: .FAB` with no parameters) assembled cleanly and, decoded back
  from the emitted absolute-mode operand bytes, resolved to exactly the
  FAB's base address plus the real `FAB$B_FAC` offset (`0x216` for a FAB at
  `0x200`) — ordinary `label+symbol` expression arithmetic already
  supported this with zero new pseudo-op code, so `.FAB`/`.RAB` don't
  generate per-field sub-symbols; see the design section's own "Resolved
  during subtask 5 planning" note.
- That probe used a *backward* reference (the label was already defined).
  The real fixture rewrite hit a real, pre-existing assembler limitation
  the probe hadn't exercised: `rms_roundtrip.asm`'s code section runs
  *before* its data (matching the original fixture's own layout, data
  after `fail:`), so `@#rab+RAB$L_RBF` in the code was a *forward*
  reference to `rab` combined with an operator — and this assembler's
  forward-reference fixup mechanism (matching the reference tool's own)
  can only defer a bare symbol, not one combined with `+`, failing with
  `VAX-E-OPERANDERR`/`VAX-E-FWDOPERATOR`. Not a bug to fix (a real,
  documented limitation, `internal/vmserrors`' existing `VAX_FWDOPERATOR`)
  — fixed by moving the `.FAB`/`.RAB` declarations up, right after
  `.RMSDEF` and before the code (rather than down with the rest of this
  fixture's data), so `fab`/`rab` are already resolved by the time any
  `label+offset-symbol` expression needs them. `FNA=fspec` inside `.FAB`
  itself stays a *bare* forward reference (fspec is still declared down
  with the rest of the data) and needed no change — only `label+offset`
  combinations need the label pre-resolved.
- Beyond the mechanical `.BLKB`/`.BYTE`/`.WORD`/`.LONG` → `.FAB`/`.RAB`
  swap, used the now-available field table to drop several runtime
  instructions the original fixture needed purely to initialize
  never-changing fields: `RAB$L_FAB` (always pointing at the same `fab`)
  and `RAB$B_RAC` (always `RAB$C_SEQ`, poked identically before both
  CONNECTs in the original) are now `.RAB` keyword values instead of
  `MOVAL`/`MOVL`/`MOVB` instructions run twice. `FAC`'s reopen-time change
  (`FAB$M_GET`) and the genuinely per-call `RAB$L_RBF`/`RAB$W_RSZ` pokes
  remain runtime code, now spelled with symbolic constants/offset symbols
  (`FAB$M_GET`, `RAB$L_RBF`) instead of the original's bare numeric
  literals with only a `;` comment naming them.
- `TestRMSRoundTrip_assembledProgram` and
  `TestRMSRoundTrip_afterKernelAlreadyP1VectoredIsIdempotent`
  (`internal/console/rms_e2e_test.go`, unchanged from Phase 11) both pass
  against the rewritten fixture with no test-side changes needed — the
  concrete, planned acceptance check for this phase.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean across the
  whole module (including the peer `ods2` module, reachable via
  `go.work`). Confirmed by grep that nothing else in the tree referenced
  the old fixture's `fab_fac`/`rab_rac`/... sub-symbol names.
- Subtask 5 complete. Subtask 6 (docs: this progress log, `docs/PLAN.md`)
  is next.
