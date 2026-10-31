package ast

import (
	"fmt"
	"strconv"
	"strings"
)

// Dump writes the file one line per statement (see doc.go): before each
// unit a line naming it, then each statement with its line number and
// label, its expressions as prefix lists. A structure declaration's
// statements follow it, indented a level for each level of nesting.
func Dump(f *File) string {
	var b strings.Builder

	for _, u := range f.Units {
		name := u.Name()
		if name == "" {
			name = "(no name)"
			if u.Kind() == MainProgram {
				name = "(no PROGRAM statement)"
			}
		}

		for _, s := range u.Prologue {
			dumpLines(&b, s, "")
		}

		fmt.Fprintf(&b, "== %s %s\n", u.Kind(), name)

		if u.Header != nil {
			dumpLines(&b, u.Header, "")
		}

		for _, s := range u.Body {
			dumpLines(&b, s, "")
		}
	}

	if len(f.Trailing) > 0 {
		b.WriteString("== after the last unit\n")

		for _, s := range f.Trailing {
			dumpLines(&b, s, "")
		}
	}

	return b.String()
}

// dumpLines writes s's line, indented by indent, and the lines of any
// statements it holds, a level further in.
func dumpLines(b *strings.Builder, s Stmt, indent string) {
	b.WriteString(dumpLine(s, indent) + "\n")

	var body []Stmt

	switch s := s.(type) {
	case *StructureStmt:
		body = s.Body
	case *UnionStmt:
		body = s.Body
	case *MapStmt:
		body = s.Body
	}

	for _, inner := range body {
		dumpLines(b, inner, indent+"  ")
	}
}

// dumpLine is one statement's line: its line number, its label, and the
// statement.
func dumpLine(s Stmt, indent string) string {
	label := ""
	if n := s.Base().Label; n != 0 {
		label = strconv.Itoa(n)
	}

	return strings.TrimRight(fmt.Sprintf("%-5d %-5s %s%s", s.Position().Line, label, indent, DumpStmt(s)), " ")
}

// DumpStmt is one statement in the dump's form, without its line number
// and label (and without the statements a structure declaration holds).
func DumpStmt(s Stmt) string {
	if text, ok := dumpSpec(s); ok {
		return text
	}

	if text, ok := dumpIO(s); ok {
		return text
	}

	switch s := s.(type) {
	case *AssignStmt:
		return DumpExpr(s.Target) + " = " + DumpExpr(s.Value)

	case *AssignLabelStmt:
		return "ASSIGN " + strconv.Itoa(s.Target.Label) + " TO " + s.Var.Name

	case *CallStmt:
		if s.Args == nil {
			return "CALL " + s.Name
		}

		return "CALL " + s.Name + args(s.Args)

	case *GotoStmt:
		return "GO TO " + strconv.Itoa(s.Target.Label)

	case *ComputedGotoStmt:
		return "GO TO " + labels(s.Targets) + " " + DumpExpr(s.Index)

	case *AssignedGotoStmt:
		if s.Targets == nil {
			return "GO TO " + s.Var.Name
		}

		return "GO TO " + s.Var.Name + " " + labels(s.Targets)

	case *IfStmt:
		return "IF " + DumpExpr(s.Cond) + " " + DumpStmt(s.Then)

	case *ArithIfStmt:
		return "IF " + DumpExpr(s.Cond) + " " + strings.Trim(labels(s.Targets[:]), "()")

	case *IfThenStmt:
		return "IF " + DumpExpr(s.Cond) + " THEN"

	case *ElseIfStmt:
		return "ELSE IF " + DumpExpr(s.Cond) + " THEN"

	case *ElseStmt:
		return "ELSE"

	case *EndIfStmt:
		return "END IF"

	case *DoStmt:
		text := fmt.Sprintf("DO %s%s = %s, %s", doLabel(s.Target), s.Var.Name, DumpExpr(s.Start), DumpExpr(s.Limit))
		if s.Step != nil {
			text += ", " + DumpExpr(s.Step)
		}

		return text

	case *DoWhileStmt:
		return "DO " + doLabel(s.Target) + "WHILE " + DumpExpr(s.Cond)

	case *EndDoStmt:
		return "END DO"

	case *ContinueStmt:
		return "CONTINUE"

	case *StopStmt:
		if s.Display == nil {
			return s.Kind.String()
		}

		return s.Kind.String() + " " + DumpExpr(s.Display)

	case *ReturnStmt:
		if s.Alt == nil {
			return "RETURN"
		}

		return "RETURN " + DumpExpr(s.Alt)

	case *EndStmt:
		return "END"

	case *BadStmt:
		return "error: " + s.Kind.String()
	}

	return fmt.Sprintf("%T", s)
}

// dumpSpec is the dump of a header, a specification statement, or a
// compilation control statement, and true; or false for any other
// statement.
func dumpSpec(s Stmt) (string, bool) {
	switch s := s.(type) {
	case *ProgramStmt:
		return "PROGRAM " + s.Name, true

	case *FunctionStmt:
		text := "FUNCTION " + s.Name + names(s.Params)
		if s.Type != nil {
			text = s.Type.String() + " " + text
		}

		return text, true

	case *SubroutineStmt:
		return "SUBROUTINE " + s.Name + names(s.Params), true

	case *BlockDataStmt:
		return strings.TrimSpace("BLOCK DATA " + s.Name), true

	case *EntryStmt:
		return "ENTRY " + s.Name + names(s.Params), true

	case *TypeStmt:
		return s.Type.String() + " " + declarators(s.Entities), true

	case *DimensionStmt:
		return s.Kind.String() + " " + declarators(s.Entities), true

	case *ImplicitStmt:
		if s.None {
			return "IMPLICIT NONE", true
		}

		rules := make([]string, len(s.Rules))
		for i, r := range s.Rules {
			rules[i] = r.String()
		}

		return "IMPLICIT " + strings.Join(rules, ", "), true

	case *ParameterStmt:
		defs := make([]string, len(s.Defs))
		for i, d := range s.Defs {
			defs[i] = d.Name.Name + " = " + DumpExpr(d.Value)
		}

		if s.Old {
			return "PARAMETER " + strings.Join(defs, ", "), true
		}

		return "PARAMETER (" + strings.Join(defs, ", ") + ")", true

	case *CommonStmt:
		blocks := make([]string, len(s.Blocks))
		for i, c := range s.Blocks {
			blocks[i] = "/" + c.Name + "/ " + declarators(c.Entities)
		}

		return "COMMON " + strings.Join(blocks, " "), true

	case *EquivalenceStmt:
		sets := make([]string, len(s.Sets))
		for i, set := range s.Sets {
			sets[i] = args(set)
		}

		return "EQUIVALENCE " + strings.Join(sets, ", "), true

	case *DataStmt:
		sets := make([]string, len(s.Sets))
		for i, set := range s.Sets {
			sets[i] = exprList(set.Names) + " " + values(set.Values)
		}

		return "DATA " + strings.Join(sets, " "), true

	case *AttrStmt:
		items := make([]string, len(s.Items))
		for i, it := range s.Items {
			items[i] = it.Name
			if it.Common {
				items[i] = "/" + it.Name + "/"
			}
		}

		return strings.TrimSpace(s.Kind.String() + " " + strings.Join(items, ", ")), true

	case *NamelistStmt:
		groups := make([]string, len(s.Groups))
		for i, g := range s.Groups {
			groups[i] = "/" + g.Name + "/ " + strings.Trim(names(g.Items), "()")
		}

		return "NAMELIST " + strings.Join(groups, " "), true

	case *RecordStmt:
		groups := make([]string, len(s.Groups))
		for i, g := range s.Groups {
			groups[i] = "/" + g.Structure + "/ " + declarators(g.Entities)
		}

		return "RECORD " + strings.Join(groups, ", "), true

	case *StructureStmt:
		text := "STRUCTURE"
		if s.Name != "" {
			text += " /" + s.Name + "/"
		}

		if s.Fields != nil {
			text += " " + declarators(s.Fields)
		}

		return text, true

	case *UnionStmt:
		return "UNION", true

	case *MapStmt:
		return "MAP", true

	case *EndBlockStmt:
		return s.Kind.String(), true

	case *DictionaryStmt:
		return "DICTIONARY " + quoted(s.Path), true

	case *OptionsStmt:
		quals := make([]string, len(s.Qualifiers))
		for i, q := range s.Qualifiers {
			quals[i] = "/" + q.String()
		}

		return "OPTIONS " + strings.Join(quals, " "), true

	case *DirectiveStmt:
		text := "CDEC$ " + s.Name

		switch {
		case s.HasText:
			text += " " + quoted(s.Text)
		case s.Name == "PSECT":
			attrs := make([]string, len(s.Attrs))
			for i, a := range s.Attrs {
				attrs[i] = a.String()
			}

			text += " /" + s.Common + "/ " + strings.Join(attrs, ", ")
		}

		return text, true

	case *FormatStmt:
		text := "FORMAT " + s.Text
		for _, v := range s.VFEs {
			text += " <" + DumpExpr(v) + ">"
		}

		return text, true
	}

	return "", false
}

// dumpIO is the dump of an I/O statement, and true; or false for any
// other statement.
func dumpIO(s Stmt) (string, bool) {
	switch s := s.(type) {
	case *PrintStmt:
		return s.Kind.String() + " " + s.Format.String() + items(s.Items), true

	case *TransferStmt:
		control := []string{s.Unit.String()}

		switch {
		case s.Format != nil:
			control = append(control, s.Format.String())
		case s.Namelist != nil:
			control = append(control, "NML="+s.Namelist.Name)
		}

		if s.Rec != nil {
			control = append(control, "REC="+DumpExpr(s.Rec))
		}

		control = append(control, specs(s.Specs)...)

		return s.Kind.String() + " (" + strings.Join(control, ", ") + ")" + items(s.Items), true

	case *CodeStmt:
		control := append([]string{DumpExpr(s.Count), s.Format.String(), DumpExpr(s.Buffer)}, specs(s.Specs)...)

		return s.Kind.String() + " (" + strings.Join(control, ", ") + ")" + items(s.Items), true

	case *AuxIOStmt:
		var control []string

		if s.Unit != nil {
			control = append(control, s.Unit.String())
		}

		if s.Rec != nil {
			control = append(control, "REC="+DumpExpr(s.Rec))
		}

		control = append(control, specs(s.Specs)...)

		return s.Kind.String() + " (" + strings.Join(control, ", ") + ")", true

	case *DefineFileStmt:
		files := make([]string, len(s.Files))
		for i, f := range s.Files {
			files[i] = fmt.Sprintf("%s(%s, %s, U, %s)", DumpExpr(f.Unit), DumpExpr(f.Records), DumpExpr(f.Length), f.Var.Name)
		}

		return "DEFINE FILE " + strings.Join(files, ", "), true
	}

	return "", false
}

// DumpExpr is an expression in the dump's form: a leaf as written, an
// operation as a prefix list, (op X Y).
func DumpExpr(e Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *IntConst:
		if e.Radix == 8 {
			return `"` + e.Text
		}

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
	case *HollerithConst:
		return strconv.Itoa(len(e.Value)) + "H" + e.Value
	case *TypelessConst:
		if e.Radix == 8 {
			return "'" + e.Digits + "'O"
		}

		return "'" + e.Digits + "'X"
	case *ComplexConst:
		return "(complex " + DumpExpr(e.Re) + " " + DumpExpr(e.Im) + ")"
	case *Substring:
		return DumpExpr(e.X) + "(" + DumpExpr(e.Lower) + ":" + DumpExpr(e.Upper) + ")"
	case *FieldRef:
		if e.Args != nil {
			return DumpExpr(e.X) + "." + e.Field + args(e.Args)
		}

		return DumpExpr(e.X) + "." + e.Field
	case *BuiltinArg:
		return "%" + e.Func + "(" + DumpExpr(e.X) + ")"
	case *AltReturn:
		return "*" + strconv.Itoa(e.Target.Label)
	case *ImpliedDo:
		text := "(do " + exprList(e.Items) + "; " + e.Var.Name + " = " +
			DumpExpr(e.Start) + ", " + DumpExpr(e.Limit)
		if e.Step != nil {
			text += ", " + DumpExpr(e.Step)
		}

		return text + ")"
	case *KeyList:
		keys := make([]string, len(e.Keys))
		for i, k := range e.Keys {
			keys[i] = DumpExpr(k.Start) + ":" + DumpExpr(k.End)
			for _, word := range []string{k.Type, k.Dir} {
				if word != "" {
					keys[i] += ":" + word
				}
			}
		}

		return "(keys " + strings.Join(keys, ", ") + ")"
	}

	return fmt.Sprintf("%T", e)
}

// String is the type as written: INTEGER, REAL*8, CHARACTER*(*).
func (t TypeSpec) String() string {
	if t.Type == 0 {
		// A length with no type: FUNCTION F*8.
		return t.Len.String()
	}

	return t.Type.String() + t.Len.String()
}

// String is the length as written, with its *: *8, *(N+1), *(*); or ""
// for none.
func (l Len) String() string {
	switch {
	case l.LenStar:
		return "*(*)"
	case l.LenExpr != nil:
		return "*(" + DumpExpr(l.LenExpr) + ")"
	case l.Length != 0:
		return "*" + strconv.Itoa(l.Length)
	}

	return ""
}

// String is the rule as written: REAL*8 (A-H, O-Z).
func (r *ImplicitRule) String() string {
	letters := make([]string, len(r.Letters))
	for i, l := range r.Letters {
		letters[i] = string(l.First)
		if l.Last != l.First {
			letters[i] += "-" + string(l.Last)
		}
	}

	return r.Type.String() + " (" + strings.Join(letters, ", ") + ")"
}

// String is the qualifier as written, without its slash:
// CHECK=(BOUNDS, OVERFLOW), ALIGN=2, NOWRT.
func (q *Qualifier) String() string {
	switch len(q.Values) {
	case 0:
		return q.Name
	case 1:
		return q.Name + "=" + q.Values[0]
	}

	return q.Name + "=(" + strings.Join(q.Values, ", ") + ")"
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

// String is the declarator in the dump's form, its length before its
// bounds whichever order the statement wrote them in, and its initial
// values after them: IVEC*4(0:9, 3), NAMES*32(100), A(3) /3*0/.
func (d *Declarator) String() string {
	text := d.Name + d.Len.String()

	if d.Dims != nil {
		text += bounds(d.Dims)
	}

	if d.Init != nil {
		text += " " + values(d.Init)
	}

	return text
}

// bounds is an array declarator's bounds: (0:9, 3, *).
func bounds(dims []Bound) string {

	list := make([]string, len(dims))

	for i, b := range dims {
		upper := DumpExpr(b.Upper)
		if b.Star {
			upper = "*"
		}

		if b.Lower != nil {
			upper = DumpExpr(b.Lower) + ":" + upper
		}

		list[i] = upper
	}

	return "(" + strings.Join(list, ", ") + ")"
}

// values is a constant list in slashes: /1, 2*3/.
func values(list []*DataValue) string {
	text := make([]string, len(list))
	for i, v := range list {
		text[i] = DumpExpr(v.Value)
		if v.Repeat != nil {
			text[i] = DumpExpr(v.Repeat) + "*" + text[i]
		}
	}

	return "/" + strings.Join(text, ", ") + "/"
}

// labels is a list of statement labels, (10, 20, 30).
func labels(list []LabelRef) string {
	text := make([]string, len(list))
	for i, l := range list {
		text[i] = strconv.Itoa(l.Label)
	}

	return "(" + strings.Join(text, ", ") + ")"
}

// doLabel is a DO statement's label and a blank, or nothing for a loop
// without one.
func doLabel(l LabelRef) string {
	if l.Label == 0 {
		return ""
	}

	return strconv.Itoa(l.Label) + " "
}

// specs are an I/O statement's keyword specifiers as written: ERR=10,
// IOSTAT=I, READONLY.
func specs(list []*IOSpec) []string {
	text := make([]string, len(list))
	for i, s := range list {
		switch {
		case s.Label != nil:
			text[i] = s.Keyword + "=" + strconv.Itoa(s.Label.Label)
		case s.Value != nil:
			text[i] = s.Keyword + "=" + DumpExpr(s.Value)
		default:
			text[i] = s.Keyword
		}
	}

	return text
}

// quoted is a string as a character constant: 'IT''S'.
func quoted(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
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
	return "(" + exprList(list) + ")"
}

// exprList is a list of expressions, separated by commas: A, B(1).
func exprList(list []Expr) string {
	text := make([]string, len(list))
	for i, e := range list {
		text[i] = DumpExpr(e)
	}

	return strings.Join(text, ", ")
}

// items is an I/O list, each item after a blank.
func items(list []Expr) string {
	var b strings.Builder

	for _, e := range list {
		b.WriteString(" " + DumpExpr(e))
	}

	return b.String()
}
