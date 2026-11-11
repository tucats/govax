// Package fortran is govax's FORTRAN compiler: it compiles a VAX FORTRAN
// source file into a VAX MACRO-32 source file, which the MACRO command
// assembles and LINK links against FTNRTL, the FORTRAN run-time library
// (docs/PHASE-74 - FORTRAN plan.md).
//
//	$ FORTRAN SQUARES      ! SQUARES.FOR -> SQUARES.MAR
//	$ MACRO SQUARES        ! SQUARES.MAR -> SQUARES.OBJ
//	$ LINK SQUARES         ! SQUARES.OBJ -> SQUARES.EXE
//	$ RUN SQUARES
//
// # How a compiler is put together
//
// A compiler is a series of passes. Each reads what the one before it
// produced and hands on something a step closer to machine code. govax's
// compiler has these, each in a package of its own beneath this one, with
// a doc comment that explains its pass:
//
//   - source: reads the source file's lines, which FORTRAN lays out in
//     columns (a label field, a continuation column, the statement), and
//     joins them into statements;
//   - lex: splits each statement into tokens (names, numbers, operators).
//     FORTRAN makes this harder than most languages do, because it
//     ignores blanks and has no reserved words;
//   - parse: recognizes each statement's grammar and builds a syntax tree;
//   - sema: semantic analysis, giving every name its type and storage and
//     checking the rules a grammar can't (labels, DO loops, arguments);
//   - ir: lowers the tree into a simpler intermediate representation of
//     basic blocks and expression trees;
//   - opt: improves the IR, optionally;
//   - codegen: chooses VAX instructions and writes the MACRO file.
//
// listing isn't a pass: it lays out what /LIST asks for from what the passes that
// ran have left (docs/PHASE-85 - FORTRAN compiler usability.md).
//
// diag holds every pass's error and warning messages, and ast the syntax
// tree's node types, which parse builds and sema reads.
//
// The passes were built a phase at a time, Phases 74 to 89, and each phase
// doc's progress log says what it added. Compile (compile.go) runs them in
// order: it reads the source and its INCLUDE files into statements,
// reporting any problem with their layout; lexes and parses each
// statement; and analyzes each program unit, reporting what each pass
// finds wrong. A program without errors is lowered to the IR, improved by
// the optimizer under /OPTIMIZE, and written by the code generator as a
// MACRO file, each source line a comment before the instructions it
// became.
//
// The compiler is plain Go. It knows nothing of govax's console, RMS, or
// the emulated machine: the FORTRAN command (internal/dclcmd/fortran.go)
// reads the source, from the host or a Files-11 volume, hands Compile a
// source.Opener that reads the files INCLUDE statements name, and writes
// the MACRO file Compile returns. That makes every pass testable with
// nothing but strings: most tests compile a few lines of FORTRAN held in
// a Go slice and compare what comes out with a "golden" file kept in the
// package's testdata directory (go test's -update flag rewrites them).
//
// # Where to start reading
//
// docs/FORTRAN-INTERNALS.md is a guided tour that follows one small
// program through every pass; read its first two sections before the
// code. Then read the passes in order, starting with each package's
// doc.go, and keep this file's glossary at hand. A good first exercise is
// to compile a three-line program with `govax fortran` and read the .MAR
// file it writes beside its source: every FORTRAN line is there as a
// comment above the VAX instructions it became.
//
// Comments cite their sources briefly. "Section 5.3.1" (or a table,
// figure, or appendix) is the 1988 VAX FORTRAN Language Reference Manual,
// the language's definition. "Decision 10" or "subtask 4" is a numbered
// item in the phase doc of the phase that wrote the code (docs/PHASE-nn -
// topic.md), where the choice and its reasons are written down.
// "Unconfirmed" or "ours" marks a rule neither the manual nor a run on a
// real VAX settled, so govax chose. "VMS's" behavior means what a real VAX
// running VMS 7.1 was seen to do, in the probes under testdata.

//
// # A glossary
//
// The compiler's comments use the words of three subjects at once:
// FORTRAN, compilers, and the VAX. These are the ones that come up most.
//
// FORTRAN:
//
//   - Program unit: a main program, a SUBROUTINE, a FUNCTION, or a BLOCK
//     DATA subprogram; one source file may hold several, and each becomes
//     one routine in the MACRO file.
//   - Fixed form: FORTRAN's card layout. Columns 1-5 hold a statement
//     label, column 6 marks a continuation of the line before, columns
//     7-72 hold the statement, and a C or * in column 1 makes a comment.
//   - Statement label: a number of up to five digits naming a statement,
//     the target of GO TO, DO, and ERR=, and the name of a FORMAT.
//   - Blanks don't count: outside quoted text FORTRAN ignores spaces, so
//     GO TO 10 and GOTO10 are the same statement. FORTRAN also has no
//     reserved words, so a variable may be named IF or DO. Together these
//     are why its lexer (package lex) is unusual.
//   - Implicit typing: a name never declared is INTEGER if it starts with
//     I to N and REAL otherwise, unless an IMPLICIT statement says
//     different.
//   - Dummy argument: a name in a SUBROUTINE or FUNCTION statement's list,
//     standing for whatever the caller passes (the actual argument).
//     FORTRAN passes arguments by reference: the routine is given the
//     argument's address, so assigning to a dummy changes the caller's
//     variable.
//   - COMMON block: storage shared by name between program units, like a
//     global struct whose fields each unit may name differently.
//   - EQUIVALENCE: two or more names made to share the same storage, like
//     a C union.
//   - Hollerith constant: 5HHELLO, the old way of writing text: a count,
//     an H, and that many characters.
//   - Intrinsic function: a function the language defines (SQRT, ABS,
//     MOD), which the compiler may compute in line or call in a library.
//   - Statement function: a one-line function defined inside a unit,
//     F(X) = X*X + 1, expanded where it is used.
//   - Unit (in I/O): a number naming a file or the terminal, as in
//     WRITE (6, 100); not to be confused with a program unit.
//
// Compilers:
//
//   - Pass: one stage of the compiler, reading the previous stage's
//     output and producing the next's.
//   - Token: one word of a statement (a name, a number, an operator), the
//     lexer's output.
//   - Syntax tree (AST): the parser's output, a tree of Go structs whose
//     shape is the statement's grammar (package ast).
//   - Semantic analysis: deciding what the tree means: each name's type
//     and storage, which name is an array or a function, which constant
//     expressions can be computed now. Its result is a symbol table per
//     unit (sema.Symbol) and a typed tree.
//   - Constant folding: computing an expression whose operands are all
//     constants at compile time, so 2*3 becomes 6 in the program.
//   - Intermediate representation (IR): a form between the tree and the
//     machine (package ir), simple enough for an optimizer to reason about.
//   - Lowering: turning a high-level construct (a DO loop, an array
//     element) into the simpler steps of the IR.
//   - Basic block: a run of statements that always execute together, one
//     after another, ending in a terminator: a jump, a branch, or a
//     return. Nothing jumps into the middle of one.
//   - Temporary: storage the compiler invents to hold a value between two
//     steps, named with a $ ($T1) so it can never clash with a FORTRAN name.
//   - Liveness: whether a value stored now may still be read later; a store
//     nothing reads is "dead" and the optimizer may remove it.
//   - Register allocation: choosing which values live in the CPU's
//     registers rather than memory.
//
// The VAX and VMS:
//
//   - MACRO-32: the VAX's assembly language, the compiler's output. One
//     line is one instruction (MOVL A,B copies A to B) or a directive to
//     the assembler (.LONG 5 reserves a longword holding 5).
//   - Byte, word, longword, quadword, octaword: 1, 2, 4, 8, and 16 bytes.
//     An instruction's last letter says its size: MOVB, MOVW, MOVL, MOVQ.
//   - F, D, G, and H floating: the VAX's floating point formats; F is
//     REAL*4, D or G (/G_FLOATING) REAL*8, and H REAL*16. They are not
//     IEEE 754, so the compiler folds constants with package vaxfloat.
//   - Registers: R0 to R11 for values, then AP (the argument pointer, the
//     address of a call's argument list), FP (the frame pointer, the
//     current call's frame on the stack), SP (the stack pointer), and PC.
//   - Calling standard: CALLS pushes the arguments and calls a routine,
//     which finds them through AP (4(AP) is the first argument's
//     longword) and returns a function's value in R0 (and R1 for 8 bytes).
//     A routine's entry mask lists the registers from R2 up it changes,
//     which CALLS saves and RET restores.
//   - Program section (psect): a named region of the image the linker
//     gathers. The compiler writes three per routine's module: $CODE (the
//     instructions), $LOCAL (static variables, writable), and $PDATA
//     (constants and other read-only data). FORTRAN's variables are static
//     by default, in $LOCAL, not on the stack.
//   - Cell: the IR's word for a variable or temporary at a place in
//     $LOCAL, the stack frame, or a register; a datum is a read-only piece
//     of $PDATA.
//   - Descriptor: a small block (length, data type, class, address) that
//     tells a called routine where a string or array is and how long it is.
//     CHARACTER arguments are always passed by descriptor.
//   - Condition and signal: VMS's exceptions. A routine signals a
//     condition value (an error code), and handlers established by the
//     routines that called it may handle it.
//   - Shareable image: VMS's shared library. FTNRTL.EXE (internal/ftnrtl)
//     is FORTRAN's run-time library, which does all I/O; MTHRTL
//     (internal/mthrtl) has the math functions; LIBRTL (internal/librtl)
//     the general ones.
package fortran
