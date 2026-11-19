# Phase 105 — Fortran 90: derived types

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Needs Phase 104 (done). Subtask 1, the survey (Part A), done
2026-11-18 and accepted by the author ("The author's answers").

## Goal

Fortran 90's derived types (the standard's 4.4 and 6.1.2), on Phase 82's
STRUCTURE machinery (Decision 9):

- **definitions**: `TYPE POINT … END TYPE POINT`, components of every
  intrinsic type and kind, arrays, character, and other derived types;
  SEQUENCE; PRIVATE components (with Phase 106);
- **objects**: `TYPE(POINT) :: P, PATH(100)`; `%` and DEC's `.` both select a
  component, including through arrays (`PATH%X`, a section, and
  `PATH(I)%X`);
- **structure constructors** (`P = POINT(1.0, 2.0)`), in expressions,
  initialization, and PARAMETERs;
- **intrinsic assignment** and **equality** only by defined operators (Phase
  103's), as the standard has it;
- derived types as **dummies**, **function results** (through a hidden argument
  past a quadword, as COMPLEX*16 is), in **COMMON** and **EQUIVALENCE** (SEQUENCE
  types only), and in **DATA**;
- **I/O** of a derived-type object, a component at a time in order (formatted,
  list-directed, unformatted), and in **NAMELIST**.

## What earlier phases leave in place

- `sema.Structure` and `Field`, unions and maps, `/ALIGN` (`sema/align.go`), the
  IR's `fieldMem`, record moves (`moveRecord`), record arguments compared by form
  (`Structure.form`, RECORDARG); `testdata/fortran/itemlist.for`.
- Scopes (a type's definition is a scope for its components' names), explicit
  interfaces, arrays as values.

## Design notes

- **One model.** A derived type is a `Structure` with a name and the `Derived`
  mark; STRUCTURE's unions and maps aren't allowed in one, and a TYPE's SEQUENCE
  and alignment rules are checked where they differ. A RECORD and a TYPE object
  of the same layout are still different types for argument checking (the
  standard's type equality, 4.4.2: same definition, or SEQUENCE types with the
  same name and components).
- **Layout** (Phase 99's question 6): SEQUENCE packed by `/ALIGN`'s rules for
  STRUCTURE, other types naturally aligned; the storage map shows every
  component's offset.
- **A section of components** (`PATH%X`) is a section with a stride of the
  type's size: an NCA descriptor (Phase 104), not a copy.
- **Teaching:** FORTRAN-INTERNALS.md compares STRUCTURE and TYPE, and shows why a
  component section is just another stride.

## Open questions

1. Should a derived type and a STRUCTURE of the same name and fields be
   interchangeable as arguments? Proposed: no (the standard's rule), with a
   clear message.

## Subtasks (provisional)

1. **Survey** (the standard's 4.4, 6.1.2, 7.5.1.5, 9.4.2; the Handbook's chapter
   4; the HP manual's derived types and records side by side), for review.
2. **Definitions and objects**; `%`; the storage map.
3. **Components of arrays** and arrays of components (sections by stride).
4. **Structure constructors**, initialization, PARAMETERs.
5. **Assignment, arguments, results**, type equality.
6. **COMMON, EQUIVALENCE, DATA** with SEQUENCE types.
7. **I/O and NAMELIST** of derived types (FTNRTL takes the components as items;
   the compiler expands them).
8. **Programs end to end**, every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 30, MANUAL.md.

# Part A — the survey (subtask 1)

Read for it: the standard's 4.4 (derived types: definitions, type equality,
values, structure constructors, operations), 5.1 and 5.2 (type declarations
with `TYPE(name)`, IMPLICIT with a derived type), 5.2.9 (DATA), 5.4 (NAMELIST),
5.5 and 14.6.3 (storage association: sequence types in COMMON and EQUIVALENCE),
6.1.2 (structure components), 7.5.1 (intrinsic and defined assignment), 9.4.2
(derived types in I/O lists), 10.9.1 (namelist input of derived types), and
12.3.2.1 (interface bodies and host association); the *Fortran 90 Handbook*'s
4.4 and 4.5; the HP Fortran manual's 3.3 (derived types, components,
constructors), 4.2.1.4 (derived-type assignment), 5.1.3, its COMMON and
EQUIVALENCE rules, 14's OPTIONS /ALIGN, and B.12 (record structures) beside
them; and what the lexer, parser, sema, the IR, FTNRTL, the storage map, and the
`/DEBUG` table do with STRUCTURE and RECORD today. There is no VMS probe (VAX
FORTRAN had no derived types); the standard is the target and gfortran 16.2,
`-std=f95`, the witness.

## A.1 What the standard asks

- **A definition** (4.4.1, R422–R429). `TYPE [[, access] ::] name`, then at most
  one SEQUENCE and at most one PRIVATE statement, then one or more component
  definitions, then `END TYPE [name]`. A component definition is a type
  statement, `type-spec [[, POINTER | DIMENSION(...)] ::] decl-list`, each decl
  `name [(bounds)] [*len]`; every bound and every CHARACTER length is a constant
  expression, so a type's size is known when compiling. A component's type is
  intrinsic or a derived type already defined (only a POINTER component may name
  the type being defined: Phase 107). An access-spec or PRIVATE is allowed only in
  a module's specification part (Phase 106). A type's name may be no intrinsic
  type's, and is a **local name of class 1** (14.1.2): it can't also name a
  variable, a constant, or a procedure of the scoping unit. Component names are
  local to the definition, so two types may both have an X.
- **SEQUENCE** (4.4.1). The order of the components is then a storage sequence;
  every derived-type component must itself be a sequence type. A sequence type
  whose ultimate components are all default numeric or logical is a *numeric
  sequence type*, all default CHARACTER a *character sequence type*. Without
  SEQUENCE the layout is the processor's.
- **Type equality** (4.4.2). Two entities have the same type if declared with one
  definition (reached by host association or, in Phase 106, from a module).
  Entities in different scoping units declared with *different* definitions have
  the same type only if both are SEQUENCE types with the same name and components
  agreeing in order, name, and attributes, none private. So until modules a
  non-SEQUENCE type can't be shared between external procedures at all, and
  since an **interface body has no host association** (12.3.2.1), an interface
  body can name a derived type only by defining it again, as a SEQUENCE type.
- **Components** (6.1.2, R612–R614). `parent % part [% part] ...`, each part
  with its subscripts or section subscripts; every part but the last must be of
  derived type. **At most one part may have nonzero rank**: `PATH%X` with PATH
  an array is an array of rank 1, `PATH(2:9:2)%X` a section, `T%COORD(1,:)` a
  section of an array component, `YEAR(J:K)%PARTS(I)` the HP manual's; but
  `YEAR%PARTS`, two arrays, is an error. A substring may follow the last part
  (`MY_CAR%MAKER(2)(4:10)`). A component has the INTENT, TARGET, or PARAMETER of
  its parent.
- **Structure constructors** (4.4.4, R430). `name(expr, ...)`: one value per
  component, in order, each converted as intrinsic assignment converts (Table
  7.9; a CHARACTER one padded or cut), an array component's value conforming in
  shape (a scalar conforms with any). A derived-type component's value is any
  expression of that type, usually a nested constructor. All constant makes a
  *derived-type constant expression*, usable as an initial value, in DATA, and as
  a named constant (`TYPE(POINT), PARAMETER :: ORIGIN = POINT(0., 0.)`). A
  constructor may not come before its type's definition.
- **Operations** (4.4.5, 7.5.1). There is **intrinsic assignment** between
  objects of one derived type (7.5.1.2(2)), component by component (pointer
  assignment for a pointer component: Phase 107). There is **no other intrinsic
  operation** on derived types, not even `==`: every operation, and any
  assignment between different types, is defined by an interface block (Phase
  103's). An ASSIGNMENT(=) interface for a type **replaces** its intrinsic
  assignment (7.5.1.2, 7.5.1.3).
- **COMMON and EQUIVALENCE** (5.5.1, 5.5.2). A derived-type object in COMMON must
  be of a sequence type; numeric and character sequence objects are as if their
  components were listed there. An EQUIVALENCE object may not be of a non-sequence
  type, nor of a sequence type with a pointer anywhere in it, nor be a structure
  component; numeric sequence objects equivalence only with default numeric ones,
  character sequence ones with default CHARACTER, other sequence types only with
  their own type.
- **DATA** (5.2.9). A value may be a structure constructor of constants (for an
  object of that type); an object may be a structure component
  (`DATA YOURNAME%AGE / 35 /`), in an implied DO too (`A(I)%X`), never of a
  constant parent.
- **I/O** (9.4.2). In a **formatted** statement (and list-directed), a derived-type
  item is as if its components were listed in order, recursively, an array item
  in array element order. In an **unformatted** statement it is one value in a
  processor-dependent form. No type with a pointer component (anywhere) may be an
  input item or an output value; no type with private components outside its
  module (Phase 106).
- **NAMELIST** (5.4, 10.9.1). A derived-type variable may be a group object
  (unless it has a pointer component). On input its name may be given whole, its
  values filling the ultimate components in order, or **a component's designator**
  (`P%X = 1.0`), qualified as deeply as its type allows. On output its values are
  its components'.
- **IMPLICIT** may name a derived type (`IMPLICIT TYPE(POSN) (A-B)`), and a
  FUNCTION may be typed by one (`TYPE(POINT) FUNCTION F(A)`, or by its result
  variable's declaration).
- **Fortran 95** adds default initialization of components (`INTEGER :: N = 0`
  in a definition): the HP manual marks it so, and it is outside Fortran 90.

## A.2 What govax has

- **STRUCTURE and RECORD** (Phase 82; `sema/structure.go`, `record.go`,
  `align.go`). A `Structure` is a name, `Fields` (each a `Type` or a
  substructure `Struct`, constant `Dims`, an `Offset`, its union and map),
  `Size`, `Natural` and `Boundary` (`/ALIGN=RECORDS=NATURAL` places each field on
  its boundary and pads `Size` to a multiple of `Boundary` so arrays of records
  stay aligned), and initial values (`Initials`, combined per record by
  `image`). Structures have their own table per unit, host-associated
  (`Unit.structure`; `scope.go`). A record is a `Symbol` with `Struct` set and
  `Type{Record, size}`: **the type itself doesn't say which structure**, only how
  long it is. `Structure.form` is a layout's signature (each field's place, type,
  and extents), which `recordArguments` compares for RECORDARG.
- **Field references.** The parser reads `R.F(I).G`, under `/F90` only after a
  record's name (`recordScope`, which already expects TYPE(t) names) or where no
  defined operator could start (`fieldNext`). sema's `FieldRef` is the record
  (or an element of an array of records) and a path of `FieldStep`s, each with
  **scalar** subscripts only (`fieldSubscripts` → `subscripts`); a field's
  parenthesized list with a colon is read as a **substring**, so `R.A(1:3)` of an
  array field is not a section (Phase 102's finding 4 left it here). The IR's
  `fieldMem` folds the constant parts of a path into one offset and adds the
  variable parts as an index or an address; `fieldArrayDescriptor` describes a
  whole array field (class A) for I/O and CHARACTER arguments.
- **What a record can do**: be assigned whole from a record of the same
  structure (`moveRecord`, a MOVC3, in pieces past 65,535 bytes), be passed by
  reference (RECORDARG for a mismatched form), be an **unformatted** I/O item as
  its bytes (DTypeZ; RECORDIOSIZE past a word's length), and sit in COMMON.
  It may not be in EQUIVALENCE, DATA, or NAMELIST (`equivalence.go`, `data.go`,
  `io.go`'s NMLITEM), nor a formatted item (`checkRecordItems`), nor a value
  (`scalar`).
- **NOTIMPL naming this phase or "records"**, each to open: a whole array of
  records as a value (`shape.go`'s `wholeArray`, `shaped.go`), an allocatable
  or assumed-shape array of records (`allocate.go`), an array-valued function
  of records (`arrayvalue.go`), a record's array field or an array of records as
  a section or expression actual (`shaped.go`, `sequence.go`), a vector
  subscript of a record's array field (`sections.go`).
- **Fortran 90's syntax today** (a program run through the compiler, 2026-11-18):
  `TYPE POINT` and `TYPE(POINT) :: Q` are recognized (`lex.TypeDef`,
  `lex.TypeDecl`) and NOTIMPL; the parser gathers a definition's statements to
  END TYPE in its `PendingStmt` (`parse/units.go`). `Q%X = 1.0` is a syntax
  error (`%` after a reference reads nothing; `parse/expr.go` knows `%` only as
  %LOC/%VAL/%REF/%DESCR). `IMPLICIT TYPE(POINT) (P)` is a syntax error ("expected
  a data type"). `TYPE(POINT) FUNCTION F(A)` isn't taken for a typed FUNCTION
  (`lex.typedFunction` is reached only from a type keyword), so it reads as a
  declaration with a blank in a name.
- **Phase 103's operators** (`sema/operators.go`): a defined operator or
  assignment is refused (DEFOPINTR) where the intrinsic one has a meaning;
  specifics are told apart by type, kind, and rank (`differ`), which today
  can't tell two record types of one size apart. Its comment already says the
  facility waits for derived types.
- **The storage map** shows every structure's fields with offsets (and maps), so
  a derived type's layout is shown by the same code.
- **The `/DEBUG` table and debugger** know a record only as its bytes
  (`dbgtable.Record`, shown as longwords); fields aren't taken apart.
- **FTNRTL's NAMELIST** (`ftn_namelist.mar`) reads names of letters, digits,
  `_`, and `$`, with subscripts and a substring; a group's block lists entities
  each with one name.

## A.3 Findings, and what they change

1. **The type says its structure.** `sema.Type` gains `Struct *Structure`, set
   for a record and a derived-type object alike (and for an expression of the
   type: a constructor, a function reference, a component). Type stays
   comparable, and `==` then tells two structures of one size apart, which every
   check that compares types (interfaces, generics' `differ`, RECORDASSIGN,
   operators) needs. `Symbol.Struct` becomes the type's, or stays as a shorthand;
   the 2 places that make `Type{Record, n}` and the 7 that test `Kind == Record`
   are where it starts. `Structure` gains `Derived` (made by TYPE, not STRUCTURE),
   `Sequence`, and later `Private` (Phase 106). **Type equality** is one function,
   `sameType`: the same `Structure`, or two `Derived` SEQUENCE structures with the
   same name and components agreeing in order, name, type, kind, length, and
   shape (4.4.2). A record and a derived-type object are never the same type,
   whatever their layout (the plan's open question 1: no, with a message naming
   both).
2. **Two front doors, one house.** A definition is parsed into an
   `ast.TypeDefStmt` (from the gathered `PendingStmt`) whose body is ordinary type
   statements (`REAL :: X, Y(3)`, `CHARACTER(LEN=8) NAME`, `TYPE(POINT) :: A`,
   `REAL, DIMENSION(2,2) :: COORD`), SEQUENCE, and PRIVATE; sema builds it with
   `structBuilder` (constant bounds and lengths already enforced). What STRUCTURE
   allows and TYPE doesn't is refused in a TYPE: UNION, MAP, `%FILL`, a RECORD
   statement, a nested STRUCTURE, and any attribute but DIMENSION (POINTER is
   Phase 107's NOTIMPL, PRIVATE outside a module and an access-spec are errors,
   inside one Phase 106's). New checks: no components (R422), SEQUENCE or PRIVATE
   twice, a SEQUENCE type's non-sequence component, END TYPE's name, a component
   of a type not yet defined (the type itself, without POINTER). TYPE(s) naming a
   STRUCTURE and RECORD /t/ naming a derived type are errors, so the two stay
   apart at the syntax while sharing layout, references, moves, and arguments.
3. **A type's name is a class 1 name.** A STRUCTURE's name lives apart from the
   unit's variables (section 4.15.1); a derived type's may not (14.1.2): TYPE POINT
   and a variable, constant, or procedure POINT in one scoping unit is an error,
   and a type name may not be an intrinsic type's (REAL, DOUBLEPRECISION, ...).
   Types keep the structures' table (it is host-associated already, so an
   internal procedure sees its host's types) but are also entered in the scope as
   a new symbol class, `DerivedType`, which is how a constructor `POINT(1., 2.)`
   is told from a function reference or an array element: `ast.Apply` whose name
   is a type in scope. An interface body's scope has no host, so a type there must
   be defined there, and only a SEQUENCE type can then match the caller's; the
   corpus and FORTRAN-INTERNALS.md must say so plainly, since it is the commonest
   surprise before modules.
4. **Layout.** Decision 9 (Phase 99's answer 6): SEQUENCE types laid out as
   STRUCTURE is (packed under V5.0's default, natural under
   `/ALIGN=RECORDS=NATURAL`); a non-SEQUENCE type naturally aligned (each
   component on its boundary, the size padded to the type's boundary), as HP's
   default. govax's `Alignment` has no state for "nothing said": its zero value is
   PACKED. So either the non-SEQUENCE rule ignores `/ALIGN`, or `Alignment` learns
   whether RECORDS was given, so that an explicit `/NOALIGN` or
   `/ALIGN=RECORDS=PACKED` packs non-SEQUENCE types as HP's does (question 1). The
   storage map's structure section shows a type as "Type POINT, 8 bytes,
   SEQUENCE" with each component's offset; padding is visible as gaps.
5. **Declarations.** `TYPE(name) [, attr ...] [::] entities` is a type statement
   whose type-spec is a derived type, through `attrtable.go`'s table: DIMENSION
   (explicit, adjustable, assumed-size, assumed-shape, deferred), ALLOCATABLE,
   SAVE, PARAMETER, INTENT, OPTIONAL, TARGET (recorded), EXTERNAL, and `= value`.
   The lexer needs `TYPE(name) FUNCTION F(...)` (with RECURSIVE) as a typed
   FUNCTION, and the parser `TYPE(name)` in IMPLICIT and as a type-spec where
   `typeSpec` is read. The parser's `recordScope` gains each TYPE(t) entity, so
   `P.X` reads as a component under `/F90` (the HP manual allows `.` for derived
   types too), and IMPLICIT TYPE(t) names count by letter. A dummy, a function
   result, and a local of a derived type are what a record already is.
6. **`%` is a component selector everywhere.** After a reference, `%` and a name
   is a component (`ast.FieldRef` with a `Percent` mark, so messages and listings
   say `P%X`, not `P.X`); before a primary it stays %LOC and the rest, which
   can't follow a reference, so the two never meet, under `/F90` and `/NOF90`
   alike (nothing old changes: `TestNoF90Unchanged`). A component's
   parenthesized list is read as a name's is, sema deciding element, section, or
   substring (Phase 102's `sections.go`), which ends Phase 102's finding 4 for
   STRUCTURE's records too: `R.A(1:3)` of an array field becomes a section, a
   change for `/NOF90` programs only where they wrote a substring of a
   *non-CHARACTER* array field's element, which was an error before.
7. **A ranked part is a strided array, and the scalarizer already does it.**
   `FieldRef` gains a shape: at most one part may have section subscripts or be a
   whole array (MULTIRANK otherwise, 6.1.2's constraint), and the reference's
   shape is that part's. Scalarizing it (`sema.Scalarize`) replaces that part's
   triplets with expressions of the loop's index symbols, leaving a *scalar*
   `FieldRef`, which `fieldMem` already reaches: `PATH(I)%X` is at PATH plus X's
   offset plus (I−1) times the type's size, with no new addressing and Phase 84's induction
   stepping it by the type's size. Overlap (`overlap.go`) already knows FieldRef
   places. As an actual argument:
   - to an **assumed-shape** dummy, an NCA built at the call (Phase 104's
     `sectionNCA`) whose stride is the type's size times the triplet's, in bytes,
     and whose POINTER is the first element's component: the plan's design note,
     no copy;
   - to an **old** dummy, never contiguous in general (only when the component is
     the type's whole storage), so Phase 104's copy in and back (`valueTemp`,
     `copyBack`), unless INTENT(IN).
   Arrays *of* a derived type as wholes and sections (`A = B`, `A(2:5) = P`,
   `WHERE`, constructors `(/ POINT(1.,2.), Q /)`, MERGE, SPREAD, CSHIFT, EOSHIFT
   with its required BOUNDARY=, TRANSPOSE, RESHAPE, PACK, UNPACK, and the
   inquiries) are leaves of Record type in the scalarizer, each element assigned
   by `moveRecord`'s CharMove; their temporaries are sized by the type. Opening
   them opens the same NOTIMPLs for STRUCTURE's records, which share every line.
8. **Structure constructors are values with a place.** sema's `StructValue{Type,
   Values}`: each value checked and converted by the assignment rules
   (`assignedValue`), the count checked (CONSTRCOUNT), an array component's value
   conforming (a scalar broadcast). All constant, it folds to the type's bytes
   (padding zero) with each component's value kept, so a constant's component
   (`ORIGIN%X`) folds too; that is a `$PDATA` datum, an `Initial` for `= value` and
   DATA (the records' `placeInitials` path), and a PARAMETER's value. Not constant,
   it is built in a temporary of the type, a store per component (a nested
   constructor in place), then used as a record is; an assignment `P = POINT(X,
   Y)` builds straight into P when no value reads P (`overlap.go`'s rule), else
   through the temporary. A constructor is a value, so it may be an actual (passed
   as an expression, a copy), but never a target or an input item.
9. **Assignment and operators.** Intrinsic assignment between one type's objects
   is `moveRecord` (padding moves too, harmlessly), checked by `sameType`
   (RECORDASSIGN's wording widened). A defined ASSIGNMENT(=) whose two dummies are
   of one derived type is allowed (`intrinsicMeaning` is false for derived types)
   and replaces the intrinsic assignment wherever it is accessible, which the
   assignment statement must look for first. Any other operator on a derived type
   without an interface is an error naming the operator and the type
   (NODERIVEDOP): `P == Q` has no meaning in Fortran 90. (The plan's goal line
   "intrinsic assignment and equality only by defined operators" means exactly
   this.)
10. **Arguments and type equality.** An object goes by reference, as a record.
    `sameType` replaces `form` wherever a derived type is involved:
    `checkInterface`/INTERFACEDIFF, generic resolution (14.1.2.3's "differ in
    type"), and a same-file reference without an interface (`recordArguments`),
    where two external units' *separate* non-SEQUENCE definitions are different
    types, which the standard calls nonconforming and gfortran refuses (question
    5). A record and a derived-type object never pass for each other. Assumed-shape,
    OPTIONAL, INTENT, and keyword arguments follow from Phases 103 and 104 once
    the type is on `Type`.
11. **Function results in R0 and R1 when they fit, else a hidden argument**
    (the author's answer 2). `TYPE(POINT) FUNCTION F`, `FUNCTION F() RESULT(R)`
    with R of a derived type, recursive or not:
    - **8 bytes or fewer** come back in R0, or R0 and R1, as the Calling
      Standard returns any value up to a quadword. The result variable is a cell
      of the function's own (in the frame when reentrant) rounded up to 4 or 8
      bytes, its components assigned there (`F%X = 1.0`); the return loads it
      whole, `MOVL cell,R0` up to 4 bytes or `MOVQ cell,R0` past that, so the
      bytes past the type's size in R0 or R1 are undefined. The caller stores R0
      (and R1) into a temporary of the same rounded size, `MOVL R0,t` or `MOVQ
      R0,t`, and uses its first size bytes as a record; for an assignment of a
      1-, 2-, 4-, or 8-byte type straight into the target, one `MOVB`, `MOVW`,
      `MOVL`, or `MOVQ` from the registers. So the IR still moves records only
      through memory: the registers are crossed only at the call and the return,
      as a COMPLEX*8's are. `Type.HiddenResult` stays `Size > 8` for a derived
      type, as for every other non-CHARACTER type.
    - **Past 8 bytes**, the caller passes the address of a temporary of the type,
      which the result variable is, as for COMPLEX*16.
    A MACRO caller or callee sees the Calling Standard's rule either way.
    Array-valued functions of a derived type follow
    Phase 104's two forms. A statement function of a derived type is NOTIMPL (it is
    allowed but rare, and its expansion in line would need a constructor's
    temporary per reference).
12. **COMMON, EQUIVALENCE, DATA.** COMMON takes SEQUENCE objects (COMMONTYPE for
    any other), laid out whole as records are now; COMMON's `/ALIGN=COMMONS` places
    the object by its `Boundary`. EQUIVALENCE, refused for records (section
    2.2.5.2), is opened for SEQUENCE objects only, never a component (5.5.1); the
    standard's type classes (numeric sequence with numeric, character with
    character, others with their own type) are stricter than VAX FORTRAN, which
    lets numeric and CHARACTER share storage (question 6). DATA takes constructors
    of constants and component designators as objects (an implied DO's `A(I)%X`
    too), through `data.go`'s walk with each component an item; records stay
    refused in DATA, as the HP manual says.
13. **I/O expands in the compiler.** Formatted and list-directed: an item of a
    derived type becomes its ultimate components in order, each a `FieldRef`, a
    nested type recursively, an array component by its descriptor as an array
    field is now; an array of a derived type, an implied loop over its elements
    with the components inside (Phase 102's arrays go element by element). FTNRTL
    is unchanged. Unformatted: one item of the object's bytes (DTypeZ, padding
    included: the "processor-dependent form"), as a record now, an array of them
    one item of all their bytes when contiguous; past 65,535 bytes, pieces of at
    most a word each (today's RECORDIOSIZE becomes a split). Input items the same
    ways. STRUCTURE's records stay refused in formatted I/O, as the HP manual
    says.
14. **NAMELIST needs FTNRTL.** The group block gets an entity per ultimate
    component, named with `%` (`P%X`, `P%C%Y`, an array component as an array
    entity), so output writes `P%X    = 1.0,` (after the record's blank) as gfortran writes `P%X=`, and
    FTNRTL's name reader takes `%` inside a name. The standard's whole-object
    input, `P = 1.0, 2.0`, filling the components in order across entities, needs
    the block to say which entities make one object (a count on the first) and
    FTNRTL to go on to the next entity where an array's values would go on to the
    next element (question 4). Records stay out of NAMELIST.
15. **What waits.** PRIVATE components, access-specs, and types shared by USE:
    Phase 106. POINTER components and a type pointing to itself, and the I/O
    refusals they bring: Phase 107. Default initialization of components: Fortran
    95's (question 3). The debugger's EXAMINE `P%X` (and `R.F`): Phase 110, with
    the rest of the `/DEBUG` table's growth; until then a derived-type object is
    shown as a record's longwords. `/CROSS_REFERENCE` lists a type's name as it
    lists a structure's, and component names under their type.
16. **The optimizer** needs nothing new: a component is a place in its parent's
    region (`places.go`), a strided component section is the scalarizer's loop
    over such places, and a hidden-result temporary is a cell like any other.

## A.4 The subtasks, revised

The plan's order put arrays of components before constructors and assignment;
this order makes scalar objects usable end to end first (assignments need
constructors to be tested readably), then arguments, then arrays, whose tests use
all three.

1. **Survey** (this Part A) (done 2026-11-18; accepted by the author the
   same day, with answer 2 changed).
2. **Definitions and objects:** `Type.Struct`, `Structure.Derived` and
   `Sequence`, `ast.TypeDefStmt` and the definition's checks (finding 2), class 1
   names and host association (finding 3), layout and `/ALIGN` (finding 4),
   `TYPE(t)` declarations, IMPLICIT, and typed FUNCTION statements (finding 5),
   `%` and `.` scalar components with substrings (finding 6), the storage map
   (done 2026-11-18).
3. **Constructors, assignment, operators:** `StructValue`, constant folding,
   initial values and PARAMETERs and their components, intrinsic assignment by
   `sameType`, defined assignment replacing it, NODERIVEDOP (findings 8 and 9)
   (done 2026-11-18).
4. **Arguments and results:** `sameType` at every check, RECORD versus TYPE,
   separate definitions (finding 10), results in R0 and R1 up to 8 bytes and the
   hidden result past that, RESULT, recursion,
   internal procedures, interface bodies with SEQUENCE types (finding 11)
   (done 2026-11-18).
5. **Arrays:** components with a rank and their sections, MULTIRANK, a field's
   list read as a name's (finding 6's change for records), arrays of a type as
   wholes, sections, constructors, and the type-generic intrinsics; to
   assumed-shape dummies by stride and to old ones by copy; allocatable,
   automatic, assumed-shape, and array-valued arrays of a type, and the same for
   records (finding 7). Likely in two parts, 5a scalarized expressions and
   assignment, 5b arguments and the dynamic kinds.
6. **COMMON, EQUIVALENCE, DATA** with SEQUENCE types (finding 12).
7. **I/O and NAMELIST:** the compiler's expansion, unformatted bytes and their
   pieces (finding 13); NAMELIST's entities and FTNRTL's names (finding 14).
8. **Programs end to end** (a geometry program with constructors and defined
   operators, a parts inventory of an array of a type read and written formatted
   and unformatted, a SEQUENCE type shared through COMMON and an interface body),
   every mode, gfortran's witness.
9. **Close-out:** FORTRAN-INTERNALS.md's section 30 (STRUCTURE and TYPE side by
   side: one `Structure`, two front doors; why a component section is just
   another stride; why an interface body needs a SEQUENCE type), MANUAL.md's
   14.13, HELP FORTRAN.

## A.5 Questions for the author

1. **`/ALIGN` and non-SEQUENCE types** (finding 4). (a) **Naturally aligned
   unless `/NOALIGN` or `/ALIGN=RECORDS=PACKED` (or an OPTIONS group's) was given
   explicitly, as HP Fortran packs them then; SEQUENCE types follow STRUCTURE's
   rule**; (b) non-SEQUENCE always natural, SEQUENCE always as STRUCTURE.
2. **Function results** (finding 11). (a) **Always through a hidden first
   argument, whatever the size**; (b) in R0 and R1 up to 8 bytes, hidden past
   that, as the plan said.
3. **Default initialization of components** (Fortran 95; finding 15). (a)
   **NOTIMPL naming Phase 112**, the corpus keeping it out; (b) compiled now (a
   local of such a type then needs its values stored at every entry, an INTENT(OUT)
   dummy at every call, which is more than STRUCTURE's static initial values).
4. **NAMELIST of derived types** (finding 14). (a) **Output, `P%X =` input, and
   whole-object `P =` input here, FTNRTL learning `%` in names and objects of
   several entities**; (b) output and `P%X =` input here, `P =` input in Phase 109
   with Fortran 90's NAMELIST form; (c) all of it in Phase 109.
5. **Separate non-SEQUENCE definitions in one file** (finding 10). An external
   unit passes its `TYPE POINT` object to another external unit's own `TYPE
   POINT` dummy, same components, no SEQUENCE. (a) **An error, the standard's rule
   and gfortran's**; (b) a warning, compiled as if the same type when the layouts
   agree.
6. **EQUIVALENCE of sequence types** (finding 12). (a) **The standard's type
   classes (numeric sequence with default numeric, character sequence with
   default CHARACTER, other sequence types with their own type), except that
   numeric and CHARACTER may share storage as VAX FORTRAN lets them**; (b) the
   standard's classes strictly; (c) any SEQUENCE object with anything.

## The author's answers (2026-11-18)

The author read the survey and accepted every recommendation but question 2's:

1. A non-SEQUENCE type is naturally aligned unless `/NOALIGN` or
   `/ALIGN=RECORDS=PACKED` (or an OPTIONS group's) was given explicitly;
   SEQUENCE types follow STRUCTURE's rule (finding 4). `Alignment` learns
   whether RECORDS was given.
2. **A function's derived-type result comes back in R0, or R0 and R1, when it
   fits in 8 bytes, and through a hidden first argument only past that**, as the
   plan first said (finding 11, rewritten for it).
3. Default initialization of components is NOTIMPL naming Phase 112, and the
   corpus keeps it out.
4. NAMELIST of derived types is all here: output, `P%X =` input, and
   whole-object `P =` input, FTNRTL learning `%` in names and objects of
   several entities (finding 14).
5. Separate non-SEQUENCE definitions passed to each other in one file are an
   error (finding 10).
6. EQUIVALENCE of SEQUENCE types follows the standard's type classes, except
   that numeric and CHARACTER storage may be shared as VAX FORTRAN allows
   (finding 12).

The plan's open question 1 is settled by finding 1: a record and a derived-type
object are never the same type, with a message naming both.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-18, subtask 1 (written, for review):** Part A, the survey. A program
  with every new form run through the compiler gave the starting point (A.2):
  definitions and `TYPE(t)` declarations NOTIMPL, `%`, `IMPLICIT TYPE(t)`, and
  `TYPE(t) FUNCTION` syntax errors. Sixteen findings; the subtasks reordered
  (scalars, constructors, and arguments before arrays); six questions.
- **2026-11-18, subtask 1 (done):** The author accepted the survey and every
  recommendation but question 2: a derived-type result of 8 bytes or fewer
  comes back in R0 and R1 (finding 11 rewritten; "The author's answers").
  Subtask 2 is next.
- **2026-11-18, subtask 2 (done):** Definitions and objects.
  - *Syntax.* The parser makes `TYPE [, access ::] name` an `ast.TypeDefStmt`
    (`parse/derived.go`), and `units.go` gathers every statement to its END TYPE
    into its `Body` (NOENDTYPE if the unit ends first, NOTYPEEND for an END TYPE
    with none open). `SEQUENCE` is an `ast.SequenceStmt`, `END TYPE [name]` an
    `ast.EndTypeStmt`; PRIVATE stays a `PendingStmt`. `TYPE(t) [, attrs ::]
    entities` is a `TypeStmt` whose `TypeSpec.Derived` names the type (the
    statement's tail shared with the intrinsic types, `typeEntities`); IMPLICIT
    takes `TYPE(t)`; the lexer takes `[RECURSIVE] TYPE(t) FUNCTION F` for a typed
    FUNCTION whose `FunctionType` is `TypeDecl`. After any reference `%` and a name
    is a component (`componentNext`), a `FieldRef` with `Percent`, under `/F90`
    and `/NOF90` alike; `recordScope` counts TYPE(t) entities and IMPLICIT
    TYPE(t)'s letters as records, so `P.X.Y` reads as components under `/F90`.
  - *The type says its structure.* `sema.Type` gained `Struct` (every positional
    `Type{k, n}` literal is keyed now); `recordType(st)` makes a record's or an
    object's type, and `Type.String` is `TYPE(POINT)` for a derived type.
    `Structure` gained `Derived` and `Sequence`, `What` and `objectText` for
    messages (NOFIELD, STRUCTSIZE, and RECORDASSIGN reworded to fit both), and
    `Display` is `TYPE(POINT)` for one.
  - *Definitions* (`sema/derived.go`'s `typeDefinition`): components through
    `structBuilder` (`componentStatement`: DIMENSION as an attribute, POINTER
    NOTIMPL for Phase 107, a default initial value NOTIMPL for Phase 112, any other
    attribute TYPEATTR; a component's type defined before it, TYPESELF for its
    own); TYPENAME, TYPEACCESS outside a module, DUPTYPE (a STRUCTURE of the
    name too), TYPESTMT, TYPEEMPTY, TYPETWICE, TYPEORDER, SEQCOMPONENT,
    ENDTYPENAME. A type is in the unit's table of structures, so an internal
    procedure sees its host's; TYPE(t) naming a STRUCTURE is TYPESTRUCT, and RECORD
    /t/ naming a type RECORDTYPE. No FIELDOP warning for a type's components,
    which `%` reaches.
  - *Layout* (the author's answer 1): `Alignment.RecordsGiven` says a setting
    named RECORDS, so a non-SEQUENCE type is natural unless PACKED was given
    explicitly (`derivedLayout`); the FORTRAN command now passes an explicit
    `/NOALIGN` (`FortranOptions.NoAlign`). The storage map heads a type "Type
    POINT, 8 bytes, SEQUENCE".
  - *Objects.* A TYPE(t) entity gets the type and its `Symbol.Struct`, and is
    compiled as a record is from there: fields, whole moves between one type's
    objects, arguments by reference (still compared by `form` until subtask 4),
    host association. IMPLICIT TYPE(t) letters hold the type's name
    (`implicitRules.derived`), since the standard puts IMPLICIT before the
    definitions; a name given such a type waits for `resolveImplicitObjects`,
    after the declarations walk (a dummy is entered at the header, before any
    definition). TYPECLASH (14.1.2, class 1) is checked once the unit is analyzed:
    any symbol of the unit with a type's name.
  - *NOTIMPL for the subtasks to come:* a structure constructor (a reference to a
    type's name in parentheses), a value of a derived type in a declaration
    (subtask 3), and a function of a derived type, by header, type statement, or
    IMPLICIT (subtask 4).
  - *Tests:* `parse/testdata/derived.f90`, `lex`'s typed FUNCTION cases,
    `sema/derived_test.go` (layouts under four settings, types told apart, every
    message), `fortran`'s `TestDerivedTypeMap`, and
    `testdata/fortran90/types.f90` in every mode. It isn't in `compare.py`: DEC's
    period is no Fortran 95.
- **2026-11-18, subtask 3 (done):** Constructors, assignment, operators
  (`sema/structvalue.go`, `ir/lowerstruct.go`).
  - *Constructors.* `POINT(...)` is a constructor when POINT is a finished
    derived type in scope and nothing else of the unit (`constructorType`),
    in the evaluator (`ev.apply`, so a constructor of constants is a constant)
    and in `apply`. A value per component (CONSTRCOUNT), no keywords
    (CONSTRKEYWORD) and none left out (CONSTRNULL), each converted as an
    assignment converts (CONSTRVALUE): a component of a derived type takes an
    object of a type `sameType` calls its own, an array component a scalar for
    every element or an array of its shape (NOTCONFORM).
  - *Constants.* A constructor of constants is a `Const` of the record type
    whose `Parts` are the components' values (a `*Const`, or an `*ArrayConst`
    for an array component) and whose `Raw` is the value's bytes, padding zero
    (`partBytes`). `ev.convert` moves one only to its own type. It is a
    PARAMETER's value (both forms; `TYPE(T), PARAMETER` arrays NOTIMPL for
    subtask 5), and a constant's component folds, `ORIGIN%X`, `TAG%AT%Y`,
    `TAG%MARKS(2)` (`ev.component`, and `field` trying the evaluator first when
    the reference starts from a constant). A component whose subscripts aren't
    constant is read from the constant's datum (`symbolMem`). A record constant
    alone where a value is wanted is AGGREGATE, as a record's name is.
  - *Initial values.* `TYPE(POINT) :: P = POINT(1.0, 2.0)` keeps the constant
    (`Symbol.recordValue`, implying SAVE), and `recordInitials` makes it one
    Initial per scalar component, an array component's per element, named
    `P%AT%X`, `P%MARKS(2)` (`valueInitials`); INITTYPE for another type's
    value, DATAVALUE for one not constant; an array's value from an array
    constructor NOTIMPL for subtask 5 (a scalar is every element's).
  - *Values not constant* are a `StructValue`: each scalar component's value
    converted, an array component's constant bytes (`Bytes`; a value that isn't
    constant NOTIMPL for subtask 5). Lowering builds it a component at a time
    (`buildRecord`: a store, a MOVC3/MOVC5 for CHARACTER, a nested constructor
    built in place, an object's bytes moved), at a place whose offset counts
    bytes (`recordPlace`: a dummy through its argument's longword, an element
    with a subscript through its address computed once). `Assign.InPlace` (sema's
    `inPlace`, over `readsAny`, which now walks a `StructValue`) builds it in the
    target when no value reads the target; `P = POINT(P%Y, P%X)` goes through a
    temporary, then `moveBytes` (`moveRecord`'s pieces, shared). A constructor
    as an argument is passed in a temporary, a constant in its datum, and a
    record copied for defined assignment's value or parentheses.
  - *Assignment and operators.* Intrinsic assignment of a record checks
    `sameType` (the same definition, or two SEQUENCE definitions agreeing in
    name and components, 4.4.2); `structureOf` answers for any record-typed
    expression. Operands are analyzed by `operand`, which keeps a record or an
    object as a whole, so that `extendedOp` and `definedOp` can give
    `P .DIST. Q` and `P == Q` an interface's meaning; otherwise
    `aggregateOperands` reports NODERIVEDOP (once) for a derived type and
    AGGREGATE (per operand, as before) for a STRUCTURE's record. The evaluator
    makes no constant of an operation on a record (`noOperators`). Defined
    assignment already took records (`intrinsicAssignment` is false for them);
    every assignment of the type goes through it, a constructor's too.
  - *Type equality where it was needed now*: generics' and operators' `differ`
    (`argumentStructures`: `sameType`, or two STRUCTUREs of one form as
    RECORDARG has it), and INTERFACEDIFF's types (`sameTypes`), so an interface
    body's own SEQUENCE definition matches the procedure's. The other argument
    checks (`recordArguments`'s form, separate non-SEQUENCE definitions) are
    subtask 4's.
  - *The parser.* Under `/F90` a period after a derived type's object is a
    component only if the name is one of a type's defined in scope
    (`recordScope.component`, gathered from each definition's statements),
    so `ORIGIN .DIST. Q` stays an operation; a STRUCTURE's record keeps the
    HP manual's rule (a field wins).
  - *Tests:* `sema/structvalue_test.go` (constants, their bytes and
    components, initial values, values in place or moved, every message),
    `testdata/fortran90/structs.f90` (in `compare.py`; gfortran prints the same
    lines). MANUAL.md's 14.13 says what operators and assignment do with
    derived types.
- **2026-11-18, subtask 4 (done):** Arguments and results (`ir/lowerresult.go`;
  `sema/results_test.go`).
  - *A function's type.* The three NOTIMPLs are gone. A header's `TYPE(T)
    FUNCTION F` types the value at once (so a type statement for it is DUPDECL)
    but finds its structure after the declarations (`Unit.headerType`, resolved in
    `resolveImplicitObjects`), since 5.1.1.7 lets the function's own body, or an
    interface body, define the type after the header. A type statement, RESULT,
    and IMPLICIT TYPE(t) give the value a derived type as they give any other.
  - *A name of a derived type has its use's class.* `TYPE(POINT) F` no longer
    makes F a variable, as `REAL F` doesn't, so F may be an external function
    (EXTERNAL takes a derived type's name; a STRUCTURE's record still not), a
    dummy procedure, or a variable, by its use.
  - *Values in registers* (the author's answer 2). Up to 8 bytes, the value's
    storage is its room, a longword up to 4 bytes and a quadword past that
    (`Symbol.room`, `sema.RecordRoom`; `placeResults` for ENTRY's shared place),
    and the return loads it whole (`resultType`, `resultValue`: `MOVL F.F,R0` or
    `MOVQ`). A caller assigning the value to a target of 1, 2, 4, or 8 bytes stores
    the registers straight into it (`assignResult`: `MOVB`, `MOVW`, `MOVL`, `MOVQ`
    `R0,target`, the target's index taken away by `recordPlace`); any other use
    (a 3-, 5-, 6-, or 7-byte type's assignment, an argument, a constructor's
    component, an operand of a defined operator) gets a temporary of the room
    (`recordResult`, through `mem`), whose first bytes are the record. Past 8
    bytes the value goes through a hidden first argument, a temporary of the type,
    as COMPLEX*16's. `TestDerivedResultCode` holds the instructions.
  - *Type equality at every check.* `recordArguments` passes what
    `argumentStructures` passes (one type, or two STRUCTUREs of one form), and
    otherwise, where a derived type is involved, says TYPEARG (a record, a number,
    or another type), or SEPARATEARG for two units' definitions of one name (each
    definition's line); FUNCTYPE compares by `sameTypes`, SEPARATEFUNC for the
    same case. A function of a derived type has entry points only of its type
    (ENTRYDERIVED, 12.5.2.5). A statement function of a derived type is NOTIMPL
    (finding 11). An array-valued function of a derived type is still NOTIMPL,
    subtask 5's.
  - *Tests:* `sema`'s `TestDerivedResults` (every way of typing the value, and
    its room) and `TestDerivedArguments` (what passes, every message);
    `fortran`'s `TestDerivedResultCode`; `testdata/fortran90/results.f90`
    (values of 3, 4, 6, 8, 12, and 16 bytes, RESULT, ENTRY, RECURSIVE, an
    internal function using its host's type, a defined `+`'s value as an operand,
    a function passed as an argument), in every mode and in `compare.py`, where
    gfortran prints the same lines. Its REALs are printed by formats:
    gfortran 16 writes a list-directed REAL*4 with 9 digits where VMS writes 7,
    and so `structs.f90` (subtask 3's) now differs on gfortran in its digits
    only, the values agreeing; a later subtask should give it formats too.
    MANUAL.md's 14.13 has "Derived types as arguments and values".
