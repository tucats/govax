# Phase 99 — Fortran 90: the plan, the decisions, and the source forms

**Status:** planned 2026-11-17; the plan reviewed and accepted by the author
the same day, every recommendation taken ("The author's answers", after the
questions). Subtask 1 (this survey and plan, the skeletal docs of Phases 100
to 111, PLAN.md's rows, the Fortran 90 Handbook's text) is done; subtask 2 is
next. Needs the FORTRAN program (Phases
74 to 86) and Phases 88 and 89 (indexed files; RECURSIVE, RESULT, and each
activation's storage in its frame).

govax's FORTRAN compiler accepts VAX FORTRAN V5.0: FORTRAN-77 with DEC's
extensions, and the later DEC dialect's RECURSIVE and RESULT (Phase 89). This
program extends it to **Fortran 90** (ISO/IEC 1539:1991): free source form,
the new declarations and kinds, the control constructs, array expressions and
sections, dynamic arrays, derived types, modules, interfaces and generic
procedures, pointers, the new intrinsic procedures, and the new I/O. The
compiler stays one compiler: a Fortran 90 program and a VAX FORTRAN program
go through the same passes, and a program may mix DEC's extensions with
Fortran 90's, as DEC's own later compilers allowed (the HP Fortran manual is
the precedent). **Nothing the compiler does today is removed or changed by
default.** Where Fortran 90 reads a text differently from VAX FORTRAN, a new
qualifier, `/F90`, chooses Fortran 90's reading (Decision 2).

**Fidelity** is as it was for the FORTRAN program (`PHASE-74`'s Decision 2),
with one difference: there is no DEC compiler for Fortran 90 on the VAX to
compare with, and none is wanted. The target is the standard's meaning:

- the **language** the standard defines, read with the Handbook beside it;
- the **run-time behavior** a program can observe, where the standard
  specifies it (values, shapes, allocation status, I/O records), and our own
  documented choice where it leaves it to the processor;
- the **VAX calling standard**, so Fortran 90 routines still call VMS and are
  called by it, and new data (dynamic arrays, pointers) is passed in VMS's own
  descriptor forms where one fits (Decision 7).

The second goal, **teaching**, is unchanged: each new piece of the compiler
explains itself to a reader who has never written a compiler, and
`docs/FORTRAN-INTERNALS.md` gains a section for each phase (Decision 17).

## Sources

- **The standard**, ISO/IEC 1539:1991 (E), Fortran 90:
  `~/Documents/Technical Doc/VMS/Fortran90.pdf` (the same file as
  `FORTRAN-90-spec-N692.pdf`), with its text `Fortran90.txt` (searchable; its
  table of contents prints section numbers apart from their titles). The
  final word on any rule. Its syntax rules (R201 to R1219) are cited by
  number in the code, as the 1988 manual's sections are today.
- ***Fortran 90 Handbook*** (Adams, Brainerd, Martin, Smith, Wagener;
  McGraw-Hill 1992), `Fortran 90 Handbook.pdf`. Its text was saved beside it
  as `Fortran 90 Handbook.txt` on 2026-11-17 (`pdftotext -layout`; a clean
  extraction, not an OCR). Chapters 1 to 14 follow the standard's sections,
  with rationale and examples; Appendix A describes every intrinsic
  procedure, Appendix B is the syntax with a cross reference, Appendix C the
  obsolescent features. The first place to read for a feature; the standard
  settles a doubt.
- ***HP Fortran for OpenVMS Language Reference Manual*** (2005,
  `VMS-FORTRAN-LANGUAGE.txt`): Fortran 95 on Alpha and Itanium, with DEC's
  extensions. **Not the target** (it is Fortran 95, and its run-time library
  and descriptors aren't the VAX's), but the **precedent** for how a DEC
  compiler joined Fortran 90 to VAX FORTRAN: the TYPE statement beside the
  derived-type statement (its 10.6), `%` and `.` both selecting components
  (its 3.x), the kinds as byte sizes, the alternative octal and hexadecimal
  forms (its B.7), and module files. Where it and the standard differ, the
  standard wins.
- ***OpenVMS Calling Standard*** (`OVMS_73_CALL_STAND.PDF`), chapter 5: the
  class A (contiguous array) and class NCA (noncontiguous array) descriptors,
  section 5.7, Figure 5-7. NCA's three blocks (prototype, strides, bounds,
  with the virtual origin A0) are exactly what a Fortran 90 compiler needs to
  describe an array section, an assumed-shape dummy, or an array pointer
  (Decision 7).
- The sources the FORTRAN program used (the 1988 *VAX FORTRAN Language
  Reference Manual* first) stay in force for everything VAX FORTRAN already
  had.

No clean-room question arises: the standard and the Handbook are public
descriptions of a language, and nothing of DEC's compilers or RTLs is
studied. **gfortran** (on the host, GCC 16.2) is a *witness*, not an oracle,
as it was for DUNGEON (Phase 86) and the recursive programs (Phase 89): a
portable program's output on gfortran and on govax is compared, and a
difference is read against the standard before either is believed
(Decision 16).

# Part A — the survey

## What Fortran 90 adds

Grouped by subject, with the standard's section and the phase that takes it
(Part C). "Already" marks what VAX FORTRAN V5.0 or Phase 89 gave govax.

| Subject | What | Standard | Phase |
|---|---|---|---|
| Source form | free form: 132 columns, `!` comments, `&` continuation (and a leading `&` in a continued character constant), `;` between statements, blanks significant; 39 continuation lines | 3.3 | 99 |
| Lexical items | names of 31 characters (already); `"` character constants; `B'…'`, `O'…'`, `Z'…'` constants; kind parameters on constants (`1.5_8`, `1_2`, `.TRUE._1`, `kind_"text"`); `==`, `/=`, `<`, `<=`, `>`, `>=`; `::`, `=>`, `%`, `(/ /)`; defined operators `.name.` | 3.1, 3.2, 4.3 | 99 |
| END statements | `END PROGRAM name`, `END SUBROUTINE name`, `END FUNCTION name`; END DO already; `END IF name` and the rest with construct names | 2.3, 8.1 | 99, 101 |
| Declarations | `::` type declarations with attribute lists; initialization in the declaration; KIND= and LEN= selectors (`REAL(KIND=8)`, `REAL(8)`, `CHARACTER(LEN=10)`, `CHARACTER(*)`); attribute statements and attributes: PARAMETER, DIMENSION, INTENT, OPTIONAL, SAVE, EXTERNAL, INTRINSIC, PUBLIC, PRIVATE, POINTER, TARGET, ALLOCATABLE; IMPLICIT with kinds | 5 | 100 (attributes needing later phases parsed then, compiled with their phase) |
| Kinds | KIND, SELECTED_INT_KIND, SELECTED_REAL_KIND; the numeric model and its inquiries (HUGE, TINY, EPSILON, DIGITS, PRECISION, RANGE, RADIX, MAXEXPONENT, MINEXPONENT, BIT_SIZE) | 4.3, 13.7 | 100 |
| Control | SELECT CASE; DO with no control; EXIT and CYCLE; construct names on IF, DO, SELECT CASE; DO WHILE already | 8.1 | 101 |
| Arrays (static) | whole-array expressions and assignment; sections with triplets and vector subscripts; array constructors with implied DO; array-valued PARAMETERs; WHERE and ELSEWHERE; elemental intrinsics on arrays; the reductions and inquiries (SUM, PRODUCT, MAXVAL, MINVAL, MAXLOC, MINLOC, ANY, ALL, COUNT, DOT_PRODUCT, SIZE, SHAPE, LBOUND, UBOUND) | 6.2, 7.1.4, 7.5.3, 13 | 102 |
| Procedures | internal procedures (CONTAINS) and host association; interface blocks and explicit interfaces; keyword arguments; OPTIONAL and PRESENT; INTENT; generic names; defined operators and ASSIGNMENT(=); RECURSIVE and RESULT already | 12 | 103 |
| Arrays (dynamic) | assumed-shape dummies; automatic arrays; ALLOCATABLE, ALLOCATE, DEALLOCATE, ALLOCATED; array-valued functions; the transformational intrinsics (MATMUL, TRANSPOSE, RESHAPE, SPREAD, PACK, UNPACK, CSHIFT, EOSHIFT, MERGE) on shapes known at run time | 5.1.2, 6.3, 13 | 104 |
| Derived types | TYPE … END TYPE; components; `%`; structure constructors; SEQUENCE; intrinsic assignment; derived-type dummies, results, and I/O | 4.4, 6.1.2 | 105 |
| Modules | MODULE, USE (ONLY, renames), PUBLIC and PRIVATE, module procedures, module data, separate compilation through module files | 11.3 | 106 |
| Pointers | POINTER and TARGET; `=>`; NULLIFY; ASSOCIATED; ALLOCATE of pointers; array pointers and pointers to sections; pointer components (lists, trees) | 5.1.2, 6.3, 7.5.2 | 107 |
| Intrinsics (the rest) | character (ACHAR, IACHAR, ADJUSTL, ADJUSTR, TRIM, LEN_TRIM, REPEAT, SCAN, VERIFY, INDEX's BACK=); CEILING, FLOOR, MODULO; the model manipulators (EXPONENT, FRACTION, NEAREST, RRSPACING, SPACING, SCALE, SET_EXPONENT); TRANSFER; MVBITS; DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER, RANDOM_SEED; keyword arguments to intrinsics | 13 | 108 |
| I/O | nonadvancing I/O (ADVANCE=, SIZE=, EOR=); OPEN's ACTION=, POSITION=, PAD=, DELIM=, STATUS='REPLACE'; INQUIRE's new specifiers and IOLENGTH=; edit descriptors B, EN, ES, and G for every type; Fortran 90's NAMELIST form (`&group … /`); sections and derived types in I/O lists | 9, 10 | 109 |
| Obsolescent | arithmetic IF, real DO variables, shared DO termination, alternate return, PAUSE, ASSIGN and assigned GO TO and FORMAT, H editing: kept, flagged by `/STANDARD=F90` | B.2 | 110 |

Fortran 90 deleted nothing from FORTRAN-77, so no existing program loses a
feature. Fortran 95's additions (FORALL, PURE and ELEMENTAL, NULL(),
CPU_TIME, default initialization of components, automatic deallocation of
allocatable locals) are not Fortran 90; question 2 asks whether they make an
optional last phase (Phase 112).

## What govax already has

- **The whole FORTRAN-77 compiler and DEC's extensions**, written to teach:
  `internal/fortran`'s passes (`source`, `lex`, `parse`, `sema`, `ir`, `opt`,
  `codegen`), about 50,000 lines of Go, and FTNRTL (`internal/ftnrtl`, about
  12,000 lines of MACRO). FORTRAN-INTERNALS.md, 23 sections.
- **Pieces of Fortran 90 DEC added before it:** `!` comments, IMPLICIT NONE,
  DO WHILE, END DO, DO without a label, INCLUDE, 31-character names with `$`
  and `_`, NAMELIST (in DEC's `$group` form), STRUCTURE and RECORD (DEC's
  derived types, selected with `.`), AUTOMATIC (a variable in the frame),
  `*n` lengths, and from Phase 89 RECURSIVE and RESULT.
- **Class A descriptors** (`ir.Descriptor`, `ir.Bound`; `ir/lowerbounds.go`):
  adjustable arrays are described by a class A descriptor in a cell, and an
  element is reached through its A0 (`ir.Mem.Ptr`). This is the groundwork for
  the NCA descriptors of Decision 7: an assumed-shape dummy is an adjustable
  array whose bounds and strides come from the caller.
- **The frame** (Phases 82 and 89): AUTOMATIC variables and a recursive
  unit's storage live in the stack frame, reached from FP; `ir.Routine.NewCell`
  is the one constructor. Automatic arrays and array temporaries (Decision 8)
  extend it to sizes known only at run time.
- **Dynamic strings** (Phase 78): class D strings from LIB$SGET1_DD, freed at
  a statement's end (`lowerer.after`). Character results of run-time length
  (TRIM, REPEAT) use the same machinery.
- **The heap**: LIB$GET_VM and LIB$FREE_VM (`internal/vmheap`, Phase 59), for
  ALLOCATE and DEALLOCATE.
- **Loops in the IR** (`ir.CountedLoop`) and an optimizer that knows them
  (Phase 84): the loops array expressions become are optimized like DO loops,
  with no new optimizer pass needed to start with.
- **The test harness**: `TestFortranPrograms` runs each corpus program both
  `/OPTIMIZE` and `/NOOPTIMIZE` (and `/RECURSIVE`, Phase 89), in both paging
  modes for some; `testdata/fortran/gfortran/compare.py` plays the portable
  ones on gfortran.

## Where Fortran 90 and VAX FORTRAN disagree

Most of Fortran 90 is new syntax that VAX FORTRAN rejects today, so accepting
it changes no existing program. A few texts mean one thing to each:

| Text | VAX FORTRAN reads | Fortran 90 reads | Settled by |
|---|---|---|---|
| `"17` | an octal INTEGER constant (the 1988 manual's A.5) | the start of a character constant | `/F90` (Decision 2) |
| `TYPE FMT` (no comma, no list) | output with the format in FMT, an empty list | the start of a derived type FMT's definition | `/F90`; `TYPE *,…`, `TYPE 10,…`, and `TYPE f, list` are always output, `TYPE, …`, `TYPE ::`, and `TYPE(name)` always Fortran 90's (the HP manual's 10.6 keeps both) |
| `X = SUM(A)` with SUM not declared | a call of an external function SUM | the intrinsic SUM | `/F90`: Fortran 90's new intrinsic names are intrinsic only under it, or by an INTRINSIC statement (Decision 2) |
| `A.B.C` with A a record | DEC's field B, then its field C | A, the defined operator `.B.`, C | the symbols: a field of a record wins, as in the HP manual; settled in sema, where the parser keeps both readings possible |
| `ENDDO`, `INTEGERX` in free form | (fixed form ignores blanks) | blanks are significant: `INTEGERX` is a name | the source form, not `/F90` (Decision 3) |

Nothing else found: `/=`, `(/`, `=>`, `::`, `%name`, `;`, construct names
(`OUTER: DO`), and the new statements are all errors in VAX FORTRAN today.
`%VAL(`, `%REF(`, `%DESCR(`, `%LOC(`, and STRUCTURE's `%FILL` start with `%`
where no component selector can stand, so they stay as they are. A survey
subtask in each later phase looks again for its own features.

## What changes in each pass

The FORTRAN program's design (`PHASE-74`'s Part B) holds; Fortran 90 makes
these structural changes, each explained where it is made:

- **source.** A second reader for free form (`source/free.go`): continuation by
  `&`, `;` splitting, `!` comments, and a record of where the blanks were, which
  fixed form never needs. An INCLUDE'd file has its own form, by its type
  (Decision 3).
- **lex.** Classification still works on the statement with its blanks taken
  out (`PHASE-74`'s Decision 5), since Fortran 90's keywords written with
  blanks (`END DO`, `ELSE IF`, `GO TO`, `SELECT CASE`, `DOUBLE PRECISION`,
  `IN OUT`) squeeze to the forms it knows. In free form, the tokenizer then
  refuses a name or a number with a blank inside it. New statement kinds
  (about 35) join `kinds.go`'s table, each recognized and reported NOTIMPL by
  name until its phase. Classification learns `::` (a declaration, whatever
  else follows), `=>` (a pointer assignment, unless a keyword with a list,
  such as USE's renames, came first), and WHERE as it knows the logical IF.
- **ast and parse.** Program units nest: a unit's CONTAINS part holds internal
  procedures, a module holds module procedures, an interface block holds
  interface bodies. The AST's flat unit list becomes a tree of scoping units.
  Expressions gain sections (`A(1:N:2)`), array constructors, components
  (`X%Y`), keyword arguments (`F(A, DIM=1)`), and defined operators.
- **sema** changes most. Today "FORTRAN has no nested scopes" (`Analyze`'s
  comment): one symbol table per unit. Fortran 90 has **scoping units** inside
  each other and **host association** (an internal procedure sees its host's
  names), **use association** (a module's public names), **types with kinds and
  ranks**, explicit interfaces checked at each call, generic resolution by type,
  kind, and rank, and **shapes** (every expression has a rank, and a shape known
  at compile time or at run time). Phase 100 introduces the scope chain and the
  kind, Phase 102 the shape.
- **ir.** Array expressions are lowered to loops by a **scalarizer**
  (`ir/lowerarray.go`, Decision 6): a whole-array assignment becomes a nest of
  `CountedLoop`s whose body is the element-wise statement, with a temporary when
  the two sides may overlap. Dynamic arrays and pointers are reached through
  descriptors (Decision 7), whose fields the IR loads like any other memory.
- **opt.** No new pass is needed at first: the scalarizer's loops are the loops
  the optimizer already knows. The analyses learn Fortran 90's aliasing rules
  (`opt/effects.go`): a POINTER may alias any TARGET or pointer of its type, and
  nothing else; a dummy may not alias another it is changed through (Phase 107).
- **codegen.** Little changes: descriptors' fields are memory operands, loops
  are loops, and the frame grows by a run-time amount with `SUBL2 Rn,SP`
  (Decision 8). Module procedures' and variables' names follow Decision 10.
- **FTNRTL** gains routines that work on descriptors: the transformational
  intrinsics that move whole arrays (RESHAPE, PACK, CSHIFT, ...), ALLOCATE's
  and DEALLOCATE's bookkeeping and errors, nonadvancing I/O, the new edit
  descriptors, and I/O items described by NCA descriptors. New entries at the
  transfer vector's end, GSMATCH's minor id raised with each (`PHASE-74`'s
  Decision 13).
- **The debugger** (`/DEBUG`'s table, `debugger/fortran.go`) learns `%`,
  arrays described at run time, allocation and association status, module
  variables, and an internal procedure's host's names (Phase 110).

# Part B — decisions (proposed)

**Decision 1 — the dialect is Fortran 90 joined to VAX FORTRAN V5.0.** The
compiler accepts ISO/IEC 1539:1991 and everything it accepts today, in one
program if the author wishes: a module may hold a STRUCTURE, a derived type's
component may be selected with `.`, a free-form program may use `%VAL`. Where
the standard and the 1988 manual disagree, the table above says which reading
is taken. INTEGER kinds are 1, 2, and 4: there is no INTEGER(8), the VAX
having no 64-bit integer arithmetic (SELECTED_INT_KIND(10) is -1, as the
standard allows; question 9). A feature not yet compiled is
`%FORT-E-NOTIMPL`, naming it, never wrong code (`PHASE-74`'s Decision 19).

**Decision 2 — `/F90` chooses Fortran 90's reading where the two
disagree, and nothing else.** Fortran 90's new syntax is accepted with or
without it, since VAX FORTRAN gives that syntax no meaning. `/F90` changes
only the rows of the table above settled "by `/F90`":

- `"` starts a character constant (and the octal `"17` is gone: write `O'17'`
  or `'17'O`);
- `TYPE name` with nothing after it starts a derived type's definition;
- Fortran 90's new intrinsic procedures (SUM, SIZE, TRIM, ... : the names not
  in the 1988 manual's Table D-3) are intrinsic. Under `/NOF90` they are
  intrinsic only when an INTRINSIC statement names them, so an old program's
  own SUM or TRIM is still called.

**`/F90` is the default for a free-form source** (Decision 3) and `/NOF90` for
a fixed-form one, so an old `.FOR` file compiles as it does today, and a
`.F90` file is Fortran 90. Each can be given explicitly, and an OPTIONS
statement may say `/[NO]F90` as it says `/[NO]F77` (question 1). `/F77` and
`/F90` are independent: `/NOF77` is FORTRAN-66's reading of DO and EXTERNAL,
which Fortran 90 shares with FORTRAN-77.

**Decision 3 — source form by file type, or `/SOURCE_FORM`.** A file whose type
is `.F90` is free form; any other (`.FOR`, `.F`, ...) is fixed form, with the
tab format and `/EXTEND_SOURCE` as today. `/SOURCE_FORM=FREE` or `=FIXED`
overrides the type (the HP compiler's qualifier). A file with no type is looked
for as `.FOR`, then `.F90` (question 1). An INCLUDE'd file has the form its own
type gives it, not its includer's (the HP manual's rule), and `INCLUDE
'(module)'`'s text library modules are fixed form, as govax's definition
modules are written. Free form follows the standard's 3.3 exactly: 132
characters, `!` anywhere outside a character context, `&` at a line's end
continues it and an optional `&` starts the continuation, `;` separates
statements, 39 continuation lines by default (`/CONTINUATIONS=n` still
counts), and **blanks are significant**: a name, a keyword, or a constant may
not contain one, and adjacent names and keywords need one. DEC's directives in
free form are written `!DEC$`, the form the HP manual gives for CDEC$ in free
form; fixed form takes both.

**Decision 4 — scoping units, and one symbol table per scope.** sema's
per-unit table becomes a table per scoping unit (a main program, an external
or module or internal subprogram, a module, an interface body, a derived type's
definition), each with a pointer to its host. A name is looked up in its own
scope, then (by host association) its host's, then in the names USE brought
in; the standard's 12.1.2.2.1 says which names a scope's own declarations hide.
The rule is one function, `scope.lookup`, so a reader can find it, and the
cross reference (Phase 85) names a mention's scope. Implicit typing is per
scope and inherited from the host.

**Decision 5 — types carry a kind and a rank; shapes come from sema.** `sema.Type`
keeps its kind and byte size (the byte size *is* the kind number, Decision 1);
a type also names its derived type when it is one (`Type.Derived`, joining the
STRUCTURE machinery, Decision 9). Every expression gets a **rank** and a
**shape**: a list of extents, each a constant or an expression to evaluate at
run time. Conformance is checked when both shapes are known, at compile time;
otherwise under `/CHECK=BOUNDS` (now meaning shapes as well), at run time.

**Decision 6 — array expressions are lowered by scalarization.** An array
assignment or array-valued expression becomes a loop nest over its shape in
`ir` (`lowerarray.go`), whose body is the scalar statement at one element, with
the leftmost subscript varying fastest (array element order), so memory is
walked in order. Rules:

- **Fusion:** every elemental operation and elemental intrinsic in one
  assignment shares one loop nest; no temporary is made for `A = B + C * D`.
- **Overlap:** if the right side may refer to storage the left side changes at
  a different element (`A(2:N) = A(1:N-1)`), the right side is first computed
  into a temporary. The test is conservative and in one place
  (`sema/overlap.go`): same base, sections whose ranges can be proven disjoint
  or identical, and Fortran 90's rules for dummies and pointers.
- **Transformational intrinsics** (MATMUL, SUM with DIM=, ...) and array-valued
  function references are computed into a temporary before the statement, then
  used elementwise.
- **WHERE** is a mask computed into a LOGICAL temporary, then a loop per
  assignment testing it, as the standard's 7.5.3.2 orders.
- Reductions with no DIM= (SUM(A), MAXVAL(A), ANY(A > 0)) are a loop
  accumulating into a scalar, in line, so `/OPTIMIZE` can keep it in a
  register.

**Decision 7 — dynamic arrays are described by VMS NCA descriptors.** An
array whose shape or place is known only at run time (an assumed-shape dummy,
an allocatable array, an array pointer, a section passed to an assumed-shape
dummy) is reached through a class NCA descriptor (the Calling Standard's 5.7):
the prototype block (length, DTYPE, class 10, the address, DIMCT, ARSIZE), the
virtual origin A0, a stride in bytes for each dimension, and each dimension's
bounds. An element's address is A0 plus the sum of each subscript times its
stride, which is what `ir.Mem.Ptr` computes for adjustable arrays today with
extents in place of strides. Because NCA is VMS's own form:

- a Fortran 90 routine with an assumed-shape dummy can be called from MACRO,
  and the debugger can read the descriptor as VMS's would;
- an allocatable array is its descriptor, POINTER 0 when unallocated (so
  ALLOCATED is a test of one longword), in the variable's place: static, or in
  the frame for a recursive unit's;
- an array pointer is the same, POINTER 0 when disassociated; a pointer to a
  section is a descriptor with the section's strides and bounds, never a copy;
- a whole contiguous array passed to an old-style dummy is still passed by
  reference, and a CHARACTER one by class A descriptor, so calls between old
  and new code are as before. A section or assumed-shape array passed to an
  explicit-shape or assumed-size dummy is copied into a contiguous temporary
  and back (the standard's 12.4.1.1 allows it; a VAX program can't know).

How a dummy is passed is decided by the callee's interface, so a routine with
an assumed-shape, pointer, or optional dummy needs an explicit interface at
each call, as the standard says (12.3.1.1); a call without one is an error the
compiler reports when it can see both (same file), and undefined otherwise.

**Decision 8 — storage of run-time size goes in the frame, or on the heap.**
Automatic arrays (`REAL W(N)` with N a dummy) and the scalarizer's temporaries
are made in the stack frame (`SUBL2 size,SP`, aligned, addressed from a cell
holding their place), and given back when the routine returns (temporaries:
when the statement ends, by restoring SP). That is cheap and survives an
unwind, which a heap block would not. A temporary larger than a limit (the
first choice: 64 KB) comes from LIB$GET_VM and is freed at the statement's end,
so a big array expression doesn't exhaust the user stack (2 MB with paging off,
Phase 89's subtask 9). ALLOCATE takes memory from LIB$GET_VM through FTNRTL
(`FTN$ALLOCATE`, which sets STAT= or signals FOR's new conditions), DEALLOCATE
gives it back. An allocatable local that isn't SAVE'd is deallocated when the
routine returns: Fortran 95's rule, which Fortran 90 leaves undefined, chosen
so a loop calling a routine doesn't leak (question 4).

**Decision 9 — derived types and STRUCTURE share one model.** A derived type
is a `sema.Structure` (Phase 82's) with a name, its components laid out in
order. A SEQUENCE type, and every type under `/ALIGN=NONE`, is packed as
STRUCTURE is; otherwise components are aligned as `/ALIGN=RECORDS` would align
a STRUCTURE's fields (natural alignment, a choice: the standard leaves layout to
the processor). `%` and `.` select components of either. A record and a
derived-type object are passed by reference as before. Structure constructors,
intrinsic assignment (a component at a time where a component is a pointer or
allocatable, a byte move otherwise), and a derived type's I/O (a component at
a time, in order) are Phase 105's.

**Decision 10 — a module is compiled to a MACRO file and a module file.**
Compiling `MODULE M` writes, beside the `.MAR` file, **`M.F90$MOD`**: govax's
own text form of everything a USE of M needs (its public and private
entities' declarations, interfaces, derived types, generic names, and the
modules it uses in turn), written to be read by a person as well as the
compiler, with a version line (question 3). The HP compiler's file type is
kept; its contents are ours. USE M finds the file in the directory of the
source being compiled, then each directory of `/MODULE_PATH=(...)` (or the
logical name `FORT$MODULES`), then the directory `/MODULE=dir` writes into.
Several modules and the program using them may be in one file, in any order the
standard allows (a module before its first use). In the object:

- **module variables** are one psect per module, `$MOD$name`, laid out as a
  COMMON block is (overlaid, global; `PHASE-74`'s Table 10-1 attributes), its
  initial values in the module's own object only. A user of the module needs no
  global data symbols, and the debugger reads the psect as it reads COMMON;
- **module procedures** have global entry names `M.P` (a period, which MACRO
  and LINK allow and no Fortran name contains, so a module procedure never
  collides with an external one); a name past 31 characters gets a stable
  form from a hash of the full name, listed in the storage map;
- **internal procedures** are local to their host's module: `host.$Pn` names,
  never global.

**Decision 11 — internal procedures reach their host's frame by a hidden
argument.** An internal procedure of a host whose variables are static reaches
them by their names, as any unit reaches `$LOCAL`. If the host's variables are in
its frame (a recursive host, AUTOMATIC variables, automatic arrays), every call
of the internal procedure passes the host's FP as a hidden last argument, and the
procedure reaches the host's frame through it, as variable format expressions
reach theirs (Phase 89's `$FP`, now R11 in FTNRTL's calls). Fortran 90 forbids
passing an internal procedure as an actual argument, so the hidden argument is
always there when it is needed.

**Decision 12 — POINTER and TARGET.** A scalar pointer is a longword
address (0 when disassociated), a CHARACTER pointer a class S descriptor, an
array pointer an NCA descriptor (Decision 7). Fortran 90 says a pointer's
association status is undefined until it is set; govax's static storage starts at
zero, so a pointer starts disassociated, and in the frame the compiler clears it
(Fortran 95's NULL() initialization, in effect: a choice). The optimizer may
assume a POINTER aliases only a TARGET or another POINTER of its type, and that a
non-TARGET variable is changed only through its name (Phase 107).

**Decision 13 — explicit interfaces, generics, and operators are resolved at
compile time.** A reference to a generic name chooses the one specific
procedure whose dummies match the actuals in type, kind, and rank (the
standard's 14.1.2.3, its rules for ambiguity checked at the interface block),
in sema, before lowering; a defined operator or assignment becomes a call of
its procedure. Keyword arguments are reordered into the dummies' order; an
absent OPTIONAL argument is passed as a 0 longword, VMS's null argument, so
PRESENT is a test of the argument list's longword and a FORTRAN-77 routine
called with a null argument sees the same thing it sees today.

**Decision 14 — the intrinsic table grows by rows, not code.** Fortran 90's
intrinsics join `sema/intrinsics.go`'s table (Table D-3's rows today) with
three new columns: **class** (elemental, inquiry, transformational, or
subroutine), the **dummies' names** (for keyword arguments: `DIM=`, `MASK=`,
`KIND=`, `BACK=`), and which arguments are **optional**. An elemental intrinsic
applied to an array is the scalarizer's (Decision 6); an inquiry is answered at
compile time where the answer is known (KIND, HUGE, SIZE of an explicit-shape
array) and from the descriptor otherwise; a transformational one is in line
(reductions, DOT_PRODUCT, MATMUL on small known shapes) or an FTNRTL routine
taking NCA descriptors (RESHAPE, PACK, CSHIFT, ...: the phase's survey decides
each). Numeric results come from `vaxfloat` and MTHRTL, as today.

**Decision 15 — FTNRTL grows at its vector's end.** New routines (ALLOCATE's,
the array movers, nonadvancing I/O, the new edits, NCA items) are new MACRO
modules (`ftn_alloc.mar`, `ftn_array.mar`, ...) and new entries at the end of
the transfer vector, GSMATCH's minor id raised with each, so an image linked
against an older FTNRTL still runs. New conditions (for ALLOCATE's and pointer
errors) take FOR message numbers past VAX FORTRAN's, in govax's own wording
(`PHASE-74`'s Decision 15), listed in MANUAL.md.

**Decision 16 — testing.** As the FORTRAN program's (`PHASE-74`'s Decision 20),
with these additions:

- **A Fortran 90 corpus**, `testdata/fortran90/`, free-form `name.f90` files
  with `name.out` (and `name.in`) worked by hand from the standard, run by a new
  `TestFortran90Programs` beside `TestFortranPrograms`, in every mode the old
  corpus runs: `/OPTIMIZE` both ways, `/RECURSIVE` both ways, both paging
  modes for a subset. Modules of several files in `testdata/fortran90/modules/`,
  each built in every order its `links.txt` allows.
- **The old corpus unchanged**: every `testdata/fortran` program, DUNGEON, and
  the oracles must give the same MACRO file under `/NOF90` as before each
  subtask, byte for byte but for the compiler's version line. A test holds
  that (`TestNoF90Unchanged`), so the promise that nothing old changes is
  checked, not hoped.
- **gfortran as a witness:** each portable corpus program is compiled with
  `gfortran -std=f95` (gfortran has no `-std=f90`; Fortran 95 features are kept
  out by review) and its output compared, by an extension of `compare.py`. A
  difference is read against the standard and recorded; neither compiler is
  assumed right.
- **Each pass's golden tests** grow: free-form source, tokens, AST dumps of
  every new construct, sema dumps with scopes and shapes, IR dumps of
  scalarized loops.
- **An optional VMS round** at the milestone: VMS 7.1's MACRO and LINK build
  FTNRTL and the Fortran 90 corpus from govax's `.MAR` files and run them, as
  `testdata/ftn86` did (Phase 111).

**Decision 17 — the code and the docs teach.** Each new file's comments
explain what the standard asks and why the compiler does it this way, with the
standard's section and syntax rule. Concepts new to a reader who knows
FORTRAN-77 get their own explanation where they are implemented: scopes and
host association, rank and shape, scalarization and why overlap needs a
temporary, descriptors and strides, generic resolution, module files and
separate compilation, aliasing. `docs/FORTRAN-INTERNALS.md` gains a section per
phase (24 onward, one subject each); MANUAL.md gains section 14.13, "Fortran
90", with a subsection per phase, and its 14.7 the differences. `HELP FORTRAN`
documents each new qualifier. CLAUDE.md's overview and PLAN.md grow as each
phase ends.

**Decision 18 — what the program finds missing elsewhere, it fixes**, as
`PHASE-74`'s Decision 21: MACRO, LINK, LIBRTL, the debugger, and RMS, each fix
with a test and a line in that phase's log.

## A sketch of what the compiler will write

For this free-form program:

```fortran
program squares
  implicit none
  integer, parameter :: n = 5
  integer :: i
  integer :: sq(n)
  sq = (/ (i*i, i = 1, n) /)
  print *, sum(sq), maxval(sq)
end program squares
```

lowering would give, in outline (the details are Phase 102's):

```
;      6    sq = (/ (i*i, i = 1, n) /)
; scalarized: one loop over the constructor's implied DO, storing as it goes
        MOVL    #1,SQUARES.I            ; i = 1
SQUARES.$1:
        MOVL    SQUARES.I,R1            ; the element's index
        MULL3   R1,R1,SQUARES.SQ-4[R1]  ; sq(i) = i*i
        AOBLEQ  #5,SQUARES.I,SQUARES.$1 ; the next i, up to n
;      7    print *, sum(sq), maxval(sq)
; SUM and MAXVAL: one loop each, in line, accumulating in a temporary
        CLRL    SQUARES.$T1             ; the sum so far
        ...
```

(Fortran 2003's `[ ]` spelling of a constructor is not planned.)

# Part C — the phases

| Phase | Subject | Builds on |
|---|---|---|
| **99** | The plan (this doc); `/F90` and `/SOURCE_FORM`; free source form; Fortran 90's lexical items; the new statements recognized; named END statements; the Fortran 90 corpus and its harness | 74–89 |
| **100** | Declarations and kinds: `::` declarations, attributes, initialization, KIND= and LEN=, kinds on constants, the kind and numeric inquiry intrinsics, IMPLICIT with kinds; sema's scope chain (one scope per unit until Phase 103) | 99 |
| **101** | Control constructs: SELECT CASE, DO with no control, EXIT, CYCLE, construct names | 100 |
| **102** | Arrays with shapes known at compile time: sections, vector subscripts, whole-array expressions and assignment, the scalarizer, overlap and temporaries, array constructors, array PARAMETERs, WHERE, elemental intrinsics on arrays, the reductions and inquiries, sections in I/O lists (an element at a time) | 101 |
| **103** | Procedures: internal procedures and host association, interface blocks, explicit interfaces and their checks, keyword and optional arguments, PRESENT, INTENT, generic names, defined operators and assignment | 102 |
| **104** | Dynamic arrays: NCA descriptors, assumed-shape dummies, sections passed by descriptor or copied, automatic arrays, ALLOCATABLE, ALLOCATE, DEALLOCATE, array-valued functions, the transformational intrinsics on run-time shapes | 103 |
| **105** | Derived types: definitions, components, `%`, constructors, SEQUENCE, assignment, derived types as dummies and results, in arrays and sections, in COMMON and EQUIVALENCE, I/O and NAMELIST of them | 104 |
| **106** | Modules: MODULE, USE (ONLY, renames), PUBLIC and PRIVATE, module data and procedures, generic interfaces and operators in modules, module files, separate compilation, the FORTRAN command's module qualifiers | 105 |
| **107** | Pointers: POINTER, TARGET, `=>`, NULLIFY, ASSOCIATED, ALLOCATE of pointers, array pointers and pointers to sections, pointer components, pointer dummies and results, the optimizer's aliasing rules | 106 |
| **108** | The rest of the intrinsic procedures: character, numeric, the model, bits, TRANSFER, date and time, random numbers, character results of run-time length | 104 |
| **109** | I/O: nonadvancing I/O, the new OPEN and INQUIRE specifiers, B, EN, ES, and G for every type, Fortran 90's NAMELIST form, sections and derived types as I/O items by descriptor | 105 |
| **110** | Usability: `/STANDARD=F90` and the obsolescent features, `/WARNINGS`, listings and cross references with scopes and modules, `/CHECK` on shapes and pointers, `/DEBUG` and the debugger on Fortran 90's data | 107–109 |
| **111** | The milestone and close-out: a substantial Fortran 90 program in every mode, gfortran's witness, the optional VMS round, the manual's 14.13, FORTRAN-INTERNALS.md's sections finished | all |
| *112* | *Optional, by question 2:* Fortran 95's additions | 111 |

Each later phase has a skeletal planning doc of its own: its goal, what
earlier phases leave in place, its design notes, its open questions, and its
provisional subtasks. Each starts with a survey subtask that may change them,
reviewed before implementation, as Phases 76 to 89 did.

- [PHASE-100 - Fortran 90 declarations and kinds](PHASE-100%20-%20Fortran%2090%20declarations%20and%20kinds.md)
- [PHASE-101 - Fortran 90 control constructs](PHASE-101%20-%20Fortran%2090%20control%20constructs.md)
- [PHASE-102 - Fortran 90 array expressions](PHASE-102%20-%20Fortran%2090%20array%20expressions.md)
- [PHASE-103 - Fortran 90 procedures and interfaces](PHASE-103%20-%20Fortran%2090%20procedures%20and%20interfaces.md)
- [PHASE-104 - Fortran 90 dynamic arrays](PHASE-104%20-%20Fortran%2090%20dynamic%20arrays.md)
- [PHASE-105 - Fortran 90 derived types](PHASE-105%20-%20Fortran%2090%20derived%20types.md)
- [PHASE-106 - Fortran 90 modules](PHASE-106%20-%20Fortran%2090%20modules.md)
- [PHASE-107 - Fortran 90 pointers](PHASE-107%20-%20Fortran%2090%20pointers.md)
- [PHASE-108 - Fortran 90 intrinsic procedures](PHASE-108%20-%20Fortran%2090%20intrinsic%20procedures.md)
- [PHASE-109 - Fortran 90 IO](PHASE-109%20-%20Fortran%2090%20IO.md)
- [PHASE-110 - Fortran 90 compiler usability](PHASE-110%20-%20Fortran%2090%20compiler%20usability.md)
- [PHASE-111 - Fortran 90 milestone](PHASE-111%20-%20Fortran%2090%20milestone.md)

Each subtask is one commit with its tests, and the work stops for review after
each.

**Why this order.** Each phase gives programs that run end to end, and each
uses only what came before: kinds and declarations first, since every later
feature declares with `::`; control constructs early, since they are small and
make the corpus's programs natural; static arrays before procedures, so the
scalarizer is built and tested before descriptors complicate it; procedures and
interfaces before dynamic arrays, because an assumed-shape dummy needs an
explicit interface; derived types before modules, because modules mostly hold
types and their procedures; pointers after both, because their best uses are
pointer components of derived types in modules. The intrinsics and I/O phases
need only dynamic arrays and derived types, so they can move earlier or later as
the work shows.

## Phase 99 — subtasks (this phase)

1. **The survey and plan** (this doc; the skeletal docs of Phases 100 to 111;
   PLAN.md's rows; the Handbook's text) (done 2026-11-17; accepted by the
   author the same day).
2. **`/F90` and `/SOURCE_FORM`.** The qualifiers in `console.dcl`, `govax
   fortran`, `fortran.Options`, and the OPTIONS statement's `/[NO]F90`; `.F90`
   as a source type (the command's default types, INCLUDE's, the FORTRAN
   command's file search, Decision 3); the form chosen per file and per
   INCLUDE'd file; `HELP FORTRAN`. Nothing reads `/F90` yet but the
   `"` rule of subtask 4; `TestNoF90Unchanged` (Decision 16) starts here, with
   the old corpus's MACRO files as its baseline.
3. **Free source form** (`source/free.go`): the standard's 3.3 (lines, `!`
   comments, `&` continuation on both lines, a character constant continued,
   `;`, 39 continuations, `!DEC$` directives, labels as the first token), each
   statement with the positions of its blanks for the tokenizer; messages for
   a missing `&`, too many continuations, a `;` alone, a line past 132
   characters. Golden tests of statements and positions, both forms side by
   side, and the old fixed-form tests untouched.
4. **Fortran 90's lexical items** (`lex`): the new tokens (`::`, `=>`, `%`,
   `(/`, `/)`, `==`, `/=`, `<`, `<=`, `>`, `>=`, defined operators), kind
   parameters on constants (`1.5_8`, `2_k`, `.TRUE._1`, `k_'text'`), `"`
   character constants under `/F90`, `B'…'`, `O'…'`, `Z'…'` (and DEC's
   `'…'O` and `'…'X` as now), and in free form a blank inside a name or
   constant an error (`BLANKINNAME`). The relational spellings parse as
   `.EQ.` and the rest, so a program using `==` already compiles and runs.
5. **The new statements recognized** (`lex/kinds.go`, `parse`, `sema/notimpl.go`):
   every Fortran 90 statement classified (the `::`, `=>`, and WHERE rules;
   TYPE's two readings; construct names before a colon), parsed far enough to
   report `%FORT-E-NOTIMPL, the SELECT CASE statement is not implemented yet`
   at the right place, and `TestEveryKindParses` extended. Named END
   statements (`END PROGRAM squares`, `END SUBROUTINE`, `END FUNCTION`) and
   their name checks are compiled here: they need nothing else.
6. **The Fortran 90 corpus and first light**: `testdata/fortran90/` with its
   README and `TestFortran90Programs` (every mode, Decision 16); first programs
   that are FORTRAN-77 in free form using Fortran 90's spellings (`==`, `"`,
   `!`, `;`, named END, `&`), and the gfortran witness extended to the new
   corpus.
7. **The close-out**: FORTRAN-INTERNALS.md's section 24 ("Fortran 90: two
   source forms and a bigger alphabet"), MANUAL.md's 14.13 begun (the
   qualifiers, free form), HELP, CLAUDE.md's overview, PLAN.md.

## Not planned

- Fortran 95 and later, unless question 2 makes Phase 112 (FORALL, PURE,
  ELEMENTAL, NULL(), CPU_TIME, default component initialization, automatic
  deallocation beyond Decision 8's).
- Fortran 2003's spellings that compilers commonly accept (`[ ]` array
  constructors, ALLOCATABLE components and dummies, `ISO_C_BINDING`,
  `IMPORT`, `PROCEDURE` pointers).
- INTEGER(8) (Decision 1; question 9).
- DEC's later extensions in the HP manual beyond Fortran 90 and what VAX
  FORTRAN had (`!DEC$ ATTRIBUTES`, Cray-style POINTER statements, `INTEGER*8`).
  VOLATILE, AUTOMATIC, and STATIC written as attributes after `::` are in
  (Phase 100): they are DEC's statements in Fortran 90's form.
- An object file writer or real DEC debugger records (as `PHASE-74`'s Future
  work).

## Questions for the author

1. **`/F90`'s reach (Decision 2).** The plan accepts Fortran 90's new syntax
   always, and lets `/F90` decide only the three conflicts (`"`, `TYPE name`,
   the new intrinsic names), defaulting on for free form and off for fixed
   form. The alternatives: (a) everything Fortran 90 only under `/F90`, so a
   `/NOF90` compilation is exactly today's language; or (b) `/F90` on by
   default everywhere. **Recommendation:** the plan's way: it is what DEC did,
   it keeps old programs unchanged (checked by `TestNoF90Unchanged`), and it
   lets an old fixed-form program adopt one Fortran 90 feature at a time. Also:
   is a file with no type looked for as `.FOR` then `.F90` (the plan), or
   `.FOR` only?
2. **Fortran 95.** gfortran, the HP manual, and most Fortran 90 code written
   since 1997 assume Fortran 95's small additions (FORALL, PURE and ELEMENTAL,
   NULL(), CPU_TIME, default initialization of components, automatic
   deallocation). **Recommendation:** an optional Phase 112 after the
   milestone, decided then; nothing before it depends on them.
3. **Module files (Decision 10).** Our own readable text format, named
   `M.F90$MOD` as the HP compiler names them, beside the `.MAR` file, with
   `/MODULE=dir` and `/MODULE_PATH=(...)` (or a logical name). Is the type
   `.F90$MOD` right, and is a readable text file preferred to a compact one?
   **Recommendation:** text, for teaching and diffing.
4. **Allocatable locals at return (Decision 8).** Deallocate them (Fortran
   95's rule), or leave them allocated and unreachable (Fortran 90's
   "undefined")? **Recommendation:** deallocate.
5. **Descriptors (Decision 7).** VMS's class NCA descriptor for every
   run-time array, so MACRO and the debugger can read them. The alternative is
   a dope vector of our own design. **Recommendation:** NCA.
6. **Layout of a non-SEQUENCE derived type (Decision 9).** Natural alignment
   by `/ALIGN=RECORDS`'s rules, or packed as STRUCTURE is by default?
   **Recommendation:** natural alignment, as the HP compiler's default; SEQUENCE
   types packed.
7. **A milestone program (Phase 111).** Is there a Fortran 90 program the
   author would like as the milestone, as DUNGEON was for FORTRAN-77 (license
   permitting)? Without one, the plan writes its own: a set of programs
   exercising each phase together (a module-based linked list and tree with
   pointers, a matrix library with assumed-shape arrays and generics, a text
   processor with the character intrinsics and nonadvancing I/O), each checked
   on gfortran.
8. **Phase numbering and size.** Thirteen phases, 99 to 111, about 90
   subtasks. Should any be merged (101 into 100, 108 into 102 and 104) or
   split further?
9. **INTEGER(8).** Out (Decision 1): the VAX has no 64-bit integer
   arithmetic, and VAX FORTRAN had no INTEGER*8. It could be done with
   EMUL/EDIV and ADWC/SBWC sequences if wanted. **Recommendation:** out.

## The author's answers (2026-11-17)

The author read the plan and accepted every recommendation:

1. `/F90`'s reach as planned (Decision 2): new syntax always, `/F90` settling
   only the three conflicts, on by default for free form; a file with no type
   looked for as `.FOR`, then `.F90`.
2. Fortran 95's additions are an optional **Phase 112**, decided at Phase
   111's close-out.
3. Module files are our own readable text, `name.F90$MOD` (Decision 10).
4. Allocatable locals are deallocated at return (Decision 8).
5. NCA descriptors for every run-time array (Decision 7).
6. Non-SEQUENCE derived types naturally aligned, SEQUENCE types packed
   (Decision 9).
7. **The milestone program:** the author will look for a Fortran 90 program;
   otherwise we write our own of enough complexity to exercise every phase.
   It isn't needed before Phase 111: each phase brings its own corpus. One
   found earlier (by about Phase 105, since real programs lean on modules,
   derived types, and allocatable arrays) is compiled as phases land, as
   DUNGEON was from Phase 84, to find gaps the corpus misses.
8. The thirteen phases stand as laid out.
9. INTEGER(8) is out (Decision 1).

## Progress log

- **2026-11-17:** Survey written (Part A), decisions proposed (Part B), and
  the phases laid out (Part C), with skeletal docs for Phases 100 to 111.
  The *Fortran 90 Handbook*'s text extracted beside its PDF (`Fortran 90
  Handbook.txt`); the standard's text was already there (`Fortran90.txt`).
  The NCA descriptor checked in the Calling Standard's 5.7 (Decision 7).
  For review before any implementation.
- **2026-11-17:** The author reviewed the plan and took every recommendation
  ("The author's answers"); Phase 112 (Fortran 95's additions) is optional,
  decided at Phase 111. Subtask 1 is done; subtask 2 is next.
