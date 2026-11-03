package sema

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The file statements (chapter 9). OPEN and CLOSE give their keywords in
// any order, and each becomes a code and a value that FTNRTL reads as VMS's
// FORTRAN run-time library reads its own (docs/PHASE-81 - FORTRAN files.md,
// "FOR$OPEN's arguments"): so one table, keyword to code, runs from here to
// the run-time library.

// keywordKind is what a file statement's keyword takes as its value.
type keywordKind int

const (
	numberKeyword keywordKind = iota // a number, converted to INTEGER*4
	choiceKeyword                    // a character value, one of a list
	nameKeyword                      // a file's name: a character value, or a numeric place
	flagKeyword                      // nothing: READONLY
	keysKeyword                      // KEY's list
	assocKeyword                     // ASSOCIATEVARIABLE's: an INTEGER variable FTNRTL stores in
	procKeyword                      // USEROPEN's: a procedure declared EXTERNAL
	numberAnswer                     // INQUIRE's: an INTEGER place to store in
	logicalAnswer                    // a LOGICAL place
	textAnswer                       // a CHARACTER place
)

// fileKeyword is one keyword's entry: its code, its kind, and for a choice
// the number each text has. Later names a phase that compiles it, for a
// keyword this one doesn't.
type fileKeyword struct {
	code    int
	kind    keywordKind
	choices map[string]int
	later   string
}

// The choices. Each number is FORRTL's for that text.
var (
	statusChoices = map[string]int{"OLD": 1, "NEW": 2, "SCRATCH": 3, "UNKNOWN": 4}
	disposeChoice = map[string]int{"KEEP": 1, "SAVE": 1, "DELETE": 2, "PRINT": 3, "SUBMIT": 4, "PRINT/DELETE": 5, "SUBMIT/DELETE": 6}
	accessChoices = map[string]int{"DIRECT": 1, "SEQUENTIAL": 2, "APPEND": 3, "KEYED": 4}
	formChoices   = map[string]int{"FORMATTED": 1, "UNFORMATTED": 2}
	ccChoices     = map[string]int{"FORTRAN": 1, "LIST": 2, "NONE": 3}
	orgChoices    = map[string]int{"SEQUENTIAL": 1, "RELATIVE": 2, "INDEXED": 3}
	typeChoices   = map[string]int{"FIXED": 1, "VARIABLE": 2, "SEGMENTED": 3, "STREAM": 4, "STREAM_CR": 5, "STREAM_LF": 6}
	blankChoices  = map[string]int{"ZERO": 1, "NULL": 2}

	// convertChoices are CONVERT='s (the 2005 manual's; VMS 7.1's FORRTL has
	// them, testdata/for81's round 2). NATIVE is 9 here, as FORRTL also takes
	// it, since FTNRTL reads 0 as no CONVERT= at all; FTNRTL converts nothing,
	// so any other is FOR$_NOTIMP when the program runs.
	convertChoices = map[string]int{"NATIVE": 9, "LITTLE_ENDIAN": 1, "BIG_ENDIAN": 2, "CRAY": 3, "IBM": 4,
		"VAXD": 5, "VAXG": 6, "FDX": 7, "FGX": 8}
)

// The codes this package needs by name.
const (
	codeDispose  = 2
	codeAccess   = 4
	codeForm     = 5
	codeRecl     = 6
	codeReadOnly = 8
	codeStatus   = 15
	codeOrg      = 19
	codeType     = 20
	codeKey      = 23
	codeBlank    = 24
)

// openKeywords are OPEN's keywords (Table 9-1), beside ERR= and IOSTAT=.
var openKeywords = map[string]fileKeyword{
	"FILE":              {code: 14, kind: nameKeyword},
	"NAME":              {code: 14, kind: nameKeyword},
	"DEFAULTFILE":       {code: 26, kind: nameKeyword},
	"STATUS":            {code: codeStatus, kind: choiceKeyword, choices: statusChoices},
	"TYPE":              {code: codeStatus, kind: choiceKeyword, choices: statusChoices},
	"DISPOSE":           {code: codeDispose, kind: choiceKeyword, choices: disposeChoice},
	"DISP":              {code: codeDispose, kind: choiceKeyword, choices: disposeChoice},
	"ACCESS":            {code: codeAccess, kind: choiceKeyword, choices: accessChoices},
	"FORM":              {code: codeForm, kind: choiceKeyword, choices: formChoices},
	"RECL":              {code: codeRecl, kind: numberKeyword},
	"RECORDSIZE":        {code: codeRecl, kind: numberKeyword},
	"CARRIAGECONTROL":   {code: 7, kind: choiceKeyword, choices: ccChoices},
	"READONLY":          {code: codeReadOnly, kind: flagKeyword},
	"BUFFERCOUNT":       {code: 9, kind: numberKeyword},
	"INITIALSIZE":       {code: 10, kind: numberKeyword},
	"EXTENDSIZE":        {code: 11, kind: numberKeyword},
	"NOSPANBLOCKS":      {code: 12, kind: flagKeyword},
	"SHARED":            {code: 13, kind: flagKeyword},
	"MAXREC":            {code: 16, kind: numberKeyword},
	"ASSOCIATEVARIABLE": {code: 17, kind: assocKeyword},
	"BLOCKSIZE":         {code: 18, kind: numberKeyword},
	"ORGANIZATION":      {code: codeOrg, kind: choiceKeyword, choices: orgChoices},
	"RECORDTYPE":        {code: codeType, kind: choiceKeyword, choices: typeChoices},
	"USEROPEN":          {code: 21, kind: procKeyword},
	"KEY":               {code: codeKey, kind: keysKeyword},
	"BLANK":             {code: codeBlank, kind: choiceKeyword, choices: blankChoices},
	"CONVERT":           {code: 27, kind: choiceKeyword, choices: convertChoices},
}

// closeKeywords are CLOSE's: STATUS is another name for DISPOSE here.
var closeKeywords = map[string]fileKeyword{
	"STATUS":  {code: codeDispose, kind: choiceKeyword, choices: disposeChoice},
	"DISPOSE": {code: codeDispose, kind: choiceKeyword, choices: disposeChoice},
	"DISP":    {code: codeDispose, kind: choiceKeyword, choices: disposeChoice},
}

// inquireKeywords are INQUIRE's (section 9.3): the file's name, and what it
// asks, each answer's code FORRTL's.
var inquireKeywords = map[string]fileKeyword{
	"FILE":            {code: 14, kind: nameKeyword},
	"DEFAULTFILE":     {code: 26, kind: nameKeyword},
	"EXIST":           {code: 30, kind: logicalAnswer},
	"OPENED":          {code: 31, kind: logicalAnswer},
	"NUMBER":          {code: 32, kind: numberAnswer},
	"NAMED":           {code: 33, kind: logicalAnswer},
	"NAME":            {code: 34, kind: textAnswer},
	"ACCESS":          {code: 35, kind: textAnswer},
	"SEQUENTIAL":      {code: 36, kind: textAnswer},
	"DIRECT":          {code: 37, kind: textAnswer},
	"FORM":            {code: 38, kind: textAnswer},
	"FORMATTED":       {code: 39, kind: textAnswer},
	"UNFORMATTED":     {code: 40, kind: textAnswer},
	"RECL":            {code: 41, kind: numberAnswer},
	"NEXTREC":         {code: 42, kind: numberAnswer},
	"BLANK":           {code: 43, kind: textAnswer},
	"ORGANIZATION":    {code: 44, kind: textAnswer},
	"RECORDTYPE":      {code: 45, kind: textAnswer},
	"KEYED":           {code: 46, kind: textAnswer},
	"CARRIAGECONTROL": {code: 47, kind: textAnswer},
	"CONVERT":         {code: 48, kind: textAnswer},
}

// codeUnit is the keyword INQUIRE passes its unit by, since by file it has none.
const codeUnit = 1

// FileStmt is OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE, or FIND (by
// its AST's kind): the unit, an INTEGER*4 value, nil for an OPEN with no
// UNIT= (FORRTL's unit 0) and for INQUIRE, whose unit is a keyword; FIND's
// record number, an INTEGER*4; its keywords, in order; and ERR= and IOSTAT=,
// as IO's.
type FileStmt struct {
	stmtBase
	Unit   Expr
	Rec    Expr
	Keys   []*FileKey
	Err    int
	IOStat Expr
}

// DefineFile is DEFINE FILE (appendix A.2): for each file, its unit, its
// number of records, and their length in 16-bit words, each an INTEGER*4, and
// its associated variable, an INTEGER*2 or INTEGER*4 *Var.
type DefineFile struct {
	stmtBase
	Files []*FileDef
}

// FileDef is one file of a DEFINE FILE statement.
type FileDef struct {
	Unit, Records, Length Expr
	Var                   Expr
}

// FileKey is one keyword: its code, and its value: Number, a number known
// now (a choice's, or a flag's 1); Value, an INTEGER*4 expression; Text, a
// character value or a numeric place holding a name; or Answer, the place
// INQUIRE stores its answer in.
type FileKey struct {
	Code   int
	Name   string
	Number int
	Value  Expr
	Text   Expr
	Answer Expr
	Proc   *Procedure
	Known  bool
}

// fileStatement analyzes OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE,
// DELETE, and UNLOCK (sections 9.1 to 9.8), and FIND (appendix A.3). A DELETE
// with a record number deletes that record of a relative file, and one without
// the unit's current record.
func (a *analyzer) fileStatement(s *ast.AuxIOStmt, path []frame) Stmt {
	st := &FileStmt{stmtBase: stmtBase{s}}
	ok := true

	if s.Unit != nil {
		if s.Unit.Star {
			a.report("BADUNIT", s.Unit.Pos, "*")

			ok = false
		} else {
			v := a.value(s.Unit.Expr)

			switch t := v.Type(); {
			case t.Kind == Invalid:
				ok = false
			case t.Kind != Integer:
				a.report("BADUNIT", s.Unit.Pos, article(t.String())+" value")

				ok = false
			default:
				st.Unit = a.convert(v, IntegerType)
			}
		}
	}

	keywords := openKeywords

	switch s.Kind {
	case lex.Close:
		keywords = closeKeywords
	case lex.Inquire:
		keywords = inquireKeywords
	case lex.Rewind, lex.Backspace, lex.EndFile, lex.Find, lex.Delete, lex.Unlock:
		keywords = nil // only ERR= and IOSTAT=
	}

	if s.Kind == lex.Delete && s.Rec != nil {
		if st.Rec = a.recordNumber(s.Rec); st.Rec == nil {
			ok = false
		}
	}

	if s.Kind == lex.Find {
		switch {
		case s.Rec == nil:
			a.report("FINDREC", s.Pos)

			ok = false
		default:
			if st.Rec = a.recordNumber(s.Rec); st.Rec == nil {
				ok = false
			}
		}
	}

	var control []*ast.IOSpec

	byCode := map[int]*FileKey{}

	for _, spec := range s.Specs {
		k, isKey := keywords[spec.Keyword]
		if !isKey {
			control = append(control, spec)

			continue
		}

		if k.later != "" {
			a.notImplForm(spec.Pos, k.later)

			ok = false

			continue
		}

		if old := byCode[k.code]; old != nil {
			a.report("KEYTWICE", spec.Pos, spec.Keyword, old.Name)

			ok = false

			continue
		}

		key, good := a.fileKeyword(spec, k)
		ok = ok && good

		if key != nil {
			byCode[k.code] = key
			st.Keys = append(st.Keys, key)
		}
	}

	io := &IO{}
	if !a.ioSpecs(io, control, path) {
		ok = false
	}

	st.Err, st.IOStat = io.Err, io.IOStat

	if s.Kind == lex.Open {
		if ok && !a.checkOpen(s, byCode) {
			ok = false
		}

		st.Keys = append(st.Keys, openDefaults(byCode, a.noF77)...)
	}

	if s.Kind == lex.Inquire && !a.inquireTarget(s, st, byCode) {
		ok = false
	}

	if !ok {
		return nil
	}

	return st
}

// inquireTarget checks that an INQUIRE asks by a unit or by a file, the two
// forms of section 9.3, and makes its unit a keyword. FORRTL takes a statement
// with both, and answers for the file (testdata/for81, #485), but the manual
// has no such form.
func (a *analyzer) inquireTarget(s *ast.AuxIOStmt, st *FileStmt, keys map[int]*FileKey) bool {
	byFile := keys[inquireKeywords["FILE"].code] != nil || keys[inquireKeywords["DEFAULTFILE"].code] != nil

	switch {
	case s.Unit != nil && byFile:
		a.report("INQUIREBOTH", s.Pos)

		return false
	case s.Unit == nil && !byFile:
		a.report("INQUIRENONE", s.Pos)

		return false
	}

	if st.Unit != nil {
		st.Keys = append([]*FileKey{{Code: codeUnit, Name: "UNIT", Value: st.Unit}}, st.Keys...)
		st.Unit = nil
	}

	return true
}

// fileAnswer analyzes a place INQUIRE stores an answer in.
func (a *analyzer) fileAnswer(spec *ast.IOSpec, k fileKeyword) (*FileKey, bool) {
	want, what := Integer, "an INTEGER"

	switch k.kind {
	case logicalAnswer:
		want, what = Logical, "a LOGICAL"
	case textAnswer:
		want, what = Character, "a CHARACTER"
	}

	v := a.place(spec.Value, false)

	switch {
	case v == nil:
		a.report("KEYPLACE", spec.Pos, spec.Keyword, what)

		return nil, false
	case v.Type().Kind == Invalid:
		return nil, false
	case v.Type().Kind != want:
		a.report("KEYPLACE", spec.Pos, spec.Keyword, what)

		return nil, false
	}

	if x, isVar := v.(*Var); isVar {
		a.checkLoopVariable(x.Sym, x.Pos)
	}

	a.noteAssigned(v)

	return &FileKey{Code: k.code, Name: spec.Keyword, Answer: v}, true
}

// fileKeyword analyzes one keyword's value.
func (a *analyzer) fileKeyword(spec *ast.IOSpec, k fileKeyword) (*FileKey, bool) {
	key := &FileKey{Code: k.code, Name: spec.Keyword}

	switch k.kind {
	case numberAnswer, logicalAnswer, textAnswer:
		return a.fileAnswer(spec, k)

	case assocKeyword:
		v := a.associatedVariable(spec.Value)
		if v == nil {
			return nil, false
		}

		key.Answer = v

		return key, true

	case procKeyword:
		// USEROPEN's routine, which the 1988 LRM's 9.1.28 says must be
		// declared EXTERNAL (or a dummy procedure, passed on).
		if n, isName := spec.Value.(*ast.Name); isName {
			if p, isProc := a.nameArg(n).(*Procedure); isProc && (p.Routine != "" || p.Dummy != nil) {
				if s := a.unit.byName[n.Name]; s != nil && s.Class != Intrinsic {
					key.Proc = p

					return key, true
				}
			}
		}

		a.report("USEROPENNAME", spec.Pos)

		return nil, false

	case flagKeyword, keysKeyword:
		// KEY='s list is the indexed program's: FTNRTL only needs to know
		// there is one (Decision 13).
		key.Number, key.Known = 1, true

		return key, true

	case numberKeyword:
		v := a.value(spec.Value)

		switch t := v.Type(); t.Kind {
		case Invalid:
			return nil, false
		case Integer, Real:
			key.Value = a.convert(v, IntegerType)
		default:
			a.report("KEYNUMBER", spec.Pos, spec.Keyword, article(t.String())+" value")

			return nil, false
		}

		return key, true

	case nameKeyword:
		var v Expr
		if n, isName := spec.Value.(*ast.Name); isName {
			v = a.name(n, true)
		} else {
			v = a.value(spec.Value)
		}

		switch t := v.Type(); {
		case t.Kind == Invalid:
			return nil, false
		case t.Kind == Character || (isArrayPlace(v) && t.Kind != Record):
			key.Text = v
		default:
			if _, isVar := v.(*Var); !isVar || t.Kind == Record {
				a.report("KEYTEXT", spec.Pos, spec.Keyword, article(t.String())+" value")

				return nil, false
			}

			// A number holding ASCII, which ends in a NUL (9.1.11).
			key.Text = v
		}

		return key, true
	}

	// A choice: known now if it is a constant, and checked here, since
	// FTNRTL would only find a mistake when the program runs.
	v := a.value(spec.Value)

	switch t := v.Type(); {
	case t.Kind == Invalid:
		return nil, false
	case t.Kind != Character:
		a.report("KEYTEXT", spec.Pos, spec.Keyword, article(t.String())+" value")

		return nil, false
	}

	c, isConst := v.(*Const)
	if !isConst {
		key.Text = v

		return key, true
	}

	n, found := k.choices[strings.ToUpper(strings.TrimRight(c.Text, " "))]
	if !found {
		a.report("KEYCHOICE", spec.Pos, c.Text, spec.Keyword)

		return nil, false
	}

	key.Number, key.Known = n, true

	return key, true
}

// associatedVariable analyzes a file's associated variable, OPEN's
// ASSOCIATEVARIABLE= or DEFINE FILE's last (section 9.1.2, appendix A.2): an
// INTEGER*2 or INTEGER*4 variable, not a dummy argument, which FTNRTL sets to
// the next record's number after each direct access statement. It is nil, the
// error reported, if e is none.
func (a *analyzer) associatedVariable(e ast.Expr) Expr {
	name := "the expression"
	if n, isName := e.(*ast.Name); isName {
		name = n.Name
	}

	v, isVar := a.place(e, false).(*Var)

	switch {
	case isVar && v.Type().Kind == Invalid:
		return nil
	case !isVar || v.Type().Kind != Integer || v.Type().Size == 1 || v.Sym.Dummy != 0:
		a.report("ASSOCVAR", e.Position(), name)

		return nil
	}

	a.noteAssigned(v)

	return v
}

// defineFile analyzes DEFINE FILE u(m,n,U,v)[,...] (appendix A.2): each file's
// unit, number of records, and record length, INTEGER values, and associated
// variable.
func (a *analyzer) defineFile(s *ast.DefineFileStmt) Stmt {
	st := &DefineFile{stmtBase: stmtBase{s}}
	ok := true

	integer := func(e ast.Expr, what string) Expr {
		v := a.value(e)

		switch v.Type().Kind {
		case Invalid:
		case Integer:
			return a.convert(v, IntegerType)
		default:
			a.report("DEFINEUNIT", e.Position(), what)
		}

		ok = false

		return nil
	}

	for _, f := range s.Files {
		def := &FileDef{
			Unit:    integer(f.Unit, "unit"),
			Records: integer(f.Records, "number of records"),
			Length:  integer(f.Length, "record length"),
			Var:     a.associatedVariable(f.Var),
		}

		if def.Var == nil {
			ok = false
		}

		st.Files = append(st.Files, def)
	}

	if !ok {
		return nil
	}

	return st
}

// checkOpen reports what FORRTL would refuse in an OPEN whose keywords are
// constants (docs/PHASE-81 - FORTRAN files.md, Part A and round 1). A value
// known only when the program runs is FTNRTL's to check, with the same rules.
func (a *analyzer) checkOpen(s *ast.AuxIOStmt, keys map[int]*FileKey) bool {
	is := func(code, want int) bool {
		k := keys[code]

		return k != nil && k.Known && k.Number == want
	}
	known := func(code int) bool {
		k := keys[code]

		return k != nil && k.Known
	}
	conflict := func(code int, what string) bool {
		a.report("OPENCONFLICT", s.Pos, keys[code].Name, what)

		return false
	}

	const (
		oldFile, newFile, scratchFile = 1, 2, 3
		direct, appendTo              = 1, 3
		formatted                     = 1
		deleteFile                    = 2
		fixed, variable, segmented    = 1, 2, 3
		relative, indexed             = 2, 3
	)

	readOnly := keys[codeReadOnly] != nil

	switch {
	case readOnly && is(codeStatus, newFile), readOnly && is(codeStatus, scratchFile):
		return conflict(codeStatus, "READONLY")
	case readOnly && (is(codeDispose, deleteFile) || is(codeDispose, 5) || is(codeDispose, 6)):
		return conflict(codeDispose, "READONLY")
	case is(codeStatus, scratchFile) && known(codeDispose) && !is(codeDispose, deleteFile):
		return conflict(codeDispose, "STATUS='SCRATCH'")
	case is(codeType, segmented) && is(codeForm, formatted):
		return conflict(codeType, "FORM='FORMATTED'")
	case is(codeAccess, direct) && known(codeType) && !is(codeType, fixed) && !(is(codeOrg, relative) && is(codeType, variable)):
		return conflict(codeType, "ACCESS='DIRECT'")
	case is(codeAccess, appendTo) && (is(codeStatus, newFile) || is(codeStatus, scratchFile)):
		return conflict(codeStatus, "ACCESS='APPEND'")
	case is(codeAccess, appendTo) && readOnly:
		return conflict(codeAccess, "READONLY")
	case keys[codeKey] != nil && known(codeOrg) && !is(codeOrg, indexed):
		return conflict(codeOrg, "KEY=")
	}

	// RECL is mandatory where a file is made with records of one size
	// (Table 9-3): an existing file has its own.
	if keys[codeRecl] == nil && !is(codeStatus, oldFile) {
		var why string

		switch {
		case is(codeAccess, direct):
			why = "ACCESS='DIRECT'"
		case is(codeOrg, relative):
			why = "ORGANIZATION='RELATIVE'"
		case is(codeType, fixed):
			why = "RECORDTYPE='FIXED'"
		}

		if why != "" {
			a.report("NEEDRECL", s.Pos, why)

			return false
		}
	}

	return true
}

// openDefaults are the keywords an OPEN gets without giving them, which
// FORRTL leaves to the compiler: STATUS='UNKNOWN' (section 9.1.25; NEW is
// what FORRTL takes with none) and BLANK='NULL' (section 9.1.4; ZERO is
// what a unit with no OPEN has). Under /NOF77 they are NEW and ZERO, those
// sections say.
func openDefaults(keys map[int]*FileKey, noF77 bool) []*FileKey {
	var added []*FileKey

	status, blank := "UNKNOWN", "NULL"
	if noF77 {
		status, blank = "NEW", "ZERO"
	}

	if keys[codeStatus] == nil {
		added = append(added, &FileKey{Code: codeStatus, Name: "STATUS", Number: statusChoices[status], Known: true})
	}

	if keys[codeBlank] == nil {
		added = append(added, &FileKey{Code: codeBlank, Name: "BLANK", Number: blankChoices[blank], Known: true})
	}

	return added
}
