# Phase 84 — the FORTRAN optimizer

**Status:** in progress: subtask 1, the survey (2026-11-01), awaits the
author's review. Planned 2026-10-29. Needs Phase 83 (the whole language
compiled, so the corpus is complete). Part of the FORTRAN program laid
out in `PHASE-74 - FORTRAN plan.md` (Decision 9).

## Goal

`/OPTIMIZE`: passes over the IR and the generated instructions that make
programs smaller and faster without changing what they print. Each pass
is its own file in `internal/fortran/opt`, with a doc comment that
teaches the technique on a small FORTRAN example. Proposed passes, in
order of payoff:

- **constant propagation and folding** across statements, and **dead
  code removal** (unreachable blocks, unused temporaries);
- **common subexpressions** within a basic block (array addresses above
  all);
- **loop-invariant code motion** out of DO loops;
- **induction variables and strength reduction:** an array subscript
  that grows by a constant each trip becomes an address that does;
- **register allocation:** R6–R11 for DO variables, trip counts, and
  addresses used in a loop, with liveness analysis; dummy argument
  addresses loaded once;
- **VAX loop instructions:** a counted loop closes with `SOBGTR`,
  `AOBLEQ`, or `ACBL` where its form allows;
- **a peephole pass** over the instructions: `MOVL #0` to `CLRL`, `ADDL2
  #1` to `INCL`, branches to branches, a move followed by a test.

## What earlier phases leave in place

- The IR (Phase 75), the whole language in it (Phases 76–83), and the
  corpus of programs with their expected output.

## Design notes

- **Correctness first:** every corpus program runs with `/OPTIMIZE` and
  `/NOOPTIMIZE` and must print the same; a test reports instruction
  counts (from govax's `--stats`) so improvements are measured.
- **VOLATILE** variables and variables whose address escapes (%LOC,
  EQUIVALENCE, COMMON across calls, arguments) are never kept in a
  register across a call or a reference through a pointer: the alias
  rules are FORTRAN's (a dummy argument may not be modified through
  another name), which the doc explains.
- **The default:** `/NOOPTIMIZE` until the whole corpus passes both
  ways, then `/OPTIMIZE`, as VAX FORTRAN's default (agreed in Phase 74's
  review, answer 7).

## Open questions

- **How far to go:** global CSE and partial redundancy elimination are
  more than the corpus may justify; the survey measures where the time
  goes in the corpus's larger programs first.

# Part A — the survey (2026-11-01)

Awaiting the author's review. The decisions below are proposals until then.

## Summary

- **The corpus can't measure an optimizer.** Its programs spend about 95% of
  their instructions in FTNRTL, which is hand-written MACRO the optimizer never
  touches. It remains the correctness test. The gains have to be measured on
  DUNGEON and on five new kernels (`testdata/opt84`).
- **The largest gains are in two places the plan ranked low.** Those are
  branches (a fifth of DUNGEON's instructions in its own code are `BRW`) and
  registers (most of the memory references in a loop). Constant propagation,
  ranked first in the plan, has little left to do.
- **Recommended order:** the branch peephole moves to the front and registers
  come before induction variables. Global CSE and PRE are not needed.
- **Measured reachable gain:** 25% fewer instructions in the whole DUNGEON run
  with the peephole and one routine done by hand, and 47% to 76% in the
  kernels. All were hand-applied, not yet the work of a real pass.

## How it was measured

A temporary hook in `Engine.Step` counted every instruction by PC, in a
throwaway worktree (`testdata/opt84/measure`; not in the tree). Each count is
bucketed by the image the PC is in. "Own code" means the program's main image.
Data references are the memory statistics of `--stats`, instruction fetches
excepted (they go through the fetch window). The runs used the copying loader,
paging off. Each hand-edited or peepholed program was assembled and linked by
govax's MACRO and LINK and run, and its output compared with the original's.

Instruction counts aren't time. A VAX's time depends heavily on memory
references, which registers remove, so the tables give both.

## Where the instructions go

**The corpus** (76 programs in `testdata/fortran`, every one still printing
its `.out`):

| | Instructions | Share |
|---|---:|---:|
| All programs | 7,881,993 | |
| Own code | 212,317 | 2.7% |
| FTNRTL | 7,475,527 | 94.8% |
| `astrace` alone | 6,311,616 | 80.1% of the total |
| All but `astrace`: own code | 82,287 of 1,570,377 | 5.2% |

The largest own-code counts are `astrace` (130,030), `unformatted` (18,673),
and `adjust` (12,354). The corpus's programs
are tests of the language: small, short, and mostly I/O.

**DUNGEON.** It already compiles (`/CONTINUATIONS=40`), assembles, links, and
runs, with an ITIME of our own and DTEXT.DAT made as a fixed-record file by a
small FORTRAN program. A scripted game of 31 lines ran to its end. Its output was only compared between builds, not checked against a
reference: that is Phase 86's job.

| Run | Total | Own code | FTNRTL |
|---|---:|---:|---:|
| Start and quit | 2,172,953 | 143,493 (6.6%) | 2,012,960 (92.6%) |
| The scripted game | 6,818,609 | 4,504,906 (66.1%) | 2,267,472 (33.3%) |

Within the game's own code:

- **One routine dominates:** `QHERE`, a small DO loop searching two arrays, is
  62.0% of it. `SCHLST`, `LIT`, `THISIT`, and `SPARSE` follow at 6.0%, 5.7%,
  5.0%, and 4.2%.
- **By opcode:** `BRW` 20.0%, `MOVL` 16.6%, `INCL` 9.8%, `BLEQ` 9.5%, `DECL`
  9.5%, `CMPL` 9.3%, `BEQL` 8.9%. `CALLS` is 1.2%, so calls aren't where the
  time goes.
- **Statically,** 3,081 of DUNGEON's 20,976 instructions are `BRW`, and 2,875
  of those (93%) are the second half of a byte branch around a `BRW`.

Startup is mostly FTNRTL, in character loops in its own MACRO: about 32
instructions for each byte of DINDX.DAT, if all of it is that file's reading. No compiler pass reaches that (question 3).

**The kernels** (`testdata/opt84/kernels`), nearly all in their own code:

| Kernel | What it is | Total | Largest opcodes |
|---|---|---:|---|
| `matmul` | 60×60 REAL multiply | 3,101,599 | `ADDL2` and `MULL3`, 28% each: subscripts |
| `sieve` | Sieve, ten times, a GO TO loop | 5,785,872 | `BRW` 19%, `MOVL` 16%, `BLEQ` 16% |
| `subs` | Bubble sort and dot product on adjustable dummies | 6,876,884 | `MOVL` 47%, `MOVF` 15% |
| `whet` | REAL mix with COMMON, dummies, MTH$ | 2,695,057 | `MOVL` 34%, `ADDF2` 18% |
| `textk` | Character loop on substrings | 485,525 | `MOVL` 21%, `BRW` 20% |

## What the generated code wastes

The inner loop of `matmul` shows most of it, 14 instructions per trip:

```
MATMUL.$7:
        MULL3   MATMUL.K,#60,R0         ; A's subscript
        ADDL2   MATMUL.I,R0
        MULL3   MATMUL.J,#60,R1         ; B's
        ADDL2   MATMUL.K,R1
        MULF3   MATMUL.A-244[R0],MATMUL.B-244[R1],R0
        MULL3   MATMUL.J,#60,R1         ; C's, on the right
        ADDL2   MATMUL.I,R1
        MULL3   MATMUL.J,#60,R2         ; C's again, on the left
        ADDL2   MATMUL.I,R2
        ADDF3   MATMUL.C-244[R1],R0,MATMUL.C-244[R2]
MATMUL.L30:
        INCL    MATMUL.I
        DECL    MATMUL.$C5
        BLEQ    MATMUL.$8
        BRW     MATMUL.$7
MATMUL.$8:
```

After the proposed passes it could be three:

```
MATMUL.$7:
        MULF3   (R6)+,R8,R0
        ADDF2   R0,(R7)+
        SOBGTR  R9,MATMUL.$7
```

The patterns, from the kernels and DUNGEON:

1. **A branch around a `BRW`** at every conditional branch and loop close: two
   instructions where one would do. The generator doesn't know distances, and
   MACRO doesn't lengthen a short branch.
2. **Index setup each time.** `MOVL I,R0` before each use of an array element,
   though the index is unchanged. `QHERE` loads it up to twice per trip.
3. **Every variable in memory.** Each operand is a 5-byte `L^` reference to
   `$LOCAL`, another psect, so registers save bytes as well as references.
4. **Whole subscripts recomputed**, as in `matmul`, including the same one
   twice in a statement.
5. **Adjustable arrays' A0** loaded from the descriptor cell for each element,
   two `MOVL`s per reference (`subs`, 47% `MOVL`).
6. **Dummy elements with constant subscripts:** `MOVL #1,R1` then
   `@4(AP)[R1]`, since `@n(AP)` has no room for an offset (`whet`, 34% `MOVL`).
   With the address in a register, this is `4(R6)`.
7. **A store and its reload at a function's end:** `CLRL F`, `MOVL F,R0`, `RET`,
   91 of DUNGEON's 1,053 `RET`s by a static count.
8. **`CMPC3` and `MOVC3` of one character** (`LINE(I:I)`, `textk`; DUNGEON runs
   28,856 `CMPC3`s), where `CMPB` and `MOVB` would do and leave R0 to R3 alone.

Little is left for constant propagation: PARAMETER names are already constants
in sema, and `arith` folds as it lowers. Unreachable code is nearly absent, with
only 3 instructions directly after a jump in DUNGEON.

## What the passes can gain

Hand-applied, then assembled, linked, and run; each printed what the original
printed. These are upper estimates of what real passes will do on this code.

**`matmul`, stage by stage (cumulative):**

| Stage | Instructions | | Data reads | Data writes |
|---|---:|---:|---:|---:|
| As generated | 3,101,599 | | 3,033,035 | 692,715 |
| Loop close (`SOBGTR` on the count in memory) | 2,673,199 | −13.8% | 3,033,035 | 692,715 |
| Local CSE | 2,025,199 | −34.7% | 2,385,271 | 683,228 |
| Invariants moved out, to memory cells | 1,391,599 | −55.1% | 2,160,122 | 694,027 |
| Registers and stepped addresses | 750,799 | −75.8% | 629,721 | 247,630 |

**All the passes together:**

| Program | Instructions before | After | Change | Data reads before → after |
|---|---:|---:|---:|---|
| `sieve` | 5,785,872 | 3,073,873 | −46.9% | 3.61M → 0.37M |
| `subs` | 6,876,884 | 2,463,514 | −64.2% | 7.60M → 1.37M |

**DUNGEON's game, as a whole run:**

| Build | Total | Change | Own code | Change | Data reads | Data writes |
|---|---:|---:|---:|---:|---:|---:|
| As generated | 6,818,609 | | 4,504,906 | | 5,919,438 | 2,641,419 |
| Branch peephole on every module | 5,667,990 | −16.9% | 3,354,287 | −25.5% | 5,679,438 | 2,641,379 |
| And `QHERE` in registers | 5,070,333 | −25.6% | 2,756,630 | −38.8% | 4,182,932 | 2,053,390 |

The peephole also shrank the image by 2.7% (191,487 to 186,367 bytes, whole
blocks). The peephole here is a real program, `measure/peep.py`, run over all
of DUNGEON's MACRO. Its output printed byte for byte what the original did.

**A warning from the prototype.** Its size estimate is crude, and it
changed almost nothing in `matmul` or `subs` (under 0.1% each): their loop
bodies looked too far for a byte branch. `sieve` gained 7.3% and `whet` 1.5%.
The hand-written `SOBGTR` in `matmul` was in range and gained 13.8%, so the real
pass needs operand sizes close to what MACRO will choose. `whet` and `textk`
have no hand-optimized version, so their full gains are unmeasured.

## The pass list, confirmed and reordered

| Pass | Plan's rank | Evidence | Proposal |
|---|:--:|---|---|
| Branch shortening, loop close, peephole | last | 25% of DUNGEON's own code, alone | **First.** Needs only an instruction list |
| Constant propagation, folding, dead code | first | Little left | **Smaller:** store-to-load forwarding, dead compiler cells, the function value |
| Local CSE | second | `matmul`: 21 points | Keep |
| Loop-invariant motion | third | `matmul`: 20 points, with hoisted array bases | Keep |
| Register allocation | fifth | `matmul`: 21 points and most of its references; `QHERE`; `sieve`'s reads −90% | Keep, **before** induction variables |
| Induction variables, strength reduction | fourth | Smaller than planned (below) | Keep, **last**, and only where a stride isn't the element's size |
| Global CSE, PRE | open question | No measured case needs them | **Leave out** (answers the phase's open question) |
| One-character strings as `CMPB`/`MOVB` | not listed | `textk`, DUNGEON | **Add**, in lowering |

Why induction variables shrink: the VAX's indexed mode already scales. Once I
is in a register and a column's base is hoisted, `A(I,K)` is `(R6)[R9]` with no
stepped address at all. Only a stride that isn't the element's size needs one:
a row-wise loop, or a CHARACTER\*n array.

## Decisions (proposed)

1. **Analyses work from the control flow graph, never from DO statements.**
   `sieve`'s hottest loop is a GO TO loop, and the extended range of a DO
   (`loops.for`) leaves and re-enters a loop. Natural loops handle both: a block
   that jumps back in is part of the loop. Assigned GO TO lists its targets, and
   ENTRY points and VFE cases are further entries to the graph.
2. **The IR gets register cells, not virtual registers.** Every pass goes on
   working with Cells: CSE and hoisting make temporaries, as lowering does, and
   the allocator's result is a `Cell.Register` that codegen prints as `R8`.
   Trees and the Sethi–Ullman generator stay as they are, and a dump stays
   readable.
3. **R6 to R11 are the allocator's, R0 to R5 the expressions'.** In all of the
   corpus and DUNGEON, one routine touches R6 and none goes higher, and MOVC and CMPC clobber
   only R0 to R5, so values in R6 and up also survive them. The generator's
   pool shrinks to R0–R5 in a routine that uses a register cell; a routine that
   needs more, as H_floating might, gets fewer register cells rather than a
   failure, with `alloc`'s panic kept as the check. A CALLS preserves R6–R11
   only because each entry mask saves them, so a routine's mask must name them.
4. **Codegen gets an instruction list before text.** `emit` writes text
   straight to a `Writer`, so nothing can look at it afterward. It will collect
   a routine's instructions, labels, and comments, the peephole pass will work
   on those, and then they are written. **`/NOOPTIMIZE` output stays byte for
   byte what it is today**, so every golden file holds.
5. **Branch distances are computed, never hoped for.** Each instruction gets an
   upper bound on its length from its operands, kept true to MACRO's choices
   by a test against what MACRO assembles. A branch is shortened only if the
   bound says it fits, repeating until nothing changes. Output must also go
   through real MACRO (Phase 86's optional round), which won't relax a branch.
6. **Variables keep their values across calls.** The manual's note to SAVE
   (section 4.14) says VAX FORTRAN retains them automatically. So a variable in
   a register has its memory home written before a RETURN, and at a loop's
   exits if used after them. Only the compiler's own cells, and a function's
   value cell, can be dead. A local's value on entry is always unknown; DATA
   values aren't constants unless nothing in the unit can change them.
7. **What may change a place.** Two places conflict by their real extents, not
   by name: cells overlap by offset in their section, since EQUIVALENCE gives
   each name its own Cell at a shared offset, and a union's fields and
   substrings are byte ranges. A local is **address-taken** if it is ever passed
   by reference or descriptor, named by %LOC, in an I/O list, in a NAMELIST, or
   used by a VFE. Address-taken is for the whole unit and for good, because
   ASSOCIATEVARIABLE and a `$QIO`'s IOSB are written by calls that don't name
   them. Any call may change an address-taken local, COMMON, or a dummy. A local
   that is neither survives calls, as the language has no recursion (the manual
   never mentions it). VOLATILE (`Mem.Volatile`) is never moved, merged, or
   removed.
8. **Faults aren't moved onto paths that had none.** Division, floating
   operations, and floating loads (a reserved operand faults) are hoisted only
   from blocks that run on every trip. Otherwise a zero-trip or conditional case
   could fault in a program that was correct. Integer adds, multiplies, and
   address arithmetic may always move, with overflow not enabled. CSE may merge
   two identical operations, so a handler can see fewer conditions: the program
   asked for the same value twice. A constant that faults when folded is left
   to fault at run time. When Phase 85 adds `/CHECK`, those checks must stay.
9. **ASTs and handlers get no special treatment beyond this.** A variable an
   AST routine changes must be VOLATILE or in COMMON across a call, as the
   manual's section 4.16 implies. `corevms`'s handling of registers on an unwind
   and of LIB$FIXUP_FLT follows the architecture, so register cells need nothing
   more.
10. **Both-ways testing covers more than the corpus.** Every corpus program
    and module runs both ways (both paging modes for the existing subset), plus
    DUNGEON's scripted game where its source is present, skipped otherwise. New
    programs in `testdata/fortran` target each decision above: EQUIVALENCE
    overlaps, a DO variable after an exit by GO TO, COMMON changed by a callee in
    a loop, ASSOCIATEVARIABLE, zero-trip loops with a division in them, ENTRY
    into a loop's routine, and VFE reading a loop variable.
11. **Measuring stays out of the engine.** A nil check per instruction on the
    hottest path isn't worth keeping for one phase. The both-ways test logs
    `Engine.InstructionCount` and the memory statistics around each RUN, which
    exist already. Per-PC detail stays in `testdata/opt84/measure`'s patch.
    The kernels join the corpus, with outputs from an independent calculation
    as the harness requires (`whet` only if that can be done: its result is a
    floating chain).
12. **The MACRO file stays readable.** A register cell's name and register go
    in the routine's storage map, moved code keeps a comment naming its source
    line, and each pass's doc comment teaches on a small FORTRAN example, as
    planned. Debugging is at the `.MAR` level, so a `DBG>` user looking at a
    register-held variable needs that map (MANUAL.md, at the close-out).

## Questions for the author

1. **Dummy arguments that alias.** The manual has no rule on a dummy changed
   through another name; the plan's design note assumes the standard's rule,
   that a program may not do this. Later compilers added `/ASSUME=DUMMY_ALIASES`
   for programs that broke it. **Recommended:** follow the standard's rule, with
   no qualifier, as the plan says. The cost of being safe instead is high: in
   `matmul` written with dummy arrays, storing C(I,J) would force B(K,J) to
   reload every trip. It does mean an old program breaking the rule could print
   something different only with `/OPTIMIZE`.
2. **The default.** The plan switches to `/OPTIMIZE` at the close-out if the
   corpus passes both ways. With a new, less-tried optimizer, debugging at the
   `.MAR` level gets harder. **Recommended:** keep the plan, and require DUNGEON's
   game to pass as well as the corpus.
3. **FTNRTL's character loops.** They are a third of DUNGEON's game and 93% of
   its startup, which no compiler pass can touch. **Recommended:** not in this
   phase, whose goal is the compiler's output. Log them for a study in
   PERFORMANCE.md after Phase 86, unless you'd like them sooner.
4. **`CMPB` and `MOVB` for one-character strings.** This is a change to
   lowering rather than a pass. **Recommended:** do it only under `/OPTIMIZE`, so
   Decision 4's golden files hold, as part of subtask 3 or a small subtask of its
   own.

## Subtasks (revised by the survey; the review may change them)

1. **Survey** (done 2026-11-01): this Part A.
2. **The pass framework:** `internal/fortran/opt`, `/[NO]OPTIMIZE` and
   `--optimize`, `ir.Check` after each pass, the both-ways test, its counts
   logged, the kernels.
3. **The instruction list, branch shortening, the peephole:** `MOVL #0`, `ADDL2
   #1`, branches to branches, inverted branches, `SOBGTR`, `AOBLEQ`, `ACBL` on
   counts in memory.
4. **The analyses:** the control flow graph with every entry, dominators,
   natural loops, the places and effects of Decision 7. No code changes yet;
   tested on their own.
5. **Forwarding and dead code:** store to load in a block, constants across
   statements, dead compiler cells, the function value, unreachable blocks.
6. **Local common subexpressions.**
7. **Loop-invariant motion:** with hoisted array bases and descriptor A0s, by
   Decision 8's rules.
8. **Register allocation:** register cells, liveness, write-backs by Decision
   6, dummy addresses loaded once, the loop instructions on registers.
9. **Induction variables and strength reduction,** where a stride isn't the
   element's size, and autoincrement.
10. **Close-out:** the measured gains against this survey's table;
    FORTRAN-INTERNALS.md's optimizer chapter; MANUAL.md; the default switched.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-11-01:** Subtask 1, the survey, written as Part A, for the author's
  review. Measured the corpus, DUNGEON, and five new kernels with a temporary
  per-PC hook; kernels, hand-optimized code, and measuring code are in
  `testdata/opt84`. DUNGEON compiled, linked, and ran a scripted game on the way,
  earlier than Phase 86 needed it to.
