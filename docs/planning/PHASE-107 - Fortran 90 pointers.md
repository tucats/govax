# Phase 107 — Fortran 90: pointers

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A), done 2026-11-19 and accepted by the
author the same day ("The author's answers").

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
2. **Scalar and character pointers**, `=>`, NULLIFY, ASSOCIATED.
3. **Array pointers** and pointers to sections.
4. **ALLOCATE and DEALLOCATE of pointers.**
5. **Pointer components**, recursive types.
6. **Pointer dummies and results.**
7. **The optimizer's aliasing rules**, with programs that would break if they
   were wrong, both settings.
8. **Programs end to end** (a linked list and a binary tree in modules, a sparse
   matrix), every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 32 (pointers and aliasing),
   MANUAL.md.

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
   text's POINTER.
3. **Array pointers:** descriptors read through every stride, `=>` to whole
   arrays, sections, components, and pointers with their bounds, the inquiries,
   ASSOCIATED of arrays, actual arguments to assumed-shape and old dummies
   (findings 3 to 5), overlap (finding 10).
4. **ALLOCATE and DEALLOCATE of pointers:** the parser's objects, FTNRTL's flag
   bits, `FTN$ALLOC_SCALAR` and `FTN$DEALLOC_SCALAR`, the header and condition
   304, the clear bit, GSMATCH 1,25, MANUAL.md's FTNRTL table (finding 6).
5. **Pointer components and recursive types:** `derived.go`, `HasPointers` and
   its refusals, constructors, component chains, `rankedStride` through a
   pointer, the module text's self-referencing types (finding 7).
6. **Pointer dummies and results** (findings 8, 9).
7. **The optimizer's aliasing rules:** `PointerRegion`, TARGETs' places, taken
   TARGETs, with programs that would print something else if the rule were wrong
   or too bold, both settings (finding 11).
8. **Programs end to end** (a linked list and a binary tree in modules, a sparse
   matrix, the standard's examples), every mode, gfortran's witness.
9. **Close-out:** FORTRAN-INTERNALS.md's section 32 (pointers and aliasing, a
   linked list's code, why TARGET exists), MANUAL.md's 14.13 and FTNRTL table,
   HELP FORTRAN.

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
