package sema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/format"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/source"
)

// Conformance is what /STANDARD asks the compiler to flag: what a program does that
// FORTRAN-77 doesn't have (docs/PHASE-85 - FORTRAN compiler usability.md, Decisions 14
// and 15). The language manual prints those things in blue, which is where the tables
// here come from: the phase doc's log says how the blue was read from its scan. Each
// finding is a remark, so a program flagged still compiles with a good status. The zero
// value flags nothing, the default.
//
// This is syntax only. What a program means is another matter, and isn't looked at: a
// non-integer subscript, .AND. of integers, a character and a number in one common
// block. VAX FORTRAN's own command kept those under a keyword of their own, which the
// phase's goal leaves out.
type Conformance struct {
	// Syntax flags statements, constants, operators, specifiers, edit descriptors,
	// names, and intrinsic functions.
	Syntax bool

	// SourceForm flags how the lines are written.
	SourceForm bool
}

// standardKeywords are /STANDARD's keywords, each what it does to a setting.
var standardKeywords = map[string]func(*Conformance){
	"ALL":           func(s *Conformance) { *s = Conformance{Syntax: true, SourceForm: true} },
	"NONE":          func(s *Conformance) { *s = Conformance{} },
	"SYNTAX":        func(s *Conformance) { s.Syntax = true },
	"NOSYNTAX":      func(s *Conformance) { s.Syntax = false },
	"SOURCE_FORM":   func(s *Conformance) { s.SourceForm = true },
	"NOSOURCE_FORM": func(s *Conformance) { s.SourceForm = false },
}

// standardKeywordList says what the keywords are, for a message.
const standardKeywordList = "ALL, NONE, [NO]SOURCE_FORM, or [NO]SYNTAX"

// With is the setting with /STANDARD's keywords applied in order. No keywords is
// /STANDARD alone, both of them (Decision 14).
func (s Conformance) With(keywords []string) (Conformance, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	for _, word := range keywords {
		apply, matches := diag.FindKeyword(standardKeywords, word)

		switch matches {
		case 1:
			apply(&s)
		case 0:
			return s, fmt.Errorf("/STANDARD has no keyword %s; give %s", strings.ToUpper(word), standardKeywordList)
		default:
			return s, fmt.Errorf("/STANDARD's keyword %s is ambiguous; give %s", strings.ToUpper(word), standardKeywordList)
		}
	}

	return s, nil
}

// ReportSourceExtensions reports what the source pass noted, as the setting asks. A text
// library's lines aren't the program's own to change, so they go unmentioned.
func ReportSourceExtensions(p *source.Program, s Conformance, messages *diag.List) {
	report := func(marks []source.Mark) {
		for _, m := range marks {
			if !m.Library {
				messages.Report("EXTENSION", m.Pos, m.What)
			}
		}
	}

	if s.SourceForm {
		marks := slices.Concat(p.Marks, p.LineMarks())
		sortMarks(marks)
		report(marks)
	}

	if s.Syntax {
		report(p.Includes)
	}
}

// sortMarks puts marks in the source's order.
func sortMarks(marks []source.Mark) {
	slices.SortStableFunc(marks, func(x, y source.Mark) int {
		return comparePositions(x.Pos, y.Pos)
	})
}

// comparePositions orders two places in the source.
func comparePositions(x, y diag.Position) int {
	if x.Serial != y.Serial {
		return x.Serial - y.Serial
	}

	return x.Column - y.Column
}

// extensionStatements are the statements FORTRAN-77 doesn't have at all.
var extensionStatements = map[lex.Kind]string{
	lex.Accept:        "the ACCEPT statement",
	lex.Automatic:     "the AUTOMATIC statement",
	lex.Byte:          "the BYTE statement",
	lex.Decode:        "the DECODE statement",
	lex.DefineFile:    "the DEFINE FILE statement",
	lex.Delete:        "the DELETE statement",
	lex.Dictionary:    "the DICTIONARY statement",
	lex.Directive:     "a CDEC$ directive",
	lex.DoWhile:       "the DO WHILE statement",
	lex.DoubleComplex: "the DOUBLE COMPLEX statement",
	lex.Encode:        "the ENCODE statement",
	lex.EndDo:         "the END DO statement",
	lex.Find:          "the FIND statement",
	lex.Namelist:      "the NAMELIST statement",
	lex.Options:       "the OPTIONS statement",
	lex.Record:        "the RECORD statement",
	lex.Rewrite:       "the REWRITE statement",
	lex.Static:        "the STATIC statement",
	lex.Structure:     "the STRUCTURE statement",
	lex.Type:          "the TYPE statement",
	lex.Unlock:        "the UNLOCK statement",
	lex.Virtual:       "the VIRTUAL statement",
	lex.Volatile:      "the VOLATILE statement",
}

// standardWords makes a set of the words in text.
func standardWords(text string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(text) {
		set[word] = true
	}

	return set
}

// standardIntrinsics are FORTRAN-77's intrinsic functions, generic and specific. Every
// other name in Table D-3 is DEC's.
var standardIntrinsics = standardWords(`
	INT IFIX IDINT REAL FLOAT SNGL DBLE CMPLX ICHAR CHAR AINT DINT ANINT DNINT NINT IDNINT
	ABS IABS DABS CABS MOD AMOD DMOD SIGN ISIGN DSIGN DIM IDIM DDIM DPROD
	MAX MAX0 AMAX1 DMAX1 AMAX0 MAX1 MIN MIN0 AMIN1 DMIN1 AMIN0 MIN1
	LEN INDEX AIMAG CONJG SQRT DSQRT CSQRT EXP DEXP CEXP LOG ALOG DLOG CLOG LOG10 ALOG10 DLOG10
	SIN DSIN CSIN COS DCOS CCOS TAN DTAN ASIN DASIN ACOS DACOS ATAN DATAN ATAN2 DATAN2
	SINH DSINH COSH DCOSH TANH DTANH LGE LGT LLE LLT`)

// standardSpecifiers are the keywords FORTRAN-77 has in each statement's control list.
var standardSpecifiers = func() map[lex.Kind]map[string]bool {
	transfer := standardWords("ERR END IOSTAT")
	position := standardWords("ERR IOSTAT")

	return map[lex.Kind]map[string]bool{
		lex.Read:      transfer,
		lex.Write:     transfer,
		lex.Open:      standardWords("ERR IOSTAT FILE STATUS ACCESS FORM RECL BLANK"),
		lex.Close:     standardWords("ERR IOSTAT STATUS"),
		lex.Backspace: position,
		lex.EndFile:   position,
		lex.Rewind:    position,
		lex.Inquire: standardWords(`ERR IOSTAT FILE EXIST OPENED NUMBER NAMED NAME ACCESS SEQUENTIAL
			DIRECT FORM FORMATTED UNFORMATTED RECL NEXTREC BLANK`),
	}
}()

// standardValues are the values FORTRAN-77 has for OPEN's and CLOSE's specifiers that
// take a word, when the program writes one as a constant.
var standardValues = map[lex.Kind]map[string]map[string]bool{
	lex.Open: {
		"STATUS": standardWords("OLD NEW SCRATCH UNKNOWN"),
		"ACCESS": standardWords("SEQUENTIAL DIRECT"),
		"FORM":   standardWords("FORMATTED UNFORMATTED"),
		"BLANK":  standardWords("NULL ZERO"),
	},
	lex.Close: {
		"STATUS": standardWords("KEEP DELETE"),
	},
}

// extensions gathers what /STANDARD finds, so that it's reported once each and in the
// source's order, whichever pass over the unit found it. With Syntax off, add does
// nothing.
type extensions struct {
	on    bool
	found []source.Mark
	seen  map[string]bool
}

// newExtensions makes one for the setting.
func newExtensions(s Conformance) *extensions {
	return &extensions{on: s.Syntax, seen: map[string]bool{}}
}

// add notes one finding.
func (x *extensions) add(pos diag.Position, what string) {
	if !x.on {
		return
	}

	key := fmt.Sprint(pos.Serial, ":", pos.Column, ":", what)
	if !x.seen[key] {
		x.seen[key] = true
		x.found = append(x.found, source.Mark{Pos: pos, What: what})
	}
}

// report gives its messages, in the source's order.
func (x *extensions) report(messages *diag.List) {
	sortMarks(x.found)

	for _, m := range x.found {
		messages.Report("EXTENSION", m.Pos, m.What)
	}

	x.found = nil
}

// intrinsic notes a reference to an intrinsic function, if FORTRAN-77 doesn't have it.
// Only semantic analysis can tell a reference from an array element, so it calls this.
func (x *extensions) intrinsic(name string, pos diag.Position) {
	if !standardIntrinsics[name] {
		x.add(pos, "the intrinsic function "+name)
	}
}

// name notes a name FORTRAN-77 wouldn't take: at most six letters and digits.
func (x *extensions) name(name string, pos diag.Position) {
	switch {
	case name == "" || name == "*":
	case strings.ContainsAny(name, "$_"):
		x.add(pos, "the name "+name+", with a $ or _ in it")
	case len(name) > 6:
		x.add(pos, "the name "+name+", longer than six characters")
	}
}

// reportExtensions is /STANDARD=SYNTAX's look at a unit, after its statements have been
// read, so that the intrinsic functions referred to have been noted. A text library's
// names and statements aren't the program's own to change.
func (a *analyzer) reportExtensions(au *ast.Unit) {
	x := a.extensions
	if !x.on {
		return
	}

	x.statements(au.Prologue)

	if au.Header != nil {
		x.statements([]ast.Stmt{au.Header})
	}

	x.statements(au.Body)

	for _, s := range a.unit.Symbols {
		// A unit's own name is its header's, if it wrote one: a main program without a
		// PROGRAM statement has one the compiler made.
		if !s.library && s.Class != UnitName {
			x.name(s.Name, s.Pos)
		}
	}

	x.report(a.messages)
}

// statements walks statements.
func (x *extensions) statements(list []ast.Stmt) {
	for _, s := range list {
		if src := s.Base().Source; src == nil || !src.Library {
			x.statement(s)
		}
	}
}

// statement looks at one statement: its kind, then what only its form can say. A
// structure declaration is flagged at its STRUCTURE statement, whatever it holds.
func (x *extensions) statement(s ast.Stmt) {
	kind := s.Base().Kind

	if what, ok := extensionStatements[kind]; ok {
		x.add(s.Position(), what)
	}

	switch s := s.(type) {
	case *ast.ProgramStmt:
		x.name(s.Name, s.Pos)
	case *ast.SubroutineStmt:
		x.name(s.Name, s.Pos)
	case *ast.BlockDataStmt:
		x.name(s.Name, s.Pos)
	case *ast.EntryStmt:
		x.name(s.Name, s.Pos)

	case *ast.FunctionStmt:
		x.name(s.Name, s.Pos)

		if s.Type != nil {
			x.typeSpec(*s.Type)
		}

	case *ast.TypeStmt:
		x.typeSpec(s.Type)
		x.declarators(s.Entities, s.Type.Type)

	case *ast.DimensionStmt:
		x.declarators(s.Entities, ast.Character)

	case *ast.ImplicitStmt:
		if s.None {
			x.add(s.Pos, "IMPLICIT NONE")
		}

		for _, rule := range s.Rules {
			x.typeSpec(rule.Type)
		}

	case *ast.ParameterStmt:
		if s.Old {
			x.add(s.Pos, "a PARAMETER statement without parentheses")
		}

		for _, def := range s.Defs {
			x.expr(def.Value)
		}

	case *ast.CommonStmt:
		for _, block := range s.Blocks {
			x.name(block.Name, block.Pos)
			x.declarators(block.Entities, ast.Character)
		}

	case *ast.EquivalenceStmt:
		for _, set := range s.Sets {
			x.exprs(set)
		}

	case *ast.DataStmt:
		for _, set := range s.Sets {
			x.exprs(set.Names)
			x.values(set.Values)
		}

	case *ast.AttrStmt:
		for _, item := range s.Items {
			switch {
			case kind == lex.Intrinsic:
				x.intrinsic(item.Name, item.Pos)
			case item.Star:
				x.add(item.Pos, "EXTERNAL's * form")
			}
		}

	case *ast.FormatStmt:
		x.format(s)

	case *ast.AssignStmt:
		x.expr(s.Target)
		x.expr(s.Value)

	case *ast.CallStmt:
		x.arguments(s.Args, s.Pos)

	case *ast.ComputedGotoStmt:
		x.expr(s.Index)

	case *ast.IfStmt:
		x.expr(s.Cond)
		x.statement(s.Then)

	case *ast.ArithIfStmt:
		x.expr(s.Cond)

	case *ast.IfThenStmt:
		x.expr(s.Cond)

	case *ast.ElseIfStmt:
		x.expr(s.Cond)

	case *ast.DoStmt:
		if s.Target.Label == 0 {
			x.add(s.Pos, "a DO statement without a label")
		}

		x.exprs([]ast.Expr{s.Start, s.Limit, s.Step})

	case *ast.DoWhileStmt:
		x.expr(s.Cond)

	case *ast.StopStmt:
		x.expr(s.Display)

	case *ast.ReturnStmt:
		x.expr(s.Alt)

	case *ast.PrintStmt:
		x.expr(s.Format.Expr)
		x.exprs(s.Items)

	case *ast.TransferStmt:
		x.transfer(s, kind)

	case *ast.CodeStmt:
		x.expr(s.Count)
		x.expr(s.Format.Expr)
		x.expr(s.Buffer)
		x.exprs(s.Items)

	case *ast.AuxIOStmt:
		if s.Unit != nil {
			x.expr(s.Unit.Expr)
		}

		x.specifiers(s.Specs, kind)
	}
}

// transfer looks at a READ, WRITE, or REWRITE with a control list.
func (x *extensions) transfer(s *ast.TransferStmt, kind lex.Kind) {
	if s.RecQuote {
		x.add(s.Rec.Position(), "a record number after an apostrophe")
	}

	if s.Namelist != nil {
		x.add(s.Namelist.Pos, "the NML= specifier")
	}

	x.expr(s.Unit.Expr)

	if s.Format != nil {
		x.expr(s.Format.Expr)
	}

	x.expr(s.Rec)
	x.specifiers(s.Specs, kind)
	x.exprs(s.Items)
}

// specifiers looks at a control list's keywords and the words they're given.
func (x *extensions) specifiers(specs []*ast.IOSpec, kind lex.Kind) {
	allowed, known := standardSpecifiers[kind]

	for _, spec := range specs {
		if known && !allowed[spec.Keyword] {
			x.add(spec.Pos, "the "+spec.Keyword+" specifier")
		}

		x.expr(spec.Value)

		if text, ok := spec.Value.(*ast.CharConst); ok {
			word := strings.ToUpper(strings.TrimSpace(text.Value))
			if words := standardValues[kind][spec.Keyword]; words != nil && !words[word] {
				x.add(text.Pos, spec.Keyword+"='"+word+"'")
			}
		}
	}
}

// typeSpec looks at a type as a statement writes it: *n after any type but CHARACTER.
func (x *extensions) typeSpec(t ast.TypeSpec) {
	if t.Type != ast.Character && t.HasLength() {
		x.add(t.Pos, "a length after "+t.Type.String())
	}

	x.expr(t.LenExpr)
}

// declarators looks at the entities a statement declares, of base type (only a
// CHARACTER one may have a length of its own).
func (x *extensions) declarators(list []*ast.Declarator, base ast.BaseType) {
	for _, d := range list {
		if base != ast.Character && d.HasLength() {
			x.add(d.Pos, "a length after the name "+d.Name)
		}

		if d.Init != nil {
			x.add(d.InitPos, "initial values in a type statement")
		}

		x.expr(d.LenExpr)
		x.values(d.Init)

		for _, bound := range d.Dims {
			x.expr(bound.Lower)
			x.expr(bound.Upper)
		}
	}
}

// values looks at a list of constants.
func (x *extensions) values(list []*ast.DataValue) {
	for _, v := range list {
		x.expr(v.Repeat)
		x.expr(v.Value)
	}
}

// format looks at a FORMAT statement's edit descriptors. A format with an error has
// its message already, and nothing to look at.
func (x *extensions) format(s *ast.FormatStmt) {
	f, err := format.Parse(s.Text, format.Options{})
	if err != nil {
		return
	}

	at := func(item *format.Item) diag.Position {
		return s.Source.Position(s.Offset + item.Offset)
	}

	if len(f.VFEs) > 0 {
		x.add(s.Pos, "a variable format expression")
	}

	var walk func(items []*format.Item)

	walk = func(items []*format.Item) {
		for _, item := range items {
			switch item.Code {
			case format.O, format.Z, format.Q, format.Dollar:
				x.add(at(item), "the "+item.Code.String()+" edit descriptor")
			case format.I, format.F, format.E, format.D, format.G, format.L:
				if !item.W.Set && item.W.VFE == 0 {
					x.add(at(item), "the "+item.Code.String()+" edit descriptor without a width")
				}
			case format.Group:
				walk(item.Items)
			}
		}
	}

	walk(f.Items)
}

// exprs looks at expressions.
func (x *extensions) exprs(list []ast.Expr) {
	for _, e := range list {
		x.expr(e)
	}
}

// arguments looks at an argument list, in which a nil is a null argument.
func (x *extensions) arguments(args []ast.Expr, pos diag.Position) {
	for _, e := range args {
		if e == nil {
			x.add(pos, "a null argument")
		}

		x.expr(e)
	}
}

// expr looks at an expression and everything in it. A missing part may be a nil of any
// kind, since many statements have optional parts, so each case asks of its own type.
func (x *extensions) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.IntConst:
		if e.Radix == 8 {
			x.add(e.Pos, "an octal constant with a quotation mark")
		}

	case *ast.RealConst:
		if strings.ContainsAny(e.Text, "Qq") {
			x.add(e.Pos, "a REAL*16 constant")
		}

	case *ast.HollerithConst:
		x.add(e.Pos, "a Hollerith constant")

	case *ast.TypelessConst:
		x.add(e.Pos, "an octal or hexadecimal constant")

	case *ast.Radix50Const:
		x.add(e.Pos, "a RADIX-50 constant")

	case *ast.ComplexConst:
		if isDoubleConst(e.Re) || isDoubleConst(e.Im) {
			x.add(e.Pos, "a COMPLEX*16 constant")
		}

		x.expr(e.Re)
		x.expr(e.Im)

	case *ast.Apply:
		x.arguments(e.Args, e.Pos)

	case *ast.Unary:
		x.expr(e.X)

	case *ast.Binary:
		if e.Op == ast.Xor {
			x.add(e.Pos, "the .XOR. operator")
		}

		if y, ok := e.Y.(*ast.Unary); ok && y.Op != ast.Not && e.Op <= ast.Pow {
			x.add(y.Pos, "a sign straight after an operator")
		}

		x.expr(e.X)
		x.expr(e.Y)

	case *ast.Paren:
		x.expr(e.X)

	case *ast.Substring:
		x.exprs([]ast.Expr{e.X, e.Lower, e.Upper})

	case *ast.FieldRef:
		x.add(e.Pos, "a reference to a record's field")
		x.expr(e.X)
		x.exprs(e.Args)

	case *ast.BuiltinArg:
		x.add(e.Pos, "the %"+e.Func+" built-in function")
		x.expr(e.X)

	case *ast.ImpliedDo:
		x.exprs(e.Items)
		x.exprs([]ast.Expr{e.Start, e.Limit, e.Step})
	}
}

// isDoubleConst reports whether a complex constant's part makes it COMPLEX*16: a real
// constant with a D or Q exponent, signed or not.
func isDoubleConst(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Unary:
		return isDoubleConst(e.X)
	case *ast.RealConst:
		return strings.ContainsAny(e.Text, "DdQq")
	}

	return false
}
