package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Statement labels: each is defined once, and a jump may go only to an
// executable statement's label, and never into a block from outside it.
//
// Where a statement is, as far as jumps are concerned, is the list of
// constructs open at it (its path): for each, the IF block's arm or the
// DO loop. A jump from statement R to statement T is into an IF block
// when T's path holds an arm R's doesn't (section 5.7.3.1: control may
// leave a block, never enter one, nor pass from one block of a
// construct to another). END IF counts as in the block just before it,
// so a jump to its label may come only from there (section 5.7.3).
//
// A jump into a DO loop from outside it gets no message: section 5.3.1.4
// allows one from the loop's extended range (code the loop jumped out
// to, jumping back), which only running the program can tell from a
// mistake. A terminal statement that nested loops share is the
// exception: only the innermost loop may jump to it (section 5.3.1.3).

// frame is one construct in a statement's path: an IF block's arm, or a
// DO loop (do set).
type frame struct {
	id  int
	arm *Arm
	do  bool
}

// labelRef is a reference to a label: a jump (GO TO's in every form, an
// arithmetic IF's, an I/O statement's ERR= and END=); with assign set, an
// ASSIGN's, which jumps nowhere itself and may name a FORMAT statement
// too; or with format set, an I/O statement's format, which must be a
// FORMAT statement's.
type labelRef struct {
	label  int
	pos    diag.Position
	path   []frame
	assign bool
	format bool
}

// jumps records a jump to each of labels from a statement at path.
func (a *analyzer) jumps(labels []ast.LabelRef, path []frame) {
	for _, l := range labels {
		a.refs = append(a.refs, labelRef{label: l.Label, pos: l.Pos, path: path})
	}
}

// defineLabel records label, on statement s at path, unless it is
// already defined (DUPLABEL).
func (a *analyzer) defineLabel(label int, s ast.Stmt, path []frame) {
	if l := a.unit.Labels[label]; l != nil {
		a.report("DUPLABEL", s.Base().Source.LabelPosition(), label, l.Stmt.Position().Line)

		return
	}

	a.unit.Labels[label] = &Label{Number: label, Stmt: s, path: path}
}

// checkLabels checks every jump in the unit against the label it goes
// to.
func (a *analyzer) checkLabels() {
	for _, r := range a.refs {
		l := a.unit.Labels[r.label]

		switch {
		case l == nil:
			a.report("UNDEFLBL", r.pos, r.label)

		case r.format && l.Stmt.Base().Kind != lex.Format:
			a.report("NOTFORMAT", r.pos, r.label, aStatement(l.Stmt))

		case r.format, r.assign && l.Stmt.Base().Kind == lex.Format:
			// A format, or a variable that will hold one (PRINT v).

		case r.assign && a.jumpTarget(l.Stmt):
			// Where the label is matters only to the GO TO that uses it.

		case !a.jumpTarget(l.Stmt):
			a.report("NOTEXEC", r.pos, r.label, aStatement(l.Stmt))

		case !inBlocks(r.path, l):
			a.report("INTOBLOCK", r.pos, r.label)

		case len(l.loops) > 1 && !inPath(r.path, frame{id: l.loops[0], do: true}):
			a.report("SHAREDTERM", r.pos, r.label)
		}
	}
}

// jumpTarget reports whether s is a statement a jump may go to: an
// executable statement, but not ELSE IF or ELSE (section 5.7.3) nor a
// statement function's definition.
func (a *analyzer) jumpTarget(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.ElseIfStmt, *ast.ElseStmt:
		return false
	}

	return ast.Executable(s) && !a.isStmtFunc(s)
}

// isStmtFunc reports whether s is a statement function's definition
// (statementFunction), with an error or without.
func (a *analyzer) isStmtFunc(s ast.Stmt) bool {
	_, ok := a.stmtFuncs[s]

	return ok
}

// inBlocks reports whether a jump from a statement at path stays out of
// any IF block l is in that the jump isn't.
func inBlocks(path []frame, l *Label) bool {
	for _, f := range l.path {
		if !f.do && !inPath(path, f) {
			return false
		}
	}

	return true
}

// inPath reports whether f is in path: the same construct and, for an
// IF block, the same arm.
func inPath(path []frame, f frame) bool {
	for _, p := range path {
		if p.id == f.id && p.arm == f.arm {
			return true
		}
	}

	return false
}

// statementName is what a statement is called in messages: GO TO, END
// IF, INTEGER, assignment: its lexer's kind's name.
func statementName(s ast.Stmt) string {
	return s.Base().Kind.String()
}

// aStatement is "a GO TO statement", "an END IF statement", "an
// assignment".
func aStatement(s ast.Stmt) string {
	name := statementName(s)

	if _, ok := s.(*ast.AssignStmt); !ok {
		name += " statement"
	}

	if name[0] == 'a' || name[0] == 'A' || name[0] == 'E' || name[0] == 'I' || name[0] == 'O' || name[0] == 'U' {
		return "an " + name
	}

	return "a " + name
}
