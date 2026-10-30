// Package ast is the FORTRAN compiler's syntax tree: the Go types the
// parser (internal/fortran/parse) builds from each statement's tokens,
// and that semantic analysis (internal/fortran/sema) reads (docs/PHASE-75
// - FORTRAN first light.md, Part B, "The AST").
//
// # What a syntax tree is
//
// The lexer hands the parser a flat list of tokens for each statement:
//
//	DO 10 I = 1, N-1      DO: 10 I = 1 , N - 1
//
// The parser recognizes the statement's grammar and builds a tree that
// says what each piece is: a DoStmt whose Target is label 10, whose Var
// is the name I, whose Start is the constant 1, and whose Limit is a
// Binary node, the subtraction of 1 from N. Later passes never look at
// tokens again: they walk the tree.
//
// The tree is plain syntax. It records what the program says, not what
// it means: a Name has no type yet, and A(I) is an Apply, which may be
// an element of an array A or a call of a function A. Only the
// declarations can tell those apart, and the declarations are semantic
// analysis's to read, so the parser doesn't try.
//
// # A file, its units, and their statements
//
// A File is a list of program units (a main program, a FUNCTION, or a
// SUBROUTINE), and each Unit is its header statement and a list of the
// statements after it, END last. The list is flat: an IF block's
// statements and a DO loop's body are simply the statements between
// their first and last, in order, as they are in the source. Semantic
// analysis matches each IF with its END IF and each DO with its terminal
// statement, so that the rules for blocks and labels (which jumps may go
// where) are in one place, and the parser stays a function per kind of
// statement.
//
// Every statement has its label (0 for none), its position (where its
// keyword or its first token is, for messages), and the source statement
// it came from, which says which source lines it occupies. Every
// expression has its position too.
//
// A statement the compiler recognizes but can't compile yet becomes a
// NotImplStmt, and one with a syntax error a BadStmt, after the parser
// has reported it. Both keep the statement's label, so that a jump to it
// isn't reported a second time as a jump to a label that doesn't exist.
//
// # The dump
//
// Dump writes a File one line per statement, the form the parser's
// golden tests compare. Each line has the statement's line number and
// label, then the statement with its expressions as prefix lists, an
// operator first, so that the tree's shape shows:
//
//	== main program SQUARES
//	1           PROGRAM SQUARES
//	2           INTEGER I
//	3           DO 10 I = 1, 5
//	4           PRINT * I (* I I)
//	5     10    CONTINUE
//	6           END
//
// X = -A + B*C is dumped as X = (+ (- A) (* B C)), and an array element
// or function reference A(I+1) as A((+ I 1)). A parenthesized expression
// keeps its parentheses as (paren X), since they matter: a parenthesized
// variable passed as an argument is a copy of it, and parentheses fix
// the order in which operations are done.
package ast
