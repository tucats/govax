# Phase 79 — FORTRAN control flow and subprograms

**Status:** in progress: subtask 1 (the survey, Part A) done
2026-10-31, its decisions and recommendations accepted by the author the
same day; subtask 2 (GO TO in every form, ASSIGN, the arithmetic IF)
done 2026-10-31; subtask 3 (DO in every form, DO WHILE, END DO, the
extended range) done 2026-10-31; subtask 4 (STOP, PAUSE, RETURN, END)
done 2026-10-31; subtask 5 (function results of every type) done
2026-10-31; subtask 6 (ENTRY) done 2026-10-31; subtask 7 (alternate
returns) done 2026-10-31; subtask 8 (adjustable and assumed-size arrays,
passed-length character arrays) done 2026-10-31; subtask 9 (procedure
arguments, null arguments, Hollerith constants as arguments, %VAL, %REF,
%DESCR, %LOC) done 2026-10-31.
Needs Phase 78 (expressions in every type). Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md` (Decisions 8, 10, and 11).

## Goal

Generate code for all of the manual's chapters 5 and 6:

- **control statements:** unconditional, computed, and assigned GO TO;
  ASSIGN (of statement and FORMAT labels); arithmetic, logical, and
  block IF; DO with every form of control (INTEGER and REAL variables,
  negative and computed increments, the trip count rule), nested DO
  loops sharing a terminal statement, transfers out of and back into a
  loop's range (5.3.1.4, extended range); DO WHILE; END DO and DO
  without a label; CONTINUE; STOP and PAUSE with a code or message;
  RETURN with alternate returns; END;
- **subprograms:** SUBROUTINE and FUNCTION of every result type
  (CHARACTER and COMPLEX through the hidden argument), statement
  functions (expanded inline), ENTRY in subroutines and functions,
  alternate return arguments (`*label`), adjustable arrays with bounds
  from arguments or COMMON, assumed-size arrays, Hollerith constants as
  actual arguments, procedure names as arguments (EXTERNAL, and calling
  through a dummy what Phase 78 passes for INTRINSIC),
  null arguments, and %VAL, %REF, %DESCR, and %LOC (Table 6-1);
- **the calling standard** (Decision 11) as the rest of VMS sees it, so
  a FORTRAN routine can be a callback (Phase 83 uses this).

## What earlier phases leave in place

- **Phase 75:** CALL, simple functions, logical and block IF, labeled DO.
- **Phase 76:** label tables and the statement order rules.
- **Phase 78:** every type's code; character arguments and passed-length
  dummies; an intrinsic's name passed as an argument (its Part A's
  corrections, accepted 2026-10-30), with FTNRTL routines for the names
  MTHRTL lacks.

## Design notes

- **ENTRY:** each entry point is a `.ENTRY` that copies its arguments'
  addresses into the routine's static argument cells, then branches to
  the shared code; dummy arguments are reached through those cells, so
  one body serves every entry. The doc explains why FORTRAN can do this
  (static storage, no recursion).
- **Assigned GO TO** stores a label's address; with a label list the
  generated code checks it (CASEL over the list). *Corrected by the
  survey:* the manual says the list "does not affect statement
  execution", so GO TO v is `JMP @v` with a list or without; the list
  only tells the compiler where the jump may go (Part A, 5.6.3).
- **Alternate returns:** RETURN n sets R0 = n; the caller's CASEL
  dispatches on it, falling through for 0 or out of range.
- **Adjustable arrays:** their bounds and multipliers are computed on
  entry, from the arguments' values, into static cells (the manual: a
  later change to the bound variables doesn't change the array).
  Phase 76's subtask 5 declares and checks them: `sema/arrays.go` keeps
  each bound that isn't constant as its syntax tree (`Dim.LowerExpr`,
  `UpperExpr`) and reports NOTIMPL at it, in `checkArrays`.
- **PAUSE** writes its message to SYS$OUTPUT and reads a line from
  SYS$INPUT (VAX FORTRAN's PAUSE at a terminal waits for CONTINUE; in a
  batch-like run it is a no-op). The choice is recorded.
- **STOP n** ends the image with a status: the manual's rules, with the
  displayed text through FTNRTL.

## Open questions

- **PAUSE's behavior** at govax's terminal: the User Manual's
  description isn't in the folder; a prompt and a line read is the
  proposal. *The survey proposes LIB$PAUSE instead* (Part A, 5.8, and
  question 3).
- **A REAL DO variable's** iteration count when the increment is
  inexact: the manual's formula, computed in the loop variable's type.
  *Settled before this phase:* Phase 78 already computes the count in
  the variable's type and truncates it (`ir.tripCount`), and
  `testdata/fortran/reals.for` runs a REAL\*8 loop (Part A, 5.3).

# Part A — the survey (2026-10-31)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s;
"the calling standard" is the *OpenVMS Calling Standard* (V7.3), chapter
2, read from its PDF. The HP Fortran manual (2005) was read for PAUSE,
STOP, and RETURN, where it describes VAX FORTRAN's behavior on VMS. As in
Phase 78, what a rule means for a pass is in brackets, and a rule the
manuals leave open is our choice, marked unconfirmed, for MANUAL.md's
differences at the close-out.

## Where the compiler stands

Two programs of every form in chapters 5 and 6 (and a third of the forms
earlier phases might already have) were compiled with `govax fortran
/NOMACRO` on 2026-10-31. What compiles without a message today:

- CALL, with **null arguments** (`CALL FOO(X, , 3)` passes a longword 0;
  nothing left for subtask 9 but tests);
- the unconditional GO TO, the logical IF, and the block IF;
- a **labeled DO** of every INTEGER and REAL length, with shared terminal
  statements, the step kept in a cell when it isn't constant, the count
  computed in the variable's type (a REAL\*8 loop runs in
  `reals.for`), and a jump out of a loop and back into it (the
  **extended range**: sema gives no message, 5.3.1.4, and the count and
  step are static cells, so going back in finds them; not yet run end to
  end);
- CONTINUE, STOP (FTN$STOP), RETURN, and END;
- FUNCTIONs whose values come back in R0 or R0:R1: every type of 8
  bytes or fewer, COMPLEX\*8 included (`complex.for`);
- passed-length CHARACTER dummies and arrays of them (`CHARACTER*(*)
  A(*)`, element lengths from the descriptor), assumed-size arrays with
  any lower bound, and an intrinsic function's name passed as an
  argument (Phase 78).

What is NOTIMPL, and where it is reported:

| Form | Reported by |
|---|---|
| ASSIGN; computed and assigned GO TO; arithmetic IF; DO WHILE; END DO; PAUSE | `handled` (`sema/notimpl.go`) |
| a DO without a label | `handled`, the `DoStmt` case |
| ENTRY | `handled` |
| RETURN i | `executable` (`sema/exec.go`) |
| a `*` dummy | `params` (`sema/analyze.go`), which leaves the unit incomplete |
| a `*s` or `&s` actual argument | `formWhat`, "an alternate return argument" |
| a CHARACTER, REAL\*16, or COMPLEX\*16 FUNCTION, and a reference to one | `functionResult` (`sema/decl.go`), `apply` (`sema/expr.go`); `Type.returned` is the test |
| an adjustable array, bounds from dummies or COMMON | `checkArrays` (`sema/arrays.go`) |
| a dummy procedure, called or referenced | `call` (`sema/exec.go`), `apply` |
| an EXTERNAL name as an argument | `name` (`sema/expr.go`) |
| a statement function's definition (its references are errors meanwhile) | `assignment` (`sema/exec.go`) |
| %VAL, %REF, %DESCR, %LOC | `formWhat` (`BuiltinArg`) |
| a Hollerith constant as an argument | `hollerith` in `sema/expr.go` |

## Control statements (chapter 5)

**CALL (5.1).** Arguments "must agree in number, order, and data type".
Actual arguments may be any expression, a Hollerith constant, an
alternate return specifier, or a subprogram name. [Done but for the last
three, which are 6.1's.]

**DO (5.3.1).** `DO [s[,]] v = e1, e2[, e3]`; the label is optional, and
a loop without one ends at END DO. The terminal statement can't be an
unconditional or assigned GO TO, an arithmetic IF, any block IF
statement, END, RETURN, or DO [DOTERM has all but the arithmetic IF and
the assigned GO TO, which become statements now]. The count is
`[(e2 - e1 + e3)/e3]`, truncated toward zero; zero or negative runs the
body no times ("once" under `/NOF77`, Phase 85's qualifier). Parameters
are converted to v's type first; changing them in the loop changes
nothing. After the last trip v has been stepped once more (`DO 100
K=1,50,2` leaves K = 51). [All done for labeled loops. The count is
computed with the parameters' type's arithmetic, so a longword loop near
the type's limit (`DO I = 1, 2147483647`) overflows the count and runs
no times; the manual says nothing of it, and VAX FORTRAN's own code isn't
known. Ours, unconfirmed: left as it is.]

**Nesting (5.3.1.2):** nested loops may share a labeled terminal
statement "but not an END DO statement". A labeled END DO may end a
labeled DO (5.4's "optional" form), and then may not be shared.
[New messages: END DO with no loop to end, and a loop without a label
that the unit's END reaches; they follow NOIF and NOENDIF.]

**Transfers (5.3.1.3, 5.3.1.4):** out of a loop always; into one only
from its extended range, which only running the program can tell (the
`labels.go` comment says why there's no message); to a shared terminal
statement only from the innermost loop (SHAREDTERM).

**DO WHILE (5.3.2):** `DO [s[,]] WHILE (e)`, e logical, tested before
every trip, the first included; without a label, END DO ends it.
[Lowering: a test block at the top, the body, a jump back. A labeled DO
WHILE shares the labeled DO's terminal rules.]

**END (5.5):** a main program's END ends the program; a subprogram's is a
RETURN. [Done.]

**GO TO (5.6).** Unconditional [done]. **Computed:** `GO TO (s1,...,sn)
[,] e`, e converted to an integer; 1 to n go to that label, anything else
goes on to the next statement. [CASEL e,#1,#n-1 with a table of words:
CASEL itself falls through when out of range. A REAL index is converted
by truncation (CVTFL), as INT does.] **Assigned:** `GO TO v [[,](s,...)]`;
v must hold a label from an ASSIGN in the same unit; the list "does not
affect statement execution and can be omitted". [`JMP @v`. The list, or
without one every label the unit ASSIGNs, is the jump's possible
targets, checked like any jump's (INTOBLOCK, SHAREDTERM) and given to
the IR as the block's successors, for Phase 84's flow analysis.]

**ASSIGN (3.5):** `ASSIGN s TO v`, v an integer variable, s an executable
statement's label or a FORMAT statement's. v "becomes defined as a
statement label reference and undefined as an integer variable", and
arithmetic on it is unpredictable. [What is stored is question 1. A
FORMAT statement's label is ASSIGNed for `PRINT v` and its kind, which
is Phase 80's: until then, ASSIGN of a FORMAT label is NOTIMPL, as the
FORMAT statement itself is.]

**Arithmetic IF (5.7.1):** `IF (e) s1, s2, s3` for e negative, zero, or
positive; all three labels required, not necessarily different. [TST or
the expression's own condition codes, then BLSS s1, BEQL s2, and a jump
to s3, with branches to the same label merged. e must be INTEGER or REAL
of any length: a COMPLEX value has no sign, and a CHARACTER or LOGICAL
one isn't arithmetic, so each gets a message (ours).]

**Logical IF (5.7.2), block IF (5.7.3):** done, but the logical IF's
statement may now be any of this phase's (not DO, END DO, a block IF, or
another logical IF, which the parser refuses).

**PAUSE (5.8):** `PAUSE [disp]`, disp a character constant or one to
five digits. In a batch job or detached process, disp is written to the
output and the program goes on; interactively it is displayed (or
"FORTRAN PAUSE" without one), and the program waits for DCL's CONTINUE
(go on), EXIT (end), or DEBUG (go on under the debugger). The HP manual
adds that most other commands end the program. That is LIB$PAUSE, which
govax has (Phase 57, subtask 8: the Ctrl/Y level, CONTINUE, EXIT, and
STOP; DEBUG at that level is govax's console DEBUG, not the VMS
debugger's attach). [Question 3: FTNRTL's FTN$PAUSE writes the display to
SYS$OUTPUT, then calls LIB$PAUSE, and goes on whatever it returns, so a
process with no CLI (LIB$_NOCLI) just goes on, as in batch.]

**RETURN (5.9):** `RETURN [i]`, i any integer expression (converted), only
in a subroutine. RETURN i takes the i-th alternate return of the actual
argument list; i below 1 or past the number of them returns normally.
A function's RETURN gives the value last assigned to its name. [The
subroutine sets R0 to i, unchecked: only the caller knows how many
alternate returns it gave. A plain RETURN or END of a subroutine with `*`
dummies sets R0 to 0.]

**STOP (5.10):** as done by Phase 75 (`FTN$STOP`, SS$_NORMAL, SYS$OUTPUT).
The HP manual says a STOP without a code displays nothing, where the 1988
manual says "FORTRAN STOP"; the 1988 manual is the target (Decision 1).
Neither manual gives STOP n's exit status, so the doc's "STOP n ends the
image with a status" stays SS$_NORMAL, unconfirmed.

## Subprograms (chapter 6)

**Argument association (6.1.1).** Each dummy takes the actual's place;
a dummy array may be no larger than the actual; a dummy used as a
subprogram must have an actual declared EXTERNAL or INTRINSIC; a
CHARACTER dummy may be no longer than its actual.

**Adjustable arrays (6.1.1.1):** a dummy array whose bounds use integer
dummies or COMMON variables. The bounds are evaluated "once each time a
subprogram is entered", and changing the variables later changes nothing
(the manual's `X(-I/2:I/2,J)` example). With ENTRY, an entry's own list
decides which arrays can be evaluated: one whose dummies aren't all in
that entry's list (or in COMMON) is undefined until a later entry. A
variable used in a bound may not have its type changed afterwards
(BOUNDTYPE). [Each entry's code computes, for each adjustable array it
can, the bounds into static cells: each dimension's lower bound and
multiplier, the offset of the first element, and the size (for a class A
descriptor and SIZEOF). An element's address is then the dummy's address
plus an index built from the cells (`Mem.Index`), the constant offset
part of today's `Mem` becoming part of the index. `sema` keeps each
bound's tree (`Dim.LowerExpr`, `UpperExpr`); BOUNDVAR and BOUNDREF check
what it may hold.]

**Assumed-size (6.1.1.2):** done, with any lower bound, but for one whose
other dimensions are adjustable (`A(1:N,1:*)`), which comes with
adjustable arrays. Such an array can't be an I/O list's whole array, an
internal file, or a run-time format (I/O phases' checks).

**Passed-length CHARACTER (6.1.1.3):** done for dummies and arrays of
them. A passed-length array may also be adjustable or assumed-size [the
element size then comes from the descriptor, in the multiplier cells].

**Character and Hollerith constants as arguments (6.1.1.4):** a Hollerith
actual goes with a numeric dummy [by reference to its bytes, a datum in
`$PDATA`]. In VAX FORTRAN a *character* constant may go with a numeric
dummy too, "converted to a Hollerith constant by the FORTRAN compiler and
the linker"; but not when the subprogram is itself a dummy, where only a
character dummy will do. The linker's part is the VAX object language's
argument validation, which govax's MACRO can't write. [Question 4.]

**Alternate return arguments (6.1.1.5):** `*` in a SUBROUTINE's or
ENTRY's dummy list; `*s` or `&s` in the CALL; s an executable statement
of the calling unit. [The caller: `CALLS`, then `CASEL R0,#1,#n-1` over
the specifiers' labels, falling through for 0 and anything out of range,
as 5.9 says. The labels are jumps from the CALL statement, checked as
GO TO's are. What the argument list holds in the specifiers' places is
question 2.]

**Built-in functions (6.1.2, Table 6-1).** %VAL, %REF, and %DESCR only
in an actual argument list:

| Actual argument | Default | %VAL | %REF | %DESCR |
|---|---|---|---|---|
| LOGICAL, INTEGER expression | REF | yes, sign-extended to 32 bits | yes | yes |
| REAL\*4 expression | REF | yes | yes | yes |
| REAL\*8, REAL\*16, COMPLEX expression | REF | no | yes | yes |
| CHARACTER expression | DESCR | no | yes | yes |
| Hollerith constant | REF | no | no | no |
| aggregate (a record, Phase 82) | REF | no | yes | no |
| numeric array name | REF | no | yes | yes |
| CHARACTER array name | DESCR | no | yes | yes |
| numeric procedure name | REF | no | yes | yes |
| CHARACTER procedure name | DESCR | no | yes | yes |

The calling standard's Table 2-2 agrees (a value only for atomic data of
32 bits or fewer). %LOC(arg) is an INTEGER\*4 address of a variable,
array, element, record, or external procedure, usable in any expression.
[%VAL: the value, widened with CVTxL for a short integer or logical
(ZEXT first, the footnote says, for zero extension), pushed as the
argument. %REF of a CHARACTER: the text's address. %DESCR of a numeric
scalar: a class S descriptor of its DTYPE (Phase 78's table); of an
array, class A, as an I/O item's. A "no" cell is an error (a new message,
naming the built-in and the argument's kind). %LOC: `MOVAx`, or `MOVAB
G^name` for a procedure. A Hollerith constant inside a built-in is an
error, since every cell of its row is "no".]

**Procedure values.** In the VAX environment "a procedure value is the
address of the entry mask" (calling standard, 1.4), which `CALLS`
starts at. [An EXTERNAL name passed is `PUSHAB G^name`, as Phase 78's
intrinsics are (`Arg.Routine`). A dummy procedure is called with
`CALLS #n,@k(AP)`: the argument list's longword is the procedure value,
and CALLS's destination is its address operand, so the deferred mode
names the procedure. Passed on, it is `PUSHL k(AP)`. A CHARACTER
procedure name goes by descriptor (Table 6-1): the descriptor's length is
the function's length as the passing unit declares it, its pointer the
procedure value, which a called unit's CHARACTER\*(\*) dummy function
takes its length from. Its DTYPE: the calling standard's
DSC$K_DTYPE_ZEM (23, "procedure entry mask"), class S; ours, unconfirmed:
the manuals don't say.]

**Statement functions (6.2.1):** `f([p[,p]...]) = e`, defined in the
unit, before the first executable statement (Phase 76 recognizes them).
A reference associates the actual's values with the dummies and
evaluates e. Its type is its name's; dummies tell only number, order,
and type, and their names may be other entities elsewhere in the unit
(an array or COMMON name is an ordinary scalar dummy inside); actuals
agree in number, order, and type; a statement function may use one
defined before it, but not itself; references only in expressions.
[Expanded in line, as the plan says. What "associates the values"
means for a CHARACTER dummy of another length, and the result's
conversion, is question 5.]

**FUNCTIONs (6.2.2):** `[typ] FUNCTION nam[*m] [(p,...)]`, and
`CHARACTER[*n] FUNCTION nam[*n]`, n a constant or `(*)`. A CHARACTER\*(\*)
function "assumes the length declared for it in the program unit that
invokes it", which must be the same in every reference in that unit (the
manual's REPEAT examples). The value is the name's when RETURN or END is
reached. [The calling standard's 2.5: values of up to 32 bits come back
in R0, of 64 in R0:R1; "two separate 32-bit entities cannot be returned
in R0 and R1", which a COMPLEX\*8 is, but Decision 11 keeps COMPLEX\*8 in
R0:R1 as MTHRTL's F complex functions return it (Phase 77), so FORTRAN's
functions and MTHRTL's can be passed for each other. Everything else
shifts the argument list one place, and the first is where the value
goes: a REAL\*16 or COMPLEX\*16 function gets its caller's temporary by
reference, and a CHARACTER function a class S descriptor of the caller's
temporary, of the caller's length. Inside, a CHARACTER function's name is
its result's text through that descriptor, as a passed-length dummy's is,
so LEN of a CHARACTER\*(\*) function is the descriptor's length. The 2.5.1
"value on top of the stack" form, a descriptor with a 0 pointer, isn't
needed: the caller always knows the length.]

**SUBROUTINE (6.2.3):** dummies and `*`s. A subprogram may not refer to
itself, directly or not (RECURSIVE for the direct case; an indirect one
through another file is never seen).

**ENTRY (6.2.4):** `ENTRY nam [([p[,p]...])]` anywhere after the
FUNCTION or SUBROUTINE statement, not inside a block IF or a DO loop;
execution starts at the first executable statement after it. Each entry
has its own dummy list, of any order, number, and type; a dummy may be
used only in statements after the first SUBROUTINE, FUNCTION, or ENTRY
that names it; a dummy the current entry didn't give is undefined. Entry
names may be typed, given EXTERNAL and passed, but not used as dummies,
and not in executable statements before their ENTRY. `*` dummies only in
a subroutine's ENTRYs. In a function, every entry name is "associated"
with the function's name: defining one defines those of the same type
and makes the others undefined, so they share storage, and their types
must all be in one group (6.2.4.1): BYTE, INTEGER\*2, INTEGER\*4,
LOGICAL\*2, LOGICAL\*4, REAL\*4, REAL\*8, COMPLEX\*8; or COMPLEX\*16 and
REAL\*16; or CHARACTER, all of the function's length. [The design note
stands: each entry is a `.ENTRY` that copies its arguments' addresses
into the unit's static argument cells, then branches to the code after
its ENTRY statement; every dummy is reached through its cell
(`@SUB.$An`, relative deferred), which is why FORTRAN's static storage
and lack of recursion allow one body for all entries. Only units with
ENTRY statements use the cells; others keep `@n(AP)`. The entry names of
a function share one result place, as EQUIVALENCE would lay them out;
at a return, R0 (or the hidden argument) gets the entry's own name's
value in its type, so the code remembers which entry it came in by when
the types differ (a cell set at each entry, and a CASEL at the return).
Messages, new: ENTRY in a block, ENTRY with `*` in a function, an entry
of the wrong group or length, an entry name used before its ENTRY.]

## What was checked on govax's MACRO

A one-off program (not kept) of the forms below assembled with no message
on 2026-10-31: two `.ENTRY`s with the same mask, one branching into the
other's code (`BRW`); `MOVL 4(AP),cell` and dummies through cells
(`MOVL @SUB.$A2,R0`, `MOVAL @SUB.$A2[R0],R1`, `TSTF @SUB.$A2`);
`MOVAB label,cell` and `JMP @cell`; `CASEL R0,#1,#2` and `CASEL @cell,#1,#1`
with `.WORD label-table` entries, backward and forward; `CALLS #1,@4(AP)`,
`CALLS #0,@cell`, `PUSHL 4(AP)`, `PUSHAB G^name`. So MACRO needs nothing
found yet. A CASEL table's words reach 32K bytes either way, as every
`BRW` the generator writes already does; a routine that big is past both.

## What the compiler has to change

**`sema`:** statements for ASSIGN, computed and assigned GO TO,
arithmetic IF, DO WHILE, END DO and unlabeled loops (the construct stack's
`label` 0), PAUSE, RETURN i, ENTRY; jumps from each label list, the
assigned GO TO's targets, and the alternate return specifiers through
`labelRef`; a `*` dummy as a Param kind, counted by ARGCOUNT in its
place; a function result of any type (`Type.returned` goes); entries as
routines for FUNCTYPE and ARGCOUNT across units; the adjustable bounds'
expressions analyzed; a dummy procedure (`Class` from its use: Subroutine
when CALLed, Function when referenced) and an EXTERNAL argument; the
built-in functions with Table 6-1; a Hollerith actual; statement function
references.

**`ir`:** two terminators: a `Switch` (an index, a first value, the
targets, and where it falls through: CASEL), and an indirect `Jump`
through a Mem (with its possible targets); an expression for a label's
address; `Return.Value` for R0 in a subroutine; `Call` through a Mem (a
dummy procedure), and its hidden first argument; `Routine.Entries`
(each with its Params, its first block, and its result type) and
argument cells; adjustable arrays' cells and indexes; Arg's by-value and
by-descriptor forms for the built-ins.

**`codegen`:** CASEL and its table, `JMP @x`, `MOVAB label`,
`CALLS #n,@x`, several `.ENTRY`s with one mask, dummies through cells, and the
return's dispatch for typed entries.

**`internal/ftnrtl`:** `FTN$PAUSE([display])`, a new vector entry (GSMATCH
1,7).

## Decisions

Each is the survey's, accepted by the author (2026-10-31).

1. **Assigned GO TO is `JMP @v`** with or without a list, which only
   names the targets for the checks and the IR (5.6.3); the design note
   is corrected.
2. **Alternate returns:** the subroutine returns i in R0 (0 for a plain
   RETURN or END) and the caller dispatches with CASEL, falling through
   for anything out of range. Decision 11 as written.
3. **Function values:** R0 or R0:R1 for up to 8 bytes, COMPLEX\*8 included;
   a hidden first argument for REAL\*16 and COMPLEX\*16 (by reference) and
   CHARACTER (a class S descriptor, the caller's length). The same as
   MTHRTL's and FTNRTL's, so any of them can be passed for a FORTRAN
   function of its type.
4. **ENTRY through static argument cells,** only in units that have
   ENTRY statements; a function's entry names share one result place,
   and a return loads the value by the entry's type.
5. **Adjustable arrays' bounds are computed at each entry** into static
   cells, for the arrays that entry's dummies and COMMON define.
6. **A dummy procedure is called through its argument longword**
   (`CALLS #n,@k(AP)`), and a CHARACTER procedure travels by a ZEM
   descriptor carrying its length (unconfirmed DTYPE).
7. **The built-ins follow Table 6-1,** a "no" cell an error.
8. **Statement functions are expanded in line** at each reference, in
   lowering, from a typed reference sema builds (its arguments and the
   definition's analyzed expression).
9. **ASSIGN of a FORMAT label waits for Phase 80,** NOTIMPL meanwhile.
10. **The count of a longword DO loop near the type's limit** is left as
    the manual's formula computes it in the variable's type (unconfirmed).

## Questions for the author

1. **What ASSIGN stores.** (a) The label's address, `MOVAB s,v`, and GO
   TO v is `JMP @v`: fast, and a FORMAT label's address is what PRINT v
   needs; but an address needs 32 bits, so v must be INTEGER\*4, and an
   INTEGER\*2 v (any plain INTEGER under `/NOI4`) is an error. (b) A small
   number for each label the unit ASSIGNs, and GO TO v a CASEL over them:
   works for INTEGER\*2 too, and checks the value. The manual says only
   "an integer variable". **Recommended: (a),** with the INTEGER\*2 error
   (unconfirmed).
2. **The alternate return specifiers' places in the argument list.** (a)
   Left out: the list holds only the real arguments, so `CALL
   CHECK(A,B,*10,*20,C)` is `CALLS #3`, and the callee's dummies after a
   `*` move up; the caller keeps the labels. (b) Each `*s` a longword 0
   in its place. Neither manual says; the calling standard doesn't speak
   of it. **Recommended: (a)**, which is what CASEL on R0 needs and no
   more (unconfirmed); ARGCOUNT still counts the `*`s, so a call with the
   wrong number of specifiers is reported.
3. **PAUSE** as FTN$PAUSE: the display (or "FORTRAN PAUSE") to
   SYS$OUTPUT, then LIB$PAUSE, going on whatever it returns.
   **Recommended**, in place of the doc's prompt and line read, since
   LIB$PAUSE is VMS's own mechanism for exactly this (CONTINUE, EXIT,
   STOP at DCL's Ctrl/Y level).
4. **A character constant for a numeric dummy (6.1.1.4).** govax can't do
   the linker's part. (a) When the called unit is in the same file, the
   compiler knows the dummy's type and passes the constant's bytes by
   reference for a numeric dummy; otherwise by descriptor, Table 6-1's
   default. A difference listed in MANUAL.md. (b) Always by descriptor.
   **Recommended: (a).**
5. **Statement function arguments and result.** Each actual's value is
   assigned to a temporary of its dummy's type, as an assignment would
   (so a CHARACTER actual is cut or padded to the dummy's declared
   length), unless it is a constant or a variable of the same type and
   length, which is used where it is; e is then evaluated and converted
   to the function's type as an assignment would (FORTRAN-77's rule;
   the 1988 manual doesn't say). **Recommended** (unconfirmed).

## The author's answers (2026-10-31)

The author accepted Part A's decisions and each question's
recommendation:

1. **ASSIGN stores the label's address** (`MOVAB s,v`), and GO TO v is
   `JMP @v`; an INTEGER\*2 variable is an error.
2. **Alternate return specifiers are left out of the argument list;**
   ARGCOUNT still counts them.
3. **PAUSE is FTN$PAUSE:** the display to SYS$OUTPUT, then LIB$PAUSE.
4. **A character constant for a numeric dummy** goes by reference when
   the called unit is in the same file and its dummy is numeric, by
   descriptor otherwise; a difference listed in MANUAL.md.
5. **Statement function arguments** are assigned to temporaries of the
   dummies' types (but for constants and variables of the same type and
   length), and the result converted to the function's type.

## Subtasks (provisional)

1. **Survey:** chapters 5 and 6, the calling standard's argument list,
   Table 6-1. Done.
2. **GO TO in every form, ASSIGN, arithmetic IF.** (The IR's Switch and
   indirect jump; ASSIGN of a FORMAT label stays NOTIMPL, Decision 9.) Done.
3. **DO in every form**, DO WHILE, END DO, extended range. (Labeled DO
   of every INTEGER and REAL type is done already; this is the loops
   without labels, DO WHILE, END DO's messages, and the extended range
   run end to end.) Done.
4. **STOP, PAUSE, RETURN, END.** (FTN$PAUSE in FTNRTL; RETURN i's R0
   comes with subtask 7.) Done.
5. **Function results of every type**, CHARACTER and COMPLEX functions. Done.
6. **ENTRY.** Done.
7. **Alternate returns.** Done.
8. **Adjustable and assumed-size arrays**, passed-length character arrays. Done.
9. **Procedure arguments, null arguments, %VAL/%REF/%DESCR/%LOC.** (With
   Hollerith constants as arguments, and answer 4.) Done.
10. **Statement functions.**
11. **End to end**, then **close-out** (FORTRAN-INTERNALS.md's chapter
    on calls and control flow).

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-31, subtask 1 (the survey):** Part A. Chapters 5 and 6, Table
  6-1, and section 3.5 (ASSIGN) read from the OCR text; the calling
  standard's chapter 2 (register use, the argument list, function value
  returns) from its PDF; the HP Fortran manual for PAUSE, STOP, and
  RETURN. Programs of every form compiled with `govax fortran /NOMACRO`
  to find what earlier phases already do (null arguments, labeled DO of
  every type, the extended range, passed-length arrays, assumed-size
  arrays) and where each NOTIMPL is reported; a MACRO program of the
  instruction forms the phase needs assembled cleanly. The design note on
  the assigned GO TO is corrected (its list doesn't affect execution),
  and the open question on REAL DO counts was settled by Phase 78. Ten
  decisions and five questions for the author: what ASSIGN stores, the
  alternate return specifiers' places in the argument list, PAUSE through
  LIB$PAUSE, character constants for numeric dummies, and statement
  functions' argument association.
- **2026-10-31, subtask 2 (GO TO in every form, ASSIGN, the arithmetic
  IF):** sema (`sema/jumps.go`): `ComputedGoto` (the index any arithmetic
  value, converted to INTEGER\*4), `AssignLabel` and `AssignedGoto` (the
  variable an INTEGER\*4 scalar, ASSIGNVAR otherwise; the unit's
  `Assigned` labels, where a GO TO v without a list may go), and `ArithIf`
  (INTEGER or REAL of any length, ARITHIF otherwise). Every label of a
  list is a jump checked as GO TO's is (`labels.go`'s `jumps`); an
  ASSIGN's is a reference of its own (`labelRef.assign`), which may name a
  FORMAT statement (NOTIMPL until Phase 80, Decision 9) and isn't checked
  for blocks, since only the GO TO jumps; an assigned GO TO without a list
  is checked nowhere. The arithmetic IF joins DOTERM's list. The IR has
  three terminators, `Switch` (CASEL), `Sign` (a TST and its branches; two
  or three targets may be one block), and `IndirectJump` (`JMP @v`, with
  the blocks it may reach), and `LabelAddr`, which ASSIGN stores; a
  constant index or value is a plain jump. Codegen (`routine.go`'s
  `caseTable`, `sign`, `indirectJump`): CASEL with a `.WORD` table of
  distances from it; a TST left out when the value's instruction set the
  codes (`IF (X - 1.0)` is SUBF3 then the branches), the case whose
  target comes next written as nothing; `JMP @K`, or for a dummy, whose
  `@n(AP)` can't be deferred again, `MOVL` to a register and `JMP (R0)`;
  `MOVAB label,v`. Tests: `jumps.for` golden files in `sema`, `ir`, and
  `codegen`; sema error cases (ARITHIF, ASSIGNVAR, NEEDSUBSCRIPT, UNDEFLBL,
  NOTEXEC, DOVARSET, INTOBLOCK from each kind of list, CHARNUMBER, DOTERM);
  `testdata/fortran/jumps.for` end to end, in both paging modes. MANUAL.md's
  14.2 and 6.5 list the statements.
- **2026-10-31, subtask 3 (DO in every form, DO WHILE, END DO, the
  extended range):** sema (`sema/stmt.go`): a loop without a label opens
  with label 0 and closes at END DO (`endDo`), which ends the innermost
  loop: one without a label, or a labeled one whose label the END DO has
  (5.4's optional form, which `Do.EndDo` records); an IF block opened
  inside the loop and not ended is DONEST, as for END IF. New messages:
  NOENDDO (the unit's END reaches a loop without a label), NOLOOP (END DO
  with no loop open), ENDDOLABEL (END DO inside a loop that ends at a
  label it doesn't have), and SHAREDENDDO (a second loop ending at a
  labeled END DO: 5.3.1.2 lets nested loops share a labeled terminal
  statement, not an END DO). `DoWhile` is a loop of its own (`construct`'s
  `while`; its condition through `condition`, LOGICAL or INTEGER as an
  IF's); DO WHILE joins DOTERM's list. A DO or DO WHILE the parser
  rejected still opens a quiet loop when no label follows DO
  (`labeledDo`), so its END DO isn't reported too. The IR: `doWhile` (a
  test block at the top, the body, a jump back), and both loops' ends in
  `loopEnd` (the END DO's line, and a labeled END DO's label block where
  the next pass starts, so a jump to it goes on to the next pass); a
  loop's notes and its cells' `Loop` are its name, "DO 10" or "DO (line
  12)" (`loopName`). Nothing new in codegen. Tests: `loops.for` golden
  files in `sema`, `ir`, and `codegen`; sema error cases for each message,
  the shared terminal of a DO and a DO WHILE, a jump to END DO, and the
  rejected statements; `testdata/fortran/loops.for` end to end in both
  paging modes, with the manual's examples (NUMBER 41 after `DO NUMBER =
  5,40,4`, J -4 after `DO J = 50,-2,-2`, the LINE scan) and the extended
  range, out and back in, of a labeled loop and of one without a label.
  MANUAL.md's 14.2 and 6.5 list the loops.
- **2026-10-31, subtask 4 (STOP, PAUSE, RETURN, END):** PAUSE is the
  parser's `StopStmt` already, so sema's `Stop` gets `Pause`, and a display
  is STOP's, digits as written (`PAUSE 00123`). The IR's `stop` calls
  `FTN$PAUSE` and goes on to the next statement, where FTN$STOP's call ends
  the block. FTNRTL: `FTN$PAUSE([display])` in `ftn_control.mar`, the
  vector's entry at 272, GSMATCH 1,7, as question 3 settled: the display,
  or "FORTRAN PAUSE", to SYS$OUTPUT, then LIB$PAUSE, going on whatever it
  returns. STOP, RETURN, and END needed no new code, and now have tests;
  RETURN i stays NOTIMPL until subtask 7. Running two pauses found a bug
  in LIB$PAUSE's status (`dclshell/dclpause.go`): CONTINUE at one pause and
  EXIT or STOP at a later one left `pauseContinued` set, so `$STATUS`
  didn't become SS$_NORMAL. Now they clear it (unconfirmed: the lib57
  probe's images pause once). Unchanged: at level 0, a program continued
  at its last pause leaves `$STATUS` as it was, whatever it returns, VMS
  V7.1's rule for an image that called LIB$PAUSE. Tests: `control.for`
  golden files in `sema`, `ir`, and `codegen`, with PAUSE and STOP as loops'
  terminal statements; `TestFortranPause` in both paging modes (CONTINUE, a
  command at the pause, EXIT at the second pause, STOP at the first, end of
  input); `testdata/fortran/stops.for` and `stopdo.for` end to end. A
  terminal STOP leaves the loop's step code after it, unreachable; Phase
  84's flow analysis can drop it. MANUAL.md's 14.2 and FTNRTL table
  list PAUSE, and its differences DEBUG at a pause.
- **2026-10-31, subtask 5 (function results of every type):** a CHARACTER,
  REAL\*16, or COMPLEX\*16 function's value goes through a hidden first
  argument (Decision 3; `sema.Type.HiddenResult`, which replaces `returned`
  and its NOTIMPLs). Inside the function, the name has no `$LOCAL` place
  (`layout`): it is an `ir.Param`, argument 1, its `Sym` the result, and the
  dummy arguments follow it, so everything dummies already do serves it: a
  CHARACTER name's text is through its descriptor, `LEN` of a CHARACTER\*(\*)
  function is the descriptor's length, a substring or a CALL's argument of
  the name works, and a passed-length name is passed on with the descriptor
  it came with. `Routine.Result` is None for such a function, and its Return
  sets no register. The caller (`callFunction`) passes the address of a
  temporary, or for CHARACTER a class S descriptor of a text temporary of
  the length the unit declares (`chars`' `FuncRef`), then uses that
  temporary. `H = F(H)` must not pass H itself, since the function writes
  its name as it runs and could change its own argument, so `assign` calls
  straight into the target only for a value in registers;
  `assignComplex` moves a COMPLEX\*16 function's value whole. `LEN(F(X))`
  makes no call (`charLen`), as `LEN` of a concatenation builds nothing:
  FORTRAN-77's rule, and ours, unconfirmed for VAX FORTRAN. Sema:
  FUNCTYPE lets a CHARACTER\*(\*) function be referenced at any CHARACTER
  length (6.2.2.2); a new message, TWOLENGTHS, in the parser, for a length
  both after CHARACTER and after the name, which 6.2.2.2 forbids; and
  PASSEDLEN now covers a function referenced under the name `CHARACTER*(*)
  F` that isn't a dummy, since it takes its length from the unit referencing
  it and this unit gives none (ours, unconfirmed: the manual doesn't speak of
  it; a dummy function is subtask 9's). The storage map and the symbols'
  comments say "hidden argument". The manual's REPEAT example declares `CARL`
  and uses `CARG`, a misprint; `funcs.for` declares `CARG`. Tests: `funcs.for`
  golden files in `sema`, `ir`, and `codegen`; sema error cases;
  `testdata/fortran/funcs.for` and `funcsg.for` end to end in both paging
  modes, REPEAT at two lengths in two units as the manual has it. MANUAL.md's
  14.2 lists the functions.
- **2026-10-31, subtask 6 (ENTRY):** Decision 4 as written. Sema
  (`sema/entry.go`): `entries` reads the ENTRY statements after the header and
  before the declarations, since a dummy's or an entry name's declaration may
  come before its ENTRY; each is an `Entry` (`Unit.Entries`) and, in the body,
  an `EntryPoint`. A dummy in several lists is one symbol, so `Symbol.Dummy` is
  now its place among `Unit.Dummies`, every dummy of the unit once; `Unit.Params`
  stays the header's. In a function an entry name is a `Result` name, and
  `placeResults` puts them all at `$LOCAL+0`, as long as the longest, marked
  `Equivalenced` so the code generator and `Check` allow the overlap; in a
  subroutine it is a `UnitName`, so calling it inside its unit is RECURSIVE, as
  for the unit's own name. New messages: ENTRYUNIT (a main program), ENTRYBLOCK,
  ENTRYRETURN (`*` in a function's ENTRY), ENTRYLATE (a dummy or entry name in an
  executable statement before the ENTRY that first names it, found in `lookup`
  through `Symbol.laterEntry` during the statement walk), and ENTRYTYPE (6.2.4.1's
  groups, and CHARACTER's one length, `checkEntryTypes`); DUPUNIT now covers entry
  points, which `unitNames` collects with the units, so FUNCTYPE and ARGCOUNT check
  references from other units against the entry point's own type and list
  (`fileUnit.params`, `result`). A `*` in a subroutine's ENTRY stays NOTIMPL until
  subtask 7. The IR (`ir/lowerentry.go`): `Routine.Entries`, the header's first;
  each dummy, and a hidden argument, has an `ArgCell` (`$A1`, ...; `Param.Through`)
  only in a unit with ENTRY statements, so other units' code doesn't change. Each
  ENTRY begins a block, which code above it falls into. If a function's names
  have different machine types there is a `SelectorCell` (`$E`), and a return is a
  `Switch` on it to blocks after the END, one a type, each a `Return` of that
  name; names of one type return as without ENTRY. Codegen: `argumentPlace` is the
  one change to how a dummy is reached (`@SUB.$A1` for `@SUB.A(AP)`, and
  `PUSHL SUB.$A1` for passing a descriptor on); `writeEntry` writes each other
  `.ENTRY` after the routine's code, with the one mask, then its copies and a
  `BRW`; the header's `.ENTRY` copies too. Ours, unconfirmed: an entry point
  whose list omits a dummy leaves its cell as it was, which section 6.2.4 calls
  undefined; an EXTERNAL statement can't name an entry point in its own unit (the
  manual's rule is about passing one, and other units can; passing is subtask 9).
  The manual's hyperbolic example uses statement functions (subtask 10), so
  `entries.for` writes them out. Tests: `entry.for` golden files in `sema`, `ir`,
  and `codegen`; sema error cases for each message; `testdata/fortran/entries.for`
  end to end in both paging modes, and `testdata/fortran/modules/entries`, entry
  points called from another file, linked in both orders. MANUAL.md's 14.2 and
  the README list them.
- **2026-10-31, subtask 7 (alternate returns):** Decision 2 and answer 2 as
  written. Sema (`sema/altreturn.go`): a `*` in a dummy list is a place in
  `Unit.AltPlaces` or `Entry.AltPlaces`, not a dummy, so `Params` and every
  argument's position are what the argument list holds, and ENTRY's cells needed
  nothing new. `altReturns` divides a CALL's list; the specifiers' labels are
  `Call.AltReturns`, each a jump checked as GO TO's is (UNDEFLBL, NOTEXEC,
  INTOBLOCK, SHAREDTERM). `Return.Alt` is RETURN i's value, any arithmetic value
  converted to INTEGER\*4 (`CHARNUMBER` for a CHARACTER one). ARGCOUNT counts the
  `*`s; new messages: ALTPLACE (the same number, but in other places, which 6.1.1.5's
  "corresponding positions" forbids and a file holding both units can see; ours, as
  ARGCOUNT is), and FUNCRETURN (a `*` in a FUNCTION statement, RETURN i in a function;
  section 5.9). The IR: `lowerer.call` follows the Call with a `Switch` on `CallValue`,
  a new expression for the R0 a call just left, and the next statement in a block of
  its own. A statement with a dynamic string to free (`S // T` among the arguments)
  frees it before any terminator, which uses R0, so there the Call's `Result` is a
  temporary and the Switch is on that. A subroutine with a `*` in any of its lists
  returns 0 from a plain RETURN or END (`returnValue`; `CLRL R0`), and RETURN i returns
  i, unchecked; a subroutine with no `*`s sets R0 only for a RETURN i, which no caller
  looks at. Ours, unconfirmed: RETURN i is allowed in a subroutine with no `*`s (5.9
  says an i past the number of alternate returns goes on normally, so it does); the
  `*`'s absence from the argument list (answer 2) means a program calling across a
  mismatched list gets the wrong longwords, as with any argument mismatch. A dummy
  procedure called with specifiers comes with subtask 9. Codegen: `CallValue` is the
  operand R0, and the note on CASEL and the RET says what the value is. Tests:
  `altreturn.for` golden files in `sema`, `ir`, and `codegen`; sema error cases for
  each message and for each jump check; `testdata/fortran/altreturns.for` end to end in
  both paging modes (the manual's CHECK, with its `Z` computed and its third list
  argument reached past the `*`s), and `testdata/fortran/modules/altreturns`, linked in
  both orders. MANUAL.md's 14.2 lists them, with the differences.
- **2026-10-31, subtask 8 (adjustable and assumed-size arrays, passed-length
  character arrays):** Decision 5, with one change to the survey's cells: an
  adjustable array's static cells are one class A descriptor (`$B1`), not
  separate lower bound, multiplier, offset, and size cells, since a whole array
  written or passed needs that descriptor anyway and it holds all an element's
  address needs. An element is at A0 plus the subscripts multiplied out from the
  last by the descriptor's extents (`ir/lowerbounds.go`; its comment works
  `X(-I/2:I/2,J)` through), one multiplication a dimension, so `Mem.Ptr` is A0
  and the first element's offset, the survey's "offset" cell, is built into A0. A
  dimension with constant bounds uses its constants, and they start in the cell;
  only what isn't constant is stored (`Bound.Set`). Sema: `checkArrays` analyzes
  each bound once every name's type is known (`boundValues`: `Dim.LowerValue`,
  `UpperValue`, converted to INTEGER) and notes the dummies in them
  (`Symbol.BoundDummies`). Each entry point computes the arrays its own list
  defines, per section 6.1.1.1: the array and every dummy in its bounds in the
  list (`defines`). The header's code is in its first block; an ENTRY's goes in a
  block after the routine's statements, which `Entry.First` names and which goes on
  to the statements, since falling into an ENTRY statement isn't entering. Any
  other array's cell keeps what it held, section 6.1.1.1's "undefined".
  A passed-length array stores its element length too, and an assumed-size one
  (`A(1:N,1:*)`) no size: it is never used as a descriptor, as sema refuses such
  an array in an I/O list (NOSIZE) and `charArg` passes it by its first element.
  SIZEOF of an adjustable array is the descriptor's size, `ArraySizeOp`, not a
  constant; of a passed-length one, BADARG, whose wording now says CHARACTER\*(\*)
  and not "assumed-size" for one that isn't. The storage map shows a computed bound
  as `?`. Plain assumed-size and passed-length arrays needed nothing new, and have
  tests now. Unchanged and ours, unconfirmed: extents aren't checked at run
  time, so a negative one (`A(N)` with N -1) gives a nonsense size, as the manual
  makes it the program's mistake; N 0 is an array with no elements. Tests: `adjust.for` golden
  files in `sema`, `ir`, and `codegen`; sema cases; `testdata/fortran/adjust.for` end
  to end in both paging modes, its output worked out by hand. MANUAL.md's 14.2 lists
  them.
- **2026-10-31, subtask 9 (procedure arguments, null arguments, Hollerith
  constants as arguments, %VAL, %REF, %DESCR, %LOC):** Decisions 6 and 7 and answer
  4 as written. Sema (`sema/arguments.go`): `arg` is one actual argument, which
  `Arg.Mode` says how to pass when that isn't its type's default. A name in an
  EXTERNAL statement is a `Procedure` (as Phase 78's intrinsics are), and a dummy
  procedure passed on one with `Dummy` set: `Symbol.DummyProcedure`, a dummy in an
  EXTERNAL statement or already called, takes its `Class` from its use as any routine
  does, Subroutine or Function, so a dummy both called and referenced is BADUSE. Its
  name is the unit's own, so `checkReferences` looks for no routine of that name.
  `notAllowed` is Table 6-1, a "no" cell BUILTINARG; BUILTINPLACE is %VAL, %REF, or
  %DESCR anywhere but an argument list, an intrinsic function's among them, or
  inside another; %LOC is `Loc`, an expression, any other operand LOCARG. A
  PARAMETER with %LOC is "not a constant", no longer NOTIMPL. A Hollerith constant
  is a CHARACTER `Const` with `Hollerith` set, `ByRef`; `constantArguments` makes a
  character constant given to a numeric dummy of a routine in the file the same
  (answer 4), unless %DESCR said otherwise, and reports HOLLCHAR for a Hollerith
  constant given to a CHARACTER dummy. Elsewhere in expressions a Hollerith constant
  stays NOTIMPL, Phase 82's. An EXTERNAL name no reference makes a function needs no
  type under IMPLICIT NONE. The IR (`ir/lowerargs.go`, with `args` and `callFunction`
  moved there): `Call.Through` is the place a dummy procedure is, `@F(AP)`, and for
  a CHARACTER one `@4(R0)`, the address its descriptor holds, the same place a
  CHARACTER dummy's text is (`dummyText`), so codegen needed only `CALLS #n,m`;
  through ENTRY's cells, `@SUB.$A1`. A CHARACTER procedure's descriptor
  (`Descriptor.Routine`, DTYPE ZEM, class S, Decision 6) is a datum,
  `.ADDRESS F`, and `RoutineAddr` is %LOC's `MOVAB G^F`. A CHARACTER\*(\*) dummy
  function's value goes in a dynamic string of its descriptor's length (`chars`).
  **Changed from Phase 78:** an intrinsic CHARACTER function's name, CHAR's, now goes
  by descriptor as Table 6-1 says, not by address, or a FORTRAN dummy couldn't call
  it; `TestFortranPassedIntrinsics`'s MACRO routine has a `CHCALL` for it. Ours,
  unconfirmed: a numeric procedure's %DESCR is ZEM with the length 0; an EXTERNAL
  name typed CHARACTER is taken for a CHARACTER function by caller and dummy alike,
  even if it is a subroutine (only IMPLICIT could make one so); %DESCR of an
  assumed-size number array is its first element's class S descriptor, as `charArg`
  does for CHARACTER; %LOC of a CHARACTER value is its text's address; a dummy passed
  on before any call and without EXTERNAL is a variable, passed the same longword, and
  a later call of it an error; a procedure's arguments and type aren't checked
  against the file's routine when it is only passed. Null arguments needed nothing
  but tests. Tests: `procargs.for` golden files in `sema`, `ir`, and `codegen`; sema
  error cases for each message and each "no" cell; `testdata/fortran/procs.for` end to
  end in both paging modes, the routines reading each form back (%VAL's value where an
  address would be, by %LOC; a descriptor's length and address, through a CHARACTER\*(\*)
  dummy), and `testdata/fortran/modules/procs`, across files in both link orders.
  MANUAL.md's 14.2 lists them, with the differences.
