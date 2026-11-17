# Phase 102 — Fortran 90: array expressions

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 101.

## Goal

Arrays as values, where every shape is known at compile time (explicit-shape
arrays with constant bounds, and adjustable arrays, whose extents are known at
entry):

- **whole arrays** in expressions and assignment: `A = B + 2.0 * C`, `A = 0`,
  `L = A > B`; conformance checked (the standard's 7.1.5);
- **sections**: subscript triplets (`A(1:N:2)`, `A(:, J)`, `A(N:1:-1)`),
  vector subscripts (`A(IV)`), and sections of character arrays with
  substrings;
- the **scalarizer** (`ir/lowerarray.go`; Decision 6): loop nests in array
  element order, fused across an assignment's elemental operations, a temporary
  where the sides may overlap (`sema/overlap.go`);
- **array constructors** (`(/ 1, 2, (I, I = 3, 10) /)`), array-valued
  PARAMETERs, and array constants in DATA and initialization;
- **WHERE**, ELSEWHERE, and END WHERE, and the one-statement WHERE;
- **elemental intrinsics** on arrays (`SQRT(A)`, `MAX(A, 0.0)`, `ABS(A - B)`);
- the **reductions and inquiries**, with DIM= and MASK=: SUM, PRODUCT, MAXVAL,
  MINVAL, MAXLOC, MINLOC, ANY, ALL, COUNT, DOT_PRODUCT; SIZE, SHAPE, LBOUND,
  UBOUND; MATMUL and TRANSPOSE on shapes known at compile time;
- **sections in I/O lists**, an element at a time in array element order (a
  descriptor per item waits for Phase 109).

## What earlier phases leave in place

- `ir.CountedLoop`, the implied-DO lowering (`countedLoop`), and the optimizer's
  loop passes (invariants, induction, `AOBLEQ`/`SOBGTR`, register allocation).
- Class A descriptors for whole arrays passed to dummies; `ir.Mem`'s indexed
  mode for elements.
- `ElementNumber` (`sema/arrays.go`) and the bounds machinery.

## Design notes

- **Shape in sema.** Every expression gets a rank and a shape (Decision 5); a
  scalar is rank 0 and conforms with anything. An elemental operation's shape is
  its array operands'. A section's extent is `MAX((u - l + s) / s, 0)`, the same
  formula as a DO's trip count.
- **Teaching the scalarizer.** FORTRAN-INTERNALS.md's section shows one
  statement, `A(2:N) = A(1:N-1) + B(2:N)`, through each step: shapes, the
  overlap test (why `A(1:N-1)` read while `A(2:N)` is written needs a
  temporary, and `A(1:N) = A(1:N) + 1` doesn't), the loop nest, and the MACRO.
- **Temporaries** are in the frame (Decision 8): their size is known here, so
  they are fixed frame cells, and the run-time-sized case waits for Phase 104.
- **MAXLOC/MINLOC** return an array of rank-1 indexes; the first maximum wins
  (the standard's 13.13.64).
- **I/O of a section** expands to an item call per element; a whole contiguous
  array keeps its single class A item.

## Open questions

1. The overlap test's strength: the plan proves disjointness for sections of
   the same array with constant triplets and treats anything else as overlap. Is
   a run-time test (compare the address ranges, choose a loop direction) worth
   it later? Proposed: Phase 110, if measurements say so.
2. MATMUL in line for constant shapes only, or an FTNRTL routine from the start?
   Proposed: in line here (a triple loop), FTNRTL's for run-time shapes in 104.

## Subtasks (provisional)

1. **Survey** (the standard's 6.2, 7.1.4, 7.5.1.5, 7.5.3, 13.10.13 to 13.10.19;
   the Handbook's chapters 6 and 7), for review.
2. **Sections and shapes in sema** (rank, shape, conformance messages), with
   whole arrays still NOTIMPL in expressions.
3. **The scalarizer**: elemental expressions and assignment, scalars broadcast,
   fusion; IR golden dumps; both optimizer settings.
4. **Overlap and temporaries.**
5. **Vector subscripts**, on either side (a vector-subscripted section with a
   repeated index may not be assigned to: checked when constant).
6. **Array constructors**, array PARAMETERs, array initialization.
7. **WHERE** in both forms.
8. **Elemental intrinsics on arrays** (the intrinsic table's class column,
   Decision 14).
9. **Reductions and inquiries** with DIM= and MASK=, MATMUL and TRANSPOSE in
   line.
10. **Sections in I/O lists.**
11. **Programs end to end** (a matrix and statistics program, image smoothing
    with WHERE), gfortran's witness, every mode.
12. **Close-out**: FORTRAN-INTERNALS.md's section 27 (arrays as values and the
    scalarizer), MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
