# Phase 01: CPU hardware definition

## Goal

Establish the Go equivalent of the C emulator's core machine state — general and
privileged registers, the Processor Status Longword (PSL), and condition codes — as an
instantiated struct with a constructor, matching the "instantiated struct, passed
explicitly" state model locked in during planning (see `docs/PLAN.md`). No instruction
decode/execute yet; this phase is pure data-structure definition plus register
read/write primitives.

## Scope / C source mapping

- `reference/eVAX/eVAX/Headers/vax.h` — `struct VAX`, the single global machine-state
  object in C. Identify which fields belong to "core hardware state" (registers, PSL,
  condition codes) vs. later phases' concerns (VM regions belong to Phase 02;
  fault/interrupt queues to Phase 03; console- and assembler-specific sub-state to
  Phases 08/11) and only port the former here.
- `reference/eVAX/eVAX/Headers/arch.h` — per-platform macros and the `LONGWORD`/
  `ULONGWORD`/`QUADWORD` typedefs. In Go these become plain fixed-width types
  (`int32`/`uint32`/`int64`) — no macro/typedef indirection needed, and no risk of the
  32-vs-64-bit bug `AUDIT.md` documents, since Go's sized integer types make the width
  explicit at every use site.
- `reference/eVAX/eVAX/Source/CPU/registers.c` — register access helpers.

## Deliverables

- An instantiated `vax.CPU` struct in `internal/vax/` with: general registers, PSL
  fields/condition codes, a constructor (`New()`), and register read/write methods.
- Unit tests covering register read/write and PSL/condition-code bit manipulation in
  isolation (no memory or instruction execution involved yet).

## Open questions / notes

- ~~Decide the exact split between `internal/vax` (this phase) and `internal/vm`
  (Phase 02)~~ — resolved by the C source inventory below: everything not registers/PSL
  (VM regions, memory pointers, fault/interrupt queues, console/assembler sub-state)
  is out of scope for this phase and deferred to where it's listed.
- ~~Confirm naming: `Machine` vs `CPU` vs `VAX`, and whether privileged registers get
  their own sub-struct~~ — resolved: `CPU`, no sub-struct. See "Naming decision" below.

## Design notes (from C source inventory)

`struct VAX` (`vax.h`) is much bigger than "core hardware state" — most of it is
console/assembler/VM/fault-queue state that belongs to later phases. The fields that
are genuinely CPU register state, and thus in scope here, are:

- `reg[MAXREG+1]` (`MAXREG` = 63, so 64 slots) — general register file: R0-R15
  architected, R16-R63 reusable temporaries (`arch.h` only names T0-T5 / R16-R21).
- `preg[MAXPRIVREG+1]` (`MAXPRIVREG` = 128, so 129 slots) — privileged register file,
  addressed by number (future MTPR/MFPR in Phase 07) with `arch.h` `#define`d mnemonics
  for the architected subset (KSP, ESP, SSP, USP, ISP, P0BR, P0LR, P1BR, P1LR, SBR,
  SLR, PCBB, SCBB, IPL, ASTLVL, SIRR, SISR, ICCS, NICR, ICR, TODR, RXCS, RXDB, TXCS,
  TXDB, TBDR, MAPEN, TBIA, TBIS, PMR, SID, TBCHK). Sized to match the C array exactly
  (rather than trimming to the architected subset) so Phase 07's bounds-checking
  behavior for MTPR/MFPR has the same room to work with and no fidelity deviation needs
  logging over array size.
- `union PSL` / `struct PSL_BITS` — the processor status longword and its bit fields
  (condition codes N/Z/V/C, T, IV/FU/DV enables, IPL, CUR_MOD/PRV_MOD, IS, FPD, TP,
  CM). The C source also keeps a `struct PSL_W` "wide" shadow copy of these bits synced
  via `read_psl_bits`/`write_psl_bits` purely as a 1999-era speed hack for bitfield
  access; Go has no bitfield access penalty to work around, so `PSL_W` is *not* ported
  — a single `PSL uint32` with bit-masking accessor methods replaces both `union PSL`
  and `struct PSL_W`.

  Bit layout is taken from the non-`BIGENDIAN` branch of `struct PSL_BITS` (declaration
  order = LSB-first for a little-endian host bitfield layout, which is what every
  actually-supported modern build target uses) and cross-checked against the VAX
  architecture manual's PSL diagram — they agree: bits 0-3 = C,V,Z,N; bit4 = T; bits
  5-7 = IV,FU,DV; bits 8-15 = MBZ; bits 16-20 = IPL; bit 21 = MBZ; bits 22-23 = PRV_MOD;
  bits 24-25 = CUR_MOD; bit 26 = IS; bit 27 = FPD; bits 28-29 = MBZ; bit 30 = TP; bit 31
  = CM.

Fields explicitly deferred to later phases: `instruction_PC`, `halted`, `exception`,
`fault*`/`interrupt*` (Phase 03+ decode/execute engine), `memory`/`memsize`/
`rom_base`/`rom_end`/`nvram_*` (Phase 02 VM), `console`/`assembler` sub-structs (Phases
08/11), `region`/`vm_initialized` (Phase 02), `quantum`/`uiquantum`/`clock*` (Phase 09
I/O), `debug` flags (cross-cutting, introduced when first needed).

**Naming decision**: the struct is `vax.CPU` (holding just registers/PSL), not
`vax.Machine` — `internal/vax`'s job per `CLAUDE.md` is specifically "registers, PSL,
condition codes," and reserving `Machine` leaves room for a later top-level type that
composes `vax.CPU` with `vm` memory and other subsystems, without a rename. Privileged
registers do **not** get a separate sub-struct — they're a flat indexed array exactly
like the general registers, and get the same treatment (typed index constants +
indexed accessors), so a sub-struct would add a layer without adding meaning.

## Sub-phases

Each is one buildable, testable commit:

1. **General & privileged register file** — `CPU` struct with `GPR`/`PR` arrays sized
   to match `MAXREG+1`/`MAXPRIVREG+1`, typed `Reg`/`PrivReg` index constants (R0-R15 +
   PC/SP/FP/AP aliases; the architected privileged-register mnemonics), `New()`
   constructor, indexed read/write methods. Unit tests: index constants resolve to the
   right slot, PC/SP/FP/AP alias the same storage as R15/R14/R13/R12, read/write
   round-trips, out-of-range behavior is deliberate (documented, not just whatever Go
   panics with).
2. **PSL and condition codes** — `PSL uint32` type with bit-masking get/set methods for
   every field above, embedded in `CPU`. Unit tests: each field round-trips without
   disturbing neighboring bits, a full-word `Set`/read round-trip, condition-code
   convenience helpers behave per the VAX condition-code semantics they'll be driven by
   in later phases (e.g. Z implies the compared values were equal).
3. **Close-out** — fill in any gaps found while writing sub-phase 1-2 tests (e.g. a
   `Reset()` that zeroes a `CPU`), finish this doc's progress log, confirm
   `go build ./...`, `go vet ./...`, `go test ./...` are all clean, final commit for
   the phase.

## Progress Log

### 2026-09-14 — Sub-phase 1: general & privileged register file

- Inventoried `struct VAX` against the "core hardware state" scope for this phase; see
  Design notes above for what's in/out and why.
- Added `internal/vax/registers.go`: `Reg`/`PrivReg` typed indices (architected general
  registers + PC/SP/FP/AP aliases; the `arch.h`-named privileged register subset),
  sized to match the C source's `MAXREG`/`MAXPRIVREG` exactly, plus `CPU` (`New`,
  `Reset`, `GPR`/`SetGPR`, `PR`/`SetPR`).
- Added `internal/vax/registers_test.go`: alias-to-index mapping, read/write
  round-trips for both register files, alias/named-register storage-sharing checks,
  and `Reset`.
- `go build ./...`, `go vet ./...`, `go test ./internal/vax/...` all clean.

### 2026-09-14 — Sub-phase 2: PSL and condition codes

- Added `internal/vax/psl.go`: `PSL uint32` with bit-masking get/set methods for every
  field in `struct PSL_BITS` (condition codes N/Z/V/C, T/IV/FU/DV enables, IPL,
  CUR_MOD/PRV_MOD via a new `AccessMode` type, IS, FPD, TP, CM), plus `SetNZVC` as the
  common "instruction just computed a result" convenience. `struct PSL_W` (the C
  source's cached-bits-as-longs speed hack) is deliberately not ported — see Design
  notes above.
- Wired `psl PSL` into `CPU`, with `PSL()`/`SetPSL()` whole-word accessors.
- Added `internal/vax/psl_test.go`: condition-code round-trips, a full-field
  no-overlap check (every field set to a distinct value, then cleared one at a time
  with neighbors verified undisturbed), IPL/access-mode masking of out-of-range input,
  whole-word round-trip, and a direct bit-offset check of the layout against the
  architecture manual's PSL diagram (independent of the accessor methods, as a guard
  against a future refactor silently shifting a field).
- `go build ./...`, `go vet ./...`, `go test ./internal/vax/...` all clean.

### 2026-09-14 — Sub-phase 3: close-out

- Reviewed sub-phases 1-2 for gaps against the phase Goal/Deliverables: `New`/`Reset`
  already cover the "constructor" deliverable and zero the PSL along with both
  register files, so no further additions were needed.
- Resolved the two open questions from planning in place (struct naming → `CPU`, no
  privileged-register sub-struct; `vax`/`vm` split → everything but registers/PSL is
  out of scope here) by cross-referencing them to the Design notes section above where
  they're explained.
- Full-repo `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and
  `go test ./...` all clean; phase complete.
