# Phase 60 — LIB$ balanced trees and table-driven parsing

**Status:** planned (2026-10-12). Needs Phases 53, 54, and 58
(callbacks). **TPARSE and TABLE_PARSE moved to Phase 80** (2026-10-31, the
author's direction: VMS's FORRTL needs LIB$TPARSE for namelist input; Phase
80's subtask 10); this phase keeps the trees. Part of the LIB$ program laid out in
`PHASE-53 - RTL LIB support.md` (Part A).

## Goal

Two subjects that are mostly calls back into the program:

- **Balanced binary trees** (the manual's section 2.7.5): INSERT_TREE
  adds a node to a tree the program keeps in its own memory, calling
  its compare routine to order the keys and its allocate routine to make
  the node. LOOKUP_TREE finds one. TRAVERSE_TREE calls an action routine
  for every node in order. The node layout (left, right, balance) is the
  manual's, since the program reads it.
- **TPARSE** (LIB-411 to LIB-452, the manual's longest entry): a
  table-driven finite-state parser. The program builds state tables with
  macros (`$INIT_STATE`, `$STATE`, `$TRAN`, `$END_STATE`, and the
  argument block `$TPADEF`), and LIB$TPARSE runs a string through them.
  It matches keywords, numbers, symbols, and characters, calls action
  routines, and branches and nests subexpressions. TABLE_PARSE is 7.x's
  version of the same interface (a probe or a later manual must settle
  how it differs).

  TPARSE's state tables are a binary format the macros build. govax's
  clean-room `starlet.mar` (Phase 32) will need those macros written from
  the manual's description of the table format and checked against real
  MACRO's objects (as `TestServiceMacroObjects` does for the system
  service macros), and LIB$TPARSE must read what they build.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Trees | INSERT_TREE (237), LOOKUP_TREE (265), TRAVERSE_TREE (459) |
| Parsing | TPARSE (411), TABLE_PARSE (7.x) |
| Macros | `$INIT_STATE`, `$STATE`, `$TRAN`, `$END_STATE`, `$TPADEF` in govax's `starlet.mar` |

## What earlier phases leave in place

- Phase 58's callbacks from a shim.
- The macro facility and govax's STARLET (`internal/bootdata/files/starlet.mar`,
  `starletdef.mar`, `go generate ./internal/bootdata`), and the probes
  that check macros against real MACRO (`testdata/mp/macros`).

## Open questions

- **The TPARSE table format**: what the manual documents (the macros'
  arguments and meaning) versus what only a real assembly shows (the
  binary layout). A VMS probe assembling a table with every kind of
  transition, with its object kept for an oracle, settles the layout
  without reading VMS's macro source.
- **Is TPARSE big enough to be a phase of its own?** If the survey
  shows it is, the trees stay here and TPARSE becomes Phase 60b (or
  moves after Phase 61).

## Subtasks (provisional)

1. **Survey**: the tree routines; TPARSE's tables, transitions, flags,
   argument block, and action routines.
2. **INSERT_TREE, LOOKUP_TREE, TRAVERSE_TREE.**
3. **TPARSE's macros** in `starlet.mar`, and the probe that checks them.
4. **LIB$TPARSE**: the interpreter.
5. **TABLE_PARSE** (if settled).
6. **The program** (`testdata/lib60/`, a TPARSE grammar and a tree), its
   console test, and the VMS probe.
7. **Close-out.**

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-31** — TPARSE and TABLE_PARSE built in Phase 80's subtask 10
  (`internal/librtl/tparse.go`, `testdata/tpa80`); only the trees remain here.
