package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Dump writes the program as the golden tests compare it. For each unit:
// a line naming it; its symbols, each with its type, class, dimensions,
// and storage; then its statements, one a line with the line number and
// label first, indented by the blocks they are in. Expressions are
// prefix lists whose operators carry their type's letter, (+:R X Y),
// with each conversion explicit, (REAL I); an array element's subscripts
// are in brackets, A[I], and a whole array is A[].
//
//	== main program SQUARES
//	  SQUARES                 program unit
//	  I         INTEGER       variable         $LOCAL+0
//	  $LOCAL is 4 bytes
//	3           DO 10 I = 1, 5, 1
//	4             PRINT * I (*:I I I)
//	5     10      CONTINUE
//	6           END
func Dump(p *Program) string {
	var b strings.Builder

	for _, u := range p.Units {
		fmt.Fprintf(&b, "== %s %s\n", u.Kind, u.Name)

		for _, s := range u.Symbols {
			b.WriteString(strings.TrimRight("  "+dumpSymbol(s), " ") + "\n")
		}

		for _, e := range u.Entries {
			params := make([]string, len(e.Params))
			for i, s := range e.Params {
				params[i] = s.Name
			}

			fmt.Fprintf(&b, "  entry %s (%s)%s\n", e.Sym.Name, strings.Join(params, ", "), dumpAltPlaces(e.AltPlaces))
		}

		if u.AltPlaces != nil {
			fmt.Fprintf(&b, "  header%s\n", dumpAltPlaces(u.AltPlaces))
		}

		for _, s := range u.Symbols {
			if d := s.StmtFunc; d != nil {
				params := make([]string, len(d.Dummies))
				for i, p := range d.Dummies {
					params[i] = p.Name + " " + p.Type.String()
				}

				fmt.Fprintf(&b, "  statement function %s (%s) = %s\n", s.Name, strings.Join(params, ", "), DumpExpr(d.Value))
			}
		}

		fmt.Fprintf(&b, "  $LOCAL is %d bytes\n", u.LocalSize)

		for _, c := range u.Commons {
			fmt.Fprintf(&b, "  %s is %d bytes\n", c, c.Size)
		}

		for _, in := range u.Initials {
			b.WriteString(dumpInitial(in) + "\n")
		}

		dumpStmts(&b, u.Body, 0)
	}

	return b.String()
}

// dumpSymbol is a symbol's line.
func dumpSymbol(s *Symbol) string {
	class := s.Class.String()

	switch {
	case s.Result:
		class = "result"
	case s.Class == UnitName:
		class = "program unit"
	case s.Class == Array:
		class = "array(" + dims(s.Dims) + ")"
	default:
		class = strings.TrimPrefix(strings.TrimPrefix(class, "a "), "an ")
	}

	typ := s.Type.String()
	if s.Class == Subroutine || s.Class == UnitName {
		typ = ""
	}

	where := ""

	switch {
	case s.Class == Constant && s.Value != nil:
		where = "= " + dumpConst(s.Value)
	case s.Dummy != 0:
		where = "argument " + strconv.Itoa(s.Dummy)
	case s.Common != nil:
		where = s.Common.Psect + "+" + strconv.Itoa(s.Offset)
	case s.Offset >= 0:
		where = "$LOCAL+" + strconv.Itoa(s.Offset)
	}

	return fmt.Sprintf("%-9s %-13s %-16s %s", s.Name, typ, class, where)
}

// dims is an array's dimensions as declared: 1:10, 0:9, 1:*, and an
// adjustable array's bounds as the syntax tree's dump writes them.
func dims(list []Dim) string {
	text := make([]string, len(list))

	for i, d := range list {
		lower, upper := strconv.Itoa(d.Lower), strconv.Itoa(d.Upper)

		if d.LowerExpr != nil {
			lower = ast.DumpExpr(d.LowerExpr)
		}

		switch {
		case d.Assumed:
			upper = "*"
		case d.UpperExpr != nil:
			upper = ast.DumpExpr(d.UpperExpr)
		}

		text[i] = lower + ":" + upper
	}

	return strings.Join(text, ", ")
}

// dumpStmts writes a list of statements at an indentation depth.
func dumpStmts(b *strings.Builder, list []Stmt, depth int) {
	for _, s := range list {
		switch s := s.(type) {
		case *If:
			if !s.Block {
				line(b, s.AST, depth, "IF "+DumpExpr(s.Arms[0].Cond)+" "+stmtText(s.Arms[0].Body))

				continue
			}

			for i, arm := range s.Arms {
				keyword := "IF "
				if i > 0 {
					keyword = "ELSE IF "
				}

				line(b, arm.AST, depth, keyword+DumpExpr(arm.Cond)+" THEN")
				dumpStmts(b, arm.Body, depth+1)
			}

			if s.Else != nil {
				line(b, s.Else.AST, depth, "ELSE")
				dumpStmts(b, s.Else.Body, depth+1)
			}

			if s.EndIf != nil {
				line(b, s.EndIf, depth, "END IF")
			}

		case *Do:
			line(b, s.AST, depth, fmt.Sprintf("DO %s%s = %s, %s, %s", loopLabel(s.Label), symName(s.Var),
				DumpExpr(s.Start), DumpExpr(s.Limit), DumpExpr(s.Step)))
			dumpStmts(b, s.Body, depth+1)
			dumpEndDo(b, s.EndDo, depth)

		case *DoWhile:
			line(b, s.AST, depth, "DO "+loopLabel(s.Label)+"WHILE "+DumpExpr(s.Cond))
			dumpStmts(b, s.Body, depth+1)
			dumpEndDo(b, s.EndDo, depth)

		default:
			line(b, s.Source(), depth, stmtText([]Stmt{s}))
		}
	}
}

// loopLabel is a DO statement's terminal label and a blank, or nothing
// for a loop END DO ends.
func loopLabel(label int) string {
	if label == 0 {
		return ""
	}

	return strconv.Itoa(label) + " "
}

// dumpEndDo writes the END DO line that ends a loop, if one does.
func dumpEndDo(b *strings.Builder, s ast.Stmt, depth int) {
	if s != nil {
		line(b, s, depth, "END DO")
	}
}

// symName is a symbol's name, or ? for none.
func symName(s *Symbol) string {
	if s == nil {
		return "?"
	}

	return s.Name
}

// line writes one statement's line: its line number, its label, and
// text indented by depth.
func line(b *strings.Builder, s ast.Stmt, depth int, text string) {
	label := ""
	if n := s.Base().Label; n != 0 {
		label = strconv.Itoa(n)
	}

	fmt.Fprintf(b, "%-5d %-5s %s%s\n", s.Position().Line, label, strings.Repeat("  ", depth), text)
}

// stmtText is a simple statement's text: the one statement in list, or
// nothing.
func stmtText(list []Stmt) string {
	if len(list) == 0 {
		return ""
	}

	switch s := list[0].(type) {
	case *Assign:
		return DumpExpr(s.Target) + " = " + DumpExpr(s.Value)
	case *Call:
		if s.Args == nil {
			return "CALL " + s.Sub.Name
		}

		text := "CALL " + s.Sub.Name + dumpArgs(s.Args)
		if s.AltReturns != nil {
			text += " or to (" + labelList(s.AltReturns) + ")"
		}

		return text
	case *Goto:
		return "GO TO " + strconv.Itoa(s.Label)
	case *ComputedGoto:
		return "GO TO (" + labelList(s.Labels) + ") " + DumpExpr(s.Index)
	case *AssignLabel:
		return "ASSIGN " + strconv.Itoa(s.Label) + " TO " + s.Var.Name
	case *AssignedGoto:
		if s.Labels == nil {
			return "GO TO " + s.Var.Name
		}

		return "GO TO " + s.Var.Name + " (" + labelList(s.Labels) + ")"
	case *ArithIf:
		return "IF (" + DumpExpr(s.Cond) + ") " + labelList(s.Labels[:])
	case *Continue:
		return "CONTINUE"
	case *Stop:
		verb := "STOP"
		if s.Pause {
			verb = "PAUSE"
		}

		if !s.HasDisplay {
			return verb
		}

		return verb + " " + quote(s.Display)
	case *EntryPoint:
		return "ENTRY " + s.Entry.Sym.Name
	case *Return:
		if s.Alt != nil {
			return "RETURN " + DumpExpr(s.Alt)
		}

		return "RETURN"
	case *End:
		return "END"
	case *Print:
		text := "PRINT *"
		if s.Write {
			text = "WRITE (*,*)"
		}

		for _, e := range s.Items {
			text += " " + DumpExpr(e)
		}

		return text
	}

	return fmt.Sprintf("%T", list[0])
}

// dumpAltPlaces shows where a dummy argument list has *s, counted from 0, or
// nothing if it has none.
func dumpAltPlaces(places []int) string {
	if places == nil {
		return ""
	}

	return " *s at places (" + labelList(places) + ")"
}

// labelList is labels separated by commas: 10, 20, 30.
func labelList(labels []int) string {
	text := make([]string, len(labels))
	for i, l := range labels {
		text[i] = strconv.Itoa(l)
	}

	return strings.Join(text, ", ")
}

// DumpExpr is an expression in the dump's form.
func DumpExpr(e Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *Const:
		return dumpConst(e)
	case *Var:
		return e.Sym.Name
	case *Elem:
		subs := make([]string, len(e.Subs))
		for i, s := range e.Subs {
			subs[i] = DumpExpr(s)
		}

		return e.Sym.Name + "[" + strings.Join(subs, ", ") + "]"
	case *ArrayRef:
		return e.Sym.Name + "[]"
	case *Substring:
		return DumpExpr(e.X) + "(" + DumpExpr(e.Lower) + ":" + DumpExpr(e.Upper) + ")"
	case *Concat:
		parts := make([]string, len(e.Parts))
		for i, x := range e.Parts {
			parts[i] = DumpExpr(x)
		}

		return "(//:" + e.T.letter() + " " + strings.Join(parts, " ") + ")"
	case *FuncRef:
		return e.Fn.Name + dumpArgs(e.Args)
	case *StmtFuncRef:
		args := make([]string, len(e.Args))
		for i, x := range e.Args {
			args[i] = DumpExpr(x)
		}

		return "{" + e.Def.Sym.Name + " " + strings.Join(args, " ") + "}"
	case *IntrinsicRef:
		args := make([]string, len(e.Args))
		for i, x := range e.Args {
			args[i] = DumpExpr(x)
		}

		// The whole type, not its letter: which length was selected matters.
		name := e.Name
		if e.Op == CallOp {
			name = e.Routine
		}

		return "(" + e.Op.String() + ":" + e.T.String() + " " + name + " " + strings.Join(args, " ") + ")"
	case *Procedure:
		if e.Dummy != nil {
			return "(procedure:" + e.T.String() + " " + e.Name + " dummy)"
		}

		return "(procedure:" + e.T.String() + " " + e.Name + " " + e.Routine + ")"
	case *Loc:
		return "(%LOC " + DumpExpr(e.X) + ")"
	case *Unary:
		return "(" + e.Op.String() + ":" + e.T.letter() + " " + DumpExpr(e.X) + ")"
	case *Binary:
		return "(" + e.Op.String() + ":" + e.T.letter() + " " + DumpExpr(e.X) + " " + DumpExpr(e.Y) + ")"
	case *Compare:
		return "(" + e.Op.String() + ":" + e.X.Type().letter() + " " + DumpExpr(e.X) + " " + DumpExpr(e.Y) + ")"
	case *Convert:
		return "(" + e.T.String() + " " + DumpExpr(e.X) + ")"
	case *bad:
		return "(error)"
	}

	return fmt.Sprintf("%T", e)
}

// dumpConst is a constant: 5, 1.5, .TRUE., 'TEXT', (1.0, -2.5). A REAL
// constant is shown with the fewest digits that give its value again in
// its format, and always with a period or an exponent, so that 1.0
// doesn't look like the integer 1: a REAL*8's exponent is D (2.5D0), a
// REAL*16's Q.
func dumpConst(c *Const) string {
	switch c.T.Kind {
	case Real:
		return dumpReal(c.Real, c.T)

	case Complex:
		return "(" + dumpReal(c.Real, c.T.part()) + ", " + dumpReal(c.Imag, c.T.part()) + ")"

	case Logical:
		switch c.Int {
		case -1:
			return ".TRUE."
		case 0:
			return ".FALSE."
		}

		return "(LOGICAL " + strconv.Itoa(int(c.Int)) + ")"

	case Character:
		return quote(c.Text)
	}

	return strconv.Itoa(int(c.Int))
}

// dumpReal is a value of REAL type t, as dumpConst shows it.
func dumpReal(v vaxfloat.Value, t Type) string {
	if t.Size == 4 {
		text := strings.ToUpper(strconv.FormatFloat(v.Float64(), 'g', -1, 32))
		if !strings.ContainsAny(text, ".E") {
			text += ".0"
		}

		return text
	}

	letter := "D"
	if t.Size == 16 {
		letter = "Q"
	}

	text := strings.ToUpper(v.Text('g', -1))
	if !strings.ContainsRune(text, '.') && !strings.ContainsRune(text, 'E') {
		text += ".0"
	}

	if i := strings.IndexByte(text, 'E'); i >= 0 {
		return text[:i] + letter + strings.TrimPrefix(text[i+1:], "+")
	}

	return text + letter + "0"
}

// quote is a character constant as FORTRAN writes it.
func quote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// dumpArgs is an argument list: (A, , (copy X), T[]).
func dumpArgs(args []Arg) string {
	text := make([]string, len(args))

	for i, arg := range args {
		text[i] = DumpExpr(arg.Expr)
		if arg.Mode != ByDefault {
			text[i] = "(" + arg.Mode.String() + " " + text[i] + ")"
		}

		if arg.Copy {
			text[i] = "(copy " + text[i] + ")"
		}
	}

	return "(" + strings.Join(text, ", ") + ")"
}
