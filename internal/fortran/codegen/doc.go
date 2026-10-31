// Package codegen is the back end of govax's FORTRAN compiler, its last
// pass: it turns the intermediate representation (package ir) into VAX
// MACRO-32 source text, which govax's MACRO command assembles
// (docs/PHASE-74 - FORTRAN plan.md, Decisions 3, 8, 10, and 11;
// docs/PHASE-75 - FORTRAN first light.md, Part B).
//
// A code generator has two jobs: choosing instructions, deciding which
// VAX instructions and addressing modes do each thing the program does;
// and writing the result as a file another program reads.
//
// # Choosing instructions
//
// The IR gives each routine's code as basic blocks of two kinds of
// statement, Assign and Call, over expression trees. The VAX makes
// trees easy to compile, because nearly every instruction takes any of
// its operands straight from memory, in any addressing mode, and has a
// three-operand form. So a tree's leaves (a variable, a constant, an
// array element) are named in place as operands, and only the values
// inside a tree need registers:
//
//	I = J*K + L        MULL3   T.J,T.K,R0
//	                   ADDL3   R0,T.L,T.I
//
// The last operation writes straight to its target, and when the target
// is one of its operands the two-operand form does (I = I + J is ADDL2
// T.J,T.I), or INCL and DECL for adding and subtracting 1.
//
// Registers. Interior values go in R0 to R11, the lowest free one first
// (expr.go); a D or G value takes two in a row and an H value four, which
// an instruction names by the first (MULD3 R0,R2,R4 is R0:R1 times R2:R3
// into R4:R5). The order operands are computed in decides how many
// registers a tree needs, and the rule for the least is Sethi and
// Ullman's: compute first the operand that needs more, since its value
// is then held while the other is computed (need counts them, by width). An
// expression needing more than a budget has its biggest parts computed
// first into spill cells, $X1 and on, in the routine's storage (fit), so
// the registers never run out. The calling standard lets a routine use
// R0 and R1 freely but says it must preserve R2 to R11, so a routine
// that uses R2 and up names them in its entry mask, and CALLS saves them.
// No value is ever held in a register across a call: lowering took the
// calls out of expressions.
//
// Places. A variable is its symbol (relative addressing, which MACRO
// chooses for a symbol); a dummy argument is @n(AP), the address in the
// argument list; an array element is indexed mode, base[Rx], whose
// address is the base's plus Rx times the element's size: the IR's
// constant offset is part of the base (T.V-4[R0] for V(I) of V(1:10)),
// and for a dummy, whose @n(AP) has no room for one, part of the index.
//
// Constants. An integer is an immediate operand, #n, which MACRO makes a
// short literal when it fits. A REAL constant is its bits in $PDATA (a
// longword for REAL*4, two for REAL*8, four for REAL*16), since MACRO
// would read #n in a floating instruction as a floating value to convert
// itself, and Decision 16
// has the compiler's own bits used: folded with package vaxfloat, they
// round as the CPU does.
//
// Branches (cond.go). A comparison is a CMP (or a TST, against zero)
// and a branch on its condition; a LOGICAL is tested by its bit 0 with
// BLBS and BLBC, bit 0 being its value (appendix C.4 of the language
// manual); .NOT., .AND., and .OR. in a condition branch as they go,
// skipping the second operand when the first decides (section 2.3.4
// allows it). The VAX's conditional branches reach only 127 bytes, and
// nothing here knows how long the code between is, so a branch to a
// block is always the opposite condition's byte branch around a BRW,
// whose word displacement reaches 32K:
//
//	TSTL    T.I               (I .GT. 0?)
//	BLEQ    T.$2              (no: go on)
//	BRW     T.L10             (yes: to label 10)
//	T.$2:
//
// A comparison of CHARACTER values is a CMPC3 or CMPC5 (chars.go), which
// compares bytes unsigned, so its branches are the unsigned ones (BLSSU,
// BGTRU, ...); an assignment of one is a MOVC3 or MOVC5, filling with
// blanks. Both leave R0 to R5 or R0 to R3 changed, which the entry mask
// saves.
//
// A place at an address computed when the program runs (a CHARACTER dummy's
// text, at the address in its descriptor) is reached through that address
// in memory when it can be, by a deferred mode (@4(R0), the place whose
// address is 4 bytes past R0's), and otherwise with the address in a
// register (3(R2)[R3]).
//
// A block the one before falls into needs no branch at all. Phase 84's
// optimizer can shorten what this writes; the point here is code that
// is always right. One small thing is done now: a TST of the value the
// instruction just before wrote is left out, since that instruction set
// the condition codes (DECL count, then BLEQ).
//
// # Writing the file
//
// Writer lays out each line of MACRO in the columns MACRO programmers
// use (a label, then the opcode at the first tab stop, the operands at
// the next, and a comment further right), keeps every line within
// MACRO's 132-character limit, and copies the FORTRAN source into the
// file as comments. Generate writes the module: the descriptor codes it
// uses, then for each routine a storage map (where each variable is, and
// its MACRO symbol), its static storage in $LOCAL, its read-only data
// in $PDATA (constants, descriptors, I/O control blocks), and its code
// in $CODE, each source line just before the instructions it became
// (source.go), comments and declarations where they are in the source.
// The generated file is meant to be read: someone tracing a program's
// behavior, or a traceback that names a line of the .MAR file, should
// find the FORTRAN it came from right there.
//
// Names (names.go) follow Decision 10: a routine's entry is its
// FORTRAN name, a global symbol, and everything it owns is named for
// it, routine.name: T.I, T.L10 (FORTRAN label 10), T.$C1 (a DO loop's
// count), T.$2 (a label of the compiler's own). MACRO allows a period
// in a symbol and FORTRAN names can't hold one, so these never collide.
//
// A routine with ENTRY statements has one more .ENTRY for each, after its
// code, each copying its arguments' addresses to the cells the body uses
// and branching to its statement (writeEntry).
//
// The tests compare golden files (testdata/*.mar, rewritten with
// -update) and assemble every file they generate with govax's MACRO,
// which must say nothing; the console's TestFortranCodeRuns links a
// compiled program and runs it.
package codegen
