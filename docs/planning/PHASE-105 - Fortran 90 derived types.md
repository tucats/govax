# Phase 105 — Fortran 90: derived types

**Status:** done 2026-11-18. Planned 2026-11-17 with Phase 99 (`PHASE-99 -
Fortran 90.md`, Part C). Needs Phase 104 (done). Subtask 1, the survey (Part A),
done 2026-11-18 and accepted by the author ("The author's answers"); subtasks 2
to 9 done the same day. "Where things stand", at the end, is what the phase left.

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
   records (finding 7). In two parts:
   - 5a scalarized expressions and assignment: components with rank,
     MULTIRANK, a field's list read as a name's, arrays of a type whole and as
     sections, scalars of a type for every element, array constructors of a
     type, the type-generic intrinsics, array PARAMETERs of a type, arrays'
     initial values, and constructors' array components from arrays (done
     2026-11-18);
   - 5b arguments and the dynamic kinds: a ranked component or an array of a
     type to assumed-shape dummies by stride and to old ones by copy;
     allocatable, automatic, assumed-shape, and array-valued arrays of a type,
     and the same for records (done 2026-11-18).
6. **COMMON, EQUIVALENCE, DATA** with SEQUENCE types (finding 12) (done
   2026-11-18).
7. **I/O and NAMELIST:** the compiler's expansion, unformatted bytes and their
   pieces (finding 13); NAMELIST's entities and FTNRTL's names (finding 14)
   (done 2026-11-18).
8. **Programs end to end** (a geometry program with constructors and defined
   operators, a parts inventory of an array of a type read and written formatted
   and unformatted, a SEQUENCE type shared through COMMON and an interface body),
   every mode, gfortran's witness (done 2026-11-18).
9. **Close-out:** FORTRAN-INTERNALS.md's section 30 (STRUCTURE and TYPE side by
   side: one `Structure`, two front doors; why a component section is just
   another stride; why an interface body needs a SEQUENCE type), MANUAL.md's
   14.13, HELP FORTRAN. (done 2026-11-18)

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
- **2026-11-18, subtask 5a (done):** Arrays in expressions and assignments
  (subtask 5 split in two, A.4).
  - *A field's list is a name's.* The parser reads a component's parenthesized
    list as subscripts that may hold triplets (`parse/expr.go`'s
    `referenceAfter`), and `substringForm` (`sections.go`'s
    `componentSubstring`, `componentOf`) makes one stride-less triplet after a
    field that isn't an array its substring, as VAX FORTRAN read it; every
    place that reads a reference goes through it (`argValue`, `aggregateValue`,
    the I/O items, SIZEOF, the inquiries). `R.B(1:2)` of an array field is now
    a section, under `/NOF90` too: before, it was an error (FIELDWHOLE).
  - *A ranked FieldRef* (finding 7). `FieldRef` gained `Ranked` (0 the
    record's own subscripts, k the path's step k) and `Sections`, the ranked
    part's section subscripts, with the part's shape in `S`: a whole array
    (`PATH%X`, an array field without subscripts outside an argument or an I/O
    list) has a triplet of its bounds per dimension (`wholeSections`), a
    section or vector subscript `sectionList`'s, the one analysis an array's
    sections and a component's now share (`section`, `triplet`, and
    `tripletExtent` take dimensions, not a symbol). A second ranked part is
    MULTIRANK (new); an array field without subscripts where a scalar must be
    is still FIELDWHOLE (`scalarValue`). `At` is the element at given
    subscripts, a scalar FieldRef, which `fieldMem` reaches as before, so
    `PATH%Y = 2.0 * PATH%X` is one loop stepping by the type's size (MULF3
    `PATH-8[R7]`, `ADDL2 #2,R7`). The scalarizer takes a ranked FieldRef as a
    leaf (`Scalarize`, `Leaves`; `ir`'s `prepareSections`, shared with a
    section, a field's constant bounds in `arrayLeaf.field`); the dump shows
    the ranked part's sections (`dumpSections`).
  - *Overlap.* `baseSymbol`, `slots`, `vectorsRead`, `checkManyOne`, and
    `readsAny` know a component reference (`readsAny` now walks its
    subscripts too). Two component references of one record or array of
    records are related by their subscripts when their fields are the same,
    apart when the first fields they differ in don't meet
    (`componentRelation`: `PATH%X = PATH%Y` needs no temporary), and unknown
    otherwise, a temporary.
  - *Arrays of a type, and of records.* `wholeArray` no longer refuses them;
    `aggregateValue` takes a whole array or an array constructor for an array
    target, `operand` a whole array, and `arrayValue` refuses one (AGGREGATE),
    as it refuses a scalar record. An element moves whole (`assignScalar`'s
    `moveBytes`), and a scalar of the type is copied once into a temporary
    before the loops (`hoist`), a constructor built there. Array constructors
    hold objects of a type (`ctorBuilder` through `operand`; two SEQUENCE
    definitions of one type by `sameType`). MERGE (`merge`, lowered by `mem`
    as two moves around the mask's branch), the transformational functions
    (`transform`'s `value` through `operand`; `sameKind` is `sameType` for
    records; EOSHIFT of a type needs its BOUNDARY), TRANSPOSE, and the
    inquiries take arrays of any type.
  - *Constants and initial values.* An array PARAMETER of a type is an
    `ArrayConst` of record constants, its datum their bytes in turn
    (`arrayConstMem`); `PTS(2)%Y` folds (`componentBase` takes an element), and
    `PTS(I)%Y` and `PTS%X` are read from the datum (`recordBase` takes an array
    constant as an array of records). `TYPE(POINT) :: R(2) = (/ ... /)` is an
    initial value per element (`Symbol.recordValues`, `recordInitials`;
    INITTYPE, NOTCONFORM). A constructor's array component may be any array of
    its shape or a scalar (`componentValue`), which `buildRecord` assigns
    through an array of lowering's own over the component's storage
    (`fillComponent`).
  - *Fixes found on the way.* A constant's component as an assignment's target
    or an input item (`ORIGIN%X = 1`, since subtask 3) said nothing and stored
    into `$PDATA`: now BADUSE (`constantComponent`), and BADUSE names a
    constant of a derived type "a constant" (`describe`). `componentBase` called
    an array constant a failure already reported, which left an unreported bad
    expression for lowering to panic on.
  - *Left for 5b, NOTIMPL:* a ranked component as an actual argument
    (`argValue`); it would have passed the first element's address as if the
    components were contiguous. Formatted I/O of whole objects stays subtask
    7's (AGGREGATEIO); a ranked component of an intrinsic type is an I/O item
    an element at a time already (`eachElement`).
  - *Tests:* `sema`'s `TestErrors` (MULTIRANK, FIELDWHOLE where a scalar must
    be, sections of fields, NOTCONFORM, SUBSCRIPTRANGE) and `TestStructMessages`
    (the two NOTIMPLs now compiled, constants as targets, initial values,
    arrays of a type as numbers or with operators, constructors and MERGE of
    two types, EOSHIFT without a BOUNDARY); the goldens
    `f90components.for` in `sema`, `ir`, and `codegen`; `testdata/fortran90`'s
    `typearrays.f90` and `typevalues.f90` (in `compare.py`; gfortran prints the
    same lines), and `testdata/fortran`'s `recarrays.for`, STRUCTURE's records,
    in every mode. `recarrays.for` is in `testdata/ftn86`'s procedure
    (`gen.go` rerun) and `ftn86AwaitingVMS`, for the next VMS round; the
    `/NOF90` baseline has it too.
- **2026-11-18, subtask 5b (done):** Arguments and the dynamic kinds.
  - *A component with a rank as an actual argument* (finding 7). `argValue` marks
    a ranked `FieldRef` an array value (`Arg.expression`), for its dummy to decide
    on, as a section is. To an **assumed-shape** dummy (`shaped`) it is described
    where it is: `Described` takes a ranked FieldRef with no vector subscript
    (`hasVector`), and the IR's `sectionNCA`/`describeSection` now take one beside
    a section, its first element the component of the ranked part's first
    (`FieldRef.At`), each stride that part's (`rankedStride`: an array of records'
    `baseStride`, the type's size or a descriptor's; an array component's element
    size times its extents before), times the triplet's step. So
    `CALL SCALE(PATH(2:6:2)%Y, 5.0)` is `MOVAL PATH+12,$D1+4`, `MOVL #16,$D1+20`,
    and no copy, the plan's design note. A substring of one (`TAGS%NAME(2:3)`) moves
    POINTER and the element length (`substringIn`, `substringOf`'s work for any
    element length). An array field without subscripts, `T%COORD`, given to an
    assumed-shape dummy is made the ranked reference of every element
    (`FieldRef.wholeRanked`), described the same way; to an old dummy it is still
    passed by address, as VAX FORTRAN passes it. To an **old** dummy (`sequenced`)
    a ranked component is never taken as contiguous: `valueTemp`'s temporary,
    copied back by `arrayNest` unless the dummy is INTENT(IN), with no change to
    the IR. A vector subscript's component and a constant's (`PTS%X` of an array
    PARAMETER, `Described` false for a Constant's FieldRef) are values, in a
    temporary never copied back.
  - *Arrays of a type, and of records, as arguments.* The NOTIMPLs in `shaped` and
    `sequenced` are gone: whole arrays and sections of records are described (an
    NCA's element type DTypeZ, its length the type's size), passed on from an
    assumed-shape dummy (`wholeNCA`, `testedSequence`), contiguous sections in
    place (`contiguousSection`), other sections and expressions (MERGE of a type)
    through a temporary. `checkShapes` takes an assumed-shape dummy of records.
    `fieldMem` reaches a component of an assumed-shape array's element through
    `stridedElement`'s address, its strides not the type's size.
  - *Ranks of records compared.* `argumentMismatch` left records out entirely;
    their types are still `recordArguments'` to compare, but their ranks are now
    checked as any other's (ARGRANK: an object to an assumed-shape dummy, an array
    of a type to a scalar dummy). `argumentRank` counts an array field without
    subscripts as its field's rank, so `T%COORD` to an array dummy through an
    explicit interface is no longer taken for a scalar. TestNoF90Unchanged's
    baseline gained only the new `recdyn.for`.
  - *The dynamic kinds.* With the NOTIMPLs in `checkShapes` (allocatable) and
    `checkArrayValue` (array-valued functions) gone, allocatable arrays of a type
    or of records, automatic ones (`TYPE(POINT) :: WORK(SIZE(A))`,
    `RECORD /ITEM/ W(SIZE(R))`), assumed-shape dummies, and functions whose values
    are arrays of a type, of a constant and of a run-time shape, compiled and ran
    with no further change: Phase 104's descriptors were already sized by the
    element's type. A STRUCTURE's records can't be a function's value (RECORD
    names no function), so that form is a derived type's only.
  - *Tests:* `sema`'s `TestDerivedArguments` (arrays and components to
    assumed-shape and old dummies, the two ARGRANKs) and `TestAllocatable` (an
    allocatable array of records and of a type); `fortran`'s
    `TestComponentArgumentCode` (the descriptor's POINTER and stride, the copy in
    and back); `testdata/fortran90`'s `typeargs.f90` and `typedyn.f90` (in
    `compare.py`, gfortran printing the same lines; worked through by hand), and
    `testdata/fortran`'s `recdyn.for` (gfortran `-fdec` agreeing), in every mode
    and with paging on; the three also ran once under `/CHECK=BOUNDS`. `recdyn.for`
    is in `testdata/ftn86`'s procedure (`gen.go` rerun) and `ftn86AwaitingVMS`.
    MANUAL.md's 14.13 has "Arrays of derived types as arguments, and dynamic ones".
    Subtask 6, COMMON, EQUIVALENCE, and DATA, is next.
- **2026-11-18, subtask 6 (done):** COMMON, EQUIVALENCE, and DATA (finding 12;
  `sema/seqtypes.go`, `sema/datatypes.go`).
  - *COMMON* took every record already, laid out whole by its `Boundary`; now an
    object of a type without SEQUENCE is NOSEQUENCE (new, 5.5.2), checked after the
    declarations (`checkCommonTypes`), since a TYPE(t) statement may follow the
    COMMON statement. A STRUCTURE's record stays allowed, as VAX FORTRAN has it.
  - *EQUIVALENCE* takes an object of a SEQUENCE type, whole, as an array element,
    or anywhere a name may be (`equivalenceName`); a type without SEQUENCE is
    NOSEQUENCE, a component EQUIVCOMPONENT (new, R547), a STRUCTURE's record and its
    fields refused as before (BADUSE, EQUIVFORM). The type classes (the author's
    answer 6) are `equivalenceClasses`, checked per list: a sequence type whose
    ultimate components are all INTEGER\*4, LOGICAL\*4, REAL\*4, REAL\*8,
    COMPLEX\*8, or CHARACTER is of the default class (`componentsClass`; numbers
    and characters one class, the exception), and shares storage with objects of
    those types and other such sequence types; a sequence type of the other class
    only with objects of its own type (`sameType`); EQUIVTYPE (new) otherwise. A
    list with no derived-type object keeps VAX FORTRAN's rules untouched. "Default"
    is by size, not by `/[NO]I4`: under `/NOI4` an INTEGER component is INTEGER\*2,
    which isn't a numeric storage unit, and so of the other class (ours).
  - *DATA.* The parser's constant list reads a name and a parenthesized list as a
    structure constructor (`parse/decl.go`'s `constant`), its values expressions,
    so `POINT(1.0 / 2, -3)` and `2*PAIR(1, 2)` are values; nothing older could
    follow a name there. An item of a derived type (a whole object, an element, a
    component of a derived type, in an implied-DO list too) takes a constant of its
    type (`dataRecordItem`; DATATYPE for another type, a number, or text), which
    becomes an Initial per scalar component as a declaration's `= value` does
    (`valueInitials`). A component designator (`dataComponent`,
    `dataComponentSlots`) names its items in array element order: `B%FIRST`,
    `L%MARKS(2)`, `L%MARKS` (every element), `PTS%X` (every element's), `L(2)%AT`,
    with MULTIRANK for two ranked parts and NOTIMPL for a section, as for an array;
    a component's substring is `dataSubstring`'s (`L%TEXT(2:3)`). A type
    statement's old `/ value /` takes a constructor the same way. A field of
    anything but a derived-type object stays DATAFORM, and a STRUCTURE's record
    BADUSE, as before (`derivedField`), so no `/NOF90` message changes; DATAFORM's
    text now names components.
  - *Tests:* `sema`'s `TestDataDerived` (every item form's Initials) and
    `TestSequenceStorage` (every message); `parse/testdata/derived.f90`'s BLOCK DATA;
    `testdata/fortran90`'s `seqtypes.f90` (a common block of SEQUENCE objects
    defined again in each unit, given values by BLOCK DATA; DATA of objects,
    elements, and components; an array of a numeric sequence type equivalenced with
    an INTEGER array; in `compare.py`, gfortran printing the same lines) and
    `seqshare.f90` (a character and a numeric sequence type and a CHARACTER
    variable sharing storage, VAX's extension, not gfortran's), in every mode and
    with paging on. MANUAL.md's 14.13 has "Derived types in COMMON, EQUIVALENCE, and
    DATA". Subtask 7, I/O and NAMELIST, is next.
- **2026-11-18, subtask 7 (done):** I/O and NAMELIST (findings 13 and 14;
  `sema/derivedio.go`, `ir/lowerio.go`'s `objectItems`, FTNRTL's
  `ftn_namelist.mar`).
  - *Formatted, list-directed, and internal file statements.* An item of a derived
    type is no longer AGGREGATEIO: lowering takes it apart (`objectItems`, while
    `lowerer.byComponents` says the statement is formatted). `sema.Components` is
    an object's ultimate components in order, each a `FieldRef` built on the object
    (`fieldOf`: on a Var, an Elem, or a scalar FieldRef), a component of a derived
    type taken apart in turn, an array component a whole array field (one class A
    descriptor, as an array field in a list always was), an array component of a
    derived type every element of it (`wholeRanked`); a constructor's values and a
    constant's `Parts` are its components as they are. An object that is none of
    these (a function's value, MERGE's) is put in its place by `mem` and a symbol of
    lowering's own bound to it (`sema.NewSymbol` now gives such a symbol its type's
    `Struct`, which `fieldMem` reads). An array of a type, or a whole array in a
    list, which a list holds without its shape (`sema.Shaped` gives it one), goes
    element by element through `eachElement`, each element's components together:
    the standard's order (9.4.2). FTNRTL is unchanged for these. A STRUCTURE's
    record stays AGGREGATEIO, as the HP manual says.
  - *Unformatted.* An object is its bytes, as a record always was. Past 65,535
    bytes, where a class S descriptor's word can't say its length, the item is a
    class A descriptor of that many bytes (BU elements), which FTNRTL's
    `FTN$$UNF_ITEM` takes by its longword ARSIZE: one item, not the pieces finding
    13 planned, and no FTNRTL change. RECORDIOSIZE is left only for a whole array
    whose elements are that long (its descriptor's element length is a word).
  - *NAMELIST* (the author's answer 4). A scalar object of a derived type in a group
    is entities (`Namelist.Entities`, `namelistObject`): a header named for the
    object, then for each component an entity named `P%X`, a component of a derived
    type its own header and entities (`T%AT`, `T%AT%X`, `T%AT%Y`), an array
    component an array entity. A header's descriptor is class Z (new, `ir.ClassZ`,
    DSC$K_CLASS_Z = 0, which no real descriptor uses), its length the number of
    entities after it that are the object's, nested headers among them, its POINTER
    0. FTNRTL: output passes over headers, so `P%X     =   1.500000    ,` is written
    as any entity is (the names padded to 8, as FORRTL pads any name); a name may
    hold `%` (`IS_NAME_CHAR`), so `P%X = 9.0` and `T%MARKS(2) = 7` find their
    entities; and a header named, `P = 1.25, 2.5`, makes its first entity current
    with the rest of the object's still to come (`PARTS`, `PARTP` in the frame),
    which `PUT_VALUE` and `NULLS` go on to when an entity's elements run out
    (`NEXT_PART`) before they report TOOMANVAL, whose message names the last
    entity. The block keeps VMS's layout; a program compiled before still works.
    No new entry, so the GSMATCH stays 1,24. An array of a derived type, or an array
    component of one, in a group is NOTIMPL (ours): its entities would need an
    element's subscripts inside a name (`PTS(2)%X`), FTNRTL's name reader then
    reading through parentheses, which waits for Phase 109's Fortran 90 NAMELIST
    form. A STRUCTURE's record stays NMLITEM.
  - *Unconfirmed* (no VMS has a Fortran 90 run-time library to ask): a header
    named with subscripts (`P(1) = ...`) applies them to its first component; and
    the `?` listing at a terminal names the components, not the objects.
  - *Tests:* `sema`'s `TestDerivedIO` and `TestNamelistEntities`, and `TestErrors`'
    RECORDIOSIZE case changed to a whole array; `fortran`'s `TestDerivedIOCode` (a
    header's descriptor, a recursive unit's block in its frame, an 80,000-byte
    item); `testdata/fortran90`'s `typeio.f90` (in `compare.py`, gfortran printing
    the same lines) and `typenml.f90` (FTNRTL's `$GRP` form, worked by FTNRTL's and
    FORRTL's rules), in every mode and with paging on. MANUAL.md's 14.13 has
    "Derived types in I/O lists and NAMELIST groups". Subtask 8, programs end to
    end, is next.
- **2026-11-18, subtask 8 (done):** Programs end to end.
  - *The programs* (`testdata/fortran90`, worked by hand, in every mode and with
    paging on): `geometry.f90`, a SEQUENCE type VEC with `+`, `-`, and `*` (a REAL
    times a VEC) extending the intrinsic operators and `.DOT.`, `.CROSS.`, and a
    unary `.ROT.` as defined operators, each an external function with an interface
    body, computing a triangle's area, centroid, and perimeter and a polygon's area
    and bounds, moving an array of VECs by component; `stock.f90`, a parts inventory
    in an array of a type, read whole from the terminal by a format, totaled by its
    components as arrays, written unformatted and read back into a section whose
    extent is the count just read, sorted by swapping whole parts; `accounts.f90`, a
    SEQUENCE type shared by a common block, interface bodies, and three external
    procedures, one returning an 18-byte object through its hidden argument as an
    output item. All three are in `compare.py`, gfortran printing the same lines,
    and each of them and subtask 7's two printed the same under `/CHECK=BOUNDS`
    (their scratch files aside, run there without a volume).
  - *A fix.* A unary defined operator starting an output item, `WRITE (*, F) .ROT.
    V`, was a syntax error: after a `)` the lexer leaves a period, a word, and a
    period apart, since there they may be DEC's record field reference, and only
    the binary reading put them back together. `parse`'s `unary` now reads them as
    a unary defined operator where an operand must start, where no record can come
    before them (`parse/testdata/procs.f90`'s new WRITE). Nothing else was found:
    the programs compiled and ran as written otherwise.
  - Subtask 9, the close-out, is next.
- **2026-11-18, subtask 9 (done):** The close-out. FORTRAN-INTERNALS.md's section
  30, "Fortran 90: derived types": one `Structure` behind two front doors, what
  makes two types one and why an interface body repeats a SEQUENCE definition, a
  ranked component as another stride (with its loop's code), constructors and
  results in R0 and R1, and I/O, the compiler taking objects apart and the
  NAMELIST header; section 14's NAMELIST part points to it. MANUAL.md's 14.13 had
  each subtask's part already; HELP FORTRAN's Fortran_90 topic has a paragraph on
  derived types (`helplib.hlb` regenerated). PLAN.md's summary. Subtask 4's
  leftover: `structs.f90` prints its REALs by formats, objects whole, so gfortran
  now prints the same lines (its `.out` worked again by hand).

## Where things stand

Phase 105 is done. What it left, and where each goes:

- **Phase 106:** PRIVATE components and access-specs, and types shared by USE (a
  module's type is one type everywhere, SEQUENCE or not). *(Resolved in Phase 106.)*
- **Phase 107:** POINTER components, a type pointing to itself, and the I/O
  refusals they bring. *(Resolved in Phase 107.)*
- **Phase 109:** an array of a derived type, or an array component of one, in a
  NAMELIST group (NOTIMPL: FTNRTL would read an element's subscripts inside a
  name, `PTS(2)%X`), with Fortran 90's NAMELIST form. *(Resolved in Phase 109.)*
- **Phase 110:** the debugger's EXAMINE of `P%X` and `R.F` (an object is shown as a
  record's longwords until then), with the rest of the `/DEBUG` table's growth;
  `/STANDARD` for the phase's forms. *(Resolved in Phase 110.)*
- **Phase 112:** default initialization of components (NOTIMPL), Fortran 95's.
- **Not planned, ours:** a statement function of a derived type (NOTIMPL; *resolved in
  Phase 113*); a section
  in DATA's component designators (NOTIMPL, as for an array); RECORDIOSIZE for a
  whole array of records whose elements pass 65,535 bytes.
- **Unconfirmed choices of ours:** EQUIVALENCE's storage classes are by size, not
  by `/[NO]I4` (an INTEGER\*2 component makes a type of the other class); a
  NAMELIST header named with subscripts applies them to its first component; the
  `?` listing at a terminal names the components, not the objects; an unformatted
  object past 65,535 bytes is one item, an array of bytes; the NAMELIST header's
  descriptor is class 0, govax's own convention, which no VMS run-time library
  reads.
- **Awaiting the next VMS round:** `testdata/fortran`'s `recarrays.for` and
  `recdyn.for`, in `testdata/ftn86`'s procedure and `ftn86AwaitingVMS`. *(Still awaiting: Phase 113's A.9.5.)*
