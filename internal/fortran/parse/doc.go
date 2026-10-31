// Package parse is the FORTRAN compiler's third pass, its parser: it
// reads each statement's tokens and builds the syntax tree of package ast
// (docs/PHASE-75 - FORTRAN first light.md, Part B, and Decision 6 of
// docs/PHASE-74 - FORTRAN plan.md).
//
// # A function per statement
//
// The lexer has already done the part of parsing that is peculiar to
// FORTRAN: it decided what kind of statement each one is. So the parser
// is simple. It looks the kind up in a table (parse.go's parsers) and
// calls that kind's function, which reads the tokens in the order its
// statement's grammar gives them. DO s [,] v = e1, e2 [, e3] becomes:
//
//	s.Target = p.labelRef()   // s
//	p.accept(",")             // [,]
//	s.Var = p.name(...)       // v
//	p.expect("=")             // =
//	s.Start = p.expr()        // e1
//	p.expect(",")
//	s.Limit = p.expr()        // e2
//	if p.accept(",") {        // [, e3]
//		s.Step = p.expr()
//	}
//	p.end()                   // and nothing more
//
// This style is recursive descent: each piece of the grammar is a
// function, which calls the functions for the pieces inside it. It is how
// most production compilers are written, because each function reads
// like the rule it implements, and an error message can say exactly what
// was expected where (p.expect("=") reports "expected '=', but found
// ','").
//
// Every statement of VAX FORTRAN has a function in the table (Phase 76),
// whether or not the compiler can compile it yet: the parser builds its
// node, and semantic analysis says "the OPEN statement is not
// implemented yet" (NOTIMPL) when it reaches one it has no meaning for.
// The files follow the manual's chapters: decl.go the headers and the
// declarations of types and shapes (type statements, DIMENSION,
// IMPLICIT, PARAMETER), spec.go the other specification statements and
// compilation control (COMMON, DATA, NAMELIST, STRUCTURE, OPTIONS, the
// CDEC$ directives, FORMAT), stmt.go assignment and control, and io.go
// input and output.
//
// # Control lists
//
// The I/O statements share one grammar, the control list of section
// 7.1.1: specifiers in parentheses, each keyword=value, except that the
// unit (first) and the format or namelist (second, after a unit without
// its keyword) may leave their keywords out, and a direct access record
// may follow the unit as u'r. io.go's controlList reads every
// statement's list, and a table says which keywords each statement takes
// and what follows each (an expression, a label for ERR= and END=, or
// nothing for OPEN's READONLY). The unit, format, namelist, and record
// are taken out of the list into the statement's own fields, so later
// passes needn't care how they were written.
//
// # Expressions: precedence climbing
//
// FORTRAN's operators bind in nine levels, from .EQV. (loosest) to **
// (tightest). Writing a function per level is the textbook way, but
// expr.go uses precedence climbing instead: one function, binary(min),
// that reads an operand and then any operators binding at least as
// tightly as min, each one's right operand read by a recursive call with
// a higher minimum. The table of operators and their levels is then the
// grammar of expressions, in one place. Two of FORTRAN's rules need care:
// ** groups right to left (2**3**2 is 2**9), and VAX FORTRAN lets a sign
// follow another operator, the sign taking the rest of the term:
// A**-B*C is A**(-(B*C)).
//
// The tree records parentheses (ast.Paren) rather than dropping them,
// because FORTRAN gives them meaning beyond grouping: CALL S((X)) passes
// a copy of X, which S can change without changing X.
//
// # Errors
//
// When a statement doesn't follow its grammar, the parser reports it
// (SYNTAX, saying what it expected and what it found; EXTRATEXT for text
// after a complete statement) and gives the statement up: the parsing
// function panics, and statement recovers, keeping a BadStmt in the tree
// with the statement's label. Then it goes on with the next statement, so
// one compilation reports every statement's error, as a programmer wants.
// FORTRAN makes recovery easy: a statement is never more than its own
// tokens, so an error in one can't confuse the parser about the next.
//
// # Program units and structures
//
// Parse also divides the file into program units. A PROGRAM, FUNCTION,
// SUBROUTINE, or BLOCK DATA statement starts a unit; so does any other
// statement when no unit is open, which begins a main program without a
// PROGRAM statement; END ends a unit. A unit with no END (the file ends,
// or another unit's header comes first) is NOEND. An OPTIONS statement
// or a directive between units starts none: it is the next unit's
// Prologue (OPTIONS must come before the header it applies to).
//
// Structure declarations are the one place statements nest (blocks.go):
// STRUCTURE ... END STRUCTURE holds field declarations, nested
// structures, and unions, a UNION holds MAPs, and a MAP holds fields.
// Parse keeps a stack of the declarations open and puts each statement
// into the innermost one's Body, so that INTEGER*2 YEAR inside a
// structure is a field, never a variable.
//
// The parser doesn't match IF with END IF, or a DO with the statement
// whose label it names: a unit's statements are a flat list, and
// semantic analysis builds the blocks, where the rules about jumping into
// and out of them are checked too.
//
// # An example
//
// For the program
//
//	      PROGRAM SQUARES
//	      INTEGER I
//	      DO 10 I = 1, 5
//	         PRINT *, I, I*I
//	   10 CONTINUE
//	      END
//
// Parse returns one unit, whose dump (ast.Dump) is
//
//	== main program SQUARES
//	1           PROGRAM SQUARES
//	2           INTEGER I
//	3           DO 10 I = 1, 5
//	4           PRINT * I (* I I)
//	5     10    CONTINUE
//	6           END
package parse
