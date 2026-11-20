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
// constructs open at it (its path): for each, the IF block's or CASE
// construct's current block (arm), or the DO loop. A jump from statement R
// to statement T is into a block when T's path holds an arm R's doesn't
// (section 5.7.3.1: control may leave a block, never enter one, nor pass
// from one block of a construct to another).
//
// The ends of constructs are Fortran 90's (the standard's 8.2, and the
// author's answer 4 in docs/planning/PHASE-101 - Fortran 90 control
// constructs.md, under both /F90 settings): a jump to END IF may come from
// any block of its IF and from outside it, so END IF is outside its IF
// (VAX FORTRAN's 5.7.3 allowed only the last block); a jump to END SELECT
// may come from any block of its construct, but not from outside it, so
// END SELECT is in its construct in a frame any block matches (anyArm).
// A CASE statement is no target at all (jumpTarget), nor are ELSE IF and
// ELSE.
//
// A jump into a DO loop from outside it gets no message: section 5.3.1.4
// allows one from the loop's extended range (code the loop jumped out
// to, jumping back), which only running the program can tell from a
// mistake. A terminal statement that nested loops share is the
// exception: only the innermost loop may jump to it (section 5.3.1.3).

// frame is one construct in a statement's path: an IF block's arm, a CASE
// construct's block (sel set), or a DO loop (do set). anyArm is set for
// END SELECT's frame, which a statement in any block of the construct is
// in.
type frame struct {
	id     int
	arm    *Arm
	do     bool
	sel    bool
	where  bool
	anyArm bool
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
	a.noteLabel(label, s.Base().Source.LabelPosition(), RefDefined)
}

// noteEndIf records, if s is END IF and the label is its own, the IF block it ends, so
// that a jump from outside the block can be found (Fortran 90's B.2.5 makes one
// obsolescent).
func (a *analyzer) noteEndIf(label int, s ast.Stmt) {
	l := a.unit.Labels[label]
	if _, ok := s.(*ast.EndIfStmt); !ok || l == nil || l.Stmt != s {
		return
	}

	for i := len(a.open) - 1; i >= 0; i-- {
		if a.open[i].isIf() {
			l.endIf = a.open[i].id

			return
		}
	}
}

// checkLabels checks every jump in the unit against the label it goes
// to.
func (a *analyzer) checkLabels() {
	for _, r := range a.refs {
		l := a.unit.Labels[r.label]

		var into *frame
		if l != nil {
			into = outside(r.path, l)
		}

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

		case into != nil:
			what := "an IF block"
			switch {
			case into.sel:
				what = "a SELECT CASE construct"
			case into.where:
				what = "a WHERE construct"
			}

			a.report("INTOBLOCK", r.pos, r.label, what)

		case len(l.loops) > 1 && !inPath(r.path, frame{id: l.loops[0], do: true}):
			a.report("SHAREDTERM", r.pos, r.label)

		case l.endIf != 0 && !r.assign && !inConstruct(r.path, l.endIf):
			a.extensions.obsolete(r.pos, "a jump to END IF from outside its IF block")
		}
	}
}

// jumpTarget reports whether s is a statement a jump may go to: an
// executable statement, but not ELSE IF or ELSE (section 5.7.3), CASE (the
// standard's 8.2), nor a statement function's definition.
func (a *analyzer) jumpTarget(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.ElseIfStmt, *ast.ElseStmt, *ast.CaseStmt:
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

// outside is the first block l is in that a jump from a statement at path
// isn't, which the jump would go into; nil if there is none.
func outside(path []frame, l *Label) *frame {
	for i, f := range l.path {
		if !f.do && !inPath(path, f) {
			return &l.path[i]
		}
	}

	return nil
}

// inPath reports whether f is in path: the same construct and, for an
// IF block or a CASE construct, the same arm (any arm for anyArm).
func inPath(path []frame, f frame) bool {
	for _, p := range path {
		if p.id == f.id && (f.anyArm || p.arm == f.arm) {
			return true
		}
	}

	return false
}

// inConstruct reports whether path is in the construct id, in any of its blocks.
func inConstruct(path []frame, id int) bool {
	for _, p := range path {
		if p.id == id {
			return true
		}
	}

	return false
}

// labelPath is where a jump to statement s's label goes, given path, the
// constructs open at s: path itself, but for END IF (outside its IF) and
// END SELECT (in its construct, in no one block).
func (a *analyzer) labelPath(s ast.Stmt, path []frame) []frame {
	var is func(*construct) bool

	switch s.(type) {
	case *ast.EndIfStmt:
		is = (*construct).isIf
	case *ast.EndSelectStmt:
		is = (*construct).isSelect
	default:
		return path
	}

	for i := len(a.open) - 1; i >= 0; i-- {
		if !is(a.open[i]) {
			continue
		}

		if a.open[i].isIf() {
			return path[:i]
		}

		end := append([]frame(nil), path[:i+1]...)
		end[i].arm, end[i].anyArm = nil, true

		return end
	}

	return path
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
