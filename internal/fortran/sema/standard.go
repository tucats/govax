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
// the standard judged by doesn't have (docs/planning/PHASE-85 - FORTRAN compiler
// usability.md, Decisions 14 and 15; docs/planning/PHASE-110 - Fortran 90 compiler
// usability.md, findings 1 to 3). For FORTRAN-77 the language manual prints those things
// in blue, which is where the first column of the tables here comes from: Phase 85's log
// says how the blue was read from its scan. For Fortran 90 it's the standard's syntax
// rules, its section 3 for the source form, and its chapter 13 for the intrinsic
// procedures. Each finding is a remark, so a program flagged still compiles with a good
// status. The zero value flags nothing, the default.
//
// This is mostly syntax. What a program means is another matter, and isn't looked at: a
// non-integer subscript, .AND. of integers, a character and a number in one common
// block. VAX FORTRAN's own command kept those under a keyword of their own, which the
// phase's goal leaves out.
type Conformance struct {
	// Syntax flags statements, constants, operators, specifiers, edit descriptors,
	// names, and intrinsic functions.
	Syntax bool

	// SourceForm flags how the lines are written.
	SourceForm bool

	// Standard is the standard to judge by, or StandardOfUnit to judge each unit by
	// its /F90 setting: Fortran 90 under /F90 (and so in free form), FORTRAN-77 under
	// /NOF90 (the author's answer 2).
	Standard Judged

	// Obsolescent flags Fortran 90's obsolescent features (the standard's B.2) when
	// judging by Fortran 90, if ObsolescentGiven says a keyword set it; otherwise they
	// are flagged with Syntax. FORTRAN-77 has no obsolescent features.
	Obsolescent      bool
	ObsolescentGiven bool
}

// Judged is a standard /STANDARD judges by.
type Judged int

// The standards.
const (
	// StandardOfUnit is each unit's own, by its /F90 setting.
	StandardOfUnit Judged = iota

	// StandardF77 is FORTRAN-77, /STANDARD=F77.
	StandardF77

	// StandardF90 is Fortran 90, /STANDARD=F90.
	StandardF90
)

// judge is how a setting judges a unit or a line under the /F90 setting f90: by Fortran
// 90 or not, and whether the obsolescent features are flagged.
type judge struct {
	f90, obsolescent bool
}

// judging is the setting's judge where /F90 is f90.
func (s Conformance) judging(f90 bool) judge {
	switch s.Standard {
	case StandardF77:
		f90 = false
	case StandardF90:
		f90 = true
	}

	obsolescent := s.Syntax
	if s.ObsolescentGiven {
		obsolescent = s.Obsolescent
	}

	return judge{f90: f90, obsolescent: f90 && obsolescent}
}

// name is the standard judged by, as a message names it.
func (j judge) name() string {
	if j.f90 {
		return "Fortran 90"
	}

	return "FORTRAN-77"
}

// standardKeyword is one of /STANDARD's keywords: what it does to a setting, and whether
// it says what's looked at, rather than taking something away or naming a standard.
type standardKeyword struct {
	apply func(*Conformance)
	looks bool
}

// standardKeywords are /STANDARD's keywords.
var standardKeywords = map[string]standardKeyword{
	"ALL":           {setAll, true},
	"NONE":          {func(s *Conformance) { *s = Conformance{Standard: s.Standard} }, true},
	"SYNTAX":        {func(s *Conformance) { s.Syntax = true }, true},
	"NOSYNTAX":      {func(s *Conformance) { s.Syntax = false }, false},
	"SOURCE_FORM":   {func(s *Conformance) { s.SourceForm = true }, true},
	"NOSOURCE_FORM": {func(s *Conformance) { s.SourceForm = false }, false},
	"OBSOLESCENT":   {func(s *Conformance) { s.Obsolescent, s.ObsolescentGiven = true, true }, true},
	"NOOBSOLESCENT": {func(s *Conformance) { s.Obsolescent, s.ObsolescentGiven = false, true }, false},
	"F77":           {func(s *Conformance) { s.Standard = StandardF77 }, false},
	"F90":           {func(s *Conformance) { s.Standard = StandardF90 }, false},
}

// setAll is ALL: everything looked at.
func setAll(s *Conformance) {
	s.Syntax, s.SourceForm, s.Obsolescent, s.ObsolescentGiven = true, true, true, true
}

// standardKeywordList says what the keywords are, for a message.
const standardKeywordList = "ALL, NONE, F77, F90, [NO]OBSOLESCENT, [NO]SOURCE_FORM, or [NO]SYNTAX"

// With is the setting with /STANDARD's keywords applied in order. A list that says
// nothing of what's looked at (none, or only F77, F90, and NO forms) starts from ALL, so
// /STANDARD alone and /STANDARD=F90 look at everything (Decision 14).
func (s Conformance) With(keywords []string) (Conformance, error) {
	var list []standardKeyword

	looks := false

	for _, word := range keywords {
		keyword, matches := diag.FindKeyword(standardKeywords, word)

		switch matches {
		case 1:
			list = append(list, keyword)
			looks = looks || keyword.looks
		case 0:
			return s, fmt.Errorf("/STANDARD has no keyword %s; give %s", strings.ToUpper(word), standardKeywordList)
		default:
			return s, fmt.Errorf("/STANDARD's keyword %s is ambiguous; give %s", strings.ToUpper(word), standardKeywordList)
		}
	}

	if !looks {
		setAll(&s)
	}

	for _, keyword := range list {
		keyword.apply(&s)
	}

	return s, nil
}

// ReportSourceExtensions reports what the source pass noted, as the setting asks, each
// mark judged by the standard where it is. A text library's lines aren't the program's
// own to change, so they go unmentioned.
func ReportSourceExtensions(p *source.Program, s Conformance, messages *diag.List) {
	report := func(marks []source.Mark) {
		for _, m := range marks {
			if j := s.judging(m.F90); !m.Library && !m.In.Has(j.f90) {
				messages.Report("EXTENSION", m.Pos, j.name(), m.What)
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

// row is a table's entry: what to call something in a message, and which standards have
// it.
type row struct {
	what string
	in   source.Standards
}

// f90 is Fortran 90's column: a row only it has.
const f90 = source.InF90

// statementRows are the statements FORTRAN-77 doesn't have, and which of them Fortran 90
// has. A derived type's definition, a CASE construct, a WHERE construct, and an
// interface block are each flagged at the statement that starts them.
var statementRows = map[lex.Kind]row{
	lex.Accept:            {"the ACCEPT statement", 0},
	lex.Automatic:         {"the AUTOMATIC statement", 0},
	lex.Byte:              {"the BYTE statement", 0},
	lex.Decode:            {"the DECODE statement", 0},
	lex.DefineFile:        {"the DEFINE FILE statement", 0},
	lex.Delete:            {"the DELETE statement", 0},
	lex.Dictionary:        {"the DICTIONARY statement", 0},
	lex.Directive:         {"a CDEC$ directive", 0},
	lex.DoWhile:           {"the DO WHILE statement", f90},
	lex.DoubleComplex:     {"the DOUBLE COMPLEX statement", 0},
	lex.Encode:            {"the ENCODE statement", 0},
	lex.EndDo:             {"the END DO statement", f90},
	lex.Find:              {"the FIND statement", 0},
	lex.Namelist:          {"the NAMELIST statement", f90},
	lex.Options:           {"the OPTIONS statement", 0},
	lex.Record:            {"the RECORD statement", 0},
	lex.Rewrite:           {"the REWRITE statement", 0},
	lex.Static:            {"the STATIC statement", 0},
	lex.Structure:         {"the STRUCTURE statement", 0},
	lex.Type:              {"the TYPE statement", 0},
	lex.Unlock:            {"the UNLOCK statement", 0},
	lex.Virtual:           {"the VIRTUAL statement", 0},
	lex.Volatile:          {"the VOLATILE statement", 0},
	lex.Allocatable:       {"the ALLOCATABLE statement", f90},
	lex.Allocate:          {"the ALLOCATE statement", f90},
	lex.Contains:          {"the CONTAINS statement", f90},
	lex.Cycle:             {"the CYCLE statement", f90},
	lex.Deallocate:        {"the DEALLOCATE statement", f90},
	lex.Exit:              {"the EXIT statement", f90},
	lex.Intent:            {"the INTENT statement", f90},
	lex.Interface:         {"an interface block", f90},
	lex.Module:            {"a module", f90},
	lex.Nullify:           {"the NULLIFY statement", f90},
	lex.Optional:          {"the OPTIONAL statement", f90},
	lex.PointerAssignment: {"pointer assignment", f90},
	lex.Pointer:           {"the POINTER statement", f90},
	lex.Private:           {"the PRIVATE statement", f90},
	lex.Public:            {"the PUBLIC statement", f90},
	lex.SelectCase:        {"the CASE construct", f90},
	lex.Target:            {"the TARGET statement", f90},
	lex.TypeDef:           {"a derived type's definition", f90},
	lex.TypeDecl:          {"a derived type's TYPE statement", f90},
	lex.Use:               {"the USE statement", f90},
	lex.Where:             {"the WHERE statement", f90},
	lex.WhereBlock:        {"the WHERE construct", f90},
}

// standardWords makes a set of the words in text.
func standardWords(text string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(text) {
		set[word] = true
	}

	return set
}

// standardIntrinsics are FORTRAN-77's intrinsic functions, generic and specific.
var standardIntrinsics = standardWords(`
	INT IFIX IDINT REAL FLOAT SNGL DBLE CMPLX ICHAR CHAR AINT DINT ANINT DNINT NINT IDNINT
	ABS IABS DABS CABS MOD AMOD DMOD SIGN ISIGN DSIGN DIM IDIM DDIM DPROD
	MAX MAX0 AMAX1 DMAX1 AMAX0 MAX1 MIN MIN0 AMIN1 DMIN1 AMIN0 MIN1
	LEN INDEX AIMAG CONJG SQRT DSQRT CSQRT EXP DEXP CEXP LOG ALOG DLOG CLOG LOG10 ALOG10 DLOG10
	SIN DSIN CSIN COS DCOS CCOS TAN DTAN ASIN DASIN ACOS DACOS ATAN DATAN ATAN2 DATAN2
	SINH DSINH COSH DCOSH TANH DTANH LGE LGT LLE LLT`)

// newIntrinsics are the intrinsic procedures Fortran 90 adds to FORTRAN-77's (its
// chapter 13, 13.10 to 13.13). Every other name in Table D-3, and the rest of govax's, is
// DEC's, an extension to both.
var newIntrinsics = standardWords(`
	ACHAR ADJUSTL ADJUSTR ALL ALLOCATED ANY ASSOCIATED BIT_SIZE BTEST CEILING COUNT CSHIFT
	DATE_AND_TIME DIGITS DOT_PRODUCT EOSHIFT EPSILON EXPONENT FLOOR FRACTION HUGE IACHAR IAND
	IBCLR IBITS IBSET IEOR IOR ISHFT ISHFTC KIND LBOUND LEN_TRIM LOGICAL MATMUL MAXEXPONENT
	MAXLOC MAXVAL MERGE MINEXPONENT MINLOC MINVAL MODULO MVBITS NEAREST NOT PACK PRECISION
	PRESENT PRODUCT RADIX RANDOM_NUMBER RANDOM_SEED RANGE REPEAT RESHAPE RRSPACING SCALE SCAN
	SELECTED_INT_KIND SELECTED_REAL_KIND SET_EXPONENT SHAPE SIZE SPACING SPREAD SUM
	SYSTEM_CLOCK TINY TRANSFER TRANSPOSE TRIM UBOUND UNPACK VERIFY`)

// standardSpecifiers are the keywords FORTRAN-77 has in each statement's control list.
// A data transfer statement's unit, format, and record number aren't among its
// specifiers (ast.TransferStmt), so they aren't here.
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

// f90Specifiers are the keywords Fortran 90 adds to each statement's control list (its
// 9.2.1, 9.3.4, 9.6), and to INQUIRE's, IOLENGTH= (9.6.3).
var f90Specifiers = map[lex.Kind]map[string]bool{
	lex.Read:    standardWords("ADVANCE SIZE EOR"),
	lex.Write:   standardWords("ADVANCE"),
	lex.Open:    standardWords("POSITION ACTION DELIM PAD"),
	lex.Inquire: standardWords("POSITION ACTION READ WRITE READWRITE DELIM PAD IOLENGTH"),
}

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

// f90Values are the values Fortran 90 adds (its 9.3.4): STATUS='REPLACE', and its new
// specifiers' words. A specifier Fortran 90 adds isn't FORTRAN-77's at all, so its words
// are judged only by Fortran 90.
var f90Values = map[lex.Kind]map[string]map[string]bool{
	lex.Open: {
		"STATUS":   standardWords("REPLACE"),
		"POSITION": standardWords("ASIS REWIND APPEND"),
		"ACTION":   standardWords("READ WRITE READWRITE"),
		"DELIM":    standardWords("APOSTROPHE QUOTE NONE"),
		"PAD":      standardWords("YES NO"),
	},
	lex.Read:  {"ADVANCE": standardWords("YES NO")},
	lex.Write: {"ADVANCE": standardWords("YES NO")},
}

// extensions gathers what /STANDARD finds, so that it's reported once each and in the
// source's order, whichever pass over the unit found it. With Syntax off, add does
// nothing. judge says which standard the unit is judged by.
type extensions struct {
	on    bool
	judge judge
	found []source.Mark
	seen  map[string]bool

	// inData says the walk is in a DATA statement's values, where Fortran 90 has its
	// B, O, and Z constants (the standard's R533).
	inData bool
}

// newExtensions makes one for the setting, for a unit whose /F90 setting is f90.
func newExtensions(s Conformance, f90 bool) *extensions {
	return &extensions{on: s.Syntax, judge: s.judging(f90), seen: map[string]bool{}}
}

// add notes one finding, which the standards in have: it's flagged only if the standard
// judged by isn't among them.
func (x *extensions) add(pos diag.Position, what string, in source.Standards) {
	if x == nil || !x.on || in.Has(x.judge.f90) {
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
		messages.Report("EXTENSION", m.Pos, x.judge.name(), m.What)
	}

	x.found = nil
}

// intrinsic notes a reference to an intrinsic procedure, if the standard doesn't have it.
// Only semantic analysis can tell a reference from an array element, so it calls this.
func (x *extensions) intrinsic(name string, pos diag.Position) {
	switch {
	case standardIntrinsics[name]:
	case newIntrinsics[name]:
		x.add(pos, "the intrinsic function "+name, f90)
	default:
		x.add(pos, "the intrinsic function "+name, 0)
	}
}

// name notes a name the standard wouldn't take: FORTRAN-77's are at most six letters
// and digits, Fortran 90's 31 letters, digits, and underscores (its 3.2.2), which the
// compiler holds every name to anyway.
func (x *extensions) name(name string, pos diag.Position) {
	switch {
	case name == "" || name == "*":
	case x.judge.f90:
		if strings.Contains(name, "$") {
			x.add(pos, "the name "+name+", with a $ in it", 0)
		}
	case strings.ContainsAny(name, "$_"):
		x.add(pos, "the name "+name+", with a $ or _ in it", 0)
	case len(name) > 6:
		x.add(pos, "the name "+name+", longer than six characters", 0)
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

	x.unit(au)

	for _, s := range a.unit.Symbols {
		// A unit's own name is its header's, if it wrote one: a main program without a
		// PROGRAM statement has one the compiler made. An internal procedure's name
		// is its own unit's to report.
		if !s.library && s.Class != UnitName && s.Internal == nil {
			x.name(s.Name, s.Pos)
		}
	}

	x.report(a.messages)
}

// unit walks a unit's statements. Its internal procedures are units of their own, each
// walked when it's analyzed.
func (x *extensions) unit(au *ast.Unit) {
	x.statements(au.Prologue)

	if au.Header != nil {
		x.statements([]ast.Stmt{au.Header})
	}

	x.statements(au.Body)
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
	base := s.Base()
	kind := base.Kind

	if r, ok := statementRows[kind]; ok {
		x.add(s.Position(), r.what, r.in)
	}

	if base.Construct != "" {
		x.add(base.ConstructPos, "a construct name", f90)
	}

	switch s := s.(type) {
	case *ast.ProgramStmt:
		x.name(s.Name, s.Pos)
	case *ast.ModuleStmt:
		x.name(s.Name, s.Pos)
	case *ast.SubroutineStmt:
		x.name(s.Name, s.Pos)

		if s.Recursive {
			x.add(s.Pos, "a RECURSIVE subprogram", f90)
		}

	case *ast.BlockDataStmt:
		x.name(s.Name, s.Pos)
	case *ast.EntryStmt:
		x.name(s.Name, s.Pos)

		if s.Result != nil {
			x.add(s.Result.Pos, "RESULT", f90)
		}

	case *ast.FunctionStmt:
		x.name(s.Name, s.Pos)

		if s.Recursive {
			x.add(s.Pos, "a RECURSIVE subprogram", f90)
		}

		if s.Result != nil {
			x.add(s.Result.Pos, "RESULT", f90)
			x.name(s.Result.Name, s.Result.Pos)
		}

		if s.Type != nil {
			x.typeSpec(*s.Type)
		}

	case *ast.EndStmt:
		if s.Unit != "" || s.Name != "" {
			x.add(s.Pos, "an END statement naming its unit", f90)
		}

	case *ast.TypeStmt:
		x.typeSpec(s.Type)
		x.attributes(s.Attrs, s.Colons, s.Pos)
		x.declarators(s.Entities, s.Type.Type, s.Type.Derived != "")

	case *ast.DimensionStmt:
		x.attributes(nil, s.Colons, s.Pos)
		x.declarators(s.Entities, ast.Character, false)

	case *ast.TypeDefStmt:
		x.name(s.Name, s.NamePos)

		if s.Access != "" {
			x.add(s.AccessPos, "the "+s.Access+" attribute", f90)
		}

		// Its components are type statements, judged as any are; the rest of its
		// statements are the definition's own.
		for _, part := range s.Body {
			if t, ok := part.(*ast.TypeStmt); ok {
				x.statement(t)
			}
		}

	case *ast.ImplicitStmt:
		if s.None {
			x.add(s.Pos, "IMPLICIT NONE", f90)
		}

		for _, rule := range s.Rules {
			x.typeSpec(rule.Type)
		}

	case *ast.ParameterStmt:
		if s.Old {
			x.add(s.Pos, "a PARAMETER statement without parentheses", 0)
		}

		for _, def := range s.Defs {
			x.expr(def.Value)
		}

	case *ast.CommonStmt:
		for _, block := range s.Blocks {
			x.name(block.Name, block.Pos)
			x.declarators(block.Entities, ast.Character, false)
		}

	case *ast.EquivalenceStmt:
		for _, set := range s.Sets {
			x.exprs(set)
		}

	case *ast.DataStmt:
		for _, set := range s.Sets {
			x.exprs(set.Names)

			x.inData = true
			x.values(set.Values)
			x.inData = false
		}

	case *ast.AttrStmt:
		x.attributes(nil, s.Colons, s.Pos)

		for _, item := range s.Items {
			switch {
			case kind == lex.Intrinsic:
				x.intrinsic(item.Name, item.Pos)
			case item.Star:
				x.add(item.Pos, "EXTERNAL's * form", 0)
			}
		}

	case *ast.InterfaceBlock:
		for _, body := range s.Bodies {
			x.unit(body)
		}

	case *ast.FormatStmt:
		x.format(s)

	case *ast.AssignStmt:
		x.expr(s.Target)
		x.expr(s.Value)

	case *ast.PointerAssignStmt:
		x.exprs([]ast.Expr{s.Pointer, s.Target})

	case *ast.AllocateStmt:
		for _, object := range s.Objects {
			if object.Component != nil {
				x.expr(object.Component)
			}

			x.bounds(object.Dims, object.Pos)
		}

		x.expr(s.Stat)

	case *ast.NullifyStmt:
		x.exprs(s.Objects)

	case *ast.SelectCaseStmt:
		x.expr(s.Expr)

	case *ast.CaseStmt:
		for _, r := range s.Ranges {
			x.exprs([]ast.Expr{r.Low, r.High})
		}

	case *ast.WhereStmt:
		x.expr(s.Mask)
		x.statement(s.Then)

	case *ast.WhereBlockStmt:
		x.expr(s.Mask)

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
			x.add(s.Pos, "a DO statement without a label", f90)
		}

		if s.Forever() {
			x.add(s.Pos, "a DO statement without loop control", f90)
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
		x.exprs(s.Items)
	}
}

// attributes looks at a declaration's Fortran 90 forms: its attributes, and its ::.
func (x *extensions) attributes(attrs []*ast.AttrSpec, colons bool, pos diag.Position) {
	if colons {
		x.add(pos, "a declaration with ::", f90)
	}

	for _, attr := range attrs {
		x.add(attr.Pos, "the "+attr.Name+" attribute", f90)
		x.bounds(attr.Dims, attr.Pos)
	}
}

// transfer looks at a READ, WRITE, or REWRITE with a control list.
func (x *extensions) transfer(s *ast.TransferStmt, kind lex.Kind) {
	if s.RecQuote {
		x.add(s.Rec.Position(), "a record number after an apostrophe", 0)
	}

	if s.Namelist != nil {
		x.add(s.Namelist.Pos, "the NML= specifier", f90)
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
			in := source.Standards(0)
			if f90Specifiers[kind][spec.Keyword] {
				in = f90
			}

			x.add(spec.Pos, "the "+spec.Keyword+" specifier", in)
		}

		x.expr(spec.Value)

		if text, ok := spec.Value.(*ast.CharConst); ok {
			word := strings.ToUpper(strings.TrimSpace(text.Value))
			old := standardValues[kind][spec.Keyword]
			added := f90Values[kind][spec.Keyword]

			switch {
			case old == nil && added == nil, old[word]:
			case added[word]:
				x.add(text.Pos, spec.Keyword+"='"+word+"'", f90)
			default:
				x.add(text.Pos, spec.Keyword+"='"+word+"'", 0)
			}
		}
	}
}

// typeSpec looks at a type as a statement writes it: *n after any type but CHARACTER,
// and Fortran 90's selectors in parentheses.
func (x *extensions) typeSpec(t ast.TypeSpec) {
	if t.Type != ast.Character && t.HasLength() && !t.Selector {
		x.add(t.Pos, "a length after "+t.Type.String(), 0)
	}

	if t.Selector {
		x.add(t.Pos, "a kind or length selector", f90)
	}

	x.expr(t.LenExpr)
	x.expr(t.Kind)
}

// declarators looks at the entities a statement declares, of base type (only a
// CHARACTER one may have a length of its own; derived says it's a derived type, which
// has none).
func (x *extensions) declarators(list []*ast.Declarator, base ast.BaseType, derived bool) {
	for _, d := range list {
		if (base != ast.Character || derived) && d.HasLength() {
			x.add(d.Pos, "a length after the name "+d.Name, 0)
		}

		if d.Init != nil {
			x.add(d.InitPos, "initial values in a type statement", 0)
		}

		if d.Value != nil {
			x.add(d.ValuePos, "an initial value after =", f90)
		}

		x.expr(d.LenExpr)
		x.values(d.Init)
		x.expr(d.Value)
		x.bounds(d.Dims, d.Pos)
	}
}

// bounds looks at an array's bounds, among them Fortran 90's deferred and assumed shape;
// pos is the name's, since a bound has no place of its own.
func (x *extensions) bounds(dims []ast.Bound, pos diag.Position) {
	for _, bound := range dims {
		if bound.Colon {
			x.add(pos, "a deferred-shape or assumed-shape array", f90)
		}

		x.expr(bound.Lower)
		x.expr(bound.Upper)
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
	f, err := format.Parse(s.Text, format.Options{F90: s.Quotes()})
	if err != nil {
		return
	}

	at := func(item *format.Item) diag.Position {
		return s.Source.Position(s.Offset + item.Offset)
	}

	if len(f.VFEs) > 0 {
		x.add(s.Pos, "a variable format expression", 0)
	}

	var walk func(items []*format.Item)

	walk = func(items []*format.Item) {
		for _, item := range items {
			switch item.Code {
			case format.O, format.Z, format.B, format.EN, format.ES:
				x.add(at(item), "the "+item.Code.String()+" edit descriptor", f90)
			case format.Q, format.Dollar:
				x.add(at(item), "the "+item.Code.String()+" edit descriptor", 0)
			case format.Slash:
				if item.Repeat.Set || item.Repeat.VFE > 0 {
					x.add(at(item), "a repeated slash edit descriptor", f90)
				}
			case format.I, format.F, format.E, format.D, format.G, format.L:
				if !item.W.Set && item.W.VFE == 0 {
					x.add(at(item), "the "+item.Code.String()+" edit descriptor without a width", 0)
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
			x.add(pos, "a null argument", 0)
		}

		x.expr(e)
	}
}

// expr looks at an expression and everything in it. A missing part may be a nil of any
// kind, since many statements have optional parts, so each case asks of its own type.
// An array section and a vector subscript are noted by semantic analysis, which alone
// can tell a section from a substring (sections.go).
func (x *extensions) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.IntConst:
		if e.Radix == 8 {
			x.add(e.Pos, "an octal constant with a quotation mark", 0)
		}

	case *ast.RealConst:
		if strings.ContainsAny(e.Text, "Qq") {
			x.add(e.Pos, "a REAL*16 constant", 0)
		}

	case *ast.CharConst:
		if e.Quoted {
			x.add(e.Pos, "a character constant in quotation marks", f90)
		}

	case *ast.HollerithConst:
		x.add(e.Pos, "a Hollerith constant", 0)

	case *ast.TypelessConst:
		switch {
		case e.Prefixed && x.inData:
			x.add(e.Pos, "a binary, octal, or hexadecimal constant", f90)
		case e.Prefixed:
			x.add(e.Pos, "a binary, octal, or hexadecimal constant outside DATA", 0)
		default:
			x.add(e.Pos, "an octal or hexadecimal constant", 0)
		}

	case *ast.Radix50Const:
		x.add(e.Pos, "a RADIX-50 constant", 0)

	case *ast.KindConst:
		x.add(e.Pos, "a constant with a kind parameter", f90)
		x.expr(e.X)

	case *ast.ComplexConst:
		if isDoubleConst(e.Re) || isDoubleConst(e.Im) {
			x.add(e.Pos, "a COMPLEX*16 constant", f90)
		}

		x.expr(e.Re)
		x.expr(e.Im)

	case *ast.Apply:
		x.arguments(e.Args, e.Pos)

	case *ast.Unary:
		x.expr(e.X)

	case *ast.Binary:
		if e.Op == ast.Xor {
			x.add(e.Pos, "the .XOR. operator", 0)
		}

		if e.Symbolic {
			x.add(e.Pos, "the "+e.Spelling()+" operator", f90)
		}

		if y, ok := e.Y.(*ast.Unary); ok && y.Op != ast.Not && e.Op <= ast.Pow {
			x.add(y.Pos, "a sign straight after an operator", 0)
		}

		x.expr(e.X)
		x.expr(e.Y)

	case *ast.Paren:
		x.expr(e.X)

	case *ast.Substring:
		x.exprs([]ast.Expr{e.X, e.Lower, e.Upper})

	case *ast.FieldRef:
		if e.Percent {
			x.add(e.Pos, "a structure component", f90)
		} else {
			x.add(e.Pos, "a reference to a record's field", 0)
		}

		x.expr(e.X)
		x.exprs(e.Args)

	case *ast.BuiltinArg:
		x.add(e.Pos, "the %"+e.Func+" built-in function", 0)
		x.expr(e.X)

	case *ast.KeywordArg:
		x.add(e.Pos, "a keyword argument", f90)
		x.expr(e.X)

	case *ast.ImpliedDo:
		x.exprs(e.Items)
		x.exprs([]ast.Expr{e.Start, e.Limit, e.Step})

	case *ast.Triplet:
		x.exprs([]ast.Expr{e.Lower, e.Upper, e.Stride})

	case *ast.ArrayConstructor:
		x.add(e.Pos, "an array constructor", f90)
		x.exprs(e.Values)

	case *ast.DefinedOp:
		x.add(e.Pos, "a defined operator", f90)
		x.exprs([]ast.Expr{e.X, e.Y})
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
