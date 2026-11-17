# Phase 101 — Fortran 90: control constructs

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A, below, with a revised subtask list
and questions for the author), is done (2026-11-17) and was accepted by the
author the same day ("The author's answers"); subtask 2 (construct names,
DO with no loop control, EXIT, and CYCLE) is done (2026-11-17), and so are
subtask 3 (SELECT CASE in sema, 2026-11-17), subtask 4 (SELECT CASE's
code, 2026-11-17), and subtask 5 (programs end to end, 2026-11-17). Needs
Phase 100 (done).

## Goal

Fortran 90's control constructs (the standard's 8.1):

- **SELECT CASE**: `SELECT CASE (expr)`, `CASE (1, 3:5, 10:)`, `CASE ('A':'Z')`,
  `CASE (.TRUE.)`, `CASE DEFAULT`, `END SELECT`; INTEGER, CHARACTER, and LOGICAL
  selectors; overlapping cases an error at compile time (8.1.3.2);
- **DO with no loop control** (`DO … END DO`), ended by EXIT;
- **EXIT** and **CYCLE**, with or without a construct name;
- **construct names** on IF, DO, and SELECT CASE (`OUTER: DO`, `END DO OUTER`,
  `EXIT OUTER`, `ELSE IF (…) THEN OUTER`, `CASE (1) OUTER`), checked as the
  standard's constraints say (a name ends where it began, EXIT and CYCLE name
  only a DO that contains them).

## What earlier phases leave in place

- `sema/stmt.go`'s stack of open constructs and `labels.go`'s jump checks; the
  IR's `Switch` (CASEL, Phase 79) and `Jump`; DO WHILE and labelless DO.
- The optimizer's loop analysis (natural loops), which sees EXIT and CYCLE as
  ordinary jumps.

## Design notes

- **SELECT CASE's code.** An INTEGER selector with dense constant cases becomes
  `Switch` (CASEL), ranges filling their slots; a sparse one, or a range wider
  than a limit, becomes a chain of comparisons ordered as the cases are, or a
  binary search over sorted cases if there are many: one function decides,
  with its threshold explained. CHARACTER cases compare as CHARACTER relations
  do (the shorter blank-padded), LOGICAL ones are a Branch.
- EXIT jumps past the construct's END DO; CYCLE to the loop's increment (for a
  counted DO) or its test (DO WHILE, no control). Both leave a DO's iteration
  count right, since the count is in a cell, not recomputed.
- Construct names are scoped to the unit: a construct name may not also be a
  variable's (the standard's 14.1.2).

## Open questions

1. Should `/WARNINGS` flag a SELECT CASE with no DEFAULT that can miss? (The
   standard allows it: nothing happens.) Proposed: no.

The provisional subtask list is replaced by Part A's A.4.

# Part A — the survey (subtask 1)

Read for it: the standard's 8.1 (R801 to R835: blocks, the IF, CASE, and DO
constructs, CYCLE and EXIT), 8.2 (branch targets), and 14.1.2 (construct
names as local entities); the Handbook's 8.3 to 8.6; the HP manual's 7.4
(CASE) and 7.6 (DO, CYCLE, EXIT); and what the lexer, parser, sema, the IR,
codegen, and the optimizer do today.

## A.1 What the standard asks

- **Construct names** (8.1). An IF, CASE, or DO construct may be named: the
  name and a colon before its first statement, and the same name as its last
  token (`END IF name`, `END SELECT name`, `END DO name`). The ELSE IF, ELSE,
  and CASE statements may repeat it, and must not name another. A construct
  named at its start must be named at its end; one unnamed at its start must
  not be named at its end (R806's, R811's, and R825's constraints). A named
  DO with a label must end at an END DO naming it (R825), not at a CONTINUE
  or an action statement. A construct name is a local entity of class (1)
  (14.1.2): it may not also be a variable's, a named constant's, a statement
  function's, a namelist group's, a procedure's, or the unit's name, nor the
  name of another construct in the unit; a COMMON block may share it. "A
  statement belongs to the innermost construct in which it appears unless it
  contains a construct name, in which case it belongs to the named construct."
- **Blocks** (8.1.1). A construct inside a block lies wholly in it; control
  may leave a block but never enter one from outside.
- **The CASE construct** (8.1.3). `[name:] SELECT CASE (expr)`, then any
  number of `CASE selector [name]` each with its block, then `END SELECT
  [name]`. Nothing may come between SELECT CASE and the first CASE (R808 has
  no place for it). The case expression is a scalar INTEGER, CHARACTER, or
  LOGICAL; a selector is DEFAULT (at most one in the construct, anywhere among
  the CASEs, not necessarily last) or a list of ranges: `v`, `low:`, `:high`,
  `low:high`, each value an initialization expression (a constant expression)
  of the case expression's type, a CHARACTER one of any length but the same
  kind. A LOGICAL construct has no colon forms. No two ranges may overlap:
  "there must be no possible value of the case-expr that matches more than
  one case-value-range". The expression is evaluated once (the case index);
  a single value matches by `.EQ.` (INTEGER, CHARACTER: the shorter padded
  with blanks, 7.2.3) or `.EQV.` (LOGICAL), a range by `.LE.` at each end it
  has; with no match the DEFAULT block runs, or nothing. A CASE statement is
  not a branch target; END SELECT is one only from within its construct
  (8.2); SELECT CASE is one from anywhere.
- **The DO construct** (8.1.4). Beside VAX FORTRAN's forms, `[name:] DO
  [label]` with no loop control, which runs "as if a large positive iteration
  count, impossible to decrement to zero, were established"; DO WHILE is
  defined as that with `IF (.NOT. (e)) EXIT` first in the body. A labeled
  loop without control may end at its labeled statement, as any labeled DO.
- **CYCLE and EXIT** (8.1.4.4.3, 8.1.4.4.4). `CYCLE [name]` and `EXIT [name]`
  must be within a DO construct, or within the named one, which must be a
  DO. Without a name each belongs to the innermost DO it is in. CYCLE goes on
  to step 3 of the execution cycle (the count decreased, the variable
  stepped, then the next test); in a nonblock DO the terminal statement is
  not executed. EXIT ends its loop and every loop inside it that contains the
  EXIT; so does a named CYCLE for the loops inside the named one. A loop's
  variable keeps the value it had. Neither may be a nonblock DO's terminal
  statement (R829's and R833's constraints). A logical IF may execute either
  (they are action statements), the common `IF (X < 0) EXIT`.
- **The HP manual** says the same, more strictly in one place: "Each
  case-value must be of the same type **and kind parameter** as expr." The
  1991 standard asks only for the same type (and for CHARACTER the same
  kind). gfortran accepts INTEGER values of another kind.

## A.2 What govax has

- **The lexer** (Phase 99) classifies every statement here. `SELECT CASE`,
  `CASE`, `END SELECT`, `EXIT`, and `CYCLE` are keyword kinds (`kinds.go`);
  the = test comes first, so `CASE(1) = 2`, `EXIT = 1`, and fixed form's
  `SELECTCASE(I) = 1` are still assignments, and no VAX FORTRAN text changes
  its meaning: nothing here depends on `/F90` (Phase 99's Decision 2). A
  construct name before a colon is `Statement.Construct`, allowed on DO, DO
  WHILE, IF THEN, and SELECT CASE (NOTCONSTRUCT otherwise). A plain `DO`, or
  `DO 10`, is classified as a DO statement whose control is missing.
- **The parser** keeps a construct name in `StmtBase.Construct` and reads
  the one after ELSE IF, ELSE, END IF, and END DO (`constructAfter`).
  `parseDo` requires a variable, so `DO` alone is a syntax error today.
  SELECT CASE, CASE, END SELECT, EXIT, and CYCLE are `PendingStmt`s.
- **sema** reports the first construct name in a unit NOTIMPL
  (`notimpl.go`'s `constructName`), and SELECT CASE NOTIMPL once, its CASEs
  and END SELECT quietly (`pendingOpeners`). Its stack of open constructs
  (`stmt.go`'s `construct`) holds IF blocks and DO and DO WHILE loops; a
  closer that finds a construct of another kind open inside its own reports
  DONEST and closes it; `closeAll` reports what the unit's END finds open.
  `labels.go` gives each label the path of constructs open at it (an IF arm
  or a loop per frame) and refuses a jump that enters an IF arm (INTOBLOCK);
  END IF counts as in its last block; ELSE IF and ELSE aren't targets.
  `forbiddenTerminal` lists what may not end a labeled loop.
- **The IR** lowers a DO through `countedLoop` (the count in `$Cn`, the
  variable stepped and the count decreased after the body, a branch back
  while the count is positive) and DO WHILE through `doWhile` (a test block,
  the body, a jump back). A labeled END DO starts a block of its own
  (`loopEnd`'s `startLabel`), since a jump to it goes on to the next pass:
  exactly CYCLE's target. `logicalIf` writes `IF (e) GO TO n` as one branch.
  `Switch` is the computed GO TO's CASEL, its index 1-based (codegen's
  `caseTable` writes base `#1`; `opt/forward.go` folds a constant index the
  same way). `cond` compares CHARACTER values (`CharCmp`) only as a branch's
  condition, where no register is held.
- **The optimizer** (Phase 84) works on the routine's graph: natural loops,
  dominators, liveness, and the register allocator's loop variables written
  back on every edge out of a loop. A GO TO out of a loop, or to a labeled
  END DO, already makes the shapes EXIT and CYCLE make. `DoLimits` keys on
  `ir.CountedLoop`'s statements (Advance, Less, Close), which stay together
  in one block whatever jumps to it.

## A.3 Findings, and what they change

1. **The lexer needs little.** Fixed form squeezes blanks, so `CASE DEFAULT
   OUTER` arrives as `CASE` and one name `DEFAULTOUTER`: the parser takes a
   name beginning `DEFAULT` after CASE as DEFAULT and the rest, if any, as
   the construct name (free form's blanks keep them apart already). `EXIT
   OUTER` is `EXIT` and the name `OUTER` in both forms. `OUTER: DO 40 I =
   1, M` (a named, labeled DO) is already classified.
2. **The parser's new nodes:** `ast.SelectCaseStmt` (the expression),
   `ast.CaseStmt` (a list of `ast.CaseRange`s, each with optional `Low` and
   `High` and a `Colon` flag, or `Default`), `ast.EndSelectStmt`, and
   `ast.ExitStmt` and `ast.CycleStmt` with the loop name they give (not
   `StmtBase.Construct`, which is a statement's own construct name).
   `parseDo` takes a DO with no control as a `DoStmt` with no variable
   (`Forever()`), labeled or not. CASE and END SELECT read their trailing
   name with `constructAfter`.
3. **sema's stack gains names and a third kind of construct.** `construct`
   gets `name` and a `sel *Select` with its current case. The checks are the
   standard's: a closing or middle statement's name against its opener's
   (CONSTRUCTNAME, one message with both names), a named labeled DO's end
   (NAMEDEND), a name used twice (DUPCONSTRUCT), and at the unit's end a name
   in the symbol table, which covers variables, PARAMETERs, statement
   functions, procedures referenced, and the unit's name (NAMECLASH; namelist
   groups are their own table, checked too, and COMMON blocks aren't). CASE
   and END SELECT find their SELECT as `matchIf` finds an IF: an IF or loop
   still open inside it is DONEST. A statement between SELECT CASE and its
   first CASE is CASEFIRST. sema's node is `Select` (the selector, `Cases`
   each with ranges and a body, `Default`, `EndSelect`); a DO with no control
   is a `DoWhile` with a nil `Cond`, as the standard defines DO WHILE through
   it.
4. **EXIT and CYCLE are resolved by sema** against the stack: without a name
   the innermost loop, with one the open construct of that name, which must
   be a loop (NOTALOOP: "EXIT OUTER names a SELECT CASE construct"); a name
   not open is NOLOOPNAME, and either statement outside every loop
   NOTINLOOP. The sema nodes `Exit` and `Cycle` point at the `Do` or
   `DoWhile` they leave or continue. `forbiddenTerminal` gains EXIT, CYCLE,
   SELECT CASE, CASE, and END SELECT.
5. **Branch targets.** `jumpTarget` refuses a CASE statement (NOTEXEC's
   text already fits). END SELECT gets a path whose last frame is its
   SELECT with no arm, and `inBlocks` lets such a frame match any arm of the
   same construct: a jump to it from inside any CASE block is good, from
   outside INTOBLOCK ("goes into a SELECT CASE construct" in its wording,
   which then names the kind). SELECT CASE itself is an ordinary target.
   One thing found beside this: VAX FORTRAN's rule that a jump to END IF may
   come only from its last block (section 5.7.3) is stricter than Fortran
   90's, which lets one come from any block of the IF and from outside it
   (8.2). Question 4.
6. **The IR's targets.** The lowerer keeps a stack of the loops it is in,
   each with its exit block and its next-pass block; `Exit` jumps to the
   first, `Cycle` to the second. The next-pass block is the counted loop's
   step code (made a block of its own only when a CYCLE needs it, so a loop
   without one lowers as today and `TestNoF90Unchanged` and the golden IR
   dumps hold), DO WHILE's test, and a DO with no control's first body
   block. `IF (e) EXIT` and `IF (e) CYCLE` become one branch, as `IF (e) GO
   TO n` does. A loop with no control is its body and a jump back; its only
   ways out are EXIT, GO TO, RETURN, and STOP.
7. **SELECT CASE's code**, by the selector's type:
   - **LOGICAL:** one `Branch` on the selector's bit 0 (the VAX's rule for a
     LOGICAL, the 1988 manual's appendix C.4), to the `.TRUE.` case, the
     `.FALSE.` case, or DEFAULT.
   - **INTEGER:** the selector widened to INTEGER\*4 into a temporary (so a
     value outside an INTEGER(1) or (2) selector's range simply never
     matches), then one function, `caseForm`, chooses among three forms. Let
     *tests* be the comparisons a chain would make (one per single value, two
     per bounded range) and *span* the distance from the lowest bounded value
     to the highest, plus one. A **table** (CASEL) when tests ≥ 4 and span ≤
     4 × tests: dense enough that the table's two bytes a slot cost less
     than the chain's compares, each range filling its slots, a gap DEFAULT.
     Otherwise a **binary search** over the sorted, disjoint ranges when
     tests ≥ 8, and a **chain** of comparisons in the order the cases are
     written when fewer (a programmer may put the common case first). Open
     ranges (`low:`, `:high`) lie wholly beyond the bounded ones, since none
     overlap, so they are two comparisons before the table or search.
     `Switch` gains a `Base` (CASEL's second operand; the computed GO TO's
     is 1), which codegen and `forward.go` use. A constant selector folds to
     a jump.
   - **CHARACTER:** a chain, or for tests ≥ 8 a binary search, of
     `CharCmp`s, each a branch's condition as `cond` requires. A selector
     that is a variable, an array element, or a substring with no function
     reference in it is compared where it is; any other (a concatenation, a
     function's value) is moved first into a temporary of its length, as a
     concatenation is (a static `TextCell`, or a class D string freed after
     the construct for a length known only at run time).
8. **Overlaps are checked in sema** on the folded values: INTEGER ranges as
   int64 intervals; CHARACTER ranges by the CHARACTER comparison (blank
   padding), in the VAX's ASCII order, which is also the run time's (CMPC);
   LOGICAL values by `.EQV.`. Each pair that overlaps is CASEOVERLAP, naming
   the line of the other. A range whose low is above its high matches nothing
   and overlaps nothing; it, and an INTEGER value outside the selector's
   kind's range, earn a warning (question 3).
9. **Types.** A REAL, COMPLEX, or record selector is CASETYPE; a case value
   that isn't a constant expression CASEVALUE; a value of another type than
   the selector's CASETYPE too; a colon with a LOGICAL selector CASERANGE; a
   second DEFAULT TWODEFAULT. INTEGER values of another kind than the
   selector's are accepted and compared by value (question 2). BYTE is
   LOGICAL\*1 in VAX FORTRAN (question 1).
10. **The optimizer sees nothing new.** EXIT and CYCLE make the edges a GO TO
    to the statement after a loop, or to a labeled END DO, already makes;
    SELECT CASE makes branches and a `Switch`, which every pass handles
    (`forward.go` gains the base). Subtask 5's tests are the optimizer's
    risk: EXIT out of nested counted loops whose variables live in registers
    (written back on the exit edges), CYCLE in a loop `DoLimits` turns into
    AOBLEQ, and a loop with no control that `Rotate` must leave alone or
    rotate correctly.
11. **What the debugger and listings see.** Under `/DEBUG` the matching code
    is the SELECT CASE line's (that is where the dispatch is); a CASE line has
    no code of its own, as ELSE has none today. The cross reference
    (`/CROSS_REFERENCE`) lists labels and names; construct names are names of
    the unit, so they should be listed too, defined at their opener and
    referenced at each statement that names them (question 5).
12. **What `/STANDARD` says** is Phase 110's (Phase 100's finding 14);
    nothing here.

## A.4 The subtasks, revised

1. **Survey** (this Part A) (done 2026-11-17; accepted by the author the
   same day).
2. **Construct names, DO with no control, EXIT, and CYCLE:** the parser's
   forms (findings 1 and 2), sema's names and their checks (finding 3),
   EXIT's and CYCLE's resolution (finding 4), the IR's loop stack and
   targets with `IF (e) EXIT` as a branch (finding 6), and the cross
   reference's construct names (question 5) (done 2026-11-17).
3. **SELECT CASE in sema:** parsing, the `Select` node, the checks of
   findings 3, 5, 8, and 9, and branch targets; the code still NOTIMPL
   (done 2026-11-17).
4. **SELECT CASE's code:** `Switch.Base`, `caseForm` and its three forms for
   INTEGER, LOGICAL's branch, CHARACTER's chain and search with the
   selector's temporary (finding 7); each form held by IR golden dumps and a
   run test, with the boundaries of `caseForm`'s thresholds tested on both
   sides (done 2026-11-17).
5. **Programs end to end** (`testdata/fortran90`: a tokenizer by SELECT
   CASE on CHARACTER, nested named loops with EXIT and CYCLE, every form of
   INTEGER selection, the standard's own examples), both optimizer settings
   and `/RECURSIVE`, gfortran's witness (`compare.py`'s `PROGRAMS90`); the
   optimizer's cases of finding 10 (done 2026-11-17).
6. **Close-out:** FORTRAN-INTERNALS.md's section 26, MANUAL.md's 14.13, HELP
   FORTRAN.

## A.5 Questions for the author

1. **BYTE selectors.** In VAX FORTRAN BYTE is LOGICAL\*1, used as a small
   integer; in the HP compiler it is INTEGER(1). By the standard, `SELECT
   CASE (B)` with B a BYTE is a LOGICAL selector, so `CASE (1:5)` is an error.
   (a) **The standard's rule: an error, whose message suggests INTEGER(1)**;
   (b) a LOGICAL\*1 selector with INTEGER case values matched as an
   INTEGER(1), DEC's treating logicals as integers carried over.
   **Recommendation:** (a): Phase 100 kept BYTE LOGICAL\*1 and added
   INTEGER(1) for exactly this kind of use, and (b) would make `CASE (.TRUE.)`
   and `CASE (1)` overlap in ways no manual describes.
2. **Case values of another INTEGER kind** than the selector's (`SELECT CASE
   (I2)` with `CASE (100000)`): accepted and compared by value, as the 1991
   standard and gfortran allow, rather than refused as the HP manual's
   wording would? **Recommendation:** accept.
3. **Warnings.** An empty range (`CASE (5:1)`) and an INTEGER value no
   selector of its kind can hold (`CASE (300)` for an INTEGER(1)) can never
   match: a warning, CASENEVER, under `/WARNINGS` as other warnings are? And
   open question 1, a SELECT CASE without DEFAULT that can miss: no warning
   (the standard defines it: nothing runs)? **Recommendation:** yes to both.
4. **END IF as a branch target.** Fortran 90 lets a jump to END IF come from
   any block of its IF and from outside it; govax (VAX FORTRAN's 5.7.3)
   accepts one only from the last block. Loosen to Fortran 90's rule, under
   both `/F90` settings since it only accepts programs refused today, and
   compiles no old program differently? **Recommendation:** yes, in subtask
   3 beside END SELECT's rule.
5. **Construct names in the cross reference** (finding 11)?
   **Recommendation:** yes, in subtask 2; they are names of the unit a
   reader looks for.
6. **`caseForm`'s thresholds** (finding 7): a table when tests ≥ 4 and
   span ≤ 4 × tests, a binary search when tests ≥ 8, else a chain in the
   written order. **Recommendation:** these; they are one function and easy
   to change once the programs of subtask 5 can be measured.

## The author's answers (2026-11-17)

Every recommendation accepted:

1. **BYTE selectors:** the standard's rule. A LOGICAL\*1 (BYTE) selector is
   LOGICAL, so a colon or an INTEGER case value is an error, whose message
   suggests INTEGER(1).
2. **INTEGER case values of another kind** than the selector's are accepted
   and compared by value.
3. **Warnings:** CASENEVER for an empty range and for an INTEGER value the
   selector's kind can't hold, under `/WARNINGS`; none for a SELECT CASE
   without DEFAULT (open question 1 settled: no).
4. **END IF as a branch target:** Fortran 90's rule (from any block of the
   IF, and from outside it), under both `/F90` settings, in subtask 3.
5. **Construct names in the cross reference:** yes, in subtask 2.
6. **`caseForm`'s thresholds:** a table when tests ≥ 4 and span ≤ 4 ×
   tests, a binary search when tests ≥ 8, else a chain in the written order.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-17:** Subtask 1, the survey: Part A above. The standard's 8.1,
  8.2, and 14.1.2, the Handbook's chapter 8, and the HP manual's 7.4 and 7.6
  read; the lexer already classifies every statement and nothing depends on
  `/F90`; EXIT and CYCLE reach blocks a GO TO already could, so the
  optimizer sees no new shapes; SELECT CASE needs a base on the IR's
  `Switch`. Twelve findings, a revised list of six subtasks, and six
  questions for the author.
- **2026-11-17:** The author accepted every recommendation of A.5 ("The
  author's answers"); subtask 1 is done.
- **2026-11-17:** Subtask 2, construct names, DO with no loop control, EXIT,
  and CYCLE. The parser: `DoStmt.Forever()` (a DO with nothing after its
  label), and `ast.LoopExitStmt` for EXIT and CYCLE (`parseLoopExit`; fixed
  form's `EXITOUTER` arrives already split after the keyword).
  `sema/constructs.go`: each construct's name as the stack opens it
  (`nameConstruct`, DUPCONSTRUCT), the names on ELSE IF, ELSE, END IF, and END
  DO checked against it (`checkName`: WRONGNAME, MISSINGNAME), a named
  labeled DO ending anywhere but an END DO (NAMEDEND, in `endLoops`), a
  construct name that also names a variable, a constant, a procedure, a
  statement function, a namelist group, or the unit (NAMECLASH, at the
  unit's end, so a later mention counts), and EXIT and CYCLE resolved to
  their loop (`loopExit`: NOTINLOOP, NOLOOPNAME, NOTALOOP) as a
  `sema.LoopExit` pointing at its `Do` or `DoWhile`; EXIT and CYCLE may not
  end a labeled loop (DOTERM). A DO with no control is a `DoWhile` with no
  `Cond`. The survey proposed one message, CONSTRUCTNAME, for a name that
  doesn't match; it became WRONGNAME and MISSINGNAME, whose texts each say
  one thing. Construct names are cross reference rows (`whatConstruct`), and
  sema's dump shows them. `ir/lowerloops.go`: a stack of the loops being
  lowered, each with its exit block and next-pass block; a counted DO's step
  code becomes a block of its own only when a CYCLE needs it (or is a
  labeled END DO's block already), so a loop without CYCLE lowers block for
  block as before; `IF (e) EXIT` and `IF (e) CYCLE` are one branch; a DO
  with no control is its body and a jump back. Tests: `TestErrors`' 21 new
  cases, `f90loops.for`'s sema and IR goldens (every kind of loop, in fixed
  form), `TestCrossReferenceConstructs`, and the program run by hand on
  govax both `/OPTIMIZE` and `/NOOPTIMIZE` (`4346 4`, as worked out); the
  FORTRAN and Fortran 90 corpora and `TestNoF90Unchanged` pass. MANUAL.md's
  14.13 and HELP FORTRAN no longer list these as not there yet (their full
  account is subtask 6's).
- **2026-11-17:** Subtask 3, SELECT CASE in sema. The lexer makes DEFAULT
  after CASE a keyword (`caseDefault`), so fixed form's `CASEDEFAULTPICK` is
  DEFAULT and the name PICK, and free form's `CASE DEFAULTS` is no DEFAULT;
  the parser's nodes are `ast.SelectCaseStmt`, `ast.CaseStmt` (its
  `ast.CaseRange`s, or `Default`), and `ast.EndSelectStmt`. sema's node is
  `Select` (its `Cases` in the order written, `Default` among them, and
  `EndSelect`); a `Case` embeds an `Arm`, so the construct stack's blocks,
  `add`, and labels' paths treat a CASE block as an IF's arm. The stack's
  matching is one function now (`match`): ELSE IF, ELSE, and END IF find the
  innermost IF block, CASE and END SELECT the innermost CASE construct
  (NOSELECT), each closing what is open inside it (DONEST), and END DO passes
  over a CASE construct as over an IF. `sema/selectcase.go` has the checks:
  CASEFIRST, NOENDSELECT, CASEEXPR (a selector of another type, which leaves
  the construct no type to check values against; a whole array is
  NEEDSUBSCRIPT already), CASEVALUE (through the declarations' evaluator, so a
  variable named in a case value isn't entered as one), CASETYPE, CASEBYTE
  (answer 1), CASERANGE, TWODEFAULT, CASEOVERLAP (each range against every one
  before it in the construct, INTEGER ones as intervals clipped to the
  selector's kind, CHARACTER ones by `compareText`, LOGICAL ones by truth),
  and CASENEVER, a warning (answer 3). The survey's one CASETYPE for both a
  selector and a value became CASEEXPR and CASETYPE. Labels
  (`labels.go`'s `labelPath`): END SELECT's frame matches any block of its
  construct (`frame.anyArm`), so a jump to it from inside is good and from
  outside INTOBLOCK, whose text now names the construct ("a SELECT CASE
  construct"); a CASE statement is no target (NOTEXEC); and END IF is outside
  its IF, so a jump to it may come from any block or from outside (answer 4,
  under both `/F90` settings; `TestErrors`' case that held the old rule now
  expects nothing). Every SELECT CASE is still NOTIMPL ("the SELECT CASE
  statement"), until subtask 4. Tests: `TestSelectCase` (68 cases, NOTIMPL
  left out), `TestIntoBlockWording`, `TestSelectCaseForms` (free form's
  forms and syntax errors), and `f90select.for`'s sema golden.
- **2026-11-17:** Subtask 4, SELECT CASE's code. `ir.Switch` has a `Base`
  (CASEL's second operand: 1 for the computed GO TO, alternate returns, and
  ENTRY's returns), which codegen's `caseTable`, `opt/forward.go`'s folding,
  and the dump (`switch I base -1 [...]`, the base shown when it isn't 1)
  use. `ir/lowerselect.go` lowers a `sema.Select`: the dispatch in the SELECT
  CASE statement, then each case's block in the order written, each ending
  with a jump past END SELECT (a labeled END SELECT's block is that join, as
  END IF's is). LOGICAL is one branch on the value (`cond`, so a CHARACTER
  comparison stays in it), to the `.TRUE.` case's block and the `.FALSE.`
  case's. INTEGER widens the value to a longword temporary, unless it is
  already a longword variable with no index, and clips every range to the
  selector's kind: an end at the kind's limit needs no test (so INTEGER(2)'s
  `-32768:-1` is one comparison), and a range left empty (CASENEVER's) is
  dropped. `caseForm(tests, span)` is the author's answer 6, `span` counting
  only ranges with both ends; a table's out-of-range exit goes to a chain of
  the open ranges, which lie wholly beyond the bounded ones. The binary search
  (`selection.search`, `half`) sorts the ranges by their low ends and splits
  on the middle one's, carrying what each comparison proved, so a leaf tests
  only the ends that knowledge doesn't settle (`condition`, `below`). A chain
  tests a range with both ends as one branch on an `.AND.`. CHARACTER is a
  chain, or a search for 8 tests or more, of `CharCmp`s (no table); a
  selector of constant length with no index is compared where it is (a
  variable, a substring with constant bounds, a concatenation's static
  temporary), any other has its address and length put in longword
  temporaries once, and its comparisons reach it through `Mem.Ptr`. The
  survey's "class D string freed after the construct" became `numbered`:
  when the selector is a dynamic string, each match stores its case's number
  in a temporary and goes to one block, which frees the string
  (`LIB$SFREE1_DD`) and goes on by a `Switch` on the number; the statement's
  `after` list is held back until then. A constant selector, of any type, is a
  jump to its case. An INTEGER selector's `after` (a dynamic string a
  function's argument used) is done once the value is in its temporary. sema
  no longer reports SELECT CASE NOTIMPL. Tests: `TestCaseForm` (the
  thresholds on both sides), `f90select.for`'s IR and codegen goldens (a
  table with base -1 and an open range outside it, a chain one value too
  sparse for a table, a chain of three tests, a search of eight tests and a
  chain of seven, INTEGER\*2 clipped, an expression and a constant selector,
  LOGICAL with DEFAULT, CHARACTER's chain, search, and run-time place, a jump
  to END SELECT, and a dynamic string in a subroutine), and the run test,
  `testdata/fortran90/selects.f90`, worked by hand, run in every mode
  `TestFortran90Programs` has and printing gfortran's lines
  (`compare.py`'s `PROGRAMS90`). MANUAL.md's 14.13 and HELP FORTRAN say
  SELECT CASE is there (their full account is subtask 6's); the manual's
  NOTIMPL example is now WHERE's.
- **2026-11-17:** Subtask 5, programs end to end. Four programs in
  `testdata/fortran90`, each worked by hand and run by `TestFortran90Programs`
  in all eight modes (paging off and on, `/NOOPTIMIZE` and `/OPTIMIZE`, each
  also `/RECURSIVE`), and the same on gfortran (`compare.py`'s `PROGRAMS90`):
  `tokens.f90`, a tokenizer by SELECT CASE on CHARACTER reading the terminal to
  its end (`tokens.in`); `loops.f90`, nested and named loops with EXIT and CYCLE;
  `cases.f90`, every INTEGER form with `caseForm`'s thresholds on both sides;
  and `examples.f90`, the standard's own examples of 8.1.2.3, 8.1.3.3, and
  8.1.4.5 in its capitals, made to run (unit IUN a scratch file). Finding 10's
  cases were looked at in `loops.f90`'s `/OPTIMIZE` MACRO: the named EXIT out of
  two counted loops whose variables are in R6 and R9 leaves through a block that
  stores both back; the CYCLE in a loop closed by `AOBLEQ` is a branch to the
  `AOBLEQ`; the loops with no control run as written. In `cases.f90`'s the
  forms are the ones `caseForm` chooses (TABLE16 a CASEL, CHAIN17 a chain,
  OPEN2's table based at -2). Example 7 (a CYCLE passing over a nonblock DO's
  terminal CALL) and example 8 (CYCLE in loops sharing a CONTINUE) ran right
  the first time. One fix: a format given as a CHARACTER constant, `WRITE (*,
  '(" QED")')`, reached FTNRTL with its quotation marks, which it reads as
  SYNERRFOR; under `/F90` sema now gives FTNRTL such a constant (or a
  PARAMETER's value) with its quoted strings between apostrophes, as a FORMAT
  statement's (`io.go`'s `constantFormat`, `format.Apostrophes` with `RunTime`),
  held by `TestConstantFormatQuotes`. A format in a variable is still given as
  it is, so `"…"` there is still SYNERRFOR (for "Where things stand").
