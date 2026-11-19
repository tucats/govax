# Phase 106 — Fortran 90: modules

**Status:** done 2026-11-19. Planned 2026-11-17 with Phase 99 (`PHASE-99 -
Fortran 90.md`, Part C). Subtask 1, the survey (Part A), done 2026-11-18 and
accepted by the author the same day; subtasks 2 to 8 done 2026-11-18 and
2026-11-19. "Where things stand", at the end, is what the phase left.

## Goal

Modules and separate compilation (the standard's 11.3), by Decision 10:

- **MODULE … END MODULE**: data, derived types, interface blocks, generic names,
  defined operators, and module procedures after CONTAINS;
- **USE**, with renames (`USE M, LOCAL => REMOTE`) and ONLY lists
  (`USE M, ONLY: X, OPERATOR(+)`); **use association** joins sema's scope chain
  (Decision 4), and a name brought in twice from different places is an error
  only when referenced (the standard's 11.3.2);
- **PUBLIC and PRIVATE**, as statements and attributes, default and per name,
  and PRIVATE components of a public type;
- **module files** (`M.F90$MOD`, our own text form, Phase 99's question 3):
  written when a module compiles, read by USE, found beside the source, in
  `/MODULE_PATH`, by `FORT$MODULES`, or where `/MODULE=dir` wrote them;
- **the object**: module variables in a psect per module (`$MOD$M`, laid out
  as COMMON), module procedures named `M.P` (Decision 10), the module's own
  `.MAR` file assembled and linked like any other;
- several modules and their users in one file, in dependency order; a module
  used before it is compiled reported clearly (`MODNOTFOUND`, naming where it
  looked);
- `/LIST`'s storage map and cross reference showing where each name came from.

## What earlier phases leave in place

- The scope chain, explicit interfaces, generics, defined operators (Phase 103),
  derived types (Phase 105).
- COMMON's layout and psect writing (`codegen`'s `writeCommons`), which module
  data reuses; LINK's overlaid psects.
- The FORTRAN command's file search on host and volume (`dclcmd/fortran.go`,
  `fortranIncludes`), which the module search follows.

## Design notes

- **What a module file holds** (teaching first): a header (the compiler's
  version, the module's name, the source and its time), the modules it uses, then
  every entity in a declaration-like text the compiler's own parser could almost
  read, PRIVATE ones marked (they are needed: a public procedure's interface may
  name a private type). A test reads each back and compares the scope it builds.
- **Recompilation.** A module file is rewritten only when its contents change, so
  a host tool can skip rebuilding users; govax has no MAKE, but MMS-like
  procedures and the manual's examples can rely on it.
- **Names past 31 characters** (`M.P` long) get Decision 10's stable hashed form,
  the same in the module and every user, since both compute it from the full
  name.
- **Module data under `/RECURSIVE`**: module variables are static (they have
  SAVE by the standard's 11.3.4 in practice), whatever the unit's storage rule.

## Open questions

1. Phase 99's question 3 (the module file's type and form).
2. Should the system volume supply module files for govax's definitions
   (`USE $SSDEF`-like modules from `sysdef`), beside INCLUDE's? Proposed: later,
   if wanted.

## Subtasks (provisional)

1. **Survey** (the standard's 11.3, 12.1.2.2.2, 14.1.2; the Handbook's chapter
   11; the HP manual's module chapter for the file conventions), for review
   (done 2026-11-18, accepted; Part A, whose A.4 revises this list).
2. **Modules in one file**: MODULE, USE, use association, the psect and names.
3. **PUBLIC and PRIVATE**, ONLY, renames.
4. **Module procedures**, and generics and operators from modules.
5. **Module files**: writing, reading, the search; separate compilation.
6. **The FORTRAN command**: `/MODULE`, `/MODULE_PATH`, `FORT$MODULES`, HELP; a
   module's `.MAR` and its file on host and volume.
7. **Programs end to end** (`testdata/fortran90/modules/`, built in every order
   allowed), every mode, gfortran's witness.
8. **Close-out**: FORTRAN-INTERNALS.md's section 31 (separate compilation and
   what a module file is), MANUAL.md.

# Part A — the survey (subtask 1)

Read for it: the standard's 11.3 (modules, module references, the USE statement
and use association, and its examples), 4.4.1's accessibility of a type and its
components, 5.1.2.2 and 5.2.3 (the PUBLIC and PRIVATE attributes and
statements), 12.1.2.2 (external, internal, and module procedures; host
association and use association, 12.1.2.2.1 and 12.1.2.2.2), 12.3.2.1
(MODULE PROCEDURE in an interface block), 14.1.1 and 14.1.2 (global and local
names, the classes), 14.1.2.3 (generics accessed from modules), 14.6.1.2, and
14.7.6 (module variables becoming undefined); the *Fortran 90 Handbook*'s
chapter 11; the HP Fortran manual's 8.3 and 5.16 beside them; and what the
lexer, parser, sema, the IR, codegen, the FORTRAN command, and the corpus do
with MODULE and USE today. The HP *language* manual says nothing of module
files (they are its user manual's), so the file conventions below are the
plan's, not DEC's. There is no VMS probe (VAX FORTRAN had no modules); the
standard is the target and gfortran 16.2, `-std=f95`, the witness.

## A.1 What the standard asks

- **A module** (11.3, R1104–R1106). `MODULE name`, a specification part, an
  optional CONTAINS and module subprograms, `END [MODULE [name]]`. The
  specification part may not hold a statement function, an ENTRY, a FORMAT, an
  automatic object, or any executable statement; it may hold type statements,
  derived types, interface blocks, PARAMETERs, COMMON, EQUIVALENCE, DATA,
  NAMELIST, SAVE, IMPLICIT, USE, and PUBLIC and PRIVATE (the HP manual adds
  that AUTOMATIC, INTENT, and OPTIONAL can't be there either). The module's
  name is **global** (14.1.1): it may name no other program unit, external
  procedure, or common block, nor any local name in the module. A module is
  **host** to its module procedures, which see its names by host association
  (12.1.2.2.1); a module subprogram may have internal procedures of its own,
  and its specification part may have the FORMATs, ENTRYs, and statement
  functions the module's may not.
- **A module reference** (11.3.1). `USE M` needs M's public part *available
  when the reference is processed*, so M is compiled first (in the same file,
  earlier; or before, separately). A module may not use itself, directly or
  through others.
- **USE** (11.3.2, R1107–R1109). `USE M [, local => remote ...]` gives every
  public entity of M, some renamed; `USE M, ONLY: [list]` only those listed,
  each a name, `local => remote`, or a generic-spec (a generic name,
  `OPERATOR(op)`, `ASSIGNMENT(=)`). Several USEs of one module in a scoping
  unit join: if any has no ONLY, all public entities come, the lists one rename
  list; if every one has ONLY, the only-lists are one list. An entity may come
  in by more than one local name (`USE M, ONLY: X, Y => X`).
- **Use association** (11.3.2, 12.1.2.2.1). A use-associated name is a local
  name of the using scoping unit, with the attributes the module gave it. It
  **hides a host's entity** of the same name (12.1.2.2.1's first rule), where a
  local declaration only does so when the declaration is one of the sixteen
  12.1.2.2.1 lists. It may be **respecified by no attribute** (no type
  statement, DIMENSION, SAVE, COMMON, EQUIVALENCE, DATA), except PUBLIC or
  PRIVATE in a module; it may be in a NAMELIST group. An interface body has no
  host association but does have use association: a USE inside an interface
  body is how it reaches a module's types (12.1.2.2.1's note).
- **Two entities of one name** (11.3.2). Generic interfaces of one name (or
  operator, or assignment) accessible from different places are **one generic
  interface**. Any other two entities of one local name are allowed **only if
  the name is never referenced** in the scoping unit; one entity by two paths
  (11.3.3.8's `JX`, reached as itself from J and as `KX` through K) is one
  entity, not two. Otherwise a use-associated local name must differ from every
  other local name of the unit. gfortran agrees (a referenced `JZ` of 11.3.3.8's
  example is "an ambiguous reference", an unreferenced one is no error; tried
  2026-11-18).
- **Accessibility** (5.1.2.2, 5.2.3, 4.4.1). PUBLIC and PRIVATE, as attributes
  (`REAL, PRIVATE :: X`) or statements (`PRIVATE :: X, OPERATOR(+)`), only in a
  module's own specification part. A statement with no list sets the module's
  default (once); otherwise the default is PUBLIC. An access statement may name
  a variable, a procedure, a derived type, a named constant, a namelist group, a
  generic-spec, or a name the module itself use-associated (making it, or not, a
  public entity of *this* module, 11.3.2; an unmentioned one takes the module's
  default). A **private type**'s name, constructor, objects, and procedures with
  a dummy or result of it are accessible only in its module; R522's constraint
  makes such a module procedure PRIVATE, and forbids a PUBLIC generic for it. A
  public type with a **PRIVATE statement** in its definition has components (and
  so a constructor) accessible only in its module: users declare, assign, and
  pass its objects, nothing more. A private specific of a public generic is
  callable through the generic (5.2.3).
- **Generics from modules** (14.1.2.3). The ambiguity rules hold for every pair
  of specifics of a name *in a scoping unit*, inaccessible specific names
  included: two modules' generic SWAPs joined in a user must still be told apart.
- **MODULE PROCEDURE** (R1205, 12.3.2.1). Only in an interface block with a
  generic-spec, in a module or a unit with use access to the procedures named,
  each a module procedure accessible there.
- **Module variables' lifetime** (14.7.6 (3)). A module variable without SAVE
  becomes undefined when no active program unit uses its module. The standard's
  11.3.3.2 example writes SAVE for that reason. A processor may keep them
  (Fortran 2008 made SAVE implicit).

## A.2 What govax has

- **Recognition** (Phase 99). `lex.Module`, `lex.Use`, `lex.Public`,
  `lex.Private`, and `lex.ModuleProcedure` are statement kinds. The parser
  reads MODULE (`ast.ModuleStmt`), END MODULE, and MODULE PROCEDURE
  (`ast.ModuleProcedureStmt`, gathered into its interface block's
  `ModuleProcedures`), and its unit stack (`parse/units.go`) already has a
  `moduleUnit` role: a FUNCTION or SUBROUTINE after a module's CONTAINS is
  nested in it as an internal procedure is in its host. USE, PUBLIC, and
  PRIVATE are `PendingStmt`s; their lists aren't parsed. The access-spec in a
  derived-type statement (`TYPE, PRIVATE :: T`) is read (`parse/derived.go`).
- **What sema does with them.** A unit with a `ModuleStmt` header is reported
  once, "the MODULE statement is not implemented yet", and not analyzed
  (`pendingHeader`); USE is NOTIMPL wherever it is; PUBLIC and PRIVATE
  statements outside a module are NOTINMODULE, and the attributes the same
  (`attrtable.go`'s `attrModuleOnly`); a type's PRIVATE statement is TYPEACCESS
  outside a module (`derived.go`); an interface block with MODULE PROCEDURE
  statements is NOTIMPL (`generic.go`'s `compiledBlock`). Tried 2026-11-18: a
  module with data, a type, an `OPERATOR(+)` interface, and two module
  procedures, used by a main program with ONLY and a rename, gives exactly the
  MODULE and USE NOTIMPLs.
- **Scopes** (Phase 100, `sema/scope.go`). A scope is a table with a host;
  `find` walks outward; `analyzer.lookup` enters an undeclared name with its
  implicit type. A unit's structures, namelist groups, and generic names are
  host-associated through `Unit.Host`. Nothing yet enters names from elsewhere.
- **Internal procedures** (Phase 103, `sema/internal.go`): a host analyzed in
  four steps (its declarations; each procedure's declarations, its interface;
  the host's statements; each procedure's statements), each procedure's
  interface explicit in the host (`Symbol.Interface`), names `HOST.INNER`,
  `Unit.Path`. A module with module procedures is this shape exactly, with no
  executable statements of its own and no frame (so never Decision 11's hidden
  FP), but with **global** procedures.
- **Explicit interfaces, generics, operators** (Phase 103): `checkInterface`,
  `Generic` with `Host`, `checkGenerics` (14.1.2.3 per unit), defined operators
  and assignment, keyword and OPTIONAL arguments; **types** with a `Structure`
  and `sameType` (Phase 105). All of it is what a use-associated procedure,
  generic, or type needs once it is in the user's scope.
- **COMMON** (Phase 76, `sema/common.go`; `codegen`'s `writeCommons`): a block is
  a `CommonBlock` with a psect, a layout, and attributes; each unit using it
  writes `.PSECT` with its size and a local base label (`BLK.$BASE`), the
  initial values only where a unit has them, and LINK overlays the
  contributions. Module data needs nothing more of codegen: a module's variables
  are such a block, under a psect name of its own.
- **Names** (`codegen/names.go`). A routine's own symbols are `UNIT.NAME`; a
  name past 31 characters gets a **numbered** form, `.$Nn`, counted through the
  MACRO module. That is fine for local symbols, but it is not stable across
  files, so it can't be how a global `M.P` past 31 characters is made: the
  module and every user must compute the same name (Decision 10's hash).
- **The FORTRAN command** (`dclcmd/fortran.go`). One compilation (a source, or
  `A+B` joined) gives one MACRO file and one listing; `/OBJECT` assembles it.
  Outputs go to the **default directory** (`filenames.go`'s
  `defaultOutputLocation`: VMS's rule, the process's default on a volume, the
  host's current directory for a host source), not beside the source; a host
  output's type takes the case of the source's. INCLUDE files are found
  relative to the including file and through text libraries
  (`fortranIncludes`, a `source.Opener`). No `/MODULE` or `/MODULE_PATH`
  qualifier exists.
- **The corpus** (`testdata/fortran90/modules/`): two programs of several files
  (`mixed`, `shortlist`), each linked in every order its `links.txt` lists. The
  files are compiled one by one, in no particular order, which a program of
  modules can't allow.

## A.3 Findings, and what they change

1. **A module is a unit with a scope and a psect.** sema analyzes MODULE as a
   unit of a new kind (`ast.ModuleUnit` exists) by the internal procedures' four
   steps: its specification part, each module procedure's interface, (no
   statements), each module procedure's body. Its specification part is checked
   against 11.3's constraints (MODSTMT naming the statement: a statement
   function, ENTRY, FORMAT, an automatic object, AUTOMATIC, INTENT, OPTIONAL,
   any executable statement). Its variables, initialized or not, SAVE'd or not,
   are laid out as one COMMON block's (`layoutCommon`, the unit's `/ALIGN`),
   psect `$MOD$M` (the plan's), with EQUIVALENCE groups placed in it; a COMMON
   block declared in the module stays its own psect, as COMMON. A module
   variable is always static (the design note; 14.7.6 allows it), even under
   `/RECURSIVE`, and its initial values are in the module's object only. A
   module with no variables writes no psect. A psect name may be 31 characters,
   so a module name past 26 gets the hashed form of finding 5, and a COMMON
   block named `$MOD$...` is refused (MODPSECT), being the only way the two
   could meet.
2. **Use association enters names in the using scope.** A USE (statement order:
   first in the specification part, before IMPLICIT; `order.go`) brings the
   module's accessible entities into the *using unit's own scope*, each a
   `Symbol` marked `Use` with its module and remote name, sharing the module's
   entity (its type, `Structure`, interface, value, place). So a use-associated
   name hides the host's, as 12.1.2.2.1 says, and a later declaration of it
   finds it there: USEATTR (respecified) for a type statement, DIMENSION, SAVE,
   DATA, and the rest, USECOMMON for COMMON and EQUIVALENCE, allowed only in a
   NAMELIST and, in a module, PUBLIC and PRIVATE. A name entered by two USEs for
   two different entities becomes an **ambiguous** symbol, which is an error,
   USEAMBIG naming both modules, only when the program refers to it (11.3.2);
   one entity by two paths is recognized by its origin (module and remote name)
   and is one symbol. Generic names from several places join (`Generic` gains
   each module's specifics), and `checkGenerics` then runs in the user too
   (14.1.2.3), reporting a clash at the USE that made it. ONLY and renames
   (several USEs joined as 11.3.2 says), ONLYPUBLIC for a name the module
   doesn't make public, USE in a module procedure, an internal procedure, and an
   interface body (12.1.2.2.2's example in the corpus). IMPLICIT doesn't come
   with a module: each unit has its own rules.
3. **Type identity across modules.** `sameType` is pointer equality of
   `Structure`s. A type reached by two paths (two USEs, or a module that
   re-exports another's) must be **one** `Structure`, so a compilation keeps one
   table of modules loaded, keyed by name, and an entity's origin decides which
   `Structure` it is. A type from a module file and the same module compiled
   earlier in this compilation are one too (finding 6).
4. **Module procedures are global, with stable names.** A module procedure's
   entry is `M.P` (Decision 10): global, never colliding with an external P;
   its own symbols `M.P.X` and its internal procedures `M.P.Q`, local as
   today's. A `Symbol` gains the name the MACRO file calls it by (`LinkName`),
   which the IR's calls, `RoutineAddr` (a module procedure may be an actual
   argument, unlike an internal one), and codegen's `.ENTRY` use; an external
   procedure's is its name. Interface blocks in a module describe **external**
   procedures (their names unchanged), and an interface body there still has no
   host association: it reaches the module's types only by redefining them as
   SEQUENCE types (Phase 105's rule; Fortran 2003's IMPORT is outside 90).
   MODULE PROCEDURE opens (`compiledBlock`), naming a module procedure of this
   module or a use-associated one (MODPROCNAME otherwise).
5. **The hashed form of a long global name.** `M.P` past 31 characters becomes
   its first 22 characters, `$`, and 8 hexadecimal digits of the full name's
   SHA-256: stable, the same in the module and every user, listed in the storage
   map beside the full name. The same rule makes a long module's psect name.
   (The numbered `.$Nn` stays for local symbols.)
6. **One path for a module's interface: through its text.** What a user needs of
   M (its accessible entities, and the private ones those mention) is made into
   the module file's text when M compiles, and **every** USE of M reads that
   text, whether M was compiled earlier in this compilation (the text in memory)
   or before (the file). So a module in the same file and a module compiled
   separately behave identically, every one-file test also tests the writer and
   reader, and a stale file never shadows a module just compiled (question 3).
7. **The module file is Fortran 90 text** (question 1). Its form follows from
   what Fortran can already say: the module's specification part as declarations
   (types and kinds written out, `REAL(KIND=8)`, never relying on `/I4` or IMPLICIT;
   each PARAMETER's value folded, a floating one in as many digits as bring back
   its exact bits, checked when written; derived types with their components;
   interface blocks; generic and operator blocks with MODULE PROCEDURE; NAMELIST;
   COMMON), then CONTAINS and each module procedure's header and dummies' and
   result's declarations with no body, which is legal Fortran: what an interface
   is. What Fortran can't say is in `!GOVAX$` comment lines: the format version,
   the compiler's version, the source and its time, the module's options that
   matter (`/G_FLOATING`, finding 10), the psect and size, each variable's offset,
   each type's layout and alignment, each procedure's link name, each entity's
   access and origin (`!GOVAX$ FROM J JX`). It is read by the compiler's own
   source, lexer, parser, and sema in a mode that makes no code, and its layout
   lines are checked against what sema computes (MODFILEBAD, naming the line, if
   they differ: a file from another version, or edited). Specification
   expressions (`REAL A(SIZE(X,2))`, `CHARACTER(LEN=LEN(S))`) are written back
   from sema's trees by a Fortran printer, with names that might be hidden
   qualified by origin. A reader learns from it what the module offers without
   the source.
8. **Self-contained files** (question 2). A module file carries every entity a
   user of it can reach or needs (finding 7's private ones), including those M
   use-associated from J and re-exports, each with its origin, so a user needs
   only M's file. J's types keep their identity by origin (finding 3): if the
   user also uses J, J's file is read and its `POINT` is the same `Structure`;
   if the two files disagree about J's `POINT` (J recompiled, M not), that is
   MODSTALE naming both.
9. **Recompilation and stale objects.** The file is rewritten only when it
   changes, compared without its header's source-time line (the design note).
   A user's object references a global symbol whose name carries 8 digits of the
   module file's digest (`$MOD$M.1A2B3C4D`), which the module's object defines,
   when the module has variables or procedures; linking a user against an object
   from a different compile of M is then LINK's undefined-symbol warning naming
   that symbol, govax's GSMATCH for modules (question 4). A module of only types,
   constants, and interfaces needs no object, and asks for none.
10. **What a module's options mean to its users.** Types are written with their
    sizes, layouts with their offsets, so `/I4`, `/ALIGN`, `/RECURSIVE`, `/CHECK`,
    and `/F90` of the module never matter to a user. `/G_FLOATING` does:
    `sema.Type` doesn't say D or G (the unit's format does), so a REAL*8 entity of
    a module compiled `/G_FLOATING` used by a `/NOG_FLOATING` unit would be read
    as D. A USE of a module whose format differs is MODFORMAT, when the module
    has a REAL*8 or COMPLEX*16 entity (question 5).
11. **Several modules in one file, in order.** A file's modules and users are
    analyzed in file order, each module's text made as it finishes, so a USE
    finds a module earlier in the file; one later in the file is MODLATER ("MODULE
    M is at line n, after this USE"); a cycle, through files too, is MODCYCLE. One
    compilation still writes one MACRO file (a module's code in it, as a unit's
    is); the goal's "the module's own `.MAR`" is a module compiled by itself.
12. **The search** (question 6). A module not in this compilation is looked for as
    `M.F90$MOD` in the directory this compilation writes module files to
    (`/MODULE=dir`, else the default directory, where the `.MAR` goes: the plan
    left the default directory out, though A.2 found that outputs land there), then
    the source's directory, then each directory of `/MODULE_PATH=(...)`, or with no
    `/MODULE_PATH` each translation of `FORT$MODULES`; on a volume and the host by
    the INCLUDE search's rules, a host name tried in the source type's case and
    then the other. MODNOTFOUND lists every place it looked. No module file is
    written when the compilation has an error, as no MACRO file is (question 7).
13. **Listings and the debugger.** The storage map shows a module's psect and
    variables as COMMON's are, a module procedure's long name's hashed form, and
    in a user each use-associated name's module and remote name ("from CONSTS as
    PI"); the cross reference lists use-associated names with their module and a
    USE row. The `/DEBUG` table lists a module's variables in the module's object
    as a COMMON block's; EXAMINE of `M.X` from a user is Phase 110's.
14. **What waits.** POINTER module variables and components (Phase 107);
    module files for govax's definition modules (`USE $SSDEF`-like; the plan's
    open question 2, later); Fortran 95's PURE and ELEMENTAL module procedures
    (Phase 112).

## A.4 The subtasks, revised

The plan put module files fifth. By finding 6 every USE reads a module's text,
so the writer and reader come with the first USE, and the command's search and
qualifiers stay a subtask of their own.

1. **Survey** (this Part A) (done 2026-11-18; accepted by the author the
   same day).
2. **Modules and their data:** `ast.UseStmt`, the module unit and its
   constraints, the psect and layout, PARAMETERs, types, NAMELIST, COMMON in a
   module; the module text's writer and reader for these, and USE without lists
   in one compilation through it (findings 1, 2, 3, 6, 7, 11); MODLATER,
   MODCYCLE; the storage map's psect (done 2026-11-18).
3. **Module procedures:** analysis as the module's procedures, `M.P` and the
   hashed form, `LinkName`, their interfaces in the text, calls and actual
   arguments from users; MODULE PROCEDURE, generics and operators from modules,
   generic names joined across USEs and checked in the user (findings 2, 4, 5) (done 2026-11-19).
4. **Access and association:** PUBLIC and PRIVATE statements and attributes,
   defaults, private types and components and R522's constraint, re-export with
   origins (finding 8), ONLY and renames, several USEs joined, USE in module,
   internal, and interface-body scopes, USEAMBIG when referenced, USEATTR and
   USECOMMON (findings 2, 3, 8) (done 2026-11-19).
5. **Module files and the command:** writing the file only when it changes, the
   search, `/MODULE`, `/MODULE_PATH`, `FORT$MODULES`, MODNOTFOUND, host and volume,
   the stamp symbol, MODFORMAT, MODSTALE, HELP FORTRAN (findings 9, 10, 12)
   (done 2026-11-19).
6. **Listings:** the storage map's and cross reference's origins, the `/DEBUG`
   table's module variables (finding 13) (done 2026-11-19).
7. **Programs end to end** (`testdata/fortran90/modules/`, each with a
   `compiles.txt` of the compile orders allowed beside `links.txt`; 11.3.3's and
   12.1.2.2.2's examples among them), every mode, gfortran's witness (done
   2026-11-19).
8. **Close-out:** FORTRAN-INTERNALS.md's section 31 (separate compilation, what a
   module file is and why it is Fortran text, use association beside host
   association), MANUAL.md's 14.13, HELP FORTRAN (done 2026-11-19).

## A.5 Questions for the author

1. **The module file's form** (finding 7). (a) **Fortran 90 text, the module's
   specification part and its procedures' interfaces, with `!GOVAX$` lines for
   layout, link names, and origins, read by the compiler's own front end**; (b)
   a record per entity in a form of our own, with a small reader of its own.
2. **Self-contained files** (finding 8). (a) **Each module file carries all its
   users can reach, re-exported entities with their origins**; (b) a module file
   names the modules it uses, and a USE reads those files too.
3. **A module in the same compilation** (finding 6). (a) **Read through its
   text, made in memory, as a separately compiled one is read from its file**;
   (b) used directly from sema's tables, the file only for later compilations.
4. **A stale module object** (finding 9). (a) **A user references a global
   symbol carrying the module file's digest, which the module's object defines,
   so LINK reports a mismatch**; (b) no check.
5. **`/G_FLOATING` across a USE** (finding 10). (a) **MODFORMAT when the formats
   differ and the module has a REAL*8 or COMPLEX*16 entity**; (b) `sema.Type`
   learns D and G, so a unit may use a module of the other format (a larger
   change, touching every REAL*8 rule); (c) no check, as COMMON has none today.
6. **The search order** (finding 12). (a) **This compilation's modules, then the
   `/MODULE` directory or the default directory, then the source's directory, then
   `/MODULE_PATH` or `FORT$MODULES`**; (b) the plan's: the source's directory
   first, the default directory only when `/MODULE` names it.
7. **Module files after an error** (finding 12). (a) **None written when the
   compilation has an error**; (b) each module that compiled without one written
   anyway.
8. The plan's open question 2: **definition modules as module files** (`USE`
   of govax's `$SSDEF` and the rest). Proposed: later, if wanted.

## The author's answers (2026-11-18)

The author read the survey and accepted every recommendation:

1. The module file is Fortran 90 text, the module's specification part and its
   procedures' interfaces, with `!GOVAX$` lines for layout, link names, and
   origins, read by the compiler's own front end (finding 7).
2. Module files are self-contained: each carries all its users can reach,
   re-exported entities with their origins (finding 8).
3. A module in the same compilation is read through its text, made in memory,
   as a separately compiled one is read from its file (finding 6).
4. A user references a global symbol carrying the module file's digest, which
   the module's object defines, so LINK reports a stale module object (finding 9).
5. MODFORMAT when the `/G_FLOATING` formats differ and the module has a REAL*8
   or COMPLEX*16 entity (finding 10).
6. The search: this compilation's modules, then the `/MODULE` directory or the
   default directory, then the source's directory, then `/MODULE_PATH` or
   `FORT$MODULES` (finding 12).
7. No module file is written when the compilation has an error (finding 12).

Definition modules as module files stay for later (the plan's open question 2).

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-18:** Subtask 1, the survey (Part A), for review: what the standard
  asks of modules, use association, and accessibility; what govax has (MODULE,
  USE, PUBLIC, and PRIVATE recognized, the parser's module role, scopes, internal
  procedures, COMMON's psects, the default directory for outputs); fourteen
  findings, among them that every USE reads a module's text so one-file and
  separate compilation share one path, the module file as Fortran 90 text with
  `!GOVAX$` lines, stable hashed names for long globals (today's `.$Nn` isn't
  stable across files), the default directory added to the search, and
  `/G_FLOATING` mismatches; the subtasks revised (the text's writer and reader
  with the first USE); seven questions. The author accepted every
  recommendation the same day ("The author's answers").
- **2026-11-18:** Subtask 2, modules and their data. The parser reads USE
  (`ast.UseStmt`: renames, ONLY lists, generic specifications) and PUBLIC and
  PRIVATE (`ast.AccessStmt`; a type's PRIVATE is one), `parse/modules.go`, with
  the golden `parse/testdata/modules.f90`. Semantic analysis compiles a MODULE
  unit (`sema/module.go`): its variables, initialized or not, are a common block
  of its own, `$MOD$M` (`CommonBlock.Module`; `moduleBlock`, before
  `layoutCommon`, by the module's `/ALIGN`), laid out and initialized as
  COMMON's; a COMMON block declared in the module stays its own section; a long
  module name's section is `LongName`'s hashed form (finding 5). The IR makes
  a module a routine of no code, `ir.Module` (`Kind.NoCode`, as BLOCK DATA),
  whose section codegen writes with its initial values. When a module has been
  analyzed without an error, `Program.finishModule` writes its text
  (`sema/modwrite.go`: types first, each after the structures its fields name,
  then constants, variables, COMMON statements, NAMELIST), kept in the
  compilation's table (`modules`) and as `Program.Modules` /
  `fortran.Result.Modules`, with its digest (SHA-256 without the SOURCE line).
  Every USE reads a module's text (`sema/modread.go`): the `!GOVAX$` lines into
  `moduleFacts`, then the compiler's own source pass, lexer, parser, and
  semantic analysis of the one unit with `analyzer.reading` set; a text that
  doesn't read is MODFILEBAD with its line. USE without a list
  (`sema/use.go`) enters each entity in the using unit's own scope as a copy
  marked `Symbol.Use` (module, remote name, entity), a variable through the
  unit's view of the module's section (`Unit.UsedBlocks`, which no statement of
  the unit lays out, and codegen writes as a contribution), types as the
  module's own `Structure` (`Unit.usedStructs`), NAMELIST groups with the
  unit's copies; USE comes first (`order.go`'s `useGroup`). MODLATER, MODCYCLE,
  MODNOTFOUND (no finder yet: subtask 5), USELOCAL, USEATTR (a type statement,
  DIMENSION, DATA, a STRUCTURE or TYPE of the name), and USECOMMON. A module in
  the same file that failed has had its messages; its users say nothing more,
  their declarations incomplete. Choices made here: a variable's offset and a
  section's size are the text's AT, PSECT, and COMMON lines, not recomputed
  (the reader checks each size against its declaration, `applyLayoutFacts`,
  and each type's size and boundary against its TYPE line, `readTypeLayout`;
  MODFACT inside MODFILEBAD); a REAL literal is written in its exact digits, a
  folded REAL value in the fewest digits that round back in D and G, with a
  FOLDED line so the reader doesn't take it for a literal (section 2.3.1.2's
  extra precision); MODSTMT is only for executable statements, statement
  functions, and FORMAT, since ENTRYUNIT, ATTRDUMMY, ADJNOTDUMMY, and
  AUTOCOMMON already say the rest; the plan's MODPSECT is dropped, no FORTRAN
  name starting with $. Until their subtasks, module procedures, USE's lists,
  and PUBLIC and PRIVATE in a module are NOTIMPL; a structure with initial
  values, an unnamed nested structure, and a constant whose bits are no number
  can't be written into a text yet (NOTIMPL). Tests:
  `internal/fortran/modules_test.go` (the text, a user compiled with the module
  and apart making the same code, the messages, the hashed name, `/G_FLOATING`,
  `/RECURSIVE`), and `testdata/fortran90/moddata.f90`, in every mode, the same
  on gfortran.
- **2026-11-19:** Subtask 3, module procedures. A module's FUNCTIONs and
  SUBROUTINEs after CONTAINS are analyzed as its contained units, by the
  internal procedures' steps (`analyzeContained`, the four steps of
  `analyzeUnit` after the declarations, recursive, so a module procedure may
  have internal procedures of its own); each has a global name,
  `Unit.LinkName`/`Symbol.LinkName`, M.P or `LongName`'s hashed form (finding 5).
  The IR lowers a module's procedures with it (`lowerHost(u, host)`, now
  recursive), each named by its link name, entered at a global `.ENTRY`
  (`ir.Routine.Global`), its internal procedures at local labels named from it
  (M.P.Q); a call by link name has no hidden argument, a module having no frame
  (`callee`; `internalCallee` and `hostFrameArg` now tell a routine's own
  internal procedure from its host's); `ir.Check` lets a routine use every
  host's cells, not only its host's. A module procedure may be an actual
  argument (`nameArg`, `PUSHAB G^M.P`). MODULE PROCEDURE is compiled
  (`compiledBlock` is always true): in a module its blocks wait until the
  module's procedures are known (`analyzer.waiting`, `moduleProcedureBlocks`),
  elsewhere they name use-associated module procedures at once
  (`moduleProcedures`; MODPROCNAME, MODPROCGENERIC); generic and operator code
  reaches a specific's interface by `explicitInterface`, and reports a
  use-associated specific at its USE (`specificPos`). The text gains interface
  blocks (an external procedure's body; each generic name and operator, its
  module procedures in a MODULE PROCEDURE statement and its external specifics'
  bodies) and after CONTAINS each module procedure's interface (header with
  RECURSIVE, RESULT, and `*`s; its own constants and types; each dummy's
  declaration with INTENT and OPTIONAL, a dummy procedure's interface or
  EXTERNAL; the value's declaration), each after its LINK line, which the
  reader takes for the procedure's link name. USE gives module procedures and
  external procedures with interfaces (`useProcedure`: the interface, the link
  name, a renamed procedure's original name), generic names (`useGeneric`) and
  operators (`useOperators`) with the unit's copies of their specifics, joined
  with what the unit has of the same name from another USE (11.3.2), so
  `checkGenerics` holds the joined list to 14.1.2.3 in the user (GENERICAMBIG at
  the USE). A reference checked against a use-associated interface names the
  USE's line. Choices made here: an external procedure that is a specific of
  two generic names or operators of a module can't be written into its text yet
  (NOTIMPL), nor can ENTRY in a module procedure (whose entry points would be
  module procedures too). Tests: `internal/fortran/modules_test.go`'s
  `TestModuleProcedureText`, `TestModuleProceduresSeparately`,
  `TestModuleProcedureMessages`, `TestModuleLongNames`; and
  `testdata/fortran90/modprocs.f90`, in every mode, the same on gfortran.
- **2026-11-19:** Subtask 4, access and association. PUBLIC and PRIVATE
  (`sema/access.go`): `moduleAccess` on a module's Unit holds the default
  (ACCESSDEFAULT for a second) and what statements and attributes gave by name
  or operator text (ACCESSTWICE); the statements are read in two steps,
  `accessStatements` with the declarations (a name nothing declares, and no
  procedure or generic interface of the module, becomes an implicitly typed
  variable then, so the module's section holds it) and `resolveAccess` in
  `analyzeContained` once the module procedures and generic interfaces are
  known (ACCESSNAME), marking each Symbol and namelist group `private`, and
  each type's local name and each operator in `privateTypes` and `privateOps`:
  access belongs to a name in a module, not the entity, so a module may make a
  used entity private. `TYPE, PRIVATE ::` and `TYPE, PUBLIC ::`, the `PUBLIC`
  and `PRIVATE` attributes, and a type's PRIVATE statement
  (`Structure.PrivateComponents`; `Structure.Module` the defining module) are
  compiled. The checks (`accessChecks`, reported in line order): ACCESSTYPE,
  ACCESSPROC, ACCESSNML, ACCESSCOMPONENT; a type the module has no name for
  (an ONLY list left it out) has no PRIVATE attribute there. COMPPRIVATE for a
  component (`record.go`, `datatypes.go`) or constructor (`constructorArgs`) of
  such a type outside its module and its procedures (`insideModule`). USE
  (`sema/use.go`): a unit's USEs of one module are joined (`useRequest`, 11.3.2),
  each public entity entered by its local names (`useModule`), ONLYPUBLIC for an
  item that names none; every copy carries its entity's origin
  (`UseOrigin.origin`, an `entityOrigin`: the defining module and its name
  there), so one entity by two paths is one name; two entities of one name are
  ambiguous (`Symbol.ambiguous`, `Unit.ambiguousTypes`, `Namelist.ambiguous`),
  USEAMBIG only where referenced (`lookup`, `derivedStructure`,
  `constructorArgs`, `namelistRef`). A generic interface's specific or a namelist
  group's item a USE gives no name is a hidden copy (`Unit.hidden`). USE in a
  module procedure, an internal procedure, and an interface body needed nothing
  new. The text (`modwrite.go`) now gives everything a user can reach and what
  the module has from other modules (finding 8): the moduleWriter decides the
  names first (`prepare`: the module's names, hidden entities and types the
  module has no name for under their own, private, NOTIMPL if the name is taken),
  writes each used type, constant, variable (a PSECT or COMMON line for its
  module's section or block, `usedBlock`), namelist group, external procedure
  (with a LINK line when the module knows it by another name), generic interface,
  and module procedure (after CONTAINS) with a `!GOVAX$ FROM name module remote`
  line, and the module's private names in a PRIVATE statement. The reader
  (`modread.go`) keeps FROM lines (`moduleFacts.from`), groups FROM variables in
  their modules' sections (`moduleBlock`), applies LINK lines to interface bodies,
  and makes each type of a module text the one Structure of its origin
  (`readType`; the compilation's `modules.types`, those of a text being read
  pending until it reads): a type two texts give differently is MODSTALE (moved
  here from subtask 5, being where the types meet), which explains whatever else
  the read found. Choices made here: the access checks aren't made on a module's
  text (they were made when it compiled, and a hidden type is private there);
  a type known by two local names in a module, a hidden entity whose name the
  module has for something else, and a COMMON block both a module and a module it
  uses declare can't be written into the text yet (NOTIMPL). Tests:
  `internal/fortran/modaccess_test.go` (`TestModuleAccessText`,
  `TestModuleReexport`, compiled with and apart from its modules, and with the
  second module's file alone; `TestModuleStale`; `TestModuleAccessMessages`),
  and `testdata/fortran90/modaccess.f90`, in every mode, the same on gfortran.
- **2026-11-19:** Subtask 5, module files and the command. The FORTRAN command
  writes each module a compilation without an error compiled as `NAME.F90$MOD`
  (`dclcmd/fortranmodules.go`'s `writeModuleFiles`) in `/MODULE=directory`'s
  directory or the default directory, where the MACRO file goes, unless a file
  there has the same digest (`sema.ModuleDigest`, which now leaves out the
  COMPILER line as well as SOURCE, so a new build of govax doesn't make every
  module new); with `/NOMACRO` too, as gfortran's `-fsyntax-only` does. A USE of
  a module the compilation doesn't compile goes to `fortranModules`, the
  `sema.ModuleFinder`: the output directory, the first source's directory, then
  `/MODULE_PATH=(dir,...)`'s directories or, without it, each translation of
  `FORT$MODULES` (the author's answer 6), each once; a directory on the host or a
  volume by `rms.Session.Side` (a logical name its translation), a volume one
  reached by `LocateRelated`; a host name tried in the case of the source's type,
  then the other; MODNOTFOUND names every file looked for, and one there but
  unreadable with its reason. `/MODULE` and `/MODULE_PATH` in the grammar
  (`console.dcl`, ids 1394 and 1395) and `govax fortran`'s `--module` and
  `--module-path`; a `/MODULE` that isn't a directory, or a host one that doesn't
  exist, is CLI_FORMODULEDIR. The stamp (finding 9): a module's text has an
  `!GOVAX$ OBJECT` line when its object holds its own section or a module
  procedure; the module's object defines its stamp, `sema.StampName`
  (`$MOD$M.` and the digest's first 8 digits, `LongName`'s form past 31
  characters; `Unit.Stamp`, a global assignment by codegen's `writeStamps`), and a
  unit that USEs a module with an OBJECT line refers to it (`Unit.Stamps`, an
  `.EXTRN` but where this MACRO file defines it), so a stale user object linked
  with a new module object is LINK's NUDFSYMS naming the stamp. MODFORMAT (finding
  10): a USE of a module of the other `/G_FLOATING` format with a REAL*8 or
  COMPLEX*16 entity anywhere (a name, a component, a procedure's dummy or value;
  `use.go`'s `formatMatters`). MODSTALE was subtask 4's. HELP FORTRAN's
  Fortran_90 topic and `/MODULE` and `/MODULE_PATH` lines; MANUAL.md's syntax
  block, qualifier paragraph, and a "Modules" paragraph in 14.13 (the whole story
  is the close-out's). Choices made here: a user refers only to the stamps of the
  modules it USEs; a module re-exporting another's entities refers to that
  module's stamp in its own object, so the check goes from object to object (a
  user of a re-exporting module with no object of its own isn't checked against
  the other's); a module always defines its stamp, whether or not its users ask
  for it. Tests: `internal/fortran/modules_test.go`'s `TestModuleFormat` and
  `TestModuleStamps`; `internal/dclcmd/fortranmodules_test.go` (written and left
  alone on a volume, none after an error, `/NOMACRO`, the search on a volume and
  the host, MODNOTFOUND, FORMODULEDIR, and `TestModuleStampLink`, MACRO and LINK
  on a stale user object); `internal/console`'s `TestDispatch_fortranModules`
  through the grammar; `cmd/govax`'s `TestFortranCommand`.
- **2026-11-19:** Subtask 6, listings. A use-associated name says where it came
  from (`Symbol.UseText`: "from CONSTS", "from CONSTS as PI" for a rename, PI being
  the module's, and ", defined in GEOM" for an entity the module has from another),
  at the end of its storage map row (`codegen`'s `storageMap`, a fifth column) and
  after what its cross reference row says it is ("INTEGER from CONSTS"); each module
  a USE names has a cross reference row, `module`, the USE statements' lines
  (`whatModule`, noted in `uses`). A name a USE of a whole module gave with the rest,
  and the unit never mentions, has neither row (`Symbol.Unmentioned`), by the rule a
  text library's names have, so a large module doesn't give every user rows for all
  of it; one an ONLY list or a rename names is the unit's choice
  (`Symbol.listed`, set by `useModule` from `localName.listed`) and has them. A
  module procedure's map and cross reference are headed by its path, M.P (and
  M.P.INNER for its internal procedures): `Unit.Path` is now the host's path and the
  unit's name, never the link name, and `StorageMap.Routine` is the path (it was
  `ir.Routine.Name`, so an internal procedure of a module procedure was headed one
  way in its map and another in its cross reference); a map whose routine's global
  name isn't its path says what that is ("VERY_LONG_MODULE_NAME_$EC613099 is
  VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO, its global name"), as a
  user's map does for each use-associated module procedure with a hashed name, the
  name its calls give (`Symbol.HashedLinkName`). The `/DEBUG` table already listed
  a module's variables at their symbols in its section, as a COMMON block's, and a
  user's under the user's names; a module now has no entry point line, having no
  code. A fix found here: `/WARNINGS=UNUSED` reported a module's public constants,
  which are its users'; now a module's public names are never reported, its private
  ones are, a name a USE of a whole module gave isn't, and one an ONLY list or a
  rename named is (`neverUsed`; a use-associated variable's module section no
  longer hides it, as a COMMON block's would). Choices made here: the `/DEBUG` table
  keeps a user's unmentioned use-associated variables, so the debugger can find
  them from any unit that has them, until Phase 110's `M.X`; a use-associated
  name's row keeps the USE's line as its definition (`D`). MANUAL.md's 14.13 has
  the listing's view, an example run on govax. Tests:
  `internal/fortran/modlisting_test.go` (`TestModuleListings`, with the modules and
  apart from them through their files; `TestModuleUnusedWarnings`;
  `TestModuleDebugTable`).
- **2026-11-19:** Subtask 7, programs end to end. `TestFortran90Modules`
  (`internal/console/fortranprograms_test.go`) reads a program's `compiles.txt`:
  each line an order its files may be compiled in, a FORTRAN command per word
  (`A B,C D` three commands), every order followed by every line of `links.txt`,
  the module files deleted before each order so none stands in for one the order
  hasn't written; and `name.in` for a program that reads the terminal.
  `testdata/fortran90/modules/standard` is the standard's examples (11.3.3.1 to
  11.3.3.6, 11.3.3.8, and 12.1.2.2.2) made to run over seven files, the
  standard's text kept where it is code; `modules/polys` a polynomial library of
  three modules in a chain (KINDS, POLYNOMIALS, ROOTS re-exporting it), a main
  program using only the last, a user with an ONLY list and a rename of a module
  variable, and a fixed-form user compiled `/NOF90`. Each runs in three compile
  orders and two links, in every mode and both paging modes; their output was
  worked by hand and is the same on gfortran (`compare.py`, which now gives
  gfortran a program's other files before its main program's). They found five
  defects, each fixed here and held by `internal/fortran/modprograms_test.go`:
  a SAVE statement naming a module's variable was SAVECOMMON, its section being
  a common block (`attributes.go`'s `saves`: a module's section is static, so
  SAVE there changes nothing; a COMMON block's member is still refused); a
  module's allocatable array had no descriptor header in its section, so
  ALLOCATE saw no dimensions (codegen's `writeCommons` now writes each one's
  unallocated descriptor from the module's own routine, as `$LOCAL` does a unit's);
  a module's text whose interface bodies name the module's types, as the writer
  makes them, couldn't be read, a body having no host (`analyzer.textTypes`, the
  module whose text is read, and `typeNamed`, every lookup of a type by name); an
  explicit-shape dummy's bound from an inquiry of an assumed-shape dummy, the
  standard's `DIMENSION (SIZE (X, 2))`, crashed lowering, its entry code having
  no statement (`lowerer.here`, the header's position then); and the floating
  constants the code generator adds were `$Fn`, FORMAT n's block's name, so a
  unit with both didn't assemble (now `$KFn`; the old corpus's MACRO files
  change by that name alone, so `testdata/nof90.sum` was rewritten, the only
  change to old programs' output). And one in the lexer, not modules': after a
  name or `)`, the period closing `.AT.` was read as the start of a number in
  `R .AT. 2.0` (`tokenize.go`'s `closesWord`). Choices made here: a fixed-form
  `/NOF90` unit using a module has no defined operators, VAX FORTRAN reading
  `A.B.C` as a field, so `polys`'s calls the module's functions; a compile order
  that USEs a module before it is compiled isn't run, MODNOTFOUND being
  `internal/dclcmd`'s tests'.
- **2026-11-19:** Subtask 8, the close-out. FORTRAN-INTERNALS.md's section 31,
  "Fortran 90: modules and separate compilation", follows SHAPES and a user
  compiled apart through it: a module as a unit with no code (its variables a
  common block's psect, its procedures global `M.P` names, `LongName`), the module
  file whole and why it is Fortran text, every USE reading text and what each kind
  of entity becomes in the user (a private specific reached through its public
  generic, a hidden copy), access, ONLY and renames, one entity by two paths and
  USEAMBIG, modules using modules and MODSTALE, the search, the file rewritten only
  when it changes, the stamp and LINK's message for a stale object, MODFORMAT, and
  use association beside host association; its MACRO and messages are govax's own
  output. MANUAL.md's 14.13 has the whole story of modules: what a module may hold,
  SHAPES and TOUR with their output, module data and names, USE with renames and
  ONLY, one name from two modules, PUBLIC and PRIVATE, generic names from modules,
  then the module files and listings subtasks 5 and 6 wrote, and what isn't there
  yet; the introduction names derived types and modules. HELP FORTRAN's Fortran_90
  topic has a module and its user (run on govax) and USE's, PUBLIC's, and
  PRIVATE's rules; the help library rebuilt (`go generate ./internal/bootdata`).
  PLAN.md and CLAUDE.md summarize the phase.

## Where things stand

Phase 106 is done. What it left, and where each goes:

- **Phase 107:** POINTER module variables and components.
- **Phase 110:** the debugger's EXAMINE of a module's variable as `M.X` from a
  user (the `/DEBUG` table lists a user's use-associated variables under its own
  names until then); `/STANDARD` for the phase's forms.
- **Phase 112:** PURE and ELEMENTAL module procedures (Fortran 95).
- **Later, if wanted (the plan's open question 2):** govax's definition modules
  (`$SSDEF` and the rest) as module files to USE, beside INCLUDE's.
- **Not planned, ours (NOTIMPL when a module compiles):** ENTRY in a module
  procedure; an external procedure that is a specific of two of a module's
  generic names or operators; a type known by two local names in a module; a
  hidden entity whose name the module has for something else; a COMMON block both
  a module and a module it uses declare; a structure with initial values, an
  unnamed nested structure, and a constant whose bits are no number, in a module's
  text.
- **Unconfirmed choices of ours:** the module file's form (Fortran 90 text with
  `!GOVAX$` lines; the author's answer 1); the stamp, checked from object to
  object, a user referring only to the modules it USEs; a module always defining
  its stamp; the search order (the author's answer 6); a fixed-form `/NOF90` user
  having no defined operators from a module; the `/DEBUG` table keeping a user's
  unmentioned use-associated variables; module variables static whatever SAVE
  says.
- **Awaiting a VMS round:** none: no VAX Fortran 90 exists, and the corpus's
  modules are held to gfortran (`compare.py`); Phase 111's optional VMS round may
  run their MACRO on VMS.
