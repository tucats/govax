# Inside govax's FORTRAN

This is a guided tour of govax's FORTRAN compiler and its run-time
library, FTNRTL, for someone who wants to know how they work, or to
write a compiler of their own for the VAX under VMS. It grows a phase at
a time with the compiler (`docs/PHASE-74 - FORTRAN plan.md` lays out
the program; its Part B holds the decisions this tour refers to). The
user's side is the manual's chapter 14, "Programming in FORTRAN".

So far (Phases 74 to 79) the compiler parses every statement of the
language and gives every declaration its meaning and its storage, and
compiles a subset of the executable statements: main programs,
subroutines, and functions of every type, with ENTRY, alternate returns,
adjustable arrays, procedures as arguments, the built-in functions, and
statement functions; every expression in every type (BYTE, LOGICAL and
INTEGER of every length, REAL\*4, \*8 in D or G, and \*16, COMPLEX\*8 and
\*16, CHARACTER), with every intrinsic function; assignment; every IF, DO,
and GO TO, CALL, RETURN, STOP, and PAUSE; and `PRINT *` and `WRITE (*,*)`.
The tour follows one small program through every pass (section 1), then
takes the passes in order, with a second program for declarations and
storage (section 8), a third for the types and how each chooses its
instructions (section 11), and a fourth for control flow and subprograms
(section 12), then FTNRTL, then how the compiler is tested.
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
 lex ------------- each statement's kind, and its tokens (Phases 75-76)
 parse ----------- tokens into a syntax tree             (Phases 75-76)
 sema ------------ names, types, blocks, labels, storage (Phases 75-78)
 ir -------------- the tree lowered to basic blocks      (Phases 75, 78)
 opt ------------- optional improvements                 (Phase 84)
 codegen --------- VAX instructions, as MACRO-32 text    (Phases 74-78)
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
expected where (`expected '=', but found ','`). Every kind has a
function, since Phase 76: the parser builds a node for every statement
of VAX FORTRAN, and it is semantic analysis that reports one it can't
compile yet, by name, where it reaches it: `%FORT-E-NOTIMPL, the READ
statement is not implemented yet`. Keeping that judgment out of the
parser means the grammar is finished once, and each later phase only
gives statements their meaning.

The I/O statements share one grammar, the **control list**: specifiers
in parentheses, each `keyword=value`, except that the first (the unit)
and the second (the format) may leave their keywords out, and a direct
access record may follow the unit as `u'r`. One function reads every
statement's list, with a table saying which keywords each statement
takes and what follows each (an expression, a label, or nothing), and
takes the unit, format, and record out into the statement's own fields,
however they were written: `WRITE (6, 10)`, `WRITE (FMT=10, UNIT=6)`.

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
the next pass. The one exception is the structure declaration
(STRUCTURE ... END STRUCTURE, with UNION and MAP inside): its type
statements declare a record's fields, not variables, so the parser puts
them inside the declaration's node, where the next pass can't mistake
them for declarations of the unit's names.

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

## 8. Declarations and storage

AVERAGE declares very little. A real FORTRAN program's declarations are
half its text, and they make an awkward demand on a compiler: **a
declaration can come after a statement that depends on it.** A name's
dimensions can come before its type or after it, and DATA may give an array
its values before the statement that says how long it is. So semantic
analysis lays nothing out as it reads. The first walk only collects; when
every declaration has been read, `analyzeUnit` (`analyze.go`) runs these
stages, among others, in the one order that works:

```
layoutCommon    each common block's names get their places
equivalences    EQUIVALENCE's groups, and those in blocks placed
checkArrays     an adjustable array's bounds, now every name is known
dataStatements  each DATA list's values worked out, as bytes
layout          everything left gets its place in $LOCAL
placeInitials   each value gets its place, now its name has one
```

Here is a program that uses most of it (`testdata/fortran/store.for`):

```
      PROGRAM STORE
      PARAMETER (N = 3)
      COMMON /SHARED/ COUNT, TOTAL(N)
      INTEGER COUNT, TABLE(N)/1, 2*7/, BITS
      REAL X
      EQUIVALENCE (X, BITS)
      DATA X /1.0/
      COUNT = TABLE(2)
      PRINT *, COUNT, BITS, TOTAL
      END

      BLOCK DATA START
      COMMON /SHARED/ COUNT, TOTAL(3)
      INTEGER COUNT
      DATA TOTAL /3*2.5/
      END
```

and what semantic analysis makes of it (`sema.Dump`):

```
== main program STORE
  STORE                   program unit
  N         INTEGER       constant         = 3
  COUNT     INTEGER       variable         SHARED+0
  TOTAL     REAL          array(1:3)       SHARED+4
  TABLE     INTEGER       array(1:3)       $LOCAL+0
  BITS      INTEGER       variable         $LOCAL+12
  X         REAL          variable         $LOCAL+12
  $LOCAL is 16 bytes
  /SHARED/ is 16 bytes
  initial $LOCAL+0 TABLE(1) INTEGER = 01 00 00 00
  initial $LOCAL+4 TABLE(2) INTEGER = 07 00 00 00
  initial $LOCAL+8 TABLE(3) INTEGER = 07 00 00 00
  initial $LOCAL+12 X REAL = 80 40 00 00
10          COUNT = TABLE[2]
11          PRINT * COUNT BITS TOTAL[]
12          END
== BLOCK DATA START
  START                   program unit
  COUNT     INTEGER       variable         SHARED+0
  TOTAL     REAL          array(1:3)       SHARED+4
  $LOCAL is 0 bytes
  /SHARED/ is 16 bytes
  initial SHARED+4 TOTAL(1) REAL = 20 41 00 00
  initial SHARED+8 TOTAL(2) REAL = 20 41 00 00
  initial SHARED+12 TOTAL(3) REAL = 20 41 00 00
18          END
```

### Types, IMPLICIT, and statement order

A `Type` is a kind and a size, so each line of the manual's Table 2-1 is its
own type, and BYTE is only another way to write LOGICAL\*1 (`types.go`).
Phase 76 gives every type its size and alignment; `Type.Compiled` says
which ones expressions can use so far. A name of another type still gets
its storage and initial values, and draws `%FORT-E-NOTIMPL` only where a
statement first uses it, so each later phase has only expressions to add.

The IMPLICIT statements are read before anything else in the unit, since
they type the names in the unit's own header (`implicit.go`).

Figure 1-1's rules of order become a number (`order.go`): each statement
has a group it puts the unit in, and the last group it may come in, and
the unit's group only rises. A statement is out of order when the unit is
already past its last group, and the message names the statement that took
the unit there.

### Constant expressions

`PARAMETER (N = 3)` makes N a name for a value, not a variable: it has no
storage, and `TOTAL(N)` is an array of three. Constant expressions are
computed from the syntax tree by an evaluator of their own (`constant.go`),
not by analyzing them and folding the result, because analyzing an
expression enters its names in the symbol table as variables, and a bound
or a DATA subscript must not create any. The evaluator works in every type
through `vaxfloat`, so that a value worked out at compile time is the one
the VAX's own instructions would have computed; and it keeps a REAL
literal's exact value until it knows the type wanted, so that `1D0 + 0.1`
adds D_floating 0.1, not F_floating's 0.1 made longer.

### COMMON: the linker does the sharing

Nothing in FORTRAN says that a unit's view of a block agrees with another's:
STORE calls its two pieces COUNT and TOTAL, and another unit may call them
something else, or stop at the first. All a unit knows is its own list, so a
name's place is simply the sum of the sizes before it, with **nothing aligned**:
aligning would move one unit's names away from another's (`common.go`). The
VAX doesn't mind, and reads unaligned data.

The sharing is the linker's. Each block is a program section of its name, with
the attributes the manual's Table 10-1 gives (blank common's named `$BLANK`):
`GBL` so that every module's section of that name is one section, and `OVR` so
that each module's part starts at its base rather than following the last. LINK
makes it as long as the longest part. So a compiler need only write, in each
module:

```
; Common block /SHARED/, 16 bytes, the most any unit in this file declares
	.PSECT	SHARED,PIC,OVR,REL,GBL,SHR,NOEXE,RD,WRT,LONG
SHARED.$BASE:
	.BLKB	4
	.LONG	^X00004120[3]		; TOTAL(1) to TOTAL(3) = 2.5
```

One catch is worth knowing if you write a compiler like this. `OVR` is a word
to the *linker*, about modules. Inside one module MACRO goes on from where a
section was left, overlaid or not, so two units of one file each writing their
part of a block would put the second after the first. Hence each block is
written **once per file**, at the top, with its longest length and every unit's
initial values merged; and each unit's names are only direct assignments from
its base, which is why they cost nothing and the debugger can name them:

```
STORE.COUNT = SHARED.$BASE		; COUNT, in /SHARED/
STORE.TOTAL = SHARED.$BASE+4		; TOTAL, REAL (3), in /SHARED/
```

### EQUIVALENCE: groups of names

An EQUIVALENCE list says its items start at one place, so what the lists
really describe are groups of names with fixed places relative to each other
(`equivalence.go`). Take

```
      DIMENSION A(4), B(6)
      EQUIVALENCE (A(2), B(1)), (B(3), C)
```

The first list puts B 4 bytes after A, since A(2) is 4 bytes into A and B(1) is
B's start: in the group's own coordinates, A is at 0 and B at 4. The second list
has C 8 bytes into B, so C joins them at 12. Reading a list is merging two groups:
each name starts alone, at 0, and merging moves every member of one group by
whatever makes the two items' places equal. This is the classic union-find with
an offset on every member, done the plain way, moving each member, since a
group is rarely more than a few names. If both items are already in one group,
nothing can move, and their places either agree or the program has put a name
in two places (`EQUIVCONFLICT`).

Then the group is placed. If a member is in a common block, the member's place
there says where the group's 0 is: it may not fall before the block's start
(`EQUIVBEFORE`), and the group may run past the end, which lengthens the block.
Any other group goes in `$LOCAL` as one unit when `layout` reaches its first
name, on the boundary of its strictest member. In the `.MAR` file a group is one
piece of storage under the label of its lowest name, and the others are
assignments from that label, as with BITS and X:

```
STORE.BITS:
	.LONG	^X00004080		; X = 1.0
STORE.X = STORE.BITS			; X
```

### DATA: values are bytes

An initial value makes no code, so every rule in the manual's section 4.3 comes
down to *which bytes an item gets* (`data.go`), and they are simple to state that
way:

```
INTEGER*2 K / '12345'X /      a hexadecimal constant is cut on the left: 45 23
CHARACTER*6 STARS / '****' /  a character constant is padded with blanks
REAL X / 'AB' /               so is one given to a number: A, B, two blanks
CHARACTER BELL / 7 /          an integer is one character's code
REAL Y / 1 /                  a number becomes the type first: 80 40 00 00
```

(That last line isn't a mistake: an F_floating value's bytes begin with the word
holding its sign and exponent.) A type statement's `TABLE(N)/1, 2*7/` is just a DATA
list of one name. A list of names and a list of values are walked together, an
array name or an implied-DO list producing items and a repeat count holding
a value's place, so that a list with too many items is reported at the first
with no value, however many its implied-DO lists would have made. An implied-DO's
variable lives only in the evaluator, and never enters the symbol table.

Each value is an `Initial`, its bytes and later its place. The code generator
writes them in order of place, with `.BLKB` for the gaps, and floating values in
hexadecimal with the source's spelling beside them, so nothing is left to MACRO's
own conversion of a decimal number. Items in a row with one value become a
repetition, `.LONG 0[1000]`, which keeps a large table's file short.

### BLOCK DATA and EXTERNAL

A block data subprogram is a unit with storage and no code: the IR's routine has
no blocks, and the generator writes no `.ENTRY`. It still needs a symbol, for
the sake of libraries. If START is in an object library, nothing makes LINK pull
it out, since no one calls it. The language's answer is to name it in an EXTERNAL
statement (the manual's section 4.1), so the compiler writes a reference for
every EXTERNAL name, called or not, and the BLOCK DATA defines its name as a
global absolute symbol:

```
	.EXTRN	START			; named in an EXTERNAL statement
START == 0				; this block data subprogram's name, for EXTERNAL statements
```

So a block data module left out of the link draws LINK's undefined symbol
message, rather than a program quietly running with zeros.

## 9. The fifth pass: the IR and lowering (`internal/fortran/ir`)

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
  datum $IO1    control
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
  call FTN$WRITE_SL(ref $IO1, val -1)
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

## 10. The last pass: code generation (`internal/fortran/codegen`)

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

- **Registers.** Interior values go in registers from R0 up, by Sethi and
  Ullman's order: of an operation's two operands, compute first the one needing
  more registers, so as few as possible are held at once. An expression
  needing more than a budget has parts computed first into spill cells
  (`$X1`), so registers never run out. The calling standard lets a
  routine use R0 and R1 freely, but it must preserve R2 to R11, so a
  routine using any of those names them in its entry mask, and `CALLS`
  saves them. (Section 11 has values that take two or four registers.)
- **Places.** A variable is its symbol (`MEAN.I`), a dummy argument
  `@n(AP)`, an element indexed mode. A dummy array has no room in
  `@n(AP)` for its offset, so the offset goes into the index instead:
  `SUBL3 #1,MEAN.I,R0` then `@MEAN.A(AP)[R0]`.
- **Constants.** An integer is an immediate operand, `#n`. A REAL
  constant is its bits in `$PDATA` (`$F1`; a longword for F_floating,
  more for the others, section 11),
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

## 11. Every type and its instructions

The first compiled subset had two types, INTEGER and REAL, and Phase 78
gave the code generator the rest. It needed less new code than one might
expect, because the VAX names its instructions by an operation and a
type: `ADDL3` adds longwords, `ADDW3` words, `ADDF3` F_floating values,
`ADDD3` D_floating, `ADDH3` H_floating. A compiler that knows a value's
type knows its instruction's last letters. What took the work was the
places where the VAX doesn't follow that pattern, and the three kinds of
value that aren't one machine value at all: COMPLEX, CHARACTER, and the
results of functions too wide for R0. This section follows a third
program through them:

```
      SUBROUTINE AREA(R, C, S, A)
      REAL*8 R, A
      COMPLEX C
      CHARACTER*(*) S
      A = 4*DATAN(1D0)*R**2
      C = C*(0.0, 1.0) + R
      IF (S .LT. 'M') S = 'SMALL'
      END
```

### The IR's types

Each IR type is a VAX data type, named by its instructions' letter:

| FORTRAN | IR type | Bytes | Registers |
| --- | --- | ---: | ---: |
| BYTE, LOGICAL\*1 | `B` | 1 | 1 |
| INTEGER\*2, LOGICAL\*2 | `W` | 2 | 1 |
| INTEGER\*4, LOGICAL\*4 | `L` | 4 | 1 |
| REAL\*4 | `F` | 4 | 1 |
| REAL\*8 | `D`, or `G` under `/G_FLOATING` | 8 | 2 |
| REAL\*16 | `H` | 16 | 4 |
| COMPLEX\*8 | `Q`, two `F` parts | 8 | 2 |
| COMPLEX\*16 | `O`, two `D` or `G` parts | 16 | 4 |
| CHARACTER\*n | a `B` place and a length | n | none |

LOGICAL and INTEGER of one length are one IR type. What differs between
them (a LOGICAL is tested by bit 0, and its descriptor says LU, not L)
is settled in sema and lowering, so the code generator never asks. A
REAL\*8 is D or G by the unit (`/G_FLOATING`, or an OPTIONS statement
before the unit), and sema decides which; a unit never has both, which
is as well, since the VAX has no instruction that converts one to the
other.

### An instruction from its letter

`operate` (`codegen/expr.go`) writes every arithmetic operation as an
opcode stem and the type's letter: `ADD`, `SUB`, `MUL`, `DIV`, `MNEG`, `BIS`
(`.OR.`), `XOR`, `MCOM` (`.NOT.`), and `BIC` for `.AND.`, which the VAX
has only as "clear the bits the mask has set", so `X .AND. Y` is
`BICL3` of Y's complement. A conversion is `CVT` and both types' letters,
`CVTWL`, `CVTFD`, `CVTDF` (which rounds), `CVTFL` (which truncates, as an
assignment of a REAL to an INTEGER must); NINT is `CVTRFL`, which rounds;
ZEXT is `MOVZWL`. Only the integers have `INC` and `DEC`.

Where the pattern breaks, each place was found by a program that went
wrong, or by reading the architecture manual:

- **Indexed mode scales by the instruction's type.** `A[R0]` is A's
  address plus R0 times the size of what the *instruction* works on. A
  `MOVAL` of a D_floating element would scale by 4, so an element's
  address is taken with its own letter (`MOVAD`, `PUSHAD`, `MOVAQ` for a
  COMPLEX\*8 element; `addressOp`). An operand an instruction reads as
  another type than its own (a count for `ASHL`, a word for `EXTZV`'s
  position) is first put in a register if it is indexed (`unindexed`).
- **`BLBC` and `BLBS` read a longword.** Testing a LOGICAL\*1 in memory
  by its bit 0 that way reads three bytes past it, which may not exist,
  so a byte or word in memory is tested with `BITB #1` or `BITW #1` and
  a `BEQL` or `BNEQ` (`cond.go`).
- **Floating constants are data.** MACRO reads `#1.5` in a floating
  instruction as a number to convert itself; the compiler wants its own
  bits, folded by `vaxfloat` as the CPU would round (Decision 16), so a
  D, G, or H constant is a datum in `$PDATA` (`AREA.$F1` below, 4D0), and
  an F constant stored is `MOVL #^X...` of its bits.
- **Character comparisons are unsigned.** `CMPC3` and `CMPC5` set the
  condition codes as an unsigned comparison of the bytes, so their
  branches are `BLSSU`, `BGTRU`, and the rest, where every other
  comparison here is signed.

### Registers in runs

A D_floating value in registers occupies two, R0 and R1 say, and an
instruction names only the first: `MULD3 R0,R2,R4` multiplies R0:R1 by
R2:R3 into R4:R5. An H value takes four. So each value takes a run of its
type's width (`ir.Type.Width`), `alloc` finds the lowest free run, and
Sethi and Ullman's count (`need`) counts registers rather than values.
The pool is R0 to R11, of which an expression may hold at most four
(`budget`) before its biggest parts are computed into spill cells of
their own type (`fit`, `spill`). The pool is so much larger than the
budget so that a run of four can always be found, however the
registers held are scattered.

Some instructions use registers the allocator never handed out: `MOVC5`
leaves R0 to R5 changed, `CMPC5` and `MATCHC` R0 to R3, and an
`OTS$..._R3` routine returns its value in R0 to R3. None of these is
ever in the middle of an expression (the next sections say how), so
nothing the allocator holds can be lost; but the caller's R2 to R5 are
lost unless the routine's entry mask saves them, so they are marked used:
AREA's `.ENTRY AREA,^M<R2,R3,R4,R5>` is for its `MOVC5`.

### Mixed types

Sema converts by rank (the language manual's 2.3.1.2: integer below
real below complex, and the longer above the shorter within each), so
by the time an expression reaches the IR, every operation's operands
have its type and every conversion is an explicit `cvt`. Two rules come
from the manual's text, not its table. An integer constant from -32768
to 32767 beside an INTEGER\*2 is a word (2.2.1.1), so `I2*1000` is a
`MULW3` and wraps where a longword wouldn't. And a REAL\*4 literal in a
REAL\*8 expression "retains additional precision": sema's constants keep a
literal's exact value, so `D + 0.1` adds 0.1D0, not the F_floating 0.1
widened.

### COMPLEX: two real expressions

A complex operation is not one instruction: a product is four
multiplications and two additions. Rather than teach the code generator
complex arithmetic, lowering splits each complex expression into its real
part's tree and its imaginary part's (`ir/lowercomplex.go`, Decision 1),
over F, D, or G operations. AREA's line 6, lowered:

```
  ; line 6
  assign $T2 = cvt.DF(add.D(cvt.FD(sub.F(mul.F(@C, 0.0), mul.F(@C+4, 1.0))), @R))
  assign @C+4 = cvt.DF(cvt.FD(add.F(mul.F(@C, 1.0), mul.F(@C+4, 0.0))))
  assign @C = $T2
```

`C*(0.0, 1.0)` is `(ac - bd, ad + bc)` written out, with `@C` the real part
and `@C+4` the imaginary one. Adding R, a REAL\*8, makes the sum a
COMPLEX\*16 (a REAL\*8 beside a COMPLEX\*8 isn't rounded), so each part is
widened, summed, and rounded back to the target's F_floating. The real
part goes to a temporary first, because the imaginary part's tree still
reads `@C`: storing the real part straight into C would change what the
second tree computes. A complex value moved or passed whole is one `MOVQ`
or `MOVO`.

Two operations aren't written out. A COMPLEX\*16 product calls
`OTS$MULCD_R3` (or `OTS$MULCG_R3`), and every quotient `OTS$DIVC` or
its D and G forms, which compute each part exactly and round once; the
compiler's constant folding computes the same way (`vaxfloat`), so a
folded expression has the value the program would compute. COMPLEX\*8 has
no such routine, so its product is in line, in F's own rounded steps, and
folding does those steps too (`vaxfloat.ComplexMulSteps`). Such a routine
takes its arguments by value, which the IR's `Arg.Value` does for any
type (`MOVQ x,-(SP)`, two longwords of the argument list), and returns
a value in R0 up, as wide as it is.

### CHARACTER: statements, not expressions

A character value isn't in registers at all: it is a place and a
length (`ir.Text`), and the instructions that work on it, `MOVC3`,
`MOVC5`, `CMPC3`, `CMPC5`, and `MATCHC`, take both and use R0 to R5 for
their own work. So character operations are the IR's statements, not its
expressions (Decision 2): `CharMove` is an assignment, `CharCmp` a
comparison that may only be a branch's condition or the whole of an
assignment's value, and every operand they need is computed before the
instruction runs. No register is held across them, and so none can be
lost.

A CHARACTER dummy argument's text is two steps from the argument list,
since a character argument is passed by descriptor: the longword at
`n(AP)` points at the descriptor, whose first word is the length and
whose second longword the text's address. AREA's line 7:

```
;    7        IF (S .LT. 'M') S = 'SMALL'
	MOVZWL	@AREA.S(AP),R0                  ; S's length
	MOVL	AREA.S(AP),R1                   ; its descriptor's address
	CMPC5	R0,@4(R1),#^A/ /,#1,AREA.$K2    ; S against 'M', blank-padded
	BLSSU	AREA.$2
	BRW	AREA.$1
AREA.$2:
	MOVZWL	@AREA.S(AP),R0
	MOVL	AREA.S(AP),R1
	MOVC5	#5,AREA.$K3,#^A/ /,R0,@4(R1)    ; 'SMALL', blank-filled
AREA.$1:
```

`@4(R1)` is the deferred mode: the operand at the address held 4 bytes
past R1, which is the text. `CMPC5` and `MOVC5` give the language's rules
for free: a comparison of different lengths as though the shorter were
filled with blanks, and an assignment cut or filled with blanks to the
target's length. They also move correctly when the source and target
overlap, so `S(2:) = S` needs nothing special.

A concatenation (`ir/lowerchars.go`, Decision 8) is always built in a
temporary, part by part, before it is used, since a dummy argument or
EQUIVALENCE can hide an overlap with its target. The temporary is a cell
as long as the value can be, or, when one part is a passed-length dummy
whose length isn't known until the program runs, a dynamic string from
LIBRTL's `LIB$SGET1_DD`, freed by `LIB$SFREE1_DD` at the statement's end.

### Powers and the intrinsic functions

`X**N` for an INTEGER or REAL X and a constant N from 1 to 8 is
multiplied out in line; anything else calls an `OTS$POW` routine
(`ir/lowerpower.go`). The in-line products are made in the same order as
the routine's (`vaxfloat.PowerBySquaring`: repeated squaring, the result
multiplied by the squares the exponent's bits select), so `X**7` has the
same value whichever way it is computed, and the same as when it is
folded. AREA's `R**2` is the one `MULD3 @AREA.R(AP),@AREA.R(AP),R2`.

The intrinsic functions are a table (`sema/intrinsics.go`), one row for
each line of the manual's Table D-3: the generic name, the specific name,
the argument count, the arguments' and result's types, and how it
compiles. A reference picks its row by its arguments' type (a generic
name) or by its name (a specific one), and becomes one of three things:

- **an operator or a conversion,** when an instruction or two does it:
  IAND is `.AND.`, REAL is a `cvt`, NINT a `round`, IBITS an `EXTZV`,
  INDEX a `MATCHC`. The functions that choose (ABS, MAX, MIN, DIM, SIGN)
  branch, so they are lowered as calls are: before the statement, through
  blocks of their own, their value in a temporary.
- **a call of MTHRTL,** for SQRT, EXP, SIN, and the rest: the specific
  name's routine with the unit's format's letter (`MTH$DATAN` for DATAN,
  `MTH$GATAN` under `/G_FLOATING`, `MTH$HATAN` for QATAN), the arguments
  by reference. AREA's DATAN is the first three instructions below. A
  REAL\*16 or COMPLEX\*16 result is too wide for R0 to R3 as the calling
  standard uses them, so it comes back through a hidden first argument,
  the address of a temporary.
- **a call of FTNRTL,** for ISHFTC, which no instruction does.

An intrinsic function's name passed as an argument is its routine's
address, `PUSHAB G^MTH$SIN`; FTNRTL has a routine for each name MTHRTL
lacks (LEN, INDEX, the lexical comparisons, the bit functions, ...), so
that every name but SIZEOF can be passed.

### AREA, compiled

```
	.ENTRY	AREA,^M<R2,R3,R4,R5>
;    5        A = 4*DATAN(1D0)*R**2
	PUSHAD	AREA.$K1                ; 1D0, by reference
	CALLS	#1,G^MTH$DATAN
	MOVD	R0,AREA.$T1             ; the result, in R0:R1
	MULD3	AREA.$F1,AREA.$T1,R0    ; 4D0 times it, into R0:R1
	MULD3	@AREA.R(AP),@AREA.R(AP),R2  ; R**2, into R2:R3
	MULD3	R0,R2,@AREA.A(AP)
;    6        C = C*(0.0, 1.0) + R
	MULF3	@AREA.C(AP),AREA.$F2,R0     ; the real part's tree
	MOVL	#1,R1
	MULF3	@AREA.C(AP)[R1],AREA.$F3,R1
	SUBF2	R1,R0
	CVTFD	R0,R0
	ADDD2	@AREA.R(AP),R0
	CVTDF	R0,AREA.$T2
	MULF3	@AREA.C(AP),AREA.$F3,R0     ; the imaginary part's
	MOVL	#1,R1
	MULF3	@AREA.C(AP)[R1],AREA.$F2,R1
	ADDF2	R1,R0
	CVTFD	R0,R0
	MOVL	#1,R2
	CVTDF	R0,@AREA.C(AP)[R2]
	MOVF	AREA.$T2,@AREA.C(AP)
```

(The comments here and in line 7's code above are the tour's; the file
has the source lines.) The
imaginary part of the dummy C is `@AREA.C(AP)[R1]` with R1 holding 1:
`@n(AP)` has no room for an offset, so the 4 bytes go in an index scaled
by F's size. Multiplying by 0.0 and 1.0 and widening a value only to
round it back are left for Phase 84's optimizer: the point now is code
that is right in every type.

## 12. Control flow and subprograms

Phase 79 gave the compiler the rest of the language's chapters 5 and 6:
every way of going somewhere in a unit, and every way of going to another
one. Most of it is a small amount of machinery, because two old decisions
carry the weight. FORTRAN's storage is static and it has no recursion, so
anything a routine must remember between its statements can sit in a cell
of `$LOCAL`; and the VAX has an instruction, or an addressing mode, for
nearly each thing a jump or a call needs. This section follows one
routine through them:

```
      SUBROUTINE SCALE(F, A, N, K, *)
      EXTERNAL F
      INTEGER N, K, I
      REAL A(N), F, DBL, X
      DBL(X) = 2.0*X
      GO TO (10, 20), K
      RETURN 1
10    DO I = 1, N
        A(I) = F(A(I))
      END DO
      RETURN
20    DO WHILE (N .GT. 0)
        A(N) = DBL(A(N))
        N = N - 1
      END DO
      RETURN
      ENTRY RESET(A, N)
      A(N) = 0.0
      END
```

It has a dummy procedure, an alternate return, an adjustable array, a
statement function, a computed GO TO, both newer loops, and a second entry
point. The code below is the compiler's own, with lines left out where the
section says so.

### Jumps: what a block can end in

Section 9's basic blocks end in a terminator, and the jumps of chapter 5
are three more kinds of it beside `Jump`, `Branch`, and `Return`:

| FORTRAN | The IR's terminator | The VAX |
|---|---|---|
| `GO TO (10, 20), K` | `Switch` | `CASEL` and a table of words |
| `IF (X) 10, 20, 30` | `Sign` | `TSTF` and branches on N and Z |
| `GO TO V` | `IndirectJump` | `JMP @V` |

CASEL is the computed GO TO's rule as an instruction. It subtracts its
second operand from its first, compares that, unsigned, with its third,
and if it is in range adds the word at that place in the table after it
to the PC; otherwise it goes on past the table. So the language's "anything
else goes on to the next statement" costs nothing:

```
;    6        GO TO (10, 20), K
	CASEL	@SCALE.$A4,#1,#1
SCALE.$1:
	.WORD	SCALE.L10-SCALE.$1
	.WORD	SCALE.L20-SCALE.$1
;    7        RETURN 1
```

A word reaches 32K bytes either way, as far as the `BRW` every long branch
uses. An `IndirectJump` keeps the blocks it may reach (its label list, or
every label the unit ASSIGNs), though the instruction doesn't use them: a
jump's targets are what the flow analysis of Phase 84 will need to see, and
sema checks them as it checks any jump's. ASSIGN is `MOVAB L10,V`, an
address, which is why V must be a longword; `LabelAddr` is the IR's
expression for it.

**A loop has two shapes.** A DO's count is computed once, before the
variable is set; its end decrements the count. A DO WHILE tests at the top
every time, so it has a block for the test:

```
;   12  20    DO WHILE (N .GT. 0)
SCALE.L20:
SCALE.$6:				; DO (line 12)'s test
	TSTL	@SCALE.$A3
	BGTR	SCALE.$8
	BRW	SCALE.$7
SCALE.$8:
```

The count and step are cells too (`$C1`, `$S1`), not registers, and that
is the whole implementation of the "extended range": a jump out of a loop
and back into it finds them where they were. No message is possible for an
illegal jump in, since only running the program can tell.

### What a routine owes the rest of VMS

A FORTRAN routine is an ordinary procedure of the calling standard: an
entry mask saving the registers from R2 up that it uses, its arguments
through AP, RET at the end, a value in R0. Nothing else makes it one of
FORTRAN's. So a routine can be a condition handler, an AST routine, or what
`LIB$CALLG` calls; `testdata/fortran/callback.for` makes one of each, a
handler's value in R0 saying to continue, and an AST routine reading its
parameter, which the system passes by value, with `%LOC`.

A value of up to 8 bytes comes back in R0 or R0 and R1, COMPLEX\*8
included (the standard says two 32-bit values can't, but MTHRTL's F complex
functions do it, and a FORTRAN function and MTHRTL's must be
interchangeable as arguments). CHARACTER, REAL\*16, and COMPLEX\*16 go
through a hidden first argument, which moves the others along one, the
address of the caller's temporary or for CHARACTER a descriptor of it.
Inside such a function its name is an `ir.Param`, argument 1, so the
function's name needs no new code: it is reached as a dummy is. And that
is how a `CHARACTER*(*)` function learns its length: the descriptor is the
caller's. The caller never passes the assignment's target as the hidden
argument, since in `H = F(H)` the function would overwrite its own argument
as it wrote its name.

**Alternate returns** are a question the manuals leave to the compiler:
what do `*10` and `*20` put in the argument list? Ours is nothing: `CALL
CHECK(A, B, *10, *20, C)` is `CALLS #3`. The routine doesn't need the labels,
only their number, so it leaves the number in R0 and the caller chooses:

```
;    7        RETURN 1
	MOVL	#1,R0			; the alternate return's number
	RET
```

and in the caller, after the CALLS, a `Switch` on `CallValue`, the IR's name
for the R0 a call just left: `CASEL R0,#1,#1` over the labels, which falls
through for 0 or anything out of range, exactly what section 5.9 asks. A plain
RETURN, in a subroutine with a `*` in any list, is `CLRL R0`.

**Dummy procedures** cost one addressing mode. The calling standard's
procedure value on the VAX is the entry mask's address, and CALLS takes the
address of the mask as an operand, so a procedure that is argument 1 is
`CALLS #1,@4(AP)`: the argument longword is the address. A name in an
EXTERNAL statement is passed with `PUSHAB G^F`. A CHARACTER function travels
by a descriptor (Table 6-1) holding that address and the length the passing
unit declares, so a dummy declared `CHARACTER*(*)` has somewhere to find it.

**%VAL, %REF, and %DESCR** change only what goes in a longword of the list:
`Arg.Mode` from sema, Table 6-1 as a function (`notAllowed`), a No an error.
`%LOC` isn't that kind of thing at all: it is an INTEGER\*4 expression.

### ENTRY: one body, several doors

An entry point can give its arguments in any order, so a dummy can't be
reached at a fixed `n(AP)`: A is argument 2 of SCALE and argument 1 of
RESET. Each dummy therefore goes through a cell, and each `.ENTRY` copies the
addresses it was given there before going to its statements:

```
	.ENTRY	SCALE,^M<R2>
	MOVL	4(AP),SCALE.$A1		; F is argument 1
	MOVL	8(AP),SCALE.$A2		; A is argument 2
	MOVL	12(AP),SCALE.$A3	; N is argument 3
	MOVL	16(AP),SCALE.$A4	; K is argument 4
```

```
	.ENTRY	RESET,^M<R2>
	MOVL	4(AP),SCALE.$A2		; A is argument 1
	MOVL	8(AP),SCALE.$A3		; N is argument 2
	BRW	SCALE.$10
```

From there `@SCALE.$A3` is N, whichever door came in, and the body needs
only `argumentPlace` to change. That is why the static storage matters: a
recursive language couldn't keep these in one place. A unit with no ENTRY
keeps `@n(AP)`, so nothing is paid for what isn't used. Every entry point
has one mask, since the body is one; the mask is computed after the whole
routine is generated.

In a function the entry names are one result place, as EQUIVALENCE would
lay them out; if their machine types differ the code remembers which door
it came in by (a cell, `$E`, set at each entry) and a return is a `Switch` on
it.

### Adjustable arrays: a descriptor filled in at the door

`REAL A(N)`'s size is known only on entry, and the manual says a later change
to N changes nothing. So the bounds are computed once, into a class A
descriptor in `$LOCAL`, `$B1`, which holds everything an element's address
needs, and which a whole array passed or printed needs anyway:

```
	MOVAL	@SCALE.$A2,SCALE.$B1+4		; the address
	MOVL	@SCALE.$A3,SCALE.$B1+28		; the upper bound, N
	MOVL	SCALE.$B1+28,SCALE.$B1+20	; so the extent
	MOVL	#-1,R0
	MOVAL	@SCALE.$A2[R0],SCALE.$B1+16	; A0, where A(0) would be
	MULL3	#4,SCALE.$B1+20,SCALE.$B1+12	; the size in bytes
```

(The comments are the tour's.) Line 14 of the routine really does change N,
and the descriptor doesn't care. An element is at A0 plus the subscripts
multiplied out by the extents, A0 being a `Mem.Ptr`, an address computed at
run time:

```
	MOVL	SCALE.$B1+16,R0
	MOVL	@SCALE.$A3,R1
	MOVF	(R0)[R1],SCALE.$T1
```

Each entry point computes the arrays its own list defines: the array and
every dummy in its bounds must be in that list (section 6.1.1.1). RESET's
does, in a block after the routine's statements, which then goes on to the
ENTRY's; putting it there is what keeps code that falls into an ENTRY
statement from running it, since falling in isn't entering.

### Statement functions: a reference is its definition

`DBL(X) = 2.0*X` makes no routine. Each reference is expanded where it
stands, so the compiler has only to answer what X means while the definition's
expression is lowered. The dummy argument's name belongs to the definition:
X may be an array elsewhere in the unit and gives this X only its type. So
sema makes X a symbol of its own, in none of the unit's lists, standing in the
unit's table only while the expression is analyzed (`sema/stmtfunc.go`).

Lowering (`ir/lowerstmtfunc.go`) then gives each such symbol a place in
`lowerer.bound`, where `symbolMem` looks first, and lowers the same expression
once per reference. Every path a variable can take, arithmetic, CHARACTER,
COMPLEX, an argument, works unchanged, and no tree is copied. The place is the
actual argument's own if it is a constant or a variable of the dummy's type
and length; anything else, like `A(N)` here, is computed into a temporary of
the dummy's type first, as an assignment to it would be:

```
;   13          A(N) = DBL(A(N))
	MOVL	SCALE.$B1+16,R0
	MOVL	@SCALE.$A3,R1
	MOVF	(R0)[R1],SCALE.$T1
	MOVL	SCALE.$B1+16,R0
	MOVL	@SCALE.$A3,R1
	MULF3	SCALE.$F1,SCALE.$T1,(R0)[R1]
```

Since a reference is lowered like any expression, the temporaries live to the
statement's end, and the tree it returns can read them. Actual arguments are
lowered before their dummies are bound, so `DBL(DBL(X))` binds this X once at
a time, and a definition can use only those before it, so nothing can be
bound twice at once. Three things need care because the actual may be the very
variable being assigned or passed: `Z = SWAP(Z)` for a COMPLEX Z computes into a
temporary (its parts would otherwise be written as they're read); a CHARACTER
value of another length goes to a temporary of the function's length; and an
argument that is a statement function's value is passed as a copy, since `F(X) =
X` would otherwise hand the routine X itself.

## 13. FTNRTL: the run-time library (`internal/ftnrtl`)

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
8 showed: `FTN$WRITE_SL` with the statement's control block and its
unit (-1, PRINT's), `FTN$IO_ITEM` with each item's descriptor, and
`FTN$IO_END`. Why calls per item rather than one call with an argument
list of every item? Because an item list can hold implied-DO loops,
whose number of items is known only at run time; the compiled code runs
the loop and calls for each.

- **The control block** is a few longwords in the program's `$PDATA`: a
  word of size, a word of flags (which of ERR=, END=, and IOSTAT= the
  statement has, and whether its format is a run-time one), and a
  longword not used yet. Later phases add fields after these and raise
  the size, and FTNRTL reads only what the size says is there, so old
  programs go on working. What changes from one execution of the
  statement to the next, the unit and the format, are the begin call's
  other arguments.
- **The descriptors** are the VAX calling standard's: the item's
  length, its data type (`DSC$K_DTYPE_L`, `_LU`, `_F`, `_D`, `_FC`, `_T`,
  and the rest, one per FORTRAN type), its class
  (S for a scalar, A for a whole array), and its address. The data type
  chooses the format, so FTNRTL needs nothing else to write a value.
- **The statement** (`ftn_statement.mar`) is a context on a stack, a
  block from `LIB$GET_VM` the begin call pushes and `FTN$IO_END` pops,
  holding the unit, the flags, the first error, and the record being
  built: a function called for one item may do I/O of its own, which
  gets a context of its own. `FTN$IO_END` returns the statement's status
  in R0, which a statement with ERR=, END=, or IOSTAT= stores and branches
  on, or signals the FOR condition if it has none of them.
- **The unit** (`ftn_units.mar`) is SYS$OUTPUT for PRINT, TYPE, and
  units -1, -2, and 6, opened at the first statement with RMS's `$CREATE`
  and `$CONNECT`, its FAB and RAB in a writable psect of FTNRTL's own; and
  SYS$INPUT, the same way, for units -3, -4, and 5. `RAT=FTN` asks RMS for FORTRAN carriage
  control: each record's first byte is its carriage control, a blank for
  a new line. That is why every list-directed record starts with a
  blank: the blank isn't printed, but turned into the line's start.
- **The formats** (`ftn_listout.mar`) fill the context's record, 80 bytes
  at most: L2
  from bit 0, I5, I7, and I12 by LIBRTL's `OTS$CVT_L_TI`, a text copied,
  and REAL's `1PG15.7E2` from the decimal digits `OTS$$CVT_F_T_R8` returns
  (REAL\*8's `1PG24.16E2` from `OTS$$CVT_D_T_R8`'s, and so on), FTNRTL
  choosing the fixed or exponent form by the rounded value's magnitude; a
  COMPLEX value is its parts' fields without their blanks, in parentheses.
  A value that won't fit what's left of the record writes the record
  (`$PUT`) and starts the next; the end call writes the last one.

FTNRTL calls LIBRTL for the conversions as any VAX program would, through
LIBRTL's transfer vector; a run-time library built on another is the
normal arrangement on VMS.

## 14. Testing a compiler

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
  FORTRAN to check against); the math functions' values were computed
  apart from govax, to 80 digits, and rounded to each format's bits. A
  program whose results depend on rounding along the way (`numeric.for`'s
  Newton's method, Simpson's rule) checks them against a tolerance or
  prints them rounded, so that its output is the mathematics'. FTNRTL's routines are tested from MACRO
  callers (`TestFTNRTL`, `TestFTNRTLListOutput`), in both of RUN's
  paging modes.
- **Programs of several files.** `TestFortranModules` compiles and
  assembles each file of a directory of `testdata/fortran/modules`
  apart, and links them once per line of the directory's `links.txt`,
  in different orders: whichever module's part of a common block comes
  first, or is longest, or holds the values, the program must print the
  same thing. That is the test of everything the compiler leaves to the
  linker.
