package sema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
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
		if u.Recursive {
			fmt.Fprintf(&b, "== recursive %s %s\n", u.Kind, u.Name)
		} else {
			fmt.Fprintf(&b, "== %s %s\n", u.Kind, u.Name)
		}

		for _, s := range u.Symbols {
			b.WriteString(strings.TrimRight("  "+dumpSymbol(s), " ") + "\n")
		}

		for _, e := range u.Entries {
			params := make([]string, len(e.Params))
			for i, s := range e.Params {
				params[i] = s.Name
			}

			result := ""
			if e.Sym.Name != e.Name {
				result = " result " + e.Sym.Name
			}

			fmt.Fprintf(&b, "  entry %s (%s)%s%s\n", e.Name, strings.Join(params, ", "), dumpAltPlaces(e.AltPlaces), result)
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

		for _, st := range u.Structures {
			b.WriteString(dumpStructure(st))
		}

		fmt.Fprintf(&b, "  $LOCAL is %d bytes", u.LocalSize)

		if u.LocalAlign != 0 {
			fmt.Fprintf(&b, ", ALIGN=%d", u.LocalAlign)
		}

		b.WriteString("\n")

		if u.FrameSize != 0 {
			fmt.Fprintf(&b, "  the frame area is %d bytes\n", u.FrameSize)
		}

		for _, c := range u.Commons {
			fmt.Fprintf(&b, "  %s is %d bytes", c, c.Size)

			if c.Attrs != (PsectAttrs{Align: DefaultAlign}) {
				fmt.Fprintf(&b, ", %s", c.Attrs)
			}

			b.WriteString("\n")
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

	typ := s.TypeText()
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
	case s.Automatic:
		where = "frame+" + strconv.Itoa(s.Offset)
	case s.Offset >= 0:
		where = "$LOCAL+" + strconv.Itoa(s.Offset)
	}

	if s.Volatile {
		where += " volatile"
	}

	// Fortran 90's recorded attributes (attrtable.go), and SAVE given by a
	// Fortran 90 form or implied by an initialization, which no VAX FORTRAN
	// program has (an old SAVE statement's isn't shown, as it never was).
	if s.Intent != "" {
		where += " intent(" + strings.ToLower(s.Intent) + ")"
	}

	if s.Optional {
		where += " optional"
	}

	if s.Interface != nil {
		where += " interface(line " + strconv.Itoa(s.Interface.AST.Position().Line) + ")"
	}

	if s.Target {
		where += " target"
	}

	if s.Allocatable {
		where += " allocatable"
	}

	switch {
	case s.AutomaticObject && s.Dims == nil:
		where += " automatic-scalar"
	case s.AutomaticObject:
		where += " automatic-array"
	}

	if s.FreedAtReturn {
		where += " freed"
	}

	if s.saved && s.f90Saved {
		where += " saved"
	}

	return fmt.Sprintf("%-9s %-13s %-16s %s", s.Name, typ, class, where)
}

// dims is an array's dimensions as declared: 1:10, 0:9, 1:*, and an
// adjustable array's bounds as the syntax tree's dump writes them.
func dims(list []Dim) string {
	text := make([]string, len(list))

	for i, d := range list {
		if d.Deferred {
			text[i] = ":"

			continue
		}

		lower, upper := strconv.Itoa(d.Lower), strconv.Itoa(d.Upper)

		if d.LowerExpr != nil {
			lower = ast.DumpExpr(d.LowerExpr)
		}

		switch {
		case d.Assumed:
			upper = "*"
		case d.AssumedShape:
			upper = ""
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

		case *Select:
			line(b, s.AST, depth, "SELECT CASE "+DumpExpr(s.Expr))

			for _, c := range s.Cases {
				line(b, c.AST, depth, caseText(c))
				dumpStmts(b, c.Body, depth+1)
			}

			if s.EndSelect != nil {
				line(b, s.EndSelect, depth, "END SELECT")
			}

		case *Where:
			mask := "WHERE " + DumpExpr(s.Mask)
			if s.MaskTemp != nil {
				mask += "  [mask in a temporary]"
			}

			line(b, s.AST, depth, mask)
			dumpStmts(b, s.Then.Body, depth+1)

			for _, m := range s.Masked {
				line(b, m.AST, depth, "ELSEWHERE "+DumpExpr(m.Mask)+"  [mask in a temporary]")
				dumpStmts(b, m.Body, depth+1)
			}

			if s.Else != nil {
				line(b, s.Else.AST, depth, "ELSEWHERE")
				dumpStmts(b, s.Else.Body, depth+1)
			}

			if s.EndWhere != nil {
				line(b, s.EndWhere, depth, "END WHERE")
			}

		case *Forall:
			if !s.Construct() {
				line(b, s.AST, depth, forallText(s))

				break
			}

			header := forallHeaderText(s)
			if s.MaskTemp {
				header += "  [mask in a temporary]"
			}

			line(b, s.AST, depth, header)

			for i, st := range s.Body {
				if !s.Temporaries[i] {
					dumpStmts(b, []Stmt{st}, depth+1)

					continue
				}

				line(b, st.Source(), depth+1, stmtText([]Stmt{st})+"  [in temporaries]")
			}

			if s.EndForall != nil {
				line(b, s.EndForall, depth, "END FORALL")
			}

		case *Do:
			line(b, s.AST, depth, fmt.Sprintf("DO %s%s = %s, %s, %s", loopLabel(s.Label), symName(s.Var),
				DumpExpr(s.Start), DumpExpr(s.Limit), DumpExpr(s.Step)))
			dumpStmts(b, s.Body, depth+1)
			dumpEndDo(b, s.EndDo, depth)

		case *DoWhile:
			if s.Cond == nil {
				line(b, s.AST, depth, strings.TrimRight("DO "+loopLabel(s.Label), " "))
			} else {
				line(b, s.AST, depth, "DO "+loopLabel(s.Label)+"WHILE "+DumpExpr(s.Cond))
			}

			dumpStmts(b, s.Body, depth+1)
			dumpEndDo(b, s.EndDo, depth)

		default:
			line(b, s.Source(), depth, stmtText([]Stmt{s}))
		}
	}
}

// caseText is a CASE statement as the dump writes it: CASE DEFAULT, or
// CASE (ranges), each end its constant.
func caseText(c *Case) string {
	if c.Default {
		return "CASE DEFAULT"
	}

	ranges := make([]string, len(c.Ranges))

	for i, r := range c.Ranges {
		low, high := "", ""
		if r.Low != nil {
			low = DumpExpr(r.Low)
		}

		if r.High != nil {
			high = DumpExpr(r.High)
		}

		ranges[i] = ast.CaseRangeText(low, high, !r.Single())
	}

	return "CASE (" + strings.Join(ranges, ", ") + ")"
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

	// A Fortran 90 construct name goes before the statement that opens the
	// construct (OUTER: DO) and after the others (END DO OUTER).
	if name := s.Base().Construct; name != "" {
		switch s.Base().Kind {
		case lex.Do, lex.DoWhile, lex.IfThen, lex.SelectCase, lex.ForallBlock:
			text = name + ": " + text
		default:
			text += " " + name
		}
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
		return DumpExpr(s.Target) + " = " + DumpExpr(s.Value) + s.Order.String()
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
	case *Where:
		return "WHERE " + DumpExpr(s.Mask) + " " + stmtText(s.Then.Body)
	case *Forall:
		return forallText(s)
	case *PointerAssign:
		return DumpExpr(s.Pointer) + " => " + DumpExpr(s.Target)
	case *Nullify:
		parts := make([]string, len(s.Pointers))
		for i, p := range s.Pointers {
			parts[i] = DumpExpr(p)
		}

		return "NULLIFY (" + strings.Join(parts, ", ") + ")"
	case *Continue:
		return "CONTINUE"
	case *Allocate:
		verb := "ALLOCATE"
		if s.Deallocate {
			verb = "DEALLOCATE"
		}

		objects := make([]string, len(s.Objects))

		for i, o := range s.Objects {
			if o.Component != nil {
				objects[i] = DumpExpr(o.Component)

				continue
			}

			objects[i] = o.Sym.Name

			if o.Upper != nil {
				bounds := make([]string, len(o.Upper))
				for j := range o.Upper {
					bounds[j] = DumpExpr(o.Lower[j]) + ":" + DumpExpr(o.Upper[j])
				}

				objects[i] += "(" + strings.Join(bounds, ", ") + ")"
			}
		}

		if s.Stat != nil {
			objects = append(objects, "STAT="+DumpExpr(s.Stat))
		}

		return verb + " " + strings.Join(objects, ", ")
	case *LoopExit:
		verb := "EXIT"
		if s.Cycle {
			verb = "CYCLE"
		}

		return verb + " (loop of line " + strconv.Itoa(s.Loop.Source().Position().Line) + ")"
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
		return "ENTRY " + s.Entry.Name
	case *Return:
		if s.Alt != nil {
			return "RETURN " + DumpExpr(s.Alt)
		}

		return "RETURN"
	case *End:
		return "END"
	case *IO:
		return dumpIO(s)
	case *FileStmt:
		return dumpFile(s)
	case *IOLength:
		return "INQUIRE (IOLENGTH=" + DumpExpr(s.Var) + ") " + dumpItems(s.Items)
	case *DefineFile:
		files := make([]string, len(s.Files))
		for i, f := range s.Files {
			files[i] = DumpExpr(f.Unit) + "(" + DumpExpr(f.Records) + "," + DumpExpr(f.Length) + ",U," + DumpExpr(f.Var) + ")"
		}

		return "DEFINE FILE " + strings.Join(files, ",")
	}

	return fmt.Sprintf("%T", list[0])
}

// dumpFile shows OPEN, CLOSE, or INQUIRE: the unit, and each keyword as its code's
// name and value.
func dumpFile(s *FileStmt) string {
	var control []string

	if s.Unit != nil {
		control = append(control, DumpExpr(s.Unit))
	}

	if s.Rec != nil {
		control = append(control, "REC="+DumpExpr(s.Rec))
	}

	for _, k := range s.Keys {
		switch {
		case k.KeyList != nil:
			control = append(control, k.Name+"="+dumpKeyList(k.KeyList))
		case k.Known:
			control = append(control, k.Name+"="+strconv.Itoa(k.Number))
		case k.Value != nil:
			control = append(control, k.Name+"="+DumpExpr(k.Value))
		case k.Answer != nil:
			control = append(control, k.Name+"="+DumpExpr(k.Answer))
		default:
			control = append(control, k.Name+"="+DumpExpr(k.Text))
		}
	}

	if s.Err != 0 {
		control = append(control, "ERR="+strconv.Itoa(s.Err))
	}

	if s.IOStat != nil {
		control = append(control, "IOSTAT="+DumpExpr(s.IOStat))
	}

	return s.AST.Base().Kind.String() + " (" + strings.Join(control, ",") + ")"
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

// dumpSubs is an element's subscripts in the dump, [I, 2], or nothing
// for none.
func dumpSubs(subs []Expr) string {
	if subs == nil {
		return ""
	}

	text := make([]string, len(subs))
	for i, s := range subs {
		text[i] = DumpExpr(s)
	}

	return "[" + strings.Join(text, ", ") + "]"
}

// dumpSections is a section's subscripts in the dump, [1:N:2, J, <IV[]>].
func dumpSections(list []SectionSub) string {
	subs := make([]string, len(list))

	for i, s := range list {
		switch {
		case s.Vector != nil:
			subs[i] = "<" + DumpExpr(s.Vector) + ">"

			continue
		case !s.Triplet():
			subs[i] = DumpExpr(s.Index)

			continue
		}

		bound := func(x Expr) string {
			if x == nil {
				return "*"
			}

			return DumpExpr(x)
		}

		subs[i] = bound(s.Lower) + ":" + bound(s.Upper) + ":" + DumpExpr(s.Stride)
	}

	return "[" + strings.Join(subs, ", ") + "]"
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
		return e.Sym.Name + dumpSubs(e.Subs)
	case *FieldRef:
		// R[I].F:+12[J], each field with its offset in its structure; an
		// array reference's ranked part with its section subscripts,
		// PATH[1:10:1].X:+0.
		subs := func(k int, list []Expr) string {
			if e.S != nil && e.Ranked == k {
				return dumpSections(e.Sections)
			}

			return dumpSubs(list)
		}

		text := e.Sym.Name + subs(0, e.Subs)
		for i, s := range e.Path {
			text += "." + s.Field.Name + ":+" + strconv.Itoa(s.Field.Offset) + subs(i+1, s.Subs)
		}

		return text
	case *ArrayRef:
		return e.Sym.Name + "[]"
	case *StructValue:
		return dumpConstructor(e.T, e.Values)
	case *ArrayConst:
		// (/:F 1.0 2.0 3.0 /) with its shape, [2,2], if its rank isn't 1.
		parts := make([]string, len(e.Elems))
		for i, c := range e.Elems {
			parts[i] = DumpExpr(c)
		}

		shape := ""
		if len(e.S) != 1 {
			shape = e.S.String()
		}

		return "(/:" + e.T.letter() + shape + " " + strings.Join(parts, " ") + " /)"
	case *ArrayFunc:
		text := e.Name + "(" + DumpExpr(e.Array)
		if e.Other != nil {
			text += ", " + DumpExpr(e.Other)
		}

		if e.Dim > 0 {
			text += ", DIM=" + strconv.Itoa(e.Dim)
		}

		if e.DimExpr != nil {
			text += ", DIM=" + DumpExpr(e.DimExpr)
		}

		if e.Mask != nil {
			text += ", MASK=" + DumpExpr(e.Mask)
		}

		if e.Pad != nil {
			text += ", PAD=" + DumpExpr(e.Pad)
		}

		if e.Order != nil && !slices.IsSorted(e.Order) {
			text += ", ORDER=" + fmt.Sprint(e.Order)
		}

		return text + ")"
	case *ArrayFuncAt:
		names := make([]string, len(e.Index))
		for i, s := range e.Index {
			names[i] = s.Name
		}

		return DumpExpr(e.F) + "[" + strings.Join(names, ", ") + "]"
	case *Constructor:
		return "(/:" + e.T.letter() + " " + dumpCtorItems(e.Items) + " /)"
	case *Section:
		// A[1:N:2, J, <IV[]>], a bound taken from an adjustable array's own
		// bounds shown as *, a vector subscript in angle brackets.
		return e.Sym.Name + dumpSections(e.Subs)
	case *Substring:
		return DumpExpr(e.X) + "(" + DumpExpr(e.Lower) + ":" + DumpExpr(e.Upper) + ")"
	case *Concat:
		parts := make([]string, len(e.Parts))
		for i, x := range e.Parts {
			parts[i] = DumpExpr(x)
		}

		return "(//:" + e.T.letter() + " " + strings.Join(parts, " ") + ")"
	case *FuncRef:
		// An array value's shape after its arguments: F(X)(3).
		if len(e.S) > 0 {
			return e.Fn.Name + dumpArgs(e.Args) + e.S.String()
		}

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
		if e.Routine != "" {
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
	case *Present:
		return "(PRESENT " + e.Sym.Name + ")"
	case *Allocated:
		return "(ALLOCATED " + e.Sym.Name + ")"
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
	case Record:
		return dumpConstructor(c.T, c.Parts)

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

// dumpConstructor is a structure constructor's value, of the derived type t, as
// the dump shows it: POINT{1.0, Y}.
func dumpConstructor(t Type, values []Expr) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = DumpExpr(v)
	}

	return t.Struct.Name + "{" + strings.Join(parts, ", ") + "}"
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

// dumpArgs is an argument list: (A, , (copy X), T[], (sequence back A[1, 1:3:1])).
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

		// Its elements one after another, and copied back (sequence.go).
		switch {
		case arg.CopyBack:
			text[i] = "(sequence back " + text[i] + ")"
		case arg.Sequence:
			text[i] = "(sequence " + text[i] + ")"
		}
	}

	return "(" + strings.Join(text, ", ") + ")"
}

// dumpIO is an I/O statement's text: its verb, its control list as the
// analysis understood it, and its items. PRINT * and WRITE (*,*) are as
// written.
func dumpIO(s *IO) string {
	kind := s.AST.Base().Kind
	text := kind.String()

	var control []string

	switch {
	case s.Code:
		control = append(control, DumpExpr(s.Count))
	case s.Unit != nil && s.Internal:
		control = append(control, "internal "+DumpExpr(s.Unit))
	case s.Unit != nil:
		control = append(control, DumpExpr(s.Unit))
	case kind == lex.Read || kind == lex.Write:
		if _, ok := s.AST.(*ast.TransferStmt); ok {
			control = append(control, "*")
		}
	}

	switch {
	case s.Form == ListForm:
		control = append(control, "*")
	case s.Form == NamelistForm:
		control = append(control, "NML="+s.Group.Name)
	case s.Form == UnformattedForm:
		control = append(control, "unformatted")
	case s.Format != nil:
		control = append(control, strconv.Itoa(s.Format.Label))
	case s.FormatVar != nil:
		control = append(control, "assigned "+DumpExpr(s.FormatVar))
	case s.RunFormat != nil:
		control = append(control, "run-time "+DumpExpr(s.RunFormat))
	default:
		control = append(control, "?")
	}

	if s.Code {
		control = append(control, DumpExpr(s.Unit))
	}

	if s.Rec != nil {
		control = append(control, "REC="+DumpExpr(s.Rec))
	}

	if s.Err != 0 {
		control = append(control, "ERR="+strconv.Itoa(s.Err))
	}

	if s.End != 0 {
		control = append(control, "END="+strconv.Itoa(s.End))
	}

	if s.IOStat != nil {
		control = append(control, "IOSTAT="+DumpExpr(s.IOStat))
	}

	switch {
	case s.Advance != nil:
		control = append(control, "ADVANCE="+DumpExpr(s.Advance))
	case s.Nonadvancing:
		control = append(control, "ADVANCE='NO'")
	}

	if s.Size != nil {
		control = append(control, "SIZE="+DumpExpr(s.Size))
	}

	if s.EOR != 0 {
		control = append(control, "EOR="+strconv.Itoa(s.EOR))
	}

	if _, ok := s.AST.(*ast.PrintStmt); ok {
		text += " " + control[len(control)-1]
	} else {
		text += " (" + strings.Join(control, ",") + ")"
	}

	if items := dumpItems(s.Items); items != "" {
		text += " " + items
	}

	return text
}

// dumpItems is an I/O list's items, separated by blanks, an implied-DO
// list in parentheses.
func dumpItems(items []*Item) string {
	parts := make([]string, 0, len(items))

	for _, it := range items {
		if it.Loop == nil {
			parts = append(parts, DumpExpr(it.Value))

			continue
		}

		d := it.Loop
		parts = append(parts, "("+dumpItems(d.Items)+", "+symName(d.Var)+" = "+DumpExpr(d.Start)+", "+
			DumpExpr(d.Limit)+", "+DumpExpr(d.Step)+")")
	}

	return strings.Join(parts, " ")
}

// dumpKeyList is OPEN's KEY= list, each key's type and direction spelled out.
func dumpKeyList(list []*KeyDef) string {
	keys := make([]string, len(list))

	for i, k := range list {
		kind, direction := "CHARACTER", "ASCENDING"
		if k.Integer {
			kind = "INTEGER"
		}

		if k.Descending {
			direction = "DESCENDING"
		}

		keys[i] = DumpExpr(k.First) + ":" + DumpExpr(k.Last) + ":" + kind + ":" + direction
	}

	return "(" + strings.Join(keys, ", ") + ")"
}

// dumpCtorItems is an array constructor's items as the dump shows them, an
// implied DO as (items, I = start, limit, step).
func dumpCtorItems(items []CtorItem) string {
	parts := make([]string, len(items))

	for i, it := range items {
		if it.Do == nil {
			parts[i] = DumpExpr(it.Value)

			continue
		}

		parts[i] = "(" + dumpCtorItems(it.Do.Items) + ", " + it.Do.Var.Name + " = " + DumpExpr(it.Do.Start) + ", " +
			DumpExpr(it.Do.Limit) + ", " + DumpExpr(it.Do.Step) + ")"
	}

	return strings.Join(parts, " ")
}

// forallText is a FORALL statement as the dump shows it: its indexes, mask,
// and assignment, and "[in temporaries]" when its values are computed first.
func forallText(f *Forall) string {
	text := forallHeaderText(f) + " " + stmtText(f.Body)
	if f.Temporaries[0] {
		text += "  [in temporaries]"
	}

	return text
}

// forallHeaderText is FORALL (indexes, mask).
func forallHeaderText(f *Forall) string {
	parts := make([]string, 0, len(f.Indexes)+1)

	for _, x := range f.Indexes {
		p := x.Sym.Name + " = " + DumpExpr(x.Lower) + ":" + DumpExpr(x.Upper)
		if x.Stride != nil {
			p += ":" + DumpExpr(x.Stride)
		}

		parts = append(parts, p)
	}

	if f.Mask != nil {
		parts = append(parts, DumpExpr(f.Mask))
	}

	return "FORALL (" + strings.Join(parts, ", ") + ")"
}
