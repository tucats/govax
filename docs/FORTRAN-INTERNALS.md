# Inside govax's FORTRAN

This is a guided tour of govax's FORTRAN compiler and its run-time
library, FTNRTL, for someone who wants to know how they work, or to
write a compiler of their own for the VAX under VMS. It was written with
the compiler, a phase at a time (`docs/planning/PHASE-74 - FORTRAN plan.md` lays out
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
every key (section 22); and recursive routines, safe for ASTs and handlers to enter
too (section 23). It has no DICTIONARY.

Fortran 90 is in the same compiler, beside VAX FORTRAN rather than in place of it (`/F90`, the
default for free form, settles only what the two languages read differently): free source form
and Fortran 90's spellings (section 24), its declarations and kinds (section 25), its control
constructs (section 26), arrays as values (section 27), procedures with explicit interfaces
(section 28), dynamic arrays: descriptors, allocatable, automatic, and assumed-shape arrays, and
array-valued functions (section 29), derived types (section 30), modules and separate compilation
(section 31), pointers (section 32), the intrinsic procedures (section 33), and its I/O (section
34), with the compiler's tools (`/STANDARD`, warnings, listings, `/CHECK`, the debugger, and the
optimizer) taught the new language (section 35). Fortran 95's additions, FORALL, PURE and
ELEMENTAL procedures, `NULL()`, default initialization, and WHERE inside WHERE, are compiled under
the same `/F90` (section 37). What it leaves out is a short list of corners, each refused by name
(the manual's 14.13, "What Fortran 90 isn't here").

A real program in each language builds and runs, and has run on a real VAX: DUNGEON, about
11,600 lines of VAX FORTRAN (section 20), and fcalc, a scientific calculator of about 11,000 lines
of Fortran 90 in sixteen modules, written for the purpose (section 36).

The tour follows one small program through every pass (section 1), then
takes the passes in order, with further programs for declarations and storage
(section 8), the types and how each chooses its instructions (section 11),
control flow and subprograms (section 12), and records (section 13). Then come
FTNRTL, what using VMS asks of compiler and library, the optimizer, testing,
the debugger, listings and checks, and what a real program and a real VAX
found. Sections 24 to 35 add Fortran 90 a part at a time, each built on the
passes as sections 1 to 23 leave them, section 36 is a whole Fortran 90
program, and section 37 adds Fortran 95. The contents below list every section.
Every dump and MACRO listing before section 16
is `/NOOPTIMIZE`'s, the compiler's plain code, which is what the earlier chapters
explain; `/OPTIMIZE` is the command's default.
Each pass's package also has a `doc.go` that teaches it in more detail;
this tour is the story across them.

## Contents

Each section stands mostly on its own, but the later ones assume the passes of sections 1 to
10; a reader who wants only Fortran 90 can start at section 24 after section 1. Section 21 maps
each topic to the files that hold it.

**The compiler, pass by pass** (AVERAGE, a small program, followed through every pass)

- [1. The shape of the compiler](#1-the-shape-of-the-compiler)
- [2. The driver: `internal/fortran`](#2-the-driver-internalfortran)
- [3. Messages: `internal/fortran/diag`](#3-messages-internalfortrandiag)
- [4. The first pass: source forms (`internal/fortran/source`)](#4-the-first-pass-source-forms-internalfortransource)
- [5. The second pass: lexing (`internal/fortran/lex`)](#5-the-second-pass-lexing-internalfortranlex)
- [6. The third pass: the syntax tree and parsing (`ast`, `parse`)](#6-the-third-pass-the-syntax-tree-and-parsing-ast-parse)
- [7. The fourth pass: semantic analysis (`internal/fortran/sema`)](#7-the-fourth-pass-semantic-analysis-internalfortransema)
- [8. Declarations and storage](#8-declarations-and-storage)
- [9. The fifth pass: the IR and lowering (`internal/fortran/ir`)](#9-the-fifth-pass-the-ir-and-lowering-internalfortranir)
- [10. The last pass: code generation (`internal/fortran/codegen`)](#10-the-last-pass-code-generation-internalfortrancodegen)

**The language, compiled** (how each part of VAX FORTRAN becomes instructions)

- [11. Every type and its instructions](#11-every-type-and-its-instructions)
- [12. Control flow and subprograms](#12-control-flow-and-subprograms)
- [13. Records, typeless constants, and the frame](#13-records-typeless-constants-and-the-frame)

**The run-time library and VMS**

- [14. FTNRTL: the run-time library (`internal/ftnrtl`)](#14-ftnrtl-the-run-time-library-internalftnrtl)
- [15. FORTRAN and VMS](#15-fortran-and-vms)

**Making it good, and knowing it is**

- [16. The optimizer (`internal/fortran/opt`)](#16-the-optimizer-internalfortranopt)
- [17. Testing a compiler](#17-testing-a-compiler)
- [18. Telling the debugger](#18-telling-the-debugger)
- [19. Listings and run-time checks](#19-listings-and-run-time-checks)
- [20. A real program, and a real VAX](#20-a-real-program-and-a-real-vax)
- [21. A map, and some advice](#21-a-map-and-some-advice)

**Later additions to VAX FORTRAN**

- [22. Indexed files](#22-indexed-files-phase-88)
- [23. Recursion and reentrancy](#23-recursion-and-reentrancy-phase-89)

**Fortran 90** (a phase per section, each ending with what its programs found)

- [24. Two source forms and a bigger alphabet](#24-fortran-90-two-source-forms-and-a-bigger-alphabet-phase-99)
- [25. Scopes, declarations, and kinds](#25-fortran-90-scopes-declarations-and-kinds-phase-100)
- [26. Control constructs](#26-fortran-90-control-constructs-phase-101)
- [27. Arrays as values](#27-fortran-90-arrays-as-values-phase-102)
- [28. Procedures and interfaces](#28-fortran-90-procedures-and-interfaces-phase-103)
- [29. Descriptors and dynamic storage](#29-fortran-90-descriptors-and-dynamic-storage-phase-104)
- [30. Derived types](#30-fortran-90-derived-types-phase-105)
- [31. Modules and separate compilation](#31-fortran-90-modules-and-separate-compilation-phase-106)
- [32. Pointers and aliasing](#32-fortran-90-pointers-and-aliasing-phase-107)
- [33. The intrinsic procedures](#33-fortran-90-the-intrinsic-procedures-phase-108)
- [34. I/O](#34-fortran-90-io-phase-109)
- [35. The compiler's usability](#35-fortran-90-the-compilers-usability-phase-110)

**A whole Fortran 90 program**

- [36. A whole program: fcalc](#36-a-whole-program-fcalc-phase-111)

**Fortran 95**

- [37. Fortran 95](#37-fortran-95-phase-112)

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
  constant is its bits in `$PDATA` (`$KF1`; a longword for F_floating,
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
  D, G, or H constant is a datum in `$PDATA` (`AREA.$KF1` below, 4D0), and
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
	MULD3	AREA.$KF1,AREA.$T1,R0    ; 4D0 times it, into R0:R1
	MULD3	@AREA.R(AP),@AREA.R(AP),R2  ; R**2, into R2:R3
	MULD3	R0,R2,@AREA.A(AP)
;    6        C = C*(0.0, 1.0) + R
	MULF3	@AREA.C(AP),AREA.$KF2,R0     ; the real part's tree
	MOVL	#1,R1
	MULF3	@AREA.C(AP)[R1],AREA.$KF3,R1
	SUBF2	R1,R0
	CVTFD	R0,R0
	ADDD2	@AREA.R(AP),R0
	CVTDF	R0,AREA.$T2
	MULF3	@AREA.C(AP),AREA.$KF3,R0     ; the imaginary part's
	MOVL	#1,R1
	MULF3	@AREA.C(AP)[R1],AREA.$KF2,R1
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
recursive language couldn't keep these in one static place (a recursive unit keeps
them in its frame, section 23). A unit with no ENTRY
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
sema makes X a symbol of its own, in none of the unit's lists: the definition
is a scope of its own (`sema/scope.go`) whose only names are its dummy
arguments and whose host is the unit's, so every other name in the expression
is the unit's (`sema/stmtfunc.go`).

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
	MULF3	SCALE.$KF1,SCALE.$T1,(R0)[R1]
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
format expression is called *by FTNRTL*, so FP isn't the routine's there. FTNRTL noted the
FP of the routine doing the I/O when the statement began, and passes it, and the expression's
code reaches the variable as a displacement from that, in R11, which the routine keeps from
its other uses (`LOCALS.T(R11)`). Note what this doesn't do: the compiler's own temporaries are
still in `$LOCAL`, so a routine with AUTOMATIC variables still isn't reentrant; a RECURSIVE
one is (section 23).

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

VFE.$V10:                        FTNRTL calls F(n, ap, fp)
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

Loading AP is safe because `RET` restores the caller's from the frame. In a routine
with static storage that is enough, nothing but a dummy argument depending on which
frame is running. In one with a frame, AUTOMATIC or RECURSIVE, the code also loads the
third argument, `MOVL 12(AP),R11`, and a frame cell is a displacement from R11 in these
blocks: FP has to stay the expression's routine's, for `RET` and a signal's search. Each
begin call keeps its caller's AP and FP in the statement's context (they are in the begin
call's own frame) for just this. The code is lowered like any statement, with one precaution: it runs
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
rules, by a probe of 786 cases (`testdata/nml80`). Fortran 90's objects of derived
types later added one kind of entity, a header for an object whose components follow
(section 30).

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
Measuring first (`planning/PHASE-84 - FORTRAN optimizer.md`, Part A) turned the order round.
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
the language: section 6.2 forbids recursion, so a callee can't reach them. (A RECURSIVE unit's
static locals are the exception, section 23.)

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
  own run. Section 20 tells what they found, and section 36 what the same two witnesses
  found in a Fortran 90 program.

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
| Recursive storage: the rule; one constructor for cells | `sema/storage.go`'s `recursiveStorage`; `ir/lower.go`'s `Routine.NewCell` |
| Free form; Fortran 90's tokens and statements | `source/free.go`; `lex/tokenize.go`, `lex/f90stmt.go`, `parse/f90.go`, `sema/notimpl.go` |
| Scopes; attributes; kinds; the inquiry functions | `sema/scope.go`; `sema/attrtable.go`; `sema/kinds.go`; `sema/inquiry.go`, `sema/intrinsickeys.go`, `vaxfloat/model.go` |
| Construct names, EXIT and CYCLE, SELECT CASE | `sema/constructs.go`, `sema/selectcase.go`; `ir/lowerloops.go`, `ir/lowerselect.go` |
| Arrays as values: shapes, overlap, the scalarizer, WHERE | `sema/sections.go`, `shape.go`, `overlap.go`, `elements.go`, `where.go`; `ir/lowerarray.go`, `lowerwhere.go`, `lowerctor.go` |
| Internal procedures and the host's storage | `sema/internal.go`; `parse/units.go` |
| Interfaces, keyword and OPTIONAL arguments, INTENT, generics, operators | `sema/interface.go`, `optional.go`, `intent.go`, `generic.go`, `operators.go`; `ir/loweroptional.go` |
| Descriptors: allocatable, automatic, assumed-shape, sections to old dummies, array values | `sema/allocate.go`, `shaped.go`, `sequence.go`, `arrayvalue.go`, `transform.go`; `ir/lowerallocate.go`, `lowerautomatic.go`, `lowershaped.go`, `lowersequence.go`, `lowertemps.go`, `lowervalue.go`; FTNRTL's `ftn_alloc.mar` |
| Derived types: layouts, values, results, I/O | `sema/derived.go`, `structvalue.go`, `seqtypes.go`, `derivedio.go`; `ir/lowerstruct.go`, `lowerresult.go`, `lowerfield.go` |
| Modules: the text a module leaves, USE, access | `sema/module.go`, `modwrite.go`, `modread.go`, `use.go`, `access.go`; `dclcmd/fortranmodules.go` |
| Pointers and what may alias them | `sema/pointer.go`, `pointercomp.go`, `pointerresult.go`, `pointerclass.go`; `ir/lowerpointer.go`, `locate.go`; `opt/places.go`'s `PointerRegion` |
| The intrinsic procedures' table, and the forms a reference takes | `sema/intrinsics.go`, `constfuncs.go`, `charfuncs.go`, `model.go`, `transfer.go`, `intrinsicsub.go`; `ir/lowercharfuncs.go`, `lowermodel.go` |
| Fortran 90's I/O under FTNRTL | `ir.ControlF90`; `ftn_format.mar`, `ftn_edit.mar`, `ftn_float.mar`, `ftn_statement.mar`'s `ITEM_NCA`, `ftn_units.mar`'s `UB_OPENREC`, `ftn_namelist.mar` |
| `/STANDARD=F90`, obsolescent features, warnings, `/CHECK=POINTERS` | `sema/standard.go`, `sema/warnings.go`; `ir/lowerguard.go` |
| Fortran 90 programs and expected output | `testdata/fortran90` (and its `modules/`), `testdata/fortran/gfortran/compare.py`'s `PROGRAMS90`, `testdata/f90r` |
| A whole Fortran 90 program | `internal/fcalc/src` (fcalc's sources), `internal/fcalc/build`, `testdata/fcalc` (its scripts); section 36 |
| Fortran 95: prefixes, FORALL, PURE, ELEMENTAL, NULL(), default initialization, nested WHERE | `lex/f95stmt.go`, `parse/f95.go`; `sema/forall.go`, `forallorder.go`, `pure.go`, `elemental.go`, `null.go`, `defaults.go`, `where.go`; `ir/lowerforall.go`, `lowerdefaults.go`, `lowerwhere.go`; section 37 |
| Why something is as it is | `docs/planning/PHASE-74` to `PHASE-86`, `PHASE-88` for indexed files, `PHASE-89` for recursion, and `PHASE-99` to `PHASE-111` for Fortran 90, `PHASE-112` for Fortran 95, each with its decisions and log |

If you're writing a compiler for this machine, this one's history suggests a few things:

- **Write assembly language.** Everything MACRO and LINK do correctly is something you needn't, the output can be read, and it runs unchanged on the real system, which is what made section 20 possible.
- **Follow one small program through every pass first** (Phase 75), and add language after. Each pass having a text `Dump` makes a change a readable diff.
- **Name what isn't done.** A message naming the missing statement, from one place, shows what's left, and a list of what's refused is more use than a program that fails oddly.
- **Follow the calling standard exactly,** and the system comes free: handlers, ASTs, services, and RTL routines needed almost no code generation (section 15).
- **Measure before optimizing.** Passes were chosen from counts on real programs, and some standard ones were left out because nothing asked for them (section 16).
- **Add a language beside the old one, and hold the old one still.** `TestNoF90Unchanged` held every fixed-form program's MACRO file to a digest through all twelve Fortran 90 phases, which worked because each new form reached an old path rather than a new one beside it (section 25).
- **Write a real program in the language** if none is at hand, and compile it every way you can. A phase's tests check its features; a program checks their combinations (section 36).
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


## 23. Recursion and reentrancy (Phase 89)

Section 12 said FORTRAN's storage is static and it has no recursion, and much of the compiler
leans on that: anything a routine must remember between its statements, a temporary, a DO
count, ENTRY's argument cells, a descriptor, can be a cell of `$LOCAL`, its address known when
the code is assembled. Phase 89 took recursion from a later DEC FORTRAN (the HP manual's 5.3 and
8.5.1.1): `RECURSIVE`, `RESULT`, and `/RECURSIVE`. This section is what that cost, and the one
idea that kept the cost small: **a storage class is a property of a routine, decided in one
place, and every pass already asks where a cell is rather than assuming.**

### Two storage classes, and who decides

There are three kinds of storage in a compiled routine: static (`$LOCAL`, COMMON's psects,
`$PDATA`), the argument list (`n(AP)`), and the frame (`-n(FP)`). Phase 82 gave AUTOMATIC the
frame: each entry point says `SUBL2 #n,SP`, and since CALLS leaves SP at FP the area sits right
below FP whatever the entry mask saved, so a frame variable is a displacement from FP, as a dummy
is one from AP. Recursion needed no new mechanism, only two decisions about who goes there.

**The program's variables** are sema's to place, and the HP manual's 5.3 gives the rule: in a
recursive unit every local is automatic unless it is SAVE'd, given an initial value, in COMMON, or
STATIC. `sema/storage.go`'s `recursiveStorage` applies it after every declaration has been read,
since any statement of the unit may SAVE or initialize a name, and marks the rest `Automatic`, so
`layout` puts them in the frame as it would an AUTOMATIC statement's names. One case the manual
doesn't settle is an EQUIVALENCE group with one SAVE'd member: storage can't be in two places, so
the whole group is static.

**The compiler's cells** are the IR's, and they all come from one constructor:

```go
func (r *Routine) NewCell(name string, kind CellKind, t Type, size int) *Cell {
	c := &Cell{Name: name, Kind: kind, T: t, Size: size}

	if r.Reentrant && kind != FormatCell {
		c.Frame, c.Offset = true, r.FrameSize
		r.FrameSize += (size + 3) &^ 3
	} else {
		c.Offset = r.LocalSize
		r.LocalSize += (size + 3) &^ 3
	}
	...
```

Temporaries (`$T`), DO counts and steps (`$C`, `$S`), descriptors (`$D`), ENTRY's argument cells
and selector (`$A`, `$E`), adjustable arrays' descriptors (`$B`), NAMELIST blocks, the optimizer's
temporaries (`$U`, `$H`), and the code generator's spill cells (`$X`) all go through it, so making a
routine reentrant is `Routine.Reentrant = Unit.Recursive` and nothing else in lowering. `ir.Check`
refuses a reentrant routine with a compiler's cell in `$LOCAL`. The code generator's spill cells
are made after lowering, so it puts them in the frame itself, counting every FP displacement from
the frame's final size. The one cell left static is a FORMAT's cache,
`$FCn`: it holds FTNRTL's parsed form of a constant FORMAT, the same for every activation, so
sharing it is right, as long as two activations can't both fill it (below).

Section 12 said a recursive language "couldn't keep these in one place", about ENTRY's argument
cells. It can, given a frame: `@SCALE.$A3(FP)` is N whichever door came in, *in this activation*.
The trick needed one place per activation, not one place.

### What a frame can't do: be assembled

A static descriptor is data, written by `.LONG` with its address already in it. A frame
descriptor is filled in at run time, and the question is when. Writing every constant longword at
each entry point is the obvious answer and wastes the work in a routine that only sometimes does
I/O, so a frame descriptor is written whole at each use (`Descriptor.Fixed`, Decision 5): one
`MOVL` of the length, type, and class together, then the address:

```text
	MOVL	#17694720,COUNT.$D1(FP)	; the descriptor's length, 0, type, and class
	MOVW	@COUNT.PATH(AP),COUNT.$D1(FP)
	MOVL	COUNT.PATH(AP),R0
	MOVAB	@4(R0),COUNT.$D1+4(FP)
```

The same holds for everything FTNRTL is handed by address: a NAMELIST group's block is built in the
frame by each statement that uses it (`ir/lowerio.go`'s `namelistCell`), and an associated
variable (`ASSOCIATEVARIABLE=`) in the frame gets a warning, ASSOCFRAME, since FTNRTL keeps the address
past the statement, and possibly past the routine's return.

### Code that runs in someone else's frame

A variable format expression is a routine FTNRTL calls in the middle of a statement (section 14,
"Variable format expressions: FTNRTL calls back"), so FP there is the expression's routine's own, not the activation doing the
I/O. FTNRTL notes the FP of the routine that began the statement and passes it as a third argument;
the expression's code loads it into R11 and reaches every frame cell as a displacement from R11
rather than FP (`routineGen.frameBase`). That was also a fix for AUTOMATIC, whose old `$FP` cell, a
static copy of the frame pointer, was wrong as soon as an AST did I/O in the same routine. The price
is one register: a routine with expressions and a frame saves R11 and has one register cell fewer.

### What the optimizer had assumed

Section 16's `effects.go` says what a call may change: an address-taken local, COMMON, or a dummy.
Other locals survive calls because section 6.2 forbids recursion. A recursive unit breaks that rule
for its *static* locals only: a SAVE'd variable can be changed by the call that recurses. Its frame
locals are safe in a way an ordinary unit's aren't, since no other activation can name them.
`ChangedByCall` is the one place that answers, and every pass asking that question asks it.

Almost every pass: the register allocator had made section 6.2's argument again by hand, holding a
local in a register across a loop with a call unless it was a dummy, in COMMON, or address-taken.
A recursive routine's SAVE'd counter, held in R6 while the loop called the routine again, lost what
the deeper activations added. The lesson is section 16's again: an assumption about the language
should live in one function, so that a change to the language changes one answer.

The frame also gave the optimizer something. A frame variable is undefined at each call (the
standard's rule, and what a stack does), so a store to one that nothing reads before RETURN can go,
which a static variable's can't: the next call might read it. That's why some programs run fewer
instructions `/RECURSIVE`.

### FTNRTL was nearly ready

FTNRTL kept its statements on a stack of contexts from the start (section 14), so a statement inside a
function inside a statement, to any depth, needed nothing. Two things did:

- **The format cache.** A FORMAT's first use parsed it straight into its `$FCn` cell. An AST whose
  statement used the same FORMAT, between the main line's test of the empty cell and its store, parsed
  it too, and one copy was lost. Now the parse goes to the statement's context, and moves to the cell
  with ASTs held off only if the cell is still empty. A timer can't find a window two instructions wide,
  so `testdata/rtl89/fmtrace.for` traces itself and declares an AST at each instruction of the first use
  in turn (the method of section 15's "ASTs: every instruction is a boundary").
- **Unwinding.** A handler may unwind out of a statement, leaving its context on the stack; FTNRTL drops a
  context when its frame is gone. A recursive routine called again after the unwind may get the very same
  frames, so a new statement from deeper than the gone one can look like it. Nothing in the frames tells
  the two apart, so this stays a known difference, held by a test (`gone89.for`) so that it's noticed if
  it goes.

### The debugger has to count frames

With one copy of each variable, a name was an address. With many, it's an address in a frame, and the
debugger needs to know which frame. SET SCOPE, `n\NAME`, and `ROUTINE n\NAME` were taken from VMS's
debugger manual and a probe on VMS 7.1 (`testdata/dbg89`). Their core is `machine/scope.go`'s `RegistersAt`:
level k's PC, FP, and AP are what level k-1's frame saved, and R2 to R11 come from the nearest frame
inside level k whose entry mask saved them, else from the live register. FORTRAN's frame variables are then
displacements from that level's FP.

### Testing what can't be compared

Every program of `testdata/fortran` is also compiled `/RECURSIVE`, in both optimizer modes, and must print
its `.out` (`recursiveModes`), as must DUNGEON. Six can't: two print addresses, one is an AST's activation
working as it should, and three read a local or function value the last call left, which is undefined, and
what the stack holds differs between optimizer modes. They're listed with their reasons
(`recursiveDifferences`), and that list is the honest form of the test: a program that reads an undefined
value can't be held to "prints right", only to "is known to read one".

The recursive programs proper were worked by hand before they ran, then given to two witnesses. gfortran
agreed on every line but an IEEE `-0.0`, and refused a function's name used as a call without `RESULT`,
which govax accepts by the author's choice. VMS 7.1's MACRO and LINK built every program's MACRO file and ran
it (`testdata/ftn86`'s fifth round). That round also found a bug far from FORTRAN, in ods2's index file
growth, because a volume with half again as many files was the first to need an index file extension header.

## 24. Fortran 90: two source forms and a bigger alphabet (Phase 99)

Fortran 90 is a large language, and govax added it a part at a time, a phase each, sections 24
to 35 (`docs/planning/PHASE-99 - Fortran 90.md` lays out the program). The first part changes nothing a program can compute. It
teaches the front of the compiler to *read* Fortran 90: a second source form, new tokens, and
about thirty new statements, each recognized and then refused by name. That is less modest than it
sounds. The two front passes, `source` and `lex`, are where FORTRAN's oddities live (sections 4 and
5), and Fortran 90 changes the oddest of them: blanks.

### One compiler, and one switch

A Fortran 90 compiler could have been a second compiler. It isn't, for the reason most of the
language's users had: their programs were mixtures, old routines called from new ones, and an
`INCLUDE` file shared by both. So Fortran 90 joins VAX FORTRAN in one grammar, and the question
becomes where the two read the same text differently. There are only a few such places: `"17`
(VAX FORTRAN's octal integer, Fortran 90's character constant), `TYPE name` (VAX FORTRAN's output
statement, Fortran 90's derived type), and the new intrinsic names, which an old program may use
for its own routines. One qualifier, `/F90`, settles all of them, defaulting by the source's form.
Everything else Fortran 90 adds was a syntax error to VAX FORTRAN, so it can be read always.

"Nothing old changes" is a promise easy to break by accident, so it is a test.
`TestNoF90Unchanged` (`internal/fortran/nof90_test.go`) compiles every fixed-form source in the
repository, and DUNGEON where it is, three ways, and holds each MACRO file and its messages to a
SHA-256 taken before the work began. Every subtask runs it; it takes a second and a half.

### Free form in the source pass

Section 4's reader answers "where does a statement start and end?" by columns. Free form
(the standard's 3.3.1) answers it by characters: `!` starts a comment, `;` ends a statement, and an
`&` at a line's end continues it. All three mean something else inside a character constant, so
the reader must know, at each character, whether it is in one, across lines. Fixed form already had
that question, for `!` comments, and its answer was a function that read a whole field at once.
Free form needed it a character at a time, so that function became a scanner,
`commentScanner`, with `feed` to move past one character and `outside` to ask where it is; the
old function is now a loop over them, and gave the same answers. `source/free.go`'s `readFree` reads
each line in segments ended by a comment, a `;`, an `&`, or the line's end, feeding the scanner as
it goes.

Continuation has one rule that decides what the next pass sees. If the next line starts with
`&`, the statement goes on right after it, so a token may be split (`INTE&` then `&GER`); if not,
it goes on with the line's first character, blanks and all. The reader joins the pieces into the
statement's `Text` exactly so: blanks where the lines had them, none where an `&` joined two halves.
So the blanks in `Text` are the significant ones, and the lexer needs no separate list of where
they were. One case the reader can't judge is `AB&` followed by `C` in column 1: the standard forbids
splitting a token without the second `&`, but nothing at this stage knows that `AB` and `C` were meant
as two tokens, and the text is `ABC`. gfortran, which has a tokenizer at that point, refuses it.

### Blanks that mean something

Section 5 begins with FORTRAN's indifference to blanks: the lexer squeezes them out before it
classifies a statement, and `G O T O 10` is a GO TO. Free form reverses that: a blank may not stand
inside a name, and must stand between two. The tempting change, a second lexer for free form,
would have doubled the trickiest code in the compiler. Instead the lexer still squeezes, so
classification is the same in both forms (`END DO` and `ENDDO` squeeze alike), but it remembers where
each blank was (`squeezed.gap`). After tokenizing, `checkBlanks` looks at those gaps against the
tokens: a gap inside one is `BLANKINNAME`, two words with no gap between them, `CALLSUB`, are
`NEEDBLANK`. The statement's keyword isn't a token, so `keywordBlanks` checks it apart, against the
standard's list of keywords that may be written as two words (`GO TO`, `END DO`, `SELECT CASE`, ...).
Fixed form never asks either question.

### A bigger alphabet

Fortran 90's new tokens are `::`, `=>`, `(/`, `/)`, `%`, and the relational operators `==`, `/=`,
`<`, `<=`, `>`, `>=`. The two-character ones are entries in the tokenizer's table (`<`, `>`, and
`%` were single characters it already knew):

```go
var operators = []string{
	"**", "//",
	"(/", "/)", "==", "/=", "<=", ">=", "::", "=>",
}
```

None of them means anything to VAX FORTRAN, so each can be read whatever `/F90` says. The parser
reads `==` as `.EQ.` and the rest as their dotted twins, so a program using them compiled the day
they were added. The tokens keep their own text, so a message shows what was written. One smaller
trap was in the `=` test that tells an assignment from everything else: an `=` inside `==` or
`/=` is no assignment's, or `PRINT *, A == B` would be one.

Three additions needed more care, because VAX FORTRAN reads the same characters:

- **Kind parameters**, `1.5_8` and `K_'TEXT'`, and **letter-first constants**, `Z'1F'`. Inside
  parentheses, an apostrophe after a name is `READ (1'10)`'s record separator. So `Z'1F'` there is
  a constant only if only hexadecimal digits and an apostrophe follow, and the source pass and the
  lexer share the one function that decides (`source.ApostropheStartsConstant`). Otherwise they could
  disagree about where a constant is, and so about where a `!` comment starts.
- **Defined operators**, `.CROSS.`, look like VAX FORTRAN's record fields, `A.B.C`. Where no field can
  stand (an expression's start, after an operator or a constant) the lexer makes it an operator.
  After a name it leaves the pieces apart, for the parser and the symbols to settle when derived
  types arrive.
- **Quotation marks** start a character constant only under `/F90`.

### Statements recognized, and refused by name

Most new statements start with a keyword no old statement starts with, so they join section 5's
table and are found the same way. A few need tests of their own (`lex/f90stmt.go`): a `::` at
level 0 makes a statement a declaration whatever `=` it holds; `=>` with no comma before it is a
pointer assignment; WHERE is told apart as IF is; `TYPE` has three readings; a name and a colon
first is a construct name, which no VAX FORTRAN statement can begin with.

The parser reads each new statement only far enough to know what it is. It hands sema an
`ast.PendingStmt` naming the form (`What`: "the SELECT CASE statement", "a kind parameter on a
constant"), and sema's `notimpl.go` reports it, once, where it stands. This is section 21's advice,
"name what isn't done", at a larger scale: the compiler's answer to every Fortran 90 program is
either code or a list of what isn't compiled yet, never a confusing error. Statements that hold
others (CONTAINS, an interface block, a derived type's definition) gather what they hold into the
pending statement, so a module's procedures aren't mistaken for its own declarations. One kind
compiles now, because it needs nothing else: the named END, `END SUBROUTINE SHOW`, whose name the
parser checks against its unit.

### First light, and where the compiler meets the run-time library

The Fortran 90 corpus, `testdata/fortran90`, starts with programs that are FORTRAN-77 written in
free form with Fortran 90's spellings. They run in every mode the old corpus runs (section 17), and
gfortran is a witness again (`compare.py`'s `PROGRAMS90`, at `-std=f95`). Every expected output was
worked out by hand first. gfortran found one line in the corpus that wasn't Fortran 90, a split token
without its second `&`, and the reader above had accepted it.

The first program found a gap at the edge of the compiler: `FORMAT (1X, "N =", I3)`. A FORMAT's
text isn't FORTRAN; it goes to FTNRTL as written (section 14), and FTNRTL, like VMS's FORRTL,
knows only apostrophes. FTNRTL can't be told about `/F90`, which is a fact about the source. So the
compiler adjusts the text it hands over: `format.Options.Quotes` reads the quotation marks for the
compile-time check, and `format.Apostrophes` rewrites each such constant between apostrophes, its own
apostrophes doubled, before sema stores the text FTNRTL gets. FTNRTL is unchanged. A format given
at run time, from a character constant or a variable, can't be adjusted at compile time, so it waits
for the I/O phase. The lesson: when a language has two halves compiled at different times, each new
rule has to be placed in one half, and sometimes the compile-time half can translate it away.

## 25. Fortran 90: scopes, declarations, and kinds (Phase 100)

The second part of Fortran 90 is the way it declares things: `::` and attribute lists, values
given with `=`, and a *kind* for every type. Most of it says again, in new words, what VAX FORTRAN
already says (`REAL(KIND=8)` is `REAL*8`), so the work was mostly in sema, mapping new forms onto
old meanings, and in two new types that have no old meaning to map onto.

### A scope chain before there is anything to chain

Fortran 90's internal and module procedures see their host's names (host association, the
standard's 12.1.2.2.1). They come in Phase 103, but every name sema reads goes through the table
they change, so the table changed first, when nothing else did and every old test could say
whether it had broken. Section 7's per-unit map became a `scope` (`sema/scope.go`): its own names,
the list it keeps them in (`Unit.Symbols`, whose order is `$LOCAL`'s), its implicit rules, and a
`host`. `find` walks outward and enters nothing; `enter` puts a new name in the nearest scope that
keeps a list; `define` makes a name a scope's own, hiding the host's.

A program unit's scope has no host yet. One thing did gain one: a statement function. Section 12
described its dummy arguments swapped into the unit's table during its expression and out again;
now the definition is a scope of its own whose host is the unit's, each dummy defined in it, and
every other name found, or entered, in the unit. The swap's two halves were the kind of code that
breaks when a third thing touches the table; a scope has none. COMMON blocks, namelists, and
structures stay the unit's: they are other classes of names, and host association doesn't reach
them.

### One table for the attributes

A type statement with `::` may list attributes, `REAL, DIMENSION(10), SAVE :: X`, and each has an
old statement that did the same: `DIMENSION X(10)`, `SAVE X`. The parser reads the list
(`parse/f90.go`'s `attrSpec`), and sema gives each entity each attribute *by running its old
statement's code* (`giveAttributes`): DIMENSION through `dimension`, SAVE, VOLATILE, AUTOMATIC, and
STATIC as one-name statements read with the unit's. So every old check applies to the new form, and
the meaning can't drift: SAVE on a dummy argument is SAVEDUMMY whichever way it's written.

What *is* new is the standard's list of which attributes may go together (5.1's constraints). It
is a table, `sema/attrtable.go`, a row per attribute with the set it excludes and its *fate* in
this phase:

```go
attrSave:   {"SAVE", attrExternal | attrIntrinsic | attrIntent | attrOptional | attrAutomatic, attrCompiled},
attrIntent: {"INTENT", attrPointer, attrRecorded},
attrPointer: {"POINTER", attrAllocatable, attrNotYet},   // attrCompiled since Phase 107
attrPublic: {"PUBLIC", attrPrivate, attrModuleOnly},
```

A pair is excluded if either row names the other, and the fate says whether the attribute is
compiled, recorded on the `Symbol` for a later phase (INTENT, OPTIONAL, TARGET: a program written
for Fortran 90 nearly always says INTENT, and refusing it would refuse nearly every program),
NOTIMPL, or a module's only. The table still says "recorded" for those three: later phases gave
each its meaning in code of its own (`sema/intent.go`, `optional.go`, and `pointer.go`'s NOTTARGET; `opt`'s `findTargets`), which
reads what the table kept, and no row is NOTIMPL now. A name's attributes are gathered from wherever they were given, old
statements included (`attrsOf`), but only a Fortran 90 form is checked against the table, so a
program of old statements hears nothing new.

A value after `=` is DATA's work with two differences: one value fills a whole array (`broadcast`),
and it's checked as an assignment would be, not as DATA's looser rules (INITTYPE). Both are flags on
section 8's `dataSet`, not a second path. An initialized name gets SAVE (the standard's 5.1), which
matters only to section 23's storage rule, and that rule already kept initialized locals static.

### A kind is a byte count

Fortran 90 leaves a kind's numbers to the processor. govax takes the HP compiler's: a kind is a
size in bytes, so it names a `Type` that VAX FORTRAN's lengths already name, `Type{Kind, Size}`
with the size the kind. The one wrinkle is COMPLEX, whose kind is its *parts'* size, so
`COMPLEX(8)` is `COMPLEX*16` (`Type.KindNumber` halves it). `sema/kinds.go`'s `typeKinds` lists the
kinds each keyword has, and `kindType` turns a keyword and a kind into a type or a reason it can't
(NOSUCHKIND, NOKIND for `DOUBLE PRECISION`, which names a size already).

A kind is any constant integer expression, so it goes through section 8's evaluator, and a
PARAMETER can name it: `INTEGER, PARAMETER :: DP = SELECTED_REAL_KIND(12)`, then `REAL(DP)` and
`1.0_DP`. The parser had one ambiguity to settle: `IMPLICIT REAL (A-H)` is VAX FORTRAN's letters,
and `IMPLICIT REAL(8) (A-H)` Fortran 90's kind and letters. `kindBeforeLetters` looks past the first
parenthesis: if another follows the one it closes, the first was a kind.

A constant's kind changes how it is *read*, not only its type. `1.1_8` is `1.1D0`, the digits read
at D's precision (`realAs`, split from the reading of `1.1`), not `1.1` read as F and widened;
`kinds.f90` prints both, `1.100000023841858` and `1.100000000000000`, to show the difference.

### Two types with nothing to map onto

`INTEGER(1)` and `COMPLEX(16)` are the kinds VAX FORTRAN lacks. They taught opposite lessons.

**INTEGER(1)** needed almost nothing below sema. The IR already had a `Byte` type (for `LOGICAL*1`
and `BYTE`), and codegen's suffix rule (section 11) already wrote `ADDB3`, `CVTBW`, and `CMPB` for
it. The work was rules: where a byte ranks in a mixed expression (below `INTEGER*2`), and what the
intrinsic functions do with one. The intrinsic table (section 11's Table D-3) has no byte rows, and
adding a row to every generic function would have been a large, error-prone edit. Instead
`selectForm` takes a byte argument to the `INTEGER*2` row, and `byteResult` converts the result back
when every `INTEGER*2` row of that function gives an `INTEGER*2`: `ABS` and `MOD` of bytes are bytes,
`INT` of one is the default integer. FTNRTL's list-directed output was the one place below sema that
refused the type, and now writes it as it writes a `BYTE`.

**COMPLEX(16)** needed a type the IR had never had. Its parts are H values, which the VAX handles,
but the whole value is 32 bytes, and no instruction moves 32 bytes. So the IR's `Hexa` is a type
that is only addressed: lowering moves one as two octawords (`moveHexa`), and `ir.Check` refuses a
`Load` or an `Assign` of one, so a later change can't create an instruction that doesn't exist.
Sums, products, and comparisons are done on the parts, as section 11 does COMPLEX\*16's; the
quotient is a new FTNRTL routine, `FTN$DIVCH`, since neither the OTS$ routines nor MTHRTL have an H
complex one. And as with section 11's F products, the constant folder must compute what the code
would: `vaxfloat.ComplexDivSteps` does `FTN$DIVCH`'s steps in its order, each rounded to H, so a
folded `(1.0_16, 0.0) / 3` equals the one computed at run time, to the bit.

### Questions answered while compiling

`HUGE(X)`, `EPSILON(X)`, `DIGITS(X)` and the rest of the standard's 13.7.1 ask about `X`'s *type*,
never its value, so sema answers them (`sema/inquiry.go`) and the code holds a constant. The answers
come from the VAX formats' models, in `vaxfloat/model.go` beside the formats themselves (24 bits and
exponents -127 to 127 for F, and so on); the phase doc's survey computed the table by hand first, and
`TestModel` holds the code to it. Since only the type matters, an argument is analyzed for its type
alone (`inquiryArgType`): `HUGE(N)` must not enter `N` as a variable, or `$LOCAL` would gain a cell
because a program asked a question.

`SELECTED_REAL_KIND(P, R)` is the one place the VAX's arithmetic shows. D_floating has 56 bits of
precision but F's exponent range, so a range above 38 needs H. The common idiom
`SELECTED_REAL_KIND(15, 307)`, meaning "double" on an IEEE machine, gives 16 here unless the program
is compiled `/G_FLOATING`. That is correct, since the program asked for a range D can't hold, but
slow, and the manual says so. gfortran disagrees, of course, and `compare.py` lists the lines where
it does.

`SELECTED_REAL_KIND(R=40)` brought keyword arguments. The intrinsic table gained two optional
columns, a function's class and its dummies' names (`?` after an optional one), and
`sema/intrinsickeys.go`'s `intrinsicSlots` puts a reference's arguments in the dummies' order by the
standard's 12.4.1 before a row is chosen. `KIND=` is taken out there too, and `resultKind` converts
the result: `INT(X, KIND=2)` is the row for `INT` and a conversion after it. Only the functions whose
rows named their dummies took keywords in this phase; the rest waited for Phase 108 (section 33),
and a program's own procedures for Phase 103's interfaces (section 28).

### What the programs found

The corpus gained a program for each part (`declare`, `kinds`, `bytes`, `inquire`, `cplx16`), and
two that use the whole phase. `forms.f90` writes every declaration form, Fortran 90's beside VAX
FORTRAN's, in one program: `REAL(8) :: X*4`, IMPLICIT with kinds, `/clist/` beside `=`, COMMON and
EQUIVALENCE of names declared with `::`. `precise.f90` is the other side: a program that names no
size at all, asking `SELECTED_REAL_KIND(P=12)` for its precision, and so prints the same lines on the
VAX's D_floating and on gfortran's IEEE double. That is what kinds are *for*, and the witness agrees
with no listed difference.

`TestNoF90Unchanged` (section 24) ran at every subtask, and held: every fixed-form program in the
repository compiles to the same bytes as before the phase. That was easier than it sounds only
because each new form reached an old path (an attribute through its old statement, a kind through
an old type) instead of a new one beside it.

## 26. Fortran 90: control constructs (Phase 101)

The third part of Fortran 90 is the way a program says where control goes: named constructs,
a DO with no loop control, EXIT and CYCLE, and SELECT CASE (the standard's 8.1). None of them
needed anything new below the IR. EXIT and CYCLE are jumps a GO TO could already make, and
SELECT CASE is comparisons and branches, or the CASEL that section 12 wrote for the computed
GO TO. The work was in sema, which has to know what each statement belongs to, and in one
lowering, where SELECT CASE chooses which code to write.

### A name on the stack

Section 12's stack of open constructs (`sema/stmt.go`'s `construct`) held IF blocks and loops,
and a closing statement found its opener by looking down it. A construct name is one more field
on each entry, given as the stack opens it (`sema/constructs.go`'s `nameConstruct`), and every
statement that may repeat it checks it there (`checkName`): an END DO naming nothing when its DO
was named is MISSINGNAME, one naming another WRONGNAME. Each message says one thing; the survey
had proposed one for both, which would have had to explain two mistakes at once.

The rule that a construct name can't also be a variable's (the standard's 14.1.2) is checked at
the unit's *end*, not where the name appears (`checkConstructNames`). A name used later as a
variable, `OUTER = 1` after `OUTER: DO`, is only a variable once that statement is read, so a check
at the construct would miss it. By the end the symbol table holds every variable, PARAMETER,
statement function, and procedure the unit mentions, and one walk over the construct names
finds every clash.

SELECT CASE is a third kind of construct on the same stack, and section 12's matching code became
one function (`match`): ELSE IF, ELSE, and END IF look for the innermost IF, CASE and END SELECT
for the innermost CASE construct, and whatever is still open inside it is DONEST and closed. A
`sema.Case` embeds an `Arm`, the type an IF's block already was, so everything that walks an IF's
blocks (adding a statement, giving a label its path for section 12's jump checks) walks a CASE's
without knowing the difference.

### EXIT and CYCLE belong to a loop, decided once

The standard says which loop an EXIT belongs to: the innermost DO around it, or the one it names.
That is a question about the stack at the moment the statement is read, so sema answers it
(`loopExit`) and records the answer: a `sema.LoopExit` points at the `Do` or `DoWhile` it leaves or
continues. A name not open around it is NOLOOPNAME, and a name that is open but belongs to an IF
or SELECT CASE is NOTALOOP. Nothing below sema looks at names again.

A DO with no loop control is a `sema.DoWhile` whose condition is nil. That isn't a shortcut: the
standard *defines* DO WHILE as a DO with no control whose body starts with `IF (.NOT. e) EXIT`, so
the two are one construct in sema, and `doWhile` in the IR writes the test only if there is one.

### Where the jumps go

The IR keeps a stack of the loops it is lowering (`ir/lowerloops.go`), each with two blocks: *exit*,
the block after the loop, and *next*, where the next pass begins. EXIT jumps to the first and CYCLE
to the second, found by looking for the `LoopExit`'s loop on the stack.

*Next* differs by loop. For a DO WHILE it is the test, and for a DO with no control the body's first
block, both of which exist anyway. For a counted DO it is the step code (the variable stepped, the
count decreased, the branch back), which section 12 put at the end of the body's last block, so
there is no block to jump to. A CYCLE makes one, and `endPass` lays it out when the body is done,
so the step code lands in it. A loop with no CYCLE is lowered exactly as before, block for block.
`TestNoF90Unchanged` and every golden IR dump depend on that. If the loop ends at a labeled END DO,
that statement's block already exists, because a GO TO to it means "go on to the next pass", and
CYCLE uses it.

Because the count lives in a cell (section 12's `$Cn`), not recomputed from the variable, a CYCLE
can neither lose a pass nor add one. And because these are ordinary jumps, the optimizer needed no
change. The survey listed three shapes it worried about, and `testdata/fortran90/loops.f90` holds each
one. A named EXIT out of two counted loops whose variables the allocator put in R6 and R9 leaves through
a block of its own that stores both back (section 16's edges out of a loop):

```text
	CMPL	(R7)[R6],#43            ; if (grid(i, j) == 43) exit search
	BNEQ	LOOPS.$8
	BRW	LOOPS.$7
	...
LOOPS.$7:				; leaving the loop
	MOVL	R6,LOOPS.I
	MOVL	R9,LOOPS.J
	BRW	LOOPS.$11
```

A CYCLE in a loop that `DoLimits` closes with AOBLEQ is a branch to the AOBLEQ. The standard's
example 7, a CYCLE in a DO whose terminal statement is a CALL, passes over the CALL as the standard
says, since the CALL is in the body and the step code comes after it.

### SELECT CASE in sema: the overlap check

A CASE construct's values are constants, so sema can check the one rule that matters most: no value
of the expression may match two cases. Each range becomes a `caseSpan`. For an INTEGER expression
that is an interval of int64s, an open end set to the limit of the *expression's* kind, so
`CASE (100:)` on an `INTEGER(1)` is `[100, 127]`. For CHARACTER the span keeps the two constants,
compared as a CHARACTER relation compares (blank-padded, in ASCII, which is CMPC's order at run time),
and a LOGICAL value is its truth. Every range is checked against every one before it, and an overlap
is CASEOVERLAP naming the other's line. That is quadratic, but a construct has a handful of cases,
and an error should name *which* earlier case it collides with.

The same clipping finds the ranges that can never match: `CASE (9:1)`, or `CASE (300)` on an
`INTEGER(1)`. These are legal Fortran, so they draw a warning, CASENEVER, and overlap nothing.

Two of the type rules are choices the author made (the phase doc's answers 1 and 2). INTEGER values
of another kind than the expression's are accepted and compared by value, as the 1991 standard and
gfortran allow. A `BYTE` selector, which VAX FORTRAN uses as a small integer, is `LOGICAL*1` and so
LOGICAL by the standard's rule; selecting it by number is CASEBYTE, whose text says to declare it
`INTEGER(1)`, the type section 25 added for exactly this.

### SELECT CASE's code: three forms, one decision

Because no two ranges overlap, the order in which the dispatch tests them changes only how fast it
finds the match, never which match it finds. That freedom is what `ir/lowerselect.go` uses. The
SELECT CASE statement's code is the whole dispatch, and each case's block follows in the order
written, ending with a jump past END SELECT, as an IF's blocks end.

A LOGICAL construct is one Branch. An INTEGER one widens its value to a longword first, so that the
comparisons are CMPLs whatever the kind, and then one function, `caseForm`, chooses among three forms
from two numbers: *tests*, the comparisons a chain would make (one per value, one per end of a
range), and *span*, how many values lie between the lowest and highest bounded ends.

- **A table** when tests ≥ 4 and span ≤ 4 × tests. CASEL takes the same time for every value, and
  its table costs a word per value; a chain costs about six bytes per comparison and runs through
  half of them on average. Four slots per test keeps the table about as small as the chain it
  replaces. `cases.f90`'s TABLE16 has four tests over 16 values:

  ```text
  ;  103    select case (i)
  	CASEL	@TABLE16.I(AP),#1,#15
  TABLE16.$1:
  	.WORD	TABLE16.$2-TABLE16.$1        ; 1
  	.WORD	TABLE16.$3-TABLE16.$1        ; 2: DEFAULT
  	.WORD	TABLE16.$3-TABLE16.$1        ; 3
  	.WORD	TABLE16.$4-TABLE16.$1        ; 4 and 5, the range
  	.WORD	TABLE16.$4-TABLE16.$1
  	...
  	.WORD	TABLE16.$5-TABLE16.$1        ; 16
  	BRW	TABLE16.$3                   ; out of the table: DEFAULT
  ```

  CASEL's second operand is the table's lowest value, which section 12's computed GO TO always had
  as 1; the IR's `Switch` gained a `Base` for it. `CHAIN17`, the same cases over 17 values, is one
  value too sparse, and is a chain.
- **A binary search** when there are at least eight tests. The ranges are sorted and the search
  splits on the middle one's low end, carrying down what each comparison proved, so a leaf tests only
  the ends that knowledge hasn't settled. In SEARCH8 (100, 200, ... 800) every value takes four
  comparisons, where a chain takes up to eight:

  ```text
  	CMPL	@SEARCH8.I(AP),#500          ; below 500?
  	BLSS	SEARCH8.$2
  	...
  SEARCH8.$5:                         ; known: 200 <= I < 300
  	CMPL	@SEARCH8.I(AP),#200          ; so only I = 200 is left to test
  ```
- **A chain** otherwise, in the order the cases are written, since a programmer may well put the
  commonest case first. A range with both ends is one Branch on an `.AND.`.

Open ranges (`:-10`, `50:`) lie wholly beyond the bounded ones, since nothing overlaps, so they are
tested before a table or a search, and the table's out-of-range exit goes to them. Clipping to the
kind helps here too: on an `INTEGER(2)`, `-32768:-1` is one comparison, since nothing is below
-32768.

CHARACTER has no table (text has no index), so it is a chain or, from eight tests, a search of
`CharCmp`s. Section 11 said `CharCmp` may stand only as a branch's condition, where no register is
held, and a dispatch is exactly that. The selector is compared where it is when its place is fixed (a
variable, a constant substring). Otherwise its address and length go in two temporaries once, and each
comparison reaches it through `Mem.Ptr`, as a passed-length dummy is reached. One case needed more
thought: a selector that is a dynamic string (section 11's class D, `S // 'x'` with `S` a
passed-length dummy) must be freed after the comparisons and before any case's block runs, because
a block may jump anywhere. So each match stores its case's number in a temporary and goes to one
block, which frees the string and goes on by a `Switch` on the number.

A constant selector (a PARAMETER, say) is decided while compiling: a jump to its case.

### What the programs found

Four programs in `testdata/fortran90` cover the phase, each worked by hand and run in all eight modes:
`tokens.f90` (a tokenizer whose character classes are a twenty-test search), `loops.f90`, `cases.f90`
(each of `caseForm`'s thresholds from both sides), and `examples.f90`, the standard's own examples of
8.1 in its capitals, made to run. gfortran printed the same lines for all four.

The examples found the one bug, and it was not in a construct. `WRITE (3, '("QED")')`, the standard's
example of a named IF, gives its format as a character constant holding `"…"`. Section 24 had made the
compiler give FTNRTL a FORMAT *statement's* quoted strings between apostrophes, because FTNRTL, like VMS's
FORRTL, reads only apostrophes. But a format in a character constant is just a value to the compiler,
passed to FTNRTL at run time, and FTNRTL refused it. Under `/F90` sema now rewrites such a constant the
same way (`io.go`'s `constantFormat`). A format in a *variable* is still passed as it is, and is the
run-time library's to read. Making FTNRTL accept `"` would also change what VAX FORTRAN programs get,
and the rule since section 24 is that they get nothing new.

The END IF rule changed under both settings. VAX FORTRAN let a GO TO reach an END IF only from the
IF's last block; Fortran 90 allows any block, and outside. Since the change only accepts programs
that were refused, `/NOF90` takes it too.

## 27. Fortran 90: arrays as values (Phase 102)

Fortran 90 lets a whole array, or a regular piece of one, stand wherever a scalar could in an
expression or an assignment: `A = B + 2.0 * C`, `A(2:N) = A(1:N-1)`, `L = A > 0`. The standard
defines such a statement element by element, but *as if* the whole right side were computed
before any element of the target changed (7.5.1.5). The VAX has no vector instructions, so this
part of the compiler is a translation: every array statement becomes loops of scalar statements,
which the passes of the earlier sections already compile. That translation is called a
*scalarizer*. Nothing below the IR changed. The optimizer took the loops as it takes a DO's, and
FTNRTL gained one routine, a message for `/CHECK`.

This section follows one statement through the passes:

```fortran
      SUBROUTINE SHIFT(N)
      COMMON /V/ A(100), B(100)
      A(2:N) = A(1:N-1) + B(2:N)
      END
```

### One form in the parser, two meanings in sema

`X(1:3)` is a *substring* if X is a CHARACTER scalar and a *section* if X is an array. The
parser can't tell which, because it doesn't know what X is. So it reads every parenthesized
list after a name as an `ast.Apply` whose items may be `ast.Triplet`s (a lower bound, an upper
bound, and a stride, each possibly left out). Sema decides (`sections.go`'s `substringForm`).
One stride-less triplet after a name that isn't an array becomes the `ast.Substring` the parser
used to build, so every older pass still sees what it always saw. A second list after an element
or section (`C(I)(1:3)`, `B(:, J)(2:3)`) is always a substring. `TestNoF90Unchanged`'s digests
show that old programs compile to the same MACRO as before.

`(/ 1, 2, (I, I = 3, 10) /)`, an *array constructor*, is a primary of its own
(`ast.ArrayConstructor`). Its implied DOs use the node that I/O lists and DATA already had.

### Shape belongs to the expression

Every expression now has a *shape*: its rank and an extent per dimension. A scalar has none, and
conforms with anything. Fortran 90's Decision 5 had put the rank in the type, but `sema.Type` is
compared with `==` in hundreds of places that care only about the element type: a dummy's type
against an actual argument's, whether a conversion is needed. It also becomes the IR's type, a
map key. A REAL array and a REAL scalar would have stopped being equal everywhere. So `Type()`
stays the element's type, and the shape is a field of its own beside it (`exprBase.S`, a
`Shape`, in `shape.go`).

An extent is either `Known`, with its size, or else `Of` some array or section and one of its
dimensions, to be computed when the statement runs. A section's extent is the DO loop's trip
count, `MAX((u - l + s) / s, 0)`. Two shapes conform if their ranks are equal and no two known
extents differ (`conform`). Where both extents are known and differ, that is a compile-time
error, NOTCONFORM, naming both shapes as `(10, :)`. Where an extent isn't known, the check waits
for `/CHECK=BOUNDS` at run time.

Arrays may stand in only some places, and sema keeps them out of the rest by having two ways to
analyze an expression. `value` is the scalar context: a subscript, a DO bound, an IF's condition,
a unit number. An array there is NOTSCALAR ("an array where a scalar is required"). `arrayValue`
is the other context, used only where the standard allows an array: an assignment's sides, an
elemental operation's operands, an intrinsic function's arguments, an I/O list's items, a WHERE
mask, a constructor's values, and a vector subscript. Our statement's sema dump shows the
section, the shape, and the answer of the overlap test that comes next:

```text
3           A[2:N:1] = (+:R A[1:(-:I N 1):1] B[2:N:1])  [down in 1]
```

### Overlap: may the loops compute what they assign?

A scalarizer that simply runs `A(k+1) = A(k) + B(k+1)` for k = 1, 2, ... is wrong. Each pass
reads the element the previous pass just wrote, so A(1) is copied all the way to A(N). The
standard's "as if before" says the second pass must read the *old* A(2). `sema/overlap.go`
decides, for the target and for each array the value reads that might share its storage
(`mayShare`), how the two relate. It compares them a subscript at a time, as `slot`s: either a
fixed subscript, or a triplet's start and stride. There are four answers.

- **Apart.** The two never meet. Either their storage differs (different names that no COMMON
  or EQUIVALENCE joins; two dummy arguments, by the rule the optimizer already relies on), or a
  fixed subscript differs by a constant (`A(:, 1) = A(:, 2)`), or the starts differ by a constant
  that isn't a multiple of the stride (`A(2:20:2) = A(1:19:2)`).
- **The same element** at every index: the same fixed subscripts, and the same triplet starts
  and strides (`A(1:N) = A(1:N) + 1`). Each pass reads only what it is about to write itself.
- **Ordered.** The starts differ by a constant multiple of a constant stride. The element read at
  index k is the one written at k + q. If the loop over that dimension runs up when q is positive
  and down when it is negative, every element is read before it is written. Our statement reads
  `A(1:N-1)` while writing `A(2:N)`. `difference` sees `N-1` and `N` as one expression plus
  different constants, so q is -1, and the answer is **down in 1**.
- **Unknown**: anything else, such as a reversal (`A(10:1:-1) = A`), a row assigned to a column,
  an EQUIVALENCE that moves one array along another, or two leaves asking for opposite directions
  in one loop (`A(2:9) = A(1:8) + A(3:10)`). The value then goes into a temporary first.

Two fixed subscripts whose difference *isn't* known, `A(I, K:N)` against `A(K, K:N)`, were
"unknown" until this phase's programs showed what that costs (below). Both are computed once,
before the loops. When the program runs they are either equal, and that dimension is the same
element, or different, and the two never meet. Neither answer asks anything of the loops, so
`relate` treats them as equal and lets the other dimensions decide. `TestArrayOrder` holds a
case for each answer.

The result is an `ArrayOrder` on the assignment: a direction for each loop, or `Temporary` and
the temporary's extents. One more case matters. A scalar the loops would compute at every element,
if it reads the target (`A = A / A(10)`), is computed once before them instead
(`ArrayOrder.Hoisted`). A(10) changes during the loops, and the standard wants its old value
throughout.

### The scalarizer

`ir/lowerarray.go` lowers an array assignment as a *nest* of counted loops, one per dimension of
the target. The leftmost dimension's loop is innermost, so memory is walked in array element
order. Each loop runs an index k from 1 to its dimension's extent by 1, or down from the extent
if the `ArrayOrder` says so. Every array in the statement, a *leaf* (a whole array, a section, a
constant, a constructor), then answers one question: "what is your element at the indexes k?"

The answer is built in sema's terms, not the IR's, and that choice is what made the scalarizer
small. The loops' indexes are `sema.Symbol`s of lowering's own (`sema.NewSymbol`), bound to IR
cells in `lowerer.bound`, the mechanism statement functions already used (section 12). A
section's element is then an ordinary `sema.Elem` whose subscripts are expressions of those
symbols, and an elemental operation's is the operation on its operands' elements
(`sema.Scalarize`, which copies each node without its shape). The element of an adjustable
array, a CHARACTER element with its substring, a COMPLEX product, or a comparison stored in a
LOGICAL is lowered by the same code that lowers it anywhere else. `countedLoop`, section 12's
DO, needed no new form either.

Before the loops, `prepare` turns each leaf's subscripts into a *normalized* form. Dimension d's
subscript at index k is `k * stride + base`, where `base = lower - stride`, and every part that
isn't a constant goes to a temporary once. The loops' extents come from the target's shape where
it knows them, from the value's otherwise, and else are computed: a triplet's trip count, never
below 0 (`atLeastZero`). A scalar in the value that isn't cheap to recompute (`sema.Inline`: a
constant, a variable, an element with constant subscripts) is computed before the loops too
(`hoist`). A function is then called once, and the standard's "before any element is defined"
holds for it as well. Here is our statement's IR, with its loop counting down:

```text
  assign $T1 = @N
  assign $T2 = sub.L(@N, 1)
  assign $T3 = @N
  assign $T4 = sub.L($T1, 1)
  branch lt.L($T4, 0) B2 B3
B2:
  assign $T4 = 0
  jump B3
B3:
  assign $C1 = $T4
  assign $I1 = $T4
  branch le.L($C1, 0) B5 B4
B4:  ; array (line 3), dimension 1's body
  assign A[$I1] = add.F(A-4[$I1], B[$I1])
  assign $I1 = sub.L($I1, 1)
  assign $C1 = sub.L($C1, 1)
  branch gt.L($C1, 0) B4 B5
```

The loop's extent is N - 1, at least 0, in `$T4`. `$I1` is the index (an `IndexCell`, `$In`,
pooled per statement as temporaries are), and `$C1` the pass count, as a DO keeps it. The three
leaves' subscripts, k + 1, k, and k + 1, are each folded into a displacement on one index:
`A[$I1]` is A(k+1) because A's own displacement already allows for its lower bound, `A-4[$I1]`
is A(k), and `B[$I1]` is B(k+1). `$T2` and `$T3`, the triplets' upper bounds, are computed but
never read, because nothing needed them; dead-code elimination removes them under `/OPTIMIZE`.
`/NOOPTIMIZE`'s MACRO is the IR an instruction at a time:

```text
SHIFT.$5:				; array (line 3), dimension 1's body
	MOVL	SHIFT.$I1,R0
	MOVL	SHIFT.$I1,R1
	MOVL	SHIFT.$I1,R2
	ADDF3	SHIFT.A-4[R0],SHIFT.B[R1],SHIFT.A[R2]
	DECL	SHIFT.$I1
	DECL	SHIFT.$C1		; one pass fewer
	BLEQ	SHIFT.$6
	BRW	SHIFT.$5
```

Because the loop is a `CountedLoop` with a longword index and a constant step, everything section
16 does to a DO applies. The index goes in a register, the count and the index turn out to be the
same value running down to 1, and `DoLimits` closes the loop with SOBGTR. One instruction per
element is left:

```text
	MOVL	@SHIFT.N(AP),SHIFT.$T1
	SUBL3	#1,SHIFT.$T1,R6
	BGEQ	SHIFT.$1
	CLRL	R6
SHIFT.$1:
	CMPL	R6,#1
	BLSS	SHIFT.$3
SHIFT.$5:				; array (line 3), dimension 1's body
	ADDF3	SHIFT.A-4[R6],SHIFT.B[R6],SHIFT.A[R6]
	SOBGTR	R6,SHIFT.$5
```

### Temporaries

When the overlap test answers "unknown", `arrayTemp` makes an array cell (`ArrayTempCell`, `$Wn`, a work array)
of the assignment's shape, and the statement becomes two nests: the value into the temporary,
then the temporary into the target. Each nest is an ordinary one, with its own `/CHECK` checks.
`A(10:1:-1) = A(1:10)` is:

```text
B14:  ; array (line 6), dimension 1's body
  assign $A1-4[$I1] = A-4[$I1]
  ...
B16:  ; array (line 6), dimension 1's body
  assign A+40[mul.L($I2, -1)] = $A1-4[$I2]
```

A temporary follows `Routine.NewCell`'s rule, as every compiler cell does: it is in `$LOCAL` in
an ordinary routine and in the frame in a recursive one, where each activation needs its own.
Its size must be known when compiling. `X(1:N) = X(N:1:-1)` would need one of N elements, and is
NOTIMPL naming Phase 104, whose run-time-sized temporaries and descriptors this phase leaves
alone. That limit, the size of a temporary and not of a shape, is the phase's real boundary.
Adjustable arrays and sections whose extents are known only at run time compile freely, as long
as nothing has to be copied.

### What is computed before the loops, and what at each element

Most of Fortran 90's array functions fit the element question too, each in its own way
(`sema/arrayfuncs.go`, `ir/lowerarrayfunc.go`):

- **An element at a time.** An elemental intrinsic (`SQRT(A)`, `MAX(A, 0.0)`; Table D-3's
  rows are now class E, but for LEN, SIZEOF, and NWORKERS) is the function of its arguments' elements. A MTHRTL routine is called once
  per element, and a scalar argument is hoisted. MERGE is a row too: FSOURCE goes in a temporary
  and is replaced by TSOURCE where MASK holds. TRANSPOSE is its argument's element with two loops
  swapped, and copies nothing.
- **A small loop at each element.** A reduction by `DIM=` (`SUM(A, DIM=1)`) and an element of
  MATMUL are scalars, each computed by an inner nest of its own over the dimension being reduced,
  with the statement's indexes standing for the other dimensions (`ArrayFuncAt`). So
  `C = MATMUL(A, B)` is the textbook triple loop, with no temporary unless C is A or B.
- **Once, before the loops.** A reduction with no DIM= is a scalar, hoisted like any other. Its
  accumulator starts at the value for an empty array: SUM 0, PRODUCT 1, MAXVAL the most negative
  number (the VAX has no infinities), MINVAL the most positive, ANY false, ALL true. MAXLOC and
  MINLOC fill a small INTEGER temporary in one scan, counting each subscript from 1, the first
  extreme winning and zeros when nothing is found. SHAPE, LBOUND, and UBOUND fill one too.

### Constant arrays and constructors

The constant evaluator (section 8) gives scalars, and now arrays too: `sema.ArrayConst`, its
elements `Const`s in array element order (`constarray.go`). Constructors of constants, array
PARAMETERs and their sections, elemental operations and intrinsics of them, RESHAPE, and the
inquiries of arrays with constant bounds all fold. Each element is computed by the scalar code,
so a folded array has the bits the run-time code would have given it. RESHAPE is the only way to
write a PARAMETER of rank 2, so it came into this phase for constants. An `ArrayConst` that the
loops read is a datum in `$PDATA`, written only if something reads it.

A constructor whose values aren't constant (`sema.Constructor`) is built before the loops into a
temporary, whose size must again be known. There is one exception (`ArrayOrder.Fill`). When the
constructor is the whole value of a rank-1 assignment and reads nothing of the target's, its
values are stored straight into the target at a running position, so its length may be anything:
`A(1:N) = (/ (I * I, I = 1, N) /)`. A constant vector subscript with a value twice, in a target
or an input list, is MANYONE: the standard forbids assigning through it, and only a constant can
be checked.

### WHERE

`WHERE (mask) A = expr` assigns only where the mask holds, and ELSEWHERE only where it doesn't.
The standard computes the mask once, at the WHERE, so an assignment in the construct that changes
what the mask reads mustn't change the mask. `sema/where.go` treats the construct as an IF block
(its arms, its labels, INTOBLOCK for a jump into it). At END WHERE, `maskInLoops` decides whether
each assignment's loops may recompute the mask element by element: they may if no earlier
assignment's target reads into the mask, and each assignment's mask relates to its target as its
value does, by the overlap test. The common `WHERE (A > 0) A = SQRT(A)` needs nothing more: each
element's test is a compare and a branch in the assignment's own loop. Otherwise the mask is
computed once into a temporary of its own type. A value that must go through a temporary is
masked in both of its nests, so `SQRT` is never taken of an element the mask excludes.

### I/O lists, and the run-time checks

An output item that is a section or an array expression is a loop of `FTN$IO_ITEM` calls in
array element order (`ioItem`), each value through a temporary as a scalar expression's is. An
input section is a loop over its elements' places, and a constructor's values are items in turn,
never built. A whole array keeps its single class A descriptor (section 14), and since Phase 109
a section is one item with a class NCA descriptor (section 34). The extents are computed when the list reaches the item, so `READ *, N,
A(1:N)` uses the N just read.

Under `/CHECK=BOUNDS` a section's first and last subscripts are checked once, before the loops
(`checkSubscripts`), not at every element. A vector subscript's values are checked where they are
used. Every extent not known when compiling is compared with the loops' (`checkShapes`), and a
difference calls FTNRTL's new `FTN$NOTCONFOR`, which signals `FOR$_NOTCONFOR`, "array shapes do
not conform". It is govax's own condition, FOR number 300, past the numbers VAX FORTRAN used.

### What the programs found

Eleven programs in `testdata/fortran90` cover the phase, each worked by hand, run in all eight
modes, and the same on gfortran. The last three are real ones: `matrix.f90` (Gaussian elimination
with partial pivoting), `stats.f90` (the reductions on a set of scores), and `smooth.f90` (image
smoothing with WHERE). They found two bugs that the smaller programs, each written for one
feature, hadn't.

The first was in the CPU. `stats.f90`'s INTEGER MAXVAL printed `***`. The accumulator starts at
-2147483648, and `CMPL elem, acc` followed by `BGTR` never branched. govax's CMP had set N from
the sign of src1 - src2, which is the wrong answer whenever that subtraction overflows: 3 -
(-2147483648) is negative in a longword. The architecture says N ← src1 LSS src2, a signed
comparison. The C emulator had the same rule, and no program had yet compared numbers that
far apart. `compareCodes` (`internal/cpu/condcodes.go`) now gives CMP, CMPV, and
CMPC that comparison itself.

The second was the overlap rule above. `matrix.f90`'s elimination works on rows of the same
array in a subroutine, `A(K, :) = A(P, :)` and `A(I, K:N) = A(I, K:N) - F * A(K, K:N)`, with N a
dummy. "Unknown" sent each statement to a temporary whose size isn't known when compiling, so
both were NOTIMPL. Treating unknown fixed subscripts as equal compiles them with no temporary.
The inner loop is now six instructions: three induction variables, one per leaf, each stepping
by the row's stride, and an `AOBLEQ`.

Optimizing gains less on these programs, 6% to 10%, than on the old corpus's kernels, because
each statement's nest is short and is the whole of the work. Phase 110 measured what was left on
kernels: adjacent assignments and a WHERE construct's arms now share one nest, and an assumed-shape
element's address is stepped (section 35); a whole-array copy is still a loop rather than a MOVC3,
which would be fewer instructions without being clearly faster.

## 28. Fortran 90: procedures and interfaces (Phase 103)

FORTRAN-77 has one kind of procedure interface: the *implicit* one. A caller knows a routine's
name and, by the first letter or a type statement, its result's type, and nothing else. It
pushes what it was given and hopes. Fortran 90 adds *explicit* interfaces: the caller knows each
dummy argument's name, type, kind, rank, INTENT, and whether it is OPTIONAL. That knowledge is
what keyword arguments, optional arguments, generic names, and defined operators all rest on,
and it comes from two new places: *internal procedures*, which a unit holds after CONTAINS and
whose declarations its own statements can see, and *interface blocks*, which describe
procedures defined elsewhere.

Most of this phase is semantic analysis, as the earlier Fortran 90 sections were. What
reaches the IR is a call of a named routine with a list of arguments, as in section 12. The
new mechanisms below sema are two small ones: how an internal procedure reaches its host's
storage, and how a routine learns which of its OPTIONAL dummies it was given.

### Units inside units

The parser's units were flat: a header, a body, an END. An internal procedure is a whole unit
inside another, and an interface body is a unit's header and declarations inside a statement.
`parse/units.go` keeps a stack of open units (`unitStack`). A FUNCTION or SUBROUTINE statement
starts a unit *inside* the innermost one if that unit is past its CONTAINS or has an interface
block open. Otherwise it ends the open units (NOEND) and starts a new unit of the file, as it
always did. `ast.Unit.Contains` holds the internal procedures, and `ast.InterfaceBlock` is a
statement of the body, holding its `Bodies` (each an `ast.Unit`) and its generic specification:
none, a name, `OPERATOR(op)`, or `ASSIGNMENT(=)`. The rules that are syntax are checked here:
CONTAINS with nothing after it, an internal procedure with a CONTAINS or an ENTRY of its own, an
executable statement in an interface body.

One question about expressions has to be settled here too. `A.B.C` is DEC's reference to the
field C of the field B of the record A, but in Fortran 90 it can also be A `.B.` C, a defined
binary operator. The two readings make different trees: in `X + A.B.C * 2` the fields are a
primary, while the operator binds more loosely than anything else and splits the expression as
`(X + A) .B. (C * 2)`. Sema can't turn one tree into the other, so the parser decides
(`recordScope`). A name is a record only if a RECORD statement declared it, and the parser has
read every declaration before the first executable statement. `.B` after a record (or after one
of its fields or elements) is a field, and anywhere else `.B.` is an operator. Old programs,
which have no defined operators, parse exactly as they did.

### Host association, in four steps

An internal procedure sees its host's names (12.1.2.2.1). A name is the procedure's own if one
of its declarations declares it; any other name it uses is its host's, if the host has it. The
catch is *wherever in the host the host mentions it*, executable statements included:

```fortran
      PROGRAM STEPS
      DO I = 1, 3
         CALL STEP
      END DO
      PRINT *, I
      CONTAINS
      SUBROUTINE STEP
         DO I = 1, 2
         END DO
      END SUBROUTINE STEP
      END PROGRAM STEPS
```

Neither unit declares I. The host's DO makes I a variable of the host, so STEP's I is the host's
I, and STEP changes its host's loop variable. That is the standard's meaning, and a classic
mistake. A compiler that analyzes the host whole and then its internal procedures would get it
right. But the host's statements need to know the internal procedures too: STEP's interface, to
check `CALL STEP`, and a FUNCTION's type, to type a reference to it. So `analyzeUnit` works in
four steps (`sema/internal.go`):

1. the host's declarations;
2. each internal procedure's header and declarations, its interface, by an analyzer of its own
   whose scope has the host's scope as its host (`innerAnalyzer`, section 25's scope chain);
3. the host's executable statements, which enter the names the host first uses there (I);
4. each internal procedure's executable statements, whose lookups now find all of those.

A name made in step 2 that the host then uses in step 3 would have had to be the host's all
along. That can happen only through a bound or a length (`REAL W(N)`, N used only in the host's
executable part). It is reported NOTIMPL rather than guessed (`checkProvisional`). The implicit
rules are the host's, letter by letter, with the internal procedure's own IMPLICIT statements
changing letters on top (`implicitRules.inherit`).

What STEP compiles to shows the rest of the story. The host's I is in `$LOCAL`, so STEP names it
as the host does:

```text
	.ENTRY	STEPS,^M<R6>
	MOVL	#3,R6			; DO (line 2)'s count
	MOVL	#1,STEPS.I
STEPS.$1:				; DO (line 2)'s body
	CALLS	#0,STEPS.STEP
	INCL	STEPS.I
	SOBGTR	R6,STEPS.$1		; one pass fewer
	...
STEPS.STEP:
	.WORD	^M<>			; internal procedure STEP of STEPS
	MOVL	#1,STEPS.I
STEPS.STEP.$1:				; DO (line 8)'s body
	INCL	STEPS.I
	CMPL	STEPS.I,#2
	BLEQ	STEPS.STEP.$1
	RET
```

The internal procedure is a routine written after its host in the same module. It is entered at
a local label with a register mask, `STEPS.STEP: .WORD ^M<>`, rather than a global `.ENTRY`, so
no other module can call it and two hosts may each have a STEP. Its own symbols are
`STEPS.STEP.X`, the path the listing and the debugger show. The loop's pass count is the host's
own, in R6, so the host's loop still runs three times. But I is 3 after each STEP, 4 after the
INCL, and the program prints 4. `/OPTIMIZE` kept the count in a register and left I in memory,
because I is `Reached` by an internal procedure: any call in the host may read or change it
(section 16's rule for a "taken" local, `opt/effects.go`). In the internal procedure a host's
cell is a region of its own, `HostRegion`, which any call may change, as COMMON may.

### Reaching the host

A host's variable in `$LOCAL`, or in a common block, is a cell with a name, and the internal
procedure uses that name. Two other kinds of host storage need more.

A host's **dummy argument** is reached through the host's AP, which only the host's own code
has. Section 12's ENTRY statements already solved that problem: each dummy gets a cell (`$A1`)
that the entry copies its address into, and the body reaches the dummy through the cell. A host
does that for each dummy an internal procedure reaches (`passHostArgs`), and the internal
procedure uses `@HOST.$A1` as the host does.

A host whose storage is in its **stack frame**, a RECURSIVE host (section 23) or one with
AUTOMATIC variables, may be active several times at once. Its internal procedure must reach the
activation that called it. So every call of one of its internal procedures passes a hidden last
argument, the host's FP, or in an internal procedure calling a sibling or itself, the FP it was
given (`Routine.HostFP`). The internal procedure loads it into R10 and reaches each frame cell by
the host's own displacement symbol:

```text
	.ENTRY	WALK,^M<>
	SUBL2	#12,SP			; this activation's own storage
	MOVL	4(AP),WALK.$A1(FP)	; N is argument 1
	...
	PUSHL	FP
	CALLS	#1,WALK.ADD
	RET
WALK.ADD.$HOST = 4			; WALK's FP is hidden argument 1
	...
WALK.ADD:
	.WORD	^M<R10>			; internal procedure ADD of WALK
	MOVL	WALK.ADD.$HOST(AP),R10	; the FP of WALK, for its frame
	ADDL2	@WALK.$A1(R10),WALK.MINE(R10)
```

The IR names the host's cell, never its offset, because the host's optimizer may still close
its frame up. The displacement is the host's symbol, fixed when the host's code is written. An
internal procedure is never called once its host has returned (it can't be an actual argument:
INTERNALARG), so the frame the FP points at is always live.

### Explicit interfaces, checked

An internal procedure's interface is its own declarations (step 2). An **interface body** is a
header and declarations with nothing to run (`sema/interface.go`). It is analyzed in a scope
with *no* host: the standard gives an interface body none of the enclosing unit's names, so a
kind written with the host's PARAMETER is an error there. The body's unit is kept in the holding
unit's `Interfaces` and never lowered. Its name becomes either a dummy procedure, if it is a
dummy argument, or an external procedure.

Every reference through an explicit interface goes to one function, `checkInterface`: the count
(ARGCOUNT, as section 12 already checked within a file), and then per argument its type and kind
(ARGTYPE), its rank (ARGRANK: an element may go to an array dummy, sequence association), a
CHARACTER length where both are known (ARGLENGTH), and for an INTENT(OUT) or INOUT dummy that it
is a variable (NOTDEFINABLE). An interface body for a procedure the same file defines is
compared with the definition, and the first difference is INTERFACEDIFF. A call through an
*implicit* interface to such a procedure is checked the same way only if
`/WARNINGS=ARGUMENT_CHECKING` asks, and only as warnings: old programs pass an array's first
element for a scalar on purpose.

### Putting a call's arguments in order

With an explicit interface a call may name its arguments (`CALL SOLVE(X, TOL=1.0E-6)`) and leave
OPTIONAL ones out. `sema/optional.go`'s `argumentOrder` puts them in the dummies' order before
anything else looks at them. The arguments without keywords come first and fill the dummies by
position. Each keyword fills the dummy it names, which must not be filled twice. Every dummy
that isn't OPTIONAL must end up with an argument. The result is a plain positional list with a
`nil` for each dummy left out, which is VAX FORTRAN's old null argument. Everything later (the
analysis of each argument, `checkInterface`, the IR) sees an ordinary list as long as the
dummies, and the IR passes a 0 for each `nil`, as it always did for `CALL S(A,,B)`. That is VMS's
convention for an absent argument.

VMS has a second convention: a caller may pass a *shorter* list, and the routine treats the
missing trailing arguments as absent. The LIB$ routines work that way, and a MACRO or FORTRAN-77
caller of a Fortran 90 routine may too. The longword at `4*n(AP)` is then not the routine's at
all, and only the count byte at `(AP)` says so. So every OPTIONAL dummy is reached through a
cell, the ENTRY mechanism once more, and the entry fills the cell only if the count reaches it:

```text
	.ENTRY	SOLVE,^M<>
	CLRL	SOLVE.$A2		; TOL is argument 2, if the caller passed it
	CMPB	(AP),#2
	BLSSU	SOLVE.$3
	MOVL	8(AP),SOLVE.$A2
SOLVE.$3:
	MOVL	#^X37BD3686,SOLVE.T	; 1E-06
	TSTL	SOLVE.$A2
	BEQL	SOLVE.$1
	MOVF	@SOLVE.$A2,SOLVE.T
SOLVE.$1:
```

`PRESENT(TOL)` is then a test of the cell (`TSTL SOLVE.$A2`). Passing TOL on to another
routine's OPTIONAL dummy passes the cell's contents, so a 0 stays a 0 however the routine was
called. An internal procedure asking about its host's OPTIONAL dummy tests the host's cell.
`testdata/fortran90/modules/shortlist` is a FORTRAN-77 caller, in a file of its own, passing such
a routine a shorter list.

INTENT changes no code. Sema checks what it can see (`sema/intent.go`). An INTENT(IN) dummy
given a value anywhere a value is given (an assignment, a DO variable, an input item, ASSIGN,
IOSTAT=), or passed to an INTENT(OUT) dummy, is INTENTIN. An INTENT(OUT) dummy used on a line
before every line that may define it is the warning INTENTOUTREF. "May define it" counts an
actual argument passed as itself, since the callee might set it. The test is about the order
of the statements, not their flow, which is why it is only a warning.

### Resolving a generic reference

`INTERFACE SWAP` with bodies for SWAPR and SWAPI makes SWAP a *generic* name
(`sema/generic.go`). A reference by it becomes a reference to the specific whose interface it
fits, chosen when compiling: `CALL SWAP(X, Y)` with REAL X and Y is `CALL SWAPR(X, Y)` in the IR,
with nothing at run time. `resolveGeneric` analyzes the arguments once and then tries each
specific in turn. `argumentOrder` puts the arguments in that specific's order, quietly, and
`fits` requires each one to have its dummy's type, kind, and *rank* exactly. A scalar doesn't
fit an array dummy here, as sequence association would allow in a plain call, because rank is
what tells specifics apart. The unit's own specifics are tried first, then its host's. If no
specific fits and the name is an intrinsic function's, the reference is the intrinsic's: a block
can *extend* SQRT to LOGICAL arguments without hiding SQRT of a REAL.

The standard makes "the one that fits" well defined by restricting the blocks rather than the
references (14.1.2.3). Two specifics of one name must both be functions or both subroutines,
and one of them must have a non-OPTIONAL dummy that the other can't take in its place *both* by
position *and* by keyword: at its position the other has no dummy, or one of a different type,
kind, or rank, and the other's dummy of the same name is missing or different in the same way.
`tellsApart` is that sentence, word for word. The standard's own example is SWAP itself. SWAPR's
A and SWAPI's I differ at position 1, and SWAPI has no A, so the pair can be told apart. Make I
REAL and the keyword test still passes (SWAPI has no A) but the position test doesn't, and
`CALL SWAP(0.0, 1.0)` could mean either: GENERICAMBIG. A specific of an intrinsic's name is
checked against the intrinsic's table rows "as if the intrinsic consisted of a collection of
specific procedures", each row with every argument count it allows (GENERICINTRINSIC).

### Operators and assignment

A defined operator is a generic name spelled differently (`sema/operators.go`).
`INTERFACE OPERATOR(.DIST.)` lists functions of one or two INTENT(IN) operands. `A .DIST. B`,
an `ast.DefinedOp` from the parser, is a reference to the one the operands fit, chosen by
position alone. A unary defined operator binds more tightly than anything else and a binary one
more loosely, which the parser's precedence table has done since this phase's first subtask that built the forms.

An intrinsic operator can be extended too, but only to operands it has no meaning for: `+` on
two LOGICALs, `//` on a CHARACTER and an INTEGER. Sema analyzes the operands as before; then, if
`intrinsicMeaning` says the standard gives none, it looks for a block that fits (`extendedOp`). A
block function for operands the standard does give a meaning to, such as `+` of two REALs, is
refused at the block (DEFOPINTR). VAX FORTRAN gives some meanings the standard doesn't: `.AND.`
of INTEGERs bit by bit, `.XOR.`, and a LOGICAL assigned an INTEGER. A block may take those over
(the author's choice): `12 .AND. 7` is 7 with a block that computes the minimum, and 4 without.
One trap: the constant evaluator folds `'N=' // 42` before anything else sees it, and it knows
nothing of blocks. So an operator that has a block isn't folded first.

`INTERFACE ASSIGNMENT(=)` makes `X = Y`, for types the standard doesn't assign between,
`CALL S(X, (Y))`. The value is passed as a copy, as the standard's parentheses say.
`assignment` analyzes the target and value, offers them to `definedAssignment`, and only then
converts the value for an intrinsic assignment (`finishAssign`).

### What the programs found

Nine programs in `testdata/fortran90` cover the phase, each worked by hand, run in all eight
modes, and the same on gfortran: `internal`, `hosts`, `recurhost`, `interfaces`, `optional`,
`generics`, `operators`, `hostopt` (the optimizer's cases: a value a loop would keep in a
register, which an internal procedure it calls changes, for each kind of host storage), and
`numbers` (everything together). `autohost` and `modules/shortlist` use DEC's AUTOMATIC and
null arguments, which gfortran doesn't take.

None of them found a fault in the code generator or the optimizer. That is largely because the
design put everything new where an older mechanism already worked. A host's dummy uses ENTRY's
cells, and so does an OPTIONAL dummy. A host's frame cell is reached as section 23's variable
format expressions reach theirs. A generic, an operator, or a defined assignment is an ordinary
call by the time the IR sees it. What the programs did catch was the hand-worked expected
output. `hostopt.f90`'s recursive total was worked wrong once, and govax and gfortran agreed on
the right answer.

## 29. Fortran 90: descriptors and dynamic storage (Phase 104)

Until this phase every array had a place and a shape the compiler could write down: a
variable's own storage, a COMMON block's, or a dummy's, with bounds that were constants or, for
an adjustable array, computed once at entry from the dummies. Fortran 90 takes both away. An
*allocatable* array has no storage until ALLOCATE gives it some, of a shape chosen then. An
*automatic* array is a subprogram's own, of a size its arguments decide. An *assumed-shape*
dummy is whatever its caller passed, a section with any strides included. A function may
return an array whose shape only it can work out. And the scalarizer of section 27, which
used to give up when a temporary's size wasn't a constant, now has to make one anyway.

All of this rests on one data structure, the *array descriptor*, and two ways of finding
memory while the program runs, the stack and the heap. This section takes them in that order,
then shows what each new kind of array does with them.

### The class NCA descriptor

The OpenVMS Calling Standard already describes an array: class A, the descriptor the compiler
has passed for CHARACTER arrays and kept for adjustable arrays since section 12. Its middle
block holds each dimension's extent as a *multiplier*, in elements, and the elements must be
one after another. Class NCA (10, "noncontiguous array") has the same layout to the byte, but
the middle block holds each dimension's *stride*, in bytes, the distance from one element to the
next along that dimension. With strides, any section is an array in its own right.

Here is what the compiler writes to pass `A(2:10:2, J)` of `REAL A(10, 5)` to an assumed-shape
dummy:

```text
	MOVL	@PASS.J(AP),PASS.$T1
	MULL3	PASS.$T1,#10,R0
	SUBL2	#9,R0                       ; A(2,J) is element 10*J - 9 from A(1,1)
	MOVAL	@PASS.A(AP)[R0],PASS.$D1+4  ; POINTER: the section's first element
	MOVL	#8,PASS.$D1+20              ; S1: every other REAL, 8 bytes
	MOVL	#1,PASS.$D1+24              ; L1
	MOVL	#5,PASS.$D1+28              ; U1: five elements
	SUBL3	#8,PASS.$D1+4,PASS.$D1+16   ; A0 = POINTER - S1*L1
	MOVL	#20,PASS.$D1+12             ; ARSIZE
	PUSHAQ	PASS.$D1
	CALLS	#1,G^SHOW
```

Laid beside the array's memory, with A at 1000 and J = 3:

```text
  descriptor $D1                     memory
   0  4         length, REAL*4       1000  A(1,1)
   2  10, 10    DTYPE_F, CLASS_NCA   ...
   4  1084      POINTER  ----------> 1084  A(2,3)   V(1) = A0 + 8*1
   8  0,0,0,1   flags, rank 1        1088  A(3,3)
  12  20        ARSIZE               1092  A(4,3)   V(2) = A0 + 8*2
  16  1076      A0 (V(0)'s place)    1096  A(5,3)
  20  8         S1                   1100  A(6,3)   V(3) = A0 + 8*3
  24  1, 5      L1, U1               ...
                                     1116  A(10,3)  V(5) = A0 + 8*5
```

A0 is where an element of all-zero subscripts would be. With it, an element's address is one
multiply-add per dimension and nothing else: no lower bound to subtract, no element size to
multiply by. In the callee, `V(3)` of `REAL V(:)` is

```text
	MULL3	#3,THIRD.$B1+20,R0          ; 3 * S1
	ADDL2	THIRD.$B1+16,R0             ; + A0
	MOVF	(R0),R0
```

(`ir.ClassNCA`; `Descriptor.StrideOffset`; `lowerallocate.go`'s `stridedElement`). The
optimizer's invariant motion (section 16) takes A0 and the outer dimensions' products out of an
inner loop, so a loop over a section costs about what a loop over a plain array does.

### Why an assumed-shape dummy needs an interface

The callee above reads its argument list's longword as the address of a descriptor. A
FORTRAN-77 caller, which knows nothing of the callee, pushes the address of the first element
instead. Nothing in the call says which the callee got. So the standard requires the caller
to *know*: a procedure with an assumed-shape dummy may be referenced only through an explicit
interface (section 28's), and govax reports EXPLICITREQ for a call of the file's own routine
without one. With the interface, the caller builds the descriptor (`ir/lowershaped.go`'s
`sectionNCA`, `arrayNCA`); without it, it doesn't, and the callee would read an element of
the array as a descriptor.

Two things make the callee robust anyway. Each entry point *copies* the caller's descriptor
into a cell of its own (`$B1` above), with the dummy's own lower bounds. Everything in the
routine then reads one descriptor in one known place, whatever the caller wrote, a MACRO
caller included, and the optimizer can see that nothing else changes it. The copy is also
where a check goes: under `/CHECK=BOUNDS` the class byte and rank are tested first, and
anything else is FTNRTL's BADARRDESC. A class A descriptor from a MACRO caller is accepted
always, its multipliers turned into strides.

### The other direction: a section to an old dummy

A FORTRAN-77 dummy, `REAL X(N)` or `X(*)`, wants the address of elements one after another
(*sequence association*). A section may not be that. Sema decides when it can (`Contiguous`:
whole by a step of 1 up to one dimension, single subscripts after); such a section is passed as
its first element's address, as FORTRAN-77 always did. Anything else is copied into a
temporary before the call and back after it (`ir/lowersequence.go`). An assumed-shape array
is tested while the program runs: if its strides are the contiguous ones (S1 the element size,
each next stride the last times the extent), its POINTER goes straight through, and only
otherwise is it copied. The copy back runs right after the call returns, before the rest of the
statement, so a function reference in an expression sees the effect of its own call.

### Two places to get memory while the program runs

The compiler has always placed storage when compiling: `$LOCAL`, a COMMON block, or since
section 23 a fixed frame area under FP. Run-time sizes need run-time allocation, and there are
two kinds, chosen by how long the memory must live.

*The stack*, for memory that dies with the routine or the statement. The IR has two new
statements, `StackAllocate` (`SUBL2 size,SP`, rounded to a longword, then `MOVL SP,cell`) and
`StackRelease` (`MOVL saved,SP`). An automatic array is made at entry and never released: RET
puts SP back from FP, and so does an unwind. A statement's temporary is made inside the
statement, after `StackMark` saved SP, and given back at the statement's end, so a statement in
a loop doesn't grow the stack. The optimizer treats both as walls: nothing moves across them.

*The heap*, for memory that must outlive a frame or is too big for the stack. ALLOCATE's
memory, a temporary past 64 KB (the user stack is 2 MB with paging off), and a function's
value of run-time shape all come from LIB$GET_VM through FTNRTL (`ftn_alloc.mar`:
`FTN$ALLOCATE`, `FTN$GET_TEMP`, `FTN$DEALLOCATE`). A statement gives its heap temporaries
back at its end (`ir/lowertemps.go`'s `endTemps`), on every way out, including the branch of an
IF, an arithmetic IF, a computed GO TO, and a CALL with alternate returns.

### Allocatable arrays

An allocatable array's storage *is* its class NCA descriptor, in the variable's own place, its
prototype block (length, type, class, rank) known when compiling and POINTER 0 while it isn't
allocated:

```text
B:   .WORD   4                  ; REAL, ALLOCATABLE :: B(:, :)
     .BYTE   DSC$K_DTYPE_F,DSC$K_CLASS_NCA
     .LONG   0                  ; POINTER: not allocated
     .BYTE   0,0,0,2
     .LONG   0,0,0,0,0,0,0,0    ; ARSIZE, A0, strides, bounds
```

ALLOCATE (B(0:M, 3)) is one call, `FTN$ALLOCATE(B, flags, 0, M, 1, 3)`, which computes the
extents, strides, ARSIZE, and A0, takes the memory, and fills the descriptor in; ALLOCATED(B)
is a `TSTL B+4` in line; DEALLOCATE is `FTN$DEALLOCATE`. Putting the work in FTNRTL keeps the
error handling in one place: with STAT= the routine returns the FOR number (301 ALREADYALLOC,
302 NOTALLOC, 41 INSVIRMEM), without it the routine signals. A zero-sized array still gets an
8-byte block, so that POINTER is never 0 for an allocated array.

A subprogram's allocatable local that isn't SAVE'd is deallocated at every return. Each entry
point also *clears* its POINTER. That looks redundant, and is, unless the routine was left by an
unwind, which runs none of the return code. Then the next call would find the array allocated
and its ALLOCATE would fail. Clearing at entry leaks the block instead, and the program goes on.

### Automatic objects

`REAL W(SIZE(A))` in a subprogram is an adjustable array with no dummy to describe it: its
bounds are computed at entry, as section 12's adjustable bounds are, into its own class A
descriptor cell, and then `StackAllocate` makes its storage and fills POINTER and A0. From
there every use reads it exactly as it reads an adjustable dummy's descriptor
(`ir/lowerautomatic.go`). The specification expressions this needs grew by Fortran 90's
inquiry functions: SIZE, LBOUND, UBOUND, and LEN of a dummy, which are lowered at entry like
any expression. `CHARACTER(LEN=N) :: S` is the same with a length and no bounds, a class S
descriptor of its text.

### Temporaries of run-time size

Section 27's scalarizer made a temporary when an assignment's sides overlapped, for a
constructor, for a WHERE's mask, or for `IV(IV) = 1`. When the size wasn't a constant it gave up.
Now `runtimeTemp` makes a class A descriptor cell (`$R1`) standing among the adjustable
arrays, stores the extents the statement computes, and takes the storage from the stack up to
64 KB and from `FTN$GET_TEMP` past that. The rest of the scalarizer doesn't know the
difference: the temporary is just another adjustable array whose bounds are in a descriptor.

### Array-valued functions

A function whose value is an array always has an explicit interface at its references, so
caller and callee agree on how the value comes back: through a hidden first argument, as a
CHARACTER or REAL\*16 value already did. Which argument depends on who can know the shape.

- **Known when compiling** (`REAL F(3)`): the caller passes a temporary of that shape, and the
  function's value is that temporary, a dummy array in all but name.
- **Known only at run time** (`REAL F(N)`): only the function can evaluate its own bounds, which
  may use its COMMON, its host's variables, or SIZE of its dummies. So the caller passes a class
  NCA descriptor with POINTER 0, and the function, at entry, computes its bounds, takes the
  storage with `FTN$GET_TEMP`, and writes the caller's descriptor whole
  (`ir/lowervalue.go`). Inside the function the value is an automatic array of its own cell.
  The caller reads the value as an allocatable array and frees it at the statement's end. The
  caller's descriptor cell (`$G1`) has POINTER 0 whenever no statement holds a value, so a
  call finding a block still there (from an unwind, or from an earlier pass of an implied-DO
  loop in the same statement) frees it first.
- **A CHARACTER value of run-time length**: the Calling Standard's dynamic text. The caller
  passes a class D descriptor, the function gets a string with LIB$SGET1_DD, and the caller
  frees it with LIB$SFREE1_DD, as it frees a concatenation's.

Each reference is called once per statement, however many times lowering needs its value: a
temporary's extents first, then its elements. The statement keeps a map from the reference to
its result for that.

### The transformational functions

SPREAD, CSHIFT, EOSHIFT, PACK, UNPACK, RESHAPE of values, and a reduction with a DIM= known only
at run time are computed whole into a temporary before the statement's loops
(`ir/lowertransform.go`), by loops of their own that walk the argument and put each element
where it goes. CSHIFT's goes to `MOD(j - 1 - SHIFT, n) + 1`; EOSHIFT fills its boundary first;
PACK and UNPACK keep a running count; RESHAPE copies SOURCE and PAD to rank one and walks the
result in ORDER's order; a run-time DIM= is one arm per dimension, each Phase 102's
constant-DIM code. The plan had the simple ones computed where they are used, as TRANSPOSE is,
but a computed subscript (CSHIFT's MOD, EOSHIFT's reach past the end) is something the
scalarizer's leaves don't take, and one mechanism for all of them is safe whatever the
statement's target overlaps.

### What the debugger needs

The `/DEBUG` table (section 18) gained one idea: a variable whose own storage is a descriptor
(`dbgtable.Described`). The debugger reads the descriptor for the first element's address
(POINTER), a CHARACTER length (the first word), the bounds, and, for a class NCA descriptor whose
strides aren't the contiguous ones, each element's address from A0 and the strides. That covers
allocatable, automatic, and assumed-shape arrays, and automatic strings, with one code path.

### What the programs found

Nineteen programs in `testdata/fortran90` cover the phase, from `allocs.f90` to `sorter.f90`,
each run in all eight modes and witnessed by gfortran where it is portable. The VMS-only ones
(`allocvms`, `seqvms`, `rampvms`) check what gfortran can't: STAT='s FOR numbers, ENCODE on an
assumed-shape buffer, and LIB$STAT_VM's count of bytes unchanged by thousands of heap blocks
given back.

The faults they found were all in how statements end, not in descriptors:

- A copy's loops end basic blocks, and a block's end ran the statement's pending frees, so a
  dynamic string an earlier argument of the same call had made was freed before the call
  (`holdingAfter`, subtask 6).
- In an internal procedure the optimizer's map of the host's storage was made once, so a
  temporary a later pass added looked like the host's, which any call may change
  (`opt/effects.go`, subtask 7a).
- A CALL with alternate returns ended its block with its Switch, and its statement's heap
  temporaries were never given back (subtask 7b).
- A transformational function's shape was a slice of its argument's, so marking the function's
  own extents rewrote the argument's (subtask 8).

And one was in the tests: subtask 6's heap check copied a 30,000-element section a thousand
times, 690 million instructions in each of eight modes, and twenty copies prove the same thing.

## 30. Fortran 90: derived types (Phase 105)

Section 13 showed DEC's STRUCTURE and RECORD: a layout is a table of offsets, a field
reference is arithmetic on an offset, and an aggregate assignment is a byte move. Fortran 90's
derived types are the same idea with a different syntax and stricter rules, and the compiler
treats them as the same idea. This section shows where the two meet, and the few places where
Fortran 90 asked for something new: what makes two types one, an array of components, values
of a type (constructors and function results), and I/O, where FTNRTL knows nothing of types.

The program the section follows:

```fortran
program tour
  type point
    real :: x, y
  end type point
  type(point) :: p, path(6)
  namelist /where/ p
  path%y = 2.0 * path%x
  p = mid(path(1), path(6))
  write (*, '(2f6.2)') p
  write (*, where)
contains
  function mid(a, b)
    type(point), intent(in) :: a, b
    type(point) :: mid
    mid = point((a%x + b%x) / 2, (a%y + b%y) / 2)
  end function mid
end program tour
```

### One house, two front doors

The parser reads `TYPE POINT ... END TYPE` into an `ast.TypeDefStmt`, and sema
(`sema/derived.go`) turns it into the same `Structure` a STRUCTURE declaration becomes, with
two flags: `Derived`, and `Sequence` if the definition has a SEQUENCE statement. From there
on the type is a table of `Field`s with offsets, and everything section 13 built works on it:
`fieldMem` (`ir/lowerfield.go`) computes `P%X` exactly as it computes `R.X`, a record
assignment is `moveBytes`, and `/ALIGN` lays a type out with the same routine. `%` and `.` are
two spellings of one selector; the parser builds an `ast.FieldRef` for both, and records which
was written only so messages can quote it.

The type itself is new: `sema.Type` gained `Struct`, the structure a record or object is of. A
RECORD and a TYPE(POINT) object of the same layout are of different types (`sameType`), and
two TYPE(POINT) objects are of the same type only if they name the same definition, which is
where the standard's rules come in.

### What makes two types one

A derived type is a *name* for a definition, not a layout. Within one scoping unit that is
simple: one definition, one type. Across units it isn't. Two external subroutines that each
say `TYPE POINT; REAL :: X, Y; END TYPE` have two types that happen to look alike, and the
standard says an object of one may not be passed to a dummy of the other. The exception is a
SEQUENCE type: two SEQUENCE definitions with the same name and the same components, in the same
order, are one type (4.4.2).

That rule is why every interface body in `geometry.f90` repeats VEC's definition with SEQUENCE.
An interface body has no host (section 28), so it can't see the program's definition; it must
define the type again, and only SEQUENCE makes the two definitions one. sema checks it where a
call meets a definition in another unit of the file (`recordArguments`, SEPARATEARG,
SEPARATEFUNC), and `sameType` checks it within a unit, in assignment, in INTERFACEDIFF, and in
an array constructor's values. COMMON and EQUIVALENCE take only SEQUENCE objects for the same
reason (NOSEQUENCE): another unit sees the storage through its own definition, and only
SEQUENCE promises the layouts agree.

### A component with a rank is just another stride

`PATH%X` is the X of every element: an array of rank 1, extent 6, whose elements are 8 bytes
apart, the size of a POINT. The scalarizer (section 27) already walks arrays whose elements
are any distance apart (a section by a step of 2 is one), so a ranked component needs no new
loop machinery. sema gives the `FieldRef` a `Ranked` part, the step of the path that carries
the rank, and its section subscripts; the scalarizer asks it for "your element at these
indexes" (`FieldRef.At`), a scalar FieldRef that `fieldMem` reaches as any other:

```text
	MOVL	#1,R7
	MULL3	R7,#2,R8
TOUR.$1:				; array (line 7), dimension 1's body
	MOVL	R8,R6
	MULF3	TOUR.$KF1,TOUR.PATH-8[R6],TOUR.PATH-4[R6]
	ADDL2	#2,R8
	AOBLEQ	#6,R7,TOUR.$1
```

The index register steps by 2 longwords, a POINT's size, and the two operands differ only in
their displacement, the field's offset. Passing `PATH%X` to an assumed-shape dummy is the
same observation once more: its class NCA descriptor (section 29) gets the type's size as its
stride, and the callee works on the components where they are. Passing it to an old dummy,
which wants elements one after another, copies it.

### Values of a type

A *structure constructor*, `POINT(1.0, 2.0)`, is a value with no name. If all its values are
constants it folds to a `Const` with `Parts` (its components) and `Raw` (its bytes), which can
be a PARAMETER, an initial value, or a DATA value, and whose components fold too
(`ORIGIN%X`). Otherwise it is a `StructValue`, which lowering builds a component at a time
(`buildRecord`), in the assignment's target when no value reads the target, else in a
temporary.

A function's value of a derived type comes back the way the Calling Standard returns any value:
in R0, or R0 and R1, when it fits in 8 bytes (`ir/lowerresult.go`). `MID` keeps its value in a
quadword cell and loads it at its return, and the caller stores it straight into P:

```text
	MOVQ	TOUR.MID.MID,R0		; the function's value
	...
	CALLS	#2,TOUR.MID
	MOVQ	R0,TOUR.P
```

Past 8 bytes the value goes through a hidden first argument, the address of a place for it,
as a COMPLEX\*16's does; `accounts.f90`'s 18-byte ACCOUNT is one.

A derived type has no operators of its own. `P == Q` is an error unless an
`INTERFACE OPERATOR(==)` block gives it a meaning, and then it is a function call (section 28);
`P = Q` is a byte move unless an `INTERFACE ASSIGNMENT(=)` block takes it over.

### I/O: the compiler takes objects apart

FTNRTL's I/O statements take items, each a descriptor of a scalar or a contiguous array of an
intrinsic type. A derived-type object is neither, and FTNRTL didn't change to take one: the
compiler gives it the object's *ultimate components* instead, the components left when every
component of a derived type has been taken apart in turn (the standard's 9.4.2).
`sema.Components` lists them as FieldRefs on the object, and `ir/lowerio.go`'s `objectItems`
calls `FTN$IO_ITEM` once for each:

```text
	CALLS	#3,G^FTN$WRITE_SF
	PUSHAQ	TOUR.$D2		; P%X
	CALLS	#1,G^FTN$IO_ITEM
	PUSHAQ	TOUR.$D3		; P%Y
	CALLS	#1,G^FTN$IO_ITEM
	CALLS	#0,G^FTN$IO_END
```

An array component is one item, a whole array field's class A descriptor; an array of a type is
each element's components, element by element, through the scalarizer's `eachElement`; a
constructor's values are items as they are; and a value with no place (a function's) is put in
one first. An unformatted statement is the other way round: its form is the processor's to
choose, and an object goes as its bytes, padding and all, as a record always did.

NAMELIST is where FTNRTL had to learn something, since a group's block names its entities and
the program's input names them back. An object becomes an entity per ultimate component,
named by its designator, after a *header* entity for the object itself:

```text
TOUR.$D4:
	.WORD	2			; length
	.BYTE	DSC$K_DTYPE_Z,DSC$K_CLASS_Z
	.LONG	0			; a NAMELIST object's header: its entities follow
TOUR.$NWHERE:
	.ADDRESS TOUR.$K2		; NAMELIST /WHERE/'s block: its name
	.LONG	3			; its entities
	.ADDRESS TOUR.$K3,TOUR.$D4	; P
	.ADDRESS TOUR.$K4,TOUR.$D2	; P%X
	.ADDRESS TOUR.$K5,TOUR.$D3	; P%Y
```

The block keeps VMS's layout (section 14's "NAMELIST: a statement with no items"). The header's descriptor has class 0, which no real
descriptor uses, and says in its length how many entities after it are the object's. Output
passes over headers, so `P%X` and `P%Y` are written as any two variables are. Input reads `%`
as part of a name, so `P%X = 9.0` finds its entity; and `P = 1.0, 2.0` finds the header, makes
its first entity the current one, and goes on to the next where an array's values would go on to
its next element, until the header's count runs out and one more value is TOOMANVAL
(`ftn_namelist.mar`'s `NEXT_PART`).

### What the programs found

Fourteen programs in `testdata/fortran90` cover the phase, from `types.f90` to `accounts.f90`,
in all eight modes, and `testdata/fortran`'s `recarrays.for` and `recdyn.for` hold STRUCTURE's
records to the forms they share with derived types. gfortran agrees with every portable one.

Most of what they found was a seam between the two front doors, places where code written for
records assumed something a derived type doesn't promise, or the reverse: a constant's
component accepted as an assignment's target, which would have stored into read-only data; an
array constant's component taken for an error already reported; RECORD-versus-TYPE comparisons
that compared layouts where the standard compares definitions. The last, in subtask 8, was in
the parser: after a `)` the lexer can't tell `.ROT.` from DEC's field selector, and so leaves
the periods apart, and only the binary operator was put back together; `WRITE (*, F) .ROT. V`
was a syntax error until the unary reading did the same.

## 31. Fortran 90: modules and separate compilation (Phase 106)

Every compiler so far in this tour compiled one file and forgot it. A unit learned about
another unit of the same file (an interface, a function's type), but nothing crossed from one
compilation to the next except global names for LINK to match, and a mistake there (a
subroutine called with the wrong arguments) was found, if at all, when the program ran. A
Fortran 90 *module* changes that. `USE SHAPES` asks for everything SHAPES declared: its types,
constants, variables, interfaces, and procedures, with their exact types, kinds, and
arguments, and it asks for them whether SHAPES is earlier in this file or was compiled last
week. So a module compilation leaves something behind for the next one to read, and that is
this section's subject: what it leaves, how a user reads it, and how the result still links
into an ordinary VMS image.

The program the section follows, split into two files compiled one after the other:

```fortran
module shapes                        ! shapes.f90
  implicit none
  private
  public :: point, origin, moves, shift, dist
  type point
    real :: x, y
  end type point
  type(point), parameter :: origin = point(0.0, 0.0)
  integer :: moves = 0
  real :: scale = 1.0
  interface shift
    module procedure shift_both, shift_x
  end interface
contains
  function shift_both(p, dx, dy)
    type(point), intent(in) :: p
    real, intent(in) :: dx, dy
    type(point) :: shift_both
    moves = moves + 1
    shift_both = point(p%x + scale*dx, p%y + scale*dy)
  end function shift_both
  function shift_x(p, dx)
    type(point), intent(in) :: p
    real, intent(in) :: dx
    type(point) :: shift_x
    shift_x = shift_both(p, dx, 0.0)
  end function shift_x
  real function dist(p, q)
    type(point), intent(in) :: p, q
    dist = sqrt((p%x - q%x)**2 + (p%y - q%y)**2)
  end function dist
end module shapes
```

```fortran
program tour                         ! prog.f90
  use shapes, only: point, origin, shift, steps => moves, dist
  type(point) :: p
  p = shift(shift(origin, 3.0, 1.0), 1.0)
  print '(1x, 3f6.2, i3)', p, dist(p, origin), steps
end program tour
```

```text
$ govax fortran shapes.f90           ! writes shapes.mar and shapes.f90$mod
$ govax fortran prog.f90             ! reads shapes.f90$mod, writes prog.mar
$ govax macro shapes.mar
$ govax macro prog.mar
$ govax link prog.obj shapes.obj
$ govax run prog.exe
  4.00  1.00  4.12  2
```

### A module is a unit with no code

sema compiles a MODULE as it compiles a host with internal procedures (section 28), by the same
four steps (`analyzeContained`): the module's declarations, each module procedure's
declarations and so its interface, the host's statements (a module has none), then each
procedure's statements. The module procedures see the module's names by host association,
exactly as an internal procedure sees its host's; `SHIFT_BOTH` reaches `MOVES` and `SCALE`
that way. What differs is that a module has no code of its own and its procedures are global.

A module's variables have to live somewhere every user can reach, for the whole run. govax
already had such a thing: a COMMON block, a psect every unit that names it contributes to and
LINK overlays (section 8). So a module's variables *are* a common block (`moduleBlock`), with
a psect name no FORTRAN name can take, `$MOD$SHAPES`, laid out by COMMON's code and initialized
the way a COMMON block's DATA is. They are static even under `/RECURSIVE`: the standard lets a
processor keep a module's variables for the whole run, and nothing else would make sense for a
psect. The IR makes the module a routine of no code (`ir.Module`, as BLOCK DATA is), so
codegen writes its section and nothing more:

```text
	.PSECT	$MOD$SHAPES,PIC,OVR,REL,GBL,SHR,NOEXE,RD,WRT,LONG
$MOD$SHAPES.$BASE:
	.LONG	0			; MOVES = 0
	.LONG	^X00004080		; SCALE = 1.0

SHAPES.MOVES = $MOD$SHAPES.$BASE	; MOVES, in module SHAPES's variables
SHAPES.SCALE = $MOD$SHAPES.$BASE+4	; SCALE, in module SHAPES's variables
```

Module procedures get global names, `SHAPES.SHIFT_BOTH`, so they never collide with an external
`SHIFT_BOTH` of the program or another module's (`Unit.LinkName`). The period can't appear in
a FORTRAN name, which is what makes the form safe, and VMS's own symbols use `$`, not `.`. A
name past MACRO's 31 characters can't be numbered as local symbols are (`.$Nn`, counted through
one MACRO file), since the module and every user must arrive at the same name without seeing
each other's files; `LongName` makes it from the full name instead: its first 22 characters,
`$`, and 8 hexadecimal digits of the full name's SHA-256. The same rule shortens a long
module's psect name.

A call of a module procedure is an ordinary external call, `CALLS #3,G^SHAPES.SHIFT_BOTH`,
with nothing hidden: a module has no stack frame, so unlike an internal procedure's call
(section 28's R10) there is no host frame to pass.

### What a module leaves behind: its text

When SHAPES compiles without an error, sema writes down what a user may need of it
(`sema/modwrite.go`), and the FORTRAN command puts that in `SHAPES.F90$MOD` beside the MACRO
file. Here is the whole of it:

```text
!GOVAX$ MODULE FILE 1
!GOVAX$ COMPILER GOVAX FORTRAN V0.0-0
!GOVAX$ SOURCE .../tour/shapes.f90 19-NOV-2026 08:30:53.79
!GOVAX$ FLOATING D
!GOVAX$ OBJECT
MODULE SHAPES
!GOVAX$ PSECT $MOD$SHAPES SIZE 8 ALIGN 0
!GOVAX$ TYPE POINT SIZE 8 BOUNDARY 4 NATURAL
  TYPE POINT
    REAL(KIND=4) :: X
    REAL(KIND=4) :: Y
  END TYPE POINT
  TYPE(POINT), PARAMETER :: ORIGIN = POINT(0E+00_4, 0E+00_4)
!GOVAX$ AT MOVES 0 4
  INTEGER(KIND=4) :: MOVES
!GOVAX$ AT SCALE 4 4
  REAL(KIND=4) :: SCALE
  INTERFACE SHIFT
    MODULE PROCEDURE SHIFT_BOTH, SHIFT_X
  END INTERFACE
  PRIVATE :: SCALE, SHIFT_BOTH, SHIFT_X
CONTAINS
!GOVAX$ LINK SHIFT_BOTH SHAPES.SHIFT_BOTH
  FUNCTION SHIFT_BOTH(P, DX, DY)
    TYPE(POINT), INTENT(IN) :: P
    REAL(KIND=4), INTENT(IN) :: DX
    REAL(KIND=4), INTENT(IN) :: DY
    TYPE(POINT) :: SHIFT_BOTH
  END FUNCTION SHIFT_BOTH
!GOVAX$ LINK SHIFT_X SHAPES.SHIFT_X
  FUNCTION SHIFT_X(P, DX)
  ...
END MODULE SHAPES
```

It is Fortran 90. Leave out the `!GOVAX$` lines, which are comments to any other reader, and
what remains is a module a Fortran compiler could compile: the specification part, written out
so nothing depends on the module's IMPLICIT rules, `/I4`, or kinds by default
(`REAL(KIND=4)`, never `REAL`); each PARAMETER's value folded (a floating one in as many digits
as bring back its exact bits); and after CONTAINS each module procedure as an *interface*, a
header and its dummies' declarations with no statements, which is what Fortran calls a
procedure body with nothing in it. The procedures' code isn't there because a user doesn't
need it: it is in the module's object.

The comments carry what Fortran can't say. `PSECT` and `AT` give the section's size and each
variable's offset, `TYPE` a type's size and alignment, `LINK` a procedure's global name,
`FLOATING` whether REAL\*8 meant D or G, and (once modules use modules) `FROM` where an entity
came from. Their reader checks every one against what analysis computes from the declarations,
so a file edited by hand, or written by another version of govax, is caught
(`MODFILEBAD`, naming the line) rather than believed.

Why text, and why Fortran? The other choice was a record format of our own with a reader of its
own, which would be one more grammar to keep in step with the compiler. Fortran text is read
by the compiler's own passes, which already know every declaration; and a person can read it,
which is how the format can be explained in one listing.

### Every USE reads text

Here is the design's central decision. When a user says `USE SHAPES`, sema doesn't look
SHAPES up in some table of analyzed units, even when SHAPES was compiled a moment ago in the
same file. It reads SHAPES's text (`sema/modread.go`): the `!GOVAX$` lines into `moduleFacts`,
then the compiler's own source pass, lexer, parser, and semantic analysis of that one unit, in a
mode that makes no code (`analyzer.reading`). The text comes from memory when the module is in
this compilation, and from its file when it isn't (the FORTRAN command's `fortranModules`, a
`sema.ModuleFinder`). So a module in the same file and a module compiled last week go down one
path; every test of a one-file program is also a test of the writer and reader; and a stale
module file on the disk can never stand in for a module just compiled.

The read leaves a *module scope*, the module's names and entities. `USE` then enters what the
user asked for into the *user's own* scope (`sema/use.go`'s `useModule`): each a copy of the
module's symbol, marked `Symbol.Use` with its module and its remote name, under the local name
the user gives it. Being in the user's own scope is what the standard's rules need. A
use-associated name hides a host's name of the same spelling (12.1.2.2.1); a later type
statement or DIMENSION for it finds it there and is `USEATTR`, since only the module may say
what it is; COMMON or EQUIVALENCE of it is `USECOMMON`.

Each kind of entity goes in its own way:

- A **variable** is reached through the user's own view of the module's section
  (`Unit.UsedBlocks`). The user's MACRO file contributes to `$MOD$SHAPES` as any COMMON
  block's user does, its size from the `PSECT` line, and names the variable at its offset
  from the `AT` line, under the user's local name:

  ```text
  	.PSECT	$MOD$SHAPES,PIC,OVR,REL,GBL,SHR,NOEXE,RD,WRT,LONG
  $MOD$SHAPES.$BASE:
  	.BLKB	8

  TOUR.STEPS = $MOD$SHAPES.$BASE		; STEPS, in module SHAPES's variables
  ```

  The rename `STEPS => MOVES` is nothing at run time: the user's name for the same bytes.
- A **type** is the module's own `Structure`, one object however many paths reach it, since
  `sameType` (section 30) compares definitions by identity (`readType`; the compilation keeps
  one table of the types its module texts gave).
- A **module procedure** is a symbol with an explicit interface (the text's body) and a link
  name (the `LINK` line); every call is checked against the interface as an internal
  procedure's is.
- A **generic name or operator** brings its specifics with it, the user's copies of them.

`SHIFT` shows why that last rule matters. `SHIFT_BOTH` and `SHIFT_X` are PRIVATE: the user may
not name them. But it may call them through the public generic `SHIFT` (5.2.3), so the module's
text carries them, the user's copies are *hidden* (`Unit.hidden`, in the scope but not under
any name the program can write), and resolution (section 28) picks one as usual:

```text
	CALLS	#3,G^SHAPES.SHIFT_BOTH
	...
	CALLS	#2,G^SHAPES.SHIFT_X
```

`PRIVATE` stopped the user from writing the name; it never stopped the call.

### PUBLIC, PRIVATE, ONLY, and the rest of association

Access belongs to a *name in a module*, not to an entity (`sema/access.go`). A module's default
is PUBLIC unless a bare `PRIVATE` statement says otherwise, as SHAPES's does, and statements and
attributes then mark names one by one; `resolveAccess` settles them once the module's procedures
and generics are known, so `PRIVATE :: SHIFT_X` can come before `SHIFT_X` is defined. The checks
are the standard's: a public entity may not be of a private type (`ACCESSTYPE`), nor a public
module procedure have an argument or value of one (`ACCESSPROC`), nor a public namelist group a
private item (`ACCESSNML`), nor a public type a component of one unless its components are
PRIVATE (`ACCESSCOMPONENT`); a type with a PRIVATE statement in its definition is a type whose
components and constructor only its module may touch (`COMPPRIVATE` elsewhere).

A user's `ONLY` list and renames choose among the public names. Several USEs of one module in
a unit join as 11.3.2 says (`useRequest`): one without ONLY gives everything, the renames
together; all with ONLY give the union of the lists.

When two USEs give one local name, there are two cases. If both reach the same entity, through
different paths (module M re-exports J's `X`, and the unit uses both J and M), it is one name:
every copy carries its entity's *origin*, the module that defined it and its name there
(`entityOrigin`), and two copies of one origin are one. If they reach different entities, the
standard makes it an error only when the program uses the name (11.3.2), so sema marks the name
ambiguous and says nothing until `lookup` finds a reference:

```text
%FORT-E-USEAMBIG, N can't be referenced here: the modules A and B each give an entity of that name
  at line 10 of AMB.F90
    print *, n
  -----------^
```

Generic names are the exception the standard makes: generics of one name from several modules
join into one, and `checkGenerics` (section 28) holds the joined list to 14.1.2.3's rules in the
user, so two modules' SWAPs that can't be told apart are reported at the second USE.

### Modules that use modules

A module's text is *self-contained*: it carries everything its users can reach, including what
it has from other modules, each such entity with a `FROM` line naming its origin. A user of M
needs M's file and no other, even when M re-exports J's types. If the user also uses J, J's
file is read too, and its `POINT` and the one M passed on are the same `Structure`, matched by
origin. If the two files disagree about it (J recompiled with a changed POINT, M not), the user
gets `MODSTALE` naming both, the one error that can explain all the others such a mixture
would cause.

A module can't use itself, directly or through others (`MODCYCLE`), and in one file a module
must come before its users (`MODLATER`, naming the line where it is): the compiler reads a file
once, from the top, as the standard expects of a processor.

### Separate compilation, and keeping the pieces in step

`MODNOTFOUND` is what a user compiled before its module gets, listing every place
`fortranModules` looked: this compilation's own module directory (`/MODULE`, else the default
directory where the MACRO file goes), the source's directory, then `/MODULE_PATH`'s directories
or the translations of `FORT$MODULES`, each name on the host in both cases:

```text
%FORT-E-MODNOTFOUND, no module file for SHAPES; looked in .../nf/shapes.f90$mod,
    .../nf/SHAPES.F90$MOD, shapes.f90$mod, SHAPES.F90$MOD
```

Two rules make separate compilation behave. First, a module file is written only when its
contents change (`writeModuleFiles`, comparing `sema.ModuleDigest`, a SHA-256 of the text
without its COMPILER and SOURCE lines), so an edit to a module procedure's body, which changes
nothing a user can see, leaves the file's date (or its version on a volume) alone, and a
procedure that rebuilds what is older than its inputs rebuilds only the module. No module file
is written by a compilation with an error.

Second, the objects must agree. A user's code bakes in offsets and sizes from the text; linked
with the object of a module since changed (a variable added before `MOVES`), it would read the
wrong bytes without a word. VMS shareable images have GSMATCH for this; modules get a *stamp*.
Each module with an object (one with variables or procedures, the `OBJECT` line) defines a
global symbol named for its text's digest (`sema.StampName`, codegen's `writeStamps`), and each
user's object refers to it:

```text
$MOD$SHAPES.2018EFBA == 0		; this module's stamp, which its users' objects refer to
...
	.EXTRN	$MOD$SHAPES.2018EFBA	; the stamps of the modules used
```

Change the module's interface, compile and assemble it, link it with the user's old object,
and LINK can't find the symbol the user asked for:

```text
%LINK-W-NUDFSYMS, 1 undefined symbol:
%LINK-I-UDFSYM, 	$MOD$SHAPES.2018EFBA
```

That costs nothing at run time (the symbol is an absolute 0 nothing reads) and needed nothing of
LINK. A module of only types, constants, and interfaces has no object, so its text has no
`OBJECT` line and its users refer to no stamp.

One of a unit's options does matter to its users: `/G_FLOATING`. `sema.Type` says REAL\*8, not D
or G, which is a unit's choice (section 11). A module compiled `/G_FLOATING` and used by a
`/NOG_FLOATING` unit would hand it G values to read as D, so a USE across the two is `MODFORMAT`
when the module has a REAL\*8 or COMPLEX\*16 entity anywhere (`formatMatters`). Every other
option is written into the text as the sizes and offsets it produced, and so can't disagree.

### Use association beside host association

Section 28's host association and this section's use association both make another unit's names
visible, and it is worth seeing why the compiler implements them so differently. A host and its
internal procedures are compiled together, in one pass over one file, and an internal procedure
may reach its host's *frame* (R10); so host association is a scope chain, `find` walking
outward, and nothing is copied. A module may have been compiled anywhere, by another run of the
compiler; it has no frame, only static storage and global names; and its user must be able to
rename, restrict, and combine what it takes. So use association copies: the module's entities
become the user's own names, read from a text, placed by its comments, and checked against
other copies by origin. The scope chain then works exactly as before, since a use-associated
name is simply local.

### What the programs found

`testdata/fortran90` has `moddata.f90`, `modprocs.f90`, and `modaccess.f90`, each a module
and its users in one file; `testdata/fortran90/modules/standard` is the standard's own examples
(11.3.3 and 12.1.2.2.2) made to run, and `modules/polys` a library of three modules in a chain,
a re-exporting module, a user with ONLY and a rename, and a fixed-form `/NOF90` user. Each
multi-file program is compiled in every order its `compiles.txt` allows and linked in every
order its `links.txt` lists, in every mode, and gfortran gives the same output.

Building them up found the seams you might expect between the module machinery and what was
there before: a SAVE of a module variable refused as a COMMON member's (`SAVECOMMON`), a
module's allocatable array with no descriptor in its section, interface bodies in a module's
text that name the module's own types and had no host to find them in, and, unrelated to
modules, two code generator names colliding (`$Fn` for both a FORMAT's block and a floating
constant, now `$KFn`) and the lexer reading `.AT. 2.0`'s period as a number's. Each is held by a
test of its own (`internal/fortran/modprograms_test.go`).

## 32. Fortran 90: pointers and aliasing (Phase 107)

A FORTRAN-77 variable *is* its storage: a name, a place in `$LOCAL` or a common block or a
frame, and every reference to the name reaches that place. A dummy argument was already a
little different (section 12: the place is wherever the caller's argument list says), but the
association was fixed for the whole call. A Fortran 90 *pointer* is a variable whose place
can change while the program runs: `P => T` makes P stand for T, a later `P => U` for U,
`NULLIFY (P)` for nothing. That one idea asks something of every pass. sema must decide, at
each mention of P, whether the program means the pointer or the thing it points at. The IR
must reach a target through an address it loads. FTNRTL must allocate memory that belongs to
no variable. And the optimizer, which until now could see every place a store might change,
must reason about a store through an address it can't see, which is what the TARGET
attribute exists for.

The program the section follows builds a linked list and walks it:

```fortran
program tour
  type node
    integer :: value
    type(node), pointer :: next
  end type node
  type(node), pointer :: head, p
  integer, target :: t
  integer, pointer :: q
  integer :: i, k, total
  nullify (head)
  do i = 1, 3
    allocate (p)
    p = node(i * 10, head)
    head => p
  end do
  total = 0
  p => head
  do while (associated (p))
    total = total + p%value
    p => p%next
  end do
  t = 1
  k = 2
  q => t
  do i = 1, 3
    q = q + k
  end do
  print *, total, head%next%value, t
end program tour
```

It prints `60 20 7`. Every listing in this section is govax's own `/OPTIMIZE` output.

### Three forms of a pointer

A pointer's storage holds where its target is, in the least that can say it (the plan's
Decision 12):

- a pointer to a scalar of any type but CHARACTER is a **longword**, the target's address, or 0
  when it points at nothing (*disassociated*);
- a CHARACTER pointer is a **class S descriptor**, 8 bytes: the target's length and address,
  exactly what a CHARACTER dummy argument is given (section 12), so a pointer passed to such a
  dummy passes its own descriptor;
- an array pointer is a **class NCA descriptor** (section 29), its POINTER 0 when disassociated,
  the same 20 bytes plus 12 for each dimension an allocatable array has.

`sema.Symbol.Pointer` marks a pointer, `ArrayPointer` an array one, and `PointerSize` is its
storage's size. The storage map says what each is:

```text
; Storage map for TOUR, the main program
;   HEAD   TYPE(NODE), POINTER  $LOCAL+0   TOUR.HEAD
;   P      TYPE(NODE), POINTER  $LOCAL+4   TOUR.P
;   T      INTEGER              $LOCAL+8   TOUR.T
;   Q      INTEGER, POINTER     $LOCAL+12  TOUR.Q
...
;   Type NODE, 8 bytes
;     +0     VALUE  INTEGER
;     +4     NEXT   TYPE(NODE), POINTER
```

A NODE is 8 bytes because NEXT is an address, not a NODE; that is why a type may name itself
only through POINTER (TYPESELF otherwise). Every walk over a structure's fields that would
follow a field's type (laying it out, comparing two types, writing a module's text) stops at a
pointer component, which may lead back to the structure being walked.

Fortran 90 calls a pointer's status *undefined* until something sets it. govax's pointers all
start disassociated, Fortran 95's rule: static storage starts at zero, and a pointer in a stack
frame (a RECURSIVE or AUTOMATIC unit's, or a function's pointer value) is cleared at every entry
point (`ir/lowerpointer.go`'s `clearPointers`, called where `clearAllocatables` fills in
descriptors). So are the pointer components of records in a frame (`Structure.PointerOffsets`,
at any depth; `clearRecords` loops over an array of them), and the memory ALLOCATE makes for a
type with pointers in it, by FTNRTL's clear flag below. The price is a few CLRLs per entry; the
gain is that `ASSOCIATED` means something from the first statement.

### One name, two meanings

`P = 5` changes P's target; `P => T` changes P. The parser can't tell which a mention means
(it reads `P` either way), so sema decides by where the mention stands, as it decides whether
`X(1:3)` is a substring or a section (section 27). In `=>`, NULLIFY, ASSOCIATED, ALLOCATE,
DEALLOCATE, and as the actual argument to a pointer dummy, a mention is the pointer; everywhere
else (an expression, the left of `=`, an I/O list, an internal file, an actual argument to any
other dummy) it is the target. sema keeps it a `Var` of the pointer's symbol in both cases, and
the IR, which sees the symbol's `Pointer`, sends the second kind to `pointerTargetMem`: a `Mem`
whose `Ptr` is the pointer's longword (a CHARACTER pointer's descriptor's POINTER). VAX
deferred addressing does the rest:

```text
;   19      total = total + p%value
	ADDL2	@TOUR.P,R6
```

`@TOUR.P` is the longword at P, used as an address: the target. Because the IR's `Mem` already
had a run-time address form for dummies and descriptors, nothing downstream of `symbolMem`
learned anything new: every expression, assignment, I/O item, and actual argument that worked
on a variable works on a pointer's target. Passing a pointer to an ordinary dummy is `PUSHAL
@P`, the target's address, at no cost.

An array pointer's elements are reached as an assumed-shape dummy's are (section 29), through
every stride, since its target may be a section. `Symbol.Strided` says so for both, and every
place that once asked "is this array assumed-shape?" to decide how to reach an element, or
whether it is known contiguous, now asks `Strided`. The descriptor *is* the pointer's storage,
so unlike an assumed-shape dummy there is no copy at entry.

### Pointer assignment

`sema.PointerAssign` checks 7.5.2: the left side a pointer (NOTPOINTER), the right a variable
whose base is a TARGET or a pointer (NOTTARGET), not a vector-subscripted section
(VECTORTARGET), of the same type, kind, and rank, and length when both lengths are constants
(POINTERTYPE). Then lowering is one of three things. A scalar's address is stored: `q => t` is
`MOVAL TOUR.T,TOUR.Q`, and `head => p` copies one pointer's longword into another, `MOVL
TOUR.P,TOUR.HEAD`. A CHARACTER pointer gets its header and then its address. An array pointer's
descriptor is filled in (`arrayPointerAssign`), from another pointer's (`copyDescriptor`) or by
describing the target (`describeTarget`, over `fillNCA`, the same routine that builds the
descriptor an assumed-shape dummy is passed):

```fortran
  integer, target :: m(3, 4)
  integer, pointer :: row(:)
  row => m(2, :)
```

```text
;    6    row => m(2, :)
	MOVAL	ROWS.M+4,ROWS.ROW+4	; POINTER: M(2,1)
	MOVL	#12,ROWS.ROW+20		; the stride: a column, 12 bytes
	MOVL	#1,ROWS.ROW+24		; bounds 1 to 4
	MOVL	#4,ROWS.ROW+28
	SUBL3	#12,ROWS.ROW+4,ROWS.ROW+16	; A0, element 0
	MOVL	#16,ROWS.ROW+12		; the size in bytes
```

(The comments are this tour's; the compiler's own are the storage map's.) ROW's elements are
now M(2,1), M(2,2), ..., 12 bytes apart, and `ROW = ROW * 10` changes the second row of M
without a byte copied. A pointer to a whole array keeps the array's bounds; one to a section
counts from 1, as the standard says. `P => P(9:1:-1)` describes its target from P itself, so
the description is built in a temporary first and then copied in. `NULLIFY` stores 0 in the
address or the descriptor's POINTER. `ASSOCIATED(P)` tests it; `ASSOCIATED(P, T)` compares
addresses, and for arrays the whole description, POINTER, each stride, and each extent, with a
target of no elements never associated (the author's answer 4, gfortran's choice).

### Memory that belongs to no variable

`ALLOCATE (P)` gives a pointer a target of its own. An array pointer goes through Phase 104's
`FTN$ALLOCATE` with a new flag; a scalar one through a new routine, since there is no
descriptor to fill in, only a longword:

```text
;   12      allocate (p)
	PUSHL	#8			; a NODE's size
	PUSHL	#6			; flags: a pointer's (2), cleared (4)
	PUSHAL	TOUR.P			; where the address goes
	CALLS	#3,G^FTN$ALLOC_SCALAR
```

Flag 2 says the block is a pointer's, flag 4 that the type holds pointers, so the block is
cleared and the new node's NEXT starts disassociated. An allocatable array's memory has one
owner, the array, and DEALLOCATE of it is always of the whole thing. A pointer's has none:
`P => A` makes P point at an allocatable array, or at a TARGET local, or at the middle of
another pointer's block, and `DEALLOCATE (P)` of any of those would hand LIB$FREE_VM memory it
never gave out. The standard calls that undefined; most compilers damage the heap. FTNRTL
catches it (the plan's open question 1, the author's answer 1): a block made for a pointer has
an 8-byte header before its first byte, a mark (the header's own address XOR `PTR_MARK`, so a
copied header doesn't pass) and the size asked for. `FTN$DEALLOC_SCALAR` and `FTN$DEALLOCATE`
with the pointer flag check, in `ftn_alloc.mar`'s `CHECK_BLOCK`, that the header can be read
(PROBER: a TARGET may be the first thing in a program's storage), that the mark is right, and
that the target is the whole block (for an array, contiguous and in order), and signal the new
`%FOR-F-BADDEALLOC`, FOR number 304, if not. Allocatable arrays' blocks have no header, so a
pointer to one is caught too.

### Components, and descriptors that aren't cells

`HEAD%NEXT%VALUE` goes through two pointers on its way. Lowering's `fieldPlace` (section 30's
`fieldMem`, renamed) walks the path; at each pointer component the place so far is a pointer's
longword, which it loads, and the walk goes on from offset 0 there:

```text
;   20      p => p%next
	MOVL	TOUR.P,R0
	MOVL	4(R0),TOUR.P
```

The scalar case was that simple because a scalar pointer component is a longword at an offset.
An *array* pointer component is a descriptor at an offset, and here Phase 104's design pushed
back: the IR reached every descriptor, in some 90 places (elements, sections, bounds, the
scalarizer's extents, `=>`, ALLOCATE, ASSOCIATED), as a *cell*, `Mem{Cell: c, Offset: n}`. Rather
than teach each of them a second way, Phase 107 made a reference to such a component a symbol of
its own in sema (`sema/pointercomp.go`'s `componentPointer`: `H%V` is read as a named array
pointer would be), and in the IR a **located cell**: a cell with no storage, only a statement of
where its storage is (`Cell.At`, a record's place; `Cell.Base`, an address in a temporary;
`Cell.Arg`, an argument's address). Every descriptor routine works on it unchanged, and
`ir/locate.go`'s `resolveLocated`, run once a routine is lowered, rewrites each place naming one
into its real place, so neither the optimizer nor the code generator ever sees one. The same
trick later carried pointer dummies and pointer results.

### Pointer dummies and results

A pointer dummy is given **the address of the caller's own pointer** (the author's answer 5),
its longword or its descriptor, so the callee's `=>`, NULLIFY, ALLOCATE, and DEALLOCATE change
the caller's pointer with nothing to copy back:

```fortran
  subroutine push (head, v)
    type(node), pointer :: head
    integer, intent(in) :: v
    type(node), pointer :: new
    allocate (new)
    new%value = v
    new%next => head
    head => new
  end subroutine push
```

```text
;   13      new%next => head
	MOVL	LISTS.PUSH.NEW,R0
	MOVL	@LISTS.PUSH.HEAD(AP),4(R0)
;   14      head => new
	MOVL	LISTS.PUSH.NEW,@LISTS.PUSH.HEAD(AP)
```

`@HEAD(AP)` is the caller's pointer, and `CALL PUSH (LIST%NEXT, 6)` passes the address of a
component in a node, `PUSHAL 4(R0)`, so the new node goes in the middle of the list. An array
pointer dummy is the third kind of located cell, `Cell.Arg`. Such a call needs an explicit
interface (EXPLICITREQ), since the caller must know to pass the pointer and not its target; and
the module text, which had written a dummy's type but not its POINTER, learned to (a user passed
PUSH the target until it did).

A pointer *result* returns an association, not a value: a scalar's address in R0, a CHARACTER
one's descriptor in R0 and R1, an array one's descriptor through a hidden first argument the
caller provides, which the function fills in (`clearPointerResult` clears it at entry, so a
function that sets nothing returns a disassociated pointer). In the caller each reference becomes
a pointer of the statement's own (`pointerValue`), a temporary longword or descriptor, after which
it is a pointer like any other: a target in an expression, and `P => F(X)` the copy of one
pointer into another.

### Overlap: a pointer may point into the array being assigned

Section 27's overlap analysis decides whether an array assignment's loops may read what they have
already written. A pointer makes the answer "maybe" more often: in `A = P + 1`, P may point into
A. `sema/overlap.go`'s `mayShare` learned that a pointer may share storage with another pointer,
a TARGET (a dummy too), or anything in COMMON, of a type it may meet, and nothing else; so `A = P`
gets a temporary when A is a TARGET and doesn't when it isn't. A run-time test of the two address
ranges, cheaper than a temporary when they don't overlap, was weighed in Phase 110 and left for
later (section 35).

### Why TARGET exists

Section 16's optimizer keeps a value in a register across a statement only if nothing the
statement does can change the value's storage. A store to `$LOCAL+8` can't change `$LOCAL+20`;
the places analysis (`opt/places.go`) knows both, compares them, and says so. A store through a
pointer is a store to an address the compiler doesn't know. Until the phase's seventh subtask the
analysis called such a place `UnknownRegion`, conflicting with everything: after `Q = Q + K`,
every variable in the routine had to be read from memory again. Correct, and slow, and in a loop that walks a list
it is most of the loop.

The standard's answer is the TARGET attribute. A pointer may point only at a TARGET (or a part of
one), at memory ALLOCATE made, or at another pointer's target, and only at a value of its own
type. So a variable without TARGET can never be changed through a pointer, and the optimizer may
believe it. Subtask 7 made that the rule:

- the lowerer marks each place it reaches through a pointer with the type the pointer points at
  (`ir.Mem.Pointee`, shown in the IR's dump as `@(P){=>INTEGER}`), so the optimizer needn't
  recognize the address's form, which its own passes rewrite;
- `places.go` makes a marked place a `PointerRegion` of that *class* (`sema/pointerclass.go`'s
  `PointersMayMeet`: an intrinsic type by kind, every CHARACTER length one class, a derived type
  by `sameType`; and a record meets any class one of its components, at any depth, belongs to);
- a `PointerRegion` place conflicts with another of a class that may meet, with any TARGET of
  such a class and anything sharing its storage by EQUIVALENCE (`Place.Targets`, from
  `findTargets`), with all of COMMON but a module's block (another unit may declare a TARGET in a
  common block; a module's variables are judged by their own attributes, which every user sees),
  and with an unknown place; with nothing else. Any call may change a `PointerRegion`.

Here is the difference it makes:

```fortran
program alias
  integer, target :: t
  integer, pointer :: q
  integer :: i, k, n, total
  read (*, *) n, k, t
  q => t
  total = 0
  do i = 1, n
    q = q + k
    total = total + t
  end do
  print *, total, t
end program alias
```

```text
	MOVL	ALIAS.Q,R6		; Q's target's address, once
	MOVL	ALIAS.I,R7
	MOVL	ALIAS.TOTAL,R8
	MOVL	ALIAS.K,R10		; K, once
ALIAS.$3:				; DO (line 8)'s body
;    9      q = q + k
	ADDL2	R10,(R6)
;   10      total = total + t
	ADDL2	ALIAS.T,R8
;   11    end do
	AOBLEQ	R9,R7,ALIAS.$3
```

K, which has no TARGET, lives in R10 for the whole loop, though every pass stores through Q. T is
a TARGET of Q's type, so the store may have changed it, and it is read from memory each time,
which is right: Q points at T, and given `3 2 1` the program prints `15 7`. Had T been declared
without TARGET, `q => t` would be NOTTARGET, and the compiler would have been free to hold T in a
register too. That is the whole bargain: the programmer says, by TARGET, which variables a pointer
may reach, and the compiler keeps every other one fast. A program that points where the standard
doesn't allow (through EQUIVALENCE to a non-TARGET in another unit, say, or by passing a
non-TARGET's address to MACRO code that keeps it) gets what it asked for; MANUAL.md's 14.9 lists
it among the rules `/OPTIMIZE` takes at their word.

Two smaller choices are worth knowing. A TARGET is judged by its type whether or not the unit
ever points at it, rather than only once `=>` names it: a TARGET is there to be pointed at, and
the bolder rule would have saved little. And the forward and dead-code passes needed no change:
they keep facts only of places with no address, so two pointers with equal-looking places are
never taken for one.

### What the programs found

`testdata/fortran90` has a program per subtask, each worked by hand and run in every mode and both
loaders, and the same on gfortran (`compare.py`): `pointers.f90`, `ptrarrays.f90`,
`ptralloc.f90` (with `ptrallocvms.f90`, VMS's own: the FOR numbers and LIB$STAT_VM's bytes),
`ptrcomps.f90`, `ptrcarr.f90`, `ptrdum.f90`, `ptrres.f90`, `ptralias.f90`, and govax's own
`ptrclear.f90` (stale pointers counted, with the clearing switched off) and `ptrequiv.f90`
(EQUIVALENCE with a TARGET, which Fortran 95 forbids and gfortran refuses). `ptralias.f90` was run
against rules made too bold on purpose: ignoring TARGETs and COMMON, six of its cases printed
wrong under `/OPTIMIZE`; without the record half of the class rule, the component case did. The
programs end to end are `ptrstd.f90`, the standard's own examples of pointers made to run;
`sparse.f90`, a sparse matrix of linked rows with defined operators; and `modules/concord`, a
concordance over four files, a queue module and a tree module with recursive procedures on pointer
dummies, compiled in four orders.

Along the way the pointer work found older seams: the lexer reading `A(::2)`'s two colons as
`::`; the places analysis taking an array pointer's descriptor for an allocatable array's, so a
store through the pointer was thought to leave every named variable alone; an
`XORL3 #PTR_MARK,R3,(R3)+` in FTNRTL, which reads R3 in the same instruction that autoincrements
it (UNPREDICTABLE, the architecture says); INTENT(IN) refusing a store *through* a dummy's
pointer component, which changes only the target; and, in `sparse.f90`, a bound read from a
dummy's component (`Y(A%N)`), a defined operator's array operand, a defined assignment's array
value, and ASSOCIATED of a pointer function's value. Each is held by a test of its own.

What waits: a CHARACTER pointer whose length is known only when the program runs. (`NULL()` and
default initialization came with Fortran 95, section 37.)

## 33. Fortran 90: the intrinsic procedures (Phase 108)

VAX FORTRAN had about 160 intrinsic function names, nearly all of them one idea in several
types: SQRT of a REAL\*4 is `MTH$SQRT`, of a REAL\*8 `MTH$DSQRT`, and a reference picks its
row by its argument's type (section 11). Fortran 90 adds about sixty more names of a different
kind. Some are trivial (`ACHAR` is `CHAR`), some are a loop (`LEN_TRIM`), some make a value of a
length nobody knows until the program runs (`TRIM`, `REPEAT`), some are about a number's bits
rather than its value (`EXPONENT`, `NEAREST`, `TRANSFER`), and four are subroutines that read
the clock and draw random numbers. Every one may be given its arguments by keyword, and every
elemental one works on arrays. The compiler's question for each reference is the same as
before, *how should this be computed?*, but the answers now range from "it is a constant" to
"it is a call that gives the program memory", and choosing well is most of the work.

The section follows a short program:

```fortran
program tour
  character(len=12) :: name = 'Ada   '
  character(len=12) :: right
  integer :: n, k
  real :: x
  n = len_trim(name)
  print *, '[', trim(name), ']', n
  right = adjustr(name)
  print *, '[', right, ']'
  x = 6.5
  k = ceiling(x) + exponent(x)
  print *, k, transfer(x, 0)
end program tour
```

```text
[Ada]           3
[         Ada]
         10       16848
```

6.5 is 0.8125 × 2\*\*3, so its EXPONENT is 3 and its CEILING 7; 16848 is ^X41D0, F_floating's
6.5 read as an integer. Every compiled listing below is govax's own `/OPTIMIZE` output.

### One table, read three ways

Section 11's table (`sema/intrinsics.go`) is still the whole story: Fortran 90's names are more
rows, in `fortran90Table`, not more code paths. A row now has eight columns:

```text
-      INDEX    2,3 CH  I4  INDEX  E STRING,SUBSTRING,BACK?
ISHFTC JISHFTC  2,3 I4  I4  SHIFTC E I,SHIFT,SIZE?
TRIM               -  1   CH  CH  TRIM T STRING
SCALE              -  2   *   *   MODEL E X,I
TRANSFER           -  2,3 *   *   TRANSFER T SOURCE[*],MOLD[*],SIZE?
```

The generic and specific names, the argument count, the types, and *how* are section 11's. The
seventh column is the standard's class (E elemental, I inquiry, T transformational), and the
eighth the dummy arguments' names, the standard's 13.13 for its functions and the HP Fortran
manual's for DEC's (`SIND (X)`, `ZEXT (X)`). `?` marks an optional dummy, `...` after MAX's `A2`
says more follow by position only, and a rank in brackets says what a transformational
function's dummy takes (`MATRIX[2]`, `ARRAY[]` any array, `SOURCE[*]` anything). The table is
read three ways:

- **to type a reference**: the arguments' types pick a row, as before;
- **to order keyword arguments**: `keywordSlots` (`intrinsickeys.go`) puts `IAND (J = 3, I =
  5)` into the dummies' order, with a hole for an optional one left out. Phase 103 wrote the
  same logic for a program's own procedures with an explicit interface; an intrinsic's
  interface is its row. A specific name takes its generic's dummies (`dummiesOf`);
- **to resolve a generic name that extends an intrinsic** (section 28): the ranks in the eighth
  column tell `takenByIntrinsic` whether a program's specific `SUM` of a scalar can be told
  apart from the intrinsic `SUM` of an array.

The four intrinsic subroutines and MVBITS have a small table of their own (`intrinsicsub.go`),
read by the same `keywordSlots`: `CALL DATE_AND_TIME (VALUES = V)` becomes a call with three null
arguments and V fourth, and no explicit interface is asked for, since the table is one.

Names only Fortran 90 has are intrinsic under `/F90` and, under `/NOF90`, only when an
INTRINSIC statement names them: an old program with its own function called `TRIM` keeps it
(`TestNoF90Unchanged`). Where VAX FORTRAN and the standard disagree on a result, the setting
decides: IBITS, IBCLR, and ISHFTC of an INTEGER\*2 are INTEGER\*4 by the 1988 manual and of I's
own kind by the standard, so `iKindFunctions` converts the other arguments to I's kind before a
row is chosen under `/F90` only.

### Four ways to compute an intrinsic

Each row's *how* names one of four forms, and a reference may take a cheaper one than its row
when its arguments allow:

1. **A constant.** When every argument is constant the reference is folded, by the same Go code
   the constant evaluator uses (`constfuncs.go`), so a PARAMETER, a type statement's initial
   value, a DATA value, or a declared length may call it: `PARAMETER (N = LEN_TRIM (TITLE))`,
   `CHARACTER (LEN = 3) :: RULE = REPEAT ('-', 3)`. The standard's 7.1.6.1 says which functions an
   initialization expression may call; the folding is the same code in an executable statement,
   so `LEN_TRIM ('AB  ')` there is the constant 2.
2. **In line**, when a few instructions do it. `LEN_TRIM` is a backward loop (the VAX has no
   SKPC that runs from the end), `CEILING` a truncation and a correction, `EXPONENT` an EXTZV.
3. **A call of FTNRTL**, when the work is a real loop, needs memory, or must signal: `ADJUSTL`,
   `REPEAT`, `SCAN` of a set, `NEAREST`, `DATE_AND_TIME`. FTNRTL's new routines are MACRO, like
   the rest of it (section 14), and its transfer vector grew from GSMATCH 1,25 to 1,30.
4. **A call of MTHRTL**, for a numeric function MTHRTL already has: a REAL `MODULO` is
   `MTH$AMOD` (or its kind) and a correction. MTHRTL's vector is VMS's, so a function it lacks
   (COMPLEX(16)'s SQRT, the model functions) is FTNRTL's, built on MTHRTL's H_floating routines:
   `FTN$CHEXP` is `MTH$HEXP` and `MTH$HSINCOS`.

`LEN_TRIM` in line is the program's line 6, the string's address in `$T1` and the length
counting down in R6 until it reaches 0 or a character that isn't a blank:

```text
;    6    n = len_trim(name)
	MOVAB	TOUR.NAME,TOUR.$T1
	MOVL	#12,R6
	BRB	TOUR.$1
TOUR.$2:
	SUBL3	#1,R6,R0
	CMPB	@TOUR.$T1[R0],#32
	BNEQ	TOUR.$3
	DECL	R6
TOUR.$1:
	TSTL	R6			; from line 6
	BNEQ	TOUR.$2
TOUR.$3:
	MOVL	R6,TOUR.N
```

The loop is `ir/lowercharfuncs.go`'s `search`, one routine for LEN_TRIM and for SCAN and VERIFY
of a one-character constant SET in either direction: they differ only in the character compared,
whether a match or a mismatch stops the loop, and which end it starts from.

Line 11 shows something about forms that matters to the optimizer. CEILING is written as
ordinary IR (a conversion to a longword, a comparison, an add), and EXPONENT reads X's bytes from
memory. In the main program, where forwarding (section 16) knows X is 6.5, the first is folded and
the second isn't:

```text
;   11    k = ceiling(x) + exponent(x)
	MOVL	#7,TOUR.$T3
	MOVL	#^X000041D0,TOUR.$T5	; 6.5
	EXTZV	#7,#8,TOUR.$T5,R0
	MOVL	R0,TOUR.$T1
	BEQL	TOUR.$11
	SUBL2	#128,TOUR.$T1
TOUR.$11:
	ADDL3	TOUR.$T1,#7,TOUR.K
```

In a subroutine, where X is a dummy, both are code:

```text
;    4    k = ceiling(x) + exponent(x)
	CVTFL	@WHOLE.X(AP),R6
	CVTLF	R6,R0
	CMPF	@WHOLE.X(AP),R0
	BLEQ	WHOLE.$1
	INCL	R6
WHOLE.$1:
	MOVF	@WHOLE.X(AP),WHOLE.$T2
	EXTZV	#7,#8,WHOLE.$T2,R0
	MOVL	R0,R7
	BEQL	WHOLE.$3
	SUBL2	#128,R7
WHOLE.$3:
	ADDL3	R6,R7,@WHOLE.K(AP)
```

`CVTFL` truncates toward zero; if the result made a REAL again is below X, X had a fraction and
was positive, so 1 is added. (A value too large to have a fraction converts back exactly, so the
test is safe at every magnitude a longword holds.) EXPONENT is the exponent field, bits 7 to 14 of
F_floating's first word, less the bias 128, with 0 left alone: the VAX's 0 is the one value whose
field is 0. An operation the optimizer can see through can be folded, moved out of a loop, and
shared; one that goes through memory, or calls a routine, usually can't. That is a reason to
prefer IR operations when an intrinsic is cheap enough to write in them.

### TRIM needs no storage

`TRIM (S)` is a value of a length known only when the program runs. Phase 104 already had such
values (section 29): concatenation of them makes a class D dynamic string, with memory from
LIB$SGET1_DD freed at the statement's end. TRIM could be done that way, but it needn't be. `TRIM
(S)` *is* `S (1 : LEN_TRIM (S))`, a substring, and everything that takes a CHARACTER value
already takes a substring of run-time length: assignment, concatenation, comparison, an I/O item,
an actual argument by descriptor. So lowering makes TRIM the argument's own text with LEN_TRIM's
length, and no bytes move. Line 7's I/O item is the descriptor `$D2` filled in from the loop's
count and NAME's address:

```text
	CVTLW	TOUR.$T3,TOUR.$D2
	MOVAB	(R6),TOUR.$D2+4
	PUSHAQ	TOUR.$D2
	CALLS	#1,G^FTN$IO_ITEM
```

ADJUSTL and ADJUSTR can't be done in place, since they move the blanks, but their result is the
argument's length, so lowering gives them a `$Tn` temporary of that length when it is known
(`madeByRoutine`) and FTNRTL writes into it:

```text
;    8    right = adjustr(name)
	PUSHAQ	TOUR.$D6
	PUSHAQ	TOUR.$D5
	CALLS	#2,G^FTN$ADJUSTR
	MOVC3	#12,TOUR.$T4,TOUR.RIGHT
```

REPEAT's length is `LEN (S) × N`, which is known when compiling only if both are constants. When
it isn't, the caller passes an empty class D descriptor and `FTN$REPEAT` gets the storage itself,
after checking N: a negative count, or a value past a descriptor's 65,535 characters, is a
condition of govax's own, `%FOR-F-BADREPEAT`, FOR number 305. The descriptor is freed at the
statement's end, as concatenation's are.

### A length of zero

Fortran 90 allows `CHARACTER (LEN = 0)`, and TRIM of blanks and `REPEAT (S, 0)` make zero lengths
when running. The second kind needed no work: MOVC3, CMPC5, descriptors, LIB$SGET1_DD, and
FTNRTL's I/O all take a length of 0. The first needed a decision, because inside sema a CHARACTER
type of size 0 already meant "a length not known when compiling" (a passed-length dummy, a
substring with variable bounds). Rather than add a third state beside "known" and "not known
yet", a declared length of zero is a length *known only when running* whose expression happens to
be the constant 0 (`Symbol.ZeroLength`, Phase 104's `LengthExpr`): every path that handles
run-time lengths handles it, and it has no storage. A substring whose upper bound is below its
lower is empty under `/F90` (`emptySubstring`), and `/CHECK=BOUNDS` checks a substring's bounds
only when it isn't empty, as the standard says; under `/NOF90` both are still errors, as in VAX
FORTRAN.

### The model functions are about bits

The standard describes a real number by a model, `s × 0.f1f2...fp × b**e`. Most processors'
formats only approximate it (IEEE's denormals and infinities don't fit). The VAX's formats fit it
exactly (`vaxfloat/model.go`): b is 2, the hidden bit is f1, e is the exponent field less the bias
(128 for F and D, 1024 for G, 16384 for H), p is 24, 56, 53, or 113, and there is nothing below
TINY and nothing above HUGE. So every model function is an exact change to a value's fields:
EXPONENT reads the field, FRACTION replaces it with the bias, SCALE adds to it, NEAREST adds 1 to
or takes 1 from the bits below the sign as one whole number.

EXPONENT and FRACTION are short enough for line. The other five are FTNRTL routines, one entry
per format (`FTN$SCALE_F`, `_D`, `_G`, `_H`, and so on: twenty entries), and the four entries of
a function share its code: each points R11 at its format's parameters and branches to it.

```text
	.ENTRY	FTN$NEAREST_D,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	FORMAT_D,R11
	BRW	NEAREST
	...
FORMAT_F:
	.LONG	7,8,128,255,24,1	; field position, size, bias, largest field,
FORMAT_D:				; digits, longwords
	.LONG	7,8,128,255,56,2
FORMAT_G:
	.LONG	4,11,1024,2047,53,2
FORMAT_H:
	.LONG	0,15,16384,32767,113,4
```

The code works on X's longwords copied as integers, never with a floating instruction, so a
reserved operand can't fault inside the routine. A result past HUGE has to be an overflow the
program can see, and the simplest honest way to make one is to do arithmetic that overflows: the
routine builds the largest exponent with X's sign and doubles it with a MULx, so a handler sees
`SS$_FLTOVF_F` exactly as from any instruction. A result below TINY is 0, as the VAX's own
underflow is with FU clear. One more thing about the VAX's words: a floating value's longwords
have their halves swapped (the sign and exponent are in the first *word*), so NEAREST's carry runs
from word to word, most significant first, with INCW and DECW, rather than as a longword add.

Because a REAL\*8 is D or G by the unit (`/G_FLOATING`), the D answers have 56 digits where an
IEEE machine's have 53: `SPACING (3D0)` is 2\*\*-54 under D and 2\*\*-51 under G. gfortran is the
witness for REAL\*4 and G only.

### TRANSFER moves bytes, not values

`TRANSFER (SOURCE, MOLD)` is SOURCE's bytes seen as MOLD's type. The compiler's job is to never
look at the bytes *as* anything. The value is a temporary of the result's size: SOURCE's leading
bytes are moved in and the rest cleared (`moveFilled`: MOVC3 and a CLRx when the lengths are
small constants, LIBRTL's `OTS$MOVE5` with a fill of 0 when they aren't), and the temporary is
then read as MOLD's type. The standard leaves the tail undefined; govax makes it zero. In the tour,
`TRANSFER (X, 0)` is two MOVLs:

```text
	MOVL	TOUR.X,TOUR.$T5
	MOVL	TOUR.$T5,TOUR.$T1
```

A REAL is moved with MOVL, not MOVF, so bits that are a reserved operand in MOLD's format pass
through TRANSFER, and through an assignment of it to a REAL variable (`rawMove`), without a fault.
A later computation with them faults, as on any VAX. Of constants TRANSFER is folded from
vaxfloat's bits (`transferValue`), so it may give a PARAMETER its value.

### The clock and random numbers

The four subroutines are calls of FTNRTL, each with a `_W` form for INTEGER\*2 arguments:

- **DATE_AND_TIME** writes the date and time from `$NUMTIM` (milliseconds are hundredths × 10)
  and takes the zone from the logical name `SYS$TIMEZONE_DIFFERENTIAL`, as VMS V6 and later
  define it: a signed count of seconds east of UTC, `"-18000"` for US Eastern standard time (the
  author's check on VMS 7.1). govax defines it in the system table as the logical name database
  is made, from the host's offset (`lnm/timezone.go`), and FTNRTL translates it through
  LNM$FILE_DEV, so a process may define its own. Without a number there, ZONE is blanks and
  VALUES(4) -HUGE.
- **SYSTEM_CLOCK** counts the system time's hundredths (rate 100). `$GETTIM`'s 64-bit count of
  100 ns units is divided by 100,000 in two EDIVs, the high half first, so the quotient fits a
  longword.
- **RANDOM_NUMBER** draws with MTHRTL's `MTH$RANDOM` on a longword seed FTNRTL keeps, so the
  sequence is the same on every host and `vax.runtime.random` reaches it as it reaches RAN. One
  draw has 24 bits, enough for F. D and G join three draws and H five, by Horner's rule from the
  last, the last cut to the bits the format has left: each step is exact, so no number rounds up
  to 1.0, which the standard forbids.
- **RANDOM_SEED**'s seed is that one longword, N = 1 (2 for INTEGER\*2, its two words). Every image
  starts at 1234567, govax's choice: the manuals suggest none, the 1988 manual's RAN asking only
  for "a large, odd integer value". `CALL RANDOM_SEED ()` sets it back.

MVBITS, VAX FORTRAN's already, became the standard's: FROM any integer expression, and
*elemental*, so `CALL MVBITS (A, 0, 4, B, 4)` with arrays is a loop nest calling `FTN$JMVBITS` per
element in array element order (`elementalCall`, the scalarizer's loops around a CALL).

### What the programs found

`testdata/fortran90` has a program per subtask, each worked by hand, run in every mode, and the
same on gfortran (`compare.py`): `keywords.f90`, `charfns.f90`, `numfns.f90`, `model.f90`,
`transfer.f90`, and `clockrand.f90` (only what any processor must print: the seed's size, a
sequence repeated after PUT, a count that doesn't go backward); and govax's own answers in
`modelvms.f90` (TINY and HUGE in each format, every overflow caught), `transfervms.f90` (the VAX's
bits, the zero tail, a reserved operand through and back), and `clockvms.f90` (the generator's
numbers worked from its formula, a zone set by LIB$SET_LOGICAL). The programs end to end are
`textfmt.f90`, a text formatter built from INDEX, VERIFY, SCAN, ADJUSTR, REPEAT, and ACHAR, and
`calendar.f90`, day numbers by MODULO, weeks by CEILING, dates packed by MVBITS and turned by
ISHFTC. Over the whole Fortran 90 corpus, gfortran and govax print the same in all 93 programs.

FTNRTL's 45 new entries are also called directly from MACRO by `TestFTNRTLIntrinsics`
(`internal/console`), at the edges the compiler never reaches: zero lengths, ±TINY and ±HUGE in
every format, (0, 0)\*\*0, a seed put as two words. Each expected value was worked from the formats
by hand. Two of those hand values were wrong (H's SPACING and RRSPACING of 1.0: the bias is 16,384,
so 1.0's field is 16,385), and the routines were right. Checking an answer against the formats from
first principles, rather than against a run of the code, is what makes such a test worth having.

Along the way the work found older seams: ISHFT of an INTEGER(1) sign-extended where it should
zero-extend to the word it is shifted in; the constant pool's key left out a COMPLEX\*32's
imaginary half, so `(0.0_16, 0.5_16)` and `(0.0_16, 1.0_16)` shared one datum; and a substring of a
constant (`MONTHS(11)(1:8)`), which the standard allows and sema refused. Each is held by a test of
its own.

What waits: nothing of Fortran 95's (CEILING's and FLOOR's KIND=, NULL, and CPU_TIME came
with it, section 37). A BACK= that is an array, a TRANSFER to a CHARACTER MOLD of zero or
run-time length, and a component or function value of length zero came with Phase 113's errata:
BACK= is one more elemental argument; such a MOLD's length is read from the MOLD itself, the
value a dynamic string (`ir/lowertransfer.go`'s `transferText`) or a run-time temporary; and a
component of length zero is `Field.Empty`, a reference to it an empty substring, (1:0), of its
place taken as CHARACTER\*1 (sema's `emptyComponent`), so nothing is ever read or written there.

## 34. Fortran 90: I/O (Phase 109)

Fortran 90's chapters 9 and 10 change less than its other chapters do. The statements are
FORTRAN-77's, and most of what is new is more of the same: a few edit descriptors, a few OPEN
specifiers, a NAMELIST that VAX FORTRAN already had in another spelling. Two things are new in
kind. A statement may end *without ending its record*, so that the next statement goes on where
it stopped. And the same text may mean two things: a quotation mark, `3/` in a format, G
editing an integer, are each an error to VMS's run-time library and a value to Fortran 90's. The
first is a change to FTNRTL's idea of a record's life; the second is a change to how FTNRTL
knows which language it is serving. Both had to leave every VAX FORTRAN program exactly as it
was, since VMS's own runs (`testdata/for80`, `nml80`, `for88`) are what FTNRTL is held to.

The section follows a short program:

```fortran
program ask
  character(len=20) :: reply
  integer :: n, ios
  write (*, '(1x, a)', advance='no') 'Name? '
  read (*, '(a)', advance='no', size=n, iostat=ios) reply
  if (ios == -2) print '(1x, a, i2, 3a)', 'got', n, ' [', reply(1:n), ']'
  write (*, '(1x, es10.3, en12.3, b8.8)') 12345.6, 12345.6, 37
end program ask
```

At a terminal, typing `Ada`:

```text
Name? Ada
got 3 [Ada]
 1.235E+04  12.346E+0300100101
```

The question stays on the line the answer is typed on; the READ asks for 20 characters and finds
3 before the record ends, which is not an error but a condition the program asked to hear about
(`IOSTAT=` is -2); SIZE= says how many characters it took. ES is scientific notation, one digit
before the point; EN engineering notation, an exponent that is a multiple of 3; B binary.

### One bit says which language a statement speaks

FTNRTL can't see the program's source. Whatever it must know about a statement comes through the
begin call's arguments, and the first of them is the statement's control block, a word of size
and a word of flags (section 14). Phase 109 added a flag: bit 4 says *this statement belongs to
a unit compiled `/F90`*. The compiler sets it for every data transfer statement of such a unit
(`ir.ControlF90`, `ir/lowerio.go`), and the MACRO file's comment says so:

```text
ASK.$IO2:
	.WORD	8			; an I/O statement's control block: its size
	.WORD	^X003C			; flags: IOSTAT= run-time format F90 nonadvancing
	.LONG	0			; (not used yet)
```

Why a flag per statement rather than a different FTNRTL, or a different set of entries? The
survey found that VMS's own library *refuses* things Fortran 90 accepts: `3/` in a format (case
375 of Phase 80's run), G of an INTEGER item (case 169), `"ABC"` read by a list-directed READ
(case 806). FTNRTL is held to those refusals, so it can't simply take Fortran 90's reading
always. A second set of begin entries would have doubled the transfer vector for one bit of
information. And the choice has to be per statement, not per image, because one image may link
a `/F90` unit with an old one, each expecting its own reading. The flag costs a bit in a word
the compiler already writes.

Under the flag, FTNRTL reads:

- a **format** with B, EN, ES, `r/`, and `"…"` (in a format held in a variable too, which the
  compiler can't rewrite with apostrophes as it does a constant one), and G of an INTEGER,
  LOGICAL, or CHARACTER item as I, L, or A (`ftn_format.mar`, PARSE's fifth argument and EDIT's
  `GENERAL`);
- **blanks in numbers** as nothing, BLANK='NULL', for an internal file and a unit no OPEN
  described, where VAX FORTRAN's default was ZERO. A unit's `UB_BLANK` has a third value, 3,
  "not said", which each statement makes ZERO or NULL by its flag, so one unit serves both
  kinds of statement correctly;
- a **field cut short** by the record's end as padded with blanks to its width (PAD='YES', the
  standard's), so `123` read by `(BZ, I5)` is 12300; VAX FORTRAN ended the field at the record's
  end, 123, and still does without the flag;
- **list-directed input's** quotation marks and undelimited text, and **namelist's** `&GRP` …
  `/` both ways (below).

Not everything new needs the flag. A specifier the old language didn't have (ADVANCE=, DELIM=,
ACTION=) can't appear in an old program, so FTNRTL honors it whenever it is given; the flag is
only for texts both languages read.

### New edit descriptors are old code with a parameter

FTNRTL's format interpreter turns a format into a list of operations, each with a code (section
14). Fortran 90's three new data descriptors got codes 10 to 12, after Q, which meant
renumbering the control edits after them, in `ftn_format.mar` and in the compiler's
`internal/fortran/format` together (the two parse the same grammar and are held to the same
cases). The editing routines' codes were left alone, so FTN_EDIT and FTN_FLOAT didn't change.

Each new descriptor is mostly an old one:

- **B** is O and Z's code with a digit of one bit instead of three or four.
- **ES** is E with a scale factor of 1, which E already knew how to do (`1PE10.3` and `ES10.3`
  print the same digits).
- **EN** is the only real newcomer: the exponent must be a multiple of 3, so the number has one
  to three digits before the point. `OUT_EN` asks `OTS$$CVT_x_T_R8` for d+3 digits to find the
  exponent, then rounds to the digits it will show; a carry from that rounding (999.96 to 1000.0)
  can add a digit before the point and so change the exponent, which is why the rounding is done
  twice.
- **G** of an INTEGER, LOGICAL, or CHARACTER item makes a copy of its operation as I, L, or A
  with w alone and edits with that.

The compiler's `format` package learned the same grammar under `format.Options.F90`, and the
`/STANDARD` walk calls each of them an extension to FORTRAN-77.

### A record that outlives its statement

In FORTRAN-77 a statement owns its records. The begin call starts one; the end call writes the
last (or, reading, passes over the rest of it); the next statement starts afresh. Section 14's
statement context, pushed by the begin call and popped by FTN$IO_END, holds the record being
built a byte per column. `ADVANCE='NO'` breaks that: the record must outlive the context.

So the record moves to the unit. A unit's block (`ftn_units.mar`) already had a buffer where
each record was built or read (`CTX_RECP` points there), so leaving a record open costs only
three fields to say what is there:

```text
UB_OPENREC	byte: 0 none, 1 an output record, 2 an input record
UB_RECLEN	long: the record's length
UB_RECPOS	long: the position the next statement goes on from
UB_DOLLAR	byte: 1 if a $ edit descriptor marked it
```

Three places decide its fate:

1. **FTN$$FMT_END**, at a nonadvancing statement's end with no error, doesn't write the record
   (or pass over the rest of an input record): it copies its length and position into the unit
   and sets `UB_OPENREC`.
2. **The begin call** of the next statement on the unit looks first. A formatted sequential
   statement of the same direction, advancing or not, will go on with the record. Anything else
   (a list-directed statement, an unformatted one, a READ after a WRITE, a direct access
   statement) must not see it, and `FTN$$UNIT_FLUSH` ends it first: an output record is written,
   an input record forgotten (the unit is already past it). CLOSE, REWIND, BACKSPACE, ENDFILE,
   and so the exit handler that closes every unit at image exit, end it the same way, so an
   image whose last statement was nonadvancing still writes its line.
3. **FTN$$FMT_START** of a statement going on reads no record: it takes the length and position
   from the unit and clears `UB_OPENREC`. The position is also the statement's *left tab limit*
   (`CTX_LEFT`, the standard's 10.6.1.1): `T5` counts five columns from where this statement
   started, not from the record's start, and `TL` goes back no further than it. Without that rule
   a statement couldn't know what columns its own format meant.

The design note's open question was whether this should share a mechanism with VAX FORTRAN's `$`
edit descriptor, the old way of leaving the cursor at a prompt's end. It does, in one place. `$`
never kept a record open in FTNRTL: it writes the record at once with `$` as its carriage control,
which tells RMS not to end the line (section 14's "Units and the terminal"). When a READ is about
to read a terminal, `FTN$$UNIT_PROMPT` looks at every unit at a terminal with an output record
open and writes it the way `$` would, so the question shows before the answer is typed:

```text
	.ENTRY	FTN$$UNIT_PROMPT,^M<R2,R3>
	...
10$:	MOVL	(R3)+,R0		; each unit's block in turn
	BEQL	20$
	BLBC	UB_OPEN(R0),20$
	CMPB	UB_OPENREC(R0),#1
	BNEQ	20$
	BBC	#DEV$V_TRM,UB_FAB+FAB$L_DEV(R0),20$
	PUSHL	#1			; written as a prompt
	PUSHL	R0
	CALLS	#2,FTN$$UNIT_FLUSH
20$:	SOBGTR	R2,10$
```

The standard's strict reading would keep the record whole until an output statement ended it,
showing `Name?` only after the answer; DEC's terminals and gfortran both show it first, and the
author chose that. The rest of the program's record after the READ is a new record. Why every
terminal unit and not only the one being read? Because units 6 and `*` are different units on one
terminal (section 14), and a program may ask on one and read on another.

### The end of a record is a condition, not an error

A nonadvancing READ that asks for more than the record holds hasn't failed: it has found the end
of the record, which is how a program reads a line of unknown length. The standard calls it the
*end-of-record condition*, and gives it EOR= and an IOSTAT= value of its own (-2 by the author's
choice, gfortran's; -1 is still an end of file).

The item whose field runs past the record's end (and no comma ended it first) is edited from what
was there padded with blanks, so `reply` gets `Ada` and 17 blanks; then FTN$$FMT_ITEM stops the
statement with the new condition `FOR$_EORDURREA`. FTN$IO_END turns that into -2 for EOR= or
IOSTAT=, and with neither signals it. The input record is done: an end of record ends it. SIZE=
is the characters the statement's data edit descriptors took from the record, padding not
counted, kept in the context (`CTX_COUNTED`) as FIELD and A take them. The context is gone by
the time the compiled code could ask, so FTN$IO_END takes one argument for a nonadvancing READ
with SIZE=, the address of a longword it stores the count in:

```text
;    5    read (*, '(a)', advance='no', size=n, iostat=ios) reply
	PUSHAQ	ASK.$D1
	PUSHL	#-4
	PUSHAB	ASK.$IO2
	CALLS	#3,G^FTN$READ_SF
	PUSHAQ	ASK.$D3
	CALLS	#1,G^FTN$IO_ITEM
	PUSHAL	ASK.$T1
	CALLS	#1,G^FTN$IO_END
	MOVL	R0,R6
	MOVL	ASK.$T1,ASK.N
	MOVL	R6,ASK.IOS
```

The count goes through a longword temporary, not straight into N, because SIZE='s variable may
be any INTEGER kind: FTNRTL always stores a longword, and the compiled code converts it, an
INTEGER\*2's store a CVTLW.

Fortran 90's new conditions take FOR numbers past VMS's, govax's own wording (Decision 15):
`REAWRIFIL` (306, a READ of a unit opened ACTION='WRITE'), `EORDURREA` (307), `BADADVANCE` (308,
an ADVANCE= neither YES nor NO), `NONADVUNIT` (309, a nonadvancing statement on a keyed unit), and
`RECTOOSHO` (310, a short field on a PAD='NO' unit). Their texts are in `vmsdef/formessages.go`
with VMS's, so `$GETMSG` and ERRSNS see them as any other.

### ADVANCE= when only the program knows

`ADVANCE='NO'` written as a constant is settled when compiling: the control block's bit 5. But
ADVANCE='s value may be any CHARACTER expression, and the begin call needs the answer before the
statement's first item. Rather than give every begin call another argument, the compiler asks
FTNRTL first, with a small routine that only reads the text (`FTN$ADVANCE`: 0 for YES, 1 for NO,
2 for anything else, blanks and case ignored), and then makes one of three begin calls, each with
its own control block. A subroutine that writes a row of a matrix, and may or may not end the line:

```fortran
subroutine row(a, how)
  real :: a(10, 10)
  character(len=*) :: how
  write (*, '(1x, 5f6.1)', advance=how) a(2, 1:9:2)
end subroutine row
```

```text
	PUSHAQ	ROW.$D1
	CALLS	#1,G^FTN$ADVANCE
	MOVL	R0,ROW.$T1
	CASEL	ROW.$T1,#0,#1
ROW.$1:
	.WORD	ROW.$2-ROW.$1
	.WORD	ROW.$3-ROW.$1
	BRB	ROW.$4
ROW.$2:
	PUSHAQ	ROW.$D2
	PUSHL	#-1
	PUSHAB	ROW.$IO1		; F90
	CALLS	#3,G^FTN$WRITE_SF
	BRB	ROW.$5
ROW.$3:
	...
	PUSHAB	ROW.$IO2		; F90 nonadvancing
	...
ROW.$4:
	...
	PUSHAB	ROW.$IO3		; F90 bad ADVANCE=
	CALLS	#3,G^FTN$WRITE_SF
ROW.$5:
```

The third block's bit 7 makes the statement fail at once with BADADVANCE, through the same
ERR=/IOSTAT= path as any other failure, so a bad value is reported where the statement is and
nothing is written. The items and FTN$IO_END follow the three calls once: only the begin call
differs, so the switch is short. It is an IR `Switch`, section 12's computed GO TO.

### A section is one item

Section 27 made an array section in an I/O list a loop of FTN$IO_ITEM calls, an element at a
time, which is correct but makes the compiled code as long as the loops. Phase 104 gave sections
class NCA descriptors (section 29), so the item can be described once. The listing above passes
`A(2, 1:9:2)` as one item, its descriptor filled in where the item stands:

```text
	MOVL	#1,R0
	MOVAL	@ROW.A(AP)[R0],ROW.$D3+4	; the first element, A(2,1)
	MOVL	#80,ROW.$D3+20			; its stride: two columns of 40 bytes
	MOVL	#1,ROW.$D3+24			; bounds 1 to 5
	MOVL	#5,ROW.$D3+28
	SUBL3	#80,ROW.$D3+4,ROW.$D3+16	; A0, one stride before element 1
	MOVL	#20,ROW.$D3+12			; 5 elements of 4 bytes
	PUSHAQ	ROW.$D3
	CALLS	#1,G^FTN$IO_ITEM
```

FTN$IO_ITEM hands a class NCA item to `ITEM_NCA` (`ftn_statement.mar`), which walks the
descriptor, the first dimension fastest, and gives each element to the statement kind's module
by a class S descriptor in its own frame. So nothing downstream changed: the format interpreter,
list-directed and namelist output, and unformatted records see exactly the element calls they
saw before. The walk stops at the statement's first failure, so an end of file part way through
a section leaves the rest of it alone, as the element calls did.

`ITEM_NCA` is a routine of its own, rather than a loop inside FTN$IO_ITEM, for a reason worth
knowing. A first try put the loop in FTN$IO_ITEM and saved R3 to R7 there; FTN$IO_ITEM's frame
grew by 20 bytes, and an old program in the corpus that reads an undefined local (`optforward.for`
under `/RECURSIVE`, where locals are in the stack frame) read a different leftover value from the
stack, and no longer printed VMS's 0 (`TestFTN86Oracle`). The program is wrong, but VMS's run
printed 0 and govax must too. Keeping FTN$IO_ITEM's frame as it was for every other item kept
the stack as it was.

The compiler passes a descriptor only where one helps: a whole assumed-shape array or array
pointer (whose own descriptor it already has), a section, a ranked component, or a substring of
a section, of an intrinsic type. A contiguous whole array keeps its class A descriptor (section
14); a derived type's object is still a component at a time (section 30); a vector subscript,
an expression, or a constructor is still an element at a time, since there is no place to
describe. The descriptor is built when the list reaches the item, so `READ *, N, A(1:N)` uses
the N just read.

A section as an internal file, or as ENCODE's or DECODE's buffer, is the same problem as a
section passed to an old dummy (section 29): FTNRTL wants a record per element of contiguous
storage. It goes through Phase 104's `sequence`: described where it is if contiguous, tested at
run time if it is an assumed-shape array's, otherwise copied to a temporary and, after a WRITE or
ENCODE, back.

### OPEN, INQUIRE, and IOLENGTH=

OPEN's new specifiers are new keyword codes after VMS's (49 to 52, and 5 for STATUS='REPLACE'),
INQUIRE's answers 53 to 59 (section 14's "OPEN and CLOSE"); FTN_OPEN's table grew and the unit
block gained `UB_ACTION`, `UB_DELIM`, `UB_PAD`, and `UB_POSITION`. Each maps onto RMS: ACTION= is
the FAB's FAC (READ is FAC GET, as READONLY always was), POSITION='APPEND' is ACCESS='APPEND''s
RAB$V_EOF, and STATUS='REPLACE' is a `$ERASE` of the FAB, an RMS$_FNF passed over, then the
`$CREATE` of STATUS='NEW'. The unit's DELIM= starts at 0, "not given", which INQUIRE answers
NONE, so that list-directed output can tell DEC's default (no delimiters) from a program that
asked for none.

`INQUIRE (IOLENGTH = L) list` asks how long an unformatted record of the list would be, without
doing any I/O. The plan considered a call of FTNRTL per item; but the length depends only on the
items' types and counts, which the compiler knows or can compute without touching the values.
So `ioLength` (`ir/lowerfile.go`) adds each item's bytes into a temporary (a scalar's size, an
array's element count times its element size, an implied DO a counted loop of its body), then
stores `(n+3)/4` (RECL= counts longwords for unformatted files on VMS). The optimizer folds a list
of constant sizes to one constant. gfortran answers in bytes, a processor's choice the standard
allows; the programs use the answer only as a RECL=, so both agree on what matters.

### List-directed and namelist I/O

Under the flag, list-directed input takes a constant between quotation marks as it takes one
between apostrophes (READ_TEXT takes its delimiter from the record), and a word that isn't a
number as undelimited text for a CHARACTER item. DELIM= on a unit delimits list-directed and
namelist output's text, the delimiter doubled inside it; a delimited constant longer than what's
left of the record starts the next, and one longer than a record goes on in the next record's
first column, so a READ \* takes it back whole.

Namelist under the flag is Fortran 90's form: ` &GRP` … ` /` on output, text undelimited unless
DELIM= asks (so such output can't be read back by a NAMELIST READ, as the HP manual warns: the
author chose the standard's rule), and on input `&GRP` from column 1, a slash ending the group
wherever DEC's `$END` would. A name may be a designator with sections and components,
`PTS(1:3:2)%X`, `BOOK(2:3)%CLEARED`. FTNRTL's `DESIGNATOR` reads the name a part at a time, each
part looked up as a prefix in the group's entities, its subscripts fixing an element or opening
a section, and the whole a walk over up to 15 dimensions, the slowest first; an object of a
derived type is filled level by level, an element's components in order before the next element.
The compiler's side is section 30's: an array of a derived type is an entity per component, each
a class NCA descriptor with constant strides, under a class Z header that points at the array's
shape.

### What the programs found

`testdata/fortran90` has a program per subtask, each worked by hand from the standard and run in
every mode with paging off and on: `edits.f90`, `cplx16io.f90`, `openings.f90`, `advance.f90`
(with a terminal's input), `listnml.f90`, `nmlsect.f90`, `ioitems.f90`, and govax's own
`ioitemsvms.f90` (ENCODE and DECODE on sections). The programs end to end are `journal.f90` (an
array of a derived type written by one reverting format with B, ES, and G, each record read back
by nonadvancing READs whose SIZE= at EOR= gives its length, so the records are compared byte for
byte where the standard fixes them, trailing blanks and all), `wordcount.f90` (the terminal read
a character at a time, each line's end by EOR=), and `gradebook.f90` (files made by
STATUS='REPLACE', appended by POSITION='APPEND', a test's column as one ranked component). The
three print the same on gfortran; the earlier ones differ only where the standard leaves the
layout to the processor (list-directed and namelist records, IOLENGTH='s unit), listed in
`compare.py`'s KNOWN. The VMS oracles and `TestNoF90Unchanged` were unchanged throughout: the
flag is clear for every statement they run.

The programs found that the three wrong answers on the way were the programs': `MAXLOC (A, DIM=1)`
is Fortran 95's, and Fortran 90's MAXLOC has no DIM=, so govax's BADARG was right; a hand-worked
`A10` of a shorter constant had been written flush left, where the standard pads on the left; and
gfortran refuses a repeat count on a component's section in namelist input (`BOOK(2:3)%CLEARED =
2*T`), which the standard allows. Writing the answers from the standard before running either
compiler is what made each of these a question rather than a silent agreement. The phase also
found an internal file's BLANK= default wrong for Fortran 90 (VAX's ZERO, where the standard says
NULL), and twelve programs from earlier phases that had never been run with paging on.

What waits: Fortran 2003's I/O (DECIMAL=, ROUND=, stream access), which no phase plans (Fortran 95's
zero widths, `I0` and `F0.d`, and namelist comments came later, under the same F90 flag: in a VAX
FORTRAN statement a zero width is still a field too narrow for any value, OUTCONERR); FTNRTL's limit of 15 dimensions and 8 levels in a NAMELIST object, past which the
compiler says NOTIMPL.

## 35. Fortran 90: the compiler's usability (Phase 110)

Sections 24 to 34 taught the compiler Fortran 90's language. A compiler is more than its language,
though: it is also what it tells you about your program (section 7's warnings and `/STANDARD`,
section 19's listings), what the compiled code looks out for (`/CHECK`), what it leaves for the debugger
(section 18), and how fast the code is (section 16). Phase 110 went through each of these for the
new language, and found that the earlier phases had done less of this than they seemed to:
`/STANDARD` judged a Fortran 90 program by FORTRAN-77's tables and missed most of it, a
module's private variables were never reported unused, a WHERE mask's temporary was listed among
the program's names, the debugger couldn't show a derived type or follow a pointer, and a loop over
an assumed-shape dummy cost three times what an explicit-shape one did. None of this changes what
a program computes, which is why it could wait; all of it changes what working on one is like.

The section follows one program through the tools:

```fortran
module store
  implicit none
  type item
     character(len=8) :: name
     real :: price
  end type item
  integer, private :: lookups = 0
contains
  subroutine raise (v, w, f)
    real, intent(inout) :: v(:)
    real, intent(in) :: w(:)
    real, intent(in) :: f
    v = v + f * w
  end subroutine raise
end module store

program tour
  use store
  implicit none
  real, allocatable, target :: a(:), b(:)
  real, pointer :: p
  type(item) :: best
  real*8 :: total
  integer :: i, n, last
  n = 6
  allocate (a(n), b(n))
  a = (/ (real (i), i = 1, n) /)
  b = a + 1.0
  a = b * 2.0
  call raise (a(1:n:2), b(1:n:2), 0.5)
  last = n
  total = sum (a)
  best = item ('PEAR', a(2))
  p => a(3)
  call show (n)
  nullify (p)
  print *, p
contains
  subroutine show (i)
    integer, intent(in) :: i
    print '(1x, 6f6.1, f8.1, 1x, a, f6.2)', a(1:i), total, best%name, p
  end subroutine show
end program tour
```

It prints `5.0 6.0 10.0 10.0 15.0 14.0 60.0 PEAR 10.00`, and then the last PRINT uses a pointer
that points nowhere, a bug left in on purpose.

### One walk, two standards

`/STANDARD` (section 7) is one walk over the syntax tree, `sema/standard.go`, with tables of what
FORTRAN-77 has: each statement kind, each declaration form, each intrinsic, each specifier, each
edit descriptor. A row the table marked "not FORTRAN-77" became `%FORT-I-EXTENSION`. Fortran 90
needed a second reading of the same tree, and the question was whether that should be a second
walk. It shouldn't: almost every finding is the same kind of fact ("this program uses the CONTAINS
statement"), and what differs between the two readings is only *which standards have it*. So each
row gained a column, `source.Standards`, a bit for each standard that has the thing:

```go
lex.Contains:          {"the CONTAINS statement", f90},
lex.Accept:            {"the ACCEPT statement", 0},
```

CONTAINS is Fortran 90's and not FORTRAN-77's; ACCEPT, DEC's, is neither's. The walk notes every
row it meets, and `extensions.add` keeps the note only when the standard being judged by isn't
among the row's bits. The same program under the two readings:

```text
$ FORTRAN/STANDARD=F77/NOMACRO TOUR.F90          50 findings, among them
%FORT-I-EXTENSION, extension to FORTRAN-77: the CONTAINS statement
%FORT-I-EXTENSION, extension to FORTRAN-77: a declaration with ::
%FORT-I-EXTENSION, extension to FORTRAN-77: free source form

$ FORTRAN/STANDARD=F90/NOMACRO TOUR.F90          one
%FORT-I-EXTENSION, extension to Fortran 90: a length after REAL
  at line 23 of TOUR.F90
    real*8 :: total
  --^
```

`REAL*8` is DEC's, an extension to both standards: Fortran 90 spells it `REAL(8)`. The message names
the standard, since the same text now means two things. With no `F77` or `F90` keyword each unit
is judged by its own `/F90` setting, so a free-form file is held to Fortran 90 and an old fixed-form
file to FORTRAN-77 in one compilation.

The walk had to learn Fortran 90's tree too. Before Phase 110 it only *descended through* the new
statements (an interface body, a WHERE's arms) to judge what was inside them by the old tables,
which is why it missed nearly everything. Now it notes `::`, each attribute, kind selectors,
deferred and assumed shapes, `==` and its kin (the parser records that the operator was written
symbolically, `ast.Binary.Symbolic`, since `==` and `.EQ.` are the same node otherwise), a
character constant between quotation marks, `%`, keyword arguments, constructors, defined
operators, construct names, and the rest. Two things only semantic analysis can tell: whether
`A(2:5)` is a section or a substring (`sectionList`, which decides it, notes it), and the
intrinsic names chapter 13 added (`noteIntrinsic`, as section 7 has it do for FORTRAN-77's). Both report
to the walk's own list, so their findings come out in source order with the rest.

Source form is the first pass's business (`source/standard.go`): a line's marks carry their
standards as the rows do. Under Fortran 90 a tab, a `$` in a name, a line past 132 characters, and
more than 39 continuation lines in free form are flagged, and small letters and `!` aren't. Under
FORTRAN-77 a free-form file is one finding, "free source form", at its first line, not one per line.

The tables are easy to get wrong in the quiet direction, flagging too little. What keeps them honest
is a witness: every Fortran 90 program the corpus also builds with gfortran `-std=f95` (section 34's
`compare.py`) must draw no EXTENSION under `/STANDARD=F90`. `TestStandardF90Witness` compiles them
all; the same programs under `F77` draw about 9,000 findings, which is the other direction's check.

### Obsolescent features need what a program means

Fortran 90 deletes nothing from FORTRAN-77, but its appendix B.2 lists nine features that a later
standard may delete. Fortran 95 deleted six of them (its B.1) and named others obsolescent in their
place; since govax's Fortran 90 takes in Fortran 95 (Phase 112), `/STANDARD=F90` reports Fortran
95's two lists, the deleted ones as `%FORT-I-DELETED` and the rest as `%FORT-I-OBSOLESCENT`, both
under the `OBSOLESCENT` keyword, on with `SYNTAX` unless named:

```fortran
subroutine old (x, n)
  real :: x, t
  do 10 t = 1.0, 3.0
     do 10 k = 1, n
        if (x) 20, 10, 10
10 x = x + t
20 pause
end subroutine old
```

```text
%FORT-I-DELETED, deleted in Fortran 95: a DO variable of type REAL
%FORT-I-OBSOLESCENT, obsolescent in Fortran 95: the arithmetic IF statement
%FORT-I-OBSOLESCENT, obsolescent in Fortran 95: a terminal statement shared by DO loops
%FORT-I-OBSOLESCENT, obsolescent in Fortran 95: a DO loop ending at an assignment
%FORT-I-DELETED, deleted in Fortran 95: the PAUSE statement
```

The arithmetic IF and PAUSE are visible in the tree, and the walk reports them as it does any
statement. The other three aren't: the tree says `DO 10 T = ...`, but whether T is REAL, whether two
loops end at label 10, and what label 10's statement is are facts semantic analysis works out
(section 12's `endDo`, `labels.go`). So the checks live where those facts are decided (`doVariable`,
`endLoops`, and for a jump to END IF from outside its block `checkLabels`), and report to the same
list. That bends section 7's rule of one walk for `/STANDARD`, as `noteIntrinsic` already had;
the alternative, recomputing in the walk what sema already knows, would be two answers to one
question, which is worse. Fortran 95's newly obsolescent features follow the same rule: the computed
GO TO and `CHARACTER*n` are the walk's, but a statement function (`defineStmtFunction`), DATA
among the executable statements (`order`, which knows the unit's group), and a `CHARACTER*(*)`
function (`assumedLengthFunction`) are sema's. Fixed source form belongs to the lines, so
`ReportSourceExtensions` flags it, once per file at its first statement.

The same is true of constant expressions. Fortran 90 lets an initialization expression call an
elemental intrinsic only of INTEGER or CHARACTER arguments (its 7.1.6.1). govax's constant
evaluator (section 25) computes far more than that, which is useful, and is an extension to say
so: the evaluator notes each intrinsic it computes while evaluating a declaration (`initRule`), not
while folding an executable statement's expression, which the standard doesn't restrict.

Writing the obsolescent checks found a lexer bug. `ASSIGN 10 TO LAB` in free form was
`%FORT-E-BLANKINNAME`: the lexer read `TOLAB` as one squeezed token and then, quite correctly for a
name, refused the blank inside it. The fix splits ASSIGN's `TO` off as a keyword after the label's
digits, so free form's blank rules apply to it as to any keyword.

### Warnings by scope

`/WARNINGS` (section 7) works from the cross reference's mentions: each name's list of where it
was declared, assigned, and used. Fortran 90 adds scopes, and two warnings are about them:

```text
$ FORTRAN/WARNINGS=ALL/NOMACRO TOUR.F90
%FORT-W-UNUSEDNAME, LOOKUPS is declared but never used
  at line 7 of TOUR.F90
    integer, private :: lookups = 0
  ----------------------^
%FORT-W-HOSTHIDDEN, I is declared here, hiding TOUR's I
  at line 39 of TOUR.F90
    subroutine show (i)
  -------------------^
%FORT-W-UNREAD, LAST is given a value here but never used
  at line 31 of TOUR.F90
    last = n
  --^
```

**A private module variable** is the only kind of module entity the compiler can call unused: a
public one is for the module's users, whom it can't see. The code meant to report such names all
along, and never did, for a reason worth knowing. UNUSEDNAME is never reported for a name in COMMON
(another unit may use it), and section 31 gave a module's variables a common block of their own,
`$MOD$STORE`. The exemption was right for COMMON and wrong for this block; the fix exempts a
block only if it isn't a module's (`CommonBlock.Module`).

**HOSTHIDDEN**: an internal procedure that declares a name its host has (here SHOW's dummy I) can't
reach the host's. That's legal and usually meant (a loop index of its own), so it's a warning under
`DECLARATIONS`, off by default. It isn't reported for a generic interface, which by the standard's
14.1.2.3 extends its host's of the same name rather than hiding it.

**UNREAD** is a variable given a value that nothing reads. "Nothing" is by the order of the
statements, not their flow: the warning has the mentions, not a flow graph, so `M = M + 1` reads M
and a name passed as an argument counts as read. It leaves out whatever something other than the
unit's own statements may read: a dummy, a function's value, COMMON and module variables, an
EQUIVALENCE, a NAMELIST group, VOLATILE, a pointer, a TARGET, and DO variables (`DO I = 1, 3` with a
body that never mentions I is an idiom, not a mistake).

UNREAD needed the mentions to be right, and two were wrong. A name that is both read and written by
one mention (RANDOM_NUMBER's seed, MVBITS's TO) had been recorded as written only, so the seed was
"never used"; and an internal file READ from an element, `READ (DIGITS(K), ...)`, recorded no
mention at all. Both are fixed where mentions are recorded (`noteReadsToo`, `notePlace`), so the
cross reference is right too.

A third was found writing this section. The tour's I appears only as the implied DO's variable in
`(/ (REAL (I), I = 1, N) /)`, and was reported "declared but never used". By the standard that
variable is a *statement entity*, not the unit's I; but it takes I's type, and under IMPLICIT NONE
the declaration is the only way to give it one, so deleting it as the warning suggested would make
an error. A statement function's dummy argument has always been treated this way (section 7); an
implied DO's variable in DATA or a constructor now is too (`Symbol.typesEntity`).

### Listings that know scopes

Phases 103 and 106 already gave each contained unit its own page, map, and cross reference. Phase
110 filled in the rest:

```text
Storage map for TOUR, the main program
  A      REAL, ALLOCATABLE, TARGET (:)  descriptor at $LOCAL+0   TOUR.A
  B      REAL, ALLOCATABLE, TARGET (:)  descriptor at $LOCAL+32  TOUR.B
  P      REAL, POINTER                  $LOCAL+64                TOUR.P
  BEST   TYPE(ITEM)                     $LOCAL+68                TOUR.BEST
  ...
Cross reference for TOUR.SHOW               (D defined, A assigned, a blank is used)
  A       REAL array, ALLOCATABLE, TARGET from TOUR  41
  BEST    TYPE(ITEM) from TOUR                       41
  I       INTEGER, INTENT(IN)                        39D  40D  41
  P       REAL, POINTER from TOUR                    41
  TOTAL   REAL*8 from TOUR                           41
```

Attributes (ALLOCATABLE, POINTER, TARGET, INTENT, OPTIONAL, PRIVATE) follow the type in both the map
and the cross reference (`Symbol.Attributes`). An allocatable array's place is its *descriptor's*
(section 29), and the map says so: `$LOCAL+0` alone would suggest the elements are there. The
internal procedure's cross reference lists the host's names it uses, `from TOUR`, as a USE's names
are listed `from STORE`; before, SHOW's page didn't mention A at all, and the host's row was the only
record that SHOW reached it. Generic names, `OPERATOR(.op.)`, and `ASSIGNMENT(=)` have rows, used at
each operation that calls one of their procedures; a module procedure has a row in its module's
cross reference. And a symbol whose name starts with `$`, a compiler temporary such as a WHERE
mask's `$W`, is never listed: the survey had found one in a program's map.

### `/CHECK=POINTERS`: a test inside the expression

The tour's last PRINT uses P after NULLIFY. A pointer is its target's address (section 32), and a
disassociated one's is 0, so without a check the program reads address 0, an access violation far
from the mistake. `/CHECK=POINTERS` tests the address where it is used:

```text
;   37    print *, p
	...
	TSTL	TOUR.P
	BNEQ	TOUR.$37
	CALLS	#0,G^FTN$NOTASSOC
TOUR.$37:
	MOVAL	@TOUR.P,TOUR.$D3+4
```

```text
%FOR-F-NOTASSOC, pointer is not associated
%TRACE-F-TRACEBACK, symbolic stack dump follows
module name     routine name                     line       rel PC    abs PC

                                                           00001CB9  00001CB9
STORE           TOUR                               37      00000304  00000A44
```

The same keyword covers allocatable arrays (an element of one not allocated is `%FOR-F-UNALLOCATED`):
both are a descriptor or cell whose address may be 0, and two keywords for one test would only be
two things to remember. The conditions are govax's own, FOR numbers 311 and 312, signaled by FTNRTL
routines that never return, as section 29's NOTCONFOR is.

Where the test goes is the interesting part. `/CHECK=BOUNDS` (section 19) puts its test, the IR's
`InRange`, *inside* the expression that computes the subscript, not in a statement before it. The
pointer check does the same, as a new IR expression, `ir.Guard`: "X, once the test is found not to
be 0". Putting it ahead of the statement would have been simpler and wrong:

```fortran
if (associated (p) .and. p%key > 0) ...
do while (associated (q%next) .and. q%next%key < k)
```

These are correct programs only because `.AND.` doesn't evaluate its right side when the left is
false, section 19's reason for putting InRange where it is. A check hoisted to the statement's start would test P before
ASSOCIATED had, and stop a correct program. Inside the expression, the guard runs only where the
reference does. Every pass that walks expressions had to learn the new node, which is the cost:
the optimizer treats a guard as something that can fault, as it does InRange, so it may drop one
it can prove passes (of an address, or a nonzero constant) but never moves one ahead of the branch
that protects it.

The survey had also found that `SUM(A)` after `DEALLOCATE(A)` ran and printed a value read from the
freed block. DEALLOCATE clears the descriptor's POINTER, but an element is reached from A0 and the
bounds, which stay. Under the check POINTER is tested first; without it, using a deallocated array
is still undefined, as the standard says.

### What the debugger is told

Section 18's `/DEBUG` table is comment lines in the MACRO file, which the debugger reads; MACRO's own
debug records know only MACRO's labels. Phase 110 added three kinds of line:

```text
;DBG$ V TOUR P ^RF 4 S:TOUR.P - -
;DBG$ V TOUR BEST REC:ITEM 12 S:TOUR.BEST - -
;DBG$ B TOUR TOUR 25
;DBG$ S TOUR ITEM 12 D
;DBG$ C TOUR ITEM NAME 0 CH 8 -
;DBG$ C TOUR ITEM PRICE 8 RF 4 -
```

An **S** line is a structure (its size, and D for a derived type, shown with `%`, or R for DEC's
record, shown with `.`), and a **C** line each component (offset, type, size, bounds). A V line of a
record names its structure, `REC:ITEM`, and a pointer's type has `^` before it, its place the
pointer's own storage. A **B** line says which FORTRAN line each entry point's code reaches first.

With those, the session the survey couldn't have:

```text
DBG> SET BREAK TOUR.SHOW
DBG> GO
break at routine STORE\TOUR.SHOW
FORTRAN line 41: print '(1x, 6f6.1, f8.1, 1x, a, f6.2)', a(1:i), total, best%name, p
DBG> EXAMINE BEST
STORE\TOUR.BEST%NAME:   'PEAR    '
STORE\TOUR.BEST%PRICE:  6.000000
DBG> EXAMINE A(1:5:2)
STORE\TOUR.A(1):        5.000000
STORE\TOUR.A(3):        10.00000
STORE\TOUR.A(5):        15.00000
DBG> EXAMINE P
STORE\TOUR.P:   10.00000
DBG> EXAMINE TOTAL
STORE\TOUR.TOTAL:       60.00000000000000
```

Four things made it work. The location reader (`debugger/fortranref.go`) used to be a regular
expression for a name and at most two groups; it is now a small scanner that splits a location into
names, `%`s, periods, and balanced parentheses, and a walk that takes each step in turn, a group
being subscripts, a section, or a substring by what it follows. A section is just a list of elements,
so `A(1:5:2)` came almost free once the walk could carry a list. A pointer is *followed*: its cell
holds an address (or, for a CHARACTER or array pointer, a descriptor), and an address of 0 is
`%DEBUG-W-NOTASSOC`. In a record shown whole a pointer component is shown as an address and not
followed, since a list's NEXT can lead round forever. A bare name is looked for in the unit, then
its hosts in turn: SHOW has no A, so A is TOUR's, the host's name being SHOW's less its last part.
And the **B** line moves a break: `SET BREAK TOUR.SHOW` used to stop at SHOW's first instruction,
before the entry code that copies an assumed-shape dummy's descriptor into its cell (section 29), so
the dummy read as not allocated until a STEP. Now a break at a FORTRAN routine stops at the
B line's statement, after that code.

A module's variable is `STORE.LOOKUPS`, the same dotted path as any unit's: the module is a unit in
the table like any other.

### Traceback lines

The traceback above says line 37. Before Phase 110 that column was empty for a FORTRAN routine,
because a traceback's line numbers come from the image's debug symbol table, which LINK fills from
the object's traceback records, and MACRO, assembling the compiler's MACRO file, had no FORTRAN
lines to give it. VAX FORTRAN, writing objects itself, gave the FORTRAN line.

Three ways were weighed: a table of lines as data in a psect of the compiler's own, read at
traceback time; line records from govax's MACRO; or line records only when `/OBJECT` has the
FORTRAN command run MACRO itself. The second was chosen. The compiler writes a marker after
`.IDENT`,

```text
	.IDENT	/01/
; TBT$ FORTRAN: a traceback's line numbers are the FORTRAN lines below
```

and govax's MACRO, seeing it, numbers each line of code by the last FORTRAN source comment before it
and writes those numbers as line records among the traceback records (`internal/asm/fortranline.go`).
LINK and the traceback (`dbgsym`'s `TraceName`) needed no change: they already read line records,
which VMS's own compilers wrote. Without the marker MACRO does exactly what it did, so a MACRO file
assembled by VMS's MACRO, which doesn't know the comment, still gives VMS's object, and the VMS
rounds' stored objects (section 20) are unchanged.

`/DEBUG` complicates it. An object has one line table, and under `/DEBUG` it must be the MACRO
file's, since the debugger steps by MACRO lines and maps them to FORTRAN lines through the comments.
So under `/DEBUG` MACRO writes no FORTRAN table, and a traceback in such an image (`RUN/NODEBUG`)
does at traceback time what the debugger does: maps the MACRO line to its FORTRAN line through the
file's comments (`internal/machine/traceline.go`).

### Two loop improvements, measured

Phase 104's assumed-shape dummies reach an element as A0 plus the subscript times the stride
(section 29). The survey compiled small kernels and read their inner loops: an explicit-shape `A = B`
is two instructions an element, but `V = V + W` on two assumed-shape dummies was six, a multiply and
an add per operand recomputing each address every trip. Section 16's induction pass already turns
"index times a constant" into "an index stepped by the constant", but it treated a computed address
(`ir.Mem.Ptr`) as untouchable. It now takes an address as a candidate too, so the register holds the
element's address and steps by the stride. RAISE, in the tour, compiled `/OPTIMIZE`:

```text
STORE.RAISE.$15:			; array (line 13), dimension 1's body
	MULF3	R10,(R8),R0
	ADDF2	R0,(R7)
	ADDL2	STORE.RAISE.$B1+20,R7
	ADDL2	STORE.RAISE.$B2+20,R8
	AOBLEQ	R9,R6,STORE.RAISE.$15
```

F is loaded into R10 before the loop (section 16's invariant motion), and each operand's address is
a register stepped by its descriptor's stride. The strides stay memory operands: in registers they
would save a read, not an instruction. One subtlety: section 32's alias analysis must still know
that `(R7)` is V's element, or a store through it could be moved past a read of V. So the stepped
register's cell records the place it addresses (`Cell.Points`).

The second improvement is **fusion**: adjacent array assignments of one shape become one loop nest,
and a WHERE construct's arms one nest testing the mask once:

```text
;    3    b = a + 1.0
	...
FUSE.$1:				; array (line 3), dimension 1's body
	ADDF3	-4(R8)[R6],FUSE.$KF1,-4(R7)[R6]
;    4    c = b * 2.0
	MULF3	-4(R7)[R6],FUSE.$KF2,-4(R9)[R6]
	AOBLEQ	#1000,R6,FUSE.$1
```

Fusing is safe when each array one statement reads that another writes is read only at the same
element, or not at all, which is section 27's overlap analysis asked across statements instead of
within one (`sema/fusion.go`'s `Fusable`). The shapes must be the same *as the compiler can tell*: an
extent of the same constant, the same bounds' text, or the same array. That is a stronger condition
than the shapes being equal, and deliberately so. The tour's `B = A + 1.0` and `A = B * 2.0` don't
fuse, though A and B are both N long, because they're allocatable: their bounds are whatever the
program last allocated, and the compiler can't prove they agree. Lowering does the fusing
(`ir/lowerfuse.go`), not the optimizer, because only lowering knows the nests are nests; each part
keeps its own statement, so its source line and the debugger's line labels come out where they
belong. It is skipped under `/CHECK=BOUNDS`, whose checks belong before each statement's own loops.

Two kernels in `testdata/fortran90`, `optfuse.f90` and `optstride.f90`, measure them: under
`/OPTIMIZE` the first ran 28.5% fewer instructions with fusion, the second 26.1% fewer with stepped
addresses. Two other ideas were measured and left: choosing an overlapping assignment's loop direction
at run time rather than copying through a temporary, and computing `A = F(X)` into A rather than
through F's temporary. Each would save about half of its statement's loop, but both are rare in the
corpus and each needs machinery of its own (a loop chosen by a test; proof that F can't reach A).

### What the programs found

The phase's programs are tests more than corpus programs: `standard90_test.go`'s witness,
`obsolescent_test.go`, `warnings90_test.go`, `listing90_test.go`, `pointercheck_test.go` and the
console's `TestFortranCheckPointers` (13 programs, each run with and without the check, both optimizer
settings, both paging modes), the debugger's `fortran90_test.go`, `TestFortranTracebackLines`, and the
two kernels. What they found was mostly in the earlier phases: the ASSIGN lexer bug, the module block
exempting private variables, the two missed mentions, the implied DO's variable, a `$W` in the map,
and SUM over a freed block. Each of them was the kind of thing that a program's output never shows,
which is why sections 24 to 34's programs, which check output, didn't find them.

What waits: the run-time overlap test and `A = F(X)` into A, above; a break at a FORTRAN line given
as `%LINE n`, which is still the MACRO file's line (`UNIT.$LINEn` is the FORTRAN one); and the
debugger's language, which stays MACRO, since the module it reads is MACRO's (section 18).

## 36. A whole program: fcalc (Phase 111)

Sections 24 to 35 each ended with what its programs found, and those programs were small, each
written to exercise its own phase. A compiler's harder test is a program written to *do*
something, which uses whatever it needs, in whatever combination. For VAX FORTRAN that program was
DUNGEON (section 20), written by other people long ago. No Fortran 90 program of that kind was at
hand, so Phase 111 wrote one: **fcalc**, a scientific calculator at a prompt, with complex values,
vectors and matrices, user-defined functions, equation solving and integration, ASCII plots, and
saved sessions. The manual's 6.6 is how to use it, and its 14.13 has a table of its sixteen files
and what each shows of Fortran 90. This section uses it the way section 1 used AVERAGE, but the
other way round: not one small program through every pass, but one large program through the
parts of the compiler only a large program reaches.

### Sixteen files, one order

fcalc is 11,017 lines of Fortran 90 (about a fifth of them comments, for a reader learning the
language and the numerical methods) in fifteen modules and a main program, each file one unit.
Compiled `/G_FLOATING /OPTIMIZE` they are 64,560 lines of MACRO and 336 routines, and the image,
linked against FTNRTL, MTHRTL, LIBRTL, and LBRSHR, is 528 blocks.

The files must be compiled in an order, `internal/fcalc/src/compiles.txt`:

```text
FCPLATFORM FCKINDS FCMESSAGES FCVALUES FCARRAYS FCLEXER FCTREE FCSYMBOLS
FCFUNCS FCPOLY FCMATRIX FCEVAL FCNUMERIC FCPLOT FCSTATE FCALC
```

because each compilation of a module writes `NAME.F90$MOD`, the module's text (section 31), and
a USE reads it. FCVALUES uses FCKINDS and FCMESSAGES; FCEVAL, near the end, uses nearly
everything. That is the only coupling at compile time. Each MACRO file is assembled alone, and
what joins them is LINK resolving names like `G^FCVALUES.V_ADD`, a module procedure's name being
its module's and its own (section 31's `Unit.LinkName`). A change to a module's private code
changes nothing its users see, and since `writeModuleFiles` rewrites a module file only when
`sema.ModuleDigest` changes, a build tool watching file times would rebuild only that module.

fcalc ships with govax (the system volume's `SYS$SYSTEM:FCALC.EXE`), and its image is built the
way FTNRTL's is (section 14): `internal/fcalc/build` compiles, assembles, and links in Go, keeping
each module's text in memory for the next USE (a `sema.ModuleFinder` of its own) instead of in
files, with times and tool names fixed, so `TestCommittedImage` can hold the committed image to
the sources byte for byte.

### One line: `a + (-b)`

Here is all of FCVALUES' subtraction:

```fortran
  type value
    complex(rk) :: z
    integer :: err
  end type value

  interface operator(+)
    module procedure v_add, v_plus
  end interface

  interface operator(-)
    module procedure v_sub, v_neg
  end interface
  ...
  function v_sub(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v

    v = a + (-b)
  end function v_sub
```

A VALUE is a complex number and an error code, so that an error travels with the result of
every operation instead of stopping the program (more on why below). The compiler makes this of
it:

```text
; Storage map for FCVALUES.V_SUB, a TYPE(VALUE) function
;   V     TYPE(VALUE)              hidden argument 1  @FCVALUES.V_SUB.V(AP)
;   A     TYPE(VALUE), INTENT(IN)  argument 2         @FCVALUES.V_SUB.A(AP)
;   B     TYPE(VALUE), INTENT(IN)  argument 3         @FCVALUES.V_SUB.B(AP)
...
FCVALUES.V_SUB.$T1:
	.BLKL	8			; characters, for one statement
FCVALUES.V_SUB.$T2:
	.BLKL	8			; characters, for one statement
...
	.ENTRY	FCVALUES.V_SUB,^M<R2,R3,R4,R5>
;  392      v = a + (-b)
	PUSHAB	@FCVALUES.V_SUB.B(AP)
	PUSHAB	FCVALUES.V_SUB.$T2
	CALLS	#2,G^FCVALUES.V_NEG
	PUSHAB	FCVALUES.V_SUB.$T2
	PUSHAB	@FCVALUES.V_SUB.A(AP)
	PUSHAB	FCVALUES.V_SUB.$T1
	CALLS	#3,G^FCVALUES.V_ADD
	MOVC3	#32,FCVALUES.V_SUB.$T1,@FCVALUES.V_SUB.V(AP)
;  393    end function v_sub
	RET
```

Four phases' decisions are in those eleven instructions.

**The type is 32 bytes, not 20.** The module's storage map says so:

```text
;   Type VALUE, 32 bytes
;     +0     Z     COMPLEX*16
;     +16    ERR   INTEGER
```

VALUE isn't a SEQUENCE type, so it is laid out naturally aligned (section 30): each component
on a boundary of its own size, here 16 for the COMPLEX\*16, and the whole a multiple of its
largest boundary, so that in an array of VALUEs every Z is aligned too. `SEQUENCE` would have
made it 20 bytes, at the cost of misaligned Zs. The VAX doesn't fault on a misaligned operand, as
some machines do; it is only slower, and the choice is the language's rule, not this machine's.

**The operators are chosen while compiling.** Neither `-` nor `+` means anything for a derived
type, so sema looks among the functions each operator's interface names (`sema/operators.go`'s
`resolveOperator`, by section 28's rules): `-b` has one operand, which fits V_NEG and not V_SUB,
and `a + ...` two, which fit V_ADD. From then on each is an ordinary function reference. There is
no dispatch when the program runs, and nothing in the MACRO shows there was an operator.

**A value past 8 bytes goes through a hidden argument** (section 30). A function's value of up
to 8 bytes comes back in R0 and R1; a larger one is written where the caller says, through an
argument the source doesn't show, the first. So each call here gets a temporary from the
statement's pool: V_NEG writes −B into `$T2`, which is then V_ADD's second argument, and V_ADD
writes the sum into `$T1`. (The pool began with CHARACTER temporaries, hence the comment; every
temporary of so many bytes shares it now, `ir`'s `textTemp`.) V_SUB's own V is a hidden argument
too, so the result is moved there whole, by `MOVC3`.

**What it doesn't do.** The last move could be avoided: V_ADD could be handed V's own place,
`@V_SUB.V(AP)`, since the standard keeps a function's result from being associated with its
arguments. The compiler instead always passes a temporary and never the assignment's target,
which is safe for every function without asking whether the target could reach the arguments.
For fcalc that costs one 32-byte move per operation, beside two or three calls. It is the scalar
cousin of computing `A = F(X)` into A, which section 35 measured and left.

### A tree walked by pointers

fcalc parses each line into a tree of NODEs (FCTREE) and evaluates the tree recursively
(FCEVAL). A NODE's operands are a list: KID points to the first, and each one's NEXT to the one
after:

```text
;   Type NODE, 96 bytes
;     +0     KIND    INTEGER
;     +4     OP      INTEGER
;     +8     COLUMN  INTEGER
;     +12    NAME    CHARACTER*31
;     +48    NUMBER  TYPE(VALUE)
;     +80    KID     TYPE(NODE), POINTER
;     +84    NEXT    TYPE(NODE), POINTER
```

A scalar pointer is just an address, a longword (section 32), which is how a type can contain a
pointer to itself: its size doesn't depend on its own. Here is FCEVAL's `if(c, a, b)`, which
evaluates only the arm it chooses, so that a user's recursive function can stop:

```fortran
  recursive function choose(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(node), pointer :: arm
    logical :: chosen
    ...
    if (chosen) then
      arm => p%kid%next
    else
      arm => p%kid%next%next
    end if
    v = evaluate(arm, f)
  end function choose
```

Its storage map, and the two pointer assignments:

```text
; Storage map for FCEVAL.CHOOSE, a RECURSIVE TYPE(QUANTITY) function
;   V       TYPE(QUANTITY)                     hidden argument 1  @FCEVAL.CHOOSE.V(AP)
;   P       TYPE(NODE), POINTER                argument 2         @FCEVAL.CHOOSE.P(AP)
;   F       TYPE(FRAME), INTENT(IN), OPTIONAL  an argument        @FCEVAL.CHOOSE.$A3(FP)
;   ARM     TYPE(NODE), POINTER                FP-112             FCEVAL.CHOOSE.ARM(FP)
;   CHOSEN  LOGICAL                            FP-108             FCEVAL.CHOOSE.CHOSEN(FP)
...
;  622        arm => p%kid%next
	MOVL	@FCEVAL.CHOOSE.P(AP),R0
	MOVL	80(R0),R0
	MOVL	84(R0),FCEVAL.CHOOSE.ARM(FP)
...
;  624        arm => p%kid%next%next
	MOVL	@FCEVAL.CHOOSE.P(AP),R0
	MOVL	80(R0),R0
	MOVL	84(R0),R0
	MOVL	84(R0),FCEVAL.CHOOSE.ARM(FP)
```

P is a pointer dummy, so its argument is the address of the caller's pointer (section 32), and
`@P(AP)` loads the pointer itself: the node's address. Each `%KID` or `%NEXT` through a pointer is
then one `MOVL` from the component's offset (`ir`'s `fieldPlace`, `pointerOf`), and `=>` stores the
address it ends with. Nothing is copied; that is what makes a tree cheap to walk. The map shows
three more phases' work. CHOOSE is RECURSIVE, so ARM and CHOSEN live in its frame, at negative
offsets from FP (section 23), and a call of CHOOSE from inside an evaluation CHOOSE started gets
its own. F is OPTIONAL, so it is reached through a cell, `$A3`, which the entry code fills only
if the call's argument count reaches it (section 28): FCEVAL passes F, a user function's frame of
parameter values, only inside a user's function. Under `/CHECK=POINTERS` each of those loads
gets a test first (section 35), and an unassociated KID would stop the program with FTNRTL's
NOTASSOC rather than read address 80.

### A program written so two machines agree

fcalc was to print the same lines on govax, under gfortran on an IEEE host, and on a real VAX.
Most of that is the program's design, not the compiler's, and it is worth saying why, because a
compiler writer meets the same differences:

- **Range.** fcalc's kind is `SELECTED_REAL_KIND(15, 300)`, IEEE double's precision and range.
  Compiled `/G_FLOATING` that is REAL\*8, G_floating, the same 53 bits. Under the default,
  D_floating, whose REAL\*8 reaches only 1E38, the same request is answered with REAL\*16,
  H_floating: the program still compiles and runs, which is what asking by need rather than by
  size is for (section 25), but twice the size and slower, so fcalc is built `/G_FLOATING`.
- **Nothing traps.** A VAX faults on floating overflow, division by zero, and a reserved
  operand, where an IEEE machine gives an infinity or a NaN and goes on. So every operation in
  FCVALUES checks its operands before computing (`|x*y| > LIMIT` is tested as `|y| > LIMIT/|x|`,
  which can't overflow), and gives a VALUE with an error code instead. That is what VALUE's ERR
  is for.
- **Underflow** gives zero on both machines only if the program never makes a denormal, which
  the VAX hasn't: fcalc treats a value below TINY as zero itself.
- **Digits.** Twelve significant digits are shown by default, so the last bits, where MTHRTL
  and the host's library may differ, don't reach the output.

The compiler's part is to compute what the source says: no pass of the optimizer reassociates
arithmetic (it swaps only the operands of a commutative operation, which is exact), so every
build computes the same bits, and fcalc printed the same lines in every one. The checking builds
earned their place on the *program*: `/CHECK=UNDERFLOW` found that fcalc's own formatting
multiplied a value near 1E-300 by 1E-12, which an IEEE machine turns silently into a denormal and
a VAX under the check faults on.

### What the program found

fcalc was written a subtask at a time, each part's scripts worked by hand from the mathematics
and then run in every build, and the compiler bugs it found were fixed in subtasks of their own,
each with a corpus program or test that holds it:

| What failed | Why | Fixed in |
| --- | --- | --- |
| `REAL(Z)` of a COMPLEX(8) was REAL\*4 under `/F90`, and `CMPLX(X, Y, 8)` of constants lost all but 24 bits | VAX FORTRAN's Table D-3 read where Fortran 90 reads differently; the constant computed in COMPLEX\*8 and converted | `sema`'s `fortran90Readings`, `evaluator.complexType` |
| An INTENT(IN) dummy as a READ's internal file was INTENTIN | A READ's internal file is read, not given a value | `sema`'s `sourcePlace` |
| A section of a scalar record's array component under `/CHECK=BOUNDS` crashed the compiler | The bounds were asked of the record, which has none, before the field | `ir`'s `checkSubscripts` |
| `/OPTIMIZE/CHECK=ALL` made a byte branch MACRO couldn't reach | `@(Rn)` is assembled `@0(Rn)`, two bytes, and was counted as one | `codegen`'s `operandBound` |
| An internal procedure of a module procedure with a long name was called by a name nothing defined | The call divided `M.HOST.INNER` at the first period, the definition at the last; past 31 characters each was shortened from a different prefix | `ir`'s call naming |
| `N = SIZE(Z)` of an INTENT(OUT) dummy was INTENTOUTREF | An inquiry asks about the dummy's shape, not its value | `sema`'s `noteInquired` |
| `rand()` stopped `/CHECK=ALL`'s build with ADJARRDIM | Under Fortran 90 an adjustable array may have no elements | `ir`'s `checkArrayBounds` |
| `'0123456789'(K:K)` was a syntax error | A substring of a literal constant, which Fortran 90 allows | `parse`'s `isCharLiteral` |
| VMS's MACRO refused six modules with DIRSYNX | The compiler wrote a bare `.ASCII` for a text of no characters, and govax's MACRO had accepted it | `codegen`'s `asciiPieces`; govax's MACRO refuses it as VMS's does |
| VMS's objects of fcalc's MACRO differed from govax's | govax's MACRO stored a repetition factor (`.LONG 0[2]`) as bytes, where VMS's stacks the value and count; govax's LINK couldn't run those commands | `asm`'s `emitRepeated`, `link`'s `storeRepeated` |

Three things stand out.

**Each bug was two features meeting.** None was in a construct a phase had set out to compile,
and each phase's programs had passed: a bounds check and a component's section, a long name and
an internal procedure, an inquiry and INTENT(OUT), a substring and a literal. A phase's programs
test its features; a real program tests their pairs, which no one writes on purpose.

**A build mode must be compared whole.** Three of the bugs showed in one build only. One of
them, the empty adjustable array, stopped the `/CHECK=ALL` build in the middle of a script, and
the rest of that script's input was read as the start of the next run, which still looked as if
it passed. Since then each build's whole output is held to every script's expected output
(`TestFcalcMilestone`, ten builds), not just "it ran".

**The real machine was again less forgiving** (section 20's lesson). The empty `.ASCII` had
assembled on govax since the code generator first wrote one, and only VMS's MACRO said it was
wrong. Getting there found one more forgiveness, below the compiler: the exchange volume's index
file had an extension header made by govax's disk library, and VMS's file system crashed the VAX
on reading it. The round went round it (the volume made with room for its headers, `/HEADERS=`),
and why VMS objects is still open (DEVIATIONS.md). The comparison of objects found the repetition
factor, and also one place where VMS's MACRO packs records by a rule no probe has explained:
`testdata/tir111` reproduces it, and the bytes it stores are the same.

fcalc found bugs in itself the same way. The rehearsal of the VMS round on govax, with input
from a file, showed that its prompt's record stays open after the read: under Fortran 90's
nonadvancing rules FTNRTL ends a prompt's record only at a terminal's READ, as VMS's would, and
fcalc had assumed the terminal. What it didn't find is worth as much: gfortran, building fcalc
plainly, optimized, and with every run-time check it has and floating traps on, printed every
line the same, so the design of the previous part held. The one disagreement is a reading:
govax refuses a public type with a component of a type that is private in the using module
(ACCESSCOMPONENT, Phase 106's reading of 4.4.1 and 5.2.3), where gfortran accepts it, so fcalc's
modules name such types PUBLIC.

### Testing a whole program

What holds fcalc, and through it the compiler:

- **Scripts worked by hand.** `testdata/fcalc`'s eleven scripts (`basics` to `errors`) each
  have the output worked from the mathematics (with Python for the arithmetic), and for plots and
  long tables checked against gfortran's, each difference resolved by hand.
- **Every build.** `TestFcalcMilestone` (`internal/console`) builds fcalc ten ways (`/OPTIMIZE`
  both ways and `/RECURSIVE` both ways, each with paging off and on; `/CHECK=ALL`; `/DEBUG`), runs
  every script in each, and checks the files SAVE and LOG write and the foreign command.
- **The tools.** `TestFcalcTools` (`internal/dclcmd`) runs `/LIST/CROSS_REFERENCE`,
  `/STANDARD=F90`, and `/WARNINGS=ALL` on every module; each message left is listed with its
  reason (`fcalcToolMessages`). `TestFcalcDebugger` holds a debugger session to a log: a break in
  the evaluator at each level of a user function's recursion, and EXAMINE through a tree's
  pointers (`FN%BODY%KID%OP`).
- **gfortran**: `testdata/fortran/gfortran/compare.py` builds it three ways and compares every
  line of every script and file.
- **VMS**: `testdata/f90r` has VMS 7.1's MACRO and LINK build FTNRTL and fcalc from govax's
  MACRO files, four builds, and run every script; `TestF90ROracle` holds govax's run of the same
  procedure to VMS's log, and `TestF90RObjects` govax's MACRO to VMS's objects.

If you're writing a compiler and no real program in its language is at hand, write one: not a
test, but a tool you would use, big enough that you stop thinking about the compiler while you
write it. That is when you write the combinations a test-writer wouldn't: fcalc's first part,
the expressions and the prompt, found two compiler bugs.

## 37. Fortran 95 (Phase 112)

Fortran 95 is a small revision of Fortran 90, and most of it is not new machinery but new
*rules* over the machinery sections 24 to 35 built. FORALL assigns over an index set, which
the scalarizer of section 27 already knew how to do for a section; ELEMENTAL procedures are
called element by element, which section 33 already did for the elemental intrinsics; default
initialization is an initial value, which section 8 placed, except that sometimes it must be
placed at every call. What Fortran 95 changes is *when* those things happen and *what a
program may rely on*, and those are what a compiler gets wrong. This section is about the
decisions, more than the code.

govax compiles Fortran 95 under the same `/F90` as Fortran 90 (Phase 112's Decision 2): there
is no Fortran 95 mode. A new form VAX FORTRAN gave no meaning (FORALL, PURE, ELEMENTAL, a
WHERE in a WHERE) is compiled under `/NOF90` too; the three that change what an old text means
(`A(5:1)`, `I0`, a `!` in namelist input) are only `/F90`'s, and VAX FORTRAN's programs get
the answers they always got.

The section follows a short program, a rod heated at one end:

```fortran
module heat
  implicit none
  type cell
    real :: t = 20.0
    logical :: fixed = .false.
  end type cell
contains
  pure real function mean(a, b)
    real, intent(in) :: a, b
    mean = (a + b) / 2.0
  end function mean

  elemental real function clamp(x, lo, hi)
    real, intent(in) :: x, lo, hi
    clamp = min(max(x, lo), hi)
  end function clamp
end module heat

program rod
  use heat
  implicit none
  integer, parameter :: n = 8
  type(cell) :: bar(n)
  real :: t(n)
  integer :: i, step

  bar(1)%t = 100.0
  bar(1)%fixed = .true.
  do step = 1, 3
    forall (i = 2:n-1, .not. bar(i)%fixed)
      bar(i)%t = mean(bar(i-1)%t, bar(i+1)%t)
    end forall
  end do
  forall (i = 1:n) t(i) = bar(i)%t
  print '(1x, 8(1x, f0.1))', t

  where (t > 20.0)
    where (t > 60.0)
      t = clamp(t, 0.0, 60.0)
    elsewhere
      t = t - 1.0
    end where
  elsewhere (t < 20.0)
    t = 20.0
  end where
  print '(1x, 8(1x, f0.1))', t
  print '(1x, a, i0)', 'hottest free cell: ', maxloc(t(2:), dim=1) + 1
end program rod
```

It prints:

```text
 100.0 70.0 40.0 30.0 20.0 20.0 20.0 20.0
 60.0 60.0 39.0 29.0 20.0 20.0 20.0 20.0
hottest free cell: 2
```

Every cell starts at 20 degrees because the type says so. Each step sets every free cell to the
mean of its neighbours *as they were before the step*: in the first step cell 2 becomes 60 and
cell 3 stays 20, since cell 2's new value isn't seen until the next step. A DO loop over I
would have given 60, 40, 30, 25, ... in one step, each cell reading its neighbour's new value.
That difference is FORALL's whole meaning, and the first thing its code has to get right.

### A header that survives a word it doesn't know

Fortran 95's procedures may start with PURE or ELEMENTAL, beside Fortran 90's RECURSIVE, in any
order, before or after the type: `ELEMENTAL REAL FUNCTION`, `REAL PURE FUNCTION`, `RECURSIVE
PURE SUBROUTINE`. The lexer reads them as it read RECURSIVE (`lex/f95stmt.go`'s
`headerPrefixes`), each a flag on the statement. That was the easy part.

The survey found the hard part by accident. Before Phase 112 a PURE function's header was an
unrecognized statement, and an unrecognized first statement meant the unit had no header: its
statements joined the unit before, whose END then came too early, whose CONTAINS was in the
wrong place, and a dozen messages followed, none of them saying PURE. A compiler that will meet
later standards' words (Fortran 2008's IMPURE, for one) should recover from a prefix it doesn't
know. `lostHeader` does: a unit's first statement that would be unrecognized, but has words
before a known prefix, FUNCTION, or SUBROUTINE, is reported once and read from the known word on:

```text
%FORT-E-UNRECPREFIX, IMPURE is not a prefix FORTRAN knows; the rest is read as a SUBROUTINE statement
  at line 16 of ERRS.F90
  impure elemental subroutine twice(x)
  ^
```

The unit keeps its header, its ELEMENTAL, and its END, and the only message is the one that
matters. The same idea runs through the parser: a statement with an error keeps its place in
the tree (section 6), and now so does a header with one.

### An empty array is (1:0)

Fortran 90 allows an array of no elements, `REAL A(5:1)`, and VAX FORTRAN called it an error,
BOUNDORDER. Under `/F90` govax now keeps such a dimension. The question was how. Keeping the
declared bounds, 5 and 1, with an extent clamped to zero, would have meant teaching every pass
that reads bounds (LBOUND and UBOUND, which the standard says are 1 and 0 for an empty
dimension of a whole array; descriptors; the debugger's table) that the bounds no longer give
the extent. Instead `sema/arrays.go`'s `emptyDimension` replaces the bounds with **(1:0)**, and
every later pass sees an ordinary array whose extent is 0. The storage map and module files show
`(1:0)` rather than what was written; that is the price.

The bug this found is worth knowing about if you write a compiler: in three places
`Elements() == 0` had meant *the shape isn't known yet* (an assumed-size array, say), and an
empty array is a known shape of no elements. `Symbol.Empty` now says which, so DATASIZE and the
size check no longer divide by zero or refuse an empty array an initial value. When a value
can be "nothing" legitimately, it can't also be the signal for "unknown".

### NULL() has no type of its own

Every other intrinsic function has a row in sema's table (section 33): arguments in, a type out.
`NULL()` has no type; it takes the type, kind, and rank of the pointer it is given to. So it has
no row. Each place a pointer's value goes asks for it by name (`sema/null.go`'s
`nullReference`), and analyzes it against the pointer it is for (`nullValue`): after `=>`, in a
declaration's `=> NULL()`, in DATA, for a pointer component in a structure constructor, as a
pointer dummy's argument. Anywhere else it is NULLPLACE:

```text
%FORT-E-NULLPLACE, a reference to NULL() is a disassociated pointer, so it can stand only where a pointer's value goes: after =>, in DATA for a pointer, for a pointer component in a structure constructor, or as a pointer dummy's argument
```

What `NULL()` compiles to is almost nothing. `P => NULL()` is a NULLIFY. A declaration's
`=> NULL()` gives no bytes: a static pointer's storage already starts at zero (section 32's
choice), which is disassociated. The one place it costs code is a pointer dummy's argument,
which must be the address of a pointer: `nullPointer` makes a pointer of the statement's own,
cleared, and passes that.

A generic name is where `NULL()` gets interesting. The specific is chosen by the arguments'
types, and `NULL()` has none, so it fits any pointer dummy; if that lets two specifics fit, the
reference is ambiguous (NULLMOLD), and the program must say `NULL(MOLD=P)`, whose type then
decides. Writing this found a bug of Phase 107's: a generic's pointer dummy was given the
*target's* address rather than the pointer's, since `resolveGeneric` passed every actual the
ordinary way. No earlier program had a generic with a pointer dummy.

### Default initialization: an initial value, sometimes at every call

A component may say what every object of its type starts with:

```fortran
  type cell
    real :: t = 20.0
    logical :: fixed = .false.
  end type cell
```

`sema/derived.go` keeps each default as `Field.Default`, and the record's bytes with them, as
the structure's `Initials`: the same bytes a STRUCTURE's initial values were (section 13). For
ROD's `BAR`, a main program's array, they are initial values, and the dump shows eight cells'
worth:

```text
  initial $LOCAL+0 BAR(1).T REAL = A0 42 00 00
  initial $LOCAL+4 BAR(1).FIXED LOGICAL = 00 00 00 00
  initial $LOCAL+8 BAR(2).T REAL = A0 42 00 00
  ...
```

(`A0 42 00 00` is 20.0 in F_floating, words swapped as the VAX stores them.)

That is right for a main program's objects, a module's, and a subprogram's SAVE'd ones, which
exist once. But a subprogram's other locals don't keep their values between calls, and the
manual and gfortran agree that default initialization doesn't imply SAVE: such a local starts
with its defaults *at every call*, and so does an INTENT(OUT) dummy (whatever the caller's
object held is gone) and a function's value. Here is a module procedure with both:

```fortran
  subroutine start(c, n)
    type(counter), intent(out) :: c
    integer, intent(in) :: n
    type(counter) :: scratch
    ...
```

Its MACRO begins:

```text
	.ENTRY	TALLY.START,^M<R2,R3,R4,R5>
	MOVC3	#12,TALLY.START.$K1,@TALLY.START.C(AP)
	MOVC3	#12,TALLY.START.$K1,TALLY.START.SCRATCH
```

`sema/defaults.go` decides, object by object, which of the two each is: initial values, or
`Unit.Defaults`, assignments of the record's default bytes (a constant in `$PDATA`, `$K1`) that
`ir/lowerdefaults.go` makes at each entry point, after the allocatable arrays are cleared. An
array is set by the scalarizer's loop, an OPTIONAL dummy only if present, a dummy only at an
entry point whose list has it; ALLOCATE sets what it makes after each successful allocation.

Two consequences are easy to miss. In a RECURSIVE unit, where an initial value makes a local
static (section 23), a local with defaults stays in the frame: it is set at entry, not given a
value once, so each level of the recursion gets its own. And SAVE of a derived type's object had
been refused since Phase 105, by VAX FORTRAN's rule for records applied to derived types too;
Fortran 90 allows it, and with default initialization SAVE is what decides which way the
defaults go, so the old refusal was found and removed.

### PURE: checking where the analysis already looks

A PURE procedure has no side effects: it changes nothing but its own locals and its value (a
subroutine also its INTENT(OUT) and INOUT dummies), does no I/O but to internal files, and calls
only PURE procedures. Fortran 95 needs the promise for FORALL and ELEMENTAL, whose order of
evaluation is the processor's: if a function referenced in a FORALL could change a global, the
answer could depend on the order the tuples were done in.

The rules are a dozen, and the tempting design is a pass of its own, walking every PURE
procedure's statements looking for violations. govax doesn't, because each rule is about
something the analysis already finds for another reason. An assignment to a dummy already went
through `checkIntentIn`, which refuses an INTENT(IN) dummy; it now also asks `pureDefines`
whether the thing defined is a *global* (a module's variable, a host's, or one in COMMON) in a
PURE unit. Every place INTENTIN was checked (assignment, a DO variable, an input item, a
specifier, ALLOCATE of a component) is now checked for globals too, for free, and no place can
be forgotten that INTENT(IN) didn't already forget. The message says which rule and why:

```text
%FORT-E-NOTPURE, CALLS, a variable of the module STATS, can't be given a value in a PURE procedure
  at line 7 of ERRS.F90
      calls = calls + 1
  ----^
```

The other rules sit where their subject is seen: `checkPure`, at the unit's end, looks at the
dummies' INTENTs and the locals' SAVE and initial values, and at `unit.calls`, the list of every
procedure the unit references, which each must reach a PURE procedure through an explicit
interface; `pureStatement` refuses STOP, PAUSE, and I/O but to internal files. A PURE interface
body for a procedure that isn't PURE is INTERFACEDIFF, and a procedure not declared PURE passed
to a dummy procedure whose interface says PURE is PUREACTUAL.

PURE also lets a function compute a declaration's bounds, a *specification function*: `CHARACTER
(LEN=WIDTH(N)) :: LINE`, where WIDTH is a PURE module function. `boundFunction`, which already
took the intrinsic inquiries in bounds (section 29), takes such a function too; lowering needed
nothing, since the bounds are computed at entry already. One of the standard's rules was left
out on purpose: a specification function may not be recursive. Under `/RECURSIVE` every unit is
recursive, and a module's text says so, so the rule would have made a program legal or not by a
compiling option. The corpus found that: the same program failed only in its `/RECURSIVE` run.

### ELEMENTAL: a call the scalarizer reaches into

`CLAMP` is written for scalars and called with arrays:

```fortran
      t = clamp(t, 0.0, 60.0)
```

means `T(K) = CLAMP(T(K), 0.0, 60.0)` for every K. The obvious lowering is a temporary: call
CLAMP once per element into an array of results, then assign. govax doesn't make one. Section
27's scalarizer already turns an array expression into a loop nest with the expression's
elements at its leaves, and the elemental *intrinsics* (`SQRT(A)`) are not leaves: the
scalarizer reaches into their arguments, so `SQRT(A(K))` is computed at each element. An
elemental reference is the same: `funcRef` makes a `FuncRef` with `Elemental` set and the
arrays' shape (the dump's `CLAMP(T[], 0.0, 60.0)(8)`), and `Scalarize` reaches into it as into
SQRT. The call is in the loop:

```text
ROD.$25:
	PUSHAL	ROD.$K3
	PUSHAL	ROD.$K2
	MOVL	ROD.$I3,R0
	PUSHAL	ROD.T-4[R0]
	CALLS	#3,G^HEAT.CLAMP
	MOVL	ROD.$I3,R1
	MOVF	R0,ROD.T-4[R1]
```

That the target is also an argument is no problem: overlap (section 27) sees the arrays the
call reads through its arguments (`Leaves`), and here T(K) is read and written at the same
element, so the loop is safe in any order. `R(2:5) = CLIP(R(1:4), ...)` would run down instead.
An elemental subroutine's CALL gets a shape too, `Call.Shape`, and lowering's `elementalCall`
(MVBITS's, from section 33) calls it once per element in array element order.

Two parts of the language had to decide between an elemental procedure and a specific one.
A generic name may have both a specific for vectors and an elemental specific for scalars; the
standard prefers an exact fit, so `resolveGeneric` tries every specific exactly first, and only
then elemental ones element by element: a vector picks the vector specific, a scalar or a matrix
the elemental one. And an elemental *defined assignment*, `CELLS = 'X'` through an elemental
subroutine, is a call per element whose value is, by the standard's rule, computed whole before
the target changes; `elementalAssignment` gives the call the array assignment's order
(`Call.Order`), so `maskedCall` either runs it in a safe direction or computes the value into a
temporary first. Inside a WHERE the same call is masked.

### FORALL: an index set, not a loop

A FORALL looks like a DO loop and isn't one. `FORALL (I = 2:N-1) V(I) = (V(I-1) + V(I+1)) / 2`
means: compute the right side for every I, *then* assign them all. The standard says the
values may be computed in any order, even at once; what it guarantees is that no assignment is
seen by any evaluation. A DO loop guarantees the opposite.

So the first question about each FORALL is whether it can be done as a loop anyway. It can if
nothing the value or the mask reads is the target's storage, except at the target's own element:
then doing the tuples in turn gives the same answer. That is the case for ROD's second FORALL,
`forall (i = 1:n) t(i) = bar(i)%t`, which reads BAR and writes T, and its code is one loop:

```text
ROD.$11:				; FORALL (line 34), index I's body
	MULL3	ROD.$I2,#2,R0
	MOVL	ROD.$I2,R1
	MOVF	ROD.BAR-8[R0],ROD.T-4[R1]
	INCL	ROD.$I2
	DECL	ROD.$C4			; one pass fewer
	BLEQ	ROD.$12
	BRW	ROD.$11
```

The first FORALL reads `BAR(I-1)%T` and `BAR(I+1)%T` and writes `BAR(I)%T`, so it can't, and
the dump says so:

```text
30            FORALL (I = 2:7, (.NOT.:L BAR[I].FIXED:+4))
31              BAR[I].T:+0 = MEAN(BAR[(-:I I 1)].T:+0, BAR[(+:I I 1)].T:+0)  [in temporaries]
32            END FORALL
```

`sema/forall.go`'s `forallOrder` decides with `readsElsewhere`: a reference with the same text as
the target (by `DumpExpr`) is the target's own element and is passed over; any other reference
to the target's storage is a conflict; and a function's arguments are looked at by the same rule,
unless the function could reach the target some other way. Section 27 asked a finer question,
whether a section's elements run up or down, so a loop in the right direction would do; FORALL's
subscripts are any expressions of the indexes, so govax asks only the coarse one, and a FORALL
that fails it goes through temporaries.

Through temporaries, lowering (`ir/lowerforall.go`) makes two loop nests. The first computes the
mask for every tuple into a LOGICAL temporary and, where it holds, the value into a temporary of
the target's type (`$W2` and `$W1`, six elements each, since I runs from 2 to 7); the second
stores each kept value where its mask held:

```text
B3:  ; FORALL (line 30), index I's body
  assign $I1 = add.L($I2, 1)
  assign $W2-4[$I2] = not.L(BAR-4[mul.L($I1, 2)])
  branch $W2-4[$I2] B4 B5
B4:
  call HEAT.MEAN(ref BAR-16[mul.L($I1, 2)], ref BAR[mul.L($I1, 2)]) -> $W1-4[$I2]
  jump B5
  ...
B7:  ; FORALL (line 30), index I's body
  assign $I1 = add.L($I2, 1)
  branch $W2-4[$I2] B8 B9
B8:
  assign BAR-8[mul.L($I1, 2)] = $W1-4[$I2]
```

Each nest counts from 1 (`$I2`) and computes the index from the count (`$I1`, the FORALL's I),
because the temporaries are indexed by position in the index set, whatever the bounds and
stride. Notice MEAN is called only where the mask holds: the standard evaluates the value only
for the active tuples, which matters if it would fault elsewhere.

The FORALL's index I is not ROD's variable I. It is a symbol of the statement's own scope
(`newInnerScope`, as a statement function's dummies are), which takes from the unit's I only its
type; ROD's I is left alone, and its `$LOCAL` cell is never touched. The bounds are read in the
statement's scope too, so a bound that uses an index finds it and is refused (FORALLBOUND). And
MEAN must be PURE, or the statement is FORALLPURE:

```text
%FORT-E-FORALLPURE, CUBE isn't declared PURE, so it can't be referenced in a FORALL
```

**The construct** holds several statements, which are done one after another, each for every
tuple before the next starts. Each statement is judged on its own, as a step: in turn if it can
be, else through temporaries (`sema/forallorder.go`'s `finishForall`, which judges the outermost
FORALL once, with every level's indexes). A nested FORALL adds indexes, and its bounds may use an
outer one, so `FORALL (J = 1:I)` inside `FORALL (I = 1:N)` walks a triangle; lowering makes it a
level inside the outer level's loops (`forallLevel`). A mask that a later statement of the
construct may change is computed for the whole index set first (`MaskTemp`), since the standard
evaluates the mask once, at the construct's start. What doesn't fit these rules is NOTIMPL by
name (the manual's 14.13 lists the forms): the cases are rare, and each needs a temporary of a
shape the code doesn't yet build.

### WHERE inside WHERE: a chain of masks

Fortran 90's WHERE has one mask and its complement. Fortran 95's may nest, and its ELSEWHERE may
have a mask of its own, so a statement deep inside is controlled by several masks together. In
ROD, `t = t - 1.0` runs where `T > 20.0` held and `T > 60.0` didn't. The standard is precise
about *when* each mask is computed: a nested mask is evaluated only where the masks around it
hold, and a masked ELSEWHERE's mask only after the block before it has run, so it sees what that
block assigned. That rules out computing every mask at the start.

`ir/lowerwhere.go`'s `generalWhere` gives each statement its control mask as a *chain*, a list
of mask temporaries each tested in turn, possibly negated. Each mask is computed into its
temporary when its WHERE or ELSEWHERE is reached, under the chain that controls it:

```text
B16:  ; array (line 38), dimension 1's body
  branch $W3-4[$I1] B17 B18
B17:
  assign $W4-4[$I1] = gt.F(T-4[$I1], 60.0)
  ...
B26:  ; array (line 41), dimension 1's body
  branch $W3-4[$I4] B27 B30
B27:
  branch $W4-4[$I4] B29 B28
B28:
  assign T-4[$I4] = sub.F(T-4[$I4], 1.0)
```

`$W3` is `T > 20.0`, `$W4` is `T > 60.0`, computed only where `$W3` holds; line 41 runs where
`$W3` holds and `$W4` doesn't. The masked ELSEWHERE's `$W5`, `T < 20.0`, is computed after the
nested WHERE has run, where `$W3` doesn't hold. The program could have divided by something the
outer mask tested for zero, and nothing would fault. A WHERE with one mask, Fortran 90's form,
is lowered as before, and the optimizer's fusion (section 35) still applies only to those.

### The rest: intrinsics and FTNRTL

The intrinsics' changes are small, and each went where its kind already lived. MAXLOC and MINLOC
with DIM= are reductions like MAXVAL's (section 27), so `maxloc(t(2:), dim=1)` is one loop with
two accumulators, the place found so far and the value there:

```text
B42:  ; array (line 47), dimension 1's body
  assign $T3 = T[$I6]
  branch eq.L($T1, 0) B44 B43
B43:
  branch gt.F($T3, $T2) B44 B45
B44:
  assign $T2 = $T3
  assign $T1 = $I6
```

`$T1` starts at 0, "nothing seen", and a later element replaces the best only if strictly
greater, so the first of equal values wins, as the standard says. CEILING's and FLOOR's KIND= are
the existing KIND= machinery of INT and NINT (section 25). CPU_TIME is an intrinsic subroutine
like DATE_AND_TIME, calling FTNRTL's `FTN$CPU_TIME_F` (or `_D`, `_G`, `_H` by the argument's
format), which asks `$GETJPIW` for the process's CPU time in 10 ms ticks.

FTNRTL's two changes are both under the F90 flag of section 34, since VAX FORTRAN gave their
texts meanings. A zero width, `I0` or `F0.1` (ROD's prints use `F0.1`), writes a value in the
fewest columns that hold it; the output paths already counted a value's columns before comparing
them with the width, so `I0` is that count taken as the width (`ftn_edit.mar`'s `ZERO_WIDTH`),
and `F0.d` lays the number out as if the width were the record's room left (`ftn_float.mar`'s
`L_ZERO`), then puts only its own columns. And a `!` in namelist input starts a comment: the one
routine that passes over blanks for list-directed and namelist input, `FTN$$LI_SKIP`, passes over
the rest of the record after one, in a namelist READ with the flag. Without the flag, `I0` is
still a field too narrow for anything (OUTCONERR) and `!` still SYNERRNAM, and
`testdata/fortran/zerovax.for` holds them there.

### What the tools say

`/STANDARD=F90` judges by Fortran 90 with Fortran 95's additions (there is no F95 keyword), so
it flags none of them, and `/STANDARD=F77` flags each as an extension. Its obsolescent list is
now Fortran 95's, and the features Fortran 95 deleted get a message of their own, still only
informational, since govax compiles them:

```text
%FORT-I-DELETED, deleted in Fortran 95: the PAUSE statement
```

Section 35 described both. A cross reference says PURE or ELEMENTAL after a procedure's type,
and gives each FORALL index a row of its own (`I  INTEGER FORALL index`), so a reader can tell
the FORALL's I from the unit's. The debugger needed nothing: a FORALL index has no storage of its
own (it lives in a pooled `$In` cell, absent from the `/DEBUG` table), so EXAMINE of I shows
ROD's variable, which the FORALL leaves alone; that is what the program means, if not always what
its reader expects.

### What the programs found

Each subtask added a program to `testdata/fortran90` with its output worked by hand from the
standard and the HP manual, and gfortran `-std=f95` printed the same: `zerosize`, `nulls`,
`defaults`, `pures`, `elementals`, `foralls`, `forallc`, `wheres`, `intrins`, and `zerowid`
(with `zerowidvms`, govax's own choices, and the VAX side, `testdata/fortran/zerovax.for`). Then
`modules/colony`, Conway's Life in three files, used all of them together: cells with default
values, an elemental defined assignment, an ELEMENTAL function of the whole board, PURE functions
in FORALL, a WHERE in a WHERE, MAXLOC with DIM=, and a NAMELIST with comments. It ran right the
first time in every mode, which says more about the subtasks' programs than about COLONY.

The bugs were in the old code, found by the new rules: the generic's pointer dummy (Phase 107),
SAVE of a derived type refused (Phase 105), `Elements() == 0` meaning two things (Phase 102), the
debugger showing one element of an empty array (it stepped before testing the bounds),
`/WARNINGS=NOGENERAL` holding back what `/STANDARD` asked for (Phase 110), an intrinsic
subroutine's CALL never judged by `/STANDARD` at all, and NULL mentioned nowhere in a cross
reference. Each is held by a test of its own. The pattern is a familiar one: a new
feature is a new way of reaching old code, and the old code's assumptions ("a shape with no
elements is unknown", "a generic's arguments are passed the ordinary way") were true only of the
ways it had been reached before.

What waits: the NOTIMPLs the manual's 14.13 lists (FORALL statements whose temporaries would need
a shape not yet built; a target whose subscripts read its own storage, `V(V(I)) = 0`); a few of
PURE's rules not checked (an EQUIVALENCE partner of a global, a global reached through a pointer
component); ELEMENTAL's rule on dummies in specification expressions; a STRUCTURE's initial
values in a module's text (Phase 113); and the VMS round of the new programs (Phase 113's probe
round). What later standards added, IMPURE among it, is not planned.
