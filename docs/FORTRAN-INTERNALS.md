# Inside govax's FORTRAN

This is a guided tour of govax's FORTRAN compiler and its run-time
library, FTNRTL, for someone who wants to know how they work, or to
write a compiler of their own for the VAX under VMS. It grows a phase at
a time with the compiler (`docs/PHASE-74 - FORTRAN plan.md` lays out
the program; its Part B holds the decisions this tour refers to). The
user's side is the manual's chapter 14, "Programming in FORTRAN".

So far (Phases 74 and 75) every pass is in place for a first subset of
the language: main programs, subroutines, and functions; INTEGER, REAL,
and LOGICAL scalars and arrays; assignment and expressions; IF, block
IF, DO, GO TO, CALL, RETURN, and STOP; and `PRINT *` and `WRITE (*,*)`.
The tour follows one small program through every pass (section 1), then
takes the passes in order, then FTNRTL, then how the compiler is tested.
Each pass's package also has a `doc.go` that teaches it in more detail;
this tour is the story across them.

## 1. The shape of the compiler

A compiler is a series of passes, each reading what the one before it
produced and handing on something a step closer to machine code:

```
 AVERAGE.FOR
     |
 source ---------- lines into statements: columns, continuations,
     |             comments, INCLUDE                     (Phase 74)
 lex ------------- each statement's kind, and its tokens (Phase 75)
 parse ----------- tokens into a syntax tree             (Phase 75)
 sema ------------ names, types, blocks, labels, storage (Phases 75-76)
 ir -------------- the tree lowered to basic blocks      (Phase 75)
 opt ------------- optional improvements                 (Phase 84)
 codegen --------- VAX instructions, as MACRO-32 text    (Phases 74-75)
     |
 AVERAGE.MAR --MACRO--> AVERAGE.OBJ --LINK (FTNRTL)--> AVERAGE.EXE
```

Each pass is a Go package under `internal/fortran`, with a `doc.go`
that teaches it. The compiler is plain Go: it knows nothing of the
emulated machine, RMS, or the console. The FORTRAN command
(`internal/dclcmd/fortran.go`) reads the source from the host or a
Files-11 volume, gives the compiler a way to read INCLUDE files, and
writes the MACRO file the compiler returns. That split is what lets
every pass be tested on sources held in memory.

Why MACRO-32 and not object modules (Decision 3)? govax's MACRO already
writes object modules byte for byte as VAX MACRO does, with traceback
and debugger records; a compiler that writes MACRO gets all of that
for nothing, and its output can be read, assembled on a real VAX, and
debugged at the instruction level with the FORTRAN beside it.

### The program this tour follows

```
      PROGRAM AVERAGE
      REAL V(4), MEAN
      DO 10 I = 1, 4
   10 V(I) = I * 1.5
      PRINT *, 'Mean:', MEAN(V, 4)
      END
C
      REAL FUNCTION MEAN(A, N)
      REAL A(*)
      MEAN = 0
      DO 10 I = 1, N
   10 MEAN = MEAN + A(I)
      MEAN = MEAN / N
      END
```

It fills an array with 1.5, 3.0, 4.5, and 6.0 and prints their mean,
`Mean:   3.750000`. Small as it is, every pass has work to do: an
implicitly typed variable (I), an INTEGER multiplied by a REAL, a DO
loop with a constant count and one with a count known only at run time,
an array of the caller's passed to a function that doesn't know its
size, a function reference inside a PRINT list, and a function's value
returned. Each chapter below shows what its pass makes of it; the dumps
are the passes' own `Dump` output.

## 2. The driver: `internal/fortran`

`fortran.Compile(Options, File) Result` runs the passes in order. Its
`Options` are the command's qualifiers (and the version, time, and
command line for the MACRO file's header); `File` is the source's
specification, its name, and its lines; `Result` holds the MACRO file's
lines (nil after an error) and the messages.

Every pass reports to one `diag.List`, and the driver decides what runs
next from it. The source pass's errors stop everything, since the
statements themselves may be wrong. The lexer, parser, and semantic
analysis then run together, even after the parser's errors: a statement
with an error stays in the tree, with its label, so one compilation
reports every mistake it can, without a cascade of messages about
the first. Lowering and code generation run only on a program with no
errors at all. The MACRO module is named for the file's first program
unit (`AVERAGE`; an unnamed main program is `file$MAIN`, section 4.12
of the language manual), or for the file when it has none.

## 3. Messages: `internal/fortran/diag`

A compiler's messages are its user interface, so every pass reports
through one package, and every message is an entry in one table
(`messages.go`): an identifier, a severity, and a format. A pass never
writes message text of its own; it names an identifier and gives the
arguments. That keeps the wording consistent and lets a test check
every message (a VMS-style identifier, text starting in lowercase).

A message names the file and line, quotes the line, and points at the
column, copying the line's tabs into the pointer so the caret lines up
however a terminal shows tabs:

```
%FORT-E-BADLABEL, statement label field holds "X", which is not a label of 1 to 5 digits
  at line 2 of DUA1:[WORK]BAD.FOR;1
    X   Y = 2
  --^
```

The compiler goes on after an error, to report as many as it can in one
run; a fatal error (an INCLUDE file that can't be read) stops it. After
an error or a fatal error there's no MACRO file, as MACRO writes no
object after an error.

## 4. The first pass: source forms (`internal/fortran/source`)

Most languages treat a source file as a stream of characters in which a
line break is one more blank. FORTRAN was designed for punched cards,
one statement to a card, and its lines still have fields in fixed
columns:

```
columns  1-5    statement label
column   6      continuation indicator (not blank or 0: this line
                continues the statement before it)
columns  7-72   the statement (to 132 with /EXTEND_SOURCE)
columns 73-80   sequence number, ignored
```

So "where does a statement start and end?" is a question about columns,
not words, and it's answered before any tokenizing. `source.Read`
answers it and hands on a `Program`: every line it read, in order
(comments, included files' lines, and all, since the code generator
copies them into the MACRO file), and the statements.

### Classifying a line

`lines.go`'s `split` looks at a line's first columns and decides what it
is: a comment (C, `*`, or `!` in column 1, or blank), a debugging line
(D in column 1: a comment, or with `/D_LINES` a statement whose D reads
as a blank), a compiler directive (`CDEC$` in columns 1 to 5), or a line
of a statement, initial or continuation, with its label field and
statement field.

Tab-format lines are the one wrinkle. Typing at a terminal, nobody
counts to column 7, so VAX FORTRAN lets a tab end the label field: what
precedes the tab is the label, a digit 1 to 9 right after it is the
continuation indicator, and anything else starts the statement. The tab
stands for the columns up to column 7, so the statement field still
ends at column 72. (That last point is our reading, and fits DUNGEON,
whose tab-format lines run past 72 characters only in comments.)

### Putting a statement together

`read.go` takes a file's classified lines and groups them: an initial
line, then its continuation lines, with any comment lines among them.
The statement's text is the statement fields joined end to end. Blanks
are kept: FORTRAN ignores blanks outside character constants, but which
blanks those are is the lexer's question, and the lexer needs the text
as written to answer it.

Each statement keeps a map from its text back to the source (`pieces`,
one per line that gave it text): `Statement.Position(offset)` turns an
offset in the text into a file, line, and column. Every later pass
reports problems through it, so a message about the 40th character of a
statement continued over three lines points at the right line and
column.

### End-of-line comments, and constants that hide them

A `!` starts a comment to the end of the line, except inside a
character constant (`'WHAT!'`) or a Hollerith constant (`5HHELLO!`), and
either may continue onto the next line. So `comment.go`'s scanner
follows constants through the statement, a line at a time, keeping its
state between lines. A character constant is easy: each apostrophe
switches between inside and outside (a doubled apostrophe switches out
and back in, which comes to the same thing).

A Hollerith constant is a count, an H, and that many characters, from
FORTRAN's early days, before character constants. Telling a count from
other digits followed by an H needs context, because blanks don't count
and keywords aren't reserved: in `REAL*8 HVAL` the 8 is a length, and
in `DO 10 H = 1, 2` the 10 is a label. A Hollerith constant can only be
where a value can be, so the scanner takes digits as a count only after
`( , / = .`, or after `*` that follows a digit (DATA's repeat count, as
in `2*4HABCD`). The lexer will meet the same question and can use the
same rule.

VAX FORTRAN uses the apostrophe once more: `READ (1'10) X` reads record
10 of unit 1. An apostrophe after a name or number inside parentheses
is that separator, since a character constant never follows a name
there.

### INCLUDE

`INCLUDE 'file'` changes where lines come from, so it's handled here,
not by the parser: `include.go` recognizes it by its whole shape (the
word, blanks and case ignored, then one character constant and nothing
else; no other statement starts with a name followed by an apostrophe),
asks the `Opener` for the file, and reads its lines in the statement's
place, one include depth deeper. The `Opener` is an interface the
FORTRAN command implements (finding files relative to the including
file, as MACRO's `.INCLUDE` does) and tests implement with maps; this
package never touches a file system.


## 5. The second pass: lexing (`internal/fortran/lex`)

In most languages the lexer runs first and knows no grammar: it reads
characters and hands the parser a token whenever a word ends. That works
because blanks separate words and keywords are reserved. FORTRAN has
neither rule. Blanks mean nothing outside character constants (`GO TO`,
`GOTO`, and `G O T O` are one statement), and a keyword is an ordinary
name anywhere else: a program may have variables named DO, IF, and END.
The textbook case is the manual's own (5.3.1):

```
      DO 10 I = 1,5        a DO loop ending at label 10
      DO 10 I = 1.5        the assignment DO10I = 1.5
```

The two differ in one character, and it comes after everything a
conventional lexer would have had to decide. So FORTRAN's lexer works
the other way round, in three steps (Decision 5):

1. **Squeeze** (`squeeze.go`): take out every blank outside character
   and Hollerith constants, and make letters capitals. `DO 10 I = 1, 4`
   becomes `DO10I=1,4`. Each character remembers its offset in the
   statement, so a message can still point at its column.
2. **Classify** (`lex.go`): decide the statement's kind from the whole
   of it, looking only at parenthesis level 0. A statement starting
   `IF(` is an IF unless `=` follows the condition's closing parenthesis
   (then it assigns an element of an array named IF); an `=` at level 0
   makes an assignment, unless the statement starts with DO and a comma
   at level 0 follows the `=` (that comma is the whole difference between
   the two statements above); anything else is a keyword statement,
   whose keyword is the longest the text starts with (`ENDIF` before
   `END`). `kinds.go`'s table names every VAX FORTRAN statement, so a
   statement the compiler can't compile yet is still known by name.
3. **Tokenize** (`tokenize.go`): with the kind known, split the text
   after the keyword into names, numbers, constants, and operators. The
   kind helps here too: in `DO10E1=1,5` the tokenizer knows a label comes
   first, and takes `10` alone rather than the REAL constant `10E1`.

One case needs more than a statement's text: `REAL FUNCTION MEAN(A,N)`
and `REAL FUNCTIONMEAN(A,N)` squeeze alike, and the second declares an
array. Since a FUNCTION statement can only begin a program unit, the
lexer takes such text for one only there, knowing where units begin
because END ends them.

AVERAGE's statements come out like this (`lex.Dump`: the label, the
kind, the tokens):

```
PROGRAM: AVERAGE
REAL: V ( 4 ) , MEAN
DO: 10 I = 1 , 4
10 assignment: V ( I ) = I * 1.5
PRINT: * , 'Mean:' , MEAN ( V , 4 )
END:
FUNCTION (REAL): FUNCTION MEAN ( A , N )
REAL: A ( * )
assignment: MEAN = 0
DO: 10 I = 1 , N
10 assignment: MEAN = MEAN + A ( I )
assignment: MEAN = MEAN / N
END:
```

The lexer reports only what it can see in the characters: a character
constant never closed, a Hollerith constant cut short, a character
FORTRAN doesn't use, and a statement it can't classify.

## 6. The third pass: the syntax tree and parsing (`ast`, `parse`)

With each statement's kind decided, parsing FORTRAN is ordinary. The
parser (Decision 6) is **recursive descent**: a table (`parse.go`'s
`parsers`) maps each kind to a function that reads the tokens in the
order its statement's grammar gives them, so each function reads like
the grammar rule it implements, and an error can say exactly what was
expected where (`expected '=', but found ','`). A kind with no function
is a statement the compiler knows but can't compile yet, reported by
name: `%FORT-E-NOTIMPL, the READ statement is not implemented yet`.

**Expressions** are parsed by **precedence climbing** (`expr.go`): one
function, `binary(min)`, reads an operand and then every operator that
binds at least as tightly as `min`, reading each right operand with a
higher minimum. FORTRAN's nine levels, from `.EQV.` to `**`, are then a
table rather than nine functions. Two rules need care: `**` groups right
to left, and VAX FORTRAN lets a sign follow an operator (`A**-B*C` is
`A**(-(B*C))`). Parentheses stay in the tree, since in FORTRAN they mean
more than grouping: `CALL S((X))` passes a copy of X.

**The tree** (`internal/fortran/ast`) records what each statement says,
not what it means. A name has no type yet, and `MEAN(V, 4)` is an
`Apply`, which may be an array element or a function reference: only the
declarations can say, and those are the next pass's to read. A file is a
list of program units, and each unit's statements are a **flat list**:
the parser doesn't match IF with END IF or a DO with its terminal
statement. Keeping block structure out of the parser keeps it a function
per statement, and puts every rule about blocks and labels in one place,
the next pass.

**Errors** recover at the statement: a parsing function that finds a
mistake reports it and panics with a private value, which the statement
loop recovers, leaving a `BadStmt` that keeps the statement's label.
Since a FORTRAN statement is never more than its own tokens, one bad
statement can't confuse the parser about the next.

AVERAGE's tree, as `ast.Dump` writes it (expressions as prefix lists,
the operator first):

```
== main program AVERAGE
1           PROGRAM AVERAGE
2           REAL V(4), MEAN
3           DO 10 I = 1, 4
4     10    V(I) = (* I 1.5)
5           PRINT * 'Mean:' MEAN(V, 4)
6           END
== FUNCTION MEAN
8           REAL FUNCTION MEAN(A, N)
9           REAL A(*)
10          MEAN = 0
11          DO 10 I = 1, N
12    10    MEAN = (+ MEAN A(I))
13          MEAN = (/ MEAN N)
14          END
```

## 7. The fourth pass: semantic analysis (`internal/fortran/sema`)

Semantic analysis works out what the program means, and checks every
rule the grammar can't express (Decision 7). It takes each program unit
in two walks over its statements.

**The first walk reads the declarations** and builds the unit's symbol
table. Every name gets a `Symbol` with its type, declared or implicit
from its first letter (I to N are INTEGER, the rest REAL: section
2.2.2.2), its dimensions if it is an array, and its position if it is a
dummy argument. What a name *is* comes from how it is used, since
FORTRAN declares little of that: `REAL MEAN` gives only a type, and MEAN
becomes an external function because the unit uses `MEAN(...)` without
dimensioning it.

**The second walk reads the executable statements**, giving every
expression a type by section 2.3.1.2's rule: an operation on operands of
two types is done in the higher-ranked type (LOGICAL, then INTEGER, then
REAL), the other converted first. Each conversion becomes an explicit
node, so no later pass has to work out where one goes; an assignment
converts its value to its target's type the same way, so `MEAN = 0`
stores the REAL constant 0.0. Operations on constants are done here
(constant folding), through package `vaxfloat`, which rounds exactly as
the VAX's floating instructions do, so a folded value is the one the
program would have computed.

The same walk builds the **blocks**. The parser's flat list becomes
structured statements (an `If` holding its arms, a `Do` its body) by a
stack of the constructs open at each statement: a DO pushes a loop, and
the statement with its label pops it, with any other loops ending there.
Each statement records the constructs open at it, which is how the
**label rules** are checked once every label is known: no jump into an
IF block, none from one arm to another, and so on (section 5.7.3.1).

Last, each variable and array gets its place in the unit's static
storage, the `$LOCAL` program section; a dummy argument has none, being
the caller's. AVERAGE after this pass (`sema.Dump`):

```
== main program AVERAGE
  AVERAGE              program unit
  V         REAL       array(1:4)       $LOCAL+0
  MEAN      REAL       function
  I         INTEGER    variable         $LOCAL+16
  $LOCAL is 20 bytes
3           DO 10 I = 1, 4, 1
4     10      V[I] = (*:R (REAL I) 1.5)
5           PRINT * 'Mean:' MEAN(V[], 4)
6           END
== FUNCTION MEAN
  MEAN      REAL       result           $LOCAL+0
  A         REAL       array(1:*)       argument 1
  N         INTEGER    variable         argument 2
  I         INTEGER    variable         $LOCAL+4
  $LOCAL is 8 bytes
10          MEAN = 0.0
11          DO 10 I = 1, N, 1
12    10      MEAN = (+:R MEAN A[I])
13          MEAN = (/:R MEAN (REAL N))
14          END
```

The DO statements now have their step, the loops' bodies are indented
under them, `I * 1.5` is a REAL multiplication of I converted, and
`V(I)` is an element (`V[I]`) while `MEAN(V, 4)` is a call passing the
whole array (`V[]`).

When the whole file has been read, the units are **checked against one
another**: a function's type where it's referenced, and the number of
arguments. VAX FORTRAN compiled each unit alone and couldn't. Leave
`MEAN` out of AVERAGE's REAL statement, and its first letter makes it
INTEGER in the main program:

```
%FORT-E-FUNCTYPE, the function MEAN is REAL (line 8), but is used here as INTEGER
  at line 5 of DUA2:[WORK]AVERAGE.FOR;1
        PRINT *, 'Mean:', MEAN(V, 4)
  ------------------------^
```

A mistake is reported once: an expression with an error has the
`Invalid` type, and nothing built on it reports again.

## 8. The fifth pass: the IR and lowering (`internal/fortran/ir`)

After semantic analysis the program still reads like FORTRAN: a DO holds
its body, PRINT its items, an element its subscripts. Lowering turns it
into the **intermediate representation** (Decision 8), where every step
is written down: where each value is, which statements run in order,
and where control goes next. FORTRAN's implicit rules become explicit
here, so the code generator has only to choose instructions.

A routine's code is a list of **basic blocks**: runs of statements that
always run together, each ended by a terminator (`jump`, `branch`,
`return`, or none after STOP). The statements are only two, `assign`
and `call`, over **expression trees**. Why trees, and not the
three-address code many compilers use (one operation per statement,
every intermediate value named)? Because the VAX takes any operand from
memory in any addressing mode, so `I = J*K + L` is two instructions with
one register between them, and a tree shows the code generator which
operands are leaves it can name in place. The exception is a function
reference: lowering takes every call out of its expression, its value
into a temporary, so no value is ever in a register across a `CALLS`.

Lowering also adds the compiler's own storage: **cells** in `$LOCAL`
(a DO loop's count `$C1`, temporaries `$T1`) and read-only **data** in
`$PDATA` (constants passed by reference `$K1`, texts, descriptors `$D1`,
I/O control blocks `$IO1`). AVERAGE lowered (`ir.Dump`):

```
routine AVERAGE main
  cell  V       F  $LOCAL+0    array V, 16 bytes
  cell  I       L  $LOCAL+16   variable I
  cell  $C1     L  $LOCAL+20   DO 10's count
  cell  $T1     L  $LOCAL+24   temporary
  $LOCAL is 28 bytes
  datum $IO1    control PRINT *
  datum $K1     text 'Mean:'
  datum $D1     descriptor T S 5 $K1
  datum $K2     L 4
  datum $D2     descriptor L S 4 $T1
B1:
  ; line 3
  assign $C1 = 4
  assign I = 1
  jump B2
B2: L10  ; DO 10's body
  ; line 4
  assign V-4[I] = mul.F(cvt.LF(I), 1.5)
  assign I = add.L(I, 1)
  assign $C1 = sub.L($C1, 1)
  branch gt.L($C1, 0) B2 B3
B3:
  ; line 5
  call FTN$PRINT_LIST(ref $IO1)
  call FTN$IO_ITEM(ref $D1)
  call MEAN(ref V, ref $K2) -> $T1
  call FTN$IO_ITEM(ref $D2)
  call FTN$IO_END()
  ; line 6
  return 1
routine MEAN function F
  param A       F  argument 1
  param N       L  argument 2
  cell  MEAN    F  $LOCAL+0    result MEAN
  cell  I       L  $LOCAL+4    variable I
  cell  $C1     L  $LOCAL+8    DO 10's count
  $LOCAL is 12 bytes
B1:
  ; line 10
  assign MEAN = 0.0
  ; line 11
  assign $C1 = @N
  assign I = 1
  branch le.L($C1, 0) B3 B2
B2: L10  ; DO 10's body
  ; line 12
  assign MEAN = add.F(MEAN, @A-4[I])
  assign I = add.L(I, 1)
  assign $C1 = sub.L($C1, 1)
  branch gt.L($C1, 0) B2 B3
B3:
  ; line 13
  assign MEAN = div.F(MEAN, cvt.LF(@N))
  ; line 14
  return MEAN
```

Things to see in it:

- **A DO loop** is its iteration count, `(limit - start + step)/step`,
  computed before the variable is set (section 5.3.1.1), a test that
  skips the body when the count is zero or less, and at the end a step
  and a count down. Lowering folds as it builds, so the main program's
  count is the constant 4, whose test it leaves out, and MEAN's is just
  `@N`.
- **An array element** is an address: the array's, less the size of the
  elements below the lower bound, indexed by the subscript. `V-4[I]` is
  V's address less 4, plus 4 times I, which is `V(I)` because V's first
  element is `V(1)`. The VAX's indexed mode does that multiplication.
- **A dummy argument** is reached through the argument list: `@N` is
  the variable N's argument longword points at.
- **The PRINT statement** is FTNRTL's calls, a descriptor for each
  item; the function's value goes into the temporary `$T1`, and the
  item's descriptor `$D2` points at it.
- **The main program returns 1**, SS$_NORMAL, and a function its value.
- Each statement keeps the FORTRAN statement it came from (`; line n`),
  so the code generator can write each source line just before its code.

`ir.Check` verifies a routine's structure (every block terminated, every
jump within the routine, operand types agreeing); the tests run it on
everything they lower.

## 9. The last pass: code generation (`internal/fortran/codegen`)

The code generator has two jobs: choosing instructions, and writing
text another program reads.

### Choosing instructions

The VAX makes trees easy to compile. Nearly every instruction takes any
operand straight from memory, and the arithmetic ones have a
three-operand form, so a tree's leaves (a variable, a constant, an
element) are operands in place, the last operation writes straight to
the assignment's target, and only values inside a tree need registers
(`expr.go`). When the target is also an operand the two-operand form
does (`ADDF2`), and `INCL` and `DECL` add and subtract 1.

- **Registers.** Interior values go in R0 to R5, by Sethi and Ullman's
  order: of an operation's two operands, compute first the one needing
  more registers, so as few as possible are held at once. An expression
  needing more than a budget has parts computed first into spill cells
  (`$X1`), so registers never run out. The calling standard lets a
  routine use R0 and R1 freely, but it must preserve R2 to R11, so a
  routine using any of those names them in its entry mask, and `CALLS`
  saves them.
- **Places.** A variable is its symbol (`MEAN.I`), a dummy argument
  `@n(AP)`, an element indexed mode. A dummy array has no room in
  `@n(AP)` for its offset, so the offset goes into the index instead:
  `SUBL3 #1,MEAN.I,R0` then `@MEAN.A(AP)[R0]`.
- **Constants.** An integer is an immediate operand, `#n`. A REAL
  constant is a longword of its F_floating bits in `$PDATA` (`$F1`),
  since MACRO reads `#n` in a floating instruction as a value to convert
  itself, and the compiler's own bits, folded by `vaxfloat`, are the ones
  wanted (Decision 16).
- **Branches** (`cond.go`). The VAX's conditional branches reach only
  127 bytes, and nothing here knows how long the code between is, so a
  branch is always the opposite condition's byte branch around a `BRW`,
  whose word displacement reaches 32K. A block reached by falling
  through needs no branch at all. Phase 84's optimizer can shorten what
  this writes; the point now is code that is always right.

MEAN's code, as it appears in `AVERAGE.MAR`:

```
;   10        MEAN = 0
	CLRF	MEAN.MEAN
;   11        DO 10 I = 1, N
	MOVL	@MEAN.N(AP),MEAN.$C1	; DO 10's count
	MOVL	#1,MEAN.I
	TSTL	MEAN.$C1
	BGTR	MEAN.$2
	BRW	MEAN.$1
MEAN.$2:
;   12     10 MEAN = MEAN + A(I)
MEAN.L10:				; DO 10's body
	SUBL3	#1,MEAN.I,R0
	ADDF2	@MEAN.A(AP)[R0],MEAN.MEAN
	INCL	MEAN.I
	DECL	MEAN.$C1		; one pass fewer
	BLEQ	MEAN.$3
	BRW	MEAN.L10
MEAN.$3:
MEAN.$1:
;   13        MEAN = MEAN / N
	CVTLF	@MEAN.N(AP),R0
	DIVF2	R0,MEAN.MEAN
;   14        END
	MOVF	MEAN.MEAN,R0		; the function's value
	RET
```

`DECL` sets the condition codes, so the loop's test needs no `TSTL`
(the one peephole done now). In the main program, `MEAN(V, 4)` is the
calling standard's call: the arguments' addresses pushed last to first,
`CALLS #2,G^MEAN`, and the value taken from R0:

```
	PUSHAL	AVERAGE.$K2
	PUSHAL	AVERAGE.V
	CALLS	#2,G^MEAN
	MOVF	R0,AVERAGE.$T1
```

### Writing the file

`Writer` lays out each MACRO line in the columns MACRO programmers use
(opcode at 8, operands at 16, comment at 40), keeps every line within
MACRO's 132 characters (a comment that won't fit moves to a line of its
own, a long one is split), and writes the FORTRAN source as comments. A
source line's prefix (`;`, the number in five columns, two blanks) is
exactly 8 characters, one tab stop, so tab-format source keeps its
layout; an included line's prefix starts with its include depth
(`;1  12  `). `Generate` writes the module: the `DSC$K_` codes its
descriptors use (govax's STARLET has no `$DSCDEF`), then for each
routine a storage map, its `$LOCAL` storage, its `$PDATA` data, and its
`$CODE`. Each routine's code is generated before its storage is written,
since generating it makes cells and constants of its own.

Names follow Decision 10: a routine's entry is its FORTRAN name, a
global symbol, and everything it owns is named for it, `routine.name`:
`MEAN.I`, `MEAN.L10` (FORTRAN's label 10), `MEAN.$C1` (a loop's count),
`MEAN.$2` (a label of the compiler's own). MACRO allows a period in a
symbol and FORTRAN names can't hold one, so these never collide, and the
debugger can name every variable. A name longer than MACRO's 31
characters becomes a numbered one, `.$Nn`, listed in the storage map.

The manual's section 14.5 shows a whole MACRO file.

## 10. FTNRTL: the run-time library (`internal/ftnrtl`)

Compiled code does arithmetic and branches itself; for I/O, STOP, and
the like it calls FTNRTL. FTNRTL is written in MACRO-32 (Decision 13),
not emulated in Go as LIBRTL is: it is part of the FORTRAN system being
built, its routines run as VAX code and can be traced and debugged like
any program's, and it shows how a run-time library is made on VMS.

### A shareable image and its transfer vector

FTNRTL is a shareable image: programs link against it rather than copy
it, and its pages are shared. A program refers to an FTNRTL routine by
the image's name and the routine's offset in the image, so those offsets
must never change, or every program linked before would call the wrong
code. The image therefore starts with a transfer vector
(`mar/ftn_vector.mar`), an 8-byte entry per routine:

```
	.TRANSFER FTN$STOP		; makes FTN$STOP universal, valued here
	.MASK	FTN$STOP		; the routine's entry mask (2 bytes)
	JMP	L^FTN$STOP+2		; jump past the routine's own mask
```

CALLS and CALLG read a register save mask at the address they're given,
so each entry starts with the routine's mask, then jumps past the real
routine's mask to its first instruction. New entries go at the end, and
the options file's `GSMATCH=LEQUAL,1,n` (n raised with each new entry)
lets a program linked against an older FTNRTL run on a newer one.

The image's code is position-independent, since the activator maps it
wherever there's room: it reaches its own data PC-relative, and never
stores an address (FTN$STOP builds the descriptor of "FORTRAN STOP" on
the stack rather than keep one in the image).

### Building it with govax's own tools

`internal/ftnrtl/build` assembles each module with `internal/asm` and
links them with `internal/link`, the code behind the MACRO and LINK
commands, as `LINK/SHAREABLE=FTNRTL/NOSYSLIB FTNRTL/OPTIONS` would. The
times and the tools' names are fixed, so the same sources always make
the same image, byte for byte; `go generate ./internal/ftnrtl` writes
`ftnrtl.exe`, which is committed and embedded in govax, and a test
checks it against a fresh build. `/NOSYSLIB` keeps any VMS libraries
out: FTNRTL's references to LIBRTL and the system services resolve
through govax's own tables.

### How programs find it

The shareable image locator (`rms.ImageLocator`) looks for an image as
VMS's activator does (a logical name, `SYS$SHARE`), then on the host, and
last among images built into govax (`Builtin`): FTNRTL's embedded bytes.
RUN's two loaders take an image from there as from a file (the mapping
loader's page source is the bytes in memory). LINK's system symbol
sources include FTNRTL's global symbol table, read from the image the
locator finds, and only when a link looks for an `FTN$` name, so a
FORTRAN program links with no options file, and a link that doesn't use
FTNRTL is unchanged. Anyone changing FTNRTL builds their own with MACRO
and LINK, and `DEFINE FTNRTL` to it: the locator finds it first.

### List-directed output

`PRINT *` and `WRITE (*,*)` are list-directed: each value is written in
a format its type chooses (the language manual's Table 7-1), not a
FORMAT statement's. A compiled statement is a call per part, as section
8 showed: `FTN$PRINT_LIST` (or `FTN$WRITE_LIST`) with the statement's
control block, `FTN$IO_ITEM` with each item's descriptor, and
`FTN$IO_END`. Why calls per item rather than one call with an argument
list of every item? Because an item list can hold implied-DO loops
(a later phase), whose number of items is known only at run time; the
compiled code runs the loop and calls for each.

- **The control block** is a few longwords in the program's `$PDATA`: a
  word of size, a word of flags (which of UNIT=, FMT=, ERR=, and the
  rest the statement has), and the unit, -1 for `*`. Later phases add
  fields after these and raise the size, and FTNRTL reads only what the
  size says is there, so old programs go on working.
- **The descriptors** are the VAX calling standard's: the item's
  length, its data type (`DSC$K_DTYPE_L`, `_LU`, `_F`, `_T`), its class
  (S for a scalar, A for a whole array), and its address. The data type
  chooses the format, so FTNRTL needs nothing else to write a value.
- **The unit** (`ftn_units.mar`) is SYS$OUTPUT, opened at the first
  statement with RMS's `$CREATE` and `$CONNECT`, its FAB and RAB in a
  writable psect of FTNRTL's own. `RAT=FTN` asks RMS for FORTRAN carriage
  control: each record's first byte is its carriage control, a blank for
  a new line. That is why every list-directed record starts with a
  blank: the blank isn't printed, but turned into the line's start.
- **The formats** (`ftn_listout.mar`) fill an 80-byte record buffer: L2
  from bit 0, I12 by LIBRTL's `OTS$CVT_L_TI`, a text copied, and REAL's
  `1PG15.7E2` from the decimal digits `OTS$$CVT_F_T_R8` returns, FTNRTL
  choosing the fixed or exponent form by the rounded value's magnitude.
  A value that won't fit what's left of the record writes the record
  (`$PUT`) and starts the next; the end call writes the last one.

FTNRTL calls LIBRTL for the conversions as any VAX program would, through
LIBRTL's transfer vector; a run-time library built on another is the
normal arrangement on VMS.

## 11. Testing a compiler

Each pass is tested on its own, on sources held in memory, and the whole
pipeline is tested end to end on govax.

- **Golden files.** The parser, semantic analysis, lowering, and the
  code generator each have a `testdata` directory of FORTRAN sources and
  the dump each should produce (`.ast`, `.sema`, `.ir`, `.mar`). A test
  runs the pass and compares; `go test -update` rewrites the files after
  an intended change, and the diff is then reviewed like code. Dumps are
  plain text so that a change to a pass shows as a readable change to
  its output.
- **Hard cases.** The lexer's tests are FORTRAN's classic puzzles
  (`DO 10 I = 1.5`, `IF(X) = 3`, `REAL*8D1`, `INTEGER FUNCTIONF(10)`),
  and each pass has tables of statements and the messages they draw.
- **Assembling.** Every MACRO file the code generator's tests write is
  assembled by govax's MACRO, which must say nothing, so a code
  generator change can't produce text MACRO rejects.
- **Running.** In `internal/console`, `TestFortranCodeRuns` links a
  program covering the subset with a small MACRO module that prints
  values, and checks every one. `TestFortranPrograms` is the end-to-end
  harness: each `testdata/fortran/name.for` is compiled, assembled,
  linked against FTNRTL, and run, and its output compared with
  `name.out`, written by hand from the language's rules (there is no VAX
  FORTRAN to check against). FTNRTL's routines are tested from MACRO
  callers (`TestFTNRTL`, `TestFTNRTLListOutput`), in both of RUN's
  paging modes.
