# Phase 107 — Fortran 90: pointers

**Status:** done 2026-11-19. Planned 2026-11-17 with Phase 99 (`PHASE-99 -
Fortran 90.md`, Part C). Subtask 1, the survey (Part A), done 2026-11-19 and
accepted by the author the same day ("The author's answers"); subtasks 2 to 9
(5 and 6 each in two parts) done 2026-11-19. "Where things stand", at the end,
is what the phase left.

## Goal

Fortran 90's pointers (the standard's 5.1.2.7, 6.3, 7.5.2), by Decision 12:

- **POINTER** and **TARGET**, as attributes and statements; scalar, character,
  array, and derived-type pointers;
- **pointer assignment** (`P => T`, `P => A(1:N:2)`, `P => Q`), with its
  constraints (a target must be a TARGET or a pointer; rank and type agree);
- **NULLIFY**, **ASSOCIATED(P [, T])**, and ALLOCATE and DEALLOCATE of pointers
  (Phase 104's routines, the memory owned by whoever deallocates it);
- **pointer components** of derived types, including a type pointing to itself
  (linked lists, trees), in modules;
- **pointer dummies** and **pointer function results**, needing an explicit
  interface;
- **using a pointer** where its target would go: in expressions, as an actual
  argument to an ordinary dummy (the target is passed), in I/O;
- the **optimizer's aliasing rules**: a POINTER aliases a TARGET or another
  POINTER of its type and kind, nothing else; a call may change any TARGET it can
  reach.

## What earlier phases leave in place

- NCA descriptors, ALLOCATE and DEALLOCATE (Phase 104), derived types (Phase
  105), modules (Phase 106).
- `opt/effects.go`'s `Storage` and the places analysis (`opt/places.go`), which
  learn the new rule rather than a new pass.

## Design notes

- **Representation** (Decision 12): a scalar pointer is an address, 0 when
  disassociated; a character pointer a class S descriptor; an array pointer an
  NCA descriptor. Pointer assignment copies the address or builds the
  descriptor; it never copies data.
- **Undefined status.** Pointers start disassociated (static storage is zero;
  frame pointers cleared at entry): Fortran 95's behavior, a choice, in MANUAL.md.
- **A dangling pointer** (to a deallocated target, or a returned routine's local)
  is undefined behavior the standard doesn't require catching; `/CHECK=POINTERS`
  (Phase 110) may catch a disassociated pointer's use.
- **Teaching:** FORTRAN-INTERNALS.md walks a linked list's code, and explains
  why the TARGET attribute exists: it is what lets the optimizer keep every other
  variable in a register across a pointer store.

## Open questions

1. Should DEALLOCATE of a pointer to memory not obtained by ALLOCATE be caught?
   Proposed: FTNRTL's heap blocks carry a mark it checks, an error if absent.
   Settled (2026-11-19): yes, for blocks made for pointers ("The author's
   answers", 1).

## Subtasks (provisional)

1. **Survey** (the standard's 5.1.2.7 and 5.1.2.8, 6.3.1.2, 6.3.3.2, 7.5.2,
   12.4.1.1; the Handbook's chapters 5 to 7) (done 2026-11-19, accepted;
   Part A, below, revises the rest of this list in A.4).
2. **Scalar and character pointers**, `=>`, NULLIFY, ASSOCIATED (done
   2026-11-19).
3. **Array pointers** and pointers to sections (done 2026-11-19).
4. **ALLOCATE and DEALLOCATE of pointers** (done 2026-11-19).
5. **Pointer components**, recursive types: 5a, scalar ones (done 2026-11-19);
   5b, array pointer components (done 2026-11-19).
6. **Pointer dummies and results** (done 2026-11-19).
7. **The optimizer's aliasing rules**, with programs that would break if they
   were wrong, both settings (done 2026-11-19).
8. **Programs end to end** (a linked list and a binary tree in modules, a sparse
   matrix), every mode, gfortran's witness (done 2026-11-19).
9. **Close-out**: FORTRAN-INTERNALS.md's section 32 (pointers and aliasing),
   MANUAL.md. (done 2026-11-19)

# Part A — the survey (subtask 1)

Read for it: the standard's 5.1 (the type declaration's constraints on POINTER
and TARGET), 5.1.2.4.3 (deferred-shape arrays: an array pointer's bounds),
5.1.2.7 and 5.1.2.8, 5.2.7 and 5.2.8 (the attributes and statements), 4.4.1
(pointer components, a type pointing to itself, sequence types), 4.4.4
(constructors with pointer components), 5.2.9, 5.4, 5.5.1, and 5.5.2 (DATA,
NAMELIST, EQUIVALENCE, COMMON), 6.1.2 and 6.2.2 (structure components and
sections: which have the POINTER attribute), 6.3 (ALLOCATE, NULLIFY,
DEALLOCATE, and 6.3.1.2's and 6.3.3.2's rules for pointers), 7.1.4.1 (a pointer
as a primary), 7.5.1.5 (intrinsic assignment through a pointer, and of a type
with pointer components), 7.5.2 (pointer assignment), 9.4.2 and 9.4.4.4 (I/O
items), 12.1.2.2.1 (a host's pointer), 12.2.1 and 12.3.1.1 (characteristics, and
when an interface must be explicit), 12.4.1.1 (pointer and TARGET dummies),
12.5.2.2 and 12.5.2.5 (pointer results, ENTRY), 12.5.2.9 (what a dummy may
alias), 13.13.13 (ASSOCIATED), 14.6.2 (association status), 14.6.3.1 (a
pointer's storage unit), and C.12.9; the *Fortran 90 Handbook*'s 5.4, 6.5, 7.5.3,
and 12.5.3 (its Figure 12-5 of actual and dummy pointers and targets) beside
them; and what the lexer, parser, sema, the IR, the optimizer, codegen, FTNRTL,
and the module text do with POINTER, TARGET, `=>`, NULLIFY, and ASSOCIATED
today. There is no VMS probe (no VAX Fortran 90); the standard is the target and
gfortran 16.2, `-std=f95`, the witness.

## A.1 What the standard asks

- **The attributes** (5.1, 5.1.2.7, 5.1.2.8, 5.2.7, 5.2.8). POINTER as an
  attribute or a statement (`POINTER :: P, A(:, :)`); an array pointer has a
  deferred shape, given by the type statement, the POINTER statement, or
  DIMENSION. POINTER excludes TARGET, INTENT, EXTERNAL, INTRINSIC, PARAMETER,
  and ALLOCATABLE; TARGET excludes POINTER, EXTERNAL, INTRINSIC, and PARAMETER,
  and may go with ALLOCATABLE, INTENT, OPTIONAL, SAVE, and an initial value. A
  pointer may be OPTIONAL, SAVE'd, PUBLIC or PRIVATE. It may not have an initial
  value (`=> NULL()` is Fortran 95's), be in DATA, NAMELIST, or EQUIVALENCE, but
  **it may be in COMMON** (5.5.2: only an explicit shape is refused there; the
  Handbook notes every declaration of the block must then agree, since what
  occupies the storage is the descriptor). A pointer occupies "a single
  unspecified storage unit", different for each type, type parameters, and rank
  (14.6.3.1), and may be storage associated only with a pointer of all three the
  same.
- **Association status** (14.6.2). Undefined at first, associated (by ALLOCATE
  or `=>`), or disassociated (NULLIFY, DEALLOCATE, `=>` a disassociated pointer).
  At a return, a pointer becomes undefined unless SAVE'd, in COMMON, in a module,
  host associated, or the function's result (6.3.3.2); its target becomes
  undefined with the frame it was in. A pointer whose status is undefined or
  disassociated may only be nullified, allocated, pointer assigned, or asked
  about by an inquiry that doesn't look at its target.
- **A pointer is its target** (7.1.4.1, 7.5.1.5, 9.4.4.4, 12.4.1.1). Wherever a
  value or a variable is wanted (an expression, the left side of `=`, an I/O
  item, an internal file, an actual argument whose dummy isn't a pointer), a
  pointer means its target, of the target's shape. Only a pointer dummy, `=>`,
  NULLIFY, ALLOCATE, DEALLOCATE, ASSOCIATED, and the type and kind inquiries see
  the pointer itself. An array element or section of a pointer array never has
  the POINTER attribute (6.2.2); a component is a pointer only if its last part
  is (6.1.2), and no part to the right of a ranked part may be one
  (`NODES(:)%NEXT` is refused).
- **Pointer assignment** (7.5.2, R736). `pointer-object => target`, the object a
  pointer name or a component; the target a variable with TARGET, a subobject of
  one (an element, a section without a vector subscript, a substring, a
  component), a pointer (associated, disassociated, or undefined: the status is
  copied), or a reference to a function with a pointer result; of the same type,
  type parameters, and rank. A whole array's bounds come with it (LBOUND and
  UBOUND of the target); a section's start at 1 (5.1.2.4.3). It never copies
  data. gfortran agrees on the bounds (`P => A` keeps `A(0:9)`'s, `P => A(3:)`
  is `1:8`; tried 2026-11-19).
- **NULLIFY, ALLOCATE, DEALLOCATE** (6.3). NULLIFY of pointers and pointer
  components. ALLOCATE of a pointer makes a new object, implicitly a TARGET, and
  **is not an error when the pointer is associated already**: the old target is
  just no longer reached through it (6.3.1.2). An allocate object may be a
  component (`ALLOCATE (P%NEXT)`) and a scalar (no bounds). DEALLOCATE of a
  pointer that is disassociated, or whose target ALLOCATE didn't make, is an
  error condition (STAT= or the program ends); one associated with part of a
  target, or with an allocatable array, "must not" be deallocated (6.3.3.2), a
  rule the processor needn't check. gfortran checks none of this: DEALLOCATE of
  a pointer to a TARGET array dies in the C library's `free` (SIGABRT, tried
  2026-11-19).
- **ASSOCIATED (POINTER [, TARGET])** (13.13.13). Without TARGET, whether the
  pointer is associated; with a target, whether it is associated with that
  object; with a pointer, whether both are associated with the same object (false
  if either is disassociated). `A_PART => A(:N)` makes `ASSOCIATED (A_PART, A)`
  true when N is A's upper bound: the elements matter, not the bounds. Fortran 90
  doesn't say what a zero-sized target answers; gfortran says false
  (`ASSOCIATED (P, A(5:4))`), and true for `ASSOCIATED (P)` (tried 2026-11-19).
- **Pointer components** (4.4.1, 4.4.4, 7.5.1.5, 9.4.2). A component with
  POINTER may be of any accessible type, **the type being defined included**
  (lists, trees); an array component with POINTER has a deferred shape. A
  sequence type with a pointer component is neither a numeric nor a character
  sequence type, so it can't be EQUIVALENCEd. A constructor's value for a pointer
  component must be an allowable target, so no constant of such a type exists
  (no PARAMETER, no DATA, no initial value). Intrinsic assignment of such a type
  **pointer-assigns** its pointer components. An object whose type ultimately
  contains a pointer component may not be an I/O item, input or output, nor in a
  NAMELIST group.
- **Pointer dummies and results** (12.2.1, 12.3.1.1, 12.4.1.1, 12.5.2.2). A
  dummy or result that is a pointer (or a dummy that is a TARGET) needs an
  explicit interface. A pointer dummy's actual must be a pointer of the same
  type, type parameters, and rank; the dummy starts with the actual's status and
  target, may change both, and at the return the actual has the dummy's. A
  pointer dummy may not have INTENT. A function with a pointer result must
  associate or nullify it before returning; its value, used in an expression, is
  its target, and on the right of `=>` its association. An ENTRY's result shares
  the function's only if its characteristics are the same (12.5.2.5).
- **TARGET dummies** (12.4.1.1). Pointers to a TARGET actual don't become
  pointers to the dummy, and pointers to a TARGET dummy become undefined at the
  return: nothing a processor must arrange; a copy-in-copy-out actual is allowed.
- **What may alias what** (5.1.2.8, 12.5.2.9). "An object without the TARGET or
  POINTER attribute must not have an accessible pointer associated with it", and
  while an entity is a dummy's actual it may be changed only through that dummy:
  so a pointer reaches only TARGETs, other pointers' targets, and what ALLOCATE
  made, and a dummy without TARGET is reached through no pointer at all. This is
  the rule the optimizer is promised (Decision 12).

## A.2 What govax has

- **Recognition** (Phases 99, 100). The lexer classifies `p => t` as
  `lex.PointerAssignment` (a `=>` at level 0 with no comma before it, so USE's
  renames stay USE's), and has `lex.Pointer`, `lex.Target`, and `lex.Nullify`.
  The parser reads TARGET as an attribute statement (`parseAttr`); NULLIFY, the
  POINTER statement, and pointer assignment are `PendingStmt`s, their text not
  parsed. Tried 2026-11-19, a module with `TYPE(NODE), POINTER :: NEXT` in NODE
  and a program using it gives NOTIMPL for the POINTER component, for each
  POINTER attribute, for each `=>`, and for NULLIFY, and a SYNTAX error for
  `ALLOCATE (HEAD)`: the parser's allocate object is a name with bounds
  (`parse/f90.go`'s `parseAllocate`), and a DEALLOCATE object a name alone.
- **Attributes** (`sema/attrtable.go`). The table already has 5.1's exclusions for
  both (POINTER with ALLOCATABLE, TARGET, INTENT, EXTERNAL, INTRINSIC, and
  PARAMETER); POINTER's fate is `attrNotYet` (NOTIMPL), TARGET's `attrRecorded`
  (`Symbol.Target`, meaning nothing yet). A deferred shape without ALLOCATABLE is
  DEFERREDNOTALLOC. The module text writes TARGET (`sema/modwrite.go`) and
  nothing for POINTER. ASSOCIATED isn't in the intrinsic table, so today it would
  be an external function's name.
- **Descriptors** (Phase 104). An allocatable array is a class NCA descriptor in
  its own cell (`ir/lowerallocate.go`), POINTER 0 when not allocated, its
  elements reached as **contiguous** (A0 indexed by the first subscript, S1 the
  element's size). An assumed-shape dummy's elements are reached through **every**
  stride (`ir/lowershaped.go`), each entry point copying the caller's descriptor
  into the dummy's `$Bn` cell with its own lower bounds (`computeShaped`); a
  caller builds a descriptor of a section or a whole array (`sectionNCA`,
  `buildNCA`), and a ranked component's stride is the record's size
  (`rankedStride`). A CHARACTER dummy's text is reached through its class S
  descriptor (`Mem.Ptr`), and a section or assumed-shape array passed to an old
  dummy is tested for contiguity when the program runs (`testedSequence`) or
  copied (`sema/sequence.go`).
- **FTNRTL** (`ftn_alloc.mar`, Phase 104). `FTN$ALLOCATE(desc, flags, bounds...)`
  refuses a descriptor whose POINTER isn't 0 (ALREADYALLOC, 301), takes the
  memory from LIB$GET_VM with no header, and fills the descriptor;
  `FTN$DEALLOCATE(desc, flags)` gives back ARSIZE bytes (8 at least) from
  POINTER and clears it (NOTALLOC, 302, if 0). Flag bit 0 is STAT=. `FTN$GET_TEMP`
  takes a value's storage, and **a MACRO caller of an array-valued function is
  told it may free that block with LIB$FREE_VM** (`TestFortranValueFromMACRO`),
  so a value's block can never carry a header. GSMATCH is 1,24.
- **Records** (Phase 105). A derived type is a `Structure` of `Field`s laid out
  packed or naturally; intrinsic assignment of records is a MOVC3 (`moveRecord`),
  which for a pointer component *is* pointer assignment (the address or
  descriptor copied); a constructor of constants is a `Const` with `Raw` bytes,
  any other a `StructValue` built a component at a time. `derived.go` refuses a
  POINTER component (NOTIMPL) and a component of the type being defined.
- **Overlap** (`sema/overlap.go`, Phase 102). An array assignment's sides share
  storage only if they are the same name, meet in a common block, or are tied by
  EQUIVALENCE (`mayShare`); two dummies are apart; a component's storage is its
  parent's (`baseSymbol`).
- **The optimizer** (`opt/places.go`, `opt/effects.go`, Phase 84 on). A `Place`
  is a region and a byte range; an address the analysis can't follow is
  `UnknownRegion`, which conflicts with everything (`throughPointer`). Dummies
  are their own regions, apart from everything (12.5.2.9's rule, already used);
  an allocatable or automatic array's elements are an `AllocatedRegion` of their
  descriptor, which any call may change. A local is changed by a call only if
  "taken" (its address given away), in a RECURSIVE unit if static, or VOLATILE
  (`ChangedByCall`).

## A.3 Findings, and what they change

1. **Declarations.** POINTER becomes `attrCompiled` with a `Symbol.Pointer`, from
   the attribute, the POINTER statement (parsed like TARGET's, with
   deferred-shape lists), or both; a deferred shape is then allowed for a
   pointer (DEFERREDNOTALLOC's text names both ALLOCATABLE and POINTER). New
   refusals: a pointer or an object of a type with an ultimate pointer component
   in DATA, NAMELIST, or EQUIVALENCE, a pointer with an initial value
   (`POINTERDATA`, `POINTERNML`, `POINTEREQUIV`, `POINTERINIT`, each naming the
   rule). A pointer in COMMON is allowed and laid out as finding 2's storage; the
   module text writes POINTER (and its layout line) as it writes TARGET.
2. **Storage** (Decision 12, confirmed). A pointer's storage is its own cell: a
   scalar pointer of a non-CHARACTER type is a longword, the target's address or
   0; a CHARACTER scalar pointer a class S descriptor, 8 bytes (its length the
   target's: for a constant length, fixed; its POINTER the address or 0); an
   array pointer a class NCA descriptor, `20 + 12*rank` bytes, as an allocatable
   array's, POINTER 0 when disassociated. Static storage starts at 0, so
   disassociated. A pointer in the frame (a RECURSIVE or AUTOMATIC unit's, or a
   function's pointer result) is cleared at every entry point, as
   `clearAllocatables` fills a frame NCA cell; so are the pointer components of
   a record in the frame, and of an automatic array of records (question 2).
3. **A reference to a pointer is its target.** sema keeps a reference to a
   pointer as the name it is, and decides where it stands, as it decides
   substring or section: in a value or variable context it is the target (the
   symbol's `Pointer` tells the IR to go through the cell); in `=>`, NULLIFY,
   ALLOCATE, DEALLOCATE, ASSOCIATED, and as a pointer dummy's actual, the pointer.
   In the IR a scalar target is a `Mem` whose `Ptr` is the cell's longword (a
   CHARACTER one's text through its descriptor, as a CHARACTER dummy's is); an
   array pointer's elements are read **as an assumed-shape array's**, through
   every stride (a pointer to a section is not contiguous), the cell its own
   descriptor, with no entry copy. SIZE, LBOUND, UBOUND, SHAPE, and LEN read the
   descriptor as for an allocatable array.
   As an actual argument to a dummy that isn't a pointer, the target is passed at
   no cost: a scalar's address is the pointer's longword itself, a CHARACTER
   one's descriptor is the pointer's own descriptor, an array to an assumed-shape
   dummy is the pointer's descriptor (the callee resets the lower bounds); to an
   explicit-shape or assumed-size dummy it goes through `testedSequence`, as an
   assumed-shape array does. In an I/O list a scalar target is an item, an array
   one an element at a time (as an allocatable array is today; Phase 109 may
   pass the descriptor), and a CHARACTER pointer may be an internal file, its
   descriptor the unit's.
4. **Pointer assignment** is a sema node, `PointerAssign`, checked by 7.5.2:
   NOTPOINTER for an object without POINTER, NOTTARGET for a variable that is
   neither a TARGET, a subobject of one, nor a pointer, `POINTERTYPE` for a
   difference of type, kind, rank, or a constant CHARACTER length, VECTORTARGET
   for a vector subscript, and a function reference whose result isn't a pointer.
   Lowering stores the address (scalar), the length and address (CHARACTER), or a
   whole descriptor (array): a pointer target's descriptor copied; a whole array
   described with **its** lower bounds; a section described by `sectionNCA`, its
   lower bounds 1, strides those of the section (through a ranked component, the
   record's size; through an array pointer of records, the pointer's own S1,
   which `rankedStride` must read rather than assume). A descriptor is built in a
   temporary and moved into the pointer when the target reads the pointer itself
   (`P => P(N:1:-1)`), otherwise in place.
5. **NULLIFY and ASSOCIATED.** NULLIFY stores 0 in the address or POINTER.
   ASSOCIATED joins the intrinsic table as an inquiry with dummies POINTER and
   TARGET (optional), analyzed by sema itself, as ALLOCATED is, and computed in
   line: with one argument, a test of the longword; with a scalar target, an
   address compared; a CHARACTER one, address and length; an array, POINTER, each
   extent, and each stride compared, and the size greater than 0, so a
   zero-sized target answers false as gfortran's does (question 4). A target that
   is a section is described into a temporary first.
6. **ALLOCATE and DEALLOCATE of pointers.** The parser's allocate object becomes
   a name or a component, with or without bounds (bounds only for an array),
   and a DEALLOCATE object a name or a component; sema accepts a pointer
   (NOTALLOCATABLE's text names both kinds). FTNRTL: an array pointer by
   `FTN$ALLOCATE` with flag bit 1, *pointer*: no ALREADYALLOC (6.3.1.2: the old
   target is left); a scalar one by a new `FTN$ALLOC_SCALAR(cell, size, flags)`,
   `cell` the longword the address goes in (a CHARACTER pointer's descriptor's
   POINTER, its length the size). DEALLOCATE by `FTN$DEALLOCATE` with bit 1, or
   `FTN$DEALLOC_SCALAR(cell, size, flags)`, which clear the address. A
   disassociated pointer is NOTALLOC (302). To answer the plan's open question 1
   (question 1 below): a block ALLOCATE makes **for a pointer** carries an
   8-byte header before its first byte, a mark and its size; DEALLOCATE of a
   pointer checks that the address is a marked block's start and, for an array,
   that the descriptor is the whole block, contiguous, else signals a new
   condition, govax's own, FOR number 304 ("the pointer's target was not made by
   ALLOCATE, or is not the whole of it"), and clears the mark before giving the
   block back. Allocatable arrays' and values' blocks keep no header (a value's
   is freed by MACRO callers with LIB$FREE_VM, A.2), so `P => A; DEALLOCATE (P)`
   of an allocatable TARGET A is caught too. Flag bit 2, *clear*, set for a type
   with pointer components, has the block cleared, so its pointers start
   disassociated (question 2). New entries at the vector's end, GSMATCH 1,25.
7. **Pointer components and recursive types.** `derived.go` takes POINTER: of
   any type accessible, the one being defined included (only through POINTER),
   an array one with a deferred shape; its storage is finding 2's, aligned to a
   longword under natural alignment. `Structure` learns whether it ultimately
   contains a pointer (`HasPointers`), which refuses it as an I/O item or a
   NAMELIST object, in EQUIVALENCE, in DATA, as a PARAMETER or initial value,
   and makes a sequence type with one neither numeric nor character
   (`equivalenceClasses`). A constructor of such a type is never a `Const` but a
   `StructValue`, whose pointer components are pointer-assigned from their
   values (the value must be an allowable target, finding 4's checks). Intrinsic
   assignment stays a MOVC3, which is 7.5.1.5's pointer assignment of the
   components. A component chain through pointers (`HEAD%NEXT%NEXT%VALUE`) is a
   load of each address in turn. The module text writes a pointer component and
   a type pointing to itself, and its reader reads one (the type's name is known
   before its components are).
8. **Pointer dummies.** The argument is **the address of the actual's own pointer
   storage**: its longword, or its descriptor, so the callee's `=>`, NULLIFY,
   ALLOCATE, and DEALLOCATE are the caller's (12.4.1.1) with no copy back, and
   every reference in the callee goes through the caller's cell (no entry copy,
   unlike an assumed-shape dummy's). For a CHARACTER pointer dummy that is the
   address of a class S descriptor, exactly what any CHARACTER dummy is given, so
   `CHARACTER(*), POINTER` reads its length there; for an array, the address of a
   class NCA descriptor, as an assumed-shape dummy's actual (question 5). A call
   needs the explicit interface (EXPLICITREQ, naming "pointer"); the actual must
   be a pointer of the same type, kind, rank, and constant length
   (`POINTERARG`); INTENT is refused by the table; an OPTIONAL pointer absent is
   a 0 longword, PRESENT as today. `checkInterface` compares POINTER and TARGET as
   characteristics (12.2.1.1); generic resolution doesn't use them (Fortran 90
   distinguishes specifics by type, kind, and rank only). A TARGET dummy is
   passed as before.
9. **Pointer results.** A FUNCTION whose name or RESULT variable is a pointer:
   a scalar one returns the address in R0; a CHARACTER one its descriptor in R0
   and R1 (as Phase 105's records of 8 bytes); an array one fills a class NCA
   descriptor the caller passes as a hidden first argument (Phase 104's form for
   values, but nothing to free: the caller keeps an association, not storage).
   The result is cleared at entry (finding 2). `P => F(X)` copies the
   association into P after the call (from a temporary, so F may use P); in an
   expression the reference is its target. ENTRY in such a function is allowed
   when every result has the same characteristics, else ENTRYVALUE's rule.
10. **Overlap in array assignments.** `mayShare` learns pointers: a pointer may
    share storage with another pointer, or with a TARGET (a dummy too), of a type
    that may hold its type (finding 11's rule); `baseSymbol` stops at a pointer
    component, whose target isn't its parent's storage. So `A = P` with P perhaps
    pointing into A gets a temporary (the "unknown" relation), and the same
    pointer on both sides keeps the ordered analysis. An array pointer is never
    known contiguous when compiling (`Contiguous`), so it is tested when the
    program runs. A run-time overlap test instead of the temporary is Phase 110's
    (question 6).
11. **The aliasing rule in the optimizer.** A new region, `PointerRegion`, keyed
    by a *type class*: an intrinsic type and kind, all CHARACTER lengths one
    class (a pointer may point at a substring), a derived type by its
    `Structure`. The lowerer marks each `Mem` it reaches through a pointer with
    the class (a field on `Mem`), so `places.go` needn't recognize the address's
    form. Two classes may meet if one is the other or a derived type ultimately
    containing a component of the other (an INTEGER pointer may point at a
    NODE's VALUE; a NODE pointer's stores change it). A `PointerRegion` place
    conflicts with another of a class that may meet; with the place of any TARGET
    variable (local, frame, host, module, or a TARGET dummy) of such a class, and
    with anything sharing that variable's storage by EQUIVALENCE (Storage keeps a
    list of TARGETs' places, as it keeps the taken ones); with COMMON (question
    3); with UnknownRegion. Never with a local or allocatable array without
    TARGET, a dummy without TARGET (12.5.2.9), $PDATA, or a register. A call may
    change any `PointerRegion`. A TARGET local is **taken** once the unit names it
    or a part of it as the target of `=>` or of a constructor's pointer
    component, so a call may change it then and only then. A pointer's own cell
    is ordinary storage, taken when passed to a pointer dummy. This is where the
    TARGET attribute pays (the design note's teaching point): the optimizer's
    registers survive a store through a pointer for every variable without it.
12. **What waits.** `/CHECK=POINTERS` (a disassociated pointer's use), the
    debugger's EXAMINE of a pointer and through one, `/STANDARD` for the
    phase's forms: Phase 110. A whole array pointer in an I/O list by its
    descriptor: Phase 109. `NULL()` and `=> NULL()` initialization (default
    initialization of components too): Phase 112. Not Fortran 90, not planned:
    procedure pointers, bounds remapping (`P(1:N) => A`), allocatable components.

## A.4 The subtasks, revised

The plan's order stands; each subtask's content is now the findings'.

1. **Survey** (this Part A) (done 2026-11-19; accepted by the author the
   same day).
2. **Scalar and character pointers:** POINTER as attribute and statement,
   storage in static, frame, COMMON, and module cells, cleared at entry, the
   refusals (findings 1, 2); references as their targets in expressions,
   assignments, actual arguments to non-pointer dummies, I/O, internal files
   (finding 3); `=>` for scalars, NULLIFY, ASSOCIATED (findings 4, 5); the module
   text's POINTER (done 2026-11-19).
3. **Array pointers:** descriptors read through every stride, `=>` to whole
   arrays, sections, components, and pointers with their bounds, the inquiries,
   ASSOCIATED of arrays, actual arguments to assumed-shape and old dummies
   (findings 3 to 5), overlap (finding 10) (done 2026-11-19).
4. **ALLOCATE and DEALLOCATE of pointers:** the parser's objects, FTNRTL's flag
   bits, `FTN$ALLOC_SCALAR` and `FTN$DEALLOC_SCALAR`, the header and condition
   304, the clear bit, GSMATCH 1,25, MANUAL.md's FTNRTL table (finding 6) (done
   2026-11-19).
5. **Pointer components and recursive types:** `derived.go`, `HasPointers` and
   its refusals, constructors, component chains, `rankedStride` through a
   pointer, the module text's self-referencing types (finding 7). Split in two
   when it began (2026-11-19): **5a**, scalar pointer components of every type,
   the one being defined among them, and everything above for them (done
   2026-11-19); **5b**, array pointer components (a class NCA descriptor in a
   record, which every descriptor routine of the IR reaches only through a cell
   today) and `rankedStride` through an array pointer of records (done
   2026-11-19).
6. **Pointer dummies and results** (findings 8, 9). Split in two when it began
   (2026-11-19): **6a**, pointer dummies (finding 8) (done 2026-11-19); **6b**,
   pointer results (finding 9), with `P => F(X)` and ENTRY in such a function
   (done 2026-11-19).
7. **The optimizer's aliasing rules:** `PointerRegion`, TARGETs' places, taken
   TARGETs, with programs that would print something else if the rule were wrong
   or too bold, both settings (finding 11) (done 2026-11-19).
8. **Programs end to end** (a linked list and a binary tree in modules, a sparse
   matrix, the standard's examples), every mode, gfortran's witness (done
   2026-11-19).
9. **Close-out:** FORTRAN-INTERNALS.md's section 32 (pointers and aliasing, a
   linked list's code, why TARGET exists), MANUAL.md's 14.13 and FTNRTL table,
   HELP FORTRAN (done 2026-11-19).

## A.5 Questions for the author

1. **DEALLOCATE of what ALLOCATE didn't make** (the plan's open question 1;
   finding 6). (a) **A header (a mark and the size) on each block ALLOCATE makes
   for a pointer, checked by DEALLOCATE of a pointer, condition 304 if absent or
   not the whole block; allocatable arrays' and values' blocks unchanged**; (b)
   no check (gfortran's way: the heap is damaged).
2. **Pointers in new storage** (finding 2, 6). (a) **Cleared: a block FTNRTL
   allocates for a type with pointer components, and the pointer components of a
   frame or automatic record at entry, so every pointer starts disassociated, as
   Decision 12 has static ones do**; (b) left as LIB$GET_VM and the stack give
   them (the standard says undefined).
3. **COMMON under the aliasing rule** (finding 11). (a) **A store through a
   pointer may change anything in any common block (another unit may declare a
   TARGET there); a module's variables by their own attributes, which every user
   knows**; (b) only blocks in which this unit declares a TARGET.
4. **ASSOCIATED with a zero-sized target** (finding 5). (a) **False, as
   gfortran's and later standards'**; (b) by the addresses alone.
5. **A pointer dummy's argument** (finding 8). (a) **The address of the actual's
   own pointer storage, so the callee changes the caller's association
   directly**; (b) a copy of the descriptor in, and back after the call.
6. **An array assignment that may overlap through a pointer** (finding 10). (a)
   **A temporary, decided when compiling; a run-time test is Phase 110's**; (b)
   the run-time test now.

## The author's answers (2026-11-19)

The author read the survey and accepted every recommendation:

1. A block ALLOCATE makes for a pointer carries a header (a mark and its size);
   DEALLOCATE of a pointer checks it and signals condition 304, govax's own, when
   it is absent or the pointer isn't associated with the whole block. Allocatable
   arrays' and values' blocks are unchanged (finding 6; the plan's open question 1).
2. Pointers in new storage are cleared: a block FTNRTL allocates for a type with
   pointer components (flag bit 2), and the pointer components of a frame or
   automatic record at entry, so every pointer starts disassociated (findings 2, 6).
3. A store through a pointer may change anything in any common block; a module's
   variables are judged by their own attributes (finding 11).
4. ASSOCIATED with a zero-sized target is false (finding 5).
5. A pointer dummy's argument is the address of the actual's own pointer storage,
   so the callee changes the caller's association directly (finding 8).
6. An array assignment that may overlap through a pointer goes through a
   temporary, decided when compiling; a run-time test is Phase 110's (finding 10).

The plan's open question 1 is settled by answer 1.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-19:** Subtask 1, the survey (Part A), for review: what the standard
  asks of pointers, targets, pointer assignment, NULLIFY, ASSOCIATED, ALLOCATE and
  DEALLOCATE of pointers, components, dummies, results, and aliasing; what govax
  has (recognition, the attribute table, Phase 104's descriptors and FTNRTL,
  Phase 105's records, overlap, the optimizer's places); twelve findings, the
  subtasks restated, and six questions. gfortran 16.2 tried for bounds after `=>`,
  ASSOCIATED of a zero-sized target, and DEALLOCATE of a pointer to a TARGET (which
  it doesn't catch). Nothing in the code changed.
- **2026-11-19:** The author accepted the survey's six recommendations ("The
  author's answers"). Subtask 2 is next.
- **2026-11-19:** Subtask 2, scalar and character pointers, done.
  - *Parsing.* The POINTER statement is `parseAttr`'s, its items declarators as
    TARGET's are; pointer assignment (`ast.PointerAssignStmt`, the pointer a
    reference, the target any expression, so that `P => 5` and `P => T + 1` are
    sema's NOTTARGET rather than syntax errors) and NULLIFY
    (`ast.NullifyStmt`) have parsers of their own. With these no statement is a
    `PendingStmt` any more, and `parsePending` is gone; the type stays for the
    semantic NOTIMPLs that use one. ALLOCATE's object may now have no bounds (a
    scalar pointer's), an array's missing bounds ALLOCBOUNDS.
  - *Declarations* (finding 1). POINTER is compiled (`attrtable.go`), recorded as
    `Symbol.Pointer` by the attribute or the statement; an array pointer's
    colons pass `checkShapes`, bounds that aren't colons are POINTERSHAPE, and
    DEFERREDNOTALLOC's text names POINTER. `checkPointers` (`sema/pointer.go`,
    after `checkArrays`) reports EQUIVALENCE and the NOTIMPLs of later subtasks:
    an array pointer (3), a pointer dummy or function result (6), a CHARACTER
    pointer whose length isn't constant (with 6's `CHARACTER(*)`); ALLOCATE and
    DEALLOCATE of a pointer (4) and a pointer component (5) are NOTIMPL where
    they are. DATA, an initial value, and NAMELIST are refused where those are
    read. **One message, POINTERATTR, with the rule as its last argument**,
    rather than the survey's four (POINTERDATA, POINTERNML, POINTEREQUIV,
    POINTERINIT), as ALLOCATTR does for allocatable arrays; DATA's is "be given
    an initial value", as ALLOCATTR's is.
  - *Storage* (finding 2). `Symbol.Size` is `PointerSize`: 4, or 8 for CHARACTER;
    a pointer is on a longword in `$LOCAL`, the frame, COMMON, and a module's
    section (`boundary`). The module text writes `, POINTER`, and USE's copy
    keeps it (`TestModulePointers`). A frame pointer is cleared at every entry
    point (`ir/lowerpointer.go`'s `clearPointers`, from `clearAllocatables`;
    `entryClears` counts it), a CHARACTER one's header written too.
  - *References* (finding 3). sema keeps a reference to a pointer a `Var` of its
    symbol; `symbolMem` sends a pointer to `pointerTargetMem`, a `Mem` whose
    `Ptr` is the cell's longword (a CHARACTER pointer's descriptor's POINTER),
    so expressions, assignments, I/O items, internal files, and actual
    arguments (`PUSHAL @P`, the target's address at no cost) all work unchanged,
    and so do a record pointer's components (`PP%X`) and an internal procedure's
    use of its host's pointer.
  - *`=>`, NULLIFY, ASSOCIATED* (findings 4, 5). `sema.PointerAssign`,
    `Nullify`, and `Associated` (ASSOCIATED analyzed by sema itself, as
    ALLOCATED is: `associatedRef`, keywords POINTER= and TARGET=). A target must
    be a variable whose base (`targetBase`: a substring's, an element's, a
    component's) is a TARGET or a pointer (NOTTARGET), not a function's
    reference (POINTERFUNC), of the pointer's type, kind, and rank, and length
    when both are constants (POINTERTYPE; `targetRank` reads a whole array's
    rank from its symbol). Lowering: `=>` stores the target's address (`MOVAL
    N,P`; a pointer's own longword for `Q => P`), a CHARACTER pointer's header
    first (its declared length, `DSC$K_DTYPE_T`, `DSC$K_CLASS_S`); NULLIFY
    stores 0; ASSOCIATED tests or compares addresses, with a pointer target
    also that P isn't 0, and with a CHARACTER target of run-time length also
    that the length isn't 0 (answer 4).
  - *Overlap* (finding 10, the scalar half). `mayShare` learns pointers now,
    not in subtask 3, since a scalar pointer read in an array assignment into
    its own target (`V = V + E`, `E => V(3)`) must be read once: a pointer may
    share with another pointer, a TARGET (a dummy too), or anything in COMMON
    (answer 3), of a type it may meet (`pointerMayMeet`: the same type and kind,
    any two CHARACTERs, or a record with anything). So `Hoisted` reads E before
    the loops.
  - *The optimizer* needed nothing yet: a place through a pointer is
    `UnknownRegion`, conflicting with everything, and `P => T` takes T's
    address, so T is "taken" and a call may change it. Subtask 7 makes that
    bolder (`PointerRegion`). A pointer's own cell may go to a register.
  - *Listings and the debugger.* The storage map's type says `INTEGER, POINTER`
    (`codegen.typeText`). The `/DEBUG` table leaves pointers out, rather than
    let the debugger show an address as a value (finding 12: Phase 110's).
  - *Tests.* `parse`'s `TestPointerForms`; `sema`'s `TestPointers` (every message
    and NOTIMPL); `ir` and `codegen` golden `f90ptr.for`;
    `TestModulePointers`; `testdata/fortran90/pointers.f90`, worked by hand, in
    every mode and both loaders, and the same on gfortran (`compare.py`): targets
    of every kind, records, an internal file, READ, an actual argument, a module's
    pointer changing a TARGET during a call, a COMMON pointer, a loop storing
    through a pointer to a variable the optimizer would keep in a register, and
    the overlap. `TestF90StatementsNotImplemented` now expects no NOTIMPL.
  - *Docs.* MANUAL.md's 14.13 ("Pointers", and "Not there yet"'s example), HELP
    FORTRAN.
- **2026-11-19:** Subtask 3, array pointers, done.
  - *Declarations and storage* (findings 1, 2). An array pointer is no longer
    NOTIMPL. `Symbol.ArrayPointer` names one, and `Symbol.Strided` (an
    assumed-shape dummy or an array pointer) says an array's elements are found
    through every stride and are never known contiguous when compiling: the
    places that asked `AssumedShape` for that (sequence association and
    `Contiguous`, I/O lists, internal files and run-time formats, record
    elements, `stridedElement`, `symbolMem`, SIZE) now ask `Strided`. Its storage
    is a class NCA descriptor, `DescriptorSize(rank)`, made by
    `allocatableCell` as an allocatable array's is, so it stands among the
    adjustable arrays' cells and bounds, extents, SIZE, LBOUND, UBOUND, SHAPE,
    and the scalarizer read it with nothing new. A frame one is cleared at entry
    by `clearAllocatables`' `fillDescriptor` (POINTER 0). A module's gets its
    disassociated form from the module's object, as an allocatable array's does
    ("disassociated" in the comment); **a COMMON one gets none** (no unit's object
    gives a common block values), so `=>` writes the descriptor's unchanging
    longwords there first (`commonHeader`), else an assumed-shape callee under
    `/CHECK=BOUNDS` would refuse its class and rank. An array pointer in an
    inquiry in a specification expression is BOUNDREF, as an allocatable array
    is.
  - *Pointer assignment* (finding 4). `pointerObject` gives an array pointer as
    an `ArrayRef` (`PointerSymbol` and `IsPointer` take both forms), and a target
    is read by `targetPlace` (a whole array's `ArrayRef` with its shape, a
    section, a substring, a component; a whole array component as
    `wholeRanked`), where subtask 2's `place` refused sections. A section with a
    vector subscript is VECTORTARGET (new); a rank that differs is POINTERTYPE,
    whose text now names a rank above 1 (`an array of rank 2 of INTEGER`).
    Lowering (`ir/lowerpointer.go`'s `arrayPointerAssign`): a pointer target's
    descriptor copied (`copyDescriptor`, every longword after the first); else
    `describeTarget` fills the pointer's cell in place through `fillNCA`
    (`buildNCA`'s work, split out so it can fill a cell it didn't make) with a
    whole array's own bounds (`wholeDims`, split out of `arrayNCA`) or a
    section's from 1 (`describeSection`). **Bounds of a component:** `T%COORD`
    keeps COORD's declared bounds (`ownBounds`); `PATH%X`, its rank from the
    array of records, is a section, from 1. Fortran 90's LBOUND text ("a whole
    array or array structure component") leaves the second unsettled; Fortran
    95's 6.2.2 calls it a section, and gfortran agrees, so govax does.
    `P => P(9:1:-1)`, a target whose base is the pointer itself
    (`sema.TargetBase`), is described into a cell of the statement's and copied.
  - *NULLIFY and ASSOCIATED* (finding 5). NULLIFY clears POINTER
    (`pointerAddress` is the descriptor's POINTER for an array pointer).
    `arrayAssociated` compares P's descriptor with the target's (a pointer's own,
    or a description made for the question): POINTER, P's POINTER not 0, each
    dimension's stride and U - L, and the target's ARSIZE greater than 0 (answer
    4).
  - *A parser bug found:* `A(::2)` and `A(2::2)` were SYNTAX errors, the lexer
    having read the two colons as Fortran 90's `::`; `triplet` now takes `::` as
    an omitted upper bound and the stride's colon (`atColon`; `TestArrayForms`).
  - *An optimizer bug found:* `places.go`'s `allocatedBase` took an array
    pointer's descriptor for an allocatable array's, making its elements an
    `AllocatedRegion`, apart from every named variable, so `A(2) = 7; P(1) = 1;
    K = A(2)` with `P => A(2:2)` printed 7 under `/OPTIMIZE`. An array pointer's
    elements are now `UnknownRegion`, as a scalar pointer's target is, until
    subtask 7's `PointerRegion` (`ptrarrays.f90` holds it).
  - *Overlap* (finding 10). `mayShare` already knew pointers (subtask 2); with
    `Contiguous` refusing a pointer's sections, `A = P` and `B(2:5) = P(1:4)`
    with P into the same array go through a temporary.
  - *Tests.* `sema`'s `TestPointers` (array forms, VECTORTARGET, POINTERTYPE by
    rank, NOTTARGET of a section, NOSHAPE of an assumed-size target); `ir` and
    `codegen` golden `f90aptr.for`; `testdata/fortran90/ptrarrays.f90`, worked by
    hand, in every mode and both loaders, and the same on gfortran: whole arrays,
    sections, a pointer's section, another pointer, the self-reversal, a zero-sized
    target, two dimensions with WHERE and reductions, assumed-shape and
    explicit-shape actuals, overlap, components, CHARACTER elements, a module's and
    a COMMON array pointer, and the store the optimizer once carried past. gfortran
    16.2 itself gets `P => P(9:0:-1)` wrong when P's lower bound is 0 (it reads the
    subscripts as from 1), so the corpus reverses from a lower bound of 1; and it
    answers ASSOCIATED of a pointer never nullified as true, where Fortran 90 calls
    the status undefined, so the corpus nullifies first.
  - *Docs.* MANUAL.md's 14.13 (array pointers; "Not there yet"'s example now
    ALLOCATE of a pointer), HELP FORTRAN.
- **2026-11-19:** Subtask 4, ALLOCATE and DEALLOCATE of pointers, done.
  - *sema* (finding 6). `allocateStatement` takes a pointer as an object as it takes
    an allocatable array; NOTALLOCATABLE's text now has the kinds as its argument
    ("an allocatable array or a pointer" for the statements, "an allocatable
    array" for ALLOCATED, whose argument a pointer may not be). A scalar pointer's
    ALLOCATE has no bounds, and bounds for one are the new ALLOCSCALAR; an array
    pointer's are ALLOCBOUNDS' as before, ALLOCSELF and STAT= unchanged. A pointer
    is never freed at a return (`freedAtReturn` looks only at allocatable arrays).
    Components as objects (`ALLOCATE (P%NEXT)`) wait for subtask 5's components.
  - *Lowering* (`ir/lowerallocate.go`'s `allocateArray` and `allocateScalar`). An
    array pointer is FTN$ALLOCATE or FTN$DEALLOCATE with flag bit 1 (`allocPointer`),
    a COMMON one's descriptor header written first (`commonHeader`: FTN$ALLOCATE reads
    the element size and rank). A scalar one is the new `FTN$ALLOC_SCALAR(cell, flags,
    size)` or `FTN$DEALLOC_SCALAR(cell, flags, size)`, `cell` `pointerAddress`'s
    longword (a CHARACTER pointer's descriptor's POINTER, its header written first),
    `size` the target type's size. **The arguments' order is (cell, flags, size)**, not
    the survey's (cell, size, flags), so that every routine of FTN_ALLOC has its flags
    at 8(AP), where the shared failure path reads them.
  - *FTNRTL* (`ftn_alloc.mar`, GSMATCH 1,25; vector entries 720 and 728). Memory is
    taken and given back by two local routines, `GET_BLOCK` and `FREE_BLOCK`, which read
    the caller's flags through its saved AP: with bit 1 a block has the 8-byte header
    (the header's address XOR `PTR_MARK`, then the size asked for), with bit 2 it is
    cleared, 65,535 bytes at a time by MOVC5 (no caller sets it until subtask 5's
    types with pointer components). A pointer's ALLOCATE is never ALREADYALLOC.
    DEALLOCATE of a pointer checks, in `CHECK_BLOCK`, that the header can be read
    (PROBER: the target may be at the start of a program's storage), that its mark is
    its own address's, and that its size is the target's: for an array, ARSIZE, after
    its strides are checked to be those FTN$ALLOCATE would give its bounds (contiguous
    and in order). Else it is the new BADDEALLOC, FOR number 304, govax's own
    (`vmsdef/formessages.go`); the mark is cleared before LIB$FREE_VM. A scalar pointer
    to the one element of an array pointer's block of its size may give it back; that
    is the whole block (unconfirmed, ours). NOTALLOC's text is now "allocatable array or
    pointer is not allocated".
  - *A slip found by the first run:* `XORL3 #PTR_MARK,R3,(R3)+` marked the block with
    R3's incremented value: the architecture calls a register read in register mode in
    the same instruction as its autoincrement UNPREDICTABLE, and govax reads it after.
    The code now writes the header without the autoincrement.
  - *A label fixed:* the MACRO file's comment on an array pointer's cell said
    "allocatable" (`codegen`'s storage comments, subtask 3's; `f90aptr.mar`).
  - *Tests.* `sema`'s `TestPointers` (every form, ALLOCSCALAR, ALLOCBOUNDS, ALLOCSELF,
    ALLOCATED of a pointer) and `TestAllocate`'s texts; `ir` and `codegen` golden
    `f90palloc.for`; `ftnrtl/build`'s vector offsets and minor id;
    `testdata/fortran90/ptralloc.f90`, worked by hand, every mode and both loaders, the
    same on gfortran; `ptrallocvms.f90`, VMS's own (the FOR numbers, the signals, and
    LIB$STAT_VM's bytes: none kept, a lost INTEGER target's 16).
  - *Docs.* MANUAL.md's 14.13 (ALLOCATE and DEALLOCATE of pointers, growing an array,
    "Not there yet"'s example now a POINTER component) and its FTNRTL table, HELP
    FORTRAN.
- **2026-11-19:** Subtask 5 split in two (A.4): array pointer components need the
  IR's descriptor routines (`fillNCA`, `copyDescriptor`, `stridedElement`, the
  bounds and the scalarizer's extents), which reach a descriptor only as a cell,
  to reach one at any place in a record; that is 5b, with `rankedStride` through
  an array pointer of records. 5a is the rest.
- **2026-11-19:** Subtask 5a, scalar pointer components and recursive types, done.
  - *Definitions* (finding 7). A component statement may say POINTER
    (`derived.go`'s `componentStatement`), of an intrinsic type or of any derived
    type in scope, the one being defined among them (`componentType`; TYPESELF
    stays for one without POINTER). `Field.Pointer` marks it; its storage is
    `PointerSize`'s (`ElementSize`), on a longword (`boundary`), and the storage
    map says `TYPE(NODE), POINTER` (`TypeText`). An array pointer component, all
    colons, is NOTIMPL until 5b; other bounds are POINTERSHAPE (`pointerDims`).
    Every walk down a structure's fields that would follow a pointer component's
    type, which may be the structure itself, stops there: `image` (no initial
    values: zero, disassociated), `form` (the target's type by name),
    `sameType` and `modread`'s `sameStructure` (a pointer to the type itself is
    the same on both sides; without this a type read from two module texts was
    MODSTALE, as `TestModulePointerComponents` shows), `componentsClass` (a
    pointer is of neither class), `modwrite` (`, POINTER`).
  - *What can't hold a pointer.* `Structure.HasPointers` (at any depth) refuses
    an object as an I/O list's item, in a NAMELIST group, in EQUIVALENCE (before
    EQUIVTYPE's class rule), and in DATA or with an initial value: one message,
    **POINTERPART**, with the rule as its last argument, as POINTERATTR has. A
    constructor of such a type is never constant (`componentConst`'s NOTCONST
    reason names the component), so a PARAMETER of one is refused there. COMMON
    is allowed.
  - *References.* sema keeps `HEAD%NEXT%VALUE` a FieldRef as before; a field
    after a part with rank may not be a pointer (**POINTERRANKED**, 6.1.2), and
    a FieldRef whose last field is a pointer is the pointer itself where
    `=>`, NULLIFY, ASSOCIATED, ALLOCATE, and DEALLOCATE want one
    (`FieldRef.IsPointer`, `IsPointer`; NOTPOINTER and NOTALLOCATABLE otherwise),
    its target anywhere else. Lowering's `fieldMem` is now `fieldPlace`: at each
    pointer component the place so far is the pointer's longword (a CHARACTER
    one's descriptor's POINTER), loaded, and the reference goes on from offset 0
    there; with `target` false the last pointer's own storage is the place
    (`pointerOf`, `setHeader`, which `pointerAssign`, `nullify`, `associated`,
    and `allocateScalar` now take for any pointer reference). A pointer
    assignment's target address is computed before the store, so `P%NEXT =>
    P%NEXT%NEXT` reads the pointer it changes first.
  - *ALLOCATE and DEALLOCATE of a component.* The parser reads an object that has
    a `%` (or a record's period) before its end as a reference
    (`componentObject`), an `ast.AllocObject` with its `Component`;
    `sema.Allocation.Component` is the FieldRef (`allocatedComponent`). ALLOCSELF
    doesn't look at components (unconfirmed: 6.3.1 forbids an object depending
    on another of the same statement, `ALLOCATE (P, P%NEXT)`; govax does them in
    order).
  - *Constructors* give each pointer component a target by pointer assignment's
    rules (`targetFor`, `pointerTarget`'s work with the pointer named
    `NODE%NEXT`), and `buildRecord` stores its address (a CHARACTER one's header
    first). Intrinsic assignment stays a MOVC3, which copies the pointers.
  - *Cleared storage* (the author's answer 2). FTNRTL's clear bit (`allocClear`,
    4) for ALLOCATE of a pointer to such a type and of an allocatable array of
    one; a frame's records and arrays of records (`clearPointers` takes
    `ArrayCell`s now) and an automatic array's, after its StackAllocate, have
    each pointer's longword cleared (`Structure.PointerOffsets`, at any depth,
    through array components), in line up to 16 stores, else by a loop over the
    records (`clearRecords`). A function's value of run-time shape from
    `FTN$GET_TEMP` isn't cleared: every element is assigned before it is read.
  - *Overlap.* A FieldRef through a pointer component isn't in its record's
    storage: `baseSymbol` gives the component's own stand-in (`Field.reached`, a
    pointer of the component's type, one per component), so it may share with
    pointers, TARGETs, and COMMON of a type that may meet; two references through
    one component are related only if they agree up to it (`throughPrefix`,
    `relate`); `readsAny` and `readSymbols` count both the record and the target.
  - *The optimizer* needed nothing: a place through a loaded address is
    `UnknownRegion`. `TestStorageOnCorpus` counts the pointer golden programs'
    unknown places apart (`pointerRoutines`) until subtask 7.
  - *Tests.* `sema`'s `TestPointerComponents` (every form and message) and
    `TestPointers`' new cases; `parse`'s `TestPointerForms` (component objects);
    `TestModulePointerComponents` (the text, and a user reading NODE from two
    modules' texts); `ir` and `codegen` golden `f90pcomp.for`;
    `testdata/fortran90/ptrcomps.f90`, worked by hand, every mode and both loaders,
    the same on gfortran; `ptrclear.f90`, govax's own (its frames printed 24 and 43
    stale pointers before `clearPointers` took arrays of records, and its automatic
    array 18 with its clearing switched off).
  - *Docs.* MANUAL.md's 14.13 (pointer components, cleared storage, "Not there
    yet"'s example now an array POINTER component), HELP FORTRAN.
- **2026-11-19:** Subtask 5b, array pointer components, done.
  - *Definitions.* A POINTER component with a deferred shape is an array pointer
    component (`derived.go`'s `pointerDims`): `Field.Rank` is its rank and its
    `Dims` stay nil, so every walk that lays out, initializes, or compares a record
    sees one item, `DescriptorSize(Rank)` bytes (`ElementSize`), on a longword.
    `PointerOffsets` gives its descriptor's POINTER, 4 bytes in, so FTNRTL's clear
    bit and `clearRecords` leave it disassociated. The storage map says `INTEGER,
    POINTER (:)`, a module's text `INTEGER(KIND=4), POINTER, DIMENSION(:) :: V`, and
    `sameType`, `sameStructure`, and `form` compare the rank.
  - *A symbol for each component reference* (`sema/pointercomp.go`). Rather than
    teach every array form (elements, sections, the scalarizer, the inquiries,
    actual arguments, I/O lists, =>, ALLOCATE, ASSOCIATED) a second way to find a
    descriptor, `field` makes the reference to the component itself, `H%V`, a
    symbol, `componentPointer`: an array pointer like a named one, with
    `Symbol.Component` the reference, in no unit's table. What follows is read as
    for a named pointer: `H%V(I)` an Elem, `H%V(1:3)` a Section, `H%V` an ArrayRef,
    and for an array pointer of records a FieldRef from the symbol (`T%KIDS(2)%KEY`,
    `T%KIDS%KEY`; `recordBase` split, `recordBaseOf` taking the symbol). One
    reference's symbol is made once per scope and text (`componentKey`), so
    `H%V = H%V + 1` is one array to overlap.go, ordered with no temporary, while a
    statement function's dummy or an internal procedure's own variable of the same
    name gets a symbol of its own (`ptrcarr.f90`'s FIRST and OWN). A one-triplet list
    after one is a section, not a substring (`componentSubstring`). ALLOCATE's
    object `H%V(0:N)` has its bounds as the reference's list (`allocatedComponent`,
    `allocBounds`; a stride is ALLOCBOUNDS); an element or section where a pointer
    must be is NOTALLOCATABLE or NOTPOINTER. A constructor's array pointer
    component takes a target of its rank (`targetFor`), a component whose
    declaration failed no message more.
  - *Located cells* (`ir/locate.go`). The IR reaches a descriptor only as a cell,
    `Mem{Cell: c, Offset: ...}`, in some 90 places. `adjustableOf` of a component's
    symbol makes a located cell (`componentCell`, `locatedCell`): no storage, only
    where its storage is, `Cell.At` (the record's place, if it has neither a
    computed address nor an index: `H+4`) or `Cell.Base` (a temporary given the
    address then: through a pointer, a dummy, an element at a subscript known only
    when the program runs). Each use makes its own, so the record is the one the
    statement means where it is used, in an implied DO too. When a routine is lowered,
    `resolveLocated` makes every place naming such a cell name its place, so the
    optimizer and the code generator never see one (`Mem.Linked` refuses one before
    then). `=>`, ALLOCATE, and a constructor write the descriptor's unchanging
    longwords first, as for a COMMON pointer (`commonHeader`): no record's initial
    value or clearing gives them. `=>` describes a target into a temporary first when
    it is reached through a pointer, which may lead back to the record (`associate`,
    which `buildRecord` now shares).
  - *`rankedStride` through an array pointer of records* needed nothing: a
    FieldRef's ranked part on an array pointer's symbol reads its S1 through
    `baseStride` (subtask 3), which `T%KIDS%KEY` and `TP%KEY` now exercise.
  - *A bug found: INTENT(IN) and pointer components* (subtask 5a's). `X%P = 4`
    with X an INTENT(IN) dummy was INTENTIN, though it changes only X%P's target;
    and `X%P => T`, NULLIFY, ALLOCATE, and DEALLOCATE of it, which change X, said
    nothing. `definedSymbol` now passes over a reference through a pointer, and
    the pointer statements check the record that holds the pointer
    (`pointerHome`).
  - *Tests.* `sema`'s `TestPointerComponents` (every form and message) and
    `TestPointers` (no NOTIMPL); `TestModulePointerComponents` (array pointer
    components in a module's text, read from two modules); `ir` and `codegen`
    golden `f90pcarr.for`; `opt`'s `pointerRoutines`;
    `testdata/fortran90/ptrcarr.f90`, every mode and both loaders, and the same on
    gfortran (also run once under `/CHECK=BOUNDS`, whose descriptor checks read the
    headers); `ptrclear.f90`'s records each with an array pointer (its frames printed
    3 and 21 stale pointers with the descriptor's offset left out of `PointerOffsets`).
  - *Docs.* MANUAL.md's 14.13 (array pointer components; "Not there yet"'s example
    now a pointer dummy), HELP FORTRAN.
- **2026-11-19:** Subtask 6 split in two (A.4): **6a**, pointer dummies (finding 8),
  and **6b**, pointer results (finding 9), which needs a new way back from a call
  (a descriptor through a hidden argument) and `=>` from a function's reference.
- **2026-11-19:** Subtask 6a, pointer dummies, done.
  - *sema* (finding 8). A pointer dummy is no longer NOTIMPL, and a CHARACTER(LEN=*)
    one is allowed (its length is its actual's, which the actual's descriptor holds);
    any other CHARACTER pointer of a length known only when the program runs stays
    NOTIMPL. `argsTo` sends a pointer dummy's actual to `pointerArg`, which reads it
    as `=>` reads its pointer (`pointerObject`, now `pointerObjectOr` with the refusal
    as a callback) and holds it to the dummy's type, kind, rank, and constant length:
    the new **POINTERARG** names the argument, the routine, the dummy, and what the
    dummy points at. `Arg.Pointer` marks it, and `argumentMismatch` and
    `checkInterface`'s per-argument checks pass over it. A pointer's ASSOCIATED
    note and INTENT(IN) check (`pointerHome`) are `=>`'s, since the callee may change
    it. `explicitDummy` says a pointer or a **TARGET** dummy needs an explicit interface
    (12.3.1.1; EXPLICITREQ's kind "pointer" or "TARGET"), and `dummyDifference` compares
    POINTER and TARGET (INTERFACEDIFF, `attributeText`). INTENT with POINTER was
    already ATTRPAIR.
  - *The module text* writes a dummy's TARGET and POINTER (`modwrite`'s `dummy`): a
    module procedure's interface had lost both, so a user passed a pointer dummy the
    target, which the first run found (`TestModulePointers` now has a procedure with
    pointer and TARGET dummies, and the call's code).
  - *Lowering.* A pointer dummy's storage is the caller's pointer, at the address its
    argument holds (`pointerStorage`: `@4(AP)`, or through ENTRY's `$An` cell, which
    an internal procedure reaching its host's pointer dummy uses too); a scalar one's
    target is a load of that longword, a CHARACTER one's its descriptor's POINTER,
    and its length its descriptor's word, as a passed-length dummy's is
    (`charLength`). An array pointer dummy's descriptor is a third kind of located
    cell, **`Cell.Arg`**, the caller's descriptor at the argument's address
    (`dummyPointerCell`, from `adjustableOf`), which `resolveLocated` turns into
    `Mem{Ptr: ArgPointer}`; so every descriptor routine (elements, sections, bounds,
    the scalarizer, `=>`, ALLOCATE, ASSOCIATED) reaches it with nothing new, and
    there is no entry copy, unlike an assumed-shape dummy's. The caller passes the
    address of its pointer's own storage (`pointerActual`: a named pointer's cell or
    descriptor, a pointer component's place in its record, an array pointer
    component's located cell), and writes what a callee may read and nothing else
    has given: a CHARACTER pointer's header (its declared length, for a
    `CHARACTER(LEN=*)` dummy), and an array pointer's unchanging longwords if it is
    in COMMON or a record (`commonHeader`, for FTN$ALLOCATE in the callee). The
    callee writes no header of a passed-length dummy (`setPointerHeader`) nor of an
    array pointer dummy's descriptor (`commonHeader`), whose length is the actual's.
    `allocateScalar`'s size is `pointerLength` for a CHARACTER pointer, so
    `ALLOCATE (C)` of a `CHARACTER(LEN=*)` pointer dummy asks for its actual's length.
    An OPTIONAL pointer dummy left out is the longword 0, and PRESENT is as before.
  - *The optimizer.* A CHARACTER dummy's descriptor was `DescriptorRegion`, read-only,
    and the text it points at the dummy's own: for a CHARACTER pointer dummy the
    descriptor is the caller's pointer, which `=>` changes, and its target anybody's,
    so `places.go` makes the first `DummyRegion` and the second unknown
    (`isDescriptorPointer`); `TestStorageOnCorpus` caught the write to a "read-only"
    descriptor. The register allocator doesn't take a pointer dummy for a scalar
    whose value it may hold (its `Param.T` is the target's type, not an address's);
    it may still keep a pointer dummy's address in a register in a loop, a target
    reached through it being unknown either way. `pointerRoutines` counts F90PDUM's
    unknown places apart until subtask 7.
  - *Tests.* `sema`'s `TestPointerDummies` (every kind of actual, POINTERARG for a
    target, an element, another type, an expression, another length; EXPLICITREQ for a
    pointer and a TARGET dummy; INTERFACEDIFF; ATTRPAIR; the remaining NOTIMPLs) and
    `TestPointers`' case; `TestModulePointers`; `ir` and `codegen` golden
    `f90pdum.for`; `testdata/fortran90/ptrdum.f90`, worked by hand, every mode and both
    loaders, the same on gfortran: a linked list's head and links passed to procedures
    that push, drop, find, and free, an array grown with its lower bound kept, a
    pointer dummy passed on as a pointer and as its target and reached by an internal
    procedure, a `CHARACTER(LEN=*)` pointer dummy, an OPTIONAL one, a rank-two one,
    pointer components as actuals, and a COMMON array pointer allocated by an external
    procedure through an interface block.
  - *Docs.* MANUAL.md's 14.13 (pointer dummies; "Not there yet"'s example now a pointer
    result), HELP FORTRAN.
- **2026-11-19:** Subtask 6b, pointer results, done.
  - *sema* (finding 9; `sema/pointerresult.go`). A function's value with POINTER is no
    longer NOTIMPL. A reference through an explicit interface whose value is a pointer is a
    `FuncRef` with **`Pointer`** set, and for an array pointer `Made` too, so that every place
    reading a value the function describes (the scalarizer's leaves, LEN, SIZE, sequence and
    assumed-shape arguments) reads it. `Symbol.HiddenValue` and `FuncRef.HiddenValue` take a
    pointer's value as an association: a scalar's in R0, a CHARACTER one's descriptor in R0
    and R1, only an array pointer's through the hidden argument. `targetFor` accepts, where
    POINTERFUNC stood, a reference to a function whose value is a pointer
    (`pointerFunctionTarget`, through `apply` so a record's pointer is no AGGREGATE), held to
    the pointer's type, kind, rank, and length by `pointerMatches`; `functionReference` knows a
    recursive function's reference to itself (`UnitName`), which `HIT => FIND (HEAD%NEXT, V)`
    found. A reference without an explicit interface is the new **EXPLICITPOINTER**;
    INTERFACEDIFF compares the value's POINTER (`pointerResultDifference`); an ENTRY of such a
    function is a pointer of the same type, and the other way round (ENTRYVALUE,
    `entryPointerDifference`). The module text writes the value's POINTER (`modwrite`'s
    `procedure`), as 6a's did a dummy's.
  - *Overlap* (finding 10). A pointer function's value is its own `baseSymbol`, a pointer of
    its type (`FuncRef.through`), so an array assignment whose target is a TARGET, a pointer, or
    in COMMON goes through a temporary (`X(4:6) = EVENS(X)`), and `readsAny` takes a scalar
    reference to read anything such a pointer may point at.
  - *Lowering* (`ir/lowerpointer.go`'s pointer results). In the function: a scalar or
    CHARACTER value is its own cell, the return loading it whole (`valueType`, Long or Quad, in
    place of `resultType` wherever a name's value is returned, ENTRY's return blocks too;
    `resultValue` reads `pointerStorage`); an array pointer's is the caller's descriptor at the
    hidden argument's address, `l.params[result]`, so `dummyPointerCell` makes it the located
    cell 6a made for array pointer dummies and every descriptor routine reaches it. Each entry
    point clears it (`clearPointerResult`, from `clearAllocatables`; `entryClears` says ENTRY's
    entry points must): an array pointer's unchanging longwords and POINTER 0, a scalar's
    address 0 and a CHARACTER one's header. In the caller: each reference is a pointer of the
    statement's own (`pointerValue`): a `$T` longword or an 8-byte text temporary from R0 (and
    R1), named by a `$P` symbol of `sema.NewSymbol` with `Pointer` set, or for an array pointer a
    `$D` descriptor cell passed as the hidden argument, once per statement
    (`pointerArray`, `lowerer.pointerValues`, cleared by `release`), nothing given back at the
    statement's end. `expr`, `mem`, `chars`, `complexParts`, and `complexValue` lower a
    reference as that pointer, and so as its target; `store` and `assignResult` no longer take
    it for a value in R0; `valueResult` sends an array pointer's to `pointerArray`.
    `targetPointer` turns the target of `=>`, ASSOCIATED, and a constructor's pointer
    component into the pointer, so that `P => F(X)` is `P => Q`: an address stored, or a
    descriptor copied (`copyDescriptor`).
  - *Tests.* `sema`'s `TestPointerResults` (every use; POINTERTYPE by type and rank;
    POINTERFUNC for a function whose value isn't a pointer; EXPLICITPOINTER; INTERFACEDIFF;
    ENTRYVALUE both ways), and `TestPointerDummies` without its NOTIMPL case;
    `TestModulePointers` (a module's scalar and array pointer functions, their text, and the
    call's code); `ir` and `codegen` golden `f90pres.for`; `opt`'s `pointerRoutines` names its
    routines; `testdata/fortran90/ptrres.f90`, worked by hand, every mode and both loaders, the
    same on gfortran but two lines `compare.py`'s `KNOWN` explains (gfortran leaves an
    unassigned pointer value undefined, where govax's is disassociated, and makes no temporary
    for `X(4:6) = EVENS(X)`, giving `2.0` where 7.5.1.5 asks `4.0`).
  - *Docs.* MANUAL.md's 14.13 (pointer results; "Not there yet"'s example now a CHARACTER
    pointer of a length known when the program runs), HELP FORTRAN. Subtask 7, the optimizer's
    aliasing rules, is next.
- **2026-11-19:** Subtask 7, the optimizer's aliasing rules, done (finding 11).
  - *The class rule* (`sema/pointerclass.go`). `PointersMayMeet(t, u)`: a value of type t
    reached through a pointer and one of type u may share storage when they are one class
    (an intrinsic type by kind and kind number; every CHARACTER length one class; a derived
    type by `sameType`, so an interface body's SEQUENCE copy is the type it copies), or one
    is a record that ultimately holds a component of the other's class, pointer components
    passed over (a record holds only an address there). `FieldTargetType` is a pointer
    component's target type. sema's own `pointerMayMeet`, overlap's coarser rule (a record
    with anything), is left as it is: it decides only temporaries.
  - *The mark* (`ir.Mem.Pointee`). The lowerer marks each place it reaches through a pointer
    with the type the pointer points at, so the optimizer needn't recognize the address's
    form, which its own passes rewrite: a scalar or CHARACTER pointer's target
    (`pointerTargetMem`), an array pointer's elements and base (`stridedElement`,
    `allocatableBase`, by `arrayPointee`), a field chain through a pointer component (the
    component's target type from there on, `fieldPlace`), an element of an array pointer of
    records, a constructor's component built through an address, a record place moved to a
    temporary (`recordPlace`), and a descriptor in a record reached through a pointer, a
    located cell at Base (`Cell.Pointee`, which `locateMem` gives the resolved place). The
    register allocator's rewritten places keep it (`registers.go`'s base candidates). The
    IR's dump writes it, `@(P){=>INTEGER}`, so the golden `.ir` files of the pointer
    programs show every mark.
  - *The places* (`opt/places.go`, "Pointers"). A marked place is a `PointerRegion` with its
    `Class`, unless its address has become a place's own (`Addr`), which says more. It
    conflicts with another of a class that may meet; with a place overlapping a TARGET of
    such a class, `Place.Targets` (local, frame, host, a module's variables, an allocatable
    or automatic array by its descriptor, a dummy; anything sharing a TARGET's storage by
    EQUIVALENCE, its range overlapping the TARGET's); with all of COMMON but a module's block
    (the author's answer 3); with `UnknownRegion`; and with nothing else. `findTargets`
    lists the TARGETs once per Storage, a module's from its block's members (all of them,
    not only those the unit names); `targetsOver` gives each place the types it overlaps as
    one list per set of TARGETs (`Storage.targetSets`), so a `Place` stays comparable, a map
    key, with equal places equal. Any call may change a `PointerRegion` (`ChangedByCall`'s
    default), and `Volatile` asks VOLATILE TARGET dummies of a class that may meet. A TARGET
    is judged by its type whether or not the unit names it in `=>`: the bolder rule, only a
    taken TARGET, would save little (a TARGET is there to be pointed at) and tie the targets'
    list to `findTaken`'s order.
  - *What was checked to be safe.* `forward.go` keeps facts only of places with no address
    (`isKey`), and `deadcode.go` compares places only for cells, so two pointers' equal
    places are never taken for one; `unsafeToMove` keeps a load through an address in its
    loop unless surely reached, so a disassociated pointer's 0 never faults earlier than it
    would have.
  - *Tests.* `opt`'s `TestPointerConflicts` (19 cases: TARGETs and not, other types,
    EQUIVALENCE, records holding the type or not, pointers to one type and two, a record's
    pointer and its component's, COMMON, allocatable arrays, substrings), `TestPointerModules`,
    `TestPointerDummies` (a TARGET dummy and one without), and `TestPointerRegisters` (K, no
    TARGET, in a register across `P = P + K`, which the old UnknownRegion denied it; a TARGET
    not). `TestStorageOnCorpus` loses `pointerRoutines`: the corpus's unknown places are back
    to the 14 it had before pointers, and 60 are pointers'. `testdata/fortran90/ptralias.f90`,
    worked by hand, every mode and both loaders, the same on gfortran (its REALs printed as
    integers, gfortran's list-directed layout being another), and `ptrequiv.f90`, govax's own
    (EQUIVALENCE with a TARGET, which Fortran 90's 5.5.1 allows and Fortran 95 forbids, so
    gfortran refuses it). Each was run against rules made too bold on purpose: ignoring
    TARGETs and COMMON, `ptralias` printed six cases wrong under `/OPTIMIZE` (a local TARGET,
    a component, a host's TARGET, a TARGET dummy, a section, a kept TARGET) and `ptrequiv`
    both; without the containment half of the class rule, the component case; with pointers
    never meeting, nearly every case. The COMMON and module cases print right even so, no pass
    yet keeping a COMMON or module value across a store; they stand guard for one that will.
  - *Docs.* MANUAL.md's 14.9 (a pointer pointing only where the standard lets it, among what
    the optimizer takes at its word) and 14.13 ("What TARGET is for"), HELP FORTRAN.
    Subtask 8, programs end to end, is next.
- **2026-11-19:** Subtask 8, programs end to end, done.
  - *The programs*, each worked by hand and the same on gfortran (`compare.py`'s
    `PROGRAMS90`, no known difference), run in every mode and both loaders:
    `testdata/fortran90/ptrstd.f90`, the standard's examples of pointers made to run
    (4.4.1's REFERENCE and NODE, 5.1.2.7 and 5.1.2.8, 6.3's ALLOCATE, NULLIFY, and
    DEALLOCATE, 7.5.2's six pointer assignments, 13.13.13's ASSOCIATED, C.5.2's
    DYNAM_ITER reading its shape from the terminal and C.5.3's ITER, C.12.8's BEST),
    each in an internal subroutine named for its section so the names can be the
    standard's; `sparse.f90`, a sparse matrix whose rows are linked lists in an array
    pointer component, with set, find (a pointer function), copy, transpose, `+` by
    merging rows, `*` by a vector and a matrix, a diagonal matrix by a defined
    assignment, and a count of live entries ending at 0; and
    `modules/concord`, a concordance over four files: LISTS a queue of line numbers
    (an opaque type, its pointer components PRIVATE), TREES a binary search tree whose
    nodes hold one, built, searched, outlined, and pruned by recursive procedures with
    pointer dummies, compiled in four orders and linked two ways. `ptrstd` passed at
    once, and `concord` once its own errors were fixed (a PUBLIC type with components
    of a PRIVATE type, which govax's ACCESSCOMPONENT refused as gfortran did; a
    component of a function reference, which both refuse); `sparse` found four
    defects, each fixed here:
  - *A bound read from a dummy's component*, `REAL :: Y (A % N)` (7.1.6.2: a subobject
    of a dummy is a specification expression's primary), was BADBOUND, which left the
    module broken and its type unknown to the program (NOTYPEDEF). `boundOperands`
    takes a `FieldRef` (`boundComponent`): its base, a name, goes on the array's names
    marked `adjustableArray.components`, checked by `checkBoundName` as a bound's name
    is (BOUNDVAR for a local's); subscripts along the way are bound expressions; the
    component's type is checked once analyzed (`boundValue`, BADBOUND for CHARACTER).
    A CHARACTER length reads the same way. A component of an array element, `D(1)%N`,
    is still BADBOUND: nothing has asked for it.
  - *A defined operation's array operands*, `D * (/ 1.0, 1.0, 1.0 /)`, panicked in
    lowering: `operandArgs` didn't mark an array section or expression as `argValue`
    does, and nothing ran the operands through `shaped` or `sequenced` against the
    procedure's dummies, as `argsTo` does a reference's. `operandArgs` marks them and
    `operandsTo` analyzes them, for operations and defined assignment alike.
  - *A defined assignment's array value*, `G = (/ 3.0, 0.0, 1.0 /)` to `DIAGONAL (A,
    X(:))`, was NOTSCALAR before the interface was asked: a scalar record's value is now
    analyzed as an array may be when an ASSIGNMENT(=) interface is in scope
    (`definedValue`), passed as `(value)` is, and an array no interface takes is still
    NOTSCALAR, from the shapes' check.
  - *ASSOCIATED of a pointer function's value*, `ASSOCIATED (FIND (A, 4, 1))`, was
    NOTPOINTER, though 6b took one as the TARGET argument: `associatedRef` reads a
    function reference through `pointerFunctionValue`, and lowering's `associated`
    makes it the statement's own pointer (`targetPointer`) as it does a target.
  - *Tests.* `sema`'s `TestBoundFromComponent`, `TestOperandArrays`, and
    `TestPointerResults`' ASSOCIATED cases; `internal/fortran/ptrprograms_test.go`
    (`TestValueFromComponent`, `TestOperandArrays`, `TestAssociatedOfFunction`, each
    compiled and assembled). Noticed, not changed: NOTRECORD calls a module function
    whose value's component is referenced, `F(A)%N`, "a procedure with an interface
    body". Subtask 9, the close-out, is next.
- **2026-11-19:** Subtask 9, the close-out. FORTRAN-INTERNALS.md's section 32,
  "Fortran 90: pointers and aliasing", follows a linked list through the passes: the
  three forms of a pointer and why a type may name itself only through one, pointers
  starting disassociated, sema's two meanings of a pointer's name and the IR's
  `pointerTargetMem` (deferred addressing does the rest), `=>` for scalars and an
  array pointer's descriptor filled in place, ALLOCATE's flags and FTNRTL's header
  for DEALLOCATE's check, component chains and the located cells that carried array
  pointer components, dummies, and results, overlap, and why TARGET exists, with a
  loop whose K stays in a register across a store through Q while the TARGET T is
  read each pass; its MACRO is govax's own `/OPTIMIZE` output. MANUAL.md's 14.13
  names pointers whole in its introduction and lists what of pointers isn't there
  (a disassociated pointer's use an ACCVIO at address 0, checked on govax); HELP
  FORTRAN the same; the help library rebuilt (`go generate ./internal/bootdata`).
  PLAN.md and CLAUDE.md summarize the phase.

## Where things stand

Phase 107 is done. What it left, and where each goes:

- **Phase 109:** a whole array pointer (and allocatable array) in an I/O list by its
  descriptor, rather than an element at a time. *(Resolved in Phase 109.)*
- **Phase 110:** `/CHECK=POINTERS` (a disassociated pointer's use, today an ACCVIO at
  address 0); a run-time overlap test in place of a temporary for an array assignment
  that may meet itself through a pointer (the author's answer 6); the debugger's EXAMINE
  of a pointer and through one (the `/DEBUG` table leaves pointers out); `/STANDARD` for
  the phase's forms. *(Resolved in Phase 110, but for the run-time overlap test, Phase 113's A.5.3.)*
- **Phase 112:** `NULL()`, `=> NULL()` initialization, and default initialization of
  components (Fortran 95).
- **Not planned, ours (NOTIMPL):** a CHARACTER pointer whose length is known only when the
  program runs, but for a `CHARACTER(LEN=*)` pointer dummy.
- **Not Fortran 90, not planned:** procedure pointers, bounds remapping (`P(1:N) => A`),
  allocatable components.
- **Noticed, not changed:** NOTRECORD calls a module function whose value's component is
  referenced, `F(A)%N`, "a procedure with an interface body"; a component of an array
  element in a bound, `Y(D(1)%N)`, is still BADBOUND.
- **Unconfirmed choices of ours:** every pointer starting disassociated, and new storage with
  pointers in it cleared (the author's answer 2; Fortran 90 says undefined); BADDEALLOC, FOR
  number 304, and its header (the author's answer 1); a scalar pointer to the one element of
  an array pointer's block of its size giving back the whole block; `ALLOCATE (P, P%NEXT)`
  done in order; the bounds of a pointer to a component of an array of records (from 1,
  Fortran 95's reading); a TARGET judged by its type whether or not `=>` names it; COMMON
  reachable by any pointer (the author's answer 3); ASSOCIATED of a zero-sized target false
  (the author's answer 4).
- **Awaiting a VMS round:** none: no VAX Fortran 90 exists, and the corpus is held to
  gfortran (`compare.py`) but for govax's own `ptrallocvms.f90`, `ptrclear.f90`, and
  `ptrequiv.f90`; Phase 111's optional VMS round may run their MACRO on VMS.
