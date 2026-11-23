# Phase 104 — Fortran 90: dynamic arrays

**Status:** done 2026-11-18. Planned 2026-11-17 with Phase 99 (`PHASE-99 -
Fortran 90.md`, Part C). Subtask 1, the survey (Part A, below, with a revised
subtask list and six questions for the author), was accepted by the author
2026-11-18 ("The author's answers"). Subtasks 2 (allocatable arrays), 3
(allocation status), 4 (run-time storage: 4a automatic arrays, 4b CHARACTER
lengths, 4c the scalarizer's temporaries), 5 (assumed-shape dummies, 5a and 5b),
6 (sections and expressions to old dummies), 7 (array-valued functions: 7a a
known shape, 7b a run-time shape, 7c run-time CHARACTER lengths and ENTRY), 8
(the transformational intrinsics), 9 (programs end to end and the debugger), and
10 (the close-out) are done; "Where things stand", at the end, is what the phase
left.
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
   ALLOCATED, the inquiries and the scalarizer's extents from a descriptor
   (done 2026-11-18).
3. **Allocation status:** STAT=, ALREADYALLOC, NOTALLOC, INSVIRMEM, zero sizes,
   deallocation at return and clearing at entry (finding 5), reentrant units,
   hosts and internal procedures; an unwind's test (done 2026-11-18).
4. **Run-time storage in the frame:** `StackAllocate` and `StackRelease`, automatic
   arrays and CHARACTER objects, nonconstant `LEN=` on dummies, the inquiry
   functions in specification expressions (finding 7); the scalarizer's temporaries
   of run-time size, the heap past 64 KB, CHARACTER values of a run-time length in
   array expressions, `IV(IV)` (finding 8). In three parts (2026-11-18):
   - 4a. `StackAllocate` and `StackRelease`, automatic arrays, and the inquiry
     functions in specification expressions (done 2026-11-18).
   - 4b. Automatic CHARACTER objects and a dummy's nonconstant `LEN=` (done
     2026-11-18).
   - 4c. The scalarizer's temporaries of run-time size, the heap past 64 KB,
     CHARACTER values of a run-time length in array expressions, `IV(IV)` (done
     2026-11-18).
5. **Assumed-shape dummies:** the entry copy, every kind of actual, EXPLICITREQ,
   rank and length checks, INTERFACEDIFF's new characteristics, OPTIONAL and ENTRY,
   `/CHECK=BOUNDS`'s descriptor check (findings 9 and 10). In two parts
   (2026-11-18):
   - 5a. The declarations, the entry copy (OPTIONAL and ENTRY among them),
     elements and every whole-array use in the routine, and as actuals whole
     arrays of every kind and sections without vector subscripts; EXPLICITREQ,
     ARGRANK, ARGLENGTH, INTERFACEDIFF (done 2026-11-18).
   - 5b. Array expressions and vector-subscripted sections as actuals, through
     temporaries; an absent OPTIONAL one passed on as absent; generic names
     whose specifics have them; `/CHECK=BOUNDS`'s BADARRDESC, and a class A
     descriptor accepted from a MACRO caller (done 2026-11-18).
6. **Sections and expressions to old dummies:** contiguity known, the run-time
   test, copy-in and copy-out, INTENT and vector subscripts, CHARACTER (finding 11)
   (done 2026-11-18).
7. **Array-valued functions and run-time CHARACTER lengths** (finding 12),
   internal and external, recursive. In three parts (2026-11-18):
   - 7a. Array values of a shape known when compiling: the caller's temporary
     through the hidden argument, references through every kind of explicit
     interface, EXPLICITVALUE, INTERFACEDIFF of the value (done 2026-11-18).
   - 7b. Array values of a shape known only at run time: the callee's heap block
     through the caller's NCA descriptor, freed at the statement's end (done
     2026-11-18).
   - 7c. CHARACTER values of a run-time length, scalar and array, through a
     class D descriptor; ENTRY in an array-valued function (done 2026-11-18).
8. **The transformational intrinsics** (finding 13), each against the standard's
   examples, and the intrinsic table's rows (done 2026-11-18).
9. **Programs end to end** (a linear-equation solver with assumed-shape arrays, a
   sieve with ALLOCATE, a sort with PACK and automatic work arrays), every mode,
   gfortran's witness; the debugger's EXAMINE through an NCA (finding 14) (done
   2026-11-18).
10. **Close-out:** FORTRAN-INTERNALS.md's section 29 (descriptors and dynamic
    storage: the NCA of `A(2:10:2, J)` beside its memory, why an assumed-shape
    dummy needs an interface, the frame and the heap), MANUAL.md's 14.13 and its
    FOR messages, HELP FORTRAN (done 2026-11-18).

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
- **2026-11-18, subtask 2 done:** Allocatable arrays, end to end.
  - *Syntax.* `parse/f90.go`'s `parseAllocate` reads ALLOCATE's and
    DEALLOCATE's lists into `ast.AllocateStmt` (each object a `Declarator`,
    its bounds read by `dimensions`; `STAT=` last), and the ALLOCATABLE
    statement is `parseAttr`'s, its items with array specifications as
    TARGET's (`TestAllocateForms`).
  - *Declarations* (`sema/allocate.go`). A colon bound is `Dim.Deferred`
    (`(lb:)`, a dummy's assumed shape, is NOTIMPL for subtask 5); the
    ALLOCATABLE attribute and statement set `Symbol.Allocatable`
    (`attrtable.go`, read whole as INTENT's are). `checkShapes` (from
    `checkArrays`, once every declaration is read) settles each deferred
    shape: an allocatable's must be all colons (ALLOCSHAPE, DEFERREDMIX), a
    dummy's colons are NOTIMPL, any other's are DEFERREDNOTALLOC; an
    allocatable array of records is NOTIMPL (Phase 105). `checkAllocatables`
    (after `equivalences`) is ALLOCDUMMY and ALLOCATTR for COMMON and
    EQUIVALENCE; DATA's (`dataName`) and NAMELIST's (`checkNamelists`) where
    those are read. `Dim.LowerKnown` and `UpperKnown` replace the tests of
    `LowerExpr == nil` that took a bound for a constant (sections' default
    bounds, constant subscripts' checks, overlap, lowering). An allocatable's
    `Symbol.Size` is its descriptor's (`DescriptorSize`), on a longword.
  - *Statements and the intrinsic.* `sema.Allocate` (NOTALLOCATABLE,
    ALLOCOBJECT, ALLOCBOUNDS; ALLOCSELF read as any mention of an object of
    the same statement in a bound, since 6.3.1 forbids its value, bounds, and
    status alike); `STAT=` is NOTIMPL until subtask 3. ALLOCATED is
    `sema.Allocated`, built as PRESENT is (no row in the table; a name only
    under `/F90`, Phase 99's rule for new intrinsic names).
  - *The IR* (`ir/lowerallocate.go`). `ClassNCA` (10), `StrideOffset`, and
    NCA's `Fixed` (flags 0, the rank, POINTER 0). The allocatable's own cell
    is its descriptor (`allocatableCell`: a `DescriptorCell` with `Sym` set,
    registered among the adjustable arrays' cells so that every reader of
    bounds and extents finds it: `lowerBound`, `upperBound`, and `extent`, U -
    L + 1 for NCA). An element is `stridedElement`: A0 plus each later
    subscript times its stride, indexed by the first (`@A+16[R6]` for rank
    one, from Phase 84's deferred indexes). A whole array by reference is
    its POINTER (`allocatableBase`, through `symbolMem`); anything that reads
    only class A (FTNRTL's I/O lists, run-time formats, CHARACTER arguments)
    gets a class A copy (`ncaAsClassA`). ALLOCATE and DEALLOCATE are a call
    per object; ALLOCATED a test of POINTER. A recursive unit's allocatable
    is in the frame, and each entry point writes its fixed longwords and a 0
    POINTER (`clearAllocatables`; finding 5's clearing of static ones is
    subtask 3's).
  - *The optimizer.* An allocatable's elements are a region of their own,
    `opt.AllocatedRegion`, keyed by the descriptor cell (`allocatedBase` finds
    it in an element's address): they conflict only with each other, and any
    call may change them.
  - *FTNRTL* (`mar/ftn_alloc.mar`, GSMATCH 1,22; vector 688 and 696):
    `FTN$ALLOCATE(desc, flags, l1, u1, ...)` and `FTN$DEALLOCATE(desc, flags)`
    as finding 4 has them, with the status and STAT= flag ready for subtask
    3; ALREADYALLOC (301) and NOTALLOC (302) in `vmsdef/formessages.go`.
    Choices of ours, unconfirmed (no VAX FORTRAN had ALLOCATE): a size past
    2**31 - 1 bytes is INSVIRMEM, found by EMUL before LIB$GET_VM is asked;
    INSVIRMEM is signaled with VAX FORTRAN's I/O arguments, unit 0 and no
    file; a block of at least 8 bytes, so that a zero-sized array's POINTER
    isn't 0, and DEALLOCATE gives back the same size.
  - *Left for later:* the `/DEBUG` table leaves allocatable arrays out until
    the debugger reads class NCA (subtask 9).
  - *A fix found on the way:* a whole array used only in an expression
    (`SUM(A)`, `B + 1.0`) wasn't noted as used, so `/WARNINGS=UNUSED` called
    it unused (`sema/expr.go`'s `arrayValue`; `TestWarningsUnusedWholeArray`).
  - *Tests:* `sema`'s `TestAllocatable` (every message), the `f90alloc`
    goldens in `ir`, `sema`, and `codegen`, and `testdata/fortran90/allocs.f90`
    (every mode, paging both ways; gfortran prints the same 18 lines).
    MANUAL.md's 14.13 has "Allocatable arrays" and FTNRTL's two routines,
    and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 3 done:** Allocation status.
  - *STAT=* (`sema/allocate.go`'s `statVariable`): an INTEGER variable or
    array element of any kind, as IOSTAT='s is (BADSTAT), using no object of
    its statement (STATSELF, 6.3.1's and 6.3.3's constraint read as any
    mention, as ALLOCSELF is); `sema.Allocate.Stat`. Lowered
    (`ir/lowerallocate.go`'s `allocate`) as each call with flag 1 and its R0 in
    a temporary, a branch to the statement's end on a failure, then the
    temporary stored (converted to STAT's kind). The statement stops at the
    first failing object, leaving the rest as they were: our choice, and
    gfortran does the same (`allocstat.f90`'s "stops at b").
  - *Deallocation at return* (finding 5). `sema.Symbol.FreedAtReturn`
    (`freedAtReturn`, after `recursiveStorage`): a subprogram's own allocatable
    array not saved by SAVE with its name, a SAVE with no list, or STATIC. A
    main program's never are, and a host's array an internal procedure uses is
    the host's symbol, so only the host frees it. `freeAllocatables` is in line
    at every return, `TSTL` of POINTER and a call of `FTN$DEALLOCATE` with no
    STAT=, from `returnFromRoutine` (RETURN, END, and so ENTRY's shared and
    typed returns) and RETURN i, whose number goes to a temporary first, since
    it may be the array's element (`alternate`).
  - *Clearing at entry*. `clearAllocatables` also writes 0 to a static
    FreedAtReturn array's POINTER at every entry point (`entryClears`, which
    `layOutEntryBounds` now asks), the author's answer 6; a frame one's
    descriptor is still written whole.
  - *FTNRTL* needed nothing: subtask 2's `ftn_alloc.mar` already had the flag,
    the statuses, INSVIRMEM by EMUL, and the 8-byte least block.
  - *Tests:* `sema`'s `TestAllocatable` (STAT='s messages) and
    `TestFreedAtReturn`; the `f90stat` goldens in `sema`, `ir`, and
    `codegen`; `testdata/fortran90/allocstat.f90` (every mode; gfortran prints
    the same) and `allocvms.f90` (STAT='s FOR numbers, the three conditions
    signaled and returned by LIB$SIG_TO_RET, an unwind out of a routine with
    its array allocated and its next call, LIB$STAT_VM's bytes unchanged by a
    recursion that never deallocates). MANUAL.md's 14.13 and HELP FORTRAN
    describe STAT= and deallocation at return.
- **2026-11-18, subtask 4 split** into 4a (automatic arrays), 4b (automatic
  CHARACTER lengths), and 4c (the scalarizer's run-time temporaries), each a
  commit (A.4).
- **2026-11-18, subtask 4a done:** Automatic arrays and the stack statements.
  - *Declarations* (`sema/arrays.go`). A non-dummy array with a bound that isn't
    constant is `Symbol.AutomaticObject` in a function or subroutine
    (`automaticAllowed`), under `/F90` and `/NOF90` alike, and still ADJNOTDUMMY in
    a main program or block data (three of `TestErrors`' cases changed to say
    so). Its storage is its class A descriptor (`Symbol.Size`, on a longword),
    in the frame in a recursive unit whatever a bare SAVE says
    (`recursiveStorage`). `checkAutomatics` (after `storageStatements`) is
    AUTOATTR for COMMON, EQUIVALENCE, SAVE, and STATIC; DATA's and NAMELIST's
    where those are read. One message where the survey named two (AUTOSAVE
    folded into AUTOATTR, "can't be saved"): ours.
  - *Specification expressions* (7.1.6.2). A bound may call SIZE, LBOUND,
    UBOUND, and LEN, whose first argument is a name (an array's among them), and
    the elemental functions of integers ABS, IABS, MAX, MIN, MAX0, MIN0, MOD,
    INT, NINT, IFIX, IDINT, IDNINT, DIM, IDIM, SIGN, ISIGN (`boundFunction`,
    keyword arguments read), where the unit hasn't made the name something
    else; any other function is BOUNDREF, as before. An inquiry's argument must
    be a dummy, in common, or a host's (BOUNDVAR otherwise), and not
    allocatable or automatic (BOUNDREF: F90's restricted expressions; Fortran
    95 would allow an earlier declaration's). This holds for adjustable dummies'
    bounds too.
  - *The IR.* `StackAllocate` (`SUBL2 size,SP`, `BICL2 #3,SP`, `MOVL SP,dst`)
    and `StackRelease` (`MOVL to,SP`; nothing makes one until 4c), in `Check`,
    the dump, codegen (`stackAllocate`, `stackRelease`), and every optimizer
    walk: the two walkers, `StmtEffects`, Forward, dead code's references, and
    CSE's replacer. No pass moves one.
  - *Lowering* (`ir/lowerautomatic.go`). The variable's own cell is the
    descriptor (`automaticCell`), among the adjustable arrays' cells after the
    dummies' (`prepareBounds`), so every reader of bounds, extents, elements,
    and whole arrays finds it; the base is POINTER (`symbolMem`, as an
    allocatable's). `computeAutomatic` at each entry point that defines it
    (`defines`: its bounds' dummies only): bounds, extents made 0 if negative
    (`notNegative`; an adjustable dummy's aren't changed, and /CHECK=BOUNDS's
    ADJARRDIM doesn't apply), ARSIZE, the storage, A0. Code lowered before the
    first statement has the header as its origin (a bound's SIZE needs a
    statement's line, `newNest`).
  - *The optimizer.* An automatic array's elements are `AllocatedRegion`, keyed
    by its descriptor (`allocatedBase`).
  - *Left for later:* the `/DEBUG` table leaves automatic arrays out with the
    allocatable ones (subtask 9).
  - *A fix found on the way:* the array temporaries (Phase 102) and the cells
    holding a dummy's address in a unit with ENTRY or an internal procedure
    (Phases 79 and 103) were both named `$An`, so a host with both had two
    labels of one name (MACRO's OUTOFPHASE and DUPSYM), and in a frame host
    one cell for both. Array temporaries are now `$Wn` (work arrays); four
    `ir` goldens renamed.
  - *Tests:* `sema`'s `TestAutomatic`, the `f90auto` goldens in `sema`, `ir`,
    and `codegen`, and `testdata/fortran90/autos.f90` (every mode, paging both
    ways; gfortran prints the same 12 lines). MANUAL.md's 14.13 has
    "Automatic arrays", and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 4b done:** CHARACTER lengths that aren't constant.
  - *Declarations* (`sema/charlength.go`). A type statement's entity whose
    length, in either form (`CHARACTER(LEN=N)`, `CHARACTER*(N)`), isn't constant
    is CHARACTER of length 0, as a passed length is, with `Symbol.LengthExpr`
    and `LengthValue` (`entityType`, `runtimeLength`); `Symbol.PassedLength`
    tells the two apart. The length is checked as a bound is
    (`adjustableArray.length`: `boundOperands`, `checkBoundName`, `boundValues`,
    its dummies joining `BoundDummies`), and BADBOUND, BOUNDVAR, and BOUNDREF
    now say "a CHARACTER length" or "an array's bounds". A subprogram's own
    such name is `AutomaticObject` (a scalar's storage a class S descriptor,
    `StringDescriptorSize`), with AUTOLENATTR beside AUTOATTR (`autoAttr`); in a
    main program or block data it is LENNOTDUMMY (`CHARACTER*(N)` there was
    BADCHARLEN, and the selector's form NOTIMPL). IMPLICIT's and a structure
    field's lengths still must be constant (`typeOf`); a PARAMETER's is
    BADCHARLEN, and a function's value of such a length NOTIMPL until subtask 7
    (`checkLengths`). Under `/F90` and `/NOF90` alike, as 4a's arrays.
  - *LEN of an array* (`sema/intrinsicref.go`): of a whole array or a section
    whose length isn't constant, its elements' length (`ir`'s `charLen`), which
    was NOTIMPL.
  - *The IR* (`ir/lowerlength.go`). A dummy's length is a `LengthCell`, `$Ln`,
    computed at each entry point that defines it, before its array's bounds
    (`computeBounds`), never less than 0; its text is still where its
    descriptor points. An automatic object's length is its descriptor's first
    word, stored by `computeAutomatic` before the bounds, its storage
    `StackAllocate`d (a scalar's at its class S descriptor's POINTER, no A0).
    `charLength` reads either (`runtimeLength`, `lengthWord`); a passed length's
    shortcuts (passing a dummy's own descriptor on) are `PassedLength` only, and
    an automatic scalar is passed by its own descriptor.
  - *Choices of ours, unconfirmed:* a length past 65,535 isn't checked (a
    descriptor's word holds it); the storage map and dumps show the length's
    expression (`CHARACTER*(LEN(C)+1)`, `lengthText`).
  - *Left for later:* the `/DEBUG` table still describes such a dummy as a
    passed-length one (its actual's length, not N) and leaves automatic
    scalars out with the automatic arrays (subtask 9).
  - *Tests:* `sema`'s `TestRuntimeLength`, the `f90autolen` goldens in `sema`,
    `ir`, and `codegen`, and `testdata/fortran90/autolen.f90` (every mode, paging
    both ways; gfortran prints the same 26 lines). MANUAL.md's 14.13 has
    "CHARACTER lengths that aren't constant", and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 4c done:** The scalarizer's temporaries of run-time size.
  - *sema.* The NOTIMPLs Phase 102 left for this phase are gone: an assignment
    through a temporary of any size (`ArrayOrder.TempShape`, -1 for an extent
    known only when the program runs, `tempShape`), a WHERE's mask kept the same
    way (`Where.MaskTemp`), a constructor of a run-time size wherever it stands
    (`constructorSizes` gone), and a vector subscript of the target's own array
    (`ArrayOrder.CopyVectors`, set before the Fill test, so `V(V) = (/ ... /)` is
    right too). `checkArrayParts` is gone: a CHARACTER function's value and a
    concatenation of a run-time length aren't `Inline`, so the scalarizer computes
    them once, before the loops; an elemental concatenation of a run-time length
    is built for each element in a dynamic string, freed by the next branch, as
    before. MERGE of CHARACTER values of a run-time length is allowed. A
    CHARACTER constructor whose first value's length isn't constant has
    `Constructor.LengthOf`, that value, if it is a variable's, an array's, or a
    concatenation of them (`lengthFromNames`; else NOTIMPL, ours), and later
    values of another length are made that length as by assignment (ours: the
    standard requires one length, which can't be checked when compiling; two
    constant lengths still draw CONSTRUCTLEN). `Dim.Runtime` and
    `NewRuntimeArray` are lowering's temporaries of run-time extents, kept as
    automatic arrays are (`AutomaticObject`).
  - *The IR* (`ir/lowertemps.go`). `runtimeTemp`: a class A descriptor cell
    (`$Rn`, pooled by form, `freeRuntime`) standing among the adjustable arrays'
    cells, so the scalarizer reads it as any adjustable array; its extents, the
    upper bounds, ARSIZE, the storage, and A0 stored by the statement. The
    storage is the stack's up to 64 KB (`tempStackLimit`, the author's answer 1),
    the heap's past it (FTNRTL's new `FTN$GET_TEMP`, vector 704, GSMATCH 1,23,
    INSVIRMEM as ALLOCATE's); SP is saved first by a new IR statement,
    `StackMark` (`MOVL SP,dst`), through check, dump, codegen, and every
    optimizer walk. `endTemps`, at the statement's end (from `release` when
    control is there, or `stmtCond` and `stmtExpr` before an IF's, a logical IF's,
    a DO WHILE's, an arithmetic IF's, a computed GO TO's, or a LOGICAL or INTEGER
    CASE's branch), frees the hoisted dynamic strings, gives a heap
    block to `FTN$DEALLOCATE`, and restores SP (`StackRelease`): a statement in a
    loop doesn't grow the stack. `shapeTemp` chooses a cell or a run-time
    temporary; `extentsOf` computes the extents a shape's leaves have;
    `countItems` counts a constructor's items, running its implied DOs without
    their values; `copyVectors` copies the target's vectors and rebuilds the
    section's shape; `hoistText` binds a CHARACTER value computed once, its
    run-time length in a cell (`lengths`), its dynamic string freed at
    `endTemps`. A temporary's CHARACTER length is the value's if known, else the
    target's (`tempType`, `elementLength`). MERGE of run-time lengths chooses an
    address and a length, with no copy (`mergeText`). A constructor's temporary
    is a run-time one when its extent or its elements' length isn't known.
  - *A fix found on the way:* a constructor of a size not known when compiling
    in a scalar context (`SUM((/ (I, I = 1, N) /))`) was built into a temporary
    of 0 elements, so its reduction was wrong (SUM gave 0) with no message.
  - *Choices of ours, unconfirmed:* the 64 KB limit is a temporary's own size,
    not the statement's sum; a temporary's size isn't checked for overflow past
    2**31 - 1 bytes (FTN$ALLOCATE's EMUL test isn't repeated); a statement left
    by an unwind leaves a heap temporary allocated.
  - *Tests:* `sema`'s `TestShapes`, `TestArrayConstants`, and `TestWhere` cases
    changed from NOTIMPL to their answers (a constructor of a substring's values
    is the new NOTIMPL); the `f90rtemp` goldens in `sema`, `ir`, and `codegen`;
    `testdata/fortran90/rtemps.f90` (every mode, paging both ways; gfortran
    prints the same 20 lines), with a temporary of 80,000 bytes and a statement
    through a temporary 6,001 times in one call. MANUAL.md's 14.13 and HELP
    FORTRAN describe temporaries of any size.
- **2026-11-18, subtask 5 split** into 5a (the routine's side, whole arrays and
  sections as actuals) and 5b (actuals through temporaries, OPTIONAL passed on,
  generics, BADARRDESC), each a commit (A.4).
- **2026-11-18, subtask 5a done:** Assumed-shape dummies.
  - *Declarations.* A dummy's deferred shape is an assumed one: `checkShapes`
    turns its colons into `Dim.AssumedShape` (`Symbol.AssumedShape`), whose lower
    bound is 1 or a specification expression read by `dimensionBounds` as an
    adjustable array's (`(lb:)`, `Dim.lowerGiven` until then). `(lb:)` on
    anything else is SHAPENOTDUMMY, on an allocatable array ALLOCSHAPE.
    `Adjustable` is false for an assumed-shape array; `UpperKnown`, `known`, and
    `ElementNumber` know its upper bounds aren't. An assumed-shape array of
    records is NOTIMPL (Phase 105).
  - *Actual arguments* (`sema/shaped.go`). `argsTo` analyzes a reference's
    arguments against its explicit interface (`interfaceOf`): an assumed-shape
    dummy's by `shapedArg`, a whole array or a section, marked `Arg.Shaped`;
    everything else as before, through `unshaped`, which refuses a whole
    assumed-shape array to any other dummy (NOTIMPL, subtask 6). A generic's
    chosen specific marks its whole arrays the same way (`resolveGeneric`).
    `argumentMismatch` makes the ranks equal for an assumed-shape dummy (an
    element is ARGRANK), `checkInterface` its CHARACTER length (ARGLENGTH, both
    known), and no longer judges an argument that already failed (it drew
    NOTDEFINABLE after a NOTIMPL). EXPLICITREQ names an assumed-shape dummy as
    it did an OPTIONAL one (`explicitDummy`; the message takes the kind),
    INTERFACEDIFF an array's kind (`shapeText`). `definable` takes a section
    without a vector subscript. A run-time format, an internal file, and
    ENCODE's or DECODE's buffer that is an assumed-shape array are NOTIMPL
    (`noContiguous`, subtask 6).
  - *The routine* (`ir/lowershaped.go`). Each assumed-shape dummy has a class
    NCA cell (`shapedCell`, `$Bn`, first among the adjustable arrays' so that
    others' bounds may ask about it), filled by each entry point whose list has
    the dummy (`computeShaped`): POINTER and the strides copied, the lower
    bounds the dummy's own, each upper bound the lower plus the actual's extent
    (never below 0) less 1, A0 and ARSIZE from those; an OPTIONAL one's copy is
    passed over when its argument is 0, and a passed length is the actual's.
    An element is A0 plus every subscript times its stride (`stridedElement`,
    the first subscript too); the whole array's base is POINTER (`symbolMem`);
    an output list takes it an element at a time (`ioItem`, through a shaped
    `NewArrayRef`); SIZEOF reads ARSIZE.
  - *The caller* (`shapedArg`, `sectionNCA`, `buildNCA`). An allocatable array's
    descriptor and an assumed-shape dummy's cell are passed as they are; any
    other whole array gets a descriptor cell of the call's with its own bounds
    and the strides its extents give (`baseStride`); a section one with bounds 1
    to each triplet's extent and the stride of the triplet's step, POINTER its
    first element, the subscripts computed once by the scalarizer's nest (and
    under `/CHECK=BOUNDS` each triplet's first and last checked, as an array
    assignment's are).
  - *The optimizer.* An element reached from an assumed-shape dummy's cell is
    the dummy's region (`opt/places.go`'s `shapedBase`): the caller's storage,
    which any call may change, as a dummy's.
  - *Left for later:* the `/DEBUG` table leaves assumed-shape dummies out
    (subtask 9). Found on the way, not this subtask's: an explicit-shape array
    of no elements (`Z(0)`, which Fortran 90 allows) is still BOUNDORDER, as in
    FORTRAN-77; MANUAL.md's "Not there yet" says so.
  - *Tests:* `sema`'s `TestAssumedShape` (every message and NOTIMPL); the
    `f90shaped` goldens in `sema`, `ir`, and `codegen`;
    `testdata/fortran90/shaped.f90` and `shapes.f90` (every mode, paging both
    ways; gfortran prints the same 18 and 16 lines). MANUAL.md's 14.13 has
    "Assumed-shape dummy arguments", and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 5b done:** Assumed-shape dummies given values.
  - *sema.* `arg` is now `argValue`, which keeps an array section or array
    expression (`Arg.expression`; an array in parentheses is one, with `Copy`),
    and `arrayChecked`, the old NOTIMPL for any other dummy. `shaped` (was
    `shapedArg`) takes every array value for an assumed-shape dummy, marking it
    `Shaped`; `Described` says whether it is described where its elements are (a
    whole array, a section without a vector subscript, or a substring of either,
    none in parentheses) or computed into a temporary. A CHARACTER array
    expression whose length is known only when the program runs must be made of
    names (`lengthFromNames`, as a constructor's first value), else NOTIMPL. A
    vector-subscripted section or a parenthesized array to INTENT(OUT) or
    INTENT(INOUT) is NOTDEFINABLE, as `definable` already said. A generic name's
    arguments are analyzed by `argValue`, and `shaped` or `arrayChecked` decided
    once the specific is chosen (`resolveGeneric`), so a section or expression can
    reach a generic's assumed-shape specific.
  - *The caller* (`ir/lowershaped.go`). `shapedArg` takes the `sema.Arg` and returns
    the argument: `valueTemp` computes any other value into a temporary of the
    statement's (4c's `shapeTemp`, a cell or a run-time temporary, given back at the
    statement's end; a CHARACTER one's length `valueLength`'s) and `wholeNCA`
    describes it, never copied back (12.4.1.1). A substring of a whole array or a
    section is an NCA of its own (`arrayNCA`, `sectionNCA` with a substring,
    `substringOf`: POINTER moved to the substring's first character, LENGTH the
    substring's, computed when not constant; `buildNCA` takes an `ncaElement`). A
    whole array that is an OPTIONAL dummy of the routine, assumed-shape or not, is
    passed by value as its descriptor's address or 0 (`optionalNCA`), so the dummy
    it reaches is absent too.
  - *The entry* (`computeShaped`). The caller's descriptor's class byte chooses:
    class A (a MACRO caller's, finding 9) has multipliers, each dimension's extent,
    so its strides are the element's size times the extents before it; anything
    else is read as NCA. Under `/CHECK=BOUNDS` `checkDescriptor` first requires class
    NCA or A and DIMCT the rank, else FTNRTL's new `FTN$BADARRDESC` (`ftn_control.mar`,
    vector 712, GSMATCH 1,24; FOR number 303, `vmsdef/formessages.go`). Our choice,
    unconfirmed: class A is accepted with or without `/CHECK=BOUNDS` (the survey put
    it under the check), since reading its multipliers as strides would be wrong
    with no message; it costs a compare and a branch at each entry. The IR's
    `ClassOffset` and `DimctOffset` name the fields.
  - *Tests:* `sema`'s `TestAssumedShape` (expressions, vector sections, parentheses,
    substrings, NOTDEFINABLE, the CHARACTER NOTIMPL, generics both ways); the
    `f90shapedx` goldens in `sema`, `ir`, and `codegen` (and `f90shaped`'s entry code
    changed); `testdata/fortran90/shapeargs.f90` (every mode, paging both ways;
    gfortran prints the same 21 lines); and `internal/console`'s
    `TestFortranShapedFromMACRO`, a MACRO program passing class A and NCA descriptors,
    then a class S one and an NCA of the wrong rank, each BADARRDESC. MANUAL.md's
    14.13, the `/CHECK` table, and Appendix B's FTNRTL routines, and HELP FORTRAN,
    describe them.
- **2026-11-18, subtask 6 done:** Sections and expressions to old dummies.
  - *sema* (`sema/sequence.go`). `argsTo` gives each argument of a dummy that
    isn't assumed-shape, or of a routine with no interface, to `sequenced`
    (was `unshaped` and `arrayChecked`'s NOTIMPL): an array section or array
    expression, or a whole assumed-shape array, is `Arg.Sequence`, and
    `Arg.CopyBack` when it is `Described` (a whole array, a section without a
    vector subscript, or a substring of either, none in parentheses) and the
    dummy isn't known INTENT(IN). A generic's chosen specific does the same.
    `Contiguous` is the compiling-time rule: the base isn't assumed-shape, and
    each dimension is whole by a step of 1 up to one taken by a step of 1 or a
    single subscript, every later one a single subscript. The survey's finding
    11 gave `A(2:5, :)` as contiguous, which it isn't (A(10, 10)'s rows 2 to 5
    of each column); the rule is right, the example was wrong. A record array's
    section is NOTIMPL (Phase 105), and so is a CHARACTER expression of a
    run-time length not made of names, as for an assumed-shape dummy. `arg`
    (the system subroutines' and the built-in functions' arguments) keeps
    NOTIMPL for sections and expressions, and now for a whole assumed-shape
    array too, which `systemBuffer` (DATE, TIME) and `%LOC` had passed as its
    POINTER without a check. The sema dump shows `(sequence X)` and
    `(sequence back X)`.
  - *The IR* (`ir/lowersequence.go`). `sequenceArg` returns the argument and
    what copies it back; `args` returns the copy backs with the list, and each
    caller (`call`, `callFunction`, `callIntrinsic`) runs them right after its
    `Call` (`copyBack`), so a function reference's are done before the rest of
    its statement, and a CALL with alternate returns keeps R0 in a temporary
    while they run. `sequence` chooses: a contiguous section is its first
    element's address (`contiguousSection`, over `describeSection`, the part of
    `sectionNCA` that finds the first element and each triplet), or for
    CHARACTER a class A descriptor (`buildClassA`); a whole assumed-shape array
    or a section of one is `testedSequence`, the descriptor's strides tested when
    the program runs (`contiguityTest`), passed where it is or through a
    temporary, a flag saying which so that only a copy is copied back, its
    run-time temporary's size made 0 on the other way so that `endTemps` frees
    nothing, and SP marked before the test (the author's answer 4); anything else
    is `valueTemp`'s temporary, copied back by `arrayNest` with the actual as the
    target. A CHARACTER one's class A descriptor from an NCA is `ncaClassA`.
  - *I/O.* An assumed-shape array as a run-time format, an internal file, or
    ENCODE's or DECODE's buffer goes the same way (`ioUnit`, `ioFormat`), copied
    back after a WRITE or ENCODE, between FTN$IO_END and ERR=/END= (`ioFinish`'s
    `backs`); the three NOTIMPLs (`noContiguous`) are gone. The control block is
    still made first, so `TestNoF90Unchanged`'s baseline holds.
  - *A fix found on the way:* a temporary's loops end blocks, and a block's end
    did the statement's `after` list, freeing a dynamic string an earlier
    argument of the same call had made: `CALL H(S // 'X', A * 2.0)` with S
    passed-length gave H freed text (subtask 5b's assumed-shape path had it too).
    `holdingAfter` holds the list aside while a copy, in or back, is lowered,
    for both paths.
  - *Choices of ours, unconfirmed:* the copy is made for INTENT(OUT) too (the
    standard allows either); the contiguous case still computes each extent,
    which only the descriptor forms use; a section's extent of one element in a
    later dimension isn't seen as contiguous (`A(2:5, 1:1)` is copied).
  - *Left for later:* `%REF`, `%DESCR`, and `%LOC` of a section, an expression,
    or an assumed-shape array, and the same given to a system subroutine, are
    NOTIMPL; a section as an internal file is Phase 109's.
  - *Tests:* `sema`'s `TestShapes`, `TestAssumedShape`, and `TestErrors` cases
    changed from NOTIMPL to compiled (and the system subroutine's NOTIMPL); the
    `f90seq` goldens in `sema`, `ir`, and `codegen`;
    `testdata/fortran90/seqargs.f90` (every mode, paging both ways; gfortran
    prints the same 26 lines) and `seqvms.f90` (ENCODE and DECODE on an
    assumed-shape buffer, copied or not; LIB$STAT_VM's bytes unchanged by a
    thousand heap temporaries). MANUAL.md's 14.13 has "Sections and expressions
    to other dummies", and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 7 split** into 7a (a value's shape known when
  compiling), 7b (known only at run time), and 7c (CHARACTER values of a
  run-time length, and ENTRY), each a commit (A.4).
- **2026-11-18, subtask 7a done:** Array-valued functions of a shape known when
  compiling.
  - *sema* (`sema/arrayvalue.go`). `dimension` lets a function's value (its name
    without RESULT, or its RESULT variable) have dimensions, under `/F90` and
    `/NOF90` alike (VAX FORTRAN gave it no meaning, BADUSE); the function's name
    with RESULT still can't (and BADUSE now calls it "the name of this function",
    not "of this subroutine", a fix). `checkArrayValue` (from `checkArrays`)
    refuses a value of the length (*) (STARARRAY, 5.1.1.5's constraint) and makes
    a value whose bounds aren't constant, one of records, and ENTRY in such a
    function NOTIMPL, taking a broken value out of the adjustable arrays.
    `Symbol.HiddenValue` (a value too wide, CHARACTER, or an array) replaces
    `Type.HiddenResult` where a unit's value is placed (`storage.go`). Every
    reference to a function is made by `funcRef`, which gives it the shape of its
    explicit interface's value (`resultShape`: an interface body, an internal
    function, a recursive function's own, a generic's specific, a dummy
    procedure's interface body), so the array expression machinery takes it as it
    is: `Scalarize` and `Leaves` treat an array-valued FuncRef as a leaf, a scalar
    context is NOTSCALAR, a shape that differs NOTCONFORM, and an argument is a
    `Sequence` or `Shaped` value through its temporary. The overlap test needs
    nothing: the value is computed before the loops, and `baseSymbol` has no
    storage for it. A reference of the file's to an array-valued function with no
    interface is EXPLICITVALUE (beside EXPLICITREQ); an interface body whose value
    differs in rank or extent is INTERFACEDIFF (`resultDifference`). The sema dump
    shows the shape after the arguments, `F(X)(3)`.
  - *The IR.* The unit's value is a `Param` at argument 1 when `HiddenValue`, an
    explicit-shape dummy array in all but name, so its elements and whole-array
    assignments need nothing new. A leaf that is an array-valued FuncRef is
    computed in `prepare`, before the loops, into an `arrayTemp` of its shape by
    `callArrayFunction` (the temporary's address, or for CHARACTER a class A
    descriptor of it by `charArg`, prepended to the arguments; `callWith` is the
    rest of `callFunction`, shared), then read as a whole array.
  - *A fix found on the way* (by `opt`'s `TestPassesOnCorpus`, a second run of the
    passes changing the golden's internal function TAGS): in an internal procedure
    `Storage.own` answered from a map made when the Storage was, so a temporary
    the common subexpression pass had just made (`$U1`) was taken for the host's,
    which any call may change, and a value over it (`$U1 + 1`) ended at every
    call until the next run. The map is made again when the routine has gained
    cells (`opt/effects.go`).
  - *Choices of ours, unconfirmed:* a value passed to a dummy that isn't
    assumed-shape is copied from its temporary into the argument's (two copies; the
    second could be left out); `SIZE` of a reference is folded without calling the
    function (7.1.7.1 allows it).
  - *Tests:* `sema`'s `TestArrayValue` (every message and NOTIMPL) and
    `TestErrors`' changed case; the `f90value` goldens in `sema`, `ir`, and
    `codegen`; `testdata/fortran90/values.f90` (every mode, paging both ways;
    gfortran prints the same 19 lines). MANUAL.md's 14.13 has "Array-valued
    functions", and HELP FORTRAN a paragraph.
- **2026-11-18, subtask 7b done:** Array-valued functions whose value's shape is
  known only when the program runs.
  - *sema* (`sema/arrayvalue.go`). A value whose bounds aren't constant is no
    longer NOTIMPL: `bound` already made it an automatic object, its bounds
    specification expressions (the dummies, SIZE of an assumed-shape one, COMMON,
    the host's variables), and that is how lowering knows the function makes its
    storage. `valueShape` leaves such an extent unknown, and `funcRef` makes each
    reference its `Extent.Of`, so the scalarizer reads it from the reference once
    called. A value whose CHARACTER length isn't constant keeps the NOTIMPL it had
    (7c), as does ENTRY.
  - *The function* (`ir/lowervalue.go`). The hidden first argument is `$VALUE`
    (the value's own name being its cell's), the caller's class NCA descriptor;
    the value is an automatic array of its own class A cell (`valueCell`), which
    every use in the function reads, computed at entry by `computeAutomatic` with
    its storage from FTNRTL's `FTN$GET_TEMP` rather than the stack, then
    `publishValue` writes the caller's descriptor whole: the prototype block,
    POINTER, ARSIZE, A0, each stride the element's size times the extents before
    it, and the bounds, U written as L plus the extent less 1 so that the caller's
    U - L + 1 is never below 0. `FTN$GET_TEMP` now takes at least `MIN_BLOCK` (8)
    bytes, the size `FTN$DEALLOCATE` gives back, so a value of no elements has an
    address (no new entry, GSMATCH unchanged).
  - *The caller.* Each reference is a `$Gn` cell, a class NCA descriptor read as an
    allocatable array's (`sema.NewValueArray`; `valueResult`, from `prepareIn` and
    `leafExtent`), pooled apart from other descriptors (`freeValues`), its POINTER 0
    whenever no statement holds a value there: the static cell's initial value, a
    frame cell's cleared at the main entry point (`fillEntryValues`, put in after
    the body is lowered, since the cells are made then) and at ENTRY's with the
    allocatable arrays. A statement calls each reference once (`runtimeTemps.values`,
    by reference), and gives the blocks back at its end (`endTemps`, `freeValue`:
    FTN$DEALLOCATE, which clears POINTER); a call finding a block still there gives
    it back first, which covers an implied DO list calling the function at each
    pass, an unwind, and a statement that ends without `endTemps`. Passed to an
    assumed-shape dummy the value goes by its own descriptor (`shapedArg`), to any
    other dummy by its POINTER (`sequenceOf`), with no copy.
  - *A fix found on the way:* a CALL with alternate returns ended its block with its
    `Switch`, so `release` dropped the statement's run-time temporaries without
    giving them back: a heap temporary of 4c's leaked there, as a value's block did.
    `call` now keeps R0 and calls `endTemps` before the `Switch`.
  - *Found too:* 7a's `values.f90` was in neither `fortran90PagingPrograms` nor the
    corpus README; both now have it. And subtask 6's `seqvms.f90` ran 690 million
    instructions (1,000 copies of a 30,000-element section through the heap),
    25 to 43 seconds in each of the corpus's eight modes, which had taken the
    console suite from about 3.5 minutes to 8.5; twenty copies show the heap given
    back as well, 14 million instructions.
  - *Choices of ours, unconfirmed:* the function writes the caller's descriptor
    whole, trusting none of it; a statement left by an unwind leaves its blocks
    allocated until the next statement to use the cell (the program goes on, as
    with a heap temporary); SIZE of a reference calls the function.
  - *Tests:* `sema`'s `TestArrayValue` (run-time bounds, SIZE of a dummy in them, an
    interface body's, NOTCONFORM and NOTSCALAR of such a value, recursion); the
    `f90valuex` goldens in `sema`, `ir`, and `codegen`;
    `testdata/fortran90/ramps.f90` (every mode, paging both ways; gfortran prints
    the same 18 lines) and `rampvms.f90` (LIB$STAT_VM's bytes unchanged by thousands
    of values in every statement form, counted after the loop's first pass, which
    makes the heap's zones and the internal WRITE's kept format); and
    `internal/console`'s `TestFortranValueFromMACRO`, a MACRO program passing its own
    descriptor, reading what the function filled in, and giving the block back with
    LIB$FREE_VM. MANUAL.md's 14.13 "Array-valued functions" and HELP FORTRAN describe
    them.
- **2026-11-18, subtask 7c done:** CHARACTER values of a run-time length, and
  ENTRY in array-valued functions.
  - *sema.* A function's own value may have a CHARACTER length that is a
    specification expression (`runtimeLength` no longer NOTIMPLs `Result`; the
    function's name when RESULT names the value still is): an automatic object,
    as an array of run-time bounds is. A reference's `FuncRef.Made` (`madeValue`)
    says the function makes its value, from its explicit interface's value being an
    automatic object, and lowering goes by it rather than 7b's unknown extents. The
    caller's own symbol for such a function, length 0, isn't a passed length
    (`madeLength`, in `passedLengths`). LEN of a function reference whose value is an
    array is no longer NOTIMPL. `checkEntryValues` (`arrayvalue.go`): when the value
    or an entry point's is an array or made, every entry point's must be declared
    alike, rank, extents, bounds as written, and length (new **ENTRYVALUE**), and
    each entry point of a made value must have every dummy its bounds or length use
    (new **ENTRYMAKE**); 7a's ENTRY NOTIMPL is gone.
  - *The function* (`ir/lowervalue.go`). A scalar made value is an automatic
    CHARACTER object of its own class S cell, its storage a dynamic string
    `madeString` gets with LIB$SGET1_DD in the caller's class D descriptor, the
    hidden argument, then copies the address from; an array's element length is
    written to the caller's NCA descriptor's first word and its first stride is
    the length (`publishValue`). With ENTRY, every entry point's name is the value's
    one cell, only the function's own computed at each entry point (`prepareBounds`),
    and `hiddenParam` is the entry lists' first argument.
  - *The caller.* A scalar is `madeText`: a class D descriptor cell of the
    statement's, passed, read, and freed with LIB$SFREE1_DD at the statement's end
    (`after`), as a concatenation's; LIB$SGET1_DD replaces a string the descriptor
    already holds, so an implied DO calling the function at each pass leaves one. An
    array is 7b's `$Gn` cell, `sema.NewValueArray` marking its length as the
    descriptor's. `charLen` (LEN) calls the function; `ncaAsClassA` copies a run-time
    element length.
  - *Choices of ours, unconfirmed:* ENTRYVALUE compares bounds and lengths as
    written (`lengthText`), so `N+1` and `1+N` differ; LEN of a reference calls the
    function.
  - *Not checked by a probe:* LIB$STAT_VM's count leaves dynamic strings out (their
    own zone), so `rampvms.f90` checks the arrays' blocks; with LIB$SFREE1_DD taken
    out it still printed T, the strings being replaced in place. The goldens show
    each LIB$SFREE1_DD.
  - *Tests:* `sema`'s `TestArrayValue` (ENTRY of each kind, ENTRYVALUE by rank,
    extent, bounds, and length, ENTRYMAKE, a made length through an interface) and
    `TestRuntimeLength`'s changed case; the `f90valuec` goldens in `sema`, `ir`, and
    `codegen`; `testdata/fortran90/madetext.f90` (every mode, paging both ways;
    gfortran prints the same 16 lines) and `rampvms.f90`'s CHARACTER cases; and
    `TestFortranValueFromMACRO`'s dynamic string, written with LIB$PUT_OUTPUT and
    freed with LIB$SFREE1_DD. MANUAL.md's 14.13 and HELP FORTRAN describe them.
- **2026-11-18, subtask 8 done:** The transformational intrinsics.
  - *sema* (`sema/transform.go`). SPREAD, CSHIFT, EOSHIFT, PACK, and UNPACK join
    `fortran90Table` (REDUCE rows with their dummies' names, so keywords work), and
    RESHAPE of values not constant goes to `transform` too; each is an `ArrayFunc`
    (gaining `Pad`, EOSHIFT's BOUNDARY or RESHAPE's PAD, and `Order`), and
    `Transformed` says lowering computes it whole. Their checks: DIMRANGE, ARRAYARG,
    ARRAYRANK, BADARG, NOTCONFORM for SHIFT and BOUNDARY (scalars, or arrays of the
    rank below ARRAY's), MASK, VECTOR, FIELD, and BADRESHAPE for a SHAPE whose size
    isn't known when compiling (the standard's constraint) or an ORDER that isn't a
    permutation. A reduction's DIM= that isn't constant on an array of rank above 1
    is `DimExpr` with a shape of unknown extents, no longer NOTIMPL. An extent known
    only at run time is the function's own (`Extent.Of` is the ArrayFunc). A fix on
    the way: a shape taken from an argument must be copied (`slices.Clone`), or
    marking the function's own extents rewrites the argument's.
  - *The IR* (`ir/lowertransform.go`). `transformed` computes the value into a
    temporary of its shape (`extentsTempOf`: a cell, or a run-time temporary), once
    per statement (`runtimeTemps.funcs`, cleared at each statement's end): its extents
    first (`transformExtents`; PACK's by COUNT of the mask, RESHAPE's from SHAPE's
    values, a run-time DIM='s chosen among ARRAY's), then its elements by loops of
    its own that walk the argument in array element order and put each where it goes
    (`walk`, over `arrayNest`, a line argument's dimensions mapped past DIM's):
    SPREAD at every copy; CSHIFT at (j - 1 - SHIFT) mod n, from 1; EOSHIFT the
    boundary everywhere, then each element whose place is inside; PACK VECTOR, then
    each element where MASK holds; UNPACK VECTOR's next element or FIELD's; RESHAPE
    from SOURCE and PAD copied to rank one (`flatten`), in ORDER's loop order; and a
    reduction by DIM's value, an arm per dimension, each Phase 102's constant DIM=
    (`sema.WithDim`), out of range FOR's NOTCONFOR under `/CHECK=BOUNDS` and an empty
    result otherwise.
  - *A departure from finding 13, by our choice:* the finding had SPREAD, CSHIFT,
    and EOSHIFT computed where they are used, as TRANSPOSE is. An element there would
    be the argument's at a subscript computed from the loops' (a MOD; outside the
    array for EOSHIFT's boundary), which the scalarizer's leaves don't take, and the
    argument may be any array expression; so every one is computed into a temporary,
    which is safe whatever the statement's target overlaps, at the cost of a copy.
    Phase 110 may make the simple ones elements again.
  - *Choices of ours, unconfirmed:* the DIM= of SPREAD, CSHIFT, and EOSHIFT, and
    RESHAPE's ORDER=, must be constants (NOTIMPL otherwise); a CHARACTER argument of a
    run-time length must be a whole array or a section; RESHAPE with too few elements
    and no PAD leaves the rest as it was (the standard calls the program wrong); MERGE
    of CHARACTER values of a run-time length needed nothing new (4b's).
  - *Tests:* `sema`'s `TestTransform` (each function, each message), and
    `TestArrayFunctions`' two cases now compiled; the `f90trans` goldens in `sema`,
    `ir`, and `codegen`; `testdata/fortran90/transform.f90` (the standard's examples
    and more, every mode, paging both ways; gfortran prints the same 32 lines).
    MANUAL.md's 14.13 and HELP FORTRAN describe them.
- **2026-11-18, subtask 9 done:** Programs end to end, and the debugger.
  - *Programs* (`testdata/fortran90`): `solver.f90` (Gaussian elimination with
    partial pivoting on assumed-shape dummies, automatic work arrays, allocatable
    systems of sizes 2 to 5, MAXLOC, MATMUL, SUM of sections, a singular system, a
    thousand solves), `sieve.f90` (an allocatable LOGICAL array of bounds 2:N,
    strided sections, PACK of a constructor into an allocatable array of COUNT's
    size, ALLOCATED, STAT=, a function's allocatable local freed at each return),
    and `sorter.f90` (a quicksort as a recursive function whose value's shape is
    SIZE of its dummy, made of a constructor of its own values over PACK's choices;
    a merge sort with an automatic work array over sections of its own array). Each
    was right on its first run, in every mode, paging both ways, and gfortran prints
    the same lines (10, 15, and 8).
  - *What they cost* (finding 15's measurement, left to Phase 110): instructions
    /NOOPTIMIZE to /OPTIMIZE, solver 1,786,346 to 1,128,807 (-36.8%), sieve 431,672
    to 204,251 (-52.7%), sorter 622,166 to 397,673 (-36.1%).
  - *The debugger* (finding 14). The `/DEBUG` table gains two place kinds,
    `dbgtable.Described` (`D:symbol`) and `DescribedFrame` (`G:n`): a variable whose
    own storage is a descriptor, its bounds read from the same one
    (`codegen/debugtable.go`'s `describedVar`): an allocatable array's class NCA
    cell, an automatic array's class A one, an automatic CHARACTER scalar's class S
    one, a function's value made at entry, and an assumed-shape dummy's class NCA
    copy (`$Bn`), none of which the table had before. The debugger
    (`debugger/fortran.go`'s `describedBase`, `describedStrides`) takes the first
    element's address from POINTER and a CHARACTER length from the first word; a
    class NCA descriptor whose strides aren't the contiguous ones (an assumed-shape
    dummy given a section) gives each element's address as A0 plus each subscript
    times its stride, in EXAMINE of an element and of the whole array. A POINTER of
    0 is govax's new **DBG-W-NOTALLOC** ("!S is not allocated").
  - *Tests:* `internal/debugger`'s `TestFortranDescribedArrays` (a section through an
    assumed-shape dummy, whole and by element; automatic INTEGER and CHARACTER; an
    allocatable array of bounds 0:1, 2; NOTALLOC; subscripts out of bounds). MANUAL.md's
    14.10 describes them.
- **2026-11-18, subtask 10 done:** The close-out. FORTRAN-INTERNALS.md's section 29,
  "Fortran 90: descriptors and dynamic storage": the class NCA descriptor of
  `A(2:10:2, J)` beside the array's memory, from the compiler's own MACRO, and an
  element of it in the callee; why an assumed-shape dummy needs an interface; a
  section to an old dummy; the stack and the heap; allocatable arrays; automatic
  objects; temporaries of run-time size; the three forms of an array-valued
  function's value; the transformational functions; the debugger; and what the
  programs found. MANUAL.md's 14.13 (its introduction, the run-time length and the
  debugger no longer "not there yet") and FTNRTL's table (`FTN$GET_TEMP`), and HELP
  FORTRAN (a stale "section passed to a program's own procedure is not there yet",
  and the debugger's part). PLAN.md's summary and CLAUDE.md.

## Where things stand

Phase 104 is done. What it left, and where each goes:

- **Phase 105:** arrays of records: an allocatable or automatic array of them, a
  section of one as an argument, and an array-valued function whose value is records
  (each NOTIMPL). *(Resolved in Phase 105.)*
- **Phase 106:** an allocatable array in a module. *(Resolved in Phase 106.)*
- **Phase 107:** POINTER arrays, on this phase's class NCA descriptors and
  `FTN$ALLOCATE`, and TARGET. *(Resolved in Phase 107.)*
- **Phase 108:** the intrinsic functions this phase's transformational ones don't
  cover, and keyword arguments for those whose table rows lack their dummies' names. *(Resolved in Phase 108.)*
- **Phase 109:** a whole array or section in an I/O list by its descriptor rather than
  an element at a time; a section as an internal file or ENCODE's buffer. *(Resolved in Phases 104 and 109.)*
- **Phase 110:** what the programs measured (subtask 9's log): the second stride
  multiply for an assumed-shape dummy's first dimension, the copies for sections to
  old dummies (a copy is made for INTENT(OUT) too), `A = F(X)` into A directly when
  they can't overlap, a value passed to an old dummy copied from its temporary, and
  SPREAD, CSHIFT, and EOSHIFT computed where they're used instead of into a temporary;
  `/STANDARD` for the phase's forms; the debugger's names through hosts. *(The stride, `/STANDARD`, and the debugger resolved in Phase 110; the rest measured and left, Phase 113's A.5.3.)*
- **Not planned, ours:** the DIM= of SPREAD, CSHIFT, and EOSHIFT and RESHAPE's ORDER=
  must be constants; a CHARACTER argument to them, or to an assumed-shape or old
  dummy, of a run-time length must be made of names (`lengthFromNames`); `%REF`,
  `%DESCR`, and `%LOC` of a section, an expression, or an assumed-shape array; a
  constructor of a substring's values.
- **Unconfirmed choices of ours:** a statement with STAT= stops at its first failing
  object; a zero-sized array takes an 8-byte block; ALREADYALLOC (301), NOTALLOC
  (302), BADARRDESC (303), and DBG-W-NOTALLOC are govax's own; a class A descriptor is
  accepted by an assumed-shape dummy with or without `/CHECK=BOUNDS`; an unwind leaves
  heap blocks allocated (an allocatable local's until its next entry clears it, a
  function's value until its cell's next use); ENTRYVALUE compares bounds and lengths
  as written; RESHAPE with too few elements and no PAD leaves the rest as it was;
  SIZE of an array-valued reference of a known shape is folded without calling the
  function, and of a made one calls it.

