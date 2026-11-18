# Phase 102 — Fortran 90: array expressions

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A, below, with a revised subtask list and
questions for the author), was accepted by the author 2026-11-17 ("The
author's answers"); subtasks 2 to 11 are done (2026-11-17), and subtask 12 is next.
Needs Phase 101 (done).

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
  (the standard's 13.13.65).
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

Replaced by Part A's A.4.

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

# Part A — the survey (subtask 1)

Read for it: the standard's 4.5 (array constructors), 6.2.2 (array elements
and sections, Table 6.1, triplets, vector subscripts), 7.1.4 and 7.1.5 (type
and shape of an expression, conformance), 7.1.6.1 (constant and
initialization expressions), 7.5.1.5 (intrinsic assignment), 7.5.3 (WHERE),
13.10.13 to 13.10.19 and the matching entries of 13.13; the Handbook's
chapters 6 and 7; and what the lexer, parser, sema, the IR, the optimizer,
and FTNRTL do today.

## A.1 What the standard asks

- **Sections** (6.2.2, R615 to R621). A subscript list may hold, in any
  dimension, a subscript, a **triplet** `[l]:[u][:s]`, or a **vector
  subscript** (an INTEGER array expression of rank one). The section's rank
  is the number of triplets and vector subscripts; its extent in each is the
  number of values its sequence has, `MAX((u - l + s) / s, 0)` for a
  triplet (Table 6.1's note: a size is never negative). A left-out `l` or
  `u` is the dimension's bound; `s` is 1 if left out, never 0, and may be
  negative (`B(9:1:-2)` is B(9), B(7), ..., B(1)). Only the subscripts
  actually used need be in bounds (`B(3:11:7)` of `B(10)` is fine). An
  assumed-size array's last dimension needs `u`. A section of a CHARACTER
  array may end in a substring range, `B(:, :, :)(2:3)`. An element's order
  in a section is its parent's (array element order, 6.2.2.3: the leftmost
  subscript varies fastest).
- **Vector subscripts** (6.2.2.4.2). `Z(3, V)` is the elements Z(3, V(1)),
  Z(3, V(2)), ... A **many-one** section (a vector with two equal values)
  must not be assigned to or read into; one with a vector subscript must not
  be an internal file, nor associated with a dummy that is defined.
- **Shape and conformance** (7.1.4.2, 7.1.5). An operation's shape is its
  array operand's; two arrays must have the same shape; a scalar conforms
  with anything, as if it were an array of the other's shape. A relational
  operation on arrays gives a default LOGICAL array.
- **Intrinsic assignment** (7.5.1.5). An array target takes the right side
  element by element, a scalar right side broadcast; the effect must be as
  if every operation on the right, and every expression in the target's
  subscripts, were evaluated **before any element is defined**. The order in
  which elements are assigned is the processor's. A target of size zero is
  assigned nothing.
- **Array constructors** (4.5, R431 to R435). `(/ v, v, ... /)`, each value
  a scalar, an array (its elements in array element order), or an implied
  DO `(values, i = e1, e2 [, e3])`, nested freely, an inner loop's variable
  not an outer's. The result is rank one. **Every value has the same type
  and type parameters**: CHARACTER values must all have one length (Fortran
  2003 relaxed this with a type-spec; Fortran 90 didn't). Empty is possible
  only through an implied DO of no trips. A constructor of constant
  expressions is a constant expression.
- **Constant and initialization expressions** (7.1.6.1). Beside scalars,
  they may hold a constant's subobjects (an element or section of an array
  PARAMETER with constant subscripts), constructors of constants, elemental
  intrinsics of constants, RESHAPE (and REPEAT, TRIM, TRANSFER, the
  SELECTED\_ functions), and the array inquiries (SIZE, LBOUND, UBOUND,
  SHAPE) of an array whose bounds are constant. Without RESHAPE no array
  PARAMETER of rank 2 or more can be written.
- **WHERE** (7.5.3). `WHERE (mask) assignment`, or a construct of WHERE,
  assignments, an optional ELSEWHERE and its assignments, and END WHERE:
  **only intrinsic assignments inside** (a nested WHERE and a masked
  ELSEWHERE are Fortran 95's). Each assignment's target and the mask have
  one shape. The mask is evaluated once, at the WHERE, and kept: later
  changes to what it reads don't change it. An elemental operation in an
  assignment is done only where the mask is true (false after ELSEWHERE);
  a nonelemental function reference (`SUM(LOG(A))`) is evaluated whole,
  unmasked. Only the WHERE statement may be a branch target.
- **The array intrinsics** (13.10.13 to 13.10.19). Reductions SUM, PRODUCT,
  MAXVAL, MINVAL with optional DIM= and MASK=; ALL, ANY, COUNT with DIM=;
  DOT_PRODUCT; MATMUL; the inquiries SIZE, LBOUND, UBOUND (with DIM=) and
  SHAPE; TRANSPOSE; MAXLOC and MINLOC, which in Fortran 90 take **MASK= but
  no DIM=** (DIM= is Fortran 95's), and whose result counts each subscript
  **from 1 to the extent**, not from the array's lower bound (13.13.65:
  "the ith subscript returned lies in the range 1 to e_i"), the first
  maximum in array element order winning. Their answers for size zero: SUM
  0, PRODUCT 1, MAXVAL the most negative number, MINVAL the most positive,
  ALL true, ANY false, MAXLOC and MINLOC processor dependent. LBOUND of a
  dimension of extent zero, and of any array expression that isn't a whole
  array, is 1. Every Fortran 77 intrinsic function is elemental, as is MERGE.

## A.2 What govax has

- **The lexer** has the tokens `(/` and `/)`, and classifies WHERE
  (`Where`, `WhereBlock`), ELSEWHERE, and END WHERE (Phase 99). `A(1:N) = 0`
  is already an assignment.
- **The parser** reads `A(1:N)` after a name as a **substring**
  (`referenceAfter`'s `substringAhead`: a colon at depth 1), whatever A is;
  `A(:, J)` and `A(1:N:2)` are syntax errors, and `primary` has no case for
  `(/`. WHERE, ELSEWHERE, and END WHERE are `PendingStmt`s. The implied-DO
  node, `ast.ImpliedDo`, serves I/O lists and DATA.
- **sema.** `value` refuses a whole array where a value is used
  (NEEDSUBSCRIPT); `name(e, wholeOK)` returns an `ArrayRef` only for an
  actual argument or an I/O item, and an assignment to one is BADTARGET. An
  expression has a type and a position (`exprBase`), no shape. `sema.Type`
  is `{Kind, Size}`, compared with `==` throughout and copied into the IR.
  `Dim` has constant bounds, or `LowerExpr`/`UpperExpr` (adjustable), or
  `Assumed`. The constant evaluator (`constant.go`) gives scalar `Const`s
  only. A PARAMETER is a scalar (Phase 100's NOTIMPL for an array one);
  an `= (/ ... /)` initial value is NOTIMPL. The intrinsic table
  (`intrinsics.go`) has Decision 14's class and dummy-name columns, used by
  Fortran 90's rows; Table D-3's rows have none. SIZE, LBOUND, UBOUND,
  SHAPE, and the reductions aren't in it. The WHERE statements are NOTIMPL
  once per construct (`pendingOpeners`).
- **The IR.** `countedLoop` lowers a DO from a `sema.Symbol` (the variable)
  and sema expressions, and records a `CountedLoop` (a Long variable and a
  constant step) for the optimizer. An element is a `Mem` with an `Index`
  (indexed mode), or through an A0 (`Ptr`) for an adjustable array
  (`lowerbounds.go`), whose bounds sit in a `$Bn` descriptor cell filled at
  entry. Temporaries (`temp`) are pooled per type and per statement, and
  made by `Routine.NewCell`: in `$LOCAL` in an ordinary routine, in the
  frame in a reentrant one. A whole array in an I/O list is one
  `FTN$IO_ITEM` call with a class A descriptor.
- **The optimizer** (Phase 84) knows counted loops (`DoLimits`, AOBLEQ,
  SOBGTR), loop invariants, induction variables, and registers. Its
  `Place`/`Conflict` (`opt/places.go`) take each dummy argument as its own
  region, conflicting only with itself: the standard's rule (12.5.2.9) that
  a program may not change an object through one name while another refers
  to it.
- **FTNRTL** needs nothing new for this phase (finding 16).

## A.3 Findings, and what they change

1. **The limit is temporaries, not shapes.** The plan says "shapes known at
   compile time", yet lists adjustable arrays, whose extents are known only
   at entry, and `A(1:N)` with N a variable is a section whose extent is
   known only when the statement runs. Neither needs anything new to
   scalarize: a loop's count may be any expression. What needs a size known
   at compile time is a **temporary**. So this phase compiles any array
   statement whose shape's extents can be computed when it starts, and
   reports NOTIMPL (naming Phase 104) only for a temporary whose size isn't
   constant. Conformance is checked when compiling where both extents are
   constant (NOTCONFORM, naming both shapes), and otherwise under
   `/CHECK=BOUNDS` when the program runs (Decision 5), as subscripts are
   (finding 15).
2. **The parser reads one form; sema decides.** Whether `X(1:3)` is a
   substring or a section depends on what X is, which the parser doesn't
   know. A parenthesized list after a name becomes an `Apply` whose
   arguments may include `ast.Triplet`s (`Lower`, `Upper`, `Stride`, each
   possibly nil); sema makes it a `Substring` when X is a CHARACTER scalar
   and the list is one triplet with no stride, a section when X is an array.
   A second list after an element or section (`C(I)(1:3)`,
   `B(:, J)(2:3)`) stays a substring. `primary` reads `(/ ... /)` into an
   `ast.ArrayConstructor` of values and `ast.ImpliedDo`s (the existing node,
   its items now any expressions), told from `(` by the token. WHERE,
   ELSEWHERE, and END WHERE get nodes (`ast.WhereStmt` with its mask and
   its assignment, `WhereBlockStmt`, `ElseWhereStmt`, `EndWhereStmt`). Old
   programs' substrings take the new path, so the parser's golden dumps
   change while sema's, the IR's, and the MACRO files don't
   (`TestNoF90Unchanged`).
3. **Shape belongs to the expression, not the type.** Decision 5 says
   "types carry a kind and a rank". But `sema.Type` is compared with `==` in
   hundreds of places (an element's type against a dummy's, a
   conversion's need) and becomes `ir.Type`, a map key; a rank in it would
   make `REAL` and `REAL` array unequal wherever only the element's type
   matters. The proposal: a `Shape` on the expression (`exprBase.Shape`,
   nil for a scalar; each extent a constant or a sema expression computed
   when the statement starts), with `Rank()`; `Type()` stays the element's
   type. Question 1.
4. **Two kinds of context.** Every caller of `value` today expects a
   scalar, and the subscripts, DO bounds, IF and CASE expressions, units,
   formats, and procedure arguments keep needing one. `value` stays
   scalar-only, a rank above 0 now NOTSCALAR ("an array where a scalar is
   required"), NEEDSUBSCRIPT kept for a bare array name. A new `arrayValue`
   is used only where an array may stand, a table in one place: an
   assignment's right side (and its target, through `target`), an
   operand of an elemental operation, an argument of an elemental or array
   intrinsic, an output item, a WHERE mask, a constructor's value, a vector
   subscript. An array-valued expression as an actual argument to a
   program's own procedure is Phase 104's (sections and copies to old
   dummies): NOTIMPL naming it. A whole array as an argument and as an I/O
   item stays as it is. A section of a record's array field, or of an array
   of records, is Phase 105's.
5. **The scalarizer works on element maps over normalized loops.** For an
   array assignment of rank n the lowerer makes n loop index cells, each
   running 1 to its dimension's extent by 1 (or down, finding 6), the
   leftmost dimension innermost, so memory is walked in order. Every
   array-valued node then answers one question, "your element at indexes
   k1..kn", as a scalar IR expression:
   - a whole array or triplet: subscript l + (k − 1) × s per dimension,
     through the existing element code (indexed mode, or the A0 of an
     adjustable array);
   - a vector subscript: V's element k, loaded, as the subscript;
   - an elemental operation or intrinsic: the operation on its operands'
     elements, a scalar operand broadcast;
   - TRANSPOSE: the element with the indexes swapped;
   - a constant constructor or array PARAMETER: an element of its `$PDATA`
     datum (finding 10);
   - a reduction with DIM=, DOT_PRODUCT, an element of MATMUL: a scalar
     computed by an inner loop of its own, accumulating into a scalar
     temporary, inside the nest (`B = SUM(A, DIM=1)`, `C = MATMUL(A, B)`
     need no array temporary).
   One nest serves the whole statement (Decision 6's fusion). The loops are
   `CountedLoop`s with a Long index and a constant step, so `DoLimits`,
   AOBLEQ and SOBGTR, invariant motion of bases and A0s, the induction pass
   (k × s), and register allocation apply as to a DO. `countedLoop` gains a
   form over a compiler cell (`Mem`) rather than a `sema.Symbol`; its body
   function is already shared. What isn't an element map is computed
   before the nest: a reduction with no DIM= (a scalar, hoisted, which
   7.5.1.5 requires anyway: `A = A / SUM(A)`), MAXLOC and MINLOC (into a
   small INTEGER temporary, several results from one scan), and a
   constructor whose values aren't constant (finding 10).
6. **Overlap, with four answers** (`sema/overlap.go`, Decision 6). For the
   target and each array the right side reads:
   - **apart**: different storage (different symbols not joined by COMMON
     or EQUIVALENCE; two dummies, by the optimizer's rule in A.2): no
     temporary;
   - **same element**: the same subscripts for every index (`A = A + 1`,
     `A(1:N) = 2 * A(1:N)`): no temporary;
   - **ordered**: the same array, equal triplets in every dimension but
     one, where the strides are equal and the starts differ by a constant:
     the loop over that dimension runs the way that reads each element
     before it is written (`A(2:N) = A(1:N-1)` runs down, `A(1:N-1) =
     A(2:N)` up): no temporary;
   - **unknown** (anything else, a vector subscript of the target's array
     on either side, TRANSPOSE of it, MATMUL of it): the right side goes
     into a temporary first, then the temporary is assigned.
   The third answer settles most of the plan's open question 1 at compile
   time; a run-time test of address ranges stays Phase 110's, if
   measurements ask for it. Every case gets an IR golden dump and a run.
7. **Temporaries** are array cells of constant size, pooled per statement
   by element type and size as `$Tn` are (`$An`). Where they live is
   question 2. A temporary whose size isn't constant is NOTIMPL until
   Phase 104 (finding 1): `X(1:N) = X(N:1:-1)` is the common case.
8. **WHERE.** The mask goes into a LOGICAL\*1 temporary, then each
   assignment is a nest testing it, the ELSEWHERE's testing its complement
   (7.5.3.2). No temporary is needed when no assignment in the construct
   changes what the mask reads (finding 6's test, "apart" or "same
   element", against every assignment), the common `WHERE (A > 0) A =
   SQRT(A)`: the mask is computed in each nest, element by element. A
   nonelemental reference in an assignment is computed whole before its
   nest, unmasked, as the standard says. A labeled statement inside a WHERE
   construct is not a branch target (`labels.go`: the construct is a block
   whose only target is its WHERE). A WHERE inside a WHERE, or `ELSEWHERE
   (mask)`, is Fortran 95's: F95ONLY, naming Phase 112.
9. **Elemental intrinsics.** Table D-3's rows gain class E (all of them are
   elemental in Fortran 90; SIZEOF and NWORKERS, DEC's, aren't, and stay
   scalar), and an elemental row given an array argument is an element map,
   other arguments broadcast. A routine of MTHRTL is called once per
   element. MERGE (13.13.67) is elemental, and costs a row and a `Branch`:
   the plan puts it in Phase 104 with the transformational functions
   (question 3).
10. **Constant arrays.** The evaluator gains an `ArrayConst` (element type,
    shape, and `Const`s in array element order) for constructors of
    constants, array PARAMETERs, their elements and sections with constant
    subscripts (folded to a `Const` or a smaller `ArrayConst`), elemental
    operations and intrinsics of constants, the inquiries of arrays with
    constant bounds, and RESHAPE of constants (7.1.6.1's list; question 3).
    An array PARAMETER referenced with a subscript that isn't constant, or
    in an array expression, is a read-only `$PDATA` datum, written only if
    referenced. `REAL :: A(3) = (/ ... /)` gives `data.go` one `Initial` per
    element, its size checked against the array's. A constructor whose
    values aren't all constant is built into a temporary (finding 7), except
    as the whole right side of an assignment to a rank-1 target it doesn't
    overlap, where each value is stored straight into the target at a
    running position; then its size may be any expression. CHARACTER
    values of different lengths are CONSTRUCTLEN. The goal's "array
    constants in DATA" is only the type declaration's `=`: a DATA
    statement's values are scalar constants (R532), and nothing changes.
11. **CHARACTER arrays.** Sections with a substring range, element-by-element
    assignment with padding (a `CharMove` per element), comparison into a
    LOGICAL array, and concatenation of arrays, each element through a text
    cell of its constant length. A passed-length dummy array's elements are
    already sized at run time (Phase 78's descriptors), so its sections need
    no new mechanism.
12. **The reductions and inquiries.** SIZE, LBOUND, UBOUND, and SHAPE of an
    array with constant bounds fold; of an adjustable array they read its
    `$Bn` cell; of a section or expression, LBOUND is 1 and UBOUND the
    extent. The reductions are in line (Decision 6), each with its
    identity for size zero (A.1), MASK= an elemental condition on each
    element. **DIM= must be constant** when ARRAY's rank is above 1,
    since the result's shape depends on its value; a DIM= that isn't is
    NOTIMPL naming Phase 104 (question 4). MAXLOC and MINLOC count from 1,
    and give zeros when no element is found (question 5). DOT_PRODUCT of
    COMPLEX conjugates its first argument, and of LOGICAL is ANY(A .AND.
    B) (13.13.30). MATMUL and TRANSPOSE in line as the plan says, with the
    three shapes of 13.13.62 (matrix × matrix, vector × matrix, matrix ×
    vector). Under `/NOF90` none of these names is intrinsic without an
    INTRINSIC statement (Decision 2).
13. **Vector subscripts** are element maps (finding 5) whose subscript is
    any rank-1 INTEGER array expression's element, itself mapped
    (`A(IV + 1)`). A constant vector with a repeated value in an assignment's
    target or an input list is MANYONE; one that isn't constant is the
    program's to get right, as the standard leaves it.
14. **I/O.** An output item that is a section, an expression, or a
    constructor becomes a loop of `FTN$IO_ITEM` calls in array element
    order, each value through a temporary as a scalar expression's is today;
    an input item that is a section, a loop over its elements' places. A
    whole array keeps its one class A item (descriptors for sections are
    Phase 109's). A many-one section in an input list is MANYONE when
    constant (finding 13).
15. **`/CHECK=BOUNDS`.** A triplet's first and last subscripts actually used
    are checked with Phase 85's `InRange` (not every element), a vector
    subscript's every value, and two extents not known to be equal when
    compiling, once before the nest. A shape that doesn't conform is a new
    condition (Decision 15: a FOR number past VAX FORTRAN's, govax's
    wording), signaled as SUBRNG is; FTNRTL gains its message and GSMATCH's
    minor id.
16. **What else sees nothing new.** FTNRTL (but finding 15's message), the
    debugger, and the listings: the storage map lists `$An` as it lists
    `$Tn`. The cross reference counts a section as a reference to its array,
    and an assignment to one as a definition. `/STANDARD` judges nothing
    (Phase 110).

## A.4 The subtasks, revised

1. **Survey** (this Part A) (done 2026-11-17; accepted by the author the
   same day).
2. **The forms:** triplets in subscript lists with sema deciding substring
   or section, array constructors, and the WHERE nodes in the parser
   (finding 2); sema's substrings through the new form, everything new
   still NOTIMPL; parser golden dumps, `TestNoF90Unchanged` (done
   2026-11-17).
3. **Shapes in sema:** `Shape` on expressions (question 1), sections, whole
   arrays in expressions, conformance when compiling, the two kinds of
   context (finding 4), assignment's checks; the code still NOTIMPL (done
   2026-11-17).
4. **The scalarizer:** normalized loop nests, element maps for whole arrays,
   sections, elemental operations, and broadcast; fused assignment;
   `countedLoop` over a cell; `/CHECK=BOUNDS`'s checks (finding 15); IR
   golden dumps; runs under both optimizer settings (done
   2026-11-17).
5. **Overlap and temporaries** (findings 6 and 7, question 2), each answer
   held by a dump and a run (done 2026-11-17).
6. **Vector subscripts** on either side (finding 13) (done 2026-11-17;
   MANYONE moved to subtask 7, which makes the first constant vectors).
7. **Constant arrays and constructors:** `ArrayConst`, array PARAMETERs,
   initialization, RESHAPE of constants and the constant inquiries,
   constructors in expressions and assignment (finding 10); MANYONE for a
   constant vector subscript with a value twice (finding 13) (done
   2026-11-17).
8. **Elemental intrinsics on arrays:** the class column for Table D-3's rows,
   MERGE (finding 9) (done 2026-11-17).
9. **Reductions and inquiries**, with MASK= and constant DIM=, DOT_PRODUCT,
   MAXLOC and MINLOC, MATMUL and TRANSPOSE (finding 12) (done 2026-11-17).
10. **WHERE** in both forms (finding 8) (done 2026-11-17).
11. **Arrays in I/O lists** (finding 14) (done 2026-11-17).
12. **Programs end to end** (a matrix and statistics program, smoothing with
    WHERE), gfortran's witness, every mode, the optimizer's cases.
13. **Close-out:** FORTRAN-INTERNALS.md's section 27 (arrays as values and the
    scalarizer, with `A(2:N) = A(1:N-1) + B(2:N)` followed through), MANUAL.md's
    14.13, HELP FORTRAN.

## A.5 Questions for the author

1. **Where shape lives (finding 3).** (a) **On the expression,
   `exprBase.Shape`, with `Type` staying the element's type**; (b) in
   `sema.Type`, Decision 5's wording, which makes every `==` on types a
   question of whether rank should count.
2. **Where array temporaries live (finding 7).** Decision 8 says the frame.
   (a) **`Routine.NewCell`'s rule, as every compiler cell today:** `$LOCAL` in
   an ordinary routine, the frame in a reentrant one (a recursive unit's
   temporaries must be per activation; an ordinary routine's need not be,
   and a large one then costs no stack, which is 2 MB with paging off);
   (b) the frame always, as Decision 8 has it, at the cost of stack for a
   big temporary in any routine.
3. **MERGE and RESHAPE (findings 9 and 10).** The plan gives both to Phase
   104. (a) **MERGE here (it is elemental: a row and a branch), and RESHAPE
   here of constants only (the only way to write an array PARAMETER of rank
   2), the rest of RESHAPE in 104**; (b) both in 104, as planned.
4. **DIM= that isn't constant (finding 12).** (a) **NOTIMPL until Phase
   104 when ARRAY's rank is above 1** (the result's shape depends on it);
   (b) here, choosing the nest at run time.
5. **MAXLOC and MINLOC with nothing to find** (size zero, or a mask all
   false; processor dependent in Fortran 90). (a) **Zeros, Fortran 95's
   rule and gfortran's answer**; (b) leave the result unchanged.

## The author's answers (2026-11-17)

The author read the survey and accepted every recommendation:

1. An expression's shape is on the expression (`exprBase.Shape`); `Type`
   stays the element's type (finding 3).
2. Array temporaries follow `Routine.NewCell`'s rule: `$LOCAL` in an
   ordinary routine, the frame in a reentrant one (finding 7).
3. MERGE comes into this phase, and RESHAPE of constants; the rest of
   RESHAPE stays Phase 104's (findings 9 and 10).
4. A `DIM=` that isn't constant, with ARRAY of rank above 1, is NOTIMPL
   until Phase 104 (finding 12).
5. MAXLOC and MINLOC with nothing to find give zeros (finding 12).

The plan's open questions are settled the same way: the run-time overlap
test stays Phase 110's, if measurements ask for it (finding 6 decides most
cases when compiling), and MATMUL is in line here, FTNRTL's for run-time
shapes in Phase 104.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-17, subtask 1:** The survey written (Part A): the standard's
  sections, constructors, constant expressions, assignment, WHERE, and array
  intrinsics read, and the passes that meet them. Its main findings: the
  phase's real limit is a temporary's size, not a shape's (finding 1); the
  parser must stop reading `A(1:N)` as a substring and let sema decide
  (finding 2); the scalarizer works on element maps over normalized loops,
  so reductions with DIM= and MATMUL need no temporary (finding 5); overlap
  has an "ordered" answer that reverses a loop instead of copying (finding
  6); MAXLOC has no DIM= in Fortran 90 and counts from 1. Five questions
  for the author (A.5). For review.
- **2026-11-17:** The author accepted the survey and every recommendation
  ("The author's answers"). Subtask 1 is done; subtask 2 is next.
- **2026-11-17, subtask 2:** The forms. The parser reads every parenthesized
  list after a name as an `ast.Apply` whose items may be `ast.Triplet`s
  (`subscripts`, `triplet`; `substringAhead` now looks only at a second list,
  `C(I)(1:3)`, `B(:, J)(2:3)`, and a record field's list, whose sections are
  Phase 105's); `primary` reads `(/ ... /)` into an `ast.ArrayConstructor`
  (`arrayConstructor`, `constructorValue`: implied-DO lists nest, and
  `impliedDo` and `substringAhead` count `(/` and `/)` as parentheses); WHERE,
  ELSEWHERE (with Fortran 95's mask read, for sema to refuse), and END WHERE
  are `ast.WhereStmt` (its assignment a `Then`, anything else BADWHERESTMT),
  `WhereBlockStmt`, `ElseWhereStmt`, and `EndWhereStmt`. A declaration's
  `= (/ ... /)` is still a `PendingStmt` until subtask 7. In sema,
  `sections.go`'s `substringForm` turns an Apply of one stride-less triplet
  after a name that isn't an array into the `ast.Substring` the parser used to
  make, and every place that tells an element from a substring by its node
  calls it first (`value`, `apply`, `target`, `place`, `dataItems`, the
  constant evaluator); EQUIVALENCE reads such a list as a substring whatever
  the name (`substringReading`: a section is never an equivalence object, so
  `A(1:2)` of an array is still NEEDSUBSCRIPT there). An array section is
  NOTIMPL ("an array section") in `element` and DATA, a stray triplet and a
  constructor by `formWhat`; WHERE stays NOTIMPL once per construct, by kind,
  as before. Two of `TestErrors`'s cases changed with it: `PRINT *, S(1:2)`
  and `DATA C(1:1)` of an array were NEEDSUBSCRIPT and are now sections,
  NOTIMPL until subtasks 3 and 11. The parser's golden dumps print a triplet
  as the substring was printed, so none changed; `TestNoF90Unchanged`'s
  digests didn't either. `parse`'s `TestArrayForms` and `TestErrors`'s new
  cases hold the forms.
- **2026-11-17, subtask 3:** Shapes in sema. `exprBase` has a third field,
  `S Shape` (the `Expr` interface's `Shape()`); its literals were made keyed
  first (`gofmt -r`, nothing else changed). `shape.go`: a `Shape` is an
  `Extent` per dimension, `Known` with its `Size`, or else `Of` an
  `*ArrayRef` or `*Section` and its `Dim`, for lowering to compute when the
  statement starts; `conform` (NOTCONFORM naming both shapes, `(10, :)`, only
  where two extents are known and differ, or the ranks do; a known extent wins
  over an unknown one), `arrayShape`, `wholeArray` (NOSHAPE for an
  assumed-size array, NOTIMPL for an array of records, Phase 105's), and
  `scalarValue` (NOTSCALAR). `sections.go`'s `section` makes a `Section` of
  `SectionSub`s (a subscript, or a triplet's INTEGER bounds and stride, a
  bound left out the dimension's constant, or nil for an adjustable
  dimension's, read from its bounds when the program runs; a stride of 1
  when left out): ZEROSTRIDE, SECTIONEND (an assumed-size array's last
  dimension needs an upper bound), and a constant triplet's first and last
  subscripts checked as an element's are (`checks.go`'s new
  `checkSubscript`, which `checkConstantSubscripts` now calls); `element`
  hands it any list with a triplet. `value` is now the scalar context
  (`scalarValue` of `arrayValue`; a bare array name still NEEDSUBSCRIPT), and
  `arrayValue` the old body plus whole arrays and sections; the operations'
  operands (`unary`, `operands`, `concat`) use it, each node taking
  `conforms`'s shape, `convert` its operand's, a `Substring` its `X`'s (a
  section's substring, `C(:)(2:3)`, allowed). Every other context says what
  it does with an array: a subscript (`subscriptValue`; a vector subscript is
  NOTIMPL until subtask 6), an actual argument (`arrayArg`: a whole array's
  name as before, anything else NOTIMPL naming Phase 104's work), an
  intrinsic's argument (NOTIMPL until subtask 8), an output item and an
  input item (`sectionPlace`; NOTIMPL until subtask 11; a whole array as
  before), a statement function's value (NOTSCALAR). An assignment's target
  may be a whole array or a section (`target`), its value an array
  (`assignedExpr`), checked by `assignedShape` (NOTSCALAR for a scalar
  target, `conform` otherwise), and an array assignment is then NOTIMPL until
  subtask 4. The sema dump shows a section as `A[1:N:2, J]`, a bound from an
  adjustable array's own bounds as `*`. Changed messages, all in error
  cases: `X = A` and `X = A + 1` of an array are NOTSCALAR (were
  NEEDSUBSCRIPT; `errors.sema` updated), `A = 1` NOTIMPL until subtask 4 (was
  BADTARGET), an array of records assigned to NOTIMPL (was BADTARGET), and
  an array as a subscript a vector subscript. `TestNoF90Unchanged`'s digests
  didn't change. `TestShapes` (`shape_test.go`) holds the shapes, the
  messages' texts, and each context.
- **2026-11-17, subtask 4:** The scalarizer, `ir/lowerarray.go`. An
  `Assign` whose target has a shape is `arrayAssign`'s: a nest of counted
  loops, the leftmost dimension innermost, each index running 1 to its
  dimension's extent by 1. The loops and what they compute are in sema's
  terms, over symbols of lowering's own (`sema.NewSymbol`) bound to places in
  `lowerer.bound`, the statement functions' mechanism: an index to an
  `IndexCell` (`$In`, a new kind, pooled per statement as temporaries are, a
  dying cell to the optimizer), a value computed once to a temporary. So
  `countedLoop` needed no new form (finding 5's "over a cell" is a bound
  symbol), and an element of an adjustable array, a CHARACTER element, a
  COMPLEX product, or a comparison into a LOGICAL array is lowered by the
  code that lowers it anywhere. `sema/elements.go` has what lowering builds
  them with: `NewVar`, `NewElem` (with the new `Elem.InBounds`, which turns
  off its subscripts' checks), `IntSum`/`IntProduct`/`IntDifference` (folding
  constants), `Scalarize` (an array expression made scalar: each leaf, an
  `ArrayRef` or `Section`, by a callback, each scalar part by another, each
  elemental node copied without its shape), and `Leaves`. Before the loops
  `prepare` computes each leaf's subscripts as `k * stride + base`, `base =
  lower - stride` (a whole array's `k + (lower - 1)`), every part not
  constant in a temporary (an adjustable array's bounds from its
  descriptor); each extent is a constant if the target's or the value's
  shape knows it, else the target's computed (`leafExtent`: a descriptor's
  multiplier, or a triplet's trip count, never below 0, `atLeastZero`); each
  scalar part of the value not `sema.Inline` (a constant, a variable, an
  element with constant subscripts, any CHARACTER value) goes to a temporary
  by `store` (`hoist`), so a function is called once and 7.5.1.5's "before
  any element is defined" holds. `assign` is split, `assignScalar` the
  element's. Under `/CHECK=BOUNDS` (finding 15) each section's first and
  last subscripts are checked once, under a test that it has elements, a
  constant in range not at all (`checkSubscripts`), and every leaf's extent
  not known when compiling to equal the loops' is compared
  (`checkShapes`): a difference calls FTNRTL's new `FTN$NOTCONFOR` (vector
  680, `ftn_control.mar`, GSMATCH 1,21), which signals `FOR$_NOTCONFOR`,
  govax's own condition at FOR number 300 (`^X188964`, "array shapes do not
  conform", `vmsdef/formessages.go`; MANUAL.md's `/CHECK` and FTNRTL tables).
  Sema keeps the scalarizer to what it computes right: `sema/overlap.go`'s
  `checkOverlap` is subtask 5's test with two answers so far, apart
  (`mayShare`: the same name, a common block's overlapping places, an
  EQUIVALENCE group; two dummies apart) and the same elements
  (`sameElements`: the same whole array, or a section of the same text), an
  inline scalar of the target's storage counting too (`A = A(1)`); anything
  else is NOTIMPL until subtask 5. `checkArrayParts` refuses (NOTIMPL) a
  CHARACTER function's value and a concatenation of a run-time length in an
  array expression, which would be made at each element. The array
  assignment's NOTIMPL of subtask 3 is gone. Index cells were first `$Kn`,
  which MACRO refused: `$Kn` are the data's names. Tests: `ir`'s
  `f90arrays.for` (every form, an adjustable array) and `f90arrchk.for`
  (`/CHECK=BOUNDS`), `TestShapes`'s new overlap cases, `TestFortranCheck`'s
  SECTION, STRIDE, EMPTY, and CONFORM programs, and `testdata/fortran90`'s
  `arrays.f90`, worked by hand, run in all eight modes and the same on
  gfortran (`compare.py`'s `PROGRAMS90`). Under `/OPTIMIZE` its first loop
  is `MULF3`, `ADDF3`, `AOBLEQ`. `TestNoF90Unchanged`'s digests didn't change.
- **2026-11-17, subtask 5:** Overlap and temporaries. `sema/overlap.go`'s
  `arrayOrder` replaces `checkOverlap` and gives every array assignment an
  `ArrayOrder` (`Assign.Order`, nil for a scalar assignment; the sema dump
  shows it after the statement, `[down in 1]` or `[through a temporary]`).
  For each array the value reads that may share the target's storage
  (`mayShare`, as before), `relate` compares the two sides' subscripts as
  `slot`s (a whole array is a triplet per dimension from its lower bound by
  1; a substring's characters don't count, an element's own characters being
  moved as if through a temporary by MOVC3 and MOVC5): the same fixed
  subscripts and the same triplet starts and strides are the same element; a
  fixed subscript differing by a constant, or starts differing by a constant
  that isn't a multiple of the stride, never meet (apart); starts differing
  by a constant multiple of a constant stride are ordered, the outermost such
  loop to run up if the leaf reads ahead, down if behind (`difference`,
  `offsetForm`: `N+1` and `N` differ by 1); anything else is unknown. Two
  leaves that need opposite directions in one loop, an unknown answer, or a
  CHARACTER scalar of the target's storage read at each element, need a
  temporary, whose extents (`TempShape`) must be known when compiling, else
  NOTIMPL naming Phase 104 (`X(1:N) = X(N:1:-1)`). A numeric scalar the loops
  would read at each element that reads the target's storage (`A = A / A(10)`)
  is computed once before them instead (`ArrayOrder.Hoisted`, asked by the
  scalarizer's `hoist`). In `ir/lowerarray.go`, `arrayAssign` is now
  `arrayNest` run once, its loops counting down from the extent where `Down`
  says, or twice through `arrayTemp`, a new `ArrayTempCell` (`$An`, pooled per
  statement by element type and size, `Routine.NewCell`'s rule, so in the
  frame in a reentrant routine), bound to a symbol of lowering's own
  (`sema.NewArray`, `NewArrayRef`): the value into the temporary, then the
  temporary into the target, each nest with its own `/CHECK=BOUNDS` checks.
  Tests: `TestArrayOrder` (19 cases, each answer), `TestShapes`'s overlap
  cases (now compiled, and the unknown size's NOTIMPL), `ir`'s
  `f90overlap.for`, and `testdata/fortran90`'s `overlap.f90`, worked by hand,
  run in all eight modes and the same on gfortran.
- **2026-11-17, subtask 6:** Vector subscripts. `SectionSub` has a `Vector`
  (an INTEGER array expression of rank one; `Triplet()` is now neither an index
  nor a vector, `Ranges()` either a triplet or a vector), and `element` is now
  `section`'s, which analyzes every subscript once with `arrayValue` and makes
  an `Elem` when nothing ranges: a rank-1 subscript is a vector, its extent the
  vector's, a higher rank VECTORRANK (new). A subscript with an error no longer
  stops the others' analysis, as `subscripts` didn't. `subscriptValue` is now
  only the record fields' (a vector there is Phase 105's). The sema dump shows
  a vector as `<IV[]>`. In the scalarizer a leaf's dimension may be a
  `vector`, and `prepareIn` prepares the vector's own arrays as leaves ranging
  over the loop of the dimension the vector is in (`A(2, IV)`'s IV over the
  nest's one loop; a vector of a vector maps through both); `element` makes the
  subscript the vector's element there (`sema.Scalarize` of it), and
  `leafExtent` takes a vector dimension's extent from the vector's shape.
  `Elem.InBounds` is now per subscript (`Elem.Checked`): a vector's values
  are each checked where they are used under `/CHECK=BOUNDS` (`inrange`), the
  vector's own section once before the loops. Overlap: the same vector
  subscript text on both sides is the same element (a many-one section may not
  be assigned to), any other vector unknown; a vector subscript in the value
  that reads the target's storage sends the value through a temporary
  (`vectorsRead`), and one in the target that does (`IV(IV) = 1`, whose
  subscripts would be read as the loops change them) is NOTIMPL. MANYONE needs
  a constant vector, and there is none until subtask 7's constructors and array
  PARAMETERs, so it moved there. Tests: `TestShapes`'s vector cases,
  `TestArrayOrder`'s four, `ir`'s `f90vector.for` and `f90vecchk.for`,
  `TestFortranCheck`'s VECTOR, and `testdata/fortran90`'s `vectors.f90`,
  worked by hand, run in all eight modes and the same on gfortran.
- **2026-11-17, subtask 7:** Constant arrays and constructors. `sema.ArrayConst`
  (its elements `Const`s in array element order, its shape known) is what the
  evaluator gives for an array form (`constarray.go`): `isArrayForm` decides
  from the syntax tree whether a constant expression is an array (a
  constructor, an array constant's name or a section of one, RESHAPE, SHAPE,
  LBOUND and UBOUND without DIM=, an elemental intrinsic of `constFuncs` with
  an array argument, an operation with an array operand), so each is evaluated
  once; `evalAny` takes either kind and `eval` refuses an array (NOTSCALAR).
  Each element is computed by the scalar code, `unaryOn`, `binaryOn`, and
  `concatOn` now split from `unary`, `binary`, and `concat`. A constructor's
  values must have one type and length (CONSTRUCTTYPE, CONSTRUCTLEN; an
  INTEGER of another length takes the first's), an implied DO's variable an
  INTEGER (CTORDOVAR), at most 2^20 elements (CONSTRUCTSIZE). An array
  PARAMETER (`defineArrayParameter`, in both the `::` form and `DIMENSION`
  then `PARAMETER (P = ...)`, which isn't read differently under `/NOF90`) is
  a `Constant` with its `Dims` and an `ArrayValue`, given an array of its
  shape or a scalar for every element (NOTCONFORM otherwise), converted to its
  type; an element or section of it with constant subscripts is folded
  (`constantSection`), one with subscripts not constant an `Elem` or `Section`
  of the constant's symbol, read from data (`ir`'s `constantArrayMem`).
  RESHAPE (SOURCE, SHAPE, PAD, ORDER; BADRESHAPE), SIZE, SHAPE, LBOUND, and
  UBOUND (of an array whose bounds are constants, or an array constant's, 1 and
  its extents; DIMRANGE) are fortran90Table's new ARRAY rows, answered by the
  evaluator; with an array whose bounds aren't constants they are subtask 9's.
  An initial value may be an array constant of the array's shape
  (`initialValue`, `dataValues.array`). The parser reads a declaration's `= (/
  ... /)` (`entityValue`; `pending` is gone). A constructor whose values aren't
  all constants is a `sema.Constructor` (`constructor.go`): items of values and
  `CtorDo`s, each implied DO's variable a symbol of its own in a scope of its
  own (the standard's 14.1.3), its extent known when every item's is. In
  `ArrayOrder`, `Fill` says a Constructor that is the whole value of a rank-1
  assignment and reads nothing of the target's (`reads`, `readsAny`: a
  function's reference reads what it is given, and a common block or a dummy's
  storage) is stored straight into the target, its extent then any; any other
  Constructor is built before the loops into an array temporary, whose size must
  be known (NOTIMPL naming Phase 104), and an `ArrayConst` is read from an
  `ArrayDatum` (`$Kn`, written only if read, `.BYTE`/`.WORD`/`.LONG` or
  longwords; a CHARACTER one a text): both are leaves to the scalarizer, apart
  from the target. `checkManyOne` is MANYONE. `ir/lowerctor.go`'s
  `fillConstructor` keeps a running position in an index cell, each item's
  values stored at the next (an array item by `eachElement`, a nest of the
  item's own shape; an implied DO a counted loop over its own variable, bound to
  an index cell); under `/CHECK=BOUNDS` a position past the target's extent, or
  an end short of it, calls FTN$NOTCONFOR. `arrayNest` is now `newNest`,
  `prepare`, the loops, and `done`. Tests: `TestArrayConstants` (49 cases),
  `TestErrors`' changed cases (an array PARAMETER and a declaration's
  constructor now compile; `N(1)` of a scalar constant is BADUSE as before),
  `ir`'s `f90ctor.for`, `TestFortranCheck`'s FILL, and `testdata/fortran90`'s
  `constants.f90`, worked by hand, run in all eight modes and the same on
  gfortran.
- **2026-11-17, subtask 8:** Elemental intrinsics on arrays. Table D-3's rows,
  which have no class column, are class E when read (`readTable`), but SIZEOF
  and NWORKERS, DEC's, and LEN, an inquiry (`notElemental`). `intrinsicRef`
  takes array arguments: their shapes conform (NOTCONFORM), the row is chosen
  by the elements' types as before, and the value `intrinsicValue` makes
  (an `IntrinsicRef`, or a `Convert`, `Binary`, `Unary`, or `Compare` of one)
  gets the shape on every node above an array (`giveShape`). A row that isn't
  elemental given an array is NOTIMPL, but LEN of a CHARACTER array of known
  length, that length. `sema.Scalarize` walks an `IntrinsicRef`'s arguments,
  so the scalarizer computes the function at each element, a MTHRTL routine
  called once per element, a scalar argument that isn't inline computed once
  before the loops; overlap and `checkArrayParts` see into its arguments too.
  MERGE (13.13.67) is a fortran90Table row, class E, analyzed by `merge`:
  TSOURCE and FSOURCE of one type and length (MERGEARGS, new), MASK LOGICAL
  (BADARG), a `MergeOp` `IntrinsicRef` of TSOURCE's type; lowered as FSOURCE
  in a temporary that TSOURCE replaces where MASK holds (`choose`), a
  CHARACTER one through a text temporary (`mergeText`); of a length not
  constant, or COMPLEX*32, NOTIMPL. An ARRAY row (SIZE and the rest) of an
  array whose bounds aren't constants is NOTIMPL until subtask 9 (it had
  reached an `IntrinsicRef` of no operation). Tests: `TestElementals`,
  `TestShapes`' changed case (`X = SQRT(A)` is NOTSCALAR), `ir`'s
  `f90elem.for`, and `testdata/fortran90`'s `elements.f90`, worked by hand, run
  in all eight modes and the same on gfortran.
- **2026-11-17, subtask 9:** Reductions and inquiries. fortran90Table has REDUCE
  rows for SUM, PRODUCT, MAXVAL, MINVAL, ALL, ANY, COUNT, DOT_PRODUCT, MAXLOC,
  MINLOC, MATMUL, and TRANSPOSE, and `intrinsicRef` sends them and the ARRAY rows
  (SIZE, SHAPE, LBOUND, UBOUND, of an array whose bounds aren't constants; RESHAPE
  of values not constant is NOTIMPL naming Phase 104) to `sema/arrayfuncs.go`'s
  `arrayFunction`, which makes a `sema.ArrayFunc` (`Array`, `Other`, `Mask`, a
  constant `Dim`, or for SIZE, LBOUND, and UBOUND a `DimExpr`): ARRAYARG,
  ARRAYRANK (new), BADARG, DIMRANGE; a DIM= not constant with ARRAY of rank above
  1 NOTIMPL naming Phase 104; a DIM= of a vector's reduction no DIM= at all.
  DOT_PRODUCT is a SUM of the product, the first vector's CONJG for COMPLEX, or an
  ANY of the .AND. for LOGICAL. MATMUL takes its three forms, the shared extent
  conforming. In `ir/lowerarrayfunc.go` a scalar one (`arrayFuncValue`, from
  `l.expr` and the COMPLEX paths) is a nest of its own: `reduce` (an accumulator
  given the empty array's value, SUM 0, PRODUCT 1, MAXVAL the most negative
  number, MINVAL the most positive, COUNT 0, ANY false, ALL true; each element
  added where MASK holds, `masked`), or `boundValue` (`boundsOf`: a whole array's
  bounds, 1 and 0 for an empty dimension, any other array's 1 and its extent; a
  DIM= not constant chooses when the program runs). With DIM=, and MATMUL, an
  element is computed where it is used: `prepareFunc` prepares the arguments'
  arrays as leaves on the statement's loops (a reduced dimension on none, -1) so
  extents and checks come from them, and `funcElement` gives an `ArrayFuncAt`
  whose lowering is the same nest over the one dimension, the statement's index
  variables standing for the others (`reduce`'s mapping; `matmulAt`). TRANSPOSE
  is its argument's element with the loops swapped. MAXLOC and MINLOC (`locate`:
  the first extreme in array element order, the loops' own indexes counting from
  1, zeros for none) and SHAPE, LBOUND, and UBOUND without DIM= (`precompute`)
  fill a small INTEGER temporary before the statement's loops. Overlap: an
  element-wise one that reads the target's storage needs a temporary
  (`funcReads`, `leafReads`); the others are computed before the loops. MERGE of
  COMPLEX values, which `complexParts` had taken for CONJG, is `mergeComplex`.
  MANUAL.md's "Not there yet" and `/CHECK` table say what now compiles. Tests:
  `TestArrayFunctions`, `ir`'s `f90reduce.for`, and `testdata/fortran90`'s
  `reduce.f90`, worked by hand, run in all eight modes and the same on gfortran.
- **2026-11-17, subtask 10:** WHERE. `sema/where.go`: WHERE (mask) opens a
  construct on the analyzer's stack (`construct.whereStart`, `where`;
  `isWhere`, and `isIf` now excludes it), its blocks two `Arm`s of a
  `sema.Where` (`Then`, `Else`), so `add` and the label paths treat it as an IF
  block: a jump into it from outside, to an assignment, ELSEWHERE, or END WHERE,
  is INTOBLOCK ("a WHERE construct"), and only the WHERE is a target. The WHERE
  statement is a `Where` of one assignment (`whereStatement`, through
  `executable`, so a logical IF's statement may be one). The mask is a LOGICAL
  array, an INTEGER one converted as an IF's condition is (WHEREMASK, new,
  for a scalar or another type); each assignment's target an array of the
  mask's shape (WHERETARGET, new; NOTCONFORM); only assignments inside
  (WHEREBODY, new, the statement's structure kept by `brokenStatement`); a
  second ELSEWHERE is ELSEWHEREORDER; NOWHERE and NOENDWHERE as SELECT CASE's;
  a WHERE inside a WHERE and ELSEWHERE (mask) are F95ONLY (new, naming Phase
  112). `assignment` is split, `assignParts` the target and value, so a WHERE's
  assignments (`whereAssign`) are checked as any are and ordered at END WHERE
  (`finishWhere`): `arrayOrder` with `analyzer.noFill` (a constructor isn't
  stored straight into the target, the mask skipping some of it; with a size
  not known it is then NOTIMPL as any constructor in a temporary), then
  `maskInLoops`, finding 8's test made exact: the loops may compute the mask
  themselves if no earlier assignment's target reads into the mask
  (`arrayReads`, `funcReads`' old body), each assignment's mask relates to its
  target as its value does (`relateValue`, `arrayOrder`'s leaf loop taken out,
  the value's needs kept in `ArrayOrder.need`; a mask one element behind runs
  the loop down, one ahead and a value behind need the temporary), a value
  through a temporary has a mask that reads nothing of the target's, and, with
  more than one assignment, the mask calls none of the program's procedures
  (`callsProcedure`). So `WHERE (A > 0) A = SQRT(A)` and `B = A ... END WHERE`'s
  last write of A are in the loops, while `A = -A` then `B = 1` puts the mask in
  a temporary (`Where.MaskTemp`, its extents; NOTIMPL naming Phase 104 when not
  known). A scalar of the target's in the mask (`A > A(1)`) is hoisted as the
  value's are. The temporary has the mask's own type, not LOGICAL\*1 as
  finding 8 said: no conversion at each element, at 4 bytes an element. In
  `ir/lowerwhere.go`, `where` computes the mask into an `arrayTemp` if it must,
  then each assignment by `maskedAssign` (`arrayAssign` with a mask), whose
  nests take the mask's arrays as leaves beside the value's (so `/CHECK=BOUNDS`
  compares their extents) and assign each element under `maskedBy`'s branch,
  ELSEWHERE's negated; a value through a temporary is masked in both nests, so
  nothing is computed where it isn't assigned (`SQRT` of a negative). The
  construct's temporaries are given back at its end. `pendingOpeners` and its
  maps are empty now. Tests: `TestWhere` (37 cases), `TestErrors`' changed
  cases, `TestF90StatementsNotImplemented`, `ir`'s `f90where.for`,
  `TestFortranCheck`'s WHERE, and `testdata/fortran90`'s `where.f90`, worked by
  hand, run in all eight modes and the same on gfortran. MANUAL.md's "Not there
  yet" says WHERE is there.
- **2026-11-17, subtask 11:** Arrays in I/O lists. An output item may now be any
  array value (`sema/io.go`'s `outputItem`, its parts held to what the
  scalarizer computes at each element by `checkArrayParts`, as an assignment's
  value is), and an input item a section or a section's substring
  (`inputPlace`: `place`'s rule, but a section is a place here; MANYONE for a
  constant vector subscript with a value twice, `checkManyOne`, whose message now
  says the section "can't be given values", fitting a READ as well as an
  assignment). In `ir/lowerio.go` an item is `ioItem`'s: a scalar, or a whole
  array of the program's (its one class A descriptor), is one `FTN$IO_ITEM` call
  as before; a constructor's values are items in turn (`fillItems`, its implied
  DOs loops of their own), so it is never built; any other array is a call per
  element through the scalarizer's `eachElement`, an input element a place read
  where it is, an output one a value in a temporary as a scalar expression's is.
  Each item's extents and subscripts are computed when the list reaches it, so
  `READ *, N, A(1:N)` uses the N just read; `/CHECK=BOUNDS` checks a section's
  first and last subscripts once before its loop, as in an assignment. An
  internal file or ENCODE's buffer that is a section stays NOTIMPL (a
  section's descriptor is Phase 109's), now with one message: `ioUnit` stops at
  a place with an error instead of going on to NOTSCALAR. FTNRTL needed nothing.
  Tests: `TestShapes`' I/O cases (the old NOTIMPLs now compile, and 11 new ones),
  `TestErrors`' changed case, `ir`'s `f90io.for`, `TestFortranCheck`'s
  IOSECTION, and `testdata/fortran90`'s `arrayio.f90` (with `arrayio.in`),
  worked by hand, run in all eight modes and the same on gfortran.
  `where.f90`, which subtask 10's entry says ran in all eight modes, was
  missing from `fortran90PagingPrograms`; it is there now, and passes.
  MANUAL.md's "Not there yet" says I/O lists take arrays.
