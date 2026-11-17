# Phase 106 — Fortran 90: modules

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 105.

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
   11; the HP manual's module chapter for the file conventions), for review.
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

## Progress log

- **2026-11-17:** Planned with Phase 99.
