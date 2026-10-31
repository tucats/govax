package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
)

// The jumps of section 5.6 and 5.7.1 that go to one of several labels:
// the computed GO TO, the assigned GO TO and the ASSIGN that gives it
// its label, and the arithmetic IF. Each label is checked as a jump from
// the statement (labels.go), as an unconditional GO TO's is.

// computedGoto analyzes GO TO (s1, ..., sn) e (section 5.6.2): e is any
// arithmetic expression, converted to an integer as an assignment would
// (a REAL one truncated) to choose a label by its position.
func (a *analyzer) computedGoto(s *ast.ComputedGotoStmt, path []frame) Stmt {
	a.jumps(s.Targets, path)

	index := a.arithmetic(a.value(s.Index))
	if index.Type().Kind == Invalid {
		return nil
	}

	g := &ComputedGoto{stmtBase: stmtBase{s}, Index: a.convert(index, IntegerType)}
	for _, t := range s.Targets {
		g.Labels = append(g.Labels, t.Label)
	}

	return g
}

// assignLabel analyzes ASSIGN s TO v (section 3.5). v holds the address
// of the statement labeled s, which takes 32 bits: so it must be an
// INTEGER*4 variable, whatever the unit's default INTEGER is (the
// author's answer 1 in docs/PHASE-79). The label may be an executable
// statement's or a FORMAT statement's (checkLabels).
func (a *analyzer) assignLabel(s *ast.AssignLabelStmt) Stmt {
	a.refs = append(a.refs, labelRef{label: s.Target.Label, pos: s.Target.Pos, assign: true})

	v := a.labelVariable(s.Var)
	if v == nil {
		return nil
	}

	a.checkLoopVariable(v, s.Var.Pos)

	if !slices.Contains(a.unit.Assigned, s.Target.Label) {
		a.unit.Assigned = append(a.unit.Assigned, s.Target.Label)
	}

	return &AssignLabel{stmtBase{s}, s.Target.Label, v}
}

// assignedGoto analyzes GO TO v [[,] (s1, ..., sn)] (section 5.6.3): a
// jump to the address v holds. The list "does not affect statement
// execution": it names the labels the jump may go to, each checked as a
// jump from here. Without one, the jump may go to any label the unit
// ASSIGNs, which can't be checked: the ASSIGN may be meant for another
// GO TO.
func (a *analyzer) assignedGoto(s *ast.AssignedGotoStmt, path []frame) Stmt {
	a.jumps(s.Targets, path)

	v := a.labelVariable(s.Var)
	if v == nil {
		return nil
	}

	g := &AssignedGoto{stmtBase: stmtBase{s}, Var: v}
	for _, t := range s.Targets {
		g.Labels = append(g.Labels, t.Label)
	}

	return g
}

// labelVariable analyzes the variable of an ASSIGN or an assigned GO TO:
// an INTEGER*4 variable, not an array element (ASSIGNVAR).
func (a *analyzer) labelVariable(n *ast.Name) *Symbol {
	v, ok := a.name(n, false).(*Var)
	if !ok {
		// An array or a constant has its message.
		return nil
	}

	switch t := v.Sym.Type; {
	case t.Kind == Invalid:
		// No type (IMPLICIT NONE): NOTYPE says so.
		return nil
	case t != IntegerType:
		a.report("ASSIGNVAR", n.Pos, n.Name, t.String())

		return nil
	}

	return v.Sym
}

// arithIf analyzes IF (e) s1, s2, s3 (section 5.7.1): e is an INTEGER or
// REAL value of any length, whose sign chooses the label. A COMPLEX value
// has no sign, and a LOGICAL or CHARACTER one is no arithmetic
// expression (ARITHIF; the manual's "arithmetic expression", read
// strictly, ours).
func (a *analyzer) arithIf(s *ast.ArithIfStmt, path []frame) Stmt {
	a.jumps(s.Targets[:], path)

	cond := a.value(s.Cond)

	switch t := cond.Type(); t.Kind {
	case Invalid:
		return nil
	case Integer, Real:
	default:
		a.report("ARITHIF", cond.Position(), article(t.String()))

		return nil
	}

	return &ArithIf{stmtBase{s}, cond, [3]int{s.Targets[0].Label, s.Targets[1].Label, s.Targets[2].Label}}
}
