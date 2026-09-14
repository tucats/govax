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

- Confirm whether `internal/vm` needs a back-reference to `internal/vax.Machine` or
  should own the raw memory buffer itself with `vax.Machine` holding a `*vm.Memory` —
  decide based on how cleanly Phase 01's struct inventory separates.

## Progress Log

_Not started._
