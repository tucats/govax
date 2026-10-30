# Inside govax's FORTRAN

This is a guided tour of govax's FORTRAN compiler and its run-time
library, FTNRTL, for someone who wants to know how they work, or to
write a compiler of their own for the VAX under VMS. It grows a phase at
a time with the compiler (`docs/PHASE-74 - FORTRAN plan.md` lays out
the program; its Part B holds the decisions this tour refers to). The
user's side is the manual's chapter 14, "Programming in FORTRAN".

So far (Phase 74) the tour covers the groundwork: the compiler's
driver, its messages, its first pass (source forms), the MACRO file it
writes, and FTNRTL's build and how programs find it. The passes that
turn statements into code arrive from Phase 75 on.

## 1. The shape of the compiler

A compiler is a series of passes, each reading what the one before it
produced and handing on something a step closer to machine code:

```
 SQUARES.FOR
     |
 source ---------- lines into statements: columns, continuations,
     |             comments, INCLUDE                     (Phase 74)
 lex ------------- statements into tokens                (Phase 75)
 parse ----------- tokens into a syntax tree             (Phase 75)
 sema ------------ names, types, storage, checks         (Phases 75-76)
 ir -------------- the tree lowered to basic blocks      (Phase 75)
 opt ------------- optional improvements                 (Phase 84)
 codegen --------- VAX instructions, as MACRO-32 text    (Phases 74-75)
     |
 SQUARES.MAR --MACRO--> SQUARES.OBJ --LINK (FTNRTL)--> SQUARES.EXE
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

## 2. The driver: `internal/fortran`

`fortran.Compile(Options, File) Result` runs the passes in order. Its
`Options` are the command's qualifiers (and the version, time, and
command line for the MACRO file's header); `File` is the source's
specification, its name, and its lines; `Result` holds the MACRO file's
lines (nil after an error) and the messages. The driver also names the
MACRO module: for now after the source file (`ModuleName`), later after
the first program unit.

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

## 5. The MACRO file: `codegen.Writer`

The code generator has two jobs: choosing instructions, which Phase
75's subtask 6 began (its chapter comes with the phase's close-out), and
writing text another program reads. `Writer` lays out each MACRO line in the
columns MACRO programmers use (opcode at 8, operands at 16, comment at
40), keeps every line within MACRO's 132 characters (a comment that
won't fit moves to a line of its own, a long one is split), and writes
the FORTRAN source as comments. A source line's prefix (`;`, the number
in five columns, two blanks) is exactly 8 characters, one tab stop, so
tab-format source keeps its layout; an included line's prefix starts
with its include depth (`;1  12  `).

## 6. FTNRTL: the run-time library (`internal/ftnrtl`)

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
