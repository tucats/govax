package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// declarations is the first walk over a unit's statements: the type
// statements, DIMENSION statements, COMMON statements, PARAMETER
// statements, EXTERNAL and INTRINSIC statements, and structure
// declarations, in order (so that a constant may be used in the
// declarations after its definition, and a structure in the RECORD
// statements after it), the
// EQUIVALENCE, DATA, and SAVE statements, kept for when every declaration
// is known, and the order of every statement
// (Figure 1-1, order.go). It also finds the statement function
// definitions, which look like assignments to array elements and can be
// told from them only here, where it is known which names are arrays.
func (a *analyzer) declarations(body []ast.Stmt) {
	for _, s := range body {
		// Note where the statement came from: a few rules (unused names'
		// warnings, /STANDARD) don't apply to lines from an INCLUDE file
		// or a text library, which the programmer didn't write here.
		from := s.Base().Source
		a.inLibrary = from != nil && from.Library
		a.inInclude = from != nil && from.Depth > 0

		stmtFunc := false

		if st, ok := s.(*ast.AssignStmt); ok {
			stmtFunc = a.statementFunction(st)
		}

		// Check the statement comes in its proper place among the others
		// (declarations before executable statements, and so on).
		a.order(s, stmtFunc)

		// Each kind of declaration has its own method. Statements not
		// listed here (the executable ones) are left for the second walk,
		// statements in stmt.go.
		switch s := s.(type) {
		case *ast.TypeStmt:
			a.typeStatement(s)

		case *ast.DimensionStmt:
			for _, d := range s.Entities {
				a.dimension(a.declare(d.Name, d.Pos), d)
			}

		case *ast.CommonStmt:
			a.commonStatement(s)

		case *ast.EquivalenceStmt:
			a.equivStmts = append(a.equivStmts, s)

		case *ast.ParameterStmt:
			for _, d := range s.Defs {
				a.parameter(s, d)
			}

		case *ast.DataStmt:
			a.dataStatement(s)

		case *ast.AttrStmt:
			a.attributeStatement(s)

		case *ast.FormatStmt:
			a.formatStatement(s)

		case *ast.NamelistStmt:
			a.namelistStatement(s)

		case *ast.StructureStmt:
			a.structureStatement(s)

		case *ast.TypeDefStmt:
			a.typeDefinition(s)

		case *ast.RecordStmt:
			a.recordStatement(s)

		case *ast.DirectiveStmt:
			a.directive(s)

		case *ast.InterfaceBlock:
			// Its bodies are read when every other declaration is
			// (interfaceBodies).
			if compiledBlock(s) {
				a.interfaces = append(a.interfaces, s)
			}
		}
	}
}

// statementFunction reports whether the assignment s is a statement
// function's definition, f(p[,p]...) = e (section 6.2.1), and enters f
// as one if it is. It is one if it comes before the unit's executable
// statements (an assignment can't come before the specification
// statements, so the unit is in their group or the statement functions'),
// its target's name isn't an array or a dummy argument, and each
// parenthesized item is a name, a dummy argument: AVG(A, B, 3.) = ...
// is an assignment to an element of an array AVG (NOTARRAY), as F(X) =
// ... is after the first executable statement.
//
// Its expression is analyzed where the statement walk reaches it, when every
// declaration is known (stmtfunc.go).
func (a *analyzer) statementFunction(s *ast.AssignStmt) bool {
	// The target must look like F(...): a type assertion, s.Target.(*ast.Apply),
	// asks whether the interface value holds that type, and with the two-result
	// form gives ok false rather than panicking if it doesn't. a.group is how
	// far through Figure 1-1's order of statements the unit has come.
	ap, ok := s.Target.(*ast.Apply)
	if !ok || a.group > stmtFuncGroup {
		return false
	}

	if sym := a.scope.find(ap.Name); sym != nil && (sym.Class == Array || sym.Dummy != 0) {
		return false
	}

	for _, arg := range ap.Args {
		if _, ok := arg.(*ast.Name); !ok {
			return false
		}
	}

	// It is a statement function's definition. Whatever is wrong with it
	// now is reported, but it's still taken for one (true below), so that
	// it isn't also reported as a misplaced assignment. stmtFuncs maps the
	// statement to its function's symbol, or to nil after an error, so the
	// statement walk knows to skip it.
	sym := a.declare(ap.Name, ap.Pos)

	switch {
	case sym.Class == StmtFunction:
		a.report("DUPSTMTFUNC", ap.Pos, ap.Name, sym.defPos.Line)
		a.stmtFuncs[s] = nil

	case sym.Class == UnitName || sym.Result || sym.Class == Constant || sym.Common != nil || sym.External ||
		sym.Class == Intrinsic || sym.Struct != nil:
		a.badUse(sym, ap.Pos, "be a statement function")
		a.stmtFuncs[s] = nil

	default:
		sym.Class, sym.defPos = StmtFunction, ap.Pos
		a.stmtFuncs[s] = sym

		seen := map[string]bool{}

		for _, arg := range ap.Args {
			n := arg.(*ast.Name)
			if seen[n.Name] {
				a.report("DUPDUMMY", n.Pos, n.Name, "statement function")
			}

			seen[n.Name] = true
		}
	}

	return true
}

// typeStatement gives each name a type statement declares its type, and
// an array declarator's name its dimensions (sections 4.4.1, 4.4.2). A
// length after a name is that name's (INTEGER*2 IVEC*4(10) makes IVEC
// INTEGER*4); the statement's is every other name's, checked once, at
// the statement.
func (a *analyzer) typeStatement(s *ast.TypeStmt) {
	// The statement's own type (REAL*8 in REAL*8 X, Y*4) is worked out the
	// first time a name needs it, and kept: so a bad length is reported
	// once, not once per name, and not at all if every name has its own.
	var (
		stmtType   Type
		stmtLength ast.Expr
		stmtOK     bool
		checked    bool
	)

	// A Fortran 90 declaration's attributes, read against the table once
	// for the statement (attrtable.go).
	specs := a.attrSpecs(s.Attrs)

	for _, d := range s.Entities {
		// declare finds the name's symbol in the unit's table, making one
		// if this is the name's first mention.
		sym := a.declare(d.Name, d.Pos)

		t, length, ok := stmtType, stmtLength, stmtOK

		switch {
		case d.HasLength():
			t, length, ok = a.entityType(s.Type, d.Len, d.Pos)
		case !checked:
			stmtType, stmtLength, stmtOK = a.entityType(s.Type, s.Type.Len, s.Type.Pos)
			t, length, ok, checked = stmtType, stmtLength, stmtOK, true
		}

		if d.Init != nil {
			a.initialValues(d)
		}

		a.checkBoundType(sym, t, ok, d.Pos)

		// A name may be given a type only once, and not after it's been
		// made something that has none of its own (a unit's name, a
		// structure's). The default case records the type. broken marks a
		// symbol whose declaration failed, so later uses of it aren't
		// reported again and again.
		switch {
		case sym.ResultVar != nil:
			a.report("RESULTDECL", d.Pos, d.Name, sym.ResultVar.Name)
		case sym.Class == UnitName || sym.Struct != nil:
			a.badUse(sym, d.Pos, "be given a type")
		case sym.Class == Constant && sym.oldForm:
			a.report("OLDPARAMTYPE", d.Pos, d.Name, "PARAMETER statement", sym.defPos.Line)
		case sym.Class == Constant:
			a.report("TYPEAFTER", d.Pos, d.Name, sym.defPos.Line)
		case sym.Typed:
			a.report("DUPDECL", d.Pos, d.Name, sym.TypePos.Line)
		case sym.Result && t.Kind == Record:
			// Subtask 4's (docs/planning/PHASE-105 - Fortran 90 derived types.md).
			a.notImplForm(d.Pos, "a function of a derived type")

			sym.broken = true

		default:
			sym.Type, sym.Typed, sym.TypePos, sym.broken = t, true, d.Pos, sym.broken || !ok

			// TYPE(POINT) P: an object of a derived type, which is compiled
			// as a record is (derived.go).
			if t.Kind == Record {
				sym.Struct = t.Struct

				if sym.Class == Unclassified {
					sym.Class = Variable
				}
			}

			// CHARACTER(LEN=N): a length known only when the program runs
			// (charlength.go).
			if length != nil && ok {
				a.runtimeLength(sym, length)
			}
		}

		if s.Colons {
			a.f90Entity(s, sym, d, specs)

			continue
		}

		if d.Dims != nil {
			a.dimension(sym, d)
		}
	}
}

// f90Entity finishes an entity of a Fortran 90 type declaration, typed
// already: its bounds, its attributes (attrtable.go), and its value, a
// PARAMETER's (parameter.go) or an initial value (data.go). An initialized
// name has SAVE (the standard's 5.1), which impliedSaves gives it once every
// declaration is known. An array that is a PARAMETER is an array constant
// (constarray.go).
func (a *analyzer) f90Entity(s *ast.TypeStmt, sym *Symbol, d *ast.Declarator, specs []*ast.AttrSpec) {
	parameter := false

	for _, spec := range specs {
		parameter = parameter || spec.Name == "PARAMETER"
	}

	if d.Dims != nil {
		a.dimension(sym, d)
	}

	a.giveAttributes(s, sym, d, specs)

	switch {
	case sym.Struct != nil && (parameter || d.Value != nil):
		// Structure constructors are subtask 3's (docs/planning/PHASE-105 -
		// Fortran 90 derived types.md).
		pos := d.ValuePos
		if d.Value == nil {
			pos = d.Pos
		}

		a.notImplForm(pos, "a value of a derived type")

		sym.broken = true

	case parameter && d.Value == nil:
		a.report("PARAMNOVALUE", d.Pos, d.Name)

		sym.broken = true

	case parameter:
		if a.conflict(sym, attrParameter, d.Pos) {
			return
		}

		st := &ast.ParameterStmt{StmtBase: s.StmtBase}
		st.Kind = lex.Parameter
		a.defineParameter(sym, st, &ast.ParamDef{Name: &ast.Name{ExprBase: ast.ExprBase{Pos: d.Pos}, Name: d.Name}, Value: d.Value})

	case d.Value != nil:
		a.initialValue(sym, d)
	}
}

// lengths are the lengths each numeric type may be given (Table 2-1), as
// a message lists them, and the type's own default. INTEGER*1 is Fortran
// 90's INTEGER(1), and COMPLEX*32 its COMPLEX(16), which the HP manual also
// lets the old form name.
var lengths = map[ast.BaseType]struct {
	def     Type
	allowed []int
	text    string
}{
	ast.Integer: {IntegerType, []int{1, 2, 4}, "1, 2, and 4"},
	ast.Logical: {LogicalType, []int{1, 2, 4}, "1, 2, and 4"},
	ast.Real:    {RealType, []int{4, 8, 16}, "4, 8, and 16"},
	ast.Complex: {Type{Kind: Complex, Size: 8}, []int{8, 16, 32}, "8, 16, and 32"},
}

// fixedTypes are the type keywords that name one length, and so may not
// be given another (Table 2-1): BYTE is LOGICAL*1, DOUBLE PRECISION
// REAL*8, DOUBLE COMPLEX COMPLEX*16.
var fixedTypes = map[ast.BaseType]Type{
	ast.Byte:            {Kind: Logical, Size: 1},
	ast.DoublePrecision: {Kind: Real, Size: 8},
	ast.DoubleComplex:   {Kind: Complex, Size: 16},
}

// maxCharLength is the longest CHARACTER length (section 2.2.4).
const maxCharLength = 65535

// typeOf is the type spec with the length l means (Table 2-1): a numeric
// type's length must be one of its own (BADLENGTH; none for the types
// that name a length, NOLENGTH), and a CHARACTER length is characterType's.
// The length 4 (8 for COMPLEX) is the default.
//
// A Fortran 90 selector names the type by its kind (selectedType; kinds.go),
// and a CHARACTER one its length as well, which a length written after an
// entity, A*5, replaces; for a numeric type such a length replaces the kind.
//
// Only a type statement's entity may have a CHARACTER length that isn't
// constant (entityType); anywhere else (IMPLICIT, a structure's field, a
// FUNCTION statement) it is still BADCHARLEN, or for a selector's NOTIMPL.
func (a *analyzer) typeOf(spec ast.TypeSpec, l ast.Len, pos diag.Position) (Type, bool) {
	t, length, ok := a.entityType(spec, l, pos)
	if length != nil {
		if spec.Selector && l == spec.Len {
			a.notImplForm(length.Position(), "a CHARACTER length that isn't constant")
		} else {
			a.report("BADCHARLEN", length.Position())
		}

		return invalid, false
	}

	return t, ok
}

// entityType is typeOf for an entity of a type statement, whose CHARACTER
// length may be a specification expression (the Fortran 90 standard's 5.1.1.5):
// then the type is CHARACTER of length 0, as a passed length's is, and the
// expression is returned, for runtimeLength.
func (a *analyzer) entityType(spec ast.TypeSpec, l ast.Len, pos diag.Position) (Type, ast.Expr, bool) {
	if spec.Derived != "" {
		t, ok := a.derivedType(spec, l, pos)

		return t, nil, ok
	}

	if spec.Type == ast.Character {
		if spec.Selector && !a.characterKind(spec) {
			return invalid, nil, false
		}

		return a.characterType(l, pos, spec.Selector && l == spec.Len)
	}

	if spec.Selector && !l.HasLength() {
		t, ok := a.selectedType(spec)

		return t, nil, ok
	}

	if t, ok := fixedTypes[spec.Type]; ok {
		if l.HasLength() {
			a.report("NOLENGTH", pos, spec.Type.String())

			return invalid, nil, false
		}

		return t, nil, true
	}

	info, ok := lengths[spec.Type]
	if !ok {
		return invalid, nil, false
	}

	if l.LenExpr != nil || l.LenStar {
		// Only an untyped function's length can be written so, when
		// its type turns out not to be CHARACTER.
		a.report("NUMLENGTH", pos, spec.Type.String())

		return invalid, nil, false
	}

	if !l.HasLength() {
		switch spec.Type {
		case ast.Integer:
			return a.defaultInt, nil, true
		case ast.Logical:
			return a.defaultLogical, nil, true
		}

		return info.def, nil, true
	}

	for _, n := range info.allowed {
		if n == l.Length {
			return Type{Kind: info.def.Kind, Size: n}, nil, true
		}
	}

	a.report("BADLENGTH", pos, spec.Type.String(), l.Length, spec.Type.String(), info.text)

	return invalid, nil, false
}

// characterType is the CHARACTER type of length l (section 4.4.2): 1
// when none is written; a constant, or a parenthesized integer constant
// expression, from 1 to 65535 (BADCHARLEN); or (*), a passed length,
// which only some names may have (passedLengths).
//
// A length, in either form (*(N) or LEN=N), may be a specification
// expression, a dummy's N (the Fortran 90 standard's 5.1.1.5): its expression
// is returned, with the type CHARACTER of length 0, and what may have one is
// runtimeLength's to say. A constant length of zero or less, which means zero,
// is NOTIMPL, named, until Phase 108.
func (a *analyzer) characterType(l ast.Len, pos diag.Position, selector bool) (Type, ast.Expr, bool) {
	n := int64(l.Length)

	switch {
	case l.LenStar:
		return Type{Kind: Character, Size: 0}, nil, true

	case l.LenExpr != nil:
		v, kind := a.constInt(l.LenExpr)

		switch {
		case kind == constFailed:
			return invalid, nil, false
		case kind == constOK:
			n = int64(v)
		default:
			return Type{Kind: Character, Size: 0}, l.LenExpr, true
		}

		pos = l.LenExpr.Position()

		if selector && n < 1 {
			a.notImplForm(pos, "a CHARACTER length of zero")

			return invalid, nil, false
		}

	case !l.HasLength():
		n = 1
	}

	if n < 1 || n > maxCharLength {
		a.report("BADCHARLEN", pos)

		return invalid, nil, false
	}

	return Type{Kind: Character, Size: int(n)}, nil, true
}

// baseTypes are the type keywords for the kinds of type, for a length
// written after an untyped function's name (functionType).
var baseTypes = map[Kind]ast.BaseType{
	Integer: ast.Integer, Real: ast.Real, Logical: ast.Logical,
	Complex: ast.Complex, Character: ast.Character,
}

// functionType is the type of the function a FUNCTION statement with a
// type spec declares (section 6.2.2). FUNCTION F*8, a length with no
// type keyword, gives the length to the type F's first letter gives it:
// the manual's form allows the length, and says only that it must be one
// of the type's (ours, unconfirmed).
func (a *analyzer) functionType(spec ast.TypeSpec, s *Symbol) (Type, bool) {
	if spec.Type == 0 {
		if s.untyped {
			// IMPLICIT NONE: NOTYPE will say so.
			return invalid, true
		}

		spec.Type = baseTypes[s.Type.Kind]
	}

	return a.typeOf(spec, spec.Len, spec.Pos)
}

// article is text with "a" or "an" before it: "an INTEGER*2 function".
func article(text string) string {
	switch text[0] {
	case 'A', 'E', 'I', 'O', 'U', 'a', 'e', 'i', 'o', 'u':
		return "an " + text
	}

	return "a " + text
}

// constInt is the value of an integer constant expression in a
// declaration (an array bound, a CHARACTER length), which may use the
// unit's constants; names that aren't constants aren't entered in the
// symbol table as variables (constNamed).
func (a *analyzer) constInt(e ast.Expr) (int32, constKind) {
	c, err := a.declEval().eval(e)

	switch {
	case err != nil && err.kind == constNot:
		return 0, constNotInteger
	case err != nil:
		return 0, err.kind
	case c.T.Kind != Integer:
		return 0, constNotInteger
	}

	return c.Int, constOK
}
