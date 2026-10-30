// Package source is the FORTRAN compiler's first pass: it reads a source
// file's lines and puts together the statements they hold
// (docs/PHASE-74 - FORTRAN plan.md, subtask 3).
//
// # Why FORTRAN needs a pass for this
//
// Most programming languages treat a source file as one long stream of
// characters, where a line break is just another blank. FORTRAN was
// designed for punched cards, one statement per card, and its source is
// still laid out in columns. The 1988 VAX FORTRAN Language Reference
// Manual's section 1.3 gives a line four fields:
//
//	columns  1-5    the statement label (a number other statements use
//	                to refer to this one: GO TO 10)
//	column   6      the continuation indicator: anything but a blank or
//	                a zero there makes the line a continuation of the
//	                statement on the lines before it
//	columns  7-72   the statement itself
//	columns 73-80   a sequence number, which the compiler ignores (cards
//	                were numbered so a dropped deck could be sorted)
//
// So "where does a statement start and end?" can't be answered by the
// lexer, which splits text into words: it is a question about columns.
// This pass answers it, and hands the lexer each statement's text as one
// string, with the label taken out and the continuation lines joined on.
//
// For example, these three lines (a dot is a blank, to show the columns):
//
//	C.....Sum.the.series
//	...10.TOTAL.=.TOTAL.+.X(I)*
//	.....1........WEIGHT(I)
//
// are a comment (a C in column 1), then one statement labeled 10, whose
// text is "TOTAL = TOTAL + X(I)*        WEIGHT(I)": column 6 of the third
// line holds a 1, so it continues the second. The blanks don't matter;
// FORTRAN ignores blanks outside character constants, and the lexer
// removes them.
//
// # The rules this pass applies
//
//   - Comment lines: a C, c, *, or ! in column 1, or a line of nothing
//     but blanks. They may come between a statement's lines. A ! anywhere
//     else in the statement field starts a comment that runs to the end
//     of the line, unless it is inside a character constant ('IT''S!')
//     or a Hollerith constant (5HHELLO!): comment.go keeps track of
//     those, across continuation lines too.
//   - Debugging lines: a D in column 1. With /D_LINES they are compiled,
//     the D read as a blank; without it they are comments.
//   - Tab-format lines (section 1.3.2): a tab in the label field ends it.
//     After the tab, a digit from 1 to 9 is the continuation indicator;
//     anything else starts the statement field. So "10<TAB>X = 1" is
//     labeled 10, and "<TAB>1+ Y" continues the statement before it.
//     The tab stands for the columns up to 6 (or 7), so the statement
//     field still ends at column 72.
//   - The statement field ends at column 72, or 132 with
//     /EXTEND_SOURCE; text past it is ignored without a message, as the
//     manual says. With /EXTEND_SOURCE there is no sequence field, and
//     text past column 132 is a fatal error.
//   - A statement may have at most /CONTINUATIONS=n continuation lines
//     (19 by default).
//   - Compiler directives (chapter 10): a line starting CDEC$ in columns
//     1 to 5, with a blank or tab in column 6, is a directive, never
//     continued. CPAR$ lines, the parallel directives, are comments:
//     govax has no /PARALLEL (Decision 1).
//   - INCLUDE 'file' (section 1.4.2) is handled here, not by the parser,
//     since it changes which file the lines come from: the included
//     file's statements take the INCLUDE statement's place.
//
// # What this pass hands on
//
// Read returns a Program: every line it read, in order, from the source
// and every included file (the code generator copies them into the MACRO
// file as comments), and the statements, each a Statement with its label,
// its text, and a map from each character of the text back to the file,
// line, and column it came from, so later passes can point at the exact
// place a problem is (Statement.Position).
package source
