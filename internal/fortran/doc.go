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
// diag holds every pass's error and warning messages, and ast the syntax
// tree's node types, which parse builds and sema reads.
//
// The passes are built a phase at a time (docs/PHASE-74's Part C). Every
// pass now exists, for Phase 75's subset of the language: Compile reads
// the source and its INCLUDE files into statements, reporting any problem
// with their layout, lexes and parses each statement, and analyzes each
// program unit, reporting what each pass finds wrong (a statement the
// compiler can't compile yet among them); a program without errors is
// lowered to the IR, and the code generator writes it as a MACRO file,
// each source line a comment before the instructions it became.
//
// The compiler is plain Go. It knows nothing of govax's console, RMS, or
// the emulated machine: the FORTRAN command (internal/dclcmd/fortran.go)
// reads the source, from the host or a Files-11 volume, hands Compile a
// source.Opener that reads the files INCLUDE statements name, and writes
// the MACRO file Compile returns.
package fortran
