# Phase 02: Virtual memory support

## Goal

Implement VAX virtual-to-physical address translation and the load/store primitive
layer that everything else (instruction decode, RTL, console EXAMINE/DEPOSIT) calls
through.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/vm.c` — page tables, address translation.
- `reference/eVAX/eVAX/Source/CPU/storage.c` — `load_memory`/`store_memory`/
  `load_register` primitive layer (note: `get_operand`/`put_operand` operand resolution
  belongs to Phase 03's decode engine, not here, since it's tied to instruction
  operand-specifier parsing).
- `reference/eVAX/eVAX/Headers/pte.h` — page table entry layout (`AUDIT.md` flags a
  `union PTE` sizing issue class — check against the now-fixed C source).
- `reference/eVAX/eVAX/Headers/memmap.h` — memory-mapping macros/offset tables (also
  used by Phase 10's RMS struct mapping).

## Deliverables

- An `internal/vm` package providing address translation and typed load/store methods
  against a `vax.Machine`'s memory, with clear byte/word/longword/quadword accessors
  (avoiding any equivalent of the C `LONGWORD` width-ambiguity bug by construction).
- Unit tests: page table setup, translation faults, load/store round-trips at each
  granularity, boundary/alignment cases.

## Open questions / notes

- ~~Confirm whether `internal/vm` needs a back-reference to `internal/vax.Machine` or
  should own the raw memory buffer itself with `vax.Machine` holding a `*vm.Memory`~~ —
  resolved: `vm.Memory` owns the raw RAM buffer itself, independent of `vax.CPU`. Every
  method that needs MAPEN/base-and-length registers/cur_mod takes a `*vax.CPU`
  parameter instead of holding a reference, consistent with Phase 01's "instantiated
  struct, passed explicitly" state model and with `vax.CPU` being registers/PSL only.
- `memmap.h`'s `struct MAP`/`map()` declarative VAX-struct-to-native-struct mapping
  (used by `structure_mapping.c` for RMS FAB/RAB fields) is not address
  translation/load-store and is out of scope here, as the doc note next to it already
  says — it's Phase 10's concern.
- The C source's translation-buffer/"sequential translation cache" (`struct TB`, the
  `cached_*` STC globals in `vm.c`) is a pure performance optimization with no effect on
  translation results; not ported, same rationale as Phase 01 skipping `struct PSL_W`.
- `DYNVM` dynamic page-in-on-demand (`validate_page()`, gated on
  `vax.console.vminit_valid`/`page_map`) is console/microkernel state owned by Phase
  08's VMINIT command. Until that exists, an invalid PTE always faults
  `TranslationNotValid` here — which is also exactly what `validate_page()` itself does
  whenever `VMVALID` is false (the case until VMINIT has run), so this isn't a fidelity
  deviation, just a scoping one, revisited when Phase 08 lands.
- `vm.c`'s protection check has two implementations selected by `VM_TABLE_ACCESS`: the
  algorithmic one (`VM_TABLE_ACCESS 0`, what the C source actually builds with) and a
  table-driven `access_map[16][4]` alternative (`VM_TABLE_ACCESS 1`, dead code in the
  current build). Only the algorithmic path is ported; hand-checking it against the
  table found they agree on every architecturally-defined protection code and diverge
  only on the reserved code `1`, which no valid PTE should carry — not logged to
  `DEVIATIONS.md` since the active implementation isn't in question.

## Progress Log

### 2026-09-14 — Sub-phase 1: PTE type and physical memory layer

- Added `internal/vm/pte.go`: `PTE uint32` with bit-masking get/set methods for every
  `pte.h` `PTEBITS` field (PFN, software bits, owner, modify, protection, valid), plus
  `Protection` (named constants matching `pte.h`'s `PTE_K_*`) and its unexported
  `allows` method — a direct port of `vm()`'s algorithmic protection check (see Open
  questions above on why the table-driven alternative isn't ported).
  `reference/AUDIT.md` finding C8 (`sizeof(union PTE)` wrongly 8 bytes from the
  `LONGWORD` bug) is confirmed fixed upstream, and moot in Go regardless since `PTE` is
  a plain 32-bit type by construction.
- Added `internal/vm/memory.go`: `Memory` (`NewMemory`, `Size`) owning a flat RAM byte
  slice, a `phys` helper bounds-checking a physical window and a
  `PhysicalAddressError` for one that doesn't fit, and `readPhysLongword`/
  `writePhysLongword` for the untranslated PTE-storage access `Translate` needs.
  Physical resolution beyond RAM (console ROM/NVRAM, memory-mapped I/O) is left to
  Phases 08/09, which own that state.
- Added `internal/vm/pte_test.go`: full-field no-overlap round-trip (each field set to a
  distinct value, cleared one at a time with neighbors verified undisturbed, mirroring
  `vax/psl_test.go`), a direct bit-offset check against the architecture manual/`pte.h`
  layout independent of the accessor methods, and a protection-matrix table exercising
  `allows` across representative protection codes/modes/access types.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and
  `go test ./internal/vm/...` all clean.
