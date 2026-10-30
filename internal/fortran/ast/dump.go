package ast

import (
	"fmt"
	"strconv"
	"strings"
)

// Dump writes the file one line per statement (see doc.go): before each
// unit a line naming it, then each statement with its line number and
// label, its expressions as prefix lists.
func Dump(f *File) string {
	var b strings.Builder

	for _, u := range f.Units {
		name := u.Name()
		if name == "" {
			name = "(no PROGRAM statement)"
		}

		fmt.Fprintf(&b, "== %s %s\n", u.Kind(), name)

		if u.Header != nil {
			b.WriteString(dumpLine(u.Header) + "\n")
		}

		for _, s := range u.Body {
			b.WriteString(dumpLine(s) + "\n")
		}
	}

	return b.String()
}

// dumpLine is one statement's line: its line number, its label, and the
// statement.
func dumpLine(s Stmt) string {
	label := ""
	if n := s.Base().Label; n != 0 {
		label = strconv.Itoa(n)
	}

	return strings.TrimRight(fmt.Sprintf("%-5d %-5s %s", s.Position().Line, label, DumpStmt(s)), " ")
}

// DumpStmt is one statement in the dump's form, without its line number
// and label.
func DumpStmt(s Stmt) string {
	switch s := s.(type) {
	case *ProgramStmt:
		return "PROGRAM " + s.Name

	case *FunctionStmt:
		text := "FUNCTION " + s.Name + names(s.Params)
		if s.Type != nil {
			text = s.Type.String() + " " + text
		}

		return text

	case *SubroutineStmt:
		return "SUBROUTINE " + s.Name + names(s.Params)

	case *TypeStmt:
		return s.Type.String() + " " + declarators(s.Entities)

	case *DimensionStmt:
		return "DIMENSION " + declarators(s.Entities)

	case *AssignStmt:
		return DumpExpr(s.Target) + " = " + DumpExpr(s.Value)

	case *CallStmt:
		if s.Args == nil {
			return "CALL " + s.Name
		}

		return "CALL " + s.Name + args(s.Args)

	case *GotoStmt:
		return "GO TO " + strconv.Itoa(s.Target.Label)

	case *IfStmt:
		return "IF " + DumpExpr(s.Cond) + " " + DumpStmt(s.Then)

	case *IfThenStmt:
		return "IF " + DumpExpr(s.Cond) + " THEN"

	case *ElseIfStmt:
		return "ELSE IF " + DumpExpr(s.Cond) + " THEN"

	case *ElseStmt:
		return "ELSE"

	case *EndIfStmt:
		return "END IF"

	case *DoStmt:
		text := fmt.Sprintf("DO %d %s = %s, %s", s.Target.Label, s.Var.Name, DumpExpr(s.Start), DumpExpr(s.Limit))
		if s.Step != nil {
			text += ", " + DumpExpr(s.Step)
		}

		return text

	case *ContinueStmt:
		return "CONTINUE"

	case *StopStmt:
		if s.Display == nil {
			return "STOP"
		}

		return "STOP " + DumpExpr(s.Display)

	case *ReturnStmt:
		if s.Alt == nil {
			return "RETURN"
		}

		return "RETURN " + DumpExpr(s.Alt)

	case *EndStmt:
		return "END"

	case *PrintStmt:
		return "PRINT " + s.Format.String() + items(s.Items)

	case *WriteStmt:
		control := s.Unit.String()
		if s.Format != nil {
			control += ", " + s.Format.String()
		}

		return "WRITE (" + control + ")" + items(s.Items)

	case *NotImplStmt:
		return "not implemented: " + s.What

	case *BadStmt:
		return "error: " + s.Kind.String()
	}

	return fmt.Sprintf("%T", s)
}

// DumpExpr is an expression in the dump's form: a leaf as written, an
// operation as a prefix list, (op X Y).
func DumpExpr(e Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *IntConst:
		return e.Text
	case *RealConst:
		return e.Text
	case *LogicalConst:
		if e.Value {
			return ".TRUE."
		}

		return ".FALSE."
	case *CharConst:
		return "'" + strings.ReplaceAll(e.Value, "'", "''") + "'"
	case *Name:
		return e.Name
	case *Apply:
		return e.Name + args(e.Args)
	case *Unary:
		return "(" + e.Op.String() + " " + DumpExpr(e.X) + ")"
	case *Binary:
		return "(" + e.Op.String() + " " + DumpExpr(e.X) + " " + DumpExpr(e.Y) + ")"
	case *Paren:
		return "(paren " + DumpExpr(e.X) + ")"
	}

	return fmt.Sprintf("%T", e)
}

// String is the type as written: INTEGER, REAL*8.
func (t TypeSpec) String() string {
	if t.Length == 0 {
		return t.Type.String()
	}

	return t.Type.String() + "*" + strconv.Itoa(t.Length)
}

// String is the format as written: *, a label, or the expression.
func (f FormatSpec) String() string {
	switch {
	case f.Star:
		return "*"
	case f.Label != nil:
		return strconv.Itoa(f.Label.Label)
	}

	return DumpExpr(f.Expr)
}

// String is the unit as written: * or the expression.
func (u UnitSpec) String() string {
	if u.Star {
		return "*"
	}

	return DumpExpr(u.Expr)
}

// String is the declarator as written, its bounds' expressions in the
// dump's form: IVEC*4(0:9, 3).
func (d *Declarator) String() string {
	text := d.Name
	if d.Length != 0 {
		text += "*" + strconv.Itoa(d.Length)
	}

	if d.Dims == nil {
		return text
	}

	bounds := make([]string, len(d.Dims))

	for i, b := range d.Dims {
		upper := DumpExpr(b.Upper)
		if b.Star {
			upper = "*"
		}

		if b.Lower != nil {
			upper = DumpExpr(b.Lower) + ":" + upper
		}

		bounds[i] = upper
	}

	return text + "(" + strings.Join(bounds, ", ") + ")"
}

// names is a parameter list, (A, B), or nothing for none.
func names(params []*Name) string {
	if params == nil {
		return ""
	}

	list := make([]string, len(params))
	for i, p := range params {
		list[i] = p.Name
	}

	return "(" + strings.Join(list, ", ") + ")"
}

// declarators is a declaration's list of entities.
func declarators(ds []*Declarator) string {
	list := make([]string, len(ds))
	for i, d := range ds {
		list[i] = d.String()
	}

	return strings.Join(list, ", ")
}

// args is an argument or subscript list, (A, , (+ B 1)); a null argument
// shows as nothing between its commas.
func args(list []Expr) string {
	text := make([]string, len(list))
	for i, e := range list {
		text[i] = DumpExpr(e)
	}

	return "(" + strings.Join(text, ", ") + ")"
}

// items is an I/O list, each item after a blank.
func items(list []Expr) string {
	var b strings.Builder

	for _, e := range list {
		b.WriteString(" " + DumpExpr(e))
	}

	return b.String()
}
