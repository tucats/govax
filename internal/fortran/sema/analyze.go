package sema

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/source"
)

// Options are the qualifiers on the FORTRAN command that semantic analysis
// needs, which a unit's OPTIONS statements may override (section 1.4.3). The
// zero value is the default.
type Options struct {
	// NoI4 is /NOI4: INTEGER and LOGICAL with no length are 2 bytes, and an
	// integer constant in a word's range is an INTEGER*2 (Table 2-1).
	NoI4 bool

	// GFloating is /G_FLOATING: REAL*8 and COMPLEX*16 are in G_floating,
	// not D_floating (Decision 16 of docs/planning/PHASE-74 - FORTRAN plan.md).
	GFloating bool

	// NoF77 is /NOF77: FORTRAN-66's reading of what FORTRAN-77 changed (Program.NoF77).
	NoF77 bool

	// F90 is /F90: Fortran 90's reading of the texts it and VAX FORTRAN read
	// differently (Unit.F90).
	F90 bool

	// Recursive is /RECURSIVE: every subprogram is recursive, as if its header
	// said RECURSIVE (Unit.Recursive).
	Recursive bool

	// Align is /[NO]ALIGN: how each unit starts, which its CDEC$ OPTIONS groups
	// change (align.go).
	Align Alignment

	// Checks is /CHECK (checks.go), which a unit's OPTIONS statements may change.
	Checks Checks

	// Warnings is /WARNINGS, for the two keywords that look for something here
	// (warnings.go). No OPTIONS statement takes it (section 1.4.3).
	Warnings diag.Warnings

	// Standard is /STANDARD: what to flag as an extension to FORTRAN-77 (standard.go).
	Standard Conformance

	// Modules finds the module files of modules the compilation doesn't
	// compile (module.go); nil finds none. Version and Source are what a
	// module's text says made it: govax's version, and the source file's
	// specification and time.
	Modules ModuleFinder
	Version string
	Source  string
}

// Analyze gives the parser's tree its meaning, reporting what is wrong
// with it to messages. mainName is the name of a main program without a
// PROGRAM statement (section 4.12: the source file's name, cut to 26
// characters, with $MAIN after it).
//
// Analysis goes on after an error, a unit at a time, so that one
// compilation reports as much as it can. A unit whose header statement
// the parser rejected is left out: without its header its dummy arguments aren't known, and what
// sema said about the rest would mislead. The Program returned is
// meaningful only if no error was reported.
func Analyze(file *ast.File, mainName string, opts Options, messages *diag.List) *Program {
	prog := &Program{}
	prog.modules = newModules(file, opts)

	// First, the names of every unit in the file, so that a unit can be
	// checked as it calls another defined later in the file.
	units := unitNames(file, mainName, messages)

	// Then each unit in turn, with an analyzer of its own: every unit has
	// its own names (a name means nothing outside its unit unless it's in
	// COMMON or names a unit), and its internal procedures, each with an
	// analyzer of its own too, its names by host association (internal.go).
	for _, au := range file.Units {
		unitOpts, psects, groups := prologue(prog, au.Prologue, opts, messages)

		if _, bad := au.Header.(*ast.BadStmt); bad {
			continue
		}

		// A module, or a header in a Fortran 90 form not compiled yet
		// (INTEGER(4) FUNCTION F), is reported where it starts, and the
		// unit isn't analyzed (docs/planning/PHASE-99 - Fortran 90.md,
		// subtask 5).
		if pending := pendingHeader(au); pending != nil {
			messages.Report("NOTIMPL", pendingPosition(pending), statementWhat(pending))

			continue
		}

		defaultInt, defaultLogical := IntegerType, LogicalType
		if unitOpts.NoI4 {
			defaultInt, defaultLogical = Type{Kind: Integer, Size: 2}, Type{Kind: Logical, Size: 2}
		}

		a := &analyzer{
			messages:       messages,
			prog:           prog,
			psects:         psects,
			units:          units,
			defaultInt:     defaultInt,
			defaultLogical: defaultLogical,
			gFloating:      unitOpts.GFloating,
			noF77:          unitOpts.NoF77,
			f90:            unitOpts.F90,
			recursive:      unitOpts.Recursive,
			align:          opts.Align,
			baseAlign:      opts.Align,
			warnings:       opts.Warnings,
			standard:       opts.Standard,
			checks:         unitOpts.Checks,
			prologueGroups: groups,
		}
		a.makeMaps()

		// A module's text is made once it has been analyzed without an error,
		// for the USE statements after it (module.go).
		name := unitName(au, mainName)
		failed := messages.Count(diag.Error) + messages.Count(diag.Fatal)

		if au.Kind() == ast.ModuleUnit {
			delete(prog.modules.later, name)
			prog.modules.current = name
		}

		units := a.analyzeUnit(au, name)
		prog.Units = append(prog.Units, units...)

		if au.Kind() == ast.ModuleUnit {
			prog.modules.current = ""

			if messages.Count(diag.Error)+messages.Count(diag.Fatal) == failed {
				prog.finishModule(units[0], opts, messages)
			}

			// A module that failed has had its messages: its users say
			// nothing more of it.
			if prog.modules.compiled[name] == nil {
				prog.modules.loaded[name] = &moduleScope{name: name, failed: true}
			}
		}
	}

	// What comes after the last unit is an extension as anywhere.
	trailing := newExtensions(opts.Standard, opts.F90)
	trailing.statements(file.Trailing)
	trailing.report(messages)

	// A PSECT directive after the last unit has no unit to belong to, so no
	// block it could name.
	_, psects, groups := prologue(prog, file.Trailing, opts, messages)
	for _, s := range psects {
		b := CommonBlock{Name: s.Common}
		messages.Report("PSECTBLOCK", s.CommonPos, b.String())
	}

	for _, s := range groups {
		messages.Report("OPTIONSPLACE", s.Position(), "program unit's first")
	}

	// Last, the checks between units, which need them all analyzed: that
	// each call matches what it calls, and that COMMON blocks of one name
	// agree with each other from unit to unit.
	checkReferences(prog, units, opts.Warnings, messages)
	checkPsects(prog, messages)
	checkCommonLayouts(prog, messages)
	raiseCommonAlignments(prog)
	checkCommonInitials(prog, messages)

	return prog
}

// prologue is the options a unit is compiled with: the compilation's, as
// the OPTIONS statements before it change them, until the unit's end
// (section 1.4.3), and the PSECT directives before it, which are that unit's
// (ours, as OPTIONS statements are), and its CDEC$ OPTIONS and END OPTIONS
// directives. The qualifiers not implemented yet are reported; /ALIGN is among the
// names no OPTIONS statement takes (the author's answer 3).
func prologue(prog *Program, list []ast.Stmt, opts Options, messages *diag.List) (Options, []*ast.DirectiveStmt, []*ast.DirectiveStmt) {
	var psects, groups []*ast.DirectiveStmt

	for _, s := range list {
		switch s := s.(type) {
		case *ast.BadStmt:

		case *ast.DirectiveStmt:
			switch d := prog.directive(s, messages); {
			case d == nil:
			case d.Name == "PSECT":
				psects = append(psects, d)
			default:
				groups = append(groups, d)
			}

		case *ast.OptionsStmt:
			for _, q := range s.Qualifiers {
				switch source.OptionsQualifier(q.Name) {
				case "EXTEND_SOURCE", "NOEXTEND_SOURCE":
					// The first pass has already acted on it (source/options.go).
				case "I4":
					opts.NoI4 = false
				case "NOI4":
					opts.NoI4 = true
				case "G_FLOATING":
					opts.GFloating = true
				case "NOG_FLOATING":
					opts.GFloating = false
				case "F77":
					opts.NoF77 = false
				case "NOF77":
					opts.NoF77 = true
				case "F90":
					opts.F90 = true
				case "NOF90":
					opts.F90 = false
				case "RECURSIVE":
					opts.Recursive = true
				case "NORECURSIVE":
					opts.Recursive = false
				case "CHECK":
					checks, err := opts.Checks.With(q.Values)
					if err != nil {
						messages.Report("BADCHECK", q.Pos, err.Error())
					} else {
						opts.Checks = checks
					}
				case "NOCHECK":
					opts.Checks = Checks{}
				default:
					messages.Report("NOTIMPL", s.Position(), "the OPTIONS statement's /"+q.Name)
				}
			}

		default:
			messages.Report("NOTIMPL", s.Position(), statementWhat(s))
		}
	}

	return opts, psects, groups
}

// unitName is the name of the unit au.
func unitName(au *ast.Unit, mainName string) string {
	if name := au.Name(); name != "" {
		return name
	}

	if au.Kind() == ast.BlockDataUnit {
		return blockDataName(mainName)
	}

	return mainName
}

// blockDataName is the name of a BLOCK DATA statement without one, from the
// name of a main program without one: SQUARES$DATA in SQUARES.FOR. Section 4.12
// mentions such a name without saying what it is (ours, unconfirmed).
func blockDataName(mainName string) string {
	return strings.TrimSuffix(mainName, "$MAIN") + "$DATA"
}

// fileUnit is a program unit of the file, as another unit's references
// to it are checked against it.
type fileUnit struct {
	kind ast.UnitKind
	pos  diag.Position
	unit *Unit // after analysis; nil until then, or if it was left out

	// entry is the entry point this name is, if it is an ENTRY statement's
	// in unit, not a unit's own.
	entry *Entry
}

// params are the routine's dummy arguments.
func (fu *fileUnit) params() []*Symbol {
	if fu.entry != nil {
		return fu.entry.Params
	}

	return fu.unit.Params
}

// altPlaces are the places of the *s in the routine's dummy argument list.
func (fu *fileUnit) altPlaces() []int {
	if fu.entry != nil {
		return fu.entry.AltPlaces
	}

	return fu.unit.AltPlaces
}

// result is the function's name, whose type is the type of its value.
func (fu *fileUnit) result() *Symbol {
	if fu.entry != nil {
		return fu.entry.Sym
	}

	return fu.unit.Result
}

// unitNames collects the file's units and entry points by name, reporting
// two with one name (DUPUNIT) and a second main program (TWOMAINS).
func unitNames(file *ast.File, mainName string, messages *diag.List) map[string]*fileUnit {
	units := map[string]*fileUnit{}

	var main *ast.Unit

	// Two passes over the units: the units' own names first, then their
	// ENTRY statements' names, so that a clash between a unit and an entry
	// point is always reported at the entry point.
	for _, au := range file.Units {
		kind := au.Kind()

		if _, pending := au.Header.(*ast.PendingStmt); pending {
			continue // a header in a Fortran 90 form not compiled yet
		}

		if kind == ast.MainProgram {
			if main != nil {
				messages.Report("TWOMAINS", au.Position(), main.Position().Line)

				continue
			}

			main = au
		}

		name := unitName(au, mainName)

		if first, ok := units[name]; ok {
			messages.Report("DUPUNIT", au.Position(), name, first.pos.Line)

			continue
		}

		units[name] = &fileUnit{kind: kind, pos: au.Position()}
	}

	for _, au := range file.Units {
		kind := au.Kind()
		if kind != ast.FunctionUnit && kind != ast.SubroutineUnit {
			continue
		}

		for _, st := range au.Body {
			s, ok := st.(*ast.EntryStmt)
			if !ok {
				continue
			}

			if first, ok := units[s.Name]; ok {
				messages.Report("DUPUNIT", s.Pos, s.Name, first.pos.Line)

				continue
			}

			units[s.Name] = &fileUnit{kind: kind, pos: s.Pos}
		}
	}

	return units
}

// analyzer is semantic analysis's state for one unit. It's made fresh for
// each unit (Analyze), and almost every function in this package is one of
// its methods: they share what's known so far through its fields rather
// than passing it about. Its result is the Unit in unit, which outlives it.
//
// Analysis reads a unit's statements twice. The first walk (decl.go)
// reads the declarations, since FORTRAN lets them come in any order
// before the executable statements and an executable statement can't be
// understood until they're all known. The second (stmt.go) reads the
// executable statements, giving each expression its type. Some declarations
// (EQUIVALENCE, DATA, SAVE) can only be settled once every other is known,
// so the first walk saves them in fields here for later.
type analyzer struct {
	// readFrom is set while a place an input statement reads from (a READ's
	// internal file, DECODE's buffer) is analyzed: it isn't given a value, so
	// it may be an INTENT(IN) dummy's (sourcePlace).
	readFrom bool

	// noFill keeps arrayOrder from filling a target straight from a
	// constructor, for a WHERE's assignments, which assign only where the
	// mask holds (where.go).
	noFill bool

	// constructNames are the unit's construct names, in the order given
	// (constructs.go).
	constructNames []constructName

	// componentPointers are the symbols that stand for array pointer
	// components, by the scope and the reference to each with its subscripts
	// (pointercomp.go).
	componentPointers map[componentKey]*Symbol

	messages *diag.List
	prog     *Program
	units    map[string]*fileUnit
	unit     *Unit

	// body is the unit's statements, as the walks read them.
	body []ast.Stmt

	// host is the analyzer of an internal procedure's host, or nil
	// (internal.go). provisional are the names an internal procedure's
	// declarations entered as its own because its host hadn't used them yet,
	// which the host's executable statements may still use (checkProvisional).
	host        *analyzer
	provisional []*Symbol

	// provisionalBounds are the names of provisional among adjustable arrays'
	// bounds, which are BOUNDVAR unless the host uses them.
	provisionalBounds []provisionalBound

	// waiting are a module's interface blocks with MODULE PROCEDURE
	// statements, until its procedures are known (interface.go).
	waiting []waitingBlock

	// reading is the facts of a module's text being read for a USE
	// (modread.go), or nil when the unit is a program's own.
	reading *moduleFacts

	// respecify says a declaration may name a use-associated name: a NAMELIST
	// group's, or in a module PUBLIC and PRIVATE (use.go). storageAssoc says it
	// is in COMMON or EQUIVALENCE, for USECOMMON.
	respecify, storageAssoc bool

	// interfacing is set while an internal procedure's declarations are read,
	// before its host's executable statements.
	interfacing bool

	// inBody is set for an interface body's analyzer, whose unit is no unit
	// of the file (interface.go); interfaces are the interface blocks the
	// declarations walk found, whose bodies interfaceBodies analyzes.
	inBody     bool
	interfaces []*ast.InterfaceBlock

	// textTypes is the module whose text is being read, for a unit inside it:
	// an interface body in a module's text names the module's types, which
	// a body can't reach by host association; in the module's source its
	// body had them by USE or defined them, and the text gives them all at
	// the module's level (modwrite.go). Nil outside a module's text.
	textTypes *Unit

	// baseAlign, standard, and checks are the options the unit started with,
	// which its internal procedures start with too.
	baseAlign Alignment
	standard  Conformance
	checks    Checks

	// psects are the PSECT directives before the unit and in it, in order,
	// applied when every COMMON statement is read (directive.go).
	psects []*ast.DirectiveStmt

	// align is the alignment setting at the declaration being read, alignGroups
	// the open CDEC$ OPTIONS groups, innermost last, and prologueGroups the
	// directives before the unit (align.go). structDepth is how many structure
	// declarations are being read.
	align          Alignment
	alignGroups    []alignGroup
	prologueGroups []*ast.DirectiveStmt
	structDepth    int

	// defaultInt and defaultLogical are the types INTEGER and LOGICAL with
	// no length mean in this unit: 4 bytes, or with /NOI4 2.
	defaultInt, defaultLogical Type

	// gFloating is set when the unit's REAL*8 is G_floating (format).
	gFloating bool

	// scope is the scope the names being read are in (scope.go): the unit's,
	// or while a statement function's definition is read, its own.
	scope *scope

	// rejected are the IMPLICIT statements left out of the unit's implicit
	// rules (implicit.go).
	rejected map[ast.Stmt]bool

	// group is the group of Figure 1-1 the unit's statements have
	// reached, and firsts the first statement of each group (order.go).
	group  group
	firsts [execGroup + 1]ast.Stmt

	// stmtFuncs are the assignments that are statement function
	// definitions (statementFunction), each with its function's symbol,
	// or nil for one with an error.
	stmtFuncs map[ast.Stmt]*Symbol

	// defining is the statement function whose definition is being
	// analyzed, which may not use itself.
	defining *Symbol

	// entryOf are the unit's ENTRY statements' entry points (entry.go).
	entryOf map[*ast.EntryStmt]*Entry

	// inLibrary is set while the declarations walk reads a statement from a
	// text library's module (source.Statement.Library).
	inLibrary bool

	// inInclude is set while it reads a statement from a file INCLUDE brought in, a
	// text library's module among them (source.Statement.Depth).
	inInclude bool

	// noF77 is the unit's /NOF77.
	noF77 bool

	// f90 is the unit's /F90.
	f90 bool

	// recursive is the unit's /RECURSIVE: a subprogram is recursive whether its
	// header says so or not (Unit.Recursive).
	recursive bool

	// warnings is /WARNINGS (warnings.go).
	warnings diag.Warnings

	// extensions is what /STANDARD has found so far in this unit (standard.go).
	extensions *extensions

	// walking is set during the statement walk, when using a name is using
	// it in a statement (usedBeforeEntry).
	walking bool

	// adjustable are the arrays declared with bounds that aren't
	// constant, and boundVars the names used in those bounds, each with
	// where it was first used so (arrays.go).
	adjustable []*adjustableArray
	boundVars  map[*Symbol]diag.Position

	// equivStmts are the unit's EQUIVALENCE statements, read when every
	// declaration is known (equivalence.go).
	equivStmts []*ast.EquivalenceStmt

	// dataSets are the unit's lists of names and values, and saveStmts its
	// SAVE statements, both read when every declaration is known (data.go,
	// attributes.go).
	dataSets  []dataSet
	saveStmts []*ast.AttrStmt

	// initialized are the names Fortran 90 declarations give values with =,
	// which have SAVE (impliedSaves).
	initialized []*Symbol

	// saveAll is set by a SAVE statement with no list, which saves every
	// variable and array of the unit that can be saved (section 4.14).
	saveAll bool

	// saveAllPos is where the first such SAVE statement is, for a PURE
	// procedure's message (pure.go).
	saveAllPos diag.Position

	// associated are the unit's files' associated variables (OPEN's
	// ASSOCIATEVARIABLE=, DEFINE FILE's last), each checked for a place in
	// the frame once the storage rule has run (checkAssociated).
	associated []*Var

	// volatileStmts are the unit's VOLATILE statements, and storageStmts its
	// AUTOMATIC and STATIC statements, read when every declaration is known
	// (storage.go).
	volatileStmts, storageStmts []*ast.AttrStmt

	// open are the IF blocks and DO loops open at the statement being
	// analyzed, outermost first; nextID numbers them.
	open   []*construct
	nextID int

	// refs are the jumps to statement labels, checked when every label
	// is known.
	refs []labelRef
}

// routineRef is a CALL or a function reference, for checking against
// the routine if the file defines it.
type routineRef struct {
	sym  *Symbol
	args []Arg
	pos  diag.Position

	// altPlaces are the places of the alternate return specifiers in the
	// CALL's list, which args leaves out.
	altPlaces []int
}

// makeMaps makes the analyzer's tables, empty.
func (a *analyzer) makeMaps() {
	a.stmtFuncs = map[ast.Stmt]*Symbol{}
	a.entryOf = map[*ast.EntryStmt]*Entry{}
	a.rejected = map[ast.Stmt]bool{}
	a.boundVars = map[*Symbol]diag.Position{}
	a.extensions = newExtensions(a.standard, a.f90)
}

// analyzeUnit analyzes one unit and its internal procedures, and returns them,
// the unit first and then its internal procedures in order.
//
// A unit with internal procedures is analyzed in four steps (internal.go): its
// declarations; each internal procedure's declarations, which are its
// interface; the unit's executable statements, which can now type a reference
// to an internal function and enter the names they use; and each internal
// procedure's executable statements, which find every one of their host's names,
// those only its executable statements used among them (host association). A unit
// without internal procedures takes the same steps with nothing between them.
func (a *analyzer) analyzeUnit(au *ast.Unit, name string) []*Unit {
	a.beginUnit(au, name)

	return a.analyzeContained(au)
}

// analyzeContained is analyzeUnit's steps after the unit's declarations. A
// module's procedures may have internal procedures of their own, so each is
// analyzed by the same steps in turn (module.go); a module read from its text
// is only its procedures' interfaces, their declarations (modread.go).
func (a *analyzer) analyzeContained(au *ast.Unit) []*Unit {
	inner := a.internalInterfaces(au)
	a.moduleProcedureBlocks()
	a.resolveAccess()

	a.executablePart()
	a.checkProvisional(inner)

	units := []*Unit{a.unit}

	for _, ia := range inner {
		switch {
		case a.reading != nil:
		case a.unit.Kind == ast.ModuleUnit:
			units = append(units, ia.analyzeContained(ia.unit.AST)...)

			continue
		default:
			ia.executablePart()
			ia.finishUnit()
		}

		units = append(units, ia.unit)
	}

	a.finishUnit()
	a.checkExplicitCalls()

	return units
}

// beginUnit makes the unit's Unit and analyzes its header and declarations.
func (a *analyzer) beginUnit(au *ast.Unit, name string) *Unit {
	u := &Unit{
		AST:     au,
		Kind:    au.Kind(),
		Name:    name,
		Checks:  a.checks,
		Labels:  map[int]*Label{},
		structs: map[string]*Structure{},

		GFloating: a.gFloating,
		NoF77:     a.noF77,
		F90:       a.f90,
	}

	if a.host == nil {
		u.scope = newUnitScope(u, defaultImplicit(a.defaultInt), nil)
	} else {
		u.Host = a.host.unit
		u.scope = newUnitScope(u, a.host.scope.rules().inherit(), a.host.scope)
	}

	if u.Kind == ast.ModuleUnit {
		u.access = newModuleAccess()
	}

	a.unit = u
	a.scope = u.scope

	// An internal procedure's name is its host's, not the file's: it is no
	// unit another unit's references are checked against.
	if fu := a.units[name]; a.host == nil && !a.inBody && fu != nil && fu.unit == nil && fu.kind == u.Kind {
		fu.unit = u
	}

	a.body = au.Body
	if u.Kind == ast.BlockDataUnit {
		a.body = a.blockDataBody(a.body)
	}

	body := a.body

	u.incomplete = incompleteDeclarations(body, u.Kind == ast.ModuleUnit)

	// The IMPLICIT statements first, since they give the header's
	// names their types too.
	a.implicits(body)
	a.header(au)
	a.entries(body)
	a.uses(body)

	if u.Kind == ast.ModuleUnit {
		a.moduleStatements(body)
	}

	// Declarations first, then the executable statements, so that every
	// statement sees every declaration: a name is an array whatever
	// statement uses it. (A declaration after an executable statement is
	// an error, LATEDECL, but it still counts, so the one mistake isn't
	// reported again at each use.)
	a.applyPrologueGroups()
	a.declarations(body)
	a.resolveImplicitObjects()
	a.interfaceBodies()
	a.checkAlignGroups()
	a.checkEntryTypes()
	a.psectDirectives()
	a.checkCommonTypes()
	a.checkDefaultCommons()

	if u.Kind == ast.ModuleUnit {
		a.accessStatements(body)
		a.moduleBlock()
	}

	a.layoutCommon()
	a.equivalences()
	a.checkAllocatables()
	a.checkArrays()
	a.checkPointers()
	a.checkCommonSizes()
	a.saves()
	a.storageStatements()
	a.checkAutomatics()

	if a.reading != nil {
		a.applyLayoutFacts()
	}

	a.impliedSaves()
	a.volatiles()
	a.dataStatements()

	return u
}

// executablePart analyzes the unit's executable statements.
func (a *analyzer) executablePart() {
	a.statements(a.body)
	a.checkNamelists()
	a.checkLabels()
	a.noteLabels()
}

// finishUnit makes the checks that need every statement analyzed, and lays the
// unit's storage out.
func (a *analyzer) finishUnit() {
	a.checkTypeNames()
	a.untyped()
	a.undeclared()
	a.hostHidden()
	a.reportUnused()
	a.reportIntentOut()
	a.checkPure()
	a.checkElemental()
	a.assumedLengthFunction()
	a.reportExtensions(a.unit.AST)
	a.checkLengths()
	a.passedLengths()
	a.recursiveStorage()
	a.freedAtReturn()
	a.defaults()
	a.checkAssociated()
	a.layout()
	a.recordInitials()
	a.checkAutomaticInitials()
	a.placeInitials()
}

// header enters the names the header statement declares: the unit's
// own name and its dummy arguments.
func (a *analyzer) header(au *ast.Unit) {
	u := a.unit

	switch h := au.Header.(type) {
	case *ast.FunctionStmt:
		s := a.declare(h.Name, h.Pos)
		s.Class = Variable
		s.Result = true
		u.Result = s
		u.Recursive = h.Recursive || a.recursive
		u.Pure, u.Elemental = h.Pure || h.Elemental, h.Elemental
		a.elementalPrefixes(h.Pos, h.Name, h.Elemental, h.Recursive)

		// With RESULT the function's name is the routine's alone, and RESULT's
		// variable is the value, with the header's type (8.5.2.1). A RESULT
		// that can't be (RESULTSAME) is reported and left out.
		if h.Result != nil {
			if r := a.resultVariable(h.Result, h.Name); r != nil {
				s.Class, s.Result, s.ResultVar = UnitName, false, r
				u.Result, s = r, r
			}
		}

		if h.Type != nil && h.Type.Derived != "" {
			// TYPE(POINT) FUNCTION F: the type may be defined in the
			// function's own body (5.1.1.7), so which structure it is waits
			// for the declarations (resolveImplicitObjects). Until then the
			// value is typed, so a type statement for it is DUPDECL.
			s.Type, s.Typed, s.TypePos = Type{Kind: Record}, true, h.Pos
			u.headerType = h.Type
		} else if h.Type != nil {
			if t, ok := a.functionType(*h.Type, s); ok {
				s.Type, s.Typed, s.TypePos = t, t.Kind != Invalid, h.Pos
			} else {
				s.broken = true
			}
		}

		a.params(h.Params, &u.AltPlaces)

	case *ast.SubroutineStmt:
		s := a.declare(h.Name, h.Pos)
		s.Class = UnitName
		u.Recursive = h.Recursive || a.recursive
		u.Pure, u.Elemental = h.Pure || h.Elemental, h.Elemental
		a.elementalPrefixes(h.Pos, h.Name, h.Elemental, h.Recursive)

		a.params(h.Params, &u.AltPlaces)

	default:
		// A main program's name, whether its PROGRAM statement gave it
		// or not, names nothing else in it. No main program or block data
		// subprogram is recursive, whatever /RECURSIVE says: neither is
		// called.
		s := a.lookup(u.Name, au.Position())
		s.Class = UnitName
	}
}

// params enters the dummy arguments, and notes the place in the list of
// each * in places.
func (a *analyzer) params(params []*ast.Name, places *[]int) {
	for i, p := range params {
		if p.Name == "*" {
			a.altPlace(p, i, places, "FUNCRETURN")

			continue
		}

		s := a.declare(p.Name, p.Pos)

		switch {
		case s.Class == UnitName || s.Result:
			a.badUse(s, p.Pos, "be a dummy argument")
		case s.Dummy != 0:
			a.report("DUPDUMMY", p.Pos, p.Name, a.unit.Kind.String())
		default:
			a.unit.Params = append(a.unit.Params, s)
			a.addDummy(s)
		}
	}
}

// addDummy makes s a dummy argument of the unit.
func (a *analyzer) addDummy(s *Symbol) {
	a.unit.Dummies = append(a.unit.Dummies, s)
	s.Dummy = len(a.unit.Dummies)
}

// lookup returns the symbol named name, entering it, with its implicit
// type, if the unit hasn't used the name before; pos is where it is
// used, which is its first appearance if it is new.
func (a *analyzer) lookup(name string, pos diag.Position) *Symbol {
	if s := a.scope.find(name); s != nil {
		if a.walking {
			a.usedBeforeEntry(s, pos)
		}

		// A name two USEs gave for two entities may be there, but not
		// referenced (11.3.2).
		if s.ambiguous != "" && !s.broken {
			a.report("USEAMBIG", pos, name, s.Use.Module, s.ambiguous)
			s.broken = true
		}

		a.note(&s.refs, pos, RefUsed)

		return s
	}

	s := a.newName(name, pos)

	// An internal procedure's declarations come before its host's executable
	// statements, which may yet use the name (checkProvisional).
	if a.interfacing {
		a.provisional = append(a.provisional, s)
	}

	return s
}

// newName enters name, which no scope in the chain has, with its implicit
// type, as a name of the nearest unit's scope; pos is where it is first used.
func (a *analyzer) newName(name string, pos diag.Position) *Symbol {
	t, untyped := a.scope.rules().typeOf(name)
	s := &Symbol{Name: name, Type: t, untyped: untyped, Offset: -1, Pos: pos, library: a.inLibrary, included: a.inInclude}
	a.scope.enter(s)
	a.note(&s.refs, pos, RefUsed)

	// IMPLICIT CHARACTER*0's letter: a length of zero, kept as a type
	// statement's is (charlength.go's runtimeLength).
	if r := a.scope.rules(); !untyped && name[0] >= 'A' && name[0] <= 'Z' && r.zero[name[0]-'A'] != nil {
		a.runtimeLength(s, r.zero[name[0]-'A'])
	}

	// IMPLICIT TYPE(t)'s letter: the type may not be defined yet.
	if t.Kind == Record && t.Struct == nil {
		a.implicitObject(s)
	}

	return s
}

// describe says what s is, for BADUSE: "a variable", "the name of this
// subroutine".
func (a *analyzer) describe(s *Symbol) string {
	switch {
	case s.Internal != nil:
		return "an internal procedure"
	case s.Generic != nil && s.Interface == nil:
		return "a generic name"
	case s.Interface != nil:
		return "a procedure with an interface body"
	case s.Result:
		return "the value of this function"
	case s.Class == UnitName && a.unit.Kind == ast.MainProgram:
		return "the name of this program"
	case s.Class == UnitName && a.unit.Kind == ast.ModuleUnit:
		return "the name of this module"
	case s.Class == UnitName && a.unit.Kind == ast.BlockDataUnit:
		return "the name of this block data subprogram"
	case s.Class == UnitName && a.unit.Kind == ast.FunctionUnit:
		// A function with RESULT: its name is the routine's alone.
		return "the name of this function"
	case s.Class == UnitName:
		return "the name of this subroutine"
	case s.External:
		return External.String()
	case s.Class == Constant:
		// A named constant of a derived type is a constant first.
		return Constant.String()
	case s.Struct != nil && s.Class == Array:
		return "an array of records"
	case s.Struct != nil:
		return "a record"
	}

	return s.Class.String()
}

// badUse reports that s, being what it is, can't do what (BADUSE).
func (a *analyzer) badUse(s *Symbol, pos diag.Position, what string) {
	a.report("BADUSE", pos, s.Name, a.describe(s), what)
}

// notImplForm reports that what, a statement or a form the program wrote
// (a COMMON statement, a Hollerith constant, an implied-DO list), isn't
// implemented yet (NOTIMPL). No declaration changes that, so every unit
// gets it, whatever its declarations.
func (a *analyzer) notImplForm(pos diag.Position, what string) {
	a.messages.Report("NOTIMPL", pos, what)
}

// report reports the message ident about pos, unless the unit's
// declarations are incomplete and the message is one they could have
// prevented (see incompleteDeclarations).
func (a *analyzer) report(ident string, pos diag.Position, args ...any) {
	if a.unit.incomplete && !structural[ident] {
		return
	}

	a.messages.Report(ident, pos, args...)
}

// structural are the messages about a unit's structure (its labels,
// blocks, loops, and statement order), which no declaration changes.
var structural = map[string]bool{
	"DUPDUMMY": true, "LATEDECL": true, "ORDER": true, "MAINRETURN": true,
	"IMPLNONE": true, "DUPIMPLICIT": true,
	"NOIF": true, "NOENDIF": true, "ELSEORDER": true,
	"NODO": true, "DOTERM": true, "DONEST": true,
	"DUPLABEL": true, "UNDEFLBL": true, "NOTEXEC": true,
	"INTOBLOCK": true, "SHAREDTERM": true,
	"ENTRYUNIT": true, "ENTRYBLOCK": true, "ENTRYRETURN": true, "ENTRYLATE": true,
}

// incompleteDeclarations reports whether a unit's body holds a
// declaration this phase can't read: a specification statement that is
// not implemented yet (DICTIONARY), or one the parser
// rejected. Without it, what semantic analysis would say about the unit's
// names would mostly be wrong (a name in RECORD isn't a variable), so
// such a unit gets only the messages about its structure, and its statements'
// own NOTIMPLs.
func incompleteDeclarations(body []ast.Stmt, module bool) bool {
	for _, s := range body {
		if _, bad := s.(*ast.BadStmt); !bad && handled(s) {
			continue
		}

		// A module's PUBLIC and PRIVATE statements are access.go's.
		if _, access := s.(*ast.AccessStmt); access && module {
			continue
		}

		switch kind := s.Base().Kind; {
		case kind == lex.Format, kind == lex.Directive, kind == lex.Options, kind == lex.Unknown:
			// These declare nothing.
		case !kind.Executable():
			return true
		}
	}

	return false
}

// checkReferences checks each unit's uses of the names of the file's
// other units: a unit's name can't be a variable elsewhere in the
// program (section 1.1.3), a SUBROUTINE can't be referenced as a
// function nor a FUNCTION called, a function's references must agree
// with its type, and every reference must give as many arguments as the
// routine has dummy arguments. VAX FORTRAN compiles each unit alone and
// can't see these mistakes; a file holding both units lets us.
func checkReferences(prog *Program, units map[string]*fileUnit, warnings diag.Warnings, messages *diag.List) {
	for _, u := range prog.Units {
		for _, s := range u.Symbols {
			// A unit's own name (a DUPUNIT's among them) is reported
			// already if it is wrong.
			// A dummy procedure's name is the unit's own, whatever routine
			// it stands for when the program runs.
			fu := units[s.Name]
			if fu == nil || fu.unit == u || fu.unit == nil || s.Class == UnitName || s.Result || s.Internal != nil || s.Interface != nil || s.Use != nil || s.broken || s.onlyDeclared || s.DummyProcedure() {
				continue
			}

			// Without all of either unit's declarations, the types
			// and classes compared would mislead.
			if u.incomplete || fu.unit.incomplete {
				continue
			}

			target := fu.unit
			kind := "a " + target.Kind.String() + " in this file"

			if target.Kind == ast.MainProgram {
				kind = "the main program's name"
			}

			switch {
			case s.Class == Function && target.Kind == ast.FunctionUnit:
				// A CHARACTER*(*) function has whatever length the unit
				// referencing it declares (section 6.2.2.2).
				t := fu.result().Type
				if t == (Type{Kind: Character, Size: 0}) && s.Type.Kind == Character {
					continue
				}

				switch {
				case sameTypes(t, s.Type) || fu.result().broken:

				case t.Struct != nil && s.Type.Struct != nil && t.Struct.Derived && s.Type.Struct.Derived &&
					t.Struct.Name == s.Type.Struct.Name:
					// Two definitions of one derived type's name, as two units
					// each define it: say which is which.
					messages.Report("SEPARATEFUNC", s.Pos, s.Name, fu.pos.Line, t.Struct.Name, t.Struct.Pos.Line,
						s.Type.Struct.Pos.Line)

				default:
					messages.Report("FUNCTYPE", s.Pos, s.Name, t.String(), fu.pos.Line, s.Type.String())
				}

			case s.Class == Subroutine && target.Kind == ast.SubroutineUnit:

			case s.Class == External || s.Class == Intrinsic:
				// A name EXTERNAL makes no reference of any kind yet (a
				// BLOCK DATA unit's is meant to be just this), and an
				// INTRINSIC name is FORTRAN's, not the unit of that name.

			case s.Class == Function:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be used as a function")

			case s.Class == Subroutine:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be called")

			default:
				messages.Report("BADUSE", s.Pos, s.Name, kind, "be "+s.Class.String())
			}
		}
	}

	for _, u := range prog.Units {
		for _, b := range u.Commons {
			checkBlockName(u, b, units, messages)
		}
	}

	for _, u := range prog.Units {
		for _, r := range u.calls {
			// A reference to an internal procedure, or an internal procedure's to
			// itself, is checked against its interface (checkExplicitCalls).
			// So is one through an interface body.
			if r.sym.Internal != nil || r.sym.Interface != nil || (u.Host != nil && (r.sym.Class == UnitName || r.sym.Result)) {
				continue
			}

			fu := units[r.sym.Name]
			// A recursive unit's references to itself are checked as any other
			// unit's would be.
			if r.sym.Dummy != 0 || fu == nil || fu.unit == nil || (fu.unit == u && !u.Recursive) || fu.kind == ast.MainProgram || fu.kind == ast.BlockDataUnit || u.incomplete || fu.unit.incomplete {
				continue
			}

			// ARGCOUNT counts an alternate return as an argument, though it
			// has no place in the argument list that is built.
			n, given := len(fu.params())+len(fu.altPlaces()), len(r.args)+len(r.altPlaces)
			if n != given {
				messages.Report("ARGCOUNT", r.pos, r.sym.Name, count(n, "dummy argument"), fu.pos.Line, given)

				continue
			}

			if !slices.Equal(fu.altPlaces(), r.altPlaces) {
				messages.Report("ALTPLACE", r.pos, r.sym.Name, fu.pos.Line)

				continue
			}

			// A routine with an OPTIONAL or assumed-shape dummy has to be
			// referenced through an explicit interface (12.3.1.1); its own
			// interface is explicit in a recursive routine's reference to itself.
			// So does an ELEMENTAL procedure (elemental.go).
			if fu.unit.Elemental && fu.unit != u {
				messages.Report("EXPLICITELEMENTAL", r.pos, r.sym.Name, fu.pos.Line)

				continue
			}

			if p, kind := explicitDummy(fu.params()); p != nil && fu.unit != u {
				messages.Report("EXPLICITREQ", r.pos, r.sym.Name, kind, p.Name, fu.pos.Line)

				continue
			}

			// So does a function whose value is an array (arrayvalue.go), or
			// a pointer (pointerresult.go).
			if v := fu.result(); fu.kind == ast.FunctionUnit && v != nil && v.Pointer && fu.unit != u {
				messages.Report("EXPLICITPOINTER", r.pos, r.sym.Name, fu.pos.Line)

				continue
			}

			if v := fu.result(); fu.kind == ast.FunctionUnit && v != nil && v.Dims != nil && fu.unit != u {
				messages.Report("EXPLICITVALUE", r.pos, r.sym.Name, fu.pos.Line)

				continue
			}

			constantArguments(r, fu, messages)
			recordArguments(r, fu, messages)

			if warnings.ArgumentChecking {
				checkImplicitArguments(r, fu, messages)
			}
		}
	}

	interfaceDifferences(prog, units, messages)
}

// checkBlockName reports a common block with the name of a routine, or of
// the main program (section 1.1.3, Table 1-1; section 4.2 says so for
// functions, subroutines, and entries): those of the file's units, and the
// routines the unit that declares the block references. A variable may
// share a block's name.
func checkBlockName(u *Unit, b *CommonBlock, units map[string]*fileUnit, messages *diag.List) {
	if b.Name == "" {
		return
	}

	what := ""

	if fu := units[b.Name]; fu != nil {
		what = "a " + fu.kind.String() + " in this file"

		if fu.kind == ast.MainProgram {
			what = "the main program"
		}
	} else if s := u.Lookup(b.Name); s != nil && !u.incomplete && !s.broken {
		switch s.Class {
		case Function:
			what = "a function this unit references"
		case Subroutine:
			what = "a subroutine this unit calls"
		}
	}

	if what != "" {
		messages.Report("COMMONNAME", b.NamePos, b.String(), what)
	}
}

// count is n things: "1 subscript", "2 subscripts".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}

	return strconv.Itoa(n) + " " + thing + "s"
}
