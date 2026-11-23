# Phase 112's survey probes: Fortran 95

Small programs, one per group of Fortran 95's additions to Fortran 90, written for
Phase 112's survey (`docs/planning/PHASE-112 - Fortran 95.md`, Part A). Each is
standard Fortran 95, built with `gfortran -std=f95` as the witness; `name.gfortran` is
what gfortran's build printed (with `name.in` on standard input where there is one).

| Program | What it holds |
|---|---|
| `forall.f90` | FORALL statement and construct, masks, nesting, WHERE inside, a pointer assignment, a named construct |
| `pure.f90` | PURE and ELEMENTAL functions and subroutines: module, internal, external with an interface block |
| `elemasg.f90` | an elemental defined assignment, in an array assignment and in WHERE |
| `nullinit.f90` | `NULL()`, `=> NULL()`, default initialization (static, allocated, INTENT(OUT), a local each call) |
| `where95.f90` | WHERE in WHERE, masked ELSEWHERE, a named WHERE |
| `intrin95.f90` | MAXLOC/MINLOC `DIM=`, CEILING/FLOOR `KIND=`, `NULL(MOLD)`, CPU_TIME |
| `zerowid.f90` | `I0`, `B0`, `O0`, `Z0`, `F0.d`, `I0.m` |
| `zerosize.f90` | zero-sized explicit-shape arrays |
| `misc95.f90` | END INTERFACE with its generic name, comments in namelist input (`misc95.in`), a PURE function in a specification expression |

They are survey material, not a test corpus: govax compiled none of them but
`zerowid.f90` on 2026-11-22 (and that one stopped at OUTCONERR); the survey's table
lists what each gave. `zerowid.gfortran`'s `-.500` and the CPU_TIME line are
processor-dependent.

Since subtask 14 (2026-11-23) govax compiles every one, in every optimizer and
`/RECURSIVE` mode, and each prints its `name.gfortran` but for the layouts of
list-directed REAL output in `misc95` and `zerosize` (VMS's widths). Each became, or
went into, programs of `testdata/fortran90` with outputs worked by hand:

| Probe | In `testdata/fortran90` |
|---|---|
| `forall.f90` | `foralls.f90`, `forallc.f90` |
| `pure.f90` | `pures.f90`, `elementals.f90` |
| `elemasg.f90` | `elementals.f90` |
| `nullinit.f90` | `nulls.f90`, `defaults.f90` |
| `where95.f90` | `wheres.f90` (its first case) |
| `intrin95.f90` | `intrins.f90`, and `nulls.f90` for `NULL(MOLD)` |
| `zerowid.f90` | `zerowid.f90`, `zerowidvms.f90` |
| `zerosize.f90` | `zerosize.f90` |
| `misc95.f90` | `modules/colony` (END INTERFACE with its generic spec, namelist comments, a PURE function in a declaration's length), and `pures.f90`, `zerowid.f90` |
