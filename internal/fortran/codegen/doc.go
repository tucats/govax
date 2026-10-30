// Package codegen is the back end of govax's FORTRAN compiler: it turns
// the compiler's intermediate representation into VAX MACRO-32 source
// text, which govax's MACRO command assembles (docs/PHASE-74 - FORTRAN
// plan.md, Decisions 3 and 8).
//
// A code generator has two jobs. The first is choosing instructions:
// deciding, for each operation the program does, which VAX instructions
// and addressing modes do it (ADDL3 to add two longwords into a third,
// indexed mode to reach an array element, and so on). Phase 75 begins
// that.
//
// The second, and the only one here so far, is writing the result as a
// text file another program reads. Writer lays out each line of MACRO in
// the columns MACRO programmers use (a label, then the opcode at the
// first tab stop, the operands at the next, and a comment further right),
// keeps every line within MACRO's 132-character limit, and copies the
// FORTRAN source into the file as comments, each line beside the code it
// produced. The generated file is meant to be read: someone tracing a
// program's behavior, or a traceback that names a line of the .MAR file,
// should find the FORTRAN it came from right there.
package codegen
