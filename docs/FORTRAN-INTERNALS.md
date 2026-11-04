# Inside govax's FORTRAN

This is a guided tour of govax's FORTRAN compiler and its run-time
library, FTNRTL, for someone who wants to know how they work, or to
write a compiler of their own for the VAX under VMS. It was written with
the compiler, a phase at a time (`docs/PHASE-74 - FORTRAN plan.md` lays out
the program; its Part B holds the decisions this tour refers to), and a
later section may say what an earlier phase added. The user's side is the
manual's chapter 14, "Programming in FORTRAN".

The compiler parses every statement of the language, gives every
declaration its meaning and its storage, and compiles the executable
statements: main programs,
subroutines, and functions of every type, with ENTRY, alternate returns,
adjustable arrays, procedures as arguments, the built-in functions, and
statement functions; every expression in every type (BYTE, LOGICAL and
INTEGER of every length, REAL\*4, \*8 in D or G, and \*16, COMPLEX\*8 and
\*16, CHARACTER), with every intrinsic function; assignment; every IF, DO,
and GO TO, CALL, RETURN, STOP, and PAUSE; and the I/O statements, READ,
WRITE, PRINT, TYPE, ACCEPT, ENCODE, and DECODE, formatted, list-directed,
namelist, and unformatted, at the terminal, to internal files, and to the
files of units 0 to 119, sequential, relative, and indexed, by sequential, direct,
and keyed access, with ERR=, END=, IOSTAT=, and ERRSNS; and the file statements, OPEN
(with USEROPEN=), CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE, DEFINE FILE,
FIND, REWRITE, DELETE, and UNLOCK; and DEC's data extensions, structures and
records, typeless constants, VOLATILE, AUTOMATIC, STATIC, the CDEC$ directives,
and `/ALIGN`; and what a program needs to use VMS: definition modules, null
arguments, handlers, ASTs, and the system subroutines; and indexed files, by
every key (section 22). It has no DICTIONARY. A real program, DUNGEON, builds and plays with it, and its
output has run on a real VAX (section 20).

The tour follows one small program through every pass (section 1), then
takes the passes in order, with a second program for declarations and
storage (section 8), a third for the types and how each chooses its
instructions (section 11), and a fourth for control flow and subprograms
(section 12), a fifth for records and how a layout reaches the code (section
13), then FTNRTL, then what using VMS asks of them both, then the optimizer,
then how the compiler is tested, what it tells the debugger, its listings and
run-time checks, and what a real program and a real VAX found. Section 21 is a
map of the files, with what building this one suggests about building another.
Every dump and MACRO listing before section 16
is `/NOOPTIMIZE`'s, the compiler's plain code, which is what the earlier chapters
explain; `/OPTIMIZE` is the command's default.
Each pass's package also has a `doc.go` that teaches it in more detail;
this tour is the story across them.

## 1. The shape of the compiler

A compiler is a series of passes, each reading what the one before it
produced and handing on something a step closer to machine code:

```
 AVERAGE.FOR
     |
 source ---------- lines into statements: columns, continuations,
     |             comments, INCLUDE
 lex ------------- each statement's kind, and its tokens
 parse ----------- tokens into a syntax tree
 sema ------------ names, types, blocks, labels, storage
 ir -------------- the tree lowered to basic blocks
 opt ------------- improvements to the blocks, under /OPTIMIZE
 codegen --------- VAX instructions, as MACRO-32 text
 opt ------------- improvements to the instructions, under /OPTIMIZE
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
copies those `Listed` into the MACRO file), and the statements.

One qualifier reaches this pass from inside the source. `OPTIONS /EXTEND_SOURCE` moves the
statement field's end for the unit after it, and every other qualifier of that statement can
wait for semantic analysis, but this one decides what the statements are. So `source/options.go`
knows just enough FORTRAN to follow it: what an OPTIONS statement looks like, that any other statement
starts a unit, and that an initial line holding only END ends one. A line's kind doesn't depend on the
field's end, so `readFile` classifies a file's lines first, and each statement's fields are worked
out when it's reached, under the form then in effect.

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
in `2*4HABCD`). The lexer meets the same question and uses the same
rule.

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

`INCLUDE '(module)'` and `'lib(module)'` go to the `Opener`'s other method,
`OpenModule`, since which libraries to search is the command's business: its
`/LIBRARY` files, the `FORT$LIBRARY` logical name, `FORSYSDEF.TLB`, and last the
modules built into govax. The compiler gets a `File` either way, and can't tell a module from a file. A
parenthesis tells the forms apart, as no file specification holds one.

A line remembers whether it's `Listed`: every line of the files compiled, and an
included one only if each INCLUDE above it says `/LIST`, since the language
manual makes `/NOLIST` the default. A library of definitions can be thousands
of lines a program uses a few of, and the MACRO file should stay readable. The
statements don't care; only the code generator's source comments (and in time
a listing) look at it, which is why a program's code still comes out with no
source beside it when its INCLUDE doesn't say `/LIST`.

A line also remembers whether it's from a `Library` module, or a file one includes,
which its statement carries on to semantic analysis. Definitions are a program's to
ignore: VMS has hundreds of system services, and a program that includes the module
naming them all shouldn't have every one looked for by LINK. So a name that an
`EXTERNAL` in a module declares reaches the MACRO file's `.EXTRN` only if the unit
uses it (`Symbol.Linked`), while an `EXTERNAL` of the program's own always does, since
naming a BLOCK DATA unit and not using it is the whole point of that statement.

**The built-in modules** are `internal/fortran/sysdef`. They are ordinary FORTRAN
source, made once from `vmsdef`'s tables when a compilation first asks. A module is a
list of name prefixes, and each name goes to the module with its longest matching
prefix, so `LIB$_NORMAL` is `$LIBDEF`'s and `LIB$K_DCFOPR_AB` `$LIBDCFDEF`'s, and no
name is in two modules. Making them text rather than symbol tables handed to `sema`
means there is nothing special about them past the search: the lexer, parser, and
semantic analysis read `$SSDEF`'s 1,463 lines as they read a program's. A file of
3,000 such statements compiles in a few hundredths of a second, so nothing here needs
to be cleverer than that. `mkforsysdef` writes the same text as a `.TLB`.

`FORTRAN A+B` is `ReadAll`: each file at depth 0 in turn, one `Program`. Nothing
ends a program unit at a file's end, so a unit may run on into the next file.


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
   statement the compiler doesn't compile, DICTIONARY, is still known by name.
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
function: the parser builds a node for every statement of VAX FORTRAN,
and it is semantic analysis that reports one it can't compile, by name,
where it reaches it: `%FORT-E-NOTIMPL, the DICTIONARY statement is not
implemented yet`. Keeping that judgment out of the parser meant the grammar
was finished once, at Phase 76, when the same message named READ and much
else, and each phase after it only gave statements their meaning.

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

A compiler's warnings are a place to see two different kinds of work. Most messages are found on the
way past, and `/WARNINGS=NOGENERAL` only has to stop showing them, which is one filter, not a test in
every pass: `CompileAll` drops them from the list after the passes run (`diag.Warnings.Held`,
`List.Drop`), so the count, `$STATUS`, and a listing all go by what's shown. `DECLARATIONS` and `UNUSED` look
for something, so they run at the end of each unit (`sema/warnings.go`). `DECLARATIONS` is IMPLICIT
NONE's own question (`needsType`) with a different answer: a warning, where IMPLICIT NONE makes
it an error. `UNUSED` needed nothing new, since the cross reference (`sema/crossref.go`) had
already recorded every mention: a name is unused if all its mentions are definitions. Most of the work is
the exceptions, each something a real program does on purpose. An `INCLUDE`d file is shared by units that each
use part of it, a `COMMON` name holds a place in a layout, `EXTERNAL BLOCK` is how a BLOCK DATA unit gets linked,
and `REAL X` may be there only to type a statement function's dummy. It was running every program in
`testdata/fortran` under `/WARNINGS=ALL`, and reading what came out, that found the last of those, and that
`DO 10 WHILE` had never noted its label.

`/STANDARD` is the other way to find something: a walk of its own (`sema/standard.go`), over the syntax tree with
tables of what FORTRAN-77 has, after each unit, leaving the other passes as they are. It works from the tree rather
than the analyzed statements because extensions are mostly a matter of how something is written: an analyzed
`INTEGER*2 I` and an `I` of the same type would look alike. Two things the tree can't say come from elsewhere. Only
analysis can tell `SIND(X)` from an array element, so the one place that notes an intrinsic reference
(`noteIntrinsic`) tells the walk too. And a line's tabs, small letters, and `!` comments are gone by the time there is
a statement, so the first pass notes them as `source.Mark`s, which nothing reads unless asked. The findings are sorted
into the source's order before they're reported, since a walk and those notes each find theirs in their own order.

`/DEBUG` and `/OBJECT` show what writing MACRO saves. The compiler knows nothing of debugger records: `/DEBUG` is a
line or two in the MACRO file's opening (`fortran.Debug`), and `/OBJECT` is the MACRO command called on the file just
written (`assembleFortran`), reading it back through RMS as it would if you'd typed it. What's left for this compiler to do is
choose names and comments a person can read, and the debugger's section of this tour says how far that goes.

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
Every type has its size and alignment here, from the start. That let
declarations and storage be finished at Phase 76, before most types could
appear in an expression: a name of such a type had its storage and initial
values, and drew a message only where a statement first used it, so
Phase 78 had only expressions to add.

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
  through needs no branch at all. The optimizer shortens what this writes
  (section 16); the point here is code that is always right.

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
layout; an included line, when it's listed at all, has a prefix that
starts with its include depth (`;1  12  `). `Generate` writes the module: a call of STARLET's `$DSCDEF` if it
has descriptors, for their `DSC$K_` codes, then for each
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
round it back are left as they are, by lowering and by the optimizer, which
nothing measured has asked to: the point here is code that is right in every type.

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
jump's targets are what the optimizer's flow analysis (section 16) needs to
see, and sema checks them as it checks any jump's. ASSIGN is `MOVAB L10,V`, an
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

## 13. Records, typeless constants, and the frame

Phase 82 added DEC's extensions to the data. None of them needs much new
machinery in the code generator, because every one of them comes down to a place
in memory: an offset, a length, and sometimes a register. This section follows
one routine, and then the same routine laid out differently:

```
      SUBROUTINE LOG(N, R)
      STRUCTURE /ENTRY/
        LOGICAL*1 FLAG /.TRUE./
        INTEGER*4 COUNT
        UNION
          MAP
            REAL*4 VALUE
          END MAP
          MAP
            CHARACTER*3 TAG
          END MAP
        END UNION
      END STRUCTURE
      RECORD /ENTRY/ TABLE(10), R
      TABLE(N).COUNT = TABLE(3).COUNT + 1
      TABLE(N) = R
      END
```

### A layout is a table of offsets

`sema/structure.go` reads a declaration into a `Structure`: its `Field`s with their
offsets, its size, its initial values. Structures have a name space of their own,
so a structure, a field, and a variable may all be called X. V5.0's rule is
*packed*: each field starts where the last ended, which the VAX allows, since it
reaches a longword at any address, only more slowly. A map has no name, so its
fields are the structure's own, and every map starts at the union's offset. The
storage map shows the result:

```
;   TABLE  /ENTRY/ (10)  $LOCAL+0    LOG.TABLE
;   Structure /ENTRY/, 9 bytes
;     +0     FLAG   LOGICAL*1
;     +1     COUNT  INTEGER
;     +5     VALUE  REAL, map 1 of the union at +5
;     +5     TAG    CHARACTER*3, map 2 of the union at +5
```

A record is an ordinary variable whose `Type` has the kind `Record` and the
structure's length, so `$LOCAL`, COMMON, and argument lists needed almost nothing
new. Its initial values are repeated at each element's place:

```
LOG.TABLE:
	.BYTE	-1			; TABLE(1).FLAG = .TRUE.
	.BLKB	8
	.BYTE	-1			; TABLE(2).FLAG = .TRUE.
```

### A field reference is arithmetic on an offset

`TABLE(3).COUNT` has constant subscripts, so `fieldMem` (`ir/lowerfield.go`) folds
it to one number: 2 × 9 + 1 is 19. `TABLE(N).COUNT` has a variable part, and here
the packing costs something. Indexed mode multiplies its register by the operand's
size, 4 for a longword, but the elements are 9 bytes apart, and 9 isn't a multiple of 4.
So the code computes the address, with `MOVAB`, whose operand's size is 1, and stores
through it. The displacement is what's left of the record's first element and the
field's offset, -9 + 1:

```
;   15        TABLE(N).COUNT = TABLE(3).COUNT + 1
	MULL3	@LOG.N(AP),#9,R0
	MOVAB	LOG.TABLE-8[R0],R0
	ADDL3	LOG.TABLE+19,#1,(R0)
```

### An aggregate is a byte move

An aggregate can't be a value: `value` refuses one, so every expression context
refuses it with no case of its own, and the three places that take one (an
assignment, an argument, an I/O list) ask for it specifically. Assignment is
`CharMove` of the length, the same IR statement a CHARACTER assignment uses, which
is written as MOVC3; past 65,535 bytes, MOVC3's limit, in pieces:

```
;   16        TABLE(N) = R
	MULL3	@LOG.N(AP),#9,R0
	MOVC3	#9,@LOG.R(AP),LOG.TABLE-9[R0]
```

An argument is the record's address, as any variable's. Across one file's units
`recordArguments` compares `Structure.form`, each field's place, type, and extents
in order of place, so that a map's order doesn't count, as the manual says it
doesn't, and names don't either. A dummy record's field goes through `ArgPointer`,
since the ordinary dummy place expresses its offset as a count of elements, which a
packed field's needn't be.

### `/ALIGN`: the same routine, laid out again

V5.0 has no choice of layout. A later VAX FORTRAN did, and `sema/align.go` takes
it from the HP manual: an `Alignment` setting, which the command starts each unit
with and `CDEC$ OPTIONS` groups change. Wrapping the structure above in a group
changes only the table of offsets:

```
;   Structure /ENTRY/, 12 bytes
;     +0     FLAG   LOGICAL*1
;     +4     COUNT  INTEGER
;     +8     VALUE  REAL, map 1 of the union at +8
;     +8     TAG    CHARACTER*3, map 2 of the union at +8
```

and the code improves with no change to the code generator, which only ever saw
offsets. The element length, 12, is now a multiple of 4, so the rule from Phase 82's
subtask 3 can use plain indexed mode, with a smaller multiplier:

```
;   17        TABLE(N).COUNT = TABLE(3).COUNT + 1
	MULL3	@LOG.N(AP),#3,R0
	ADDL3	LOG.TABLE+28,#1,LOG.TABLE-8[R0]
```

Three things make alignment harder than it looks. A **union's** place depends on
its largest field, which isn't known until its maps are read, so `unionStatement`
reads them as if the union were at 0, then moves everything in it to the place: a
field's boundary, reached from the union's start, survives the move because the
place is a multiple of every boundary inside. An **offset's alignment is an
address's only if the section's is as strict**, so a common block's `.PSECT` gets
the most any unit's layout needs (`LayoutAlign`, `raiseCommonAlignments`), and the
natural record's `$LOCAL` too. And **`$LOCAL` is one section across a module's
routines**, which MACRO continues from where it was left, and a second `.PSECT` with
other attributes would conflict. So every routine in the module writes the same
alignment (`localAlignment`), and one that needs the boundary starts with `.ALIGN`:

```
	.PSECT	$LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,4
	.ALIGN	4			; the boundary of this routine's aligned records
```

### Constants with no type

A `'FF'X` or `5HHELLO` has no type until it's used (`sema/typeless.go`): beside a
typed operand it takes that one's, and in an assignment the target's, by way of the
bytes memory would hold. So far that is folding. The surprise is a REAL target.
`X = '8000'X` has bits that are a *reserved operand*, on which any floating instruction
faults; so a `Const` keeps its `Raw` bytes, the IR's `RawConst` is never folded, and the
code generator moves them as an integer of the width: `MOVL`, `MOVQ`, or `MOVO`. Since
nothing but a move sees them, nothing can fault. RADIX-50 is only in DATA, so it is just
words in `$LOCAL`; MACRO-32 has no `.RAD50` (that was MACRO-11's), so they are `.WORD`s.

### AUTOMATIC: the only storage FORTRAN keeps on the stack

An AUTOMATIC name gets its `Symbol.Offset` in a frame area, as a name in `$LOCAL` gets
one, and each entry point starts with `SUBL2 #n,SP`. CALLS and CALLG leave SP at FP,
on a longword, so the area is right below FP whatever the entry mask saved. The symbol is
then a displacement, `LOCALS.T = -12`, and the operand `LOCALS.T(FP)`, just as a dummy's
is `SUB.A(AP)`. Two things can't be assembled ahead of time: a descriptor of a frame
variable can't be read-only data, so it is a cell filled in at run time; and a variable
format expression is called *by FTNRTL*, so FP isn't the routine's there. A routine that
needs it keeps its FP in a cell, `$FP`, stored at each entry point, and reaches the
variable through that. Note what this doesn't do: the compiler's own temporaries are
still in `$LOCAL`, so a routine with AUTOMATIC variables still isn't reentrant.

### Directives

`CDEC$` lines reach sema as `DirectiveStmt`s, read where they stand. IDENT becomes the
file's `.IDENT`, a file being one module. PSECT changes a `CommonBlock`'s `Attrs`, which
`commonAttributes` writes, and `checkPsects` compares across the file's units; between files
it is LINK that sees a conflict, and VMS's LINK decides for the first module (`testdata/link82`).

## 14. FTNRTL: the run-time library (`internal/ftnrtl`)

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
  longword not used. A new field would go after these and raise the
  size, and FTNRTL reads only what the size says is there, so old
  programs would go on working. What changes from one execution of the
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
  blank: the blank isn't printed, but turned into the line's start. A
  number's field begins with a blank anyway, and at a record's start that
  blank serves; a text gets one of its own. It looks odd in the source and
  is what VMS's library does, which the probe in `testdata/for80` showed.
- **The formats** (`ftn_listout.mar`) fill the context's record, 81 bytes
  at most, the carriage control and 80 of text: L2
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

### List-directed input

A list-directed READ has no format to say what comes next, so the input says
it, and `ftn_listin.mar` is a small tokenizer. Two things shape it.

- **Records don't matter, but only one is read ahead.** The end of a record is
  a blank, so a value may be on any later record; but a statement must not read
  a record it doesn't need, or the next statement's input is lost. So the
  separator after a value isn't consumed when the value is read: the next item
  looks for it (`CTX_LI_AFTER` says that a comma it finds belongs to the last
  value, where any other comma is a null value), and the last item never reads
  on.
- **A value is converted per item, not per value.** `3*14.6` read into an
  INTEGER, a REAL, and a REAL\*8 is 14, the nearest F_floating value to 14.6,
  and the nearest D_floating one, which aren't conversions of each other. So
  the value's text is kept (in a buffer that grows, since a character constant
  may run across any number of records) and `STORE` converts it for each item,
  into a place in its own frame, so that a value that fails reaches nothing:
  a failed list-directed item keeps its value, where a failed formatted one is
  zeros. An integer goes through D_floating too (`OTS$CVT_T_D`, then `CVTDL`),
  which holds any longword exactly, and is why `1.5E3` is 1500.

The two conditions come from VMS's run: what can't start a number is a syntax
error, `FOR$_LISIO_SYN`, and anything that could but doesn't convert is
`FOR$_INPCONERR`. `TestFor80InputOracle` holds the module to 55 cases of VMS's.

### The format interpreter

A formatted statement, `WRITE (6,10) I, X`, is the same calls with a
different begin call, `FTN$WRITE_SF`, whose third argument is the format:
the FORMAT statement's block, which the compiler puts in `$PDATA` (the
text's address and length, and the address of a longword cell in
`$LOCAL`, and the routine for any variable format expressions), or a
run-time format's descriptor. `ftn_format.mar` does the rest, in two steps a compiler writer will recognize.

- **Parsing, once.** Reading `(1X,I5,2(F8.2,A))` character by character
  for every item would be slow, so the text is first turned into a list of
  fixed-size operations, one per edit descriptor, its numbers already
  read: a small compiler of its own, a recursive-descent parser written
  as a loop with the open groups' operations on the stack. It is the
  grammar of `internal/fortran/format`, the compiler's check of a FORMAT
  statement, written a second time in MACRO, and both are held to the
  formats VMS's run-time library accepted and refused. The parsed list goes
  in a `LIB$GET_VM` block whose address is kept in the FORMAT's cell, so a
  FORMAT is parsed the first time it is used and never again; a run-time
  format's text can change, so it is parsed for each statement and freed at
  its end. The cell is in `$LOCAL`, not `$PDATA`, because it is written.
- **Walking, item by item.** The statement's context holds where the walk
  is: the next operation, how many repeats the current data descriptor has
  left, and a stack of the groups being done with their counts. Each
  `FTN$IO_ITEM` walks forward to the next data descriptor, doing the
  others it passes (text into the record, X and T moving the position, a
  slash ending the record), and edits the item with it. At the format's
  last parenthesis with items still to come, the record ends and the walk
  starts again at the last group at the outer level, *reversion*. The end
  call walks on to the next data descriptor or colon and writes the last
  record. A format whose walk finds no data descriptor at all would loop
  for ever, and is stopped with `FOR$_INFFORLOO`.

The record is built a byte per column, with a position separate from its
length, so that `T` and `TL` can go back over what was written and `X`
can move on without making the record longer. VMS's FORTRAN library was
the model for what the manual leaves open: a record's 133-column limit, a
`T0` that loses what is written there, a `$` changing the first byte to
`$` (a prompt) when the record is written.

### Variable format expressions: FTNRTL calls back

`FORMAT (I<J+1>)` is the one place a run-time library needs the program's
code in the middle of its own work. The width isn't a number the compiler
knows, and it is computed again each time the scan reaches it, so it has to be
code; and J may be a dummy argument, reached through the AP of a routine that is
now some frames up the stack. The compiler turns each FORMAT with such
expressions into a text and a routine:

```
$K1:  .ASCII  /(I<1>)/           the text: <J+1> is now expression 1

VFE.$V10:                        FTNRTL calls F(n, ap)
      .WORD   ^M<R2>             the routine's own mask
      MOVL    4(AP),R0           which expression
      MOVL    8(AP),AP           the AP of the routine doing the I/O
      CASEL   R0,#1,#0
VFE.$12:
      .WORD   VFE.$5-VFE.$12
      RET
VFE.$5:                          (among the routine's blocks)
      ADDL3   #1,@VFE.J(AP),R0
      RET
```

Loading AP is safe because `RET` restores the caller's from the frame, and it
is enough because everything else compiled code uses is static: nothing but a
dummy argument depends on which frame is running. Each begin call keeps its
caller's AP in the statement's context (it is in the begin call's own frame) for
just this. The code is lowered like any statement, with one precaution: it runs
between an I/O statement's calls, when that statement's temporaries, implied-DO
counts, and descriptors are all live, so its temporaries come from a pool of
their own (`ir/lowervfe.go`).

FTNRTL's parser reads `<1>` where a number could be and notes in the operation
which of its numbers is an expression's. The walk calls `RESOLVE` on such an
operation, which copies it into the statement's context and fills the copy in. The
parsed form is left alone, since a statement nested in this one may be using the
same FORMAT, and the editing code, handed the copy, never knows.

### Editing a value

The walk hands each item to the module for its data descriptor; the
integer and logical ones, I, O, Z, and L, are `ftn_edit.mar`'s, given the
context, the operation, and the item's address, length, and type. Output
needs no conversion routine: an integer's decimal digits come from a loop
of `EDIV` by ten, which leaves them last digit first on the stack, and an
O or Z field's digits are bit fields of the item, `EXTZV` three or four
bits at a time from the most significant, so an item of any length (a
REAL, a CHARACTER) is written as the unsigned number its bytes make, as
VMS's library writes it. Input reads a *field*: w characters, but fewer at
the record's end or before a comma, which is how a FORTRAN user can type
`1,-2,,` for three values without counting columns. I accumulates the
value with `EMUL`, whose quadword product shows an overflow; O and Z shift
each digit into the item from its low end, byte by byte, so they too work
for any length.

Two kinds of error matter here. A value too wide for its field is written
as asterisks, and the statement goes on: the condition, `OUTCONERR`, is
kept apart from the error that stops a statement (`CTX_WARNING`), and
reported at its end. An item of the wrong type is edited by its bytes
anyway, and the statement goes on, failing only at its end, after its records
(`CTX_MISMATCH`). Both rules are VMS's library's, found by the probe.

The floating descriptors, F, E, D, and G, are `ftn_float.mar`'s. Writing a
real number correctly is the hard part of a run-time library, and FTNRTL
doesn't do it: LIBRTL's `OTS$$CVT_x_T_R8` gives the value's exact decimal
digits in a buffer in the caller's frame, rounded half up at the place
asked for. (The routine is undocumented, and what it writes and changes had to
be learned twice: section 20.) The module's own work is the layout, and one routine, `LAYOUT`,
serves every form: it is told how many digits go before the point, how many
after, how far the first digit is from the point (`L_SHIFT`, so that a
negative scale factor's or a small value's leading zeros need no case of
their own), and how many columns the exponent or G's blanks hold at the
field's end. Each descriptor just works out those numbers:

    F8.2 of 25.0 with 2P    shift 4    4 digits . 2 digits    2500.00
    E12.3 of 270.139        shift 0    0 digits . 3 digits    0.270E+03
    1PE12.3                 shift 1    1 digit  . 3 digits    2.701E+02
    -1PE12.2                shift -1   0 digits . 2 digits    0.03E+04
    G13.6 of 12.3456789     shift 2    2 digits . 4 digits    12.3457, 4 blanks

G is the instructive one: the form depends on the value *after* rounding
(999.6 under G13.3 rounds up to 1000, so it takes E form), so it asks for
the digits first, looks at the exponent, and for E form asks again, since E
rounds at a different digit. Input is shorter still: the field goes to
`OTS$CVT_T_x` with d, the scale factor, and BN's flag, and the routine's
rules, an exponent overriding the scale factor among them, are FORTRAN's.

### Internal files: the record is the variable

`WRITE (STR, 100) X` looks like the same statement as `WRITE (6, 100) X`, and to the
format interpreter and every editing routine it is. The difference is where a record
goes, so that is the only thing that changes. Each routine reaches the record through
a pointer in the statement's context:

```
	MOVB	(R2),@CTX_RECP(R6)[R4]	; byte R4 of the record
```

`@CTX_RECP(R6)[R4]` reads as: the longword at `CTX_RECP(R6)` is an address, and R4
indexes from it. For a unit that address is the context's own 256-byte buffer, which
goes to RMS when the record ends. For an internal file it is the variable. Nothing
is copied, so a record can be any length, and a statement that fails leaves in the
variable exactly what it had written, which is also what VMS's library does.

What's left is what a record's end means (`ftn_internal.mar`). Output fills the rest
with blanks and moves on by the record's length; an array is simply those records one
after another in memory. Running out differs by form, by VMS's library: a READ gets
an end of file, so END= works, while a WRITE, ENCODE, and DECODE get an error, and
ENCODE's isn't filled first. Those aren't rules you could derive; they are what a probe
of the real library shows, and `for80x.for` holds each to it.

ENCODE and DECODE are the same machinery with a count: one record of that many
characters, at the address of a variable of any type. Their fourth argument is the
count as a longword rather than a descriptor's length, because the real library takes
counts a descriptor's word can't hold.

### NAMELIST: a statement with no items

`READ (5, PARAM)` names no items: the group's names are in the input, in any
order, any subset. So a namelist statement can't be the begin, item, and end calls
the others are; `FTN$READ_SN` and `FTN$WRITE_SN` do the whole statement at the begin
call, and `FTN$IO_END` only ends it. What the compiler gives them is the group's
block in `$PDATA`: the group's name, a count, and for each entity its name and a
descriptor, the same descriptor an item call would have had. With the names at run
time, input becomes a lookup: read a name, find its descriptor, read the subscripts
or substring after it and narrow the descriptor to the place they name, then read
values into it as list-directed input does.

`ftn_namelist.mar` is mostly reuse, which is the point of having built list-directed
I/O in pieces: input takes `ftn_listin.mar`'s exported routines for the blanks
between values, a character constant across records, and a value's text kept for each
item's conversion, and output sends each value through `ftn_listout.mar`'s field
routine with a flag that captures the field rather than placing it in a record, so a
namelist value is written exactly as a list-directed one is. What is new is the
grammar around them (where a group starts and ends, what a word followed by `=` means,
the conditions that name what they found) and VMS's library settled each of those
rules, by a probe of 786 cases (`testdata/nml80`).

### Units and the terminal

A unit (`ftn_units.mar`) is a block from `LIB$GET_VM`, the address kept in a table with
a longword for each number from -4 to 119. It holds a FAB, a RAB, a NAM, an XABFHC, and
what INQUIRE and the statements ask (form, access, blank, disposition), each copied from
a prototype when the unit is set up, because a shareable image can't store addresses at
link time. A unit is connected at its first use by `$CREATE` (or for a READ `$OPEN`) and
`$CONNECT`, never at image start: a program that does no I/O opens nothing. The record
being built or read lives in the unit's buffer, reached through the context's `CTX_RECP`,
as long as the unit's record or the file's longest.

Each unit has its own FAB, as VMS's library has, even -1, -2, and 6 on one SYS$OUTPUT, so
it is RMS and not FTNRTL that remembers a line left open: a prompt written by `TYPE` with
`$` leaves the line open, and the `READ (5,...)` that follows reads the reply on it.

The default units' files are process-permanent (`SYS$OUTPUT` and `SYS$INPUT` exist before
the image does), so `$CREATE` opens nothing new: it gives the FAB indirect access to the
process's own file, the terminal or a procedure's data lines. That is why FTNRTL needn't
know which it has, with two exceptions it asks the FAB's device characteristics about:
a namelist `?` is answered only at a terminal, and an output unit may be read only at
one, as a program like DUNGEON does (elsewhere VMS's library refuses, RMS$_FAC).

`RAT=FTN` makes RMS, not FTNRTL, turn a record's first byte into what the terminal is
sent: a new line, a blank line first, a new page, nothing for `+`, and for `$` no
carriage return after, the cursor left at the prompt's end. govax's terminal is written
a line at a time, so RMS keeps one fact, whether the last line is still open, and ends
it before any record that starts its own line, when the terminal is read, and at
image rundown, so DCL's prompt comes on a line of its own.

### OPEN and CLOSE: a statement of one call

A file statement has no items, so it is one call, and the compiler's arguments
(`ir/lowerfile.go`) are pairs after the unit: a keyword's code and how its value comes,
then the value.

```text
OPEN (3, FILE=NAME, STATUS='OLD', RECL=N)

  call FTN$OPEN($IO1, 3, 1806, <descriptor of NAME>, 1039, 1, 1030, N, 1048, 2)
```

The codes are VMS's library's, found by calling it, so one table (`sema/fileio.go`) runs
from the source's keyword to the run-time library; `1048, 2` is a `BLANK='NULL'` no one
wrote, since VMS's library leaves the LRM's defaults to the compiler. A choice whose value
is a constant is a number here, checked at compile time; one known only at run time is
a descriptor, and `ftn_open.mar` looks its text up in a table of the same numbers, and
applies the same rules. FTN$OPEN is still a statement like any other: it has a context,
and ends by calling FTN$IO_END, so its failures are signaled or returned as every other
statement's are. The one difference is the user PC in the signal, the PC FTN$OPEN returns
to rather than FTN$IO_END's, which the context carries (`CTX_USERPC`).

### INQUIRE: the statement that can't fail

INQUIRE is one call too, `FTN$INQUIRE` (`ftn_inquire.mar`), but with no unit argument,
since it may ask by file: its unit is a keyword, code 1. Its answers are keywords as well,
each followed by where to store it. The compiler knows the place's width and the run-time
library doesn't, so the keyword's mechanism says: a byte, a word, or a longword for a
number or logical, and a descriptor for a text.

```text
INQUIRE (FILE=F, EXIST=L, NUMBER=N2, NAME=T)

  call FTN$INQUIRE($IO1, 1806, <descriptor of F>, 1310, L, 800, N2, 1058, <descriptor of T>)
```

VMS's INQUIRE never reports a failure. A unit that can't exist, or a name that can't be
one, is an answer like any other. So this one statement needs no context and never signals:
R0 is 0, and the compiler's code for IOSTAT= stores it as always. That also lets the routine
work in two steps, which is worth copying: it first works out every answer into a table
with a longword for each code, then walks the keywords and stores what was asked. Which
specifiers a program wrote, and in what order, can't change what any of them says.

Asking by name is a small lesson in RMS: `$PARSE` says whether it can be a name (and a
wildcard can't), `$SEARCH` says whether the file is there, and the resultant name, version
and all, is compared with each connected unit's. Only a file no unit has is opened, shared,
to read its attributes; it is closed again, and a last `$PARSE` given a bad default name ends
any search the first began, which is how VMS's library does it, and why RMS must leave
a NAM's answers alone when it can't read a string.

### REWIND, BACKSPACE, ENDFILE: what RMS can't do for you

The positioning statements (`ftn_position.mar`) are one call each, `FTN$REWIND($IO1, u)`
and the like, begun by the same `FTN$$STMT_FILE` as OPEN, so they too end in FTN$IO_END.
REWIND is RMS's `$REWIND`. ENDFILE is a `$PUT` of one byte, `^Z`, and the reading side
(`FTN$$UNIT_GET`) turns a record of that byte alone into an end of file, though RMS sees
nothing unusual in it; so a READ after it simply goes on.

BACKSPACE is the instructive one. A file of variable-length records can be read forwards
only: each record's length is in front of it, and nothing says where the previous one
began. So the run-time library counts the records before the unit's place (the count it
already kept for `FOR$_INVTEXREC`'s message), and BACKSPACE is a `$REWIND` and a `$GET`
for each record but the last, which is exactly the manual's own description and what VMS's
library does over govax. Two details follow from it: a READ that meets the end of the file
mustn't add to the count, and a file opened with `ACCESS='APPEND'` has no count until a
REWIND gives it one, which is why VMS refuses that BACKSPACE.

### Unformatted records: segments of a record

An unformatted WRITE's items go into the unit's buffer as their bytes, one after
another (`ftn_unformatted.mar`), and a READ takes them out the same way. RMS's
variable-length records are at most 32,767 bytes, and a FORTRAN record has no
limit, so VMS's library cuts a long one into segments, each an RMS record whose
first word says where it falls: 1 the first, 0 a middle one, 2 the last, 3 the
only. The run-time library writes a segment only when it is full and another
byte comes, so a record that fills one exactly is still an only segment, and the
READ side follows the words to a record's end. The unit's record count, which
BACKSPACE relies on, is of these logical records, so BACKSPACE reads a record's
segments back to its last.

### Records by number: direct access and relative files

A direct access statement's record number is the begin call's last argument
(`FTN$WRITE_DU($IO1, 3, R)`), and the unit's RAB reads and writes by key:
`RAB$B_RAC` KEY, with `RAB$L_KBF` pointing into the unit's block, where the
number goes before each `$GET` or `$PUT`. Two kinds of file take it. A
sequential file of fixed-length records is what DEFINE FILE makes, as VMS's
library makes it, the manual notwithstanding: RMS finds record n at n-1 times
the record's size, padded to a word, and a `$PUT` past the end fills the gap
with zeros, which read back as records. A relative file (`rms/relative.go`) is
cells by number, each with a control byte saying whether a record is there, so
a record never written, or deleted, is missing (`ATTACCNON`) rather than zeros,
and a sequential READ passes over the empty cells. Each record a statement moves
takes the next number, so `READ (3'NR)` with an associated variable reads a
file in order, and FIND moves the number without moving data.

### The current record: what REWRITE and DELETE may change

REWRITE and `DELETE (u)` act on the unit's current record, the one its last READ
reached. RMS has a current record of its own, but VMS's library doesn't simply
ask RMS: its runs show a unit with no READ since its last WRITE refusing with
`NO_CURREC`, which is FORRTL's message, while a unit whose READ failed at the
end of the file passes the request to RMS, which refuses it (`DELERR`,
`REWRITERR`). So the unit keeps a flag (`UB_CURRENT`): a READ or FIND that
reached RMS sets it, whatever RMS answered, and a WRITE, a DELETE by number, or
a statement that moves the unit's place clears it. REWRITE is then an ordinary
output statement with one difference, which only the record routine sees: while
`UB_REWRITE` is set, `FTN$$UNIT_PUT` does an `$UPDATE` instead of a `$PUT`. The
formatting, the padding of a fixed-length record, and the error model are all
shared, and `FTN$IO_END` only renames a failed write `REWRITERR`.

Record locks come with relative files on a SHARED unit: RMS locks the record a
`$GET` reads until the stream's next operation, so another unit's READ of it is
RMS$_RLK, which the run-time library reports as `SPERECLOC` at once, as VMS's
does, and UNLOCK is a `$FREE`.

### USEROPEN: handing the FAB to the program

`USEROPEN=name` passes a routine's address where a keyword's value would go
(code 21). FTN_OPEN builds the FAB and RAB exactly as it would for itself, its
own names, sizes, and options, and where it would call `$CREATE` or `$OPEN` it
calls the routine instead, with the FAB, the RAB, and the unit's number. The
routine may change anything before it opens the file, and what it leaves is
what the unit uses: FTNRTL then reads the FAB's record format and size as for any
file. A RAB the routine didn't connect FTNRTL connects, because VMS's library
took such an OPEN, so `FTN$$UNIT_READY` connects only a RAB with no ISI. The
test that holds this to VMS (`TestFor81UserOpenOracle`) uses the probe's own
routine, which prints what it was given: it is how the RAB's options (locate
mode, `ROP LOC`, so `FTN$$UNIT_GET` copies a record RMS left in its own buffer)
and the FAB's block sizes came to be VMS's.

### Errors: one status, three ways to take it

An I/O error is found deep inside a statement: in an item's conversion, in RMS, in
the format's walk. FTNRTL doesn't act on it there. The first error goes in the
statement's context, the remaining item calls do nothing, and `FTN$IO_END` returns it
as R0. Compiled code then does what the statement asked: with END= or ERR=, a branch
on the status (an end of file is one value, `FOR$_ENDDURREA`, and anything else
another); with IOSTAT=, the FOR number stored, -1 at an end of file; with neither,
FTNRTL signals the condition itself before returning. So the compiler's side of every
I/O statement is the same few instructions, whatever went wrong, and the run-time
library alone decides what an error is.

A signaled condition is laid out as VMS's library's, because a user sees its
message: the condition, then FAO arguments for its text (the unit, the file's name,
which RMS returned in the unit's NAM, and the user PC), then RMS's own status and
status value for an error that came from RMS, so `$PUTMSG` adds `-RMS-E-EOF`, then
for a conversion error the text that failed, `FOR$_INVTEXREC`. An error, not severe,
is signaled with LIB$SIGNAL and the program goes on; a severe one with LIB$STOP. Every
failure is also saved, taken or not, for `CALL ERRSNS`, which the compiler turns into
a call of `FTN$ERRSNS` or, for INTEGER\*2 arguments, `FTN$ERRSNS_W`: a routine taking
arguments by reference can't see their lengths, so the compiler, which can, chooses
the routine.

## 15. FORTRAN and VMS

Almost nothing in this section is code generation. A compiler that follows VMS's
calling standard has services, RTL routines, handlers, and AST routines already: `CALL
SYS$HIBER` is a CALLS through an external name the linker finds, and a function given to
LIB$ESTABLISH is a routine with an entry mask, like any other. What VMS asks of a compiler
is a handful of details, and what it asks of a run-time library is a good deal more.

### Definitions come from the same table as everything else

`INCLUDE '($SSDEF)'` is an INCLUDE like any other. `source.Opener` has a method for
modules, and the command's opener (`dclcmd`'s `fortranIncludes`) searches the libraries
named, FORT$LIBRARY, and FORSYSDEF.TLB, in the order the manual's section 14.4 gives. Its last
place is `internal/fortran/sysdef`, which makes FORTRAN text from `vmsdef`, the table of
VMS's names the assembler and LINK use. A module is a list of prefixes, a name goes to the
module with its longest matching prefix so that no name is in two, and each comes out as
`INTEGER*4` and `PARAMETER`. So a module is ordinary source, read by the same passes, and the
compiler knows nothing about it but two facts on each line, which `source` keeps:

- `Line.Listed`: an INCLUDE without `/LIST` leaves its lines out of the MACRO file.
  `$SSDEF` alone would otherwise add about 1,460 comment lines to each unit that includes it.
- `Line.Library`: this line came from a text library. Then `$SYSSRVNAM`'s 200-odd EXTERNAL
  names don't each become a `.EXTRN`: sema's `Symbol.Linked` gives one only to a name
  called or passed. A program's own `EXTERNAL BLKDAT` always gets one, because that's how it
  pulls a BLOCK DATA in from a library.

### Null arguments: the parser couldn't tell

`F(A,,B)` is a syntax error if F is an array and three arguments if it's a routine, and the
parser doesn't know which. So it keeps the empty place as a nil and lets sema refuse it
where the name turns out not to be a routine. It's the usual answer for FORTRAN, whose
grammar doesn't depend on declarations until something forces it to.

### System subroutines: the types choose the routine

A routine taking its arguments by reference can't see their lengths, and `CALL IDATE(M,D,Y)`
must store words or longwords. The compiler can see them, so `sema/system.go` has a table,
and each subroutine picks FTNRTL's routine from its arguments' types: `FTN$IDATE` for
INTEGER\*2, `FTN$JDATE` for INTEGER\*4; `FTN$DATE` for a place with no descriptor, and
`FTN$DATE_T_DS` for a CHARACTER one. As with ERRSNS, a symbol in no unit's table stands for
the routine, so a unit's own `DATE` or an `EXTERNAL DATE` wins without a special case.
`RAN` is just a call of `MTH$RANDOM`, typed REAL\*4 whatever IMPLICIT says.

### What a handler asks of a run-time library

A handler is called by VMS in the middle of whatever FTNRTL was doing, and may do three things
to it: go on, unwind it away, or start another statement. Each has a consequence.

**Going on.** Whether a statement can continue after an error is the library's decision, by
which call it signals with: LIB$SIGNAL returns if a handler continues, and LIB$STOP doesn't.
`SIGNAL_FAILURE` uses LIB$SIGNAL for INPCONERR, OUTCONERR, and FORVARMIS and LIB$STOP for
the rest. INPCONERR comes by a different road, because "going on" has to mean going on *at that
item*: `FTN$$FMT_FAIL` signals it at once when the statement has no ERR= or IOSTAT=
(`FTN$$IO_SIGNAL`), where the other two wait for `FTN$IO_END`. Which is which
was found by running VMS's FORRTL under handlers (`testdata/for83`'s part E), not assumed.

**Being unwound.** FTNRTL keeps a stack of statement contexts, and a handler's `$UNWIND` takes
frames off the VAX stack without telling it. So a context records its frame's FP and saved PC.
A begin call pops any context the current call chain no longer reaches, and `FTN$IO_ITEM` and
`FTN$IO_END` pop those above their own statement. One case can't be told apart: a routine
called again from the same place has the very frame of its gone statement
(DEVIATIONS.md).

**Starting another statement.** A statement on another unit is simply pushed on that stack. On the
same unit it's RECIO_OPE, and the unit's own state stays safe because of it.

### ASTs: every instruction is a boundary

An AST routine runs between any two instructions, and if it does I/O it uses the same stack
and free list as the statement it interrupted. A timer test won't find what goes wrong: one
unrelated line moved `astrace.for`'s first failure past its 2,000 iterations. So
`TestFor83ASTEverywhere` makes a program trace itself (`for83t.for`), with a trace handler
declaring an AST at every instruction, about 8,000 of them, and that found a real one: `FTN$IO_END`
put its context on the free list and only then read its status, so an AST between the two took the
context and the status became the AST statement's. Now a context goes on the free list only once
it's been read, and `HOLD_ASTS` and `RELEASE_ASTS` (`$SETAST`) go around changes to the stack and the
list. That's six more service calls per statement, the price of a library that's safe here.

### STR$ and the rest aren't FTNRTL's

Everything a FORTRAN program may call that isn't about FORTRAN is in Go, in `internal/librtl`
beside the LIB$ routines, usable from MACRO just the same: `STR$COPY_DX` and the rest of the 26
string routines (`str.go`, `strparts.go`, `strsearch.go`) and the condition routines
(`condition.go`, `fixupflt.go`). The test of the whole thing is `testdata/fortran/vmstour.for`,
and each piece is held to VMS's own run (`testdata/for83`, `testdata/str83`).

## 16. The optimizer (`internal/fortran/opt`)

Everything up to here makes code that is right, and takes no trouble over anything
else. `/OPTIMIZE`, the default, makes it fast. The rule is one sentence: a correct
program must print the same. Most of what follows is what "correct" allows.

### Measure first

The plan listed the classic passes in the classic order, constant propagation first.
Measuring first (`PHASE-84 - FORTRAN optimizer.md`, Part A) turned the order round.
The corpus's programs spend 95% of their time in FTNRTL, so it can't measure a compiler;
five small kernels had to be written. In DUNGEON's own code, a fifth of the instructions
were `BRW`, the far half of a branch around one (section 10). Constant propagation
had almost nothing to do, since PARAMETER is a constant in `sema`, and `arith` folds as
it lowers. So the branch peephole went first, and registers before induction variables.
Hand-optimizing the kernels and running them first said what each pass could gain, and
which a real pass had then to reach. The last column here is the survey's hand version,
which the survey took for an upper estimate; three of them were passed:

| Program | Plain | Peephole | CSE | Invariants | Registers | Loop forms | By hand |
|---|---:|---:|---:|---:|---:|---:|---:|
| `matmul` | 3,101,599 | 2,658,800 | 2,223,200 | 1,362,920 | 930,989 | 696,690 | 750,799 |
| `sieve` | 5,785,872 | 4,785,922 | 4,785,922 | 4,785,922 | 3,907,317 | 3,051,278 | 3,073,873 |
| `subs` | 6,876,884 | 6,671,464 | 6,669,464 | 3,866,184 | 2,259,946 | 1,856,936 | 2,463,514 |

### Two lists of passes

`opt.Passes` work on the IR, in order, `ir.Check` after each, so that a pass that breaks a
routine is named at once. `opt.CodePasses` work on the instructions the code generator
chose, the only place to ask how far a branch goes (`codegen.Code`, a list of items that
`Generate` hands to `Improve` before writing any text). `/NOOPTIMIZE` runs neither, and
its output is byte for byte what it was before the optimizer existed: the golden files
in `codegen/testdata` never changed.

Each pass's own file teaches its technique on a small FORTRAN example. In the order they run:

| Pass | File | What it does |
|---|---|---|
| DoLimits | `dolimit.go` | A DO's count goes, its variable and limit ending the loop |
| OneCharacter | `onechar.go` | `MOVC3` and `CMPC3` of one character become byte operations |
| Forward | `forward.go` | Constants carried across blocks; settled branches become jumps |
| Unreachable | `unreachable.go` | What no block goes to goes |
| DeadCode | `deadcode.go` | Stores nothing reads, of the compiler's cells and a function's value |
| DeferredIndexes | `deferred.go` | `@S.$B1+16[R1]`, an A0 read by the addressing mode |
| LoopInvariants | `invariant.go` | Values no trip changes, before the loop |
| CommonSubexpressions | `cse.go` | A value computed twice in a block, once; `ADDF2 X,C[R1]` in place |
| Registers | `registers.go`, `induction.go` | R6 to R11, and indexes stepped |
| Rotate | `rotate.go` | A test at a loop's top, to its bottom |
| Peephole | `peephole.go` | One instruction at a time: `MOVL #0` to `CLRL` |
| Branches | `branches.go` | Short and inverted branches, `SOBGTR`, `AOBLEQ`, `ACBL` |

### The two questions every pass asks

**Where can control go?** `graph.go` is the control flow graph, with every entry, ENTRY's and
variable format expressions' among them, and its dominators. `loops.go` finds natural loops from
back edges, never from DO statements. A GO TO loop is as good as a DO, and the extended range
is just a block in the loop that happens to lie elsewhere in the file. A cycle with a second
way in isn't a natural loop, the graph lists it as `Irreducible`, and no pass touches it. One
find here is worth repeating: the survey took an assigned GO TO to go only to its list, and
the manual's section 5.6.3 says the list "does not affect statement execution".

**What can change this place?** `places.go` makes each `Mem` a `Place`, a region and a byte
range, so that EQUIVALENCE's overlaps and a union's fields are just byte ranges that conflict
(Decision 7). `effects.go` says what a statement reads and writes and what a call may change: an
address-taken local, COMMON, or a dummy. Other locals survive calls, and that rests on a rule of
the language: section 6.2 forbids recursion, so a callee can't reach them.

### What correct allows

The rules worth knowing, since each is a choice a compiler writer has to make somewhere:

- **Variables keep their values** across calls (the note to SAVE, section 4.14). So a variable
  in a register has its home written on each way out of a loop, and a compiler's cell can die
  and a variable can't.
- **Dummies don't alias**, by the standard's rule, which the author chose over safety. Being
  safe would force `B(K,J)` to be reloaded on every store to `C(I,J)`.
- **Faults aren't moved onto paths that had none.** `invariant.go`'s rule ended up stricter than
  the plan's "blocks that run on every trip". In `DO: IF (Z.EQ.0.0) GO TO 90; X(I)=Y/Z`, the
  division is on every trip that goes round, and the program never divides by zero. So what
  can fault moves only if the first trip was sure to reach it, with no call before.
- **ASTs and handlers aren't seen.** VOLATILE (section 4.16) is the program's way to say so,
  and `optasts.for` holds what that means. The analysis can't help, and doesn't pretend to.

### Registers without virtual registers

Most compilers give every value a virtual register and color a graph at the end. This one
has two pools by design (Decisions 2 and 3): R0 to R5 for Sethi-Ullman's expressions within a
statement, which MOVC and CMPC clobber anyway, and R6 to R11 for what lasts longer, which every
entry mask saves, so a value there survives a CALLS. The IR goes on naming cells. A register
is a `Cell.Register` the code generator prints as `R8`, so a dump stays readable and every other
pass goes on working on what it knows.

`registers.go` lists every candidate first, each with its gain, a use counting ten times more per
loop it's in, and rewrites only the winners. So no earlier pass needs a fallback: if a temporary
loses, it just stays in memory, and the earlier passes' cost models say what that would cost.
Five kinds compete: the compiler's cells, for the whole routine by liveness; a variable, for a
loop, copied in before and written back on each exit edge in a block of its own, so that
trips pay nothing; a dummy's address; an address that doesn't change, A0 or an array's base; and
an index that goes up with the loop's variable.

That last one is strength reduction, and it belongs here because it is a loss without a
register: in memory it's an `ADDL2` and a `MOVL`, against one `MULL3`. The VAX's index mode
scales already, so down a column `A(I,K)` is `(R7)[R6]`, and only a stride that isn't the
element's size asks for it: `C(I,I)` with 60 rows is 61 elements a trip.

### A pass that moves nothing

`rotate.go` is the simplest of them, and it was a surprise: the terminators name both ways, so
moving a loop's header below its body changes when nothing runs. Only where it lies changes,
so it needs no analysis of what the test reads. One branch less per trip, for one jump on the
way in. Layout is free to change like this because the IR never falls through implicitly.

### After the program

`AVERAGE`'s `MEAN` loop, from `MEAN.L10`:

```
MEAN.L10:				; DO 10's body
	SUBL3	#1,MEAN.I,R0
	ADDF2	@MEAN.A(AP)[R0],MEAN.MEAN
	INCL	MEAN.I
	DECL	MEAN.$C1		; one pass fewer
	BLEQ	MEAN.$3
	BRW	MEAN.L10
```

becomes

```
MEAN.L10:				; DO 10's body
	ADDF2	-4(R8)[R6],R7
	AOBLEQ	R9,R6,MEAN.L10
```

The `SUBL3` went because the subscript's constant went to the displacement, which needs a dummy's address
in a register (`-4(R8)`, where `@n(AP)` has no room for one). Six instructions with nine
references to memory became two with one.

### What it didn't do

Global common subexpressions and partial redundancy elimination: the survey found no case that
asked for them. Autoincrement: `(R7)+` for `(R7)[R6]` changes no count, and would take a register
per array. Copying a rotated loop's test to its way in, which would let `K = K + 1` GO TO loops
close with `AOBLEQ`. The phase doc's log has these, with what each subtask measured and left.

### FTNRTL gets its turn

A third of DUNGEON's game was in FTNRTL, which no compiler pass reaches. Its character loops
took a byte at a time what the VAX does in one instruction, and hand-tuning them
(`MOVC3`, `MOVC5`, `SKPC`) took a quarter off DUNGEON's startup. A run-time library written in
one language's terms and compiled by nobody still has to be measured like the rest.

### Testing it

Every program runs both ways and must print the same (`optimizerModes`), logging its gain.
DUNGEON's game does too (`TestFortranDungeonBothWays`). The `opt*.for` programs each target one
pass's rules, with values worked by hand. `TestBoundsHold` holds each instruction's size bound to the
distance in MACRO's symbol table, since a branch is shortened only if the bound says it fits,
and MACRO won't lengthen one. And each subtask broke its rules on purpose, one at a time, and
checked that a test failed: a guard that no test notices is a guard nobody knows is there.

## 17. Testing a compiler

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
- **A real run-time library as the requirement.** No VAX FORTRAN compiler was at
  hand, but VMS's FORRTL.EXE was, and MACRO programs can call its statement
  routines directly, a format as text. So FTNRTL's behavior is VMS's library's,
  studied from outside, never its code: `testdata/for80` and `testdata/nml80` ran
  thousands of cases on VMS (and on FORRTL over govax), and generated FORTRAN
  twins of the same cases, `for80.for` among them, hold FTNRTL to those records line
  for line (`TestFor80Oracle`, `TestFor80InputOracle`, `TestNML80Oracle`). A
  program that reads the terminal in `testdata/fortran` has a `name.in` of what is
  typed.
- **Programs of several files.** `TestFortranModules` compiles and
  assembles each file of a directory of `testdata/fortran/modules`
  apart, and links them once per line of the directory's `links.txt`,
  in different orders: whichever module's part of a common block comes
  first, or is longest, or holds the values, the program must print the
  same thing. That is the test of everything the compiler leaves to the
  linker.
- **Another compiler, and the real machine.** `testdata/dungeon86/gfortran` and
  `testdata/ftn86` compare a whole program's output with gfortran's and with VMS's
  own run. Section 20 tells what they found.

## 18. Telling the debugger

A compiler has to tell a debugger what a program's names mean, and on VMS the way is the debug symbol table: records in
the object module, carried by LINK into the image, with a type for each variable. This compiler writes MACRO, and MACRO
writes records only for what it knows: a label over a directive. To MACRO `TOTAL.N` is the number 8, its place in the argument
list, and `TOTALS.X` a longword. Writing FORTRAN's own records instead would have meant a compiler that writes objects, which
Phase 74 chose against (Decision 3).

What the compiler does instead (`codegen/debugtable.go`, under `/DEBUG`) is leave a note, in a file both ends can see:

```
;DBG$ E TOTAL TOTAL
;DBG$ V TOTAL A RF 4 A:1 1:? TOTAL.$B1
;DBG$ V TOTAL N I 4 A:2 - -
```

MACRO sees comments. The debugger, which reads that file anyway to show source lines, finds a variable's type, how to
reach it, and its bounds (`internal/fortran/dbgtable`, a leaf package each side imports, so neither knows the other).
The cost is that the file must be there, the same cost as seeing source at all; the gain is that MACRO and LINK needed nothing
and a program's bytes depend on nothing it wasn't already asked. The alternative was the same table as data in the image, which
would survive losing the file but make `/DEBUG` change the program.

Most of `debugger/fortran.go` is then the calling standard read backwards. A dummy argument's address is in its routine's argument
list, so it needs that routine's frame: the debugger walks the saved FPs (`fortranFrames`), since a routine two calls up has
arguments too. Each frame's saved AP is at `8(FP)`. A CHARACTER dummy's longword is a descriptor's address, and in a unit with ENTRY statements
a dummy's address is in a cell, since the entry points fill those in. An adjustable array's bounds are in the class A
descriptor the entry point filled, so what `EXAMINE A` shows is what this call has.

Statements get labels, `TOTAL.$LINE15`, written among the source's comment lines, an item in the code list (`codegen.Text`)
no optimizer pass looks at. That matters: a debugging qualifier that moved one instruction would make bugs vanish when looked
for. `TestDebugChangesNoCode` compiles every program both ways and takes the directive, the table, and the labels out of the result: what's
left has to be the same file. That test earned its place at once: the first name tried, `$Ln`, was already the optimizer's name for a
loop's limit.

## 19. Listings and run-time checks

**A listing is laid out, not generated.** `internal/fortran/listing` isn't a pass. It takes an `Input` (the source's lines, the
messages, where units start, the headings, the maps' text) and makes pages of it, and `listinginput.go` gathers that input from
whichever passes ran. That shape comes from when a listing is wanted: after errors, when later passes never ran. So the listing
can't be something the last pass writes. A line read twice, through two INCLUDEs, needs its messages under the right reading, which
is why `source.Line` has a `Serial`, and the map is one piece of text (`codegen.StorageMap`) that both files print.

**A cross reference is noted, not computed.** Only semantic analysis knows what a name is, so it notes each mention as it goes:
`lookup` a use, `declare` a definition, `noteAssigned` a value given (`sema/crossref.go`). The trouble is that analysis may look at one
place more than once, evaluating a constant expression twice or trying an input item two ways, so a mention is keyed by its line and
column, and the strongest thing said of it wins. As section 7 says, `/WARNINGS=UNUSED` then needed nothing new.

**`/CHECK=BOUNDS` is one instruction.** The VAX has an instruction for it. This program stores past its array:

```
      PROGRAM CHK
      REAL V(10)
      I = 11
      V(I) = 2.5
```

The store becomes

```
	INDEX	CHK.I,#1,#10,#1,#0,R0
	MOVL	#^X00004120,CHK.V-4[R0]	; 2.5
```

`INDEX` checks its subscript against the two limits and computes `(subscript + index) * size`, here just the subscript, and if the
limits are broken takes the subscript range trap, so the failure is `%SYSTEM-F-SUBRNG` with no run-time library involved. In the IR it
is an expression like any other, `InRange` (`ir/lowerchecks.go`), and it matters that it is. A check that went ahead of the statement,
as a function's call does, would run in

```
      IF (I .GE. 1 .AND. A(I) .GT. 0.0) ...
```

when I is 0, which the branch's `.AND.` exists to prevent. The check must run only when the subscript would have, so it lives inside
the expression that uses the subscript. Subscripts of the form `I+1` check `I` with both limits moved by one, which keeps the element's
address in its usual form.

**The other checks are bits.** `OVERFLOW` sets the IV bit in each entry mask, `^M<IV>`, and the hardware does the rest. A left shift under
it has to be a `ROTL` and a mask, since `ASHL` sets V for bits shifted out, which FORTRAN doesn't call overflow. `UNDERFLOW` is a `BISPSW #^X40` at each entry point. An entry mask holds only for the routine it's on, so each unit keeps its own
checks, even when called from one compiled otherwise.

**The cost is in the optimizer.** A check is an exception the program can see, so `/CHECK` changes what the optimizer may do. Under
`OVERFLOW`, integer arithmetic may fault, so it's treated as division is: `mayFault` and `unsafeToMove` take the unit's setting, folding
goes through `ir.FoldFor`, which leaves an operation that overflows to trap when it runs, and stepped indexes are left alone, since one runs ahead of what the program computes. A check is removed only when `Forward`'s
facts show its value in range. A check that will surely fail stays, so that a program fails the same whichever way it's compiled.

**`/NOF77` is four small tests.** A one-trip `DO` is a flag in `ir`'s `countedLoop`, a test skipped; OPEN's two defaults are what `sema` passes when the
statement says nothing; and `EXTERNAL SIN` meaning the library's function is `sema`'s `libraryExternal`. It takes per-unit settings like the
rest (`OPTIONS /NOF77`), so each of these reads the unit, not the command.

## 20. A real program, and a real VAX

Everything before this was tested against rules: the manual's, as read when writing the expected output by hand, or those of VMS's FORRTL,
studied from outside. Neither tells you whether the compiler's own idea of the program is right, or whether the machine it's been running on
is the real one. Phase 86 had two ways of finding out.

**DUNGEON against gfortran.** DUNGEON is about 11,600 lines of VAX FORTRAN, which builds silently here and has done since Phase 84 first
tried it. The game's text stays out of the repository, because of the notice on its files, so `TestDungeonMilestone` holds only a digest,
and a digest proves nothing about itself: it only says the output hasn't changed. What makes this one worth having is
`testdata/dungeon86/gfortran/compare.py`, which builds the same source with gfortran, each patch only saying in that dialect what VAX FORTRAN
says in its own, and compares the two line by line. `RAN` there is the manual's generator, so combat and the thief follow the same numbers. `randgame.py` writes
random games that use the game's own debugging commands to reach any room, any object, and every table. No line differed, in 22 games and some 48,000 lines.
If you're writing a compiler, look for a second compiler: it is a witness that doesn't share your reading of the manual.

**Everything on VMS.** There's no VAX FORTRAN on the simh VAX, and none is needed, which is the choice of section 1 paying off again: the compiler's
output and FTNRTL's sources are MACRO-32, so VMS's MACRO and LINK can build them. `testdata/ftn86` has them build FTNRTL and every program in the
corpus, compiled both ways, and run them all (212 sections of a log). DUNGEON's game came out the same to the byte, 60,117 bytes on the third round.
That round also settled a worry from section 16: the branch shortening that only `TestBoundsHold` had held to govax's MACRO assembled on the real one.

What's instructive is the first round, of whose 206 runs only 109 matched. In all three rounds' findings, none came from an instruction the code
generator chose. They came from everything around it:

| What VMS showed | Why govax had hidden it |
| --- | --- |
| `OTS$$CVT_x_T_R8` writes whole groups of nine digits, past the size it's given, over FTNRTL's frame and saved PC | govax's emulation of the undocumented routine stopped at the size |
| The same routine changes R0 and R2 to R8, where FTNRTL kept two values in R7 and R8 | The emulation changed only what its authors assumed |
| MACRO refuses `.EXTRN` of a routine the module defines, and LINK then makes no image (48 runs) | Our MACRO accepted it |
| A faulting `CLRL` leaves the condition codes alone | The CPU set Z before the store faulted |
| A virtual `$QIO` with no file accessed is refused, with no AST, and a program waited for ever | Ours accepted it and completed it with an error |
| `$EXIT` in an exit handler ends the image at once, with no more handlers | Ours went on to the next |

The pattern is the lesson. A compiler tested on an emulator inherits the emulator's forgiveness: every place the emulation was kinder than the machine was a
place FTNRTL's bug could live unseen, and an emulated routine called through a contract nobody wrote down is the kindest of all. So the fix came in two halves.
FTNRTL was corrected, and the emulation was made as unforgiving as the real routine, so that the same mistake can't hide again: it now signals `SS$_BUFFEROVF` for a size VMS's would overrun. `go test` found a second caller that way at once, and a REAL\*16 buffer one byte short.

The comparison had its own failures, too. A script's text mode hid a `\r` that VMS's traceback heading has, and `grep` passed over matches in a log it took for binary. What's still different is kept in
one list, `ftn86VMSDifferences`, each with its reason: a service govax lacks, a timing, and the last bits of negative powers and complex functions, where six candidate orders of operations were fitted to VMS's 408 values and none explained them.

## 21. A map, and some advice

| To see | Look in |
| --- | --- |
| How the command finds files, and runs MACRO | `internal/dclcmd/fortran.go` |
| The passes in order | `internal/fortran/compile.go` |
| A message's text and severity | `internal/fortran/diag/messages.go` |
| Lines into statements, INCLUDE | `internal/fortran/source` |
| Statement kinds, the IF and DO tests | `internal/fortran/lex` (`lex.go`, `kinds.go`) |
| A statement's grammar | `internal/fortran/parse` |
| A statement's meaning, its errors | `internal/fortran/sema` (`decl.go`, `stmt.go`, `io.go`, `fileio.go`) |
| Storage: arrays, COMMON, EQUIVALENCE, DATA, records | `sema/arrays.go`, `common.go`, `equivalence.go`, `data.go`, `structure.go` |
| Statements into blocks | `internal/fortran/ir` (`lowerstmt.go`, `lowerio.go`, and the other `lower*.go`) |
| Choosing instructions | `internal/fortran/codegen` (`expr.go`, `cond.go`, `routine.go`, `chars.go`) |
| The optimizer's list of passes | `internal/fortran/opt` |
| A FORMAT's grammar | `internal/fortran/format` |
| Definition modules | `internal/fortran/sysdef` |
| Listings; the debugger's table | `internal/fortran/listing`; `internal/fortran/dbgtable` |
| The run-time library | `internal/ftnrtl/mar/*.mar`, `ftnrtl.opt`, and `go generate ./internal/ftnrtl` |
| Math routines | `internal/mthrtl`, Go, not MACRO |
| Programs and expected output | `testdata/fortran`, `testdata/for80`, `testdata/for81`, `testdata/for88`, `testdata/ftn86` |
| Indexed files under FTNRTL | `ftn_open.mar`'s `KEYS`, `ftn_keyed.mar`, `ftn_units.mar`'s `UB_KEYED`; RMS's `internal/rms/indexed*.go`, `internal/isam` |
| Why something is as it is | `docs/PHASE-74` to `PHASE-86`, and `PHASE-88` for indexed files, each with its decisions and log |

If you're writing a compiler for this machine, this one's history suggests a few things:

- **Write assembly language.** Everything MACRO and LINK do correctly is something you needn't, the output can be read, and it runs unchanged on the real system, which is what made section 20 possible.
- **Follow one small program through every pass first** (Phase 75), and add language after. Each pass having a text `Dump` makes a change a readable diff.
- **Name what isn't done.** A message naming the missing statement, from one place, shows what's left, and a list of what's refused is more use than a program that fails oddly.
- **Follow the calling standard exactly,** and the system comes free: handlers, ASTs, services, and RTL routines needed almost no code generation (section 15).
- **Measure before optimizing.** Passes were chosen from counts on real programs, and some standard ones were left out because nothing asked for them (section 16).
- **Find a witness outside your own reading:** a real run-time library to call, another compiler, and in the end the real machine. Every one of them found what the hand-written expected output couldn't, since the same person wrote both.

## 22. Indexed files (Phase 88)

The FORTRAN program left one part of the language out: the keyed statements, since RMS had no indexed
files. Phase 88 gave RMS its indexed organization first (`internal/isam`, the format in plain Go, held
to VMS's own files to the byte; `internal/rms/indexed.go` and `indexedrecords.go` its services), and
only then came back to FORTRAN. Most of the work is in FTNRTL, little in the compiler.

**The compiler** has two jobs. A keyed READ's specifier is which of the eight was written, passed as a
number (`sema/io.go`'s `keyMatches`; FORRTL's numbers, 0 for KEY and KEYEQ to 6 for KEYLT), with the
value by descriptor and KEYID= by value, -1 for none. Whether KEYGE suits the key can't be known until
the file is open, so the check is FTNRTL's. `KEY=` on OPEN is checked where its bounds are constants
(`sema/keylist.go`) and lowered to a block the statement fills in when it runs (`ir/lowerfile.go`'s
`keyListBlock`): a word, a count of longwords, and for each key its type, first byte, and last byte.
That block's layout isn't the manual's; it's what VMS's FORRTL takes, found by calling `FOR$OPEN`
in a loop over thousands of shapes until one made an XABKEY (`testdata/for88`'s README). FTNRTL could
have had its own form, but taking FORRTL's let one probe check both libraries.

**FTNRTL's OPEN** (`ftn_open.mar`'s `KEYS`) builds an XABSUM and an XABKEY per key on its own stack,
chains them to the unit's FAB, and cuts the chain before it returns, since the stack goes. For a file
that's there, `$OPEN` fills the XABKEYs in, and each key KEY= gave is compared with them.

**A keyed READ** (`ftn_keyed.mar`'s `FTN$$KEY_START`) checks in the order VMS's did, each step seen in
what it left in the RAB: the unit, its access, the key's number into KRF, the value into KBF and KSZ,
then the direction. Knowing the direction needs the key's descriptor, so it does a `$DISPLAY` through
a copy of the FAB on the stack: `$DISPLAY` writes its FAB, and VMS's READ left the unit's alone. Then
`$GET` by key, and the unit remembers it read by key (`UB_KEYED`), so a READ with no key goes on in
that key's order. A WRITE is a `$PUT` by key; RMS's `RMS$_DUP` and `RMS$_CHG` become `INCKEYCHG`.

**What tested it.** `testdata/for88` ran 310 cases on VMS's FORRTL; `for88f.mar` passes the same calls
to FTNRTL, so one program holds FTNRTL to VMS's answers case by case and file by file. Then
`testdata/vms88` had VMS judge govax's files, and that found what nothing else had: govax put a record
past a bucket's last *after* the bucket's RRVs, where VMS's RMS, reading in sequence, never looks. Every
key but the primary still found the record, and VMS's own ANALYZE/RMS_FILE/CHECK passed the file. Only
VMS's RMS walking it in order, which `vms88w.mar` did, showed 65 records of 2,000 missing. The lesson is
section 20's again, one level down: a checker built from your own reading of the format agrees with
your writer, and only the real reader of the format says what the format is.

