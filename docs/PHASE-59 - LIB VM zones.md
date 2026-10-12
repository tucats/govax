# Phase 59 — LIB$ virtual memory zones

**Status:** planned (2026-10-12). Needs Phases 53, 54, and 58
(callbacks, for user-defined zones). Part of the LIB$ program laid out
in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The 5.0 manual's chapters 5 and 6: the RTL's memory manager as the
manual describes it. That is pages (GET_VM_PAGE, FREE_VM_PAGE) beneath
blocks (GET_VM, FREE_VM) allocated from **zones**. Each zone has its
attributes (algorithm: first fit, quick fit, frequent sizes, fixed size;
block size and alignment; boundary tags; area sizes and extension; fill
on allocate and free), and there is a default zone. User-defined zones
pass every request to the program's own routines.

govax's heap today is a port of eVAX's. LIB$GET_VM ignores its zone
argument, LIB$FREE_VM returns 4042 (not the manual's LIB$_BADBLOADR) for
an address it didn't allocate, and LIB$DELETE_VM_ZONE frees blocks
tagged with a zone number. DECC$MALLOC shares the same heap. This phase
redesigns it to the manual. The statuses become the manual's
(LIB$_BADBLOADR, LIB$_BADBLOSIZ, LIB$_INSVIRMEM, LIB$_BADZONE, ...), the
statistics routines appear, and the C library's malloc family (the 7.x
VM_MALLOC routines) sits on a zone.

The manual describes interfaces and documented behavior (what the
algorithms promise, alignment, statistics). How blocks are laid out in
memory is govax's, as long as it keeps what the manual promises. The
debugging chapter (6) describes what SHOW_VM_ZONE and VERIFY_VM_ZONE
report.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Zones | CREATE_VM_ZONE (44), CREATE_USER_VM_ZONE (40), DELETE_VM_ZONE (118, redone), RESET_VM_ZONE (329) |
| Blocks | GET_VM (223), FREE_VM (177), both redone |
| Pages | GET_VM_PAGE (225), FREE_VM_PAGE (179) |
| Statistics | SHOW_VM (356), STAT_VM (392) |
| C library (7.x) | VM_MALLOC, VM_FREE, VM_CALLOC, VM_REALLOC; VM_MALLOC_P, VM_FREE_P, VM_CALLOC_P, VM_REALLOC_P, VM_MALLOC_INIT_P: only as far as an interface can be settled; DECC$MALLOC's current heap is their model |
| LIBRTL2 | FIND_VM_ZONE (163), SHOW_VM_ZONE (359), VERIFY_VM_ZONE (464): need LIBRTL2.EXE's offsets |

## What earlier phases leave in place

- `corevms`'s heap (`memory.go`: `AllocateVM`, `FreeVM`, `FreeVMZone`),
  shared by LIB$GET_VM and DECC$MALLOC; `$EXPREG`, `$CRETVA`, and
  `$DELTVA` for pages.
- Phase 54's dynamic strings, which allocate from the default zone.
- Phase 58's callbacks, for user zones.
- LIB$GET_VM's ignored zone and LIB$FREE_VM's 4042 are noted only in
  the code's comments (`internal/librtl/vm.go`), not in DEVIATIONS.md.
  This phase fixes both and records the change there.

## Open questions

- **Where zones live**: zone control blocks in the program's memory (as
  VMS's are, so a zone ID is an address) or in Go with an ID. The
  manual's zone ID is a longword the program keeps. VMS's being an
  address isn't documented, so Go-side state with opaque IDs is likely
  enough.
- **The default zone's attributes** and how DECC$MALLOC's blocks relate
  to it.
- **SHOW_VM_ZONE's and SHOW_VM's report layouts** are for a probe.

## Subtasks (provisional)

1. **Survey**: chapters 5 and 6 and each routine.
2. **Pages**: GET_VM_PAGE, FREE_VM_PAGE.
3. **Zones and the algorithms**: CREATE_VM_ZONE, DELETE_VM_ZONE,
   RESET_VM_ZONE; GET_VM and FREE_VM redone, with the manual's statuses.
4. **The default zone**, dynamic strings and DECC$MALLOC moved onto it.
5. **User zones**: CREATE_USER_VM_ZONE.
6. **Statistics**: SHOW_VM, STAT_VM.
7. **The C library's routines** (if settled).
8. **LIBRTL2's zone routines** (if their offsets are known).
9. **The program** (`testdata/lib59/`), its console test, and the VMS
   probe.
10. **Close-out**: DEVIATIONS.md records the heap's change from the
    eVAX port.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
