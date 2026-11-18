# Phase 103 — Fortran 90: procedures and interfaces

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A, below, with a revised subtask list
and six questions for the author), was written 2026-11-17, for review.
Needs Phase 102 (done).

## Goal

Fortran 90's procedures (the standard's chapter 12):

- **internal procedures**: a main program's or a subprogram's CONTAINS part,
  each internal procedure a scope whose host is its container, seeing the
  host's names by **host association** (12.1.2.2.1), and reaching a host's
  frame by Decision 11's hidden argument when the host's storage is there;
- **interface blocks** (`INTERFACE … END INTERFACE`) describing external and
  dummy procedures; an **explicit interface** (an internal procedure's, an
  interface block's, and, in Phase 106, a module procedure's) checked at every
  call: the number, types, kinds, and ranks of the arguments;
- **keyword arguments** (`CALL SOLVE(A, TOL=1.0E-6)`) and **OPTIONAL** dummies
  with **PRESENT**; an absent argument is VMS's null argument (Decision 13);
- **INTENT(IN/OUT/INOUT)**: an INTENT(IN) dummy not changed, an INTENT(OUT)
  one not read before it's set where the compiler can tell (`/WARNINGS`);
- **generic interfaces** (`INTERFACE SWAP` with several specific procedures),
  resolved by type, kind, and rank at compile time, with the standard's
  ambiguity rules checked at the block (14.1.2.3);
- **defined operators** (`INTERFACE OPERATOR(.CROSS.)`, `OPERATOR(+)` on new
  types) and **defined assignment** (`INTERFACE ASSIGNMENT(=)`);
- a FUNCTION whose result is an array or has a run-time length, when its
  interface is explicit, waits for Phase 104 (its result is passed through a
  descriptor).

## What earlier phases leave in place

- RECURSIVE and RESULT (Phase 89); ENTRY; alternate returns; procedure dummies
  (Phase 79); `%VAL`, `%REF`, `%DESCR`, `%LOC`; the argument list's
  conventions (Table 6-1; `sema/arguments.go`).
- The scope chain (Phase 100), which internal procedures and interface bodies
  give its first hosts.
- `checkReferences`: a file's units checked against each other (FUNCTYPE,
  ARGCOUNT), which explicit interfaces make exact.

## Design notes

- **Nesting in the AST.** `ast.Unit` gains `Contains []*Unit`; the parser's
  unit forming (`Parse`) learns CONTAINS and that an internal procedure's END
  must name its kind (`END SUBROUTINE`).
- **Where an internal procedure's code goes:** after its host's in the same
  `.MAR` module, its entry a local name (`host.$Pn`, Decision 10), so it can't be
  called from outside, as the standard says.
- **Host association's code.** A host variable in `$LOCAL` is named directly; one
  in the host's frame is `n(Rx)` with Rx loaded from the hidden argument. The
  optimizer must know that a call of an internal procedure may change any host
  variable it can see (`opt/effects.go`'s call rule widens).
- **Generic resolution** is one function over candidate specifics, explained
  with the standard's own example; a reference matching none or two is an error
  naming them.
- **Defined operators** are parsed as any `.name.` operator, with the standard's
  precedence (a unary defined operator highest, a binary one lowest), and DEC's
  record field reading tried first (`PHASE-99`'s table).

## Open questions

1. Should a call through an implicit interface to a routine in the same file
   be checked against its definition as an explicit one would be? Today's
   ARGCOUNT says yes for counts; the plan extends it to kinds and ranks as
   warnings.

## Subtasks (provisional)

Replaced by Part A's A.4.

1. **Survey** (the standard's chapter 12 and 14.1; the Handbook's chapter 12),
   for review.
2. **CONTAINS and internal procedures** with static hosts.
3. **Hosts in the frame**: the hidden argument; recursive hosts; `/RECURSIVE`
   everywhere.
4. **Interface blocks and explicit interfaces**, checked.
5. **Keyword arguments, OPTIONAL, PRESENT, INTENT.**
6. **Generic interfaces.**
7. **Defined operators and assignment.**
8. **Programs end to end**, every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 28 (nested scopes, host
   association, and resolving a call), MANUAL.md.

# Part A — the survey (subtask 1)

Read for it: the standard's section 12 (12.1.2.2 internal procedures and
12.1.2.2.1 host association, 12.3 interfaces, 12.3.2.1 interface blocks and
their generic, operator, and assignment forms, 12.4.1 the actual argument
list and its association rules, R1216 to R1222's constraints), 14.1.2.3 and
14.1.2.4 (unambiguous generics, resolving a reference), 5.1.2.3 (INTENT),
3.2.5 and 7.1.3 (defined operators); the Handbook's 12.5.5 and 12.5.6
(optional arguments, intent); the HP manual's component references; and
what the lexer, parser, sema, the IR, codegen, and the optimizer do today.

## A.1 What the standard asks

- **Internal procedures** (12.1.2.2, R210, R1215 to R1222). A main program,
  an external subprogram, or a module subprogram may end with a CONTAINS
  statement and **one or more** internal subprograms (R210: CONTAINS alone is
  an error in Fortran 90). An internal procedure is an external one but for
  four things: its name is **not global**; it may not contain an ENTRY
  statement nor a CONTAINS of its own; it may **not be an actual argument**
  (R1213's constraint), so it can never be called except from its host or
  its host's other internal procedures; and it sees its host's entities. Its
  END must name its kind (`END SUBROUTINE`, R1218's and R1222's
  constraints). Its interface is **explicit** in its host and its siblings.
- **Host association** (12.1.2.2.1). An internal procedure sees its host's
  variables, constants, procedures and interfaces, derived types, and
  **namelist groups**, by the same names and with the same attributes. A name
  is the internal procedure's own, hiding the host's, if it appears there in
  one of the standard's sixteen ways: a type declaration, a dummy argument, a
  result name, the procedure's own name, COMMON, DATA, EQUIVALENCE, DIMENSION,
  PARAMETER, SAVE's or TARGET's entity list, INTRINSIC, NAMELIST, a generic
  name, an ENTRY name, a statement function, a construct name. Anything else
  it mentions is the host's if the host has it, **wherever in the host the
  host mentions it**, executable part included: `DO I = ...` in both, with I
  declared in neither, is one I (the classic mistake, and the standard's
  meaning). A name neither has is the internal procedure's own, typed by the
  implicit rules in force, which are the host's except for the letters the
  internal procedure's own IMPLICIT statements change (5.3). Statement labels
  are never shared: each scoping unit has its own.
- **Interface blocks** (12.3.2.1, R1201 to R1206). `INTERFACE [generic-spec]`,
  interface bodies (a FUNCTION or SUBROUTINE statement, a specification part,
  an END), MODULE PROCEDURE statements (only where a module is reachable),
  END INTERFACE. A body declares the name an external procedure, or, when the
  name is a dummy argument's, a dummy procedure, with that interface. A body
  holds no ENTRY, DATA, FORMAT, or statement function, and **does not see its
  host** ("an interface body does not access the named entities by host
  association", 12.1.2.2.1; its implicit rules are the defaults, 5.3): a kind
  written with the host's PARAMETER is an error there, and Fortran 90 offers
  only USE in its place (IMPORT is Fortran 2003's). A procedure has at most
  one explicit interface in a scope; a body for a procedure the same
  subprogram defines is an error; none may appear in BLOCK DATA.
- **When an interface must be explicit** (12.3.1.1): a reference with a
  keyword, by a generic name, as a defined operator or assignment; a
  procedure with an OPTIONAL dummy, an array-valued or pointer result, an
  assumed-shape, POINTER, or TARGET dummy, or a CHARACTER result of a
  length neither assumed nor constant.
- **The argument list** (12.4.1). Positional arguments first, then keyword
  arguments (`NAME = actual`), only with an explicit interface, each keyword
  a dummy's name in the interface the caller sees (an interface body's names
  may differ from the definition's). Each nonoptional dummy gets exactly one
  actual, each optional one at most one. An actual's **type and kind must
  equal** its dummy's (12.4.1.1); with an explicit interface the compiler can
  check it. A scalar dummy takes only a scalar; an array dummy takes an array
  or, by **sequence association** (12.4.1.4), an array element, whose
  following elements it reaches. Ranks must agree exactly for a generic
  reference, a defined operation, and a defined assignment. A CHARACTER
  dummy's length may not exceed its actual's. An actual for an INTENT(OUT) or
  INTENT(INOUT) dummy must be definable (a variable, not an expression nor a
  vector-subscripted section).
- **OPTIONAL and PRESENT** (5.1.2.6, 13.13.80; the Handbook's 12.5.5). An
  absent dummy must not be referenced or defined, may be passed on only to
  another optional dummy (which is then absent too), and is asked about with
  PRESENT(A), A an optional dummy "accessible in the procedure" (so an
  internal procedure may ask about its host's).
- **INTENT** (5.1.2.3). Only on a dummy, never a dummy procedure or pointer.
  IN: the dummy must not be redefined or become undefined; OUT: undefined at
  entry, defined before it is referenced; INOUT: both ways. With no INTENT,
  the dummy is what its actual allows, FORTRAN-77's rule.
- **Generic interfaces** (12.3.2.1, 14.1.2.3, 14.1.2.4). A generic name stands
  for every specific procedure of its blocks; a reference chooses the one
  whose interface the reference is consistent with. 14.1.2.3 makes that one at
  most: within a scope, two specifics of one generic name must both be
  functions or both subroutines, and one must have a nonoptional dummy that
  differs (absent, another type, kind, or rank) from the other's **by
  position** and one that differs **by keyword**; for operators and
  assignment, by position. A generic name may be a specific's own, and may be
  an **intrinsic's**, which it then extends (14.1.2.4.1: no specific fits, the
  intrinsic is tried). A generic name established in a host is generic in its
  internal procedures too.
- **Defined operators** (3.2.5, 7.1.3, 12.3.2.1.1). `.name.` (1 to 31
  letters, not an intrinsic operator nor `.TRUE.`/`.FALSE.`), or an intrinsic
  operator extended to operands **it has no meaning for** (7.1.3: types or
  ranks not those of the intrinsic operation). Each function takes one or two
  nonoptional INTENT(IN) dummies; a unary defined operator binds tightest
  (R704), a binary one loosest (R723); an extended intrinsic operator keeps its
  precedence. `<=` and `.LE.` are one operator.
- **Defined assignment** (7.5.1.3, 12.3.2.1.2). `INTERFACE ASSIGNMENT(=)`:
  subroutines of two nonoptional dummies, the first INTENT(OUT) or INOUT,
  the second INTENT(IN); `X = Y` is `CALL S(X, (Y))`. It may not redefine an
  assignment Fortran already defines (14.1.2.3's "as if the intrinsic
  consisted of a collection of specific procedures").

## A.2 What govax has

- **The lexer** recognizes CONTAINS, INTERFACE (with whatever follows it),
  END INTERFACE, MODULE PROCEDURE, INTENT, OPTIONAL (Phase 99), and END
  SUBROUTINE and the rest with a name (`endStatement`). A defined operator is
  one `Operator` token (`Token.DefinedOperator`) at an expression's start, after
  an operator, or after a constant; **after a name or a `)`** it is left as
  `.`, a name, `.`, since `A.B.C` may be DEC's record fields
  (`definedOperator`'s comment: "for the parser and the symbols to settle
  (Phase 103)").
- **The parser** keeps CONTAINS, INTERFACE, and TYPE as `PendingStmt`s and
  gathers what they hold into `PendingStmt.Inner` (`nesting`), checking the
  inner ENDs (`checkEnd`: ENDNOKIND for an internal procedure's bare END).
  `ast.Unit` is flat (`Header`, `Body`); there is no node for an interface
  block or an internal procedure. A function reference's argument list takes
  `NAME = expr` (`subscripts`, `ast.KeywordArg`); **CALL's doesn't**
  (`parseCall`). Binary operators are a precedence table (`binaryOps`, 0 to
  8), with no place for a defined one; `referenceAfter` reads `A.B` as a
  field whatever A is.
- **sema.** The scope chain is in place (`scope.go`: `find`, `define`,
  `enter`, `rules`; hosts so far only for a statement function's dummies).
  Its comment says namelist groups and structures "aren't host-associated",
  which is right for COMMON blocks but **wrong for namelist groups**
  (12.1.2.2.1 lists them) and, by Decision 9's sharing of one model, for
  structures. `implicitRules` is a whole table per scope, nil to take the
  host's: there is no "the host's, with these letters changed". `Analyze`
  takes the file's units one after another, each with its own analyzer,
  then `checkReferences` compares references across units: FUNCTYPE and
  ARGCOUNT are **errors** today for same-file calls through implicit
  interfaces, with `constantArguments` and `recordArguments`. Arguments
  are classified by `sema/arguments.go` (Table 6-1's modes); an intrinsic's
  keyword arguments by `intrinsickeys.go` (KEYWORDNAME, KEYWORDTWICE,
  KEYWORDORDER), and any other `KeywordArg` is NOTIMPL naming this phase.
  INTENT, OPTIONAL, and TARGET are checked for placement (on a dummy:
  `attrtable.go`) and kept on the `Symbol` (`Intent`, `Optional`, `Target`),
  meaning nothing (`attrRecorded`). An assumed-shape dummy is NOTIMPL
  (`arrays.go`).
- **The IR and codegen.** Each unit is an `ir.Routine`; the file is one
  `.MAR` module. A routine's symbols are `ROUTINE.name` (`codegen/names.go`),
  a numbered `.$Nn` form past 31 characters. **Two mechanisms this phase
  reuses:** a unit with ENTRY statements copies each dummy's address into a
  cell at every entry (`ir.Param.Through`, `$A1`), and an adjustable array's
  bounds sit in a `$Bn` descriptor cell; and an address computed at run time
  is an `ir.Mem.Ptr` (an A0, a text pointer), which the optimizer hoists and
  keeps in a register (Phase 84). Phase 89's variable format expressions are
  the one precedent for code reaching another routine's activation: their
  `.WORD`-entered blocks take the I/O routine's AP and FP as arguments
  (`writeVFE`; a frame cell then a displacement from R11). A `.WORD` entry is
  local, where `.ENTRY` makes a global symbol.
- **The optimizer** (`opt/effects.go`): a call may change COMMON, a dummy, and
  a "taken" local (one whose address was let out); a RECURSIVE unit's static
  locals too (`ChangedByCall`). Places are regions of the routine's own
  (local, frame, data, descriptors).

## A.3 Findings, and what they change

1. **The tree nests.** `ast.Unit` gains `Contains []*Unit` (internal
   procedures, each with its `Header`, `Body`, and END) and an interface block
   becomes a statement of the unit's body, `ast.InterfaceBlock` (its generic
   spec: none, a name, `OPERATOR(op)`, or `ASSIGNMENT(=)`; its `Bodies
   []*Unit`; its MODULE PROCEDURE names), built from what `nesting` gathers
   today. The parser checks what is syntax: CONTAINS with nothing after it
   (EMPTYCONTAINS), a CONTAINS inside an internal procedure (NESTEDCONTAINS),
   an ENTRY in one (ENTRYINTERNAL), ENTRY, DATA, FORMAT, or a statement
   function in an interface body (BADINBODY). A derived type's definition
   stays a `PendingStmt` until Phase 105, MODULE PROCEDURE NOTIMPL until 106.
   Old programs' trees don't change.
2. **sema analyzes a host in four steps**, because each needs something the
   one before gives and 12.1.2.2.1 makes a name the host's wherever the host
   mentions it:
   1. the host's declarations (as today);
   2. each internal procedure's **interface**: its header and declarations, so
      the host's statements can type `F(X)` and check `CALL S(A, B)`;
   3. the host's executable part, which enters its implicitly typed names;
   4. each internal procedure's executable part, whose lookups now find every
      host name, implicit ones included.
   Step 2 may meet a name its declarations don't declare and the host hasn't
   mentioned yet (a bound `REAL W(N)` with N only in the host's executable
   part): 7.1.6.2 allows only a host's (or a constant, dummy, or COMMON) name
   there, so it is entered as the host's, and if the host never mentions it
   after all, it is reported (BADSPECNAME). Each internal procedure's scope has
   the host's as its host; `implicitRules` becomes copied from the host's and
   changed by the internal procedure's own IMPLICITs (a whole new table for
   IMPLICIT NONE); namelist groups and structures are found through the chain
   as names are (`scope.go`'s comment corrected). A host's statement
   functions are expanded in an internal procedure as anywhere else.
3. **What the host may and may not do with its internal procedures.** Each is a
   procedure name of the host's scope with an explicit interface (finding 6);
   a type statement, EXTERNAL, or INTRINSIC for it in the host is a second
   interface (DUPINTERFACE, our reading of 12.3.2.1's "not more than one
   explicit specific interface"); two of one host with one name, or one
   with the host's own name, is DUPNAME; one as an actual argument is
   INTERNALARG (LIB$ESTABLISH, `$DCLAST`, and LIB$CALLG included: an internal
   procedure is never an AST routine or a handler). Its name isn't global:
   `unitNames` doesn't see it, so it may share a name with an external
   procedure of the file, which it hides in its host. A FUNCTION calling itself
   still needs RECURSIVE (Phase 89).
4. **Internal procedures in the object.** Each is an `ir.Routine` written after
   its host in the same module, entered at a **local** label with a `.WORD`
   mask as a variable format expression's block is, and called with `CALLS`.
   Its name: Decision 10 says `host.$Pn`; this survey proposes **`HOST.INNER`**,
   and its own symbols `HOST.INNER.X` (question 2). Two hosts in one file may
   each contain an INNER, and the file may have an external INNER too, so the
   prefix must be qualified; a period can't be in a FORTRAN name, so
   `HOST.INNER` collides with nothing of the host's (whose symbols are
   `HOST.name` for its own names, and INNER isn't a variable there), and it
   reads in a listing, a map, and later the debugger as the path it is. A
   module procedure's internal procedure (Phase 106) is then `M.P.INNER`. A
   long name takes the existing `.$Nn` form.
5. **How an internal procedure reaches its host.** Every host entity it can
   reach is a **cell** already, or can be made one:
   - a host variable in `$LOCAL`, a host COMMON variable, a host's `$PDATA`
     datum: named directly, `HOST.X`, as the host names it;
   - a host **dummy**: through the host's `Param.Through` cell, the ENTRY
     mechanism, which the host fills at entry for each dummy an internal
     procedure reaches (a host's adjustable array through its `$Bn` cell
     likewise, and a passed-length CHARACTER dummy through its descriptor's
     address in the cell);
   - a cell in the host's **frame** (a RECURSIVE host's, an AUTOMATIC
     variable, and those Through cells when the host is reentrant): through
     the host's FP, which each call of the internal procedure passes as a
     **hidden last argument** (Decision 11), and the internal procedure
     reaches as an `ir.Mem.Ptr`, the host FP plus the cell's displacement, so
     the optimizer's A0 machinery hoists the pointer and the allocator may keep
     it in a register, with no new addressing in codegen. A sibling or a
     recursive call passes its own hidden argument on.
   So the hidden argument exists only when the host has something in its frame
   that the internal procedures reach (Decision 11 as written), and a static
   host's dummies cost one MOVL each at entry. Since an internal procedure is
   never called once its host has returned (finding 3), a static host's cells
   always belong to the activation that called it. The alternative, Phase 89's
   pair of hidden arguments (the host's AP and FP) on every call, is
   question 3.
6. **Explicit interfaces are one structure.** `sema.Interface`: function or
   subroutine; the result's type (and rank, for Phase 104); each dummy's name,
   type, kind, rank and shape, CHARACTER length, INTENT, OPTIONAL, and whether
   it is a procedure (with its own interface, if declared) or an alternate
   return `*`. It is made from an internal procedure's declarations (finding
   2's step 2), from an interface body, and in Phase 106 from a module
   procedure. A reference through one is checked by one function: count
   (ARGCOUNT, as today), keywords (finding 8), type and kind (ARGTYPE, an
   error: 12.4.1.1), rank (ARGRANK: a scalar to an array dummy only if it is an
   element, sequence association; exactly equal for a generic or a defined
   operation), CHARACTER length when both are constant (ARGLENGTH), and
   definability for INTENT(OUT) and INOUT (NOTDEFINABLE). A function's type
   comes from its interface, never from implicit typing in the caller. Where
   a procedure's interface would need Phase 104 (an array-valued result, an
   assumed-shape dummy, a run-time CHARACTER length) or Phase 107 (a POINTER
   dummy or result), it is NOTIMPL naming that phase.
7. **Interface bodies are scopes with no host.** Each body is analyzed as a
   unit's declarations alone, in a scope whose host is nil and whose implicit
   rules are the defaults, making its `Interface` and no code. A body for an
   external procedure the same file defines is compared with the definition
   (INTERFACEDIFF, naming the first difference: 12.3.2.1 says they "must be
   consistent", and we can see both); a body whose name is a dummy makes it a
   dummy procedure with that interface (`Call.Through`, as today). A
   procedure with an OPTIONAL dummy, called in the same file with no explicit
   interface, is EXPLICITREQ (Decision 7's rule; elsewhere the program's own
   mistake). The plan's open question 1, the same-file checks through an
   implicit interface, is question 1: the HP compiler's answer is a
   `/WARNINGS=ARGUMENT_CHECKING` keyword, and FUNCTYPE and ARGCOUNT stay as
   they are either way, so no old program hears anything new.
8. **Keyword arguments.** CALL's list takes `NAME = actual` as a function
   reference's does (the parser; `KEYWORD=` in CALL can't be anything else,
   since `CALL S(X=1)` was never valid). sema reorders the actuals into the
   interface's order before anything else looks at them, with the intrinsic
   path's messages generalized (KEYWORDNAME, KEYWORDTWICE, KEYWORDORDER), plus
   KEYWORDNOINTERFACE for a keyword with an implicit interface and ARGMISSING
   for a nonoptional dummy left without one. An alternate return `*label` is
   positional only.
9. **OPTIONAL and PRESENT** (Decision 13). An absent argument is a 0 longword
   in a full-length list, so the callee's argument count is always its
   dummies' (plus the hidden one). Passing an absent dummy on needs nothing:
   a dummy passed by reference passes its address, which is the 0, and a
   CHARACTER dummy is passed on with its own descriptor (Phase 78), whose
   address is the 0. PRESENT(A) is a test of A's longword: `TSTL n(AP)`, or of
   its Through cell; in an internal procedure, of the host's. VMS's own
   convention also lets a caller write a shorter list (LIB$ routines treat a
   missing trailing argument as absent), and a MACRO or old FORTRAN caller of a
   Fortran 90 routine may do so; PRESENT then needs a count test too
   (`CMPB (AP),#n`), question 4. The code a routine runs at entry must never
   read through an absent dummy: an adjustable array's descriptor fill only
   computes with the address, and a passed-length CHARACTER's length is read
   at each use, so nothing does today; subtask 6 holds that with a test (an
   absent dummy of every kind, at address 0, which faults if touched).
10. **INTENT.** IN: a dummy as an assignment's target, a DO variable, an input
    item, an IOSTAT= or other returned specifier, ASSIGN's variable, or an
    actual for an INTENT(OUT) or INOUT dummy of an explicit interface, is
    INTENTIN (an error: 5.1.2.3's "must not"); through an implicit interface
    nothing can be said. OUT: a reference that comes before every definition in
    the routine's statement order is INTENTOUTREF, a warning under
    `/WARNINGS` (the plan's "where the compiler can tell", in its simplest
    sound form, not a flow analysis). INTENT changes no code, and the optimizer
    gains nothing from it yet.
11. **Generic resolution** (`sema/generic.go`, Decision 13). One function takes
    a reference's actuals (with their types, kinds, and ranks, Phase 102's
    `Shape`) and a generic's specifics, and returns the one whose interface the
    reference is consistent with by finding 6's checks, else the intrinsic of
    that name when the name is one (14.1.2.4.1's (2) and (4)), else
    GENERICNONE naming the actuals' types. Its comment works the standard's own
    `INTERFACE A` example (AR and AI, and why `CALL A(0.0)` would be ambiguous
    with J REAL). The ambiguity check runs when the blocks of a scope are
    complete, over each pair of specifics of one generic name, the two rules of
    14.1.2.3 written as they are (GENERICAMBIG, naming both). A generic in the
    host is generic in its internal procedures (14.1.2.4.1(d)), and a block in an
    internal procedure adds to it there.
12. **Defined operators and the record fields.** `A.B.C` with A a record is DEC's
    field B, then C; with A anything else, A `.B.` C. The two readings make
    different trees (`X + A.B.C * 2` is `X + (A.B.C)*2` one way and `(X + A)
    .B. (C*2)` the other, the defined binary operator binding loosest), so sema
    can't turn one into the other: **the parser must know**. It knows the
    records: a name is a record only by a RECORD statement (and from Phase 105 a
    `TYPE(t)` declaration), in the specification part, which the parser has read
    before any executable statement, in the unit or, by host association, its
    host. So `referenceAfter` reads `.B` as a field only after a name the parser
    has seen declared a record in this scope or its hosts (or after a field or
    element of one), and otherwise the lexer's `.B.` is an operator token; this
    moves Phase 99's "settled in sema" into the parser (question 6). The
    parser's precedence table gains a unary defined level above `**` and a
    binary one below `.EQV.` (R704, R723); an `ast.DefinedOp` node holds the
    operator and its one or two operands. An intrinsic operator stays an
    `ast.Binary`, and sema, finding no intrinsic meaning for its operands' types
    or ranks, looks for an `OPERATOR(op)` interface (and for `.LE.` the `<=` one:
    one operator).
13. **What a defined operation or assignment may extend.** The standard forbids
    redefining an intrinsic meaning (7.1.3, 14.1.2.3). DEC gives some meanings the
    standard doesn't: `.AND.` and the others on INTEGERs (bitwise), LOGICAL =
    INTEGER and the reverse, `.XOR.`. Question 5: should an interface take
    precedence over a meaning only DEC gives, or is such an interface an error?
    Either way, no old program changes (none has an interface). Defined
    assignment between two numeric types, two CHARACTERs, or two LOGICALs is
    always an error (DEFASSIGNINTR). Until Phase 105's derived types, what may
    be defined is operations and assignments on intrinsic types the standard
    leaves meaningless (`.DIST.` on REALs, `+` on LOGICALs, CHARACTER = LOGICAL),
    with scalar results (an array result is Phase 104's); each operand passes
    as any actual does, the right side of a defined assignment through a
    temporary ("enclosed in parentheses"). Phase 105 brings the derived types the
    feature is for, through the same code.
14. **The optimizer.** In the host, a place an internal procedure reaches is
    taken (any call may change it), computed per host as the union of what its
    internal procedures name; a refinement to what each call's callee reaches
    can wait. In an internal procedure, a host place is a new region, `HostRegion`,
    changed by any call, as COMMON is. No pass changes beyond `effects.go`'s
    rules. An inline expansion of internal procedures is left out.
15. **What is left to other phases.** The cross reference, listings, `/DEBUG`,
    and the debugger: each internal procedure gets its own storage map and cross
    reference, headed `HOST.INNER`, and nothing more until Phase 110 (scopes in
    the cross reference, the debugger's host names). `/STANDARD`: Phase 110.
    MODULE PROCEDURE and USE inside bodies: Phase 106. Elemental user
    procedures are Fortran 95's.
16. **`/RECURSIVE` and the corpus.** Every program with internal procedures is
    run in every mode, so under `/RECURSIVE` every host is reentrant and needs
    finding 5's hidden argument. Subtask 3 reports a host in its frame NOTIMPL
    (naming subtask 4's work), and its programs join `recursiveModes` with
    subtask 4.

## A.4 The subtasks, revised

1. **Survey** (this Part A).
2. **The forms:** `ast.Unit.Contains`, `ast.InterfaceBlock`, keyword arguments in
   CALL, `ast.DefinedOp` with its two precedence levels and finding 12's record
   rule, the parser's checks (finding 1); everything new still NOTIMPL in sema;
   parser golden dumps, `TestNoF90Unchanged`.
3. **Internal procedures with static hosts:** finding 2's four steps, the host's
   implicit rules and namelists through the chain, finding 3's rules, each
   internal procedure's interface checked at its calls (finding 6's count, type,
   kind, rank, length, definability); the routines and their names (finding 4),
   host cells and Through cells (finding 5), the optimizer's rules (finding 14);
   IR golden dumps, runs both optimizer settings.
4. **Hosts in the frame:** the hidden argument, `Mem.Ptr` through the host's FP,
   RECURSIVE and AUTOMATIC hosts, recursive and sibling internal procedures; the
   corpus programs of subtask 3 in `recursiveModes`.
5. **Interface blocks:** bodies as hostless scopes (finding 7), external and dummy
   procedures with explicit interfaces, INTERFACEDIFF, EXPLICITREQ, and
   question 1's checks.
6. **Keyword arguments, OPTIONAL, PRESENT, INTENT** (findings 8 to 10), with the
   absent-dummy test.
7. **Generic interfaces** (finding 11), intrinsic names extended, hosts' generics.
8. **Defined operators and assignment** (findings 12 and 13).
9. **Programs end to end**, every mode, gfortran's witness, the optimizer's
   cases (a host variable kept in a register across a call that changes it).
10. **Close-out:** FORTRAN-INTERNALS.md's section 28 (nested scopes, host
    association, resolving a call, with `DO I` in a host and its internal
    procedure followed through), MANUAL.md's 14.13, HELP FORTRAN.

## A.5 Questions for the author

1. **Same-file calls through an implicit interface** (the plan's open question
   1; finding 7). (a) **A `/WARNINGS=ARGUMENT_CHECKING` keyword, off by default
   (the HP compiler's), warning of type, kind, and rank differences; FUNCTYPE and
   ARGCOUNT stay errors as today**; (b) those warnings always under `/F90`; (c)
   always, as the plan proposed (old corpus programs that pass an INTEGER to a
   REAL dummy would start to warn).
2. **Internal procedures' names** (finding 4). (a) **`HOST.INNER`, its symbols
   `HOST.INNER.X`**; (b) Decision 10's `HOST.$Pn`, its symbols `HOST.$Pn.X`.
3. **How a host is reached** (finding 5). (a) **Decision 11 as written: a static
   host's cells by name, its dummies through Through cells, the host's FP as a
   hidden argument only when something reached is in its frame**; (b) Phase 89's
   pair, the host's AP and FP, on every call of an internal procedure that reaches
   a host dummy or frame cell, with no Through cells.
4. **PRESENT and a shorter argument list** (finding 9). (a) **Calls govax
   compiles always pass the full list, 0 for an absent argument (Decision 13),
   and PRESENT also tests the count, so a caller using VMS's shorter list is
   understood**; (b) PRESENT tests only the longword.
5. **Defined operations over DEC's meanings** (finding 13). (a) **An interface
   takes precedence over a meaning only DEC gives (INTEGER `.AND.`, LOGICAL =
   INTEGER, `.XOR.`), never over one the standard gives**; (b) DEC's meanings
   count as intrinsic, so such an interface is GENERICAMBIG.
6. **Where `A.B.C` is settled** (finding 12). (a) **In the parser, by the names
   RECORD statements (and Phase 105's TYPE declarations) declare in the scope and
   its hosts: a field of a record wins**, as the HP manual has it; (b) in sema as
   Phase 99's table said, which would need the parser to build both trees for
   every such expression and sema to choose one.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-17, subtask 1:** The survey written (Part A): the standard's
  chapter 12, 14.1.2.3 and 14.1.2.4, INTENT, and defined operators read, and
  the passes that meet them. Its main findings: host association reaches every
  name the host mentions, executable part included, so sema analyzes a host in
  four steps (finding 2); every host entity an internal procedure reaches is a
  cell, a static host's named directly and its dummies through the ENTRY
  mechanism's Through cells, a frame's through the hidden FP as an
  `ir.Mem.Ptr` (finding 5); interface bodies are scopes with no host, so a kind
  from the host's PARAMETER is an error in one (finding 7); `A.B.C` can only be
  settled in the parser, by the names RECORD statements declare (finding 12);
  CALL's list doesn't take keywords yet (finding 8). Six questions for the
  author (A.5). For review.
