# Phase 104 — Fortran 90: dynamic arrays

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A, below, with a revised subtask list
and six questions for the author), was accepted by the author 2026-11-18
("The author's answers").
Needs Phase 103 (done).

## Goal

Arrays whose shape, or whose place, is known only at run time:

- **NCA descriptors** (Decision 7, the Calling Standard's 5.7) built, passed,
  and read: the IR's descriptor gains class NCA, with strides and bounds, and
  `Mem.Ptr` reaches an element through A0 and strides;
- **assumed-shape dummies** (`REAL A(:, :)`): passed an NCA descriptor by every
  caller that has an explicit interface (a whole array, a section with any
  strides, another assumed-shape array); `LBOUND` of a dummy is 1 unless
  declared (`A(0:, :)`);
- **sections to old dummies**: a section or assumed-shape array passed to an
  explicit-shape or assumed-size dummy is copied to a contiguous temporary and
  back when not known contiguous (the standard's 12.4.1.1);
- **automatic arrays** (`REAL W(SIZE(A))`, `CHARACTER(LEN=N) :: S`) in the frame
  (Decision 8);
- **ALLOCATABLE, ALLOCATE, DEALLOCATE, ALLOCATED**, with STAT= and the errors
  (allocating an allocated array, deallocating an unallocated one); a local
  deallocated at return (Decision 8; question 4 of Phase 99);
- **array-valued functions** and functions of run-time character length, their
  result through a hidden descriptor argument, as CHARACTER results are today;
- the **transformational intrinsics** on run-time shapes: MATMUL, TRANSPOSE,
  RESHAPE (of values that aren't constants), SPREAD, PACK, UNPACK, CSHIFT,
  EOSHIFT, and the reductions of Phase 102 over descriptors, with a `DIM=` that
  isn't constant on an array of rank above 1; MERGE of CHARACTER values of a
  run-time length (Phase 102 compiles MERGE otherwise);
- the scalarizer's **temporaries of run-time size**, in the frame or, past the
  limit, on the heap (Decision 8): an assignment the overlap test sends through
  a temporary, a constructor not stored straight into its target, a WHERE
  mask, and a vector subscript in the target that reads the target (`IV(IV) =
  1`), all NOTIMPL in Phase 102 when their extents aren't constants; and
  CHARACTER values of a run-time length in array expressions (Phase 102's
  `checkArrayParts`).

## What earlier phases leave in place

- Class A descriptors and adjustable arrays (`ir/lowerbounds.go`); the frame and
  `ir.Routine.NewCell` (Phase 89); class D strings and `lowerer.after`.
- LIB$GET_VM and LIB$FREE_VM; FTNRTL's error model (`FTN$$IO_SIGNAL`, FOR's
  messages).
- The scalarizer and explicit interfaces (Phases 102, 103).

## Design notes

- **Teaching descriptors.** FORTRAN-INTERNALS.md draws the NCA descriptor of
  `A(2:10:2, J)` beside the array's memory and shows how A0 and the strides give
  each element's address with one multiply-add per dimension; then why an
  assumed-shape dummy needs the caller's interface (the callee can't know how it
  was called).
- **ALLOCATE** is an FTNRTL call (`FTN$ALLOCATE(desc, element size, bounds...)`,
  Decision 15) that fills the descriptor and takes the memory from LIB$GET_VM, so
  its error handling (STAT= or a signal) is in one place; DEALLOCATE gives it back
  and clears POINTER.
- **Strides are bytes**, so a section of a REAL*8 array with stride 2 has stride
  16; a contiguous array has strides equal to the element size times the lower
  dimensions' extents.
- **The copy-in/out rule** uses a "known contiguous" flag from sema: a whole
  explicit-shape array, a section with unit stride in the first dimension and
  whole later dimensions, never an assumed-shape dummy (unless `/ASSUME=CONTIGUOUS`
  is wanted later).
- **FTNRTL's array movers** take NCA descriptors and an element size, so one
  routine serves every type.
- **From Phase 100's survey** (2026-11-17): a dummy's CHARACTER length that
  isn't constant (`CHARACTER(LEN=N) :: S`, N a dummy), not FORTRAN-77's, needs
  the same computation at entry as an automatic object, and is NOTIMPL until
  this phase.

## Open questions

1. The heap limit for temporaries (64 KB proposed): fixed, or a qualifier?
2. An allocatable array in COMMON is not allowed (the standard), but one in
   EQUIVALENCE neither; confirm no DEC extension wants either.

## Subtasks (provisional)

Replaced by Part A's A.4.

1. **Survey** (the standard's 5.1.2.4, 6.3, 12.4.1.1, 13.10.13 to 13.10.18; the
   Handbook's chapters 5, 6, 12), the NCA layout checked in MACRO, for review.
2. **NCA descriptors in the IR** and an element through them.
3. **Assumed-shape dummies**, with every kind of actual.
4. **Copy-in and copy-out** for old dummies.
5. **Automatic arrays** and run-time-sized temporaries.
6. **ALLOCATABLE and ALLOCATE/DEALLOCATE** (`ftn_alloc.mar`), STAT=, the new
   FOR conditions, deallocation at return.
7. **Array-valued functions** and run-time character lengths.
8. **The transformational intrinsics** (`ftn_array.mar`, or in line where Phase
   102's are), each against the standard's examples.
9. **Programs end to end** (a linear-equation solver with assumed-shape
   arrays, a sieve with ALLOCATE), every mode, gfortran's witness.
10. **Close-out**: FORTRAN-INTERNALS.md's section 29 (descriptors and dynamic
    storage), MANUAL.md.

# Part A — the survey (subtask 1)

Read for it: the standard's 5.1.2.4 (explicit-shape, automatic, assumed-shape,
deferred-shape, and assumed-size arrays), 5.1.2.9 and 5.2.6 (ALLOCATABLE),
5.1's constraints on attributes, 6.3 (ALLOCATE, DEALLOCATE, and 6.3.3.1's
statuses at return), 7.1.6.2 (specification expressions), 12.3.1.1 (when an
interface must be explicit), 12.4.1.1 and 12.4.1.4 (argument association and
sequence association), and 13.10.13 to 13.10.19 (the array functions); the
HP Fortran manual's chapter 6 (dynamic allocation) and its rules for COMMON,
EQUIVALENCE, DATA, and NAMELIST; the OpenVMS Calling Standard's 5.4 and 5.7
(class A and class NCA descriptors) and 3.8.7.3 (function values by
descriptor); and what sema, the IR, codegen, the optimizer, FTNRTL, and the
`/DEBUG` table do today. There is no VMS probe: VAX FORTRAN had no Fortran 90,
and DEC Fortran 90 never ran on a VAX, so the standard is the target and
gfortran the witness (`PHASE-99`'s Part B).

## A.1 What the standard asks

- **Five kinds of array** (5.1.2.4). An **explicit-shape** array has its bounds
  written; if they aren't constants it must be a dummy (FORTRAN-77's adjustable
  array), a function result, or an **automatic array**: a procedure's local
  whose bounds are specification expressions, evaluated at entry and fixed for
  the activation (5.1.2.4.1). An **assumed-shape** array (`A(:)`, `B(0:)`) is a
  nonpointer dummy that takes its extents from its actual; its lower bound is
  the one declared, 1 by default, never the actual's (5.1.2.4.2). A
  **deferred-shape** array (`A(:)` with ALLOCATABLE or POINTER) has a rank and
  nothing else until ALLOCATE (or, for a pointer, pointer assignment: Phase 107)
  gives it bounds (5.1.2.4.3). An **assumed-size** array (`A(N,*)`) is
  FORTRAN-77's, unchanged. Rank is at most 7.
- **Specification expressions** (7.1.6.2). An automatic array's bounds, an
  automatic CHARACTER object's length (`CHARACTER(LEN=N) :: S`), and an
  adjustable array's bounds may use constants, dummies (not OPTIONAL, not
  INTENT(OUT)), COMMON and host-associated variables, and the intrinsic
  **inquiry functions** of their arguments' bounds and lengths (SIZE, LBOUND,
  UBOUND, LEN, ...) and elemental intrinsics of integer arguments:
  `REAL W(SIZE(A, 1))` is the usual form. An automatic object may not be SAVE'd,
  in COMMON, EQUIVALENCE, DATA, or NAMELIST, nor appear in a main program.
- **ALLOCATABLE** (5.1's constraints, 5.2.6). Only on an array, declared with a
  deferred-shape list, that is **not a dummy and not a function result** (both
  are Fortran 2003's), never with POINTER, never PARAMETER. The HP manual adds
  what the standard says elsewhere: no allocatable array in COMMON,
  EQUIVALENCE, DATA, or NAMELIST, which answers this plan's open question 2: no
  DEC extension wants either.
- **ALLOCATE and DEALLOCATE** (6.3.1, 6.3.3). `ALLOCATE (X(N), B(-3:M, 0:9),
  STAT=IERR)`: each object gets bounds from scalar integer expressions (lower 1
  by default; an upper below the lower gives extent 0, a zero-sized array, which
  is still allocated). A bound may not ask an inquiry function about an object
  of the same statement. STAT= is 0 on success and a processor-dependent
  positive value on an error; with no STAT= an error **ends the program**.
  Allocating an allocated array and deallocating an unallocated one are errors.
  `ALLOCATED(A)` is the status (13.13.9), a default LOGICAL.
- **Status at return** (6.3.3.1). A RETURN or END keeps the allocation of an
  allocatable array that is SAVE'd, a module's (Phase 106), or **reached by host
  association**; every other allocated one becomes undefined, which Decision 8
  (with question 4 of Phase 99) makes deallocated, Fortran 95's rule and the HP
  manual's: "If an allocatable array does not have the SAVE attribute, it has the
  status of 'not currently allocated' at the beginning of each invocation".
- **Argument association** (12.4.1.1, 12.4.1.4). An assumed-shape dummy's actual
  must have its rank and must not be a scalar (an element included) nor an
  assumed-size array; a CHARACTER one's length must agree. Any array actual
  (whole, section, expression) to an explicit-shape or assumed-size dummy is
  **sequence associated**: the dummy sees the actual's elements in array element
  order, whatever their storage, which for a section means a copy; an array
  element actual still passes the rest of the array after it. A section with a
  vector subscript isn't definable, so its dummy may not be INTENT(OUT) or INOUT.
- **When an interface must be explicit** (12.3.1.1), for this phase: an
  assumed-shape dummy, an array-valued result, and a CHARACTER result whose
  length is neither assumed nor constant.
- **The array functions** (13.10.13 to 13.10.19). Phase 102 compiles MATMUL,
  DOT_PRODUCT, the reductions, the inquiries, TRANSPOSE, MERGE, MAXLOC, MINLOC,
  and RESHAPE of constants. Left for here: **PACK** (ARRAY, MASK, VECTOR=), a
  rank-one result whose size is the mask's count; **UNPACK** (VECTOR, MASK,
  FIELD), MASK's shape, VECTOR's elements in element order where MASK is true;
  **SPREAD** (SOURCE, DIM, NCOPIES), rank plus one; **CSHIFT** and **EOSHIFT**
  (ARRAY, SHIFT, BOUNDARY=, DIM=), ARRAY's shape, SHIFT a scalar or an array of
  rank n-1; **RESHAPE** (SOURCE, SHAPE, PAD=, ORDER=) whose SHAPE has a constant
  size (so the rank is known) but values known only at run time; and a `DIM=`
  that isn't constant where the result's shape depends on it.

## A.2 What govax has

- **Class A descriptors** (`ir.Descriptor`, `ir/lowerbounds.go`). An adjustable
  array's bounds live in a `$Bn` descriptor cell, filled at each entry point
  whose list defines it; an element is A0 plus the subscripts times the
  **multipliers** (extents, in elements), the last step by the VAX's indexed
  mode, scaled by the element's size. `Descriptor` knows the offsets of
  POINTER (4), ARSIZE (12), A0 (16), the multipliers (20 + 4i), and the bounds
  (20 + 4n + 8i), and `Fixed` the longwords known when compiling. A whole
  array is passed to a CHARACTER dummy by class A descriptor (Phase 78).
- **Run-time addresses.** `ir.Mem.Ptr` is any address expression, with an
  optional `Index` scaled by the type; the optimizer hoists it, keeps it in a
  register, and steps an index (Phase 84's invariant motion, induction, and
  register allocation, and `Deferred`'s `@S.$B1+16[R1]`).
- **The frame** (Phase 89): `Routine.FrameSize`, a fixed area below FP made by
  `SUBL2 #n,SP` at each entry; `Cell.Frame` cells FP-relative; a reentrant
  routine's compiler cells all there (`Routine.NewCell`), its descriptor cells
  written whole at each use (`Descriptor.Fixed`).
- **Shapes** (Phase 102, `sema/shape.go`). An `Extent` is a constant or "dimension
  `Dim` of `Of`", computed by lowering; the scalarizer (`ir/lowerarray.go`)
  takes extents from either. Array temporaries (`$An`, `arrayTemp`) are cells of
  a size known when compiling, pooled by type and size per statement.
- **What is NOTIMPL naming this phase**, each where sema finds it: an assumed- or
  deferred-shape declarator (`sema/arrays.go`'s `dimensionBounds`); ALLOCATABLE
  and POINTER (`attrtable.go`'s `attrNotYet`); ALLOCATE and DEALLOCATE (the
  parser's `parsePending`); a section or array expression as an actual argument
  (`arguments.go`'s `arrayArg`); a temporary of run-time size for an overlapping
  assignment (`overlap.go`), a constructor (`constructorSizes`), and a WHERE mask
  (`where.go`); a vector subscript in a target that reads the target (constant
  sizes too, `overlap.go`); RESHAPE of values not constant and DIM= not constant
  (`intrinsicref.go`, `arrayfuncs.go`); MERGE and constructors of CHARACTER
  values of a run-time length, and such values in array expressions
  (`checkArrayParts`); a nonconstant `LEN=` in a type selector (`decl.go`'s
  `characterType`). A non-dummy array with nonconstant bounds is ADJNOTDUMMY, an
  error. PACK, UNPACK, SPREAD, CSHIFT, EOSHIFT, and ALLOCATED have no rows in
  the intrinsic table.
- **FTNRTL's conditions.** FOR's numbers past VAX FORTRAN's begin with govax's
  own NOTCONFOR (300, `ftn_control.mar`); INSVIRMEM (41) is VAX FORTRAN's.
- **The `/DEBUG` table** (`dbgtable`) names a descriptor holding an array's
  bounds (`$Bn`, or `F:n` in the frame), which the debugger reads as class A.

## A.3 Findings, and what they change

1. **The NCA descriptor fits `ir.Descriptor` as it is.** The Calling Standard's
   class NCA (10) has class A's layout to the byte: the same prototype block,
   ARSIZE at 12, A0 at 16, then a longword per dimension at 20 + 4i, then the
   bounds at 20 + 4n + 8i; govax's `$DSCDEF` has `DSC$L_S1` = 20 beside
   `DSC$L_M1` = 20. Only the middle block's meaning differs: **strides in
   bytes** (DSC$L_Si), not multipliers in elements, so an element is at
   A0 + S1·i1 + ... + Sn·in with no scaling. `Class` gains `ClassNCA`; the offset
   methods serve both. Two differences: an NCA's AFLAGS are **0** (the Standard's
   table gives NCA no FL_COLUMN, FL_COEFF, or FL_BOUNDS, and puts FL_UNALLOC and
   FL_NODEALLOC in two of those bit positions, flags VMS 7.1's definitions don't
   have, so govax sets neither), and ARSIZE is the elements' size in bytes when
   contiguous and is written the same way for a section, where the Standard calls
   it meaningless. Old code keeps class A: Phase 79's `$Bn` cells don't change
   (`TestNoF90Unchanged`).
2. **An element through an NCA needs no new addressing.** `Mem.Ptr` already
   takes any address expression: Ptr = A0 + S2·i2 + ... + Sn·in, and for the
   first dimension either `Index` = i1, when S1 is known to be the element's size
   (an allocatable array, which is always contiguous; a whole explicit-shape
   array), or one more S1·i1 term in Ptr when it isn't (an assumed-shape dummy,
   which may be a section with any stride). The second costs one MULL per element
   that a contiguous array doesn't; Phase 84's invariant motion takes A0 and the
   outer terms out of the inner loop, and induction can step S1·i1 by S1. The
   scalarizer's extents for such an array are a new `Extent` source, "dimension d
   of this descriptor", read as Ui - Li + 1. SIZE, LBOUND, UBOUND, and SHAPE read
   the descriptor; LBOUND of a section or an array expression stays 1.
3. **sema's arrays gain two kinds.** A `Dim` may be **deferred** (`:` on an
   ALLOCATABLE array) or **assumed-shape** (`[lb]:` on a dummy without
   ALLOCATABLE, its lower bound a specification expression or 1); every other
   use of `Bound.Colon` stays an error (a local `A(:)` without ALLOCATABLE is
   DEFERREDNOTALLOC, a POINTER is Phase 107's NOTIMPL). ALLOCATABLE joins
   `attrtable.go` with its constraints (ALLOCDUMMY for a dummy or a result,
   ALLOCATTR for COMMON, EQUIVALENCE, DATA, NAMELIST, PARAMETER, or an initial
   value), and the ALLOCATABLE statement its own form.
4. **Allocatable arrays are their descriptors** (Decision 7). Each is an NCA
   descriptor cell in the variable's place (static, or in the frame for a
   reentrant unit), its prototype block and DIMCT known when compiling, POINTER
   0 while unallocated. ALLOCATE is one call per object of `FTN$ALLOCATE(desc,
   flags, l1, u1, ..., ln, un)` (the bounds by value, the element's size taken
   from the descriptor's LENGTH), in `ftn_alloc.mar` (Decision 15): it checks
   POINTER is 0, computes the extents, strides, ARSIZE (EMUL, an overflow past
   2³¹ - 1 bytes being INSVIRMEM), A0, takes the memory from LIB$GET_VM, and
   fills the descriptor. DEALLOCATE is `FTN$DEALLOCATE(desc, flags)`: LIB$FREE_VM
   of ARSIZE bytes, then POINTER 0. ALLOCATED is `TSTL desc+4` in line. Each
   returns a status in R0; with STAT= (the flag set) the caller stores 0 or the
   condition's FOR number, without it FTNRTL signals, as FTN$IO_END does. Two
   choices to log as unconfirmed: a statement with STAT= stops at its first
   failing object, leaving the rest as they were; and a **zero-sized array takes
   an 8-byte block**, so that POINTER is never 0 for an allocated array and
   Decision 7's one-longword status holds (LIB$GET_VM refuses 0 bytes anyway).
   The new conditions, govax's own wording and numbers past NOTCONFOR's 300:
   **ALREADYALLOC** (301, "allocatable array is already allocated") and
   **NOTALLOC** (302, "allocatable array is not allocated"); out of memory is
   VAX FORTRAN's INSVIRMEM (41). STAT= gets 41, 301, or 302.
5. **Deallocation at return, and at entry.** An allocatable local that isn't
   SAVE'd (explicitly or by a bare SAVE) is deallocated at every return of its
   unit (each RETURN, END, alternate return, and ENTRY's shared exit), in line:
   `TSTL desc+4`, and if set, `FTN$DEALLOCATE`. A main program's never are. But a
   routine left by an **unwind** (a condition handler's `$UNWIND`, LIB$SIG_TO_RET)
   runs none of that code, and a static descriptor would still say "allocated" at
   the next call, whose ALLOCATE would then fail with ALREADYALLOC though the
   program did nothing wrong. So each entry point also **clears POINTER** of each
   such array: an unwind then leaks the block (as a heap temporary would,
   finding 8) but never breaks the program. A reentrant unit's descriptors are in
   the frame and must be cleared at entry in any case. An internal procedure that
   allocates its host's array changes the host's descriptor (6.3.3.1(3) keeps it
   allocated), and the host frees it at the host's return.
6. **Run-time sizes in the frame need one new IR statement.** Decision 8's
   frame storage is made by moving SP after the fixed frame area: `SUBL2 size,SP`,
   rounded to a longword, the address `MOVL SP,cell` into a cell (static, or a
   frame cell when reentrant), the area FP-relative only through that cell. Two
   lifetimes:
   - **an automatic object** is made at each entry point, after the fixed area, and
     never given back: RET restores SP from FP, an unwind too;
   - **a statement's temporary** is made at the statement's start and given back at
     its end by restoring SP from a cell saved before (`MOVL SP,$SPn` ...
     `MOVL $SPn,SP`), so a statement in a loop doesn't grow the stack.
   The IR gains `StackAllocate` and `StackRelease` statements, opaque to the
   optimizer's passes (effects: SP changes; invariant motion never moves one, and
   no value is assumed to survive a release). A temporary past **64 KB** comes from
   LIB$GET_VM and goes back with LIB$FREE_VM at the statement's end (question 1),
   so a large array statement doesn't use up the user stack (2 MB with paging off,
   Phase 89); an automatic array has no such limit, being the program's own
   declaration (and runs out as Phase 89's stack does, `%X1000000C`).
7. **Automatic objects are adjustable arrays whose storage the entry makes.** A
   non-dummy array with nonconstant bounds in a subprogram is no longer
   ADJNOTDUMMY but automatic (still ADJNOTDUMMY in a main program and BLOCK
   DATA), under `/F90` and `/NOF90` alike (Decision 2: VAX FORTRAN gave it no
   meaning). Its bounds are computed as an adjustable array's are, into a class A
   `$Bn` cell (its own storage is contiguous, so class A serves), then finding
   6's allocation fills POINTER and A0. `CHARACTER(LEN=expr) :: S` is the same
   with a length; a dummy's nonconstant `LEN=` is the length computed at entry,
   with no storage (the dummy is its actual's leftmost characters, 12.4.1.1). The
   specification expressions grow by 7.1.6.2's inquiry functions (SIZE, LBOUND,
   UBOUND, LEN of a dummy) and host variables, for adjustable bounds too. As with
   adjustable arrays, each entry point makes the automatic objects its list
   defines; under another entry point they are undefined (Phase 79's rule), their
   address cell 0. Their checks: AUTOSAVE (SAVE of one), and the COMMON,
   EQUIVALENCE, DATA, and NAMELIST errors.
8. **The scalarizer's temporaries of run-time size** (Phase 102's NOTIMPLs) become
   finding 6's statement temporaries, each an `$An` cell holding an address with
   its extents in a class A descriptor (they are contiguous): an overlapping
   assignment, a constructor not stored straight into its target, a WHERE mask, a
   vector subscript in the target that reads the target (`IV(IV) = 1`: the
   subscript vector is copied first, whatever its size), and an array of CHARACTER
   values of a run-time length (its element length a run-time value too, each
   element moved by MOVC5). An unwind out of a statement leaves a heap
   temporary allocated; the frame's are recovered.
9. **Assumed-shape dummies are passed an NCA and copied at entry.** The argument
   list's longword is the descriptor's address (the dummy is passed by
   descriptor, Table 6-1's form). Each entry point whose list has the dummy
   copies it into a **local NCA cell** (`$Bn`, in the frame when reentrant) with
   its own bounds: the strides and extents from the actual's descriptor, the
   lower bounds the declared ones (5.1.2.4.2), A0 recomputed as
   POINTER - Σ Si·Li. Everything in the routine then reads one descriptor in a
   known place whatever the caller wrote, a MACRO caller included, and the
   optimizer can hoist from it. The copy **skips an absent OPTIONAL dummy** (a 0
   longword, or a short list), Phase 103's rule that entry code never reads
   through an absent argument. Under `/CHECK=BOUNDS` the copy also checks the
   actual's class byte (10, or 4 from a class A caller, which the copy accepts by
   converting multipliers to strides) and its DIMCT, signaling a new
   **BADARRDESC** (303) when the caller passed something else, as an old-style
   call with no interface would (question 5).
10. **What a caller passes to an assumed-shape dummy** (only through an explicit
    interface: EXPLICITREQ for a same-file call without one, as for OPTIONAL in
    Phase 103; a rank or CHARACTER length that differs is ARGRANK or ARGLENGTH):
    - a whole explicit-shape array of known address: an NCA wholly known when
      compiling, in `$PDATA`, as class A descriptors of whole arrays are today;
    - an adjustable array: an NCA built from its `$Bn` (strides from multipliers);
    - an allocatable array: its own descriptor, already NCA (the callee copies, so
      it never writes it; `opt/effects.go` treats it as read);
    - another assumed-shape dummy: its local cell;
    - a section: an NCA built at the call, POINTER the first element's address,
      each stride the base array's stride times the triplet's, bounds 1 to the
      extent; no copy, even with negative strides;
    - an array expression or a vector-subscripted section: computed into a
      temporary (finding 8), passed by an NCA of it, not copied back.
    An assumed-size array or a scalar is an error (12.4.1.1).
11. **Sections and the rest to old dummies** (explicit-shape, assumed-size, or no
    interface: every FORTRAN-77 call). The actual must arrive as the address of
    contiguous elements in array element order (sequence association):
    - known contiguous when compiling (a whole explicit-shape array; an
      allocatable array; a section with stride 1 in its first ranging dimension
      and whole later dimensions, `A(:, J)`, `A(2:5, :)`): its first element's
      address, no copy, as today;
    - an assumed-shape dummy, or a section not known contiguous: copied into a
      contiguous temporary before the call and back after it, element by element
      by the scalarizer's nest; not back when the dummy is INTENT(IN), an
      expression, or vector-subscripted (12.4.1.1). For an assumed-shape dummy a
      **run-time test** (S1 the element size and each Si+1 = Si times extent i)
      passes POINTER with no copy when it is contiguous after all (question 4);
    - a CHARACTER one by a class A descriptor of whichever it is.
    The temporary is the call's statement's, so a function reference inside an
    expression gets its copy-back when its call returns, before the statement
    goes on (sequence association, 12.4.2's order).
12. **Array-valued functions and run-time CHARACTER lengths** (question 3). A
    function whose result is an array (explicit-shape, 5.1's constraint) or whose
    CHARACTER length is a specification expression has an explicit interface at
    every call (12.3.1.1), so caller and callee always agree. Its result goes
    through a hidden first argument, as a CHARACTER or REAL\*16 result does today
    (`sema.Type.HiddenResult`):
    - **a shape known when compiling** (`REAL F(3)`): the caller passes the
      address of a temporary of that size, which the function's result variable
      is;
    - **a shape or length known only at run time** (`REAL F(N)`, `CHARACTER(LEN=N)
      F`): the caller passes a descriptor with POINTER 0 (an NCA of the result's
      rank, or a class D string), and the callee, at entry, computes the bounds
      from its own specification expressions, takes the storage from LIB$GET_VM
      (LIB$SGET1_DD for a string, as concatenation's are), and fills the
      descriptor; the result variable is reached through it. The caller frees it
      at the statement's end, as a heap temporary.
    The second form is the Calling Standard's "dynamic text" case extended to
    arrays: the callee is the only one that can evaluate its bounds (they may use
    its COMMON, its host's variables, or SIZE of its dummies), and the result is
    contiguous, so a MACRO caller can use it. An array-valued reference in an
    expression is computed into its temporary before the statement (Decision 6);
    `A = F(X)` goes through it too (an optimization for Phase 110: into A
    directly when they can't overlap).
13. **The transformational intrinsics are written in line** (question 2). Each is
    a form the scalarizer already handles, rather than an FTNRTL mover on NCA
    descriptors as the plan proposed:
    - **SPREAD, CSHIFT, EOSHIFT** are index maps, an element of the result being
      an element of the argument at changed subscripts (CSHIFT's is
      `MOD(i - 1 + SHIFT, n) + 1` in its dimension; EOSHIFT's a BOUNDARY value
      outside), so they are elements computed where used, as TRANSPOSE is, with no
      temporary: `ArrayFunc.Element`;
    - **PACK** is two loops in line: COUNT of the mask (or VECTOR's size) for the
      result's extent, a temporary of that size, then the elements where the mask
      holds, in element order, with a running subscript; the mask's expression is
      scalarized in each, never kept;
    - **UNPACK** and **RESHAPE** fill a temporary in array element order with a
      running subscript into VECTOR or SOURCE (RESHAPE's ORDER= permuting which
      loop is innermost, PAD= taking over past SOURCE's end);
    - a **DIM= not constant** where the shape depends on it (a reduction, SPREAD,
      of rank above 1) is a `Switch` on DIM with an arm per dimension, each Phase
      102's constant-DIM code into a temporary of run-time extents; out of range
      under `/CHECK=BOUNDS`, FOR's NOTCONFOR;
    - **MERGE of CHARACTER values of a run-time length** goes with finding 8's
      CHARACTER temporaries.
    Writing them in line keeps one mechanism, lets `/OPTIMIZE` see the loops, and
    needs no MACRO walker of NCA descriptors; FTNRTL then grows only by
    `ftn_alloc.mar` (`ftn_array.mar` isn't needed). Each function joins the
    intrinsic table with its dummies' names and optional arguments (Decision 14),
    as does ALLOCATED.
14. **I/O, the debugger, and the rest.** A whole allocatable or assumed-shape
    array, or a section of one, in an I/O list is an item per element, as Phase
    102's arrays are (by descriptor in Phase 109). The `/DEBUG` table's
    descriptor field already names where an array's bounds are; the debugger
    learns class 10 (the address from POINTER and A0, strides in bytes) so EXAMINE
    of an allocatable or assumed-shape array works; `HOST.INNER`'s scopes and the
    rest stay Phase 110's. Arrays of records stay NOTIMPL (Phase 105), and a POINTER
    array is Phase 107's, on this phase's descriptors and FTN$ALLOCATE.
15. **The optimizer.** No pass changes beyond the rules: a descriptor cell is a
    place like any other (a call changes an allocatable's descriptor only if the
    callee can reach it: COMMON can't hold one; a host's can be reached by its
    internal procedures, `HostRegion`; one passed to an assumed-shape dummy isn't
    written, finding 10); `StackAllocate` and `StackRelease` move nothing across
    them; and data reached through a descriptor is a dummy's or a variable's as
    today. The cost of a second stride multiply for assumed-shape dummies, and of
    copies for sections to old dummies, is measured with the programs of subtask
    9 and left to Phase 110.

## A.4 The subtasks, revised

The plan's order put NCA descriptors before anything that makes one; this order
makes allocatable arrays first, the simplest descriptors to test end to end (no
calls), then the frame's run-time storage everything after needs.

1. **Survey** (this Part A) (done 2026-11-18; accepted by the author the
   same day).
2. **NCA descriptors and allocatable arrays:** `ir.ClassNCA`, elements through a
   descriptor (finding 2), deferred-shape declarations and ALLOCATABLE (finding
   3), `ftn_alloc.mar`'s FTN$ALLOCATE and FTN$DEALLOCATE, ALLOCATE, DEALLOCATE,
   ALLOCATED, the inquiries and the scalarizer's extents from a descriptor.
3. **Allocation status:** STAT=, ALREADYALLOC, NOTALLOC, INSVIRMEM, zero sizes,
   deallocation at return and clearing at entry (finding 5), reentrant units,
   hosts and internal procedures; an unwind's test.
4. **Run-time storage in the frame:** `StackAllocate` and `StackRelease`, automatic
   arrays and CHARACTER objects, nonconstant `LEN=` on dummies, the inquiry
   functions in specification expressions (finding 7); the scalarizer's temporaries
   of run-time size, the heap past 64 KB, CHARACTER values of a run-time length in
   array expressions, `IV(IV)` (finding 8).
5. **Assumed-shape dummies:** the entry copy, every kind of actual, EXPLICITREQ,
   rank and length checks, INTERFACEDIFF's new characteristics, OPTIONAL and ENTRY,
   `/CHECK=BOUNDS`'s descriptor check (findings 9 and 10).
6. **Sections and expressions to old dummies:** contiguity known, the run-time
   test, copy-in and copy-out, INTENT and vector subscripts, CHARACTER (finding 11).
7. **Array-valued functions and run-time CHARACTER lengths** (finding 12),
   internal and external, recursive.
8. **The transformational intrinsics** (finding 13), each against the standard's
   examples, and the intrinsic table's rows.
9. **Programs end to end** (a linear-equation solver with assumed-shape arrays, a
   sieve with ALLOCATE, a sort with PACK and automatic work arrays), every mode,
   gfortran's witness; the debugger's EXAMINE through an NCA (finding 14).
10. **Close-out:** FORTRAN-INTERNALS.md's section 29 (descriptors and dynamic
    storage: the NCA of `A(2:10:2, J)` beside its memory, why an assumed-shape
    dummy needs an interface, the frame and the heap), MANUAL.md's 14.13 and its
    FOR messages, HELP FORTRAN.

## A.5 Questions for the author

1. **The heap limit for temporaries** (the plan's open question 1; finding 6).
   (a) **A fixed 64 KB, a constant in the compiler; a qualifier later if a
   program wants one**; (b) a qualifier setting the limit now; (c) every run-time
   temporary on the heap.
2. **Where the transformational intrinsics and the copies are** (finding 13).
   (a) **In line, by the scalarizer, `ftn_array.mar` not written**; (b) FTNRTL
   movers on NCA descriptors and an element size, as the plan's design note said.
3. **Array-valued and run-time-length results** (finding 12). (a) **The caller's
   temporary when the shape is known when compiling, else the callee's heap block
   through the caller's descriptor**; (b) always the callee's heap block; (c) the
   caller evaluates the result's bounds by putting its actuals into the callee's
   specification expressions (gfortran's way), which fails when they use the
   callee's COMMON or host.
4. **An assumed-shape array to an old dummy** (finding 11). (a) **A run-time
   contiguity test, copying only when it fails**; (b) always copy.
5. **The descriptor check** (finding 9). (a) **Under `/CHECK=BOUNDS`, an
   assumed-shape dummy's actual must be a class NCA or A descriptor of its rank,
   else BADARRDESC**; (b) no check (a call without the interface is the
   program's mistake, as Decision 7 says).
6. **Unwinds and allocatable locals** (finding 5). (a) **Clear each non-SAVE
   allocatable's POINTER at entry, so an unwind leaks but never makes the next
   ALLOCATE fail**; (b) don't clear (an unwound routine's next ALLOCATE of it is
   ALREADYALLOC, which is what the program's state says).

## The author's answers (2026-11-18)

The author read the survey and accepted every recommendation:

1. Temporaries of run-time size are in the frame up to a fixed 64 KB, a
   constant in the compiler, and on the heap past it; a qualifier can come
   later if a program wants one (finding 6). This settles the plan's open
   question 1.
2. The transformational intrinsics and the copies for old dummies are written
   in line by the scalarizer; `ftn_array.mar` isn't written (finding 13).
3. An array-valued or run-time-length result is the caller's temporary when its
   shape is known when compiling, else a heap block the callee allocates through
   the caller's descriptor, freed by the caller at the statement's end
   (finding 12).
4. An assumed-shape array passed to an old dummy gets a run-time contiguity
   test, and is copied only when it fails (finding 11).
5. Under `/CHECK=BOUNDS`, an assumed-shape dummy's actual must be a class NCA or
   A descriptor of its rank, else BADARRDESC (finding 9).
6. Each non-SAVE allocatable local's POINTER is cleared at entry, so an unwind
   leaks its block but never makes the next ALLOCATE fail (finding 5).

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-18, subtask 1:** The survey written (Part A): the standard's 5.1.2.4,
  6.3, 7.1.6.2, 12.3.1.1, 12.4.1, and 13.10 read, with the HP manual's chapter 6
  and the Calling Standard's 5.7, and the passes that meet them. Its main
  findings: the NCA descriptor has class A's layout with strides in bytes, so
  `ir.Descriptor` serves both and `Mem.Ptr` reaches an element with no new
  addressing; allocatable arrays are their descriptors, made by FTNRTL's
  FTN$ALLOCATE, deallocated at return and cleared at entry against unwinds; one
  new IR statement pair moves SP for automatic objects and run-time temporaries;
  an assumed-shape dummy is copied at entry into a descriptor with its own
  bounds; sections reach old dummies by a contiguity rule and copies; run-time
  results come back in a block the callee allocates; and the transformational
  intrinsics are all in line, so `ftn_array.mar` isn't needed. The plan's open
  question 2 is answered from the standard and the HP manual (no allocatable in
  COMMON or EQUIVALENCE). A revised subtask list (allocatable arrays first) and
  six questions for the author.
- **2026-11-18, subtask 1 done:** The author accepted every recommendation
  ("The author's answers"); subtask 2 is next.
