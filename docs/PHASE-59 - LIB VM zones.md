# Phase 59 — LIB$ virtual memory zones

**Status:** in progress (subtasks 1 to 5 done 2026-10-14).
Needs Phases 53, 54, and 58 (callbacks, for user-defined zones). Part
of the LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part
A).

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

- **Where zones live** (proposed, Decision 1 below): each zone has a
  control block in the program's memory, from the page pool, whose
  address is the zone ID, as VMS's are (the manual's sample output shows
  zone IDs like `0001B858`); the zone's state is kept in Go, found by
  that address. Blocks' bookkeeping is Go's too (Decision 2).
- **The default zone's attributes** (proposed, Decision 5): Table 5-3's,
  with what the probe shows (SHOW_VM_ZONE's sample adds EXTEND_AREA);
  dynamic strings go to a second, string zone, as FIND_VM_ZONE's
  description says VMS's do; DECC$MALLOC to the default zone until the
  probe says otherwise.
- **SHOW_VM_ZONE's and SHOW_VM's report layouts** are for the probe.
- **CREATE_VM_ZONE's argument list**: the 5.0 manual has 13 arguments
  (`number-of-areas` twelfth), the VSI manual 12 (no `number-of-areas`;
  `get-page` twelfth). Which VMS 7.3 has is a probe case.
- **The VM_MALLOC family's interfaces**: neither manual documents them.

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey; page numbers are the manual's LIB-nnn. Chapters 5
("Memory Allocation Routines", pages 5-1 to 5-22) and 6 ("Debugging
Programs That Use Virtual Memory Zones", 6-1) give the model the
routines share. Choices the manuals leave open are marked
**unconfirmed**, and each is a case for the probe. Beside the routines,
this survey reviews the heap govax has today (it is not a sound base to
build on) and proposes a new subtask 2 that replaces it first.

Every routine is in VMS 7.3's transfer vectors (`vmsdef.ImageSymbols`):

| Routine | Image, offset | govax today |
| --- | --- | --- |
| SHOW_VM, STAT_VM | LIBRTL 0x520, 0x528 | missing-routine stubs |
| FREE_VM, GET_VM | LIBRTL 0x548, 0x550 | shims 30, 29 (`librtl/vm.go`) |
| CREATE_VM_ZONE, CREATE_USER_VM_ZONE | LIBRTL 0xA30, 0xA38 | stubs |
| RESET_VM_ZONE, DELETE_VM_ZONE | LIBRTL 0xA40, 0xA48 | stub; shim 31 |
| GET_VM_PAGE, FREE_VM_PAGE | LIBRTL 0xA50, 0xA58 | stubs |
| VM_MALLOC, VM_FREE, VM_CALLOC, VM_REALLOC | LIBRTL 0xB28 to 0xB40 | stubs |
| VM_MALLOC_P, VM_FREE_P, VM_CALLOC_P, VM_REALLOC_P, VM_MALLOC_INIT_P | LIBRTL 0xB48 to 0xB68 | stubs |
| FIND_VM_ZONE, SHOW_VM_ZONE, VERIFY_VM_ZONE | LIBRTL2 0x18, 0x20, 0x28 | stubs |

LIBRTL2's offsets are known, so subtask 9 (was 8) isn't conditional.
`LIB$$GET_VM_POINTERS` (0xA78) is internal and stays not planned (Phase
53, Part A).

### Sources

- **The 5.0 LIB$ manual** (`AA-LA76A-TE`, its OCR text) is the
  reference. The OCR's errors that matter here: "VM-20NE" and "VM_ZQNE"
  for VM_ZONE, "GET_VMJAGE" for GET_VM_PAGE, "LIB$VERIFV_VM_ZONE",
  "LIB$_1NSVIRMEM" and "LIB$_1NVOPEZON" (1 for I), "$$$_NORMAL" in
  SHOW_VM_ZONE's list, and "LIB$K_VM_FJRST_FIT". CREATE_VM_ZONE's
  algorithm-argument table runs its rows together; the VSI manual sets
  it out (and agrees).
- **The VSI OpenVMS RTL Library (LIB$) Manual**. It agrees with the 5.0
  text but where marked **VSI** below; it describes current OpenVMS, so
  what it adds is a probe case. Most of its additions are the `_64`
  routines (64-bit zones), which the VAX doesn't have, and "pagelets"
  for pages (the same 512 bytes on a VAX).
- **Neither manual documents the VM_MALLOC family.** Their names come
  only from LIBRTL.EXE's transfer vector.

### Codes

From `vmsdef` (STARLET.OLB's definition modules, `LibrarySymbols`; the
conditions are in `Symbols` too):

| Name | Value | Use |
| --- | --- | --- |
| LIB$_INSVIRMEM | %X158214 | insufficient virtual memory |
| LIB$_INVSTRDES | %X158224 | zone-name's descriptor |
| LIB$_INVARG | %X158234 | an attribute out of range; STAT_VM's code |
| LIB$_BADBLOADR | %X158264 | bad block address; an invalid zone ID (GET_VM, DELETE_VM_ZONE, RESET_VM_ZONE) |
| LIB$_BADBLOSIZ | %X15826C | a size of 0 or less; wrong size for a fixed-size zone |
| LIB$_NOTFOU | %X158274 | FIND_VM_ZONE's end (alternate success) |
| LIB$_WRONUMARG | %X15835C | FIND_VM_ZONE, SHOW_VM_ZONE, VERIFY_VM_ZONE |
| LIB$_BADTAGVAL | %X1583B4 | a corrupted boundary tag (FREE_VM, **VSI**) |
| LIB$_INVOPEZON | %X1583BC | a user zone without the routine for the operation |
| LIB$_PAGLIMEXC | %X1583C4 | the zone's page limit exceeded |
| LIB$_BADZONE | %X1583D4 | not a valid zone (FIND, SHOW, VERIFY) |

The constants are all there: LIB$K_VM_FIRST_FIT 1, QUICK_FIT 2,
FREQ_SIZES 3, FIXED 4; LIB$M_VM_BOUNDARY_TAGS 1, GET_FILL0 2, GET_FILL1
4, FREE_FILL0 8, FREE_FILL1 %X10, EXTEND_AREA %X20, and **NO_EXTEND
%X40 and TAIL_LARGE %X80**, which the 5.0 manual reserves ("bits 6
through 31 are reserved and must be 0") but VMS 7.3's definitions have;
the VSI manual describes them (below). govax's STARLET.MLB has no macro
that defines them; LINK resolves them from STARLET.OLB, so the probe
program can use them as globals or define them itself.

### The model (chapter 5)

- **Three levels.** The memory management services (`$EXPREG`,
  `$CRETVA`, `$DELTVA`, ...); the page routines GET_VM_PAGE and
  FREE_VM_PAGE, which keep a **processwide pool of free pages** and call
  `$EXPREG` (P0) when none fit; and the heap routines, whose blocks come
  from **zones**. "Memory that is allocated by an allocation routine at
  one level of the hierarchy must be freed by calling a deallocation
  routine at the same level."
- **Pages** (5.2). Page aligned, user-mode read/write "even if the call
  ... is made from a more privileged access mode", contents
  unpredictable. A group may be freed in parts, and contiguous groups
  from several calls freed together. Freeing pages that aren't
  allocated (never, or already freed) or aren't page aligned is an
  error. The pool uses "a bitmap allocation method" and is meant for
  requests of one to a few hundred pages.
- **Zones and areas** (5.3). A zone is "a logically independent memory
  pool or subheap". It gets pages in contiguous groups, **areas**, from
  GET_VM_PAGE (or its own get-page routine, below). When no free block
  and no area's unallocated end can satisfy a request, a new area of
  `max(extend-size, request)` pages is added; with EXTEND_AREA, new pages
  adjacent to an existing area are appended to it. "Each area control
  block is 64 bytes long" (Table 5-1's overheads: 12.5% of a 1-page
  area, 0.1% of 128 pages). The attributes are fixed at creation.
- **Attributes** (5.3.1, and CREATE_VM_ZONE):

  | Attribute | Rule | Default |
  | --- | --- | --- |
  | algorithm | 1 to 4 | 1, first fit |
  | algorithm-argument | quick fit: lists, 1 to 128; frequent sizes: lists, 1 to 16; fixed: the block size, > 0; required for 2 to 4 | (first fit ignores it) |
  | flags | bits 0 to 5 (and 6, 7: **VSI**) | 0 |
  | extend-size | pages, 1 to 1024 (**VSI**: ≥ 1) | 16 |
  | initial-size | pages; 0 or omitted: none until the first GET_VM; negative: LIB$_INVARG (**VSI**) | 0 |
  | block-size | power of 2, 8 to 512; requests are rounded up to a multiple | 8 |
  | alignment | power of 2, 4 to 512 | 8 |
  | page-limit | pages, 0 to 32,767 (**VSI**: ≥ 0); 0: none; needs initial-size | 0 |
  | smallest-block-size | quick fit: the smallest size with a lookaside list | block-size |
  | zone-name | any string class | none |
  | number-of-areas | (5.0 only; nothing says what it does) | |
  | get-page, free-page | procedures with GET_VM_PAGE's and FREE_VM_PAGE's arguments (**VSI**); 0: those routines | |

  Block size and alignment: without boundary tags, "the larger value of
  block size and alignment controls both"; with tags, block size 8 and
  alignment 4 minimize the tags' overhead. **NO_EXTEND** (**VSI**):
  never extended beyond initial-size, which must be given;
  extend-size unused. **TAIL_LARGE** (**VSI**): an area larger than
  extend-size goes to the end of the area list.
- **Boundary tags.** Two longwords added to each block ("a minimum of
  eight bytes"), invisible to the caller (the address returned is the
  first usable byte). They give the block's length, so FREE_VM needs no
  size, and they "encode the area", so freeing needn't search; free
  blocks are merged with their neighbors by the tags, and an unmerged
  block goes to the free list's tail, unsorted.
- **Fill.** GET_FILL0/1 fill each byte allocated with 0 or FF;
  FREE_FILL0/1 each byte freed (FREE_VM, and every allocated block
  DELETE_VM_ZONE and RESET_VM_ZONE free), but "part of a free block is
  used to store control information, so some bytes will not contain the
  fill value".
- **The algorithms** (5.5). GET_VM tries a lookaside list, then each
  area in turn (its free list, then its unallocated end), then adds an
  area. FREE_VM puts the block on a lookaside list if one fits, else
  finds its area (by the tags, or by searching the areas) and inserts it
  in the area's free list, merging with free neighbors.
  - **First fit**: a free list sorted by address (unless tagged); the
    first block large enough, split, the fragment kept.
  - **Quick fit**: lookaside lists indexed by size for the n sizes from
    smallest-block-size up by block-size; other sizes first fit.
  - **Frequent sizes**: n lookaside lists whose sizes are those of the
    blocks freed (a match, or an empty list takes the size); else first
    fit.
  - **Fixed size**: a single queue of free blocks, no splitting or
    merging.
  - Blocks on lookaside lists are never split or merged.
- **The default zone** (5.3.2, Table 5-3): ID 0, first fit, 128-page
  areas, block size 8, quadword aligned, no tags, no page limit, no
  fill. DELETE_VM_ZONE and RESET_VM_ZONE of zone 0 are errors.
  FIND_VM_ZONE returns it (and "the string zone" second) by a real ID:
  its sample shows `ZONE_ID = 0001B858, ZONE_NAME = "DEFAULT_ZONE"`, and
  SHOW_VM_ZONE's sample of the default zone shows flags `00000020`
  (EXTEND_AREA), initial size 124 pages (**unconfirmed**: the OCR may
  have garbled 128), extend size 128.
- **Zone IDs** (5.3.3): "a unique 32-bit zone identifier"; GET_VM's
  zone must be FREE_VM's ("an error status" otherwise).
- **Reentrancy.** Every routine is "fully reentrant", callable at AST
  level. In govax a shim runs to completion on the engine's goroutine,
  so an AST can't arrive inside one; a user zone's routine or a
  get-page routine can be interrupted (it's VAX code), and the zone must
  be consistent at each callback.
- **Debugging** (chapter 6): "this information is implementation-
  dependent". Run with both free fills and compare; FIND_VM_ZONE at
  image exit lists zones not deleted; SHOW_VM_ZONE and VERIFY_VM_ZONE
  check the structures. **"Freeing a block more than once returns
  SS$_NORMAL"** in a fixed-size zone or for a block that goes to a
  lookaside list, leaving the structures invalid; first fit "checks to
  see if you are freeing a block that is already on the free list, and
  if so, returns the error LIB$_BADBLOADR".

### GET_VM_PAGE (225) and FREE_VM_PAGE (179)

    LIB$GET_VM_PAGE  number-of-pages.rl.r ,base-address.wa.r
    LIB$FREE_VM_PAGE number-of-pages.rl.r ,base-address.ra.r

- GET_VM_PAGE: LIB$_BADBLOSIZ for a count ≤ 0; LIB$_INSVIRMEM when
  `$EXPREG` can't ("no partial allocation"). FREE_VM_PAGE: LIB$_BADBLOSIZ
  for ≤ 0; LIB$_BADBLOADR for pages not allocated, freed already, or an
  address not on a page boundary.
- **Unconfirmed**: whether consecutive one-page calls return adjacent
  pages, and in which direction; how much the pool grows by at a time
  (exactly the request, or more); whether a freed group is reused
  first-fit by address; whether a FREE_VM_PAGE that fails part way frees
  any of the pages; what the pool does with a page `$DELTVA` deleted.

### CREATE_VM_ZONE (44)

    LIB$CREATE_VM_ZONE zone-id.wl.r [,algorithm.rl.r]
        [,algorithm-argument.rl.r] [,flags.rlu.r] [,extend-size.rl.r]
        [,initial-size.rl.r] [,block-size.rl.r] [,alignment.rl.r]
        [,page-limit.rl.r] [,smallest-block-size.rl.r]
        [,zone-name.rt.dx] [,number-of-areas.rl.r]
        [,get-page.szem.v] [,free-page.szem.v]

- Statuses: SS$_NORMAL, LIB$_INSVIRMEM, LIB$_INVARG, LIB$_INVSTRDES. "If
  an error status is returned, the zone is not created."
- **Unconfirmed**: the argument list's length and get-page's position
  (5.0: 13, with number-of-areas; **VSI**: 12, without); which of the
  attribute rules give LIB$_INVARG (each bound, a block size or
  alignment not a power of 2, a missing algorithm-argument, page-limit
  without initial-size, a reserved flag bit, both fill bits of a pair, a
  fixed size of 0, a quick fit range past what a list holds); the zone
  ID's form (an address, and of what); when get-page is called (at
  creation for initial-size, at each extension) and what a failing
  get-page gives; whether initial-size beyond page-limit fails.
- The manual's three "restrictions" (a shared zone mapped by
  `PPL$CREATE_SHARED_MEM`, which "cannot expand") concern PPL$'s shared
  zones, which govax doesn't have.

### CREATE_USER_VM_ZONE (40)

    LIB$CREATE_USER_VM_ZONE zone-id.wl.r [,user-argument.rlu.r]
        [,user-allocation-procedure.szem.v]
        [,user-deallocation-procedure.szem.v]
        [,user-reset-procedure.szem.v] [,user-delete-procedure.szem.v]
        [,zone-name.rt.dx]

- GET_VM, FREE_VM, RESET_VM_ZONE, and DELETE_VM_ZONE of the zone call
  the user's routine instead: `user-rtn num-bytes ,base-adr
  ,user-argument` for get and free (all by reference; base-adr written
  by get, read by free), `user-rtn user-argument` for reset and delete,
  and **the routine's status is the heap routine's**. The zone stores a
  copy of the user argument's value ("copies the value of
  user-argument"), and passes it by reference. A routine not given is
  LIB$_INVOPEZON. Statuses of the create: SS$_NORMAL, LIB$_INSVIRMEM,
  LIB$_INVSTRDES.
- The manual's Example 5-1 passes LIB$DELETE_VM_ZONE itself as the
  delete routine, with the real zone's ID as user-argument: so the
  delete routine's argument is the address of a longword that holds the
  user argument.
- **Unconfirmed**: whether the user zone is gone after DELETE_VM_ZONE
  whatever its routine returns; whether GET_VM checks the size (≤ 0)
  before calling the user's routine; what FREE_VM passes when its size
  is omitted; which address the user argument is passed at (one in the
  zone's control block, presumably, the same each call); whether
  SHOW_VM and STAT_VM count calls that went to user routines.

### DELETE_VM_ZONE (118) and RESET_VM_ZONE (329)

    LIB$DELETE_VM_ZONE zone-id.rl.r
    LIB$RESET_VM_ZONE  zone-id.rl.r

- DELETE returns all the zone's pages to the page pool; RESET frees
  every block but keeps the pages ("the memory is not returned to the
  processwide page pool"). Both fill the freed blocks with a free fill.
  Both are LIB$_BADBLOADR for an invalid zone ID (**VSI** adds "or a
  corrupted zone" to DELETE's) and for zone 0 (5.3.3: "an error status"
  for the default zone; that it's BADBLOADR is **unconfirmed**).
- **Unconfirmed**: a zone ID used after its zone is deleted (BADBLOADR,
  or BADZONE, or nothing detected); a zone made with get-page and
  free-page: does DELETE call free-page per area.

### GET_VM (223) and FREE_VM (177)

    LIB$GET_VM  number-of-bytes.rl.r ,base-address.wa.r [,zone-id.rlu.r]
    LIB$FREE_VM [number-of-bytes.rl.r] ,base-address.ra.r [,zone-id.rlu.r]

- GET_VM: rounded up to the zone's block size, aligned by its alignment,
  filled if asked, user-mode read/write. Statuses: SS$_NORMAL;
  LIB$_INSVIRMEM ("no partial allocation"); LIB$_BADBLOSIZ (≤ 0, or more
  than a fixed-size zone's size); LIB$_BADBLOADR (an invalid zone ID;
  **VSI**: "or a corrupt zone"); LIB$_PAGLIMEXC.
- FREE_VM: the whole block, by one call, in the zone it came from; the
  size is rounded as GET_VM's and may be omitted in a tagged zone.
  Statuses: SS$_NORMAL; LIB$_BADBLOADR (outside the zone's areas,
  misaligned, freed already); LIB$_BADBLOSIZ (≤ 0, or wrong for a
  fixed-size zone); LIB$_BADTAGVAL (**VSI**).
- A zone-id argument omitted or holding 0 is the default zone.
- **Unconfirmed**: what FREE_VM does with a size different from the
  block's (VMS's first fit trusts the caller's size, presumably, so a
  smaller one would keep the rest allocated and a larger one free part
  of the next block); FREE_VM of a block with the wrong zone ID (the
  status); FREE_VM of an interior address; a double free in each
  algorithm (chapter 6 says NORMAL for lookaside lists and fixed size,
  BADBLOADR for first fit); a fixed-size zone's GET_VM of fewer bytes
  than its size (allowed, by the manual's wording); the smallest block
  and overhead in each algorithm (the distance between consecutive
  blocks); whether a fresh area's first block starts after a 64-byte
  control block; INSVIRMEM versus PAGLIMEXC at the limit.

### SHOW_VM (356) and STAT_VM (392)

    LIB$SHOW_VM [code.rl.r] [,user-action-procedure.szem.v]
        [,user-specified-argument.rz.v]
    LIB$STAT_VM code.rl.r ,value-argument.wz.r

- The statistics, by code: 1 successful GET_VM calls, 2 successful
  FREE_VM calls, 3 bytes allocated by GET_VM and not freed, 5 GET_VM_PAGE
  calls, 6 FREE_VM_PAGE calls, 7 pages allocated and not freed. SHOW_VM's
  0 (or omitted) shows 1 to 3 on one line, `mmm calls to LIB$GET_VM, nnn
  calls to LIB$FREE_VM, ppp bytes still allocated`, and 4 shows 5 to 7.
  STAT_VM returns one; 0, 4, or another code is LIB$_INVARG.
- SHOW_VM writes with LIB$PUT_OUTPUT, or calls the action routine with
  the line (by descriptor, "the first character is a blank", no CR/LF)
  and the user argument by value (0 when omitted), and returns the
  action routine's status.
- **Unconfirmed**: code 4's text, and each single code's; the numbers'
  widths; whether 4 is one line or three, one action call or three;
  whether the statistics count every zone's calls or the default zone's,
  RTL-internal allocations (dynamic strings) and user zones' calls;
  whether "bytes" are those asked for or those rounded; whether an
  unsuccessful call counts in 5 and 6 (the manual says "calls", not
  "successful calls", for them).

### FIND_VM_ZONE (163), SHOW_VM_ZONE (359), VERIFY_VM_ZONE (464)

    LIB$FIND_VM_ZONE   context.mlu.r ,zone-id.wlu.r
    LIB$SHOW_VM_ZONE   zone-id.rlu.r [,detail-level.rl.r]
        [,user-action-procedure.szem.v] [,user-arg.rz.v]
    LIB$VERIFY_VM_ZONE zone-id.rlu.r

- **FIND_VM_ZONE**: context 0 starts the scan, which first verifies the
  database (VERIFY_VM_ZONE); then each call returns the next zone (the
  default zone, the string zone, then the rest) and LIB$_NOTFOU at the
  end. Statuses: SS$_NORMAL, LIB$_BADZONE, LIB$_NOTFOU,
  LIB$_WRONUMARG.
- **SHOW_VM_ZONE**: detail level 0 (ID and name), 1 (with algorithm,
  flags, sizes), 2 (and the cache, the lookaside lists, and an area
  summary), 3 (and queue validation); lines go to LIB$PUT_OUTPUT or the
  action routine (`string, user-arg`; a failure stops it, successes are
  ignored). Zone 0 is the default zone. The manual shows level 1 of the
  default zone and level 3 of a quick fit zone, and the error and
  warning lines (`**** ERROR -- ... ****`, the link analysis and the
  block dump) for a corrupted list. Statuses: SS$_NORMAL, LIB$_NOTFOU
  (sic), LIB$_BADZONE, LIB$_INVARG, LIB$_INVOPEZON, LIB$_WRONUMARG, and
  the action routine's or LIB$PUT_OUTPUT's.
- **VERIFY_VM_ZONE**: checks the zone header, lookaside lists, and free
  lists, and with a free fill the free blocks' contents; stops at the
  first error. Statuses: SS$_NORMAL, LIB$_BADZONE, LIB$_INVARG,
  LIB$_WRONUMARG (and LIB$_INSVIRMEM, **VSI**).
- **Unconfirmed**: every layout but the two samples (and those need
  checking: the samples differ in case, `ZONE_ID = ...` against `Zone
  Id = ...`); what a user zone shows; the string zone's attributes;
  which zones VMS's RTL has made before the program's first call (DCL's
  image activation, the C RTL); what VERIFY reports for a free-fill
  mismatch (BADZONE, presumably).

### The VM_MALLOC family

LIB$VM_MALLOC, VM_FREE, VM_CALLOC, VM_REALLOC, and the `_P` forms and
VM_MALLOC_INIT_P, are in VMS 7.3's LIBRTL transfer vector but in neither
manual. By their names they are the C library's malloc, free, calloc,
and realloc on a LIB$ zone (the C RTL's own, presumably). The probe can
try C's conventions (sizes and pointers by value, the result in R0) and
look at FIND_VM_ZONE before and after; the `_P` forms' extra argument
(a zone, a pool?) can't be settled by a probe alone. Proposed: VM_MALLOC,
VM_FREE, VM_CALLOC, VM_REALLOC if the probe confirms C's conventions;
the `_P` forms stay missing-routine stubs unless it settles them too.

### The heap govax has today

`corevms/memory.go` (`shimDeccMalloc`, `freeBlock`) is one heap per
process, its bookkeeping in Go (`Environment.memAllocated`,
`memFreed`), shared by DECC$MALLOC/DECC$FREE, LIB$GET_VM/FREE_VM/
DELETE_VM_ZONE (`librtl/vm.go`), and the RTL's own blocks through
`Environment.AllocateVM`, `FreeVM`, and `BlockSize` (dynamic strings,
`dynstr.go`; FIND_FILE's, FILE_SCAN's, and the file walk's blocks; the
date and time contexts; timers; GET_ACCNAM's table). Image rundown
forgets it (`releaseHeap`). It was ported as eVAX had it, and reading
it for this phase shows it isn't sound. In order of harm:

1. **Live blocks overlap.** The free-list search takes a block at least
   as large as the request (`p.size < size`) and then marks it with the
   rounded size (`p.size = roundedSize`), which can be larger than the
   free block was. Rounding is to the next multiple of 16 above the size
   (`(size+16) &^ 0xF`), and free blocks are multiples of 16, so a
   request of a multiple of 16 bytes into a free block of exactly that
   size takes 16 bytes of whatever follows. This needs no frees: a
   page's unused tail is a free block too. A throwaway test (rounds of
   50 random sizes from 8 to 207 bytes, then 90% freed in random order)
   had two live blocks sharing bytes in its first round, before any
   free, and 111 overlapping pairs of blocks after 200 rounds. Two
   callers then own the same memory: a dynamic string and a FIND_FILE
   block, say.
2. **Free blocks merge with at most one neighbor.** `freeBlock` stops at
   the first adjacent free block it finds, and the merged block isn't
   checked again, so a block freed between two free blocks leaves two
   free blocks where there should be one (shown by the same test).
   Fragmentation accumulates: the test's heap grew to 11,776 bytes for
   4,272 bytes in use, with 9,392 bytes in 45 free blocks.
3. **"First fit" isn't by address.** The free list is a slice in the
   order blocks were freed or split; the first block that fits is the
   first in that order. Allocations scatter, and the low-address reuse
   first fit relies on to keep fragmentation down doesn't happen.
4. **Rounding wastes and wraps.** Every request gains at least one byte
   before rounding, so 16 bytes take 32 and 512 bytes take 528; size 0
   gets a block; a size of %XFFFFFFF0 or more wraps to a 0-byte block. A
   new page run is `(size+512) &^ 0x1FF`, a page more than needed when
   the size is a multiple of 512.
5. **Growth has no bound.** New space is taken by moving P0's
   high-water mark (`RegionSize[0]`), never checked against P0's page
   table length (`ProcessSpace.P0Pages`, VMINIT's size): past it the
   heap hands out addresses that fault when used, and nothing returns
   INSVIRMEM. (`$EXPREG` of P0, `core.go`, has the same gap.)
6. **Freeing doesn't validate.** Any address inside a block frees the
   whole block (an interior pointer is accepted); the size and zone
   arguments are ignored; an unknown address is 4042, not
   LIB$_BADBLOADR.
7. **Zones are a tag nobody sets.** LIB$GET_VM tags every block 0, so
   LIB$DELETE_VM_ZONE of any nonzero ID does nothing, and of zone 0 (an
   error, by the manual) frees every block in the heap: every program
   block, malloc block, dynamic string, and RTL context.
8. **Failure is an address.** When filling fails, the allocator returns
   %XFFFFFFFF as the block's address; DECC$MALLOC returns it to C
   (malloc's failure is NULL), and `dynstr.go` has to test for both 0 and
   %XFFFFFFFF.
9. **Cost.** Freeing and BlockSize search every allocated block, and
   allocation every free block: linear per call, quadratic for a program
   with thousands of blocks.
10. The `libvm*` flags (virgin, carved, zeroed) are written and never
    read.

Items 1 and 2 are defects whatever this phase decides: memory
corruption a program can't avoid. The rest the manual's model replaces.
None is an ISA question, so by the bug-fixing policy they're fixed, not
replicated, and DEVIATIONS.md records the change from the eVAX port.

### The foundation (proposed new subtask 2)

Zones, lookaside lists, statistics, and SHOW_VM_ZONE all stand on two
mechanisms: a page pool and a block allocator for an area. They should
be correct, coalescing, and tested on their own before anything is
built on them, and the RTL's current users moved onto them, so that
subtask 3 onward is the manual's interfaces and nothing else. Proposed:

- **A leaf package** (`internal/vmheap`, say), pure Go like
  `internal/sched` and `internal/lck`: no CPU or memory of its own. It
  reaches the process through two small interfaces, one for pages
  (`$EXPREG`'s code; later a zone's get-page and free-page routines) and
  one for filling memory, so its tests use fakes.
- **The page pool**: allocated pages as a bitmap (or interval set) over
  the pool's ranges, free pages as address-ordered runs merged with both
  neighbors when freed. First fit by address; when nothing fits, the
  pool grows by `$EXPREG`, and new pages adjacent to the top free run
  join it. Partial frees and frees across groups (the manual requires
  both) fall out of the bitmap; freeing a page not allocated is
  detected (LIB$_BADBLOADR) before anything changes.
- **An area's block allocator**: free extents kept sorted by address
  (a slice searched by binary search is enough at these sizes), a
  freed block merged with *both* neighbors, the area's unallocated end
  kept apart from the free list (5.5's order: free list, then the end);
  allocated blocks in a map by address (exact address and size, so an
  interior pointer, a double free, or a wrong size is detected, and
  BlockSize is a lookup). Rounding and alignment by the zone's block size
  and alignment, with a fragment before an aligned start returned to the
  free list.
- **An invariant checker** (no overlaps; no two adjacent free extents;
  alignment; free plus allocated plus unallocated equals each area;
  the pool's bitmap agrees with its runs), which the tests run after
  every operation, and which VERIFY_VM_ZONE can use later.
- **Tests**: unit tests of each rule, and long randomized workloads
  (mixed sizes, random free orders, partial page frees) with the
  checker after each step, plus a fragmentation bound: after freeing
  everything, one free extent per area and the pool's pages all free.
  The workload above as a regression case.
- **Moved onto it**: `AllocateVM`, `FreeVM`, `BlockSize`, DECC$MALLOC
  and DECC$FREE, and LIB$GET_VM/FREE_VM as they are today (one
  first-fit heap, block size 8, quadword aligned: Table 5-3's default
  zone, without the zone routines yet); DECC$MALLOC returns 0 on
  failure; the heap's pages from the pool, and the pool bounded by P0's
  page table (with `$EXPREG` of P0 checked the same way: SS$_VASFULL, as
  `$CRETVA` answers past the table). `releaseHeap` drops the pool too.
- **Risk**: block addresses change (8-byte rounding, no extra 16
  bytes), so govax-only tests that pin heap addresses change with them.
  The oracles compare VMS's output, where heap addresses are masked or
  not printed; none should move, and any that does is a finding.

### Proposed decisions

Accepted as proposed (2026-10-14).

1. **A zone ID is the address of a control block in P0**, allocated
   from the page pool when the zone is made, as VMS's are; it holds a
   validity longword and the user zone's user argument (passed by
   reference from there). The zone's state is in Go, found by the
   address, so a longword that isn't a zone's ID is detected by a map
   lookup (BADZONE, BADBLOADR) without trusting memory. The control
   block's size, and each area's 64-byte control block at the area's
   start, are reserved so that addresses, overheads, and SHOW_VM_ZONE's
   counts come out as VMS's would; their contents beyond the validity
   longword are govax's.
2. **Blocks' bookkeeping is in Go**, not in the free blocks (chapter 6:
   the layout is "implementation-dependent"). A program that writes past
   its block damages its own data, not the heap. Boundary tags take
   their two longwords and hold the block's size; FREE_VM without a size
   uses Go's record, and a tag the program changed is LIB$_BADTAGVAL
   (**VSI**; **unconfirmed** for 7.3). With nothing in memory to
   corrupt, SHOW_VM_ZONE's link analysis has nothing to find; its
   free-fill check and VERIFY_VM_ZONE's read memory and are real.
3. **Double frees** are detected everywhere (Go knows every block); the
   status is what the probe shows per algorithm (chapter 6: BADBLOADR
   for first fit, NORMAL for lookaside and fixed size, where govax then
   leaves the zone as it was rather than corrupting it).
4. **User zones are tail calls.** GET_VM, FREE_VM, RESET_VM_ZONE, and
   DELETE_VM_ZONE of a user zone end in a call of the user's routine
   whose status is theirs: Phase 55's `CallRequest.Tail`, as SHOW_TIMER's
   action routine is, so the routines needn't be in `callsBack` and each
   GET_VM doesn't start a goroutine (Phase 58's mechanism starts one per
   call of a registered routine). A zone's get-page and free-page
   routines are true callbacks (the allocation continues after them);
   for those, corevms grows a way for a plain shim to hand its call to
   the callback machinery when it finds it needs it (`startShimCall`
   exists; it needs an exported entry), so only those zones pay.
   SHOW_VM_ZONE, which calls an action routine per line, registers as a
   callback routine.
5. **Two zones from the start**, made when the image's first allocation
   needs them: the default zone (Table 5-3, with what the probe shows)
   and the string zone (the probe's attributes), with dynamic strings in
   the second, as FIND_VM_ZONE says VMS's are. The RTL's own blocks
   (FIND_FILE's, the contexts, timers) go to the default zone;
   DECC$MALLOC too, unless the probe shows the C RTL with a zone of its
   own.
6. **Per image**, as today: image rundown forgets every zone and the
   pool (VMS deletes the P0 pages they were in).

### For the probe (subtask 10)

A MACRO program (`testdata/lib59/`) in the form of Phases 53 to 58's
probes: each case under a handler that tells a signaled condition from
a returned status; addresses shown relative to a base the case takes
(the zone's first block, the pool's first page) or as differences, so
the run-to-run part is masked; SHOW_VM_ZONE's and SHOW_VM's lines through
an action routine that prints them, and also to SYS$OUTPUT. What it
should settle:

- **Pages**: consecutive GET_VM_PAGE(1) addresses; a freed group reused;
  partial and combined frees; FREE_VM_PAGE of an unallocated page, a
  freed one, an unaligned address, count 0 and -1, a page of a zone's
  area (govax refuses it, LIB$_BADBLOADR); GET_VM_PAGE 0, -1,
  and more than P0 can hold; STAT_VM 5 to 7 around them; `$EXPREG` of a
  page between two GET_VM_PAGE calls.
- **CREATE_VM_ZONE**: each attribute at and past each bound; a missing
  algorithm-argument; page-limit without initial-size; flag bits 6 to
  8; both fills of a pair; a get-page routine given as argument 12 and
  as 13 (which is called, with what); the zone ID's relation to the
  pool's pages; initial-size against page-limit.
- **GET_VM and FREE_VM** in each algorithm: the distance between
  consecutive blocks of sizes 1, 8, 9, 16, 24, 100 (rounding, alignment,
  tags' overhead); a fresh area's first block's offset in its page;
  fills (GET_FILL0/1, FREE_FILL0/1, which bytes keep control
  information); size 0 and -1; the wrong zone; an interior address; a
  double free; a smaller and a larger size than allocated (then what the
  next allocations return); a tagged zone's FREE_VM without size, and
  after the program changes a tag; quick fit's lookaside range with
  smallest-block-size; frequent sizes with more sizes than lists; fixed
  size with a smaller and larger request; PAGLIMEXC and NO_EXTEND at
  the limit; EXTEND_AREA (two extensions, one area or two, by
  SHOW_VM_ZONE); TAIL_LARGE's area order.
- **DELETE and RESET**: zone 0; a deleted zone's ID afterwards (GET_VM,
  FREE_VM, DELETE again); RESET then GET_VM (same addresses?); free fill
  on both.
- **User zones**: Example 5-1's monitor, with the routines printing
  their arguments' addresses and values; each operation with its
  routine omitted (INVOPEZON); a failing user delete routine and then
  the zone; GET_VM size 0 on a user zone; SHOW_VM_ZONE and FIND_VM_ZONE
  of a user zone.
- **Statistics**: SHOW_VM with no arguments, each code 0 to 7, and 8;
  with an action routine (its descriptor's class and text, the user
  argument, a failing status); STAT_VM 0 to 8; whether calls on other
  zones, user zones, failed calls, and a dynamic string's allocation
  (LIB$SGET1_DD) count; bytes asked versus rounded.
- **FIND, SHOW, VERIFY**: FIND_VM_ZONE from 0 at the program's start
  (which zones exist), and after creating and deleting zones;
  SHOW_VM_ZONE of each, at each detail level, for each algorithm, with
  blocks allocated and freed; a bad zone ID; VERIFY of a sound zone and
  of one whose freed block the program overwrote (with a free fill);
  wrong argument counts.
- **The VM_MALLOC family**: VM_MALLOC(n) and VM_FREE(p) by value, R0;
  VM_CALLOC(n, m), VM_REALLOC(p, n); FIND_VM_ZONE after; DECC$MALLOC's
  blocks against them (the same zone?).

### The shape of the work

The new package (subtask 2) holds the mechanisms; `corevms` holds each
process's zone database and pool (beside `memory.go`, which shrinks to
DECC$MALLOC/FREE's shims), and its exported methods are what `librtl`
reaches; `librtl` has the routines: `vm.go` (GET_VM, FREE_VM),
`vmzone.go` (CREATE, CREATE_USER, DELETE, RESET), `vmpage.go`,
`vmstat.go` (SHOW_VM, STAT_VM), `vmshow.go` (FIND, SHOW_VM_ZONE,
VERIFY), and `vmmalloc.go`. `dynstr.go` allocates in the string zone.

## Subtasks (provisional)

1. **Survey**: chapters 5 and 6 and each routine, and a review of the
   heap govax has. **Done** 2026-10-14.
2. **The heap's foundation** (new, a predicate of the rest): the page
   pool and the area block allocator, address-ordered and coalescing,
   with an invariant checker and randomized and fragmentation tests;
   the current heap's users (AllocateVM, FreeVM, BlockSize,
   DECC$MALLOC/FREE, GET_VM/FREE_VM) moved onto it; P0's bound for the
   heap and `$EXPREG`. Fixes the overlapping blocks and partial merges.
   **Done** 2026-10-14.
3. **Pages**: GET_VM_PAGE, FREE_VM_PAGE. **Done** 2026-10-14.
4. **Zones and the algorithms**: CREATE_VM_ZONE, DELETE_VM_ZONE,
   RESET_VM_ZONE; GET_VM and FREE_VM redone, with the manual's statuses;
   quick fit, frequent sizes, fixed size; tags and fills. **Done**
   2026-10-14.
5. **The default and string zones**, dynamic strings and DECC$MALLOC
   moved onto them. **Done** 2026-10-14.
6. **User zones**: CREATE_USER_VM_ZONE (tail calls), and get-page and
   free-page routines (callbacks).
7. **Statistics**: SHOW_VM, STAT_VM.
8. **The C library's routines** (if settled).
9. **LIBRTL2's zone routines**: FIND_VM_ZONE, SHOW_VM_ZONE,
   VERIFY_VM_ZONE (offsets known).
10. **The program** (`testdata/lib59/`), its console test, and the VMS
    probe; govax changed to its answers.
11. **Close-out**: DEVIATIONS.md records the heap's change from the
    eVAX port.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-14** — Subtask 1, the survey. Chapters 5 and 6 and each
  routine's entry in the 5.0 manual, with the VSI manual's differences
  (12 arguments for CREATE_VM_ZONE against 5.0's 13, the NO_EXTEND and
  TAIL_LARGE flags, LIB$_BADTAGVAL); all routines are in the transfer
  vectors (LIBRTL2's three included), and every condition and constant
  is in `vmsdef`. Neither manual documents the VM_MALLOC family. At the
  author's request the survey reviews govax's current heap: it hands
  out overlapping live blocks (a block's fit is tested against the
  request and then marked with the larger rounded size), merges a freed
  block with at most one neighbor, keeps its free list unordered, has
  no bound on P0 growth, accepts interior addresses, and frees the
  whole heap for DELETE_VM_ZONE of zone 0; a throwaway randomized test
  showed the overlap in its first round. So a new subtask 2 builds the
  page pool and block allocator as a tested leaf package and moves the
  heap's users onto it before the zone routines; the later subtasks are
  renumbered. Six decisions are proposed for the author: zone IDs as
  control blocks' addresses with Go-side state, Go-side block
  bookkeeping, double frees detected, user zones as tail calls (get-page
  routines as callbacks), the default and string zones, and the heap
  per image.
- **2026-10-14** — The decisions accepted as proposed.
- **2026-10-14** — Subtask 2, the heap's foundation. A new leaf package,
  `internal/vmheap`: `PagePool` (the processwide page pool: owned and
  free pages as address-ordered spans, a freed range merged with both
  neighbors, first fit by address, growth through a `Region`, asking
  only for the pages the free top run lacks; partial and combined frees,
  and anything not allocated, unaligned, or freed already refused before
  anything changes) and `Zone` (first fit: areas from the pool, each
  with its 64-byte control block kept at its start and an unallocated
  end used after its free spans; free spans sorted, merged with both
  neighbors, and given back to the unallocated end when they reach it;
  allocated blocks in a map by exact address; the quantum the larger of
  block size and alignment; ExtendArea appends adjoining pages). `Check`
  verifies both (order, no touching or overlapping spans, alignment,
  every byte of an area in exactly one block or free span, the areas'
  pages allocated in the pool); the tests run it after every operation,
  in unit tests, randomized workloads under four attribute sets (all
  freed at the end, each area empty again), the page pool's random
  partial frees, and the survey's workload as a fragmentation bound
  (one area, within twice the peak in use). Injected faults (no merge
  with the previous span; a fit test 8 bytes too lenient, the old
  heap's fault) fail 8 and 2 tests. `corevms/memory.go` is now the
  image's heap on it: the default zone's attributes (Table 5-3, with
  EXTEND_AREA), pages from P0 by `$EXPREG`'s rule (`expandP0`, page
  aligned, never page 0, stopping at P0's page table, `p0Limit`), made
  at the first allocation and forgotten at rundown. `AllocateVM` lost
  its unused zone argument; DECC$MALLOC returns 0 for no room;
  free(NULL) does nothing; `$EXPREG` of P0 past the table is
  SS$_VASFULL; LIB$DELETE_VM_ZONE returns LIB$_BADBLOADR (no zone can
  exist yet, and zone 0 can't be deleted) instead of freeing the heap.
  Blocks moved (8-byte rounding, after the area's control block): one
  govax test pinned the first block's address (0x4000, now 0x4040);
  no oracle moved. DEVIATIONS.md records the change from the eVAX port.
- **2026-10-14** — Subtask 3, the pages. LIB$GET_VM_PAGE and
  LIB$FREE_VM_PAGE (`librtl/vmpage.go`, shim codes 171 and 172) on the
  image's page pool, which corevms now makes apart from the heap zone
  (`pagePool`; `GetVMPages`, `FreeVMPages`), so pages and the heap's
  areas come from one pool, as the manual's levels have it. A count of
  0 or less (a signed longword) is LIB$_BADBLOSIZ; FREE_VM_PAGE of an
  unaligned address or pages not all allocated is LIB$_BADBLOADR, with
  nothing freed; no room in P0 is LIB$_INSVIRMEM with base-address left
  alone; `vmStatus` maps vmheap's errors to the conditions for the
  routines to come. The pool now knows which pages a zone holds as its
  areas (`PagePool.Lend`, the `held` list, checked by `Check`), and
  `Free` refuses them, so a program's FREE_VM_PAGE of a zone's page
  can't hand those pages out again under live blocks (**unconfirmed**:
  VMS presumably frees them; a probe case). Tests: vmheap's held pages;
  librtl's unit tests (adjacent pages upward from P0's end, reuse lowest
  first, partial and cross-group frees, each bad argument changing
  nothing, no room, pages and the default zone's area side by side);
  and `TestLibrtlVMPages`, a linked program that gets three pages,
  writes them in user mode, frees them in parts, and sees a double free
  refused. STAT_VM's page counts (codes 5 to 7) wait for subtask 7.
- **2026-10-14** — Subtask 4, zones and the algorithms.
  `internal/vmheap`'s `Zone` gained the manual's attributes
  (`Attributes`: the algorithm and its argument, boundary tags, initial
  size, page limit, NO_EXTEND, TAIL_LARGE; `ErrBadAttributes` for
  broken rules) and the other three algorithms as lookaside lists
  (`lookaside`): quick fit's lists for the sizes from smallest-block-size
  up by block-size, frequent sizes' lists that each take the size of the
  first block freed to them while empty, and fixed size's single list.
  A listed block stays part of its area (never split or merged) and is
  taken last in, first out; freed again it's `ErrFreedAgain`, the zone
  unchanged. Boundary tags: a longword before and after each block, the
  block's size (tags included) a multiple of the block size and its
  first usable byte aligned (`place`); `Tags` gives their addresses and
  value. New areas go to the head of the area list (searched first),
  or with TAIL_LARGE an area larger than extend-size to its end (the
  VSI manual's wording implies the head; **unconfirmed**). At the page
  limit the last area is what the limit leaves, if the request fits
  (**unconfirmed**); past it `ErrPageLimit`. `Reset` empties the areas;
  `Delete` gives them back (`PagePool.Reclaim`). `Check` covers the
  lists. Tests: each algorithm, tags, initial size and page limit,
  NO_EXTEND, area order, reset, delete, the bad attributes, and a
  randomized workload under six attribute sets with double frees, the
  checker after every step, then reset and delete. Injected faults (no
  double-free guard on lookaside lists; tags ignored in placement) fail
  6 and 5 tests.
  `corevms/vmzone.go` is the image's zones: `CreateZone` (a 128-byte
  control block, `ZoneControlSize`, govax's size, from a private zone no
  ID names, holding its own address, the ID; the state in Go,
  `Environment.zones`), `ZoneAllocate` and `ZoneFree` (zone 0 the
  default zone; tags written and checked, `ErrBadTag`; fills;
  a size given must round to the block's), `ResetZone`, `DeleteZone`
  (the control block freed and cleared), `ZoneSettingsOf`; image rundown
  forgets them. `librtl`: LIB$CREATE_VM_ZONE (`vmzone.go`, code 173)
  with the manual's defaults and ranges, LIB$_INVARG for anything out of
  range and, by govax's choice (**unconfirmed**), for a flag bit VMS
  doesn't define, both fills of a pair, a page limit or NO_EXTEND
  without an initial size, and an initial size past the limit;
  number-of-areas, get-page, and free-page are read as nothing until
  subtask 6. LIB$RESET_VM_ZONE (code 174). LIB$GET_VM, LIB$FREE_VM,
  and LIB$DELETE_VM_ZONE redone (`vm.go`) with the manual's statuses
  (`vmFailure`, replacing subtask 3's `vmStatus`): LIB$_BADBLOSIZ for a
  size of 0 or less, more than a fixed zone's, or a LIB$FREE_VM size that
  doesn't round to the block's (**unconfirmed**: VMS presumably trusts
  it); LIB$_BADBLOADR for an invalid zone ID, zone 0 to RESET and DELETE,
  and a block not the zone's (no longer 4042, DEVIATIONS.md);
  LIB$_PAGLIMEXC; LIB$_BADTAGVAL (**VSI**); LIB$FREE_VM's size may be
  omitted in any zone (govax knows every block's size; **unconfirmed**);
  a lookaside double free is SS$_NORMAL. NO_EXTEND past the initial
  area is LIB$_INSVIRMEM (**unconfirmed**). Tests: librtl's unit tests
  of each routine and status, fills on GET, FREE, RESET, and DELETE,
  tags, fixed size and double frees, the page limit, and a deleted
  zone's pages reused by LIB$GET_VM_PAGE; `TestLibrtlVMZones`, a linked
  program that makes a fixed-size zone with a fill and runs it through
  every routine. Dynamic strings and DECC$MALLOC still use the default
  zone directly (subtask 5).
- **2026-10-14** — Subtask 5, the default and string zones. The RTL's
  two zones are now zones like the program's (`corevms/vmzone.go`,
  `makeZone`, shared with `CreateZone`): each has a control block from
  the control zone, whose address is its ID and which holds the ID, and
  an entry in `Environment.zones`, marked `system`. `systemZones` makes
  both at the image's first allocation, the default zone first, so their
  IDs (and LIB$FIND_VM_ZONE's order, subtask 9) are default, then
  string. The default zone (`defaultZone`, now `ZoneSettings`) is named
  DEFAULT_ZONE, as the manual's LIB$FIND_VM_ZONE example shows; the
  string zone has the default zone's attributes and the name
  STRING_ZONE (both **unconfirmed**, probe cases). Zone 0 and the default
  zone's own ID name the same zone; LIB$RESET_VM_ZONE and
  LIB$DELETE_VM_ZONE of either system zone are LIB$_BADBLOADR (the
  string zone's by govax's choice, **unconfirmed**). `Environment`'s
  `heapZone` field is gone: `defaultHeap` and `stringHeap` replace it.
  DECC$MALLOC/FREE and the RTL's own blocks (`AllocateVM`, `FreeVM`,
  `BlockSize`) use the default zone; dynamic strings
  (`librtl/dynstr.go`) the string zone, through `AllocateString`,
  `FreeString`, and `StringSize`. So LIB$SFREE1_DD of a block from
  LIB$GET_VM is STR$_ERRFREDYN, and LIB$FREE_VM of a dynamic string's
  text in zone 0 LIB$_BADBLOADR (VMS's answers presumably; probe cases).
  `DefaultZoneID` and `StringZoneID` are for the tests now and
  LIB$FIND_VM_ZONE later. Blocks moved again: the control blocks' page
  comes first, so the default zone's first block is a page later (three
  govax tests pinned it: 0x4040 is now 0x4240); no oracle moved. Tests:
  `TestLibSystemZones` (the IDs and their order, the names and
  attributes, zone 0 as the default zone's ID, RESET and DELETE refused,
  a dynamic string's text in the string zone and not freeable in the
  default zone, a LIB$GET_VM block not freeable as a dynamic string);
  corevms's heap tests check every zone and the pool.
