# Phase 115 — Fortran 90's remaining NOTIMPLs

**Status:** planned 2026-11-23, at the review of Phase 113's refreshed survey
(`PHASE-113 - Fortran errata.md`, A.15 and A.16): the author wants the compiler's NOTIMPL list
cleared where possible, and these items were split off so Phase 113 stays the size of the
phases before it. A skeletal doc: subtask 1 is the survey, reviewed before any implementation.
Needs Phase 113 (its Fortran 90 items, subtasks 2 and 3, touch the same code).

## Goal

Every NOTIMPL the compiler still reports, after Phases 112 and 113, either compiled or
refused for a stated reason: a message that names what the standard forbids, or a limit
documented in MANUAL.md. Phase 113's survey placed these items **not planned** ("ours"); the
author has since chosen to clear them. No VMS round can check them (there is no VAX FORTRAN or
Fortran 90 compiler on VMS), so their programs join the next round's waiting list
(`f90rAwaitingVMS`), as the corpus's always have.

## The seed list

From Phase 113's A.16, each site read 2026-11-23; the weights are that reading's estimates
(small about half a subtask, medium about one).

1. **A section, or a whole assumed-shape array, to a system subroutine or a built-in**
   (`sema/arguments.go`, `arg` and `arrayArg`): `%LOC`, `%REF`, `%DESCR`, and the system
   subroutines. Phase 104's copy to a temporary and back (`sequence.go`, `valueTemp`) mostly
   fits; what `%LOC` of a strided array gives is our rule to choose. Small to medium.
2. **CHARACTER lengths known only when the program runs**: a CHARACTER array expression not made
   of names, as an actual argument, an assumed-shape dummy's actual, or a constructor's first
   value (`shaped.go`, `sequence.go`, `constructor.go`; `lengthFromNames`); a function value of
   such a length where it is still refused (a RESULT-named value, `charlength.go`'s
   `runtimeLength`; a caller's declaration of an external function, `checkLengths`; a selector's
   length in a FUNCTION statement, `decl.go`'s `typeOf`). A way to compute an expression's length
   without its elements is the design question. Medium, one or two subtasks.
3. **A name an internal procedure's declarations use that its host uses only in executable
   statements** (`internal.go`'s `checkProvisional`): another step in Phase 103's host analysis,
   so the host's names are known before its internal procedures' declarations are read. Medium;
   the analysis order is delicate.
4. **A CHARACTER pointer of run-time length** other than a `LEN=*` dummy (`pointer.go`): the
   pointer's class S descriptor already holds a length. Small to medium.
5. **The module file's gaps** (`modwrite.go`'s `fail`, `internal.go`): ENTRY in a module
   procedure; an external procedure that is a specific of two generic names or operators; a type
   known by two names; an entity reached through a generic or a namelist whose name the module has
   for something else, and a type in the same case; an unnamed structure; a constant of a
   structure, and one whose bits are no number. Each extends the module text and its reader
   (`modread.go`). Medium to large, about two subtasks.
6. **DATA naming an array section** (`data.go`, `datatypes.go`; the standard's R537): DATA's walk
   following the section's triplets. Small. (Not in Phase 113's survey; found at its refresh.)
7. **A NAMELIST object past 8 levels or 15 dimensions** (`derivedio.go`): FTNRTL's limits
   (`ftn_namelist.mar`), so raising them is MACRO work. Small, of little value: the survey's
   first question.

Not here: the dead hooks found beside these (the attribute, kind, file-specifier, and I/O keyword
tables' NOTIMPL paths with no entry left) are removed in Phase 113's housekeeping (its A.16).

## Open questions (for subtask 1)

1. Item 7: raise FTNRTL's NAMELIST limits, or keep them and make the message a limit's, not
   NOTIMPL?
2. Item 1: what `%LOC` of an assumed-shape array or a section gives (its first element's
   address, a temporary's, or a refusal saying why).
3. Item 2: whether the length of every CHARACTER expression can be computed apart from its
   values, or which forms stay refused with a message of their own.
4. Item 5: the module text's form for each gap, and whether files written before this phase
   still read (MODFORMAT).
5. Whether anything left after this phase is NOTIMPL at all, or each remaining site becomes an
   error with its reason (the Goal's second place), so the compiler's NOTIMPL list is empty but for
   statements of later standards.

## Subtasks (provisional)

1. **The survey**: every NOTIMPL site after Phase 113, checked and weighed again, with a small
   program each; the questions answered.
2. Onward: as its review decides.

## Progress log

- **2026-11-23:** Planned at the review of Phase 113's refreshed survey: the remaining NOTIMPLs
  Phase 113 had left not planned, split into a phase of their own by the author's choice, so
  the phases stay alike in size.
