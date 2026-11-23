package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// A module's text (finding 7; docs/planning/PHASE-106 - Fortran 90 modules.md,
// the author's answer 1). When a module compiles, what its users may reach is
// written as Fortran 90: the module's specification part as declarations, with
// every type and kind written out, every constant's value folded, every bound
// a number, then (subtask 3) CONTAINS and each module procedure's header and
// declarations without its body, which is what an interface is. What Fortran
// can't say is in comment lines that start !GOVAX$, which the compiler reads as
// facts about the declaration after them (modread.go):
//
//	!GOVAX$ MODULE FILE 1
//	!GOVAX$ COMPILER GOVAX FORTRAN V1.0-140
//	!GOVAX$ SOURCE DUA1:[WORK]CONSTS.F90;1
//	!GOVAX$ FLOATING D
//	MODULE CONSTS
//	!GOVAX$ PSECT $MOD$CONSTS SIZE 44 ALIGN 0
//	!GOVAX$ TYPE POINT SIZE 8 BOUNDARY 4 NATURAL
//	  TYPE POINT
//	    REAL(KIND=4) :: X
//	    REAL(KIND=4) :: Y
//	  END TYPE POINT
//	  REAL(KIND=4), PARAMETER :: PI = 3.14159274E+00_4
//	!GOVAX$ AT CALLS 0 4
//	  INTEGER(KIND=4) :: CALLS
//	!GOVAX$ AT TABLE 4 40
//	  REAL(KIND=4) :: TABLE(1:10)
//	END MODULE CONSTS
//
// So a reader learns what a module offers without its source, and the
// compiler reads it with its own source, lexer, parser, and semantic analysis.
// A variable's offset is the AT line's, the layout its module computed, so the
// module's /ALIGN means nothing to a user (finding 10); a type's layout is the
// TYPE line's, checked against what the reader computes from the components.
// Initial values aren't written: they are in the module's object only.
//
// A module's access is written as Fortran too: a PRIVATE statement names
// what its users can't reach, and a type whose components are private says
// PRIVATE in its definition. What the module has from another module is
// written with a FROM line, the name the text gives it, its module, and its
// name there (moduleWriter):
//
//	!GOVAX$ FROM TOP STACKS POP
//
// The text's lines are free form, at most 132 characters; a long declaration
// goes on in lines that start with &, which continue it exactly where the line
// before ended (the standard's 3.3.1.3).

// moduleFormat is the module text's form's number, its first line's.
const moduleFormat = "1"

// moduleWriter makes a module's text.
//
// The text holds every entity of the module a user can reach, and those the
// module has from other modules (finding 8): a module file is all its users
// need, whatever modules it used. Each entity it has from another module is
// written under the name the module knows it by, with a FROM line naming the
// module that defines it and its name there, which is how a reader knows it
// for the same entity it may reach by another path (use.go's origins, and
// readType for types). An entity the module's users can't reach is written
// too, in a PRIVATE statement: a public procedure's argument may be of a
// private type, and a public generic interface's specific may be private.
// The text also gives what the module can't name but its entities need: a
// generic interface's specific known only through it, a namelist group's
// item, a type of a component or argument; each under its own name, private.
type moduleWriter struct {
	u     *Unit
	lines []string

	// failed are the entities the text can't describe yet, each reported
	// NOTIMPL; a module with one makes no text.
	failed []failedEntity

	// written are the structures written so far, each once, before any that
	// mentions it; seen those the walk for types has visited.
	written map[*Structure]bool
	seen    map[*Structure]bool

	// typeNames are the names the text gives types, and names those it gives
	// entities, by origin; taken are the names the text has given.
	typeNames map[*Structure]string
	names     map[entityOrigin]string
	taken     map[string]bool

	// entities are the module's names the text gives, in order; hidden the
	// entities it gives that the module has no name for, and hiddenTypes the
	// types: both private.
	entities    []*Symbol
	hidden      []*Symbol
	hiddenTypes []*Structure
}

// failedEntity is an entity of a module its text can't describe.
type failedEntity struct {
	pos  diag.Position
	what string
}

// finishModule makes the text of the module u, analyzed without an error, and
// keeps it for the USE statements after it and the module file. What the text
// can't describe yet is reported NOTIMPL, and then there is no text.
func (p *Program) finishModule(u *Unit, opts Options, messages *diag.List) {
	w := &moduleWriter{
		u: u, written: map[*Structure]bool{}, seen: map[*Structure]bool{},
		typeNames: map[*Structure]string{}, names: map[entityOrigin]string{}, taken: map[string]bool{},
	}
	w.module(opts)

	if len(w.failed) > 0 {
		for _, f := range w.failed {
			messages.Report("NOTIMPL", f.pos, f.what)
		}

		return
	}

	mt := &ModuleText{Name: u.Name, Lines: w.lines, Digest: ModuleDigest(w.lines)}
	u.Stamp = StampName(u.Name, mt.Digest)
	p.Modules = append(p.Modules, mt)
	p.modules.compiled[u.Name] = mt
}

// ModuleDigest is the SHA-256 of a module's text, in hexadecimal, but for its
// SOURCE and COMPILER lines: two compilations of one source with nothing
// changed give one digest, whatever the files' times and whichever build of
// govax compiled them (finding 9). The FORTRAN command compares a module file's
// digest with the new text's to leave a file that hasn't changed alone, and a
// module's stamp (StampName) carries the digest's first digits.
func ModuleDigest(lines []string) string {
	h := sha256.New()

	for _, line := range lines {
		if strings.HasPrefix(line, "!GOVAX$ SOURCE ") || strings.HasPrefix(line, "!GOVAX$ COMPILER ") {
			continue
		}

		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}

	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// hasObject reports whether the module's object holds something its users
// need: a section of its own variables, or a module procedure.
func (w *moduleWriter) hasObject() bool {
	if len(w.u.Contains) > 0 {
		return true
	}

	for _, b := range w.u.Commons {
		if b.Module == w.u.Name {
			return true
		}
	}

	return false
}

// fact writes a !GOVAX$ line.
func (w *moduleWriter) fact(words ...string) {
	w.lines = append(w.lines, "!GOVAX$ "+strings.Join(words, " "))
}

// put writes a statement, indented by depth, in lines of at most 132
// characters: past that, each line ends with & and the next starts with one,
// which continues a token, even a character constant, exactly.
func (w *moduleWriter) put(depth int, text string) {
	const width = 100

	text = strings.Repeat("  ", depth) + text

	for len(text) > width+2 {
		w.lines = append(w.lines, text[:width]+"&")
		text = "&" + text[width:]
	}

	w.lines = append(w.lines, text)
}

// fail notes an entity the text can't describe, once.
func (w *moduleWriter) fail(pos diag.Position, what string) {
	for _, f := range w.failed {
		if f.pos == pos && f.what == what {
			return
		}
	}

	w.failed = append(w.failed, failedEntity{pos, what})
}

// originOf is the entity s's origin: where a USE got it from, or this module.
func (w *moduleWriter) originOf(s *Symbol) entityOrigin {
	if s.Use != nil {
		return s.Use.origin
	}

	return entityOrigin{Module: w.u.Name, Name: s.Name}
}

// nameOf is the name the text gives the entity s.
func (w *moduleWriter) nameOf(s *Symbol) string {
	if name := w.names[w.originOf(s)]; name != "" {
		return name
	}

	return s.Name
}

// from writes the FROM line of an entity the module has from another module,
// named name in the text.
func (w *moduleWriter) from(name string, s *Symbol) {
	if s.Use != nil && s.Use.origin.Module != w.u.Name {
		w.fact("FROM", name, s.Use.origin.Module, s.Use.origin.Name)
	}
}

// typeName is the name the text gives the type st.
func (w *moduleWriter) typeName(st *Structure) string {
	if name := w.typeNames[st]; name != "" {
		return name
	}

	return st.Name
}

// typeSpec is typeSpec with the text's name for a derived type.
func (w *moduleWriter) typeSpec(t Type) string {
	if t.Kind == Record && t.Struct != nil && t.Struct.Derived {
		return "TYPE(" + w.typeName(t.Struct) + ")"
	}

	return typeSpec(t)
}

// module writes the whole text.
func (w *moduleWriter) module(opts Options) {
	u := w.u

	w.prepare()

	w.fact("MODULE", "FILE", moduleFormat)
	w.fact("COMPILER", "GOVAX", "FORTRAN", "V"+opts.Version)

	if opts.Source != "" {
		w.fact("SOURCE", opts.Source)
	}

	floating := "D"
	if u.GFloating {
		floating = "G"
	}

	w.fact("FLOATING", floating)

	// OBJECT says the module's object defines what its users need: its
	// variables' section or its procedures. A user of such a module refers to
	// its stamp (StampName), so that LINK finds an object of another
	// compilation of the module out of date; a module of only types,
	// constants, and interfaces needs no object, and a user asks for none.
	if w.hasObject() {
		w.fact("OBJECT")
	}

	w.put(0, "MODULE "+u.Name)

	for _, b := range u.Commons {
		if b.Module != "" {
			w.fact("PSECT", b.Psect, "SIZE", strconv.Itoa(b.Size), "ALIGN", strconv.Itoa(b.LayoutAlign))
		}
	}

	for _, st := range u.Structures {
		w.structure(st)
	}

	for _, t := range u.usedTypes {
		w.structure(t.st)
	}

	for _, st := range w.hiddenTypes {
		w.structure(st)
	}

	for _, s := range w.all() {
		if s.Class == Constant {
			w.from(w.nameOf(s), s)
			w.constant(s)
		}
	}

	w.variables()
	w.namelists()
	w.interfaces()
	w.privateStatement()

	procs := w.usedProcedures()

	if len(u.Contains) > 0 || len(procs) > 0 {
		w.put(0, "CONTAINS")

		for _, pu := range u.Contains {
			w.fact("LINK", pu.Name, pu.LinkName)
			w.procedure(pu, 1, pu.Name)
		}

		for _, s := range procs {
			name := w.nameOf(s)
			w.fact("LINK", name, s.LinkName)
			w.from(name, s)
			w.procedure(explicitInterface(s), 1, name)
		}
	}

	w.put(0, "END MODULE "+u.Name)
}

// all are the entities the text gives: the module's names, then those it
// has no name for.
func (w *moduleWriter) all() []*Symbol {
	return append(slices.Clip(w.entities), w.hidden...)
}

// prepare decides what the text holds and the names it gives: the module's
// entities by their names, each hidden entity by its own name, if the module
// doesn't have it, and each type its local name, or its own for a type the
// module has no name for.
func (w *moduleWriter) prepare() {
	u := w.u

	for _, s := range u.Symbols {
		if u.scope.byName[s.Name] != s || s.ambiguous != "" || !useEntity(s) {
			continue
		}

		w.entities = append(w.entities, s)
		w.names[w.originOf(s)] = s.Name
	}

	for _, s := range u.Symbols {
		w.taken[s.Name] = true
	}

	for name := range u.structs {
		w.taken[name] = true
	}

	for _, n := range u.Namelists {
		w.taken[n.Name] = true
	}

	for _, h := range u.hidden {
		o := h.Use.origin

		switch {
		case w.names[o] != "":
		case w.taken[h.Name]:
			w.fail(h.Pos, "an entity of the module "+o.Module+" reached through a generic interface or a namelist group, whose name "+h.Name+" this module has for something else")
		default:
			w.names[o] = h.Name
			w.taken[h.Name] = true
			w.hidden = append(w.hidden, h)
		}
	}

	for _, name := range keysInOrder(u.structs) {
		st := u.structs[name]

		if old := w.typeNames[st]; old != "" {
			w.fail(u.AST.Position(), "a type known by two names, "+old+" and "+name+", in a module")

			continue
		}

		w.typeNames[st] = name
	}

	for _, name := range keysInOrder(u.structs) {
		w.need(u.structs[name], nil)
	}

	for _, s := range w.all() {
		w.need(symbolStruct(s), nil)

		if pu := explicitInterface(s); pu != nil {
			w.needProcedure(pu)
		}
	}
}

// need gives the type st a name in the text, if it has none, and so each type
// its fields name: a type the module has no name for is written under its own,
// private. Types in local are a procedure's own, which its interface writes.
func (w *moduleWriter) need(st *Structure, local []*Structure) {
	if st == nil || w.seen[st] || slices.Contains(local, st) {
		return
	}

	w.seen[st] = true

	if w.typeNames[st] == "" {
		switch {
		case st.Name == "":
			w.fail(st.Pos, "an unnamed structure in a module")

			return
		case w.taken[st.Name]:
			w.fail(st.Pos, "the type "+st.Name+" of the module "+st.Module+", which this module's entities need, and whose name this module has for something else")

			return
		}

		w.typeNames[st] = st.Name
		w.taken[st.Name] = true
		w.hiddenTypes = append(w.hiddenTypes, st)
	}

	for _, f := range st.Fields {
		w.need(f.Struct, local)
	}
}

// needProcedure gives the types of a procedure's arguments and value names.
func (w *moduleWriter) needProcedure(pu *Unit) {
	for _, p := range pu.Params {
		w.need(symbolStruct(p), pu.Structures)
	}

	if pu.Result != nil {
		w.need(symbolStruct(pu.Result), pu.Structures)
	}
}

// usedProcedures are the module procedures of other modules the text gives,
// written after the module's own, after CONTAINS.
func (w *moduleWriter) usedProcedures() []*Symbol {
	var list []*Symbol

	for _, s := range w.all() {
		if s.Internal == nil && isModuleProcedure(s) {
			list = append(list, s)
		}
	}

	return list
}

// privateStatement writes a PRIVATE statement naming what the module's users
// can't reach: its private entities, types, namelist groups, and operators,
// and what the text gives that the module has no name for.
func (w *moduleWriter) privateStatement() {
	u := w.u

	var names []string

	for _, s := range w.entities {
		if s.private {
			names = append(names, s.Name)
		}
	}

	for _, s := range w.hidden {
		names = append(names, w.nameOf(s))
	}

	for _, name := range keysInOrder(u.structs) {
		if u.access != nil && u.access.privateTypes[name] {
			names = append(names, name)
		}
	}

	for _, st := range w.hiddenTypes {
		names = append(names, w.typeNames[st])
	}

	for _, n := range u.Namelists {
		if n.private {
			names = append(names, n.Name)
		}
	}

	for _, key := range keysInOrder(u.operators) {
		if u.access != nil && u.access.privateOps[key] {
			names = append(names, operatorText(key))
		}
	}

	if len(names) > 0 {
		w.put(1, "PRIVATE :: "+strings.Join(names, ", "))
	}
}

// interfaces writes the module's interface blocks: one for each external
// procedure with an interface body that is no generic name's or operator's
// specific, then each generic name's and operator's, its module procedures in a
// MODULE PROCEDURE statement and its external ones' bodies in the block.
func (w *moduleWriter) interfaces() {
	u := w.u
	inGeneric := map[*Symbol]bool{}

	var generics []*Symbol

	for _, s := range w.entities {
		if s.Generic != nil {
			generics = append(generics, s)

			for _, p := range s.Generic.Specifics {
				inGeneric[p] = true
			}
		}
	}

	keys := keysInOrder(u.operators)
	for _, k := range keys {
		for _, p := range u.operators[k].Specifics {
			inGeneric[p] = true
		}
	}

	for _, s := range w.all() {
		if s.Interface != nil && s.Dummy == 0 && s.Internal == nil && !isModuleProcedure(s) && !inGeneric[s] {
			w.put(1, "INTERFACE")
			w.external(s, 2)
			w.put(1, "END INTERFACE")
		}
	}

	written := map[*Symbol]bool{}

	for _, s := range generics {
		w.genericBlock("INTERFACE "+s.Name, s.Generic.Specifics, written)
	}

	for _, k := range keys {
		head := "INTERFACE OPERATOR(" + k + ")"
		if k == assignmentKey {
			head = "INTERFACE ASSIGNMENT(=)"
		}

		w.genericBlock(head, u.operators[k].Specifics, written)
	}
}

// external writes an external procedure's interface body, with the lines
// that say where it came from and, when the module knows it by a name not
// its own, its own name.
func (w *moduleWriter) external(s *Symbol, depth int) {
	name := w.nameOf(s)

	if s.LinkName != "" {
		w.fact("LINK", name, s.LinkName)
	}

	w.from(name, s)
	w.procedure(s.Interface, depth, name)
}

// genericBlock writes one generic interface block: head, the specifics'
// bodies or MODULE PROCEDURE statement, END INTERFACE. An external specific's
// body can be written once only: one that is a specific of two generic names
// is beyond the text yet.
func (w *moduleWriter) genericBlock(head string, specifics []*Symbol, written map[*Symbol]bool) {
	w.put(1, head)

	var names []string

	for _, p := range specifics {
		switch {
		case isModuleProcedure(p):
			names = append(names, w.nameOf(p))
		case written[p]:
			w.fail(p.Pos, "an external procedure that is a specific of two generic names or operators in a module")
		case p.Interface != nil:
			written[p] = true
			w.external(p, 2)
		}
	}

	if len(names) > 0 {
		w.put(2, "MODULE PROCEDURE "+strings.Join(names, ", "))
	}

	w.put(1, "END INTERFACE")
}

// procedure writes a procedure's interface under the name name: its header,
// the constants and types of its own its declarations may name, each dummy
// argument's declaration with its attributes, a function's value's, and END
// (what an interface body holds, 12.3.2.1).
func (w *moduleWriter) procedure(pu *Unit, depth int, name string) {
	h := pu.AST.Header

	args := make([]string, 0, len(pu.Params)+len(pu.AltPlaces))
	params := pu.Params

	for i := range len(pu.Params) + len(pu.AltPlaces) {
		if slices.Contains(pu.AltPlaces, i) {
			args = append(args, "*")
		} else {
			args = append(args, params[0].Name)
			params = params[1:]
		}
	}

	prefix := ""
	if pu.Recursive {
		prefix = "RECURSIVE "
	}

	word := "SUBROUTINE"
	header := name + "(" + strings.Join(args, ", ") + ")"
	result := name

	if f, ok := h.(*ast.FunctionStmt); ok {
		word = "FUNCTION"

		if f.Result != nil {
			result = pu.Result.Name
			header += " RESULT(" + result + ")"
		}
	}

	w.put(depth, prefix+word+" "+header)

	for _, st := range pu.Structures {
		w.structure(st)
	}

	for _, s := range pu.Symbols {
		if s.Class == Constant && s.Use == nil {
			w.constant(s)
		}
	}

	for _, p := range pu.Params {
		w.dummy(p, depth+1)
	}

	if word == "FUNCTION" {
		// A pointer value is a characteristic a reference must know
		// (12.2.2): the caller gets an association, not a value.
		attrs := ""
		if pu.Result.Pointer {
			attrs = ", POINTER"
		}

		w.entity(pu.Result, depth+1, attrs, result)
	}

	w.put(depth, "END "+word+" "+name)
}

// dummy writes a dummy argument's declaration: a dummy procedure's interface
// body or EXTERNAL (with its type for a function), any other's type,
// dimensions, INTENT, OPTIONAL, TARGET, and POINTER.
func (w *moduleWriter) dummy(p *Symbol, depth int) {
	if !p.DummyProcedure() {
		attrs := ""
		if p.Intent != "" {
			attrs += ", INTENT(" + p.Intent + ")"
		}

		if p.Optional {
			attrs += ", OPTIONAL"
		}

		// Characteristics of the dummy too (12.2.1.1), which a reference must
		// know: a pointer dummy is given the actual pointer itself.
		if p.Target {
			attrs += ", TARGET"
		}

		if p.Pointer {
			attrs += ", POINTER"
		}

		w.entity(p, depth, attrs, p.Name)

		return
	}

	if p.Interface != nil {
		w.put(depth, "INTERFACE")
		w.procedure(p.Interface, depth+1, p.Interface.Name)
		w.put(depth, "END INTERFACE")
	} else {
		if p.Class == Function {
			w.entity(p, depth, "", p.Name)
		}

		w.put(depth, "EXTERNAL "+p.Name)
	}

	if p.Optional {
		w.put(depth, "OPTIONAL "+p.Name)
	}
}

// entity writes the declaration of a dummy argument or function value s,
// named name, with attributes attrs (", INTENT(IN)"): its type, its CHARACTER
// length, and its dimensions. A VAX structure's record is a RECORD statement,
// its attributes statements of their own.
func (w *moduleWriter) entity(s *Symbol, depth int, attrs, name string) {
	if s.Struct != nil && !s.Struct.Derived {
		w.put(depth, "RECORD /"+w.typeName(s.Struct)+"/ "+name+dimsText(s.Dims))

		for _, a := range strings.Split(strings.TrimPrefix(attrs, ", "), ", ") {
			if a != "" {
				w.put(depth, a+" "+name)
			}
		}

		return
	}

	spec := w.typeSpec(s.Type)
	if s.LengthExpr != nil {
		spec = "CHARACTER(LEN=" + fortranText(s.LengthExpr) + ")"
	}

	w.put(depth, spec+attrs+" :: "+name+dimsText(s.Dims))
}

// typeSpec is the type t as a Fortran 90 type specification with its kind or
// length written out: INTEGER(KIND=2), REAL(KIND=8), CHARACTER(LEN=10),
// TYPE(POINT). A VAX structure's record has no such form (RECORD /S/ is a
// statement of its own), and gives "".
func typeSpec(t Type) string {
	switch t.Kind {
	case Integer, Real, Logical, Complex:
		name := map[Kind]string{Integer: "INTEGER", Real: "REAL", Logical: "LOGICAL", Complex: "COMPLEX"}[t.Kind]

		return name + "(KIND=" + strconv.Itoa(t.KindNumber()) + ")"

	case Character:
		if t.Size == 0 {
			return "CHARACTER(LEN=*)"
		}

		return "CHARACTER(LEN=" + strconv.Itoa(t.Size) + ")"

	case Record:
		if t.Struct != nil && t.Struct.Derived {
			return "TYPE(" + t.Struct.Name + ")"
		}
	}

	return ""
}

// dimsText is an array's dimensions as a declaration writes them: (1:10, 0:3),
// a deferred or assumed shape's colons, an assumed size's *.
func dimsText(dims []Dim) string {
	if len(dims) == 0 {
		return ""
	}

	parts := make([]string, len(dims))

	for i, d := range dims {
		switch {
		case d.Deferred:
			parts[i] = ":"
		case d.AssumedShape && d.LowerExpr != nil:
			parts[i] = fortranText(d.LowerExpr) + ":"
		case d.AssumedShape:
			parts[i] = strconv.Itoa(d.Lower) + ":"
		default:
			lower := strconv.Itoa(d.Lower)
			if d.LowerExpr != nil {
				lower = fortranText(d.LowerExpr)
			}

			upper := strconv.Itoa(d.Upper)

			switch {
			case d.Assumed:
				upper = "*"
			case d.UpperExpr != nil:
				upper = fortranText(d.UpperExpr)
			}

			parts[i] = lower + ":" + upper
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// structure writes a structure or derived type, after those its fields name,
// with its layout's fact.
func (w *moduleWriter) structure(st *Structure) {
	if w.written[st] {
		return
	}

	w.written[st] = true

	if st.Name == "" {
		w.fail(st.Pos, "an unnamed structure in a module")

		return
	}

	for _, f := range st.Fields {
		if f.Struct != nil {
			w.structure(f.Struct)
		}
	}

	layout := "PACKED"
	if st.Natural {
		layout = "NATURAL"
	}

	name := w.typeName(st)

	w.fact("TYPE", name, "SIZE", strconv.Itoa(st.Size), "BOUNDARY", strconv.Itoa(st.Boundary), layout)

	if st.Module != "" && st.Module != w.u.Name {
		w.fact("FROM", name, st.Module, st.Name)
	}

	if st.Derived {
		w.put(1, "TYPE "+name)

		if st.Sequence {
			w.put(2, "SEQUENCE")
		}

		if st.PrivateComponents {
			w.put(2, "PRIVATE")
		}

		for _, f := range st.Fields {
			spec := w.typeSpec(f.Type)
			if f.Struct != nil {
				spec = w.typeSpec(recordType(f.Struct))
			}

			if f.Pointer {
				spec += ", POINTER"
			}

			if f.Rank > 0 {
				spec += ", DIMENSION(" + deferredText(f.Rank) + ")"
			}

			w.put(2, spec+" :: "+f.Name+dimsText(f.Dims)+w.componentDefault(st, f))
		}

		w.put(1, "END TYPE "+name)

		return
	}

	if len(st.Initials) > 0 {
		w.fail(st.Pos, "a structure with initial values in a module")

		return
	}

	w.put(1, "STRUCTURE /"+name+"/")

	var union *Union

	mapNo := 0

	for _, f := range st.Fields {
		if f.Union != union {
			if union != nil {
				w.put(3, "END MAP")
				w.put(2, "END UNION")
			}

			union, mapNo = f.Union, 0

			if union != nil {
				w.put(2, "UNION")
			}
		}

		if union != nil && f.Map != mapNo {
			if mapNo != 0 {
				w.put(3, "END MAP")
			}

			mapNo = f.Map
			w.put(3, "MAP")
		}

		depth := 2
		if union != nil {
			depth = 4
		}

		switch {
		case f.Name == "%FILL":
			w.put(depth, "BYTE %FILL("+strconv.Itoa(f.Size())+")")
		case f.Struct != nil:
			w.put(depth, "RECORD /"+w.typeName(f.Struct)+"/ "+f.Name+dimsText(f.Dims))
		default:
			w.put(depth, oldTypeText(f.Type)+" "+f.Name+dimsText(f.Dims))
		}
	}

	if union != nil {
		w.put(3, "END MAP")
		w.put(2, "END UNION")
	}

	w.put(1, "END STRUCTURE")
}

// componentDefault is the text after a component's name that gives its
// default value, if it has one (Fortran 95's default initialization): " =
// value", or " => NULL()" for a pointer component; "" for none.
func (w *moduleWriter) componentDefault(st *Structure, f *Field) string {
	if f.NullDefault {
		return " => NULL()"
	}

	ft := f.Type
	if f.Struct != nil {
		ft = recordType(f.Struct)
	}

	var (
		text string
		ok   bool
	)

	switch d := f.Default.(type) {
	case nil:
		return ""
	case *Const:
		text, ok = w.constText(d, ft)
	case *ArrayConst:
		text, ok = w.arrayConstText(d, ft, f.Dims)
	}

	if !ok {
		w.fail(f.Pos, "the default value of the component "+f.Name+" of "+st.Name+" in a module's text")

		return ""
	}

	return " = " + text
}

// oldTypeText is the type t as a VAX FORTRAN type statement writes it, with
// its length: INTEGER*4, CHARACTER*10, a structure's field's form.
func oldTypeText(t Type) string {
	switch t.Kind {
	case Integer:
		return "INTEGER*" + strconv.Itoa(t.Size)
	case Real:
		return "REAL*" + strconv.Itoa(t.Size)
	case Logical:
		return "LOGICAL*" + strconv.Itoa(t.Size)
	case Complex:
		return "COMPLEX*" + strconv.Itoa(t.Size)
	case Character:
		return "CHARACTER*" + strconv.Itoa(t.Size)
	}

	return t.String()
}

// constant writes a named constant with its value.
func (w *moduleWriter) constant(s *Symbol) {
	name := w.nameOf(s)
	if s.Use == nil {
		name = s.Name
	}

	var value string

	switch {
	case s.ArrayValue != nil:
		v, ok := w.arrayConstText(s.ArrayValue, s.Type, s.Dims)
		if !ok {
			w.fail(s.Pos, "the value of the constant "+s.Name+" in a module's text")

			return
		}

		value = v

	case s.Value != nil:
		v, ok := w.constText(s.Value, s.Type)
		if !ok {
			w.fail(s.Pos, "the value of the constant "+s.Name+" in a module's text")

			return
		}

		value = v

	default:
		return
	}

	spec := w.typeSpec(s.Type)
	if spec == "" {
		w.fail(s.Pos, "a constant of a structure in a module")

		return
	}

	// A REAL or COMPLEX value computed, not written, is its rounded value,
	// whose digits the reader must not take for a literal's.
	if c := s.Value; c != nil && (s.Type.Kind == Real || s.Type.Kind == Complex) && !c.literal {
		w.fact("FOLDED", name)
	}

	w.put(1, spec+", PARAMETER :: "+name+dimsText(s.Dims)+" = "+value)
}

// variables writes the module's variables, each after its AT fact, the
// module's own in its section's order, then each COMMON block's with its
// COMMON statement, then those it has from other modules, in their modules'
// sections and COMMON blocks, each with its FROM line.
func (w *moduleWriter) variables() {
	u := w.u

	for _, b := range u.Commons {
		if b.Module == "" {
			continue
		}

		for _, s := range u.Symbols {
			if s.Common == b {
				w.variable(s, s.Name)
			}
		}
	}

	for _, b := range u.Commons {
		if b.Module != "" {
			continue
		}

		w.fact("COMMON", b.Psect, "SIZE", strconv.Itoa(b.Size), "ALIGN", strconv.Itoa(b.LayoutAlign))

		for _, s := range u.Symbols {
			if s.Common == b {
				w.variable(s, s.Name)
			}
		}

		names := make([]string, len(b.Members))
		for i, s := range b.Members {
			names[i] = s.Name
		}

		w.put(1, "COMMON /"+b.Name+"/ "+strings.Join(names, ", "))
	}

	for _, b := range u.UsedBlocks {
		w.usedBlock(b)
	}
}

// usedBlock writes the variables the module has from another module's section
// or COMMON block b, which the text gives: its PSECT or COMMON line, each
// variable's FROM and AT lines and declaration, and for a COMMON block its
// statement, naming only those variables (the AT lines place them).
func (w *moduleWriter) usedBlock(b *CommonBlock) {
	var members []*Symbol

	given := map[entityOrigin]bool{}

	for _, s := range b.Members {
		o := w.originOf(s)

		if given[o] || !slices.Contains(w.all(), s) {
			continue
		}

		given[o] = true
		members = append(members, s)
	}

	if len(members) == 0 {
		return
	}

	for _, own := range w.u.Commons {
		if own.Psect == b.Psect {
			w.fail(b.NamePos, "the COMMON block /"+b.Name+"/ of both this module and a module it uses")

			return
		}
	}

	line := "PSECT"
	if b.Module == "" {
		line = "COMMON"
	}

	w.fact(line, b.Psect, "SIZE", strconv.Itoa(b.Size), "ALIGN", strconv.Itoa(b.LayoutAlign))

	names := make([]string, len(members))

	for i, s := range members {
		names[i] = w.nameOf(s)
		w.from(names[i], s)
		w.variable(s, names[i])
	}

	if b.Module == "" {
		w.put(1, "COMMON /"+b.Name+"/ "+strings.Join(names, ", "))
	}
}

// variable writes one variable's AT fact and declaration, named name.
func (w *moduleWriter) variable(s *Symbol, name string) {
	w.fact("AT", name, strconv.Itoa(s.Offset), strconv.Itoa(s.Size()))

	if s.Struct != nil && !s.Struct.Derived {
		w.put(1, "RECORD /"+w.typeName(s.Struct)+"/ "+name+dimsText(s.Dims))

		if s.Volatile {
			w.put(1, "VOLATILE "+name)
		}

		return
	}

	spec := w.typeSpec(s.Type)
	if s.LengthExpr != nil {
		w.fail(s.Pos, "a CHARACTER variable of a length not constant in a module")

		return
	}

	if s.Allocatable {
		spec += ", ALLOCATABLE"
	}

	if s.Volatile {
		spec += ", VOLATILE"
	}

	if s.Target {
		spec += ", TARGET"
	}

	if s.Pointer {
		spec += ", POINTER"
	}

	w.put(1, spec+" :: "+name+dimsText(s.Dims))
}

// namelists writes the module's NAMELIST groups, each it has from another
// module with its FROM line.
func (w *moduleWriter) namelists() {
	for _, n := range w.u.Namelists {
		if n.ambiguous != "" {
			continue
		}

		if o := n.origin; o.Module != "" && o.Module != w.u.Name {
			w.fact("FROM", n.Name, o.Module, o.Name)
		}

		names := make([]string, len(n.Items))
		for i, s := range n.Items {
			names[i] = w.nameOf(s)
		}

		w.put(1, "NAMELIST /"+n.Name+"/ "+strings.Join(names, ", "))
	}
}

// constText is the constant c, of type t, as Fortran 90 writes it so that
// reading it back gives the same value: an INTEGER or LOGICAL with its kind, a
// REAL in as many digits as bring back its bits exactly, a COMPLEX as its two
// parts, a CHARACTER constant between apostrophes, with CHAR(n) for a
// character that can't be in one, a derived type's by its constructor. It
// reports false for a value it can't write (a typeless constant's bits that
// are no number).
func (w *moduleWriter) constText(c *Const, t Type) (string, bool) {
	kind := "_" + strconv.Itoa(t.KindNumber())

	switch t.Kind {
	case Integer:
		if c.Int == -2147483648 {
			return "(-2147483647_4-1)", true
		}

		return strconv.Itoa(int(c.Int)) + kind, true

	case Logical:
		if c.Int&1 != 0 {
			return ".TRUE." + kind, true
		}

		return ".FALSE." + kind, true

	case Real:
		if c.Raw != nil && !rawMatches(c.Raw, c.Real, t) {
			return "", false
		}

		if c.literal {
			return literalText(c.exactRe, t), true
		}

		return realText(c.Real, t)

	case Complex:
		part := Type{Kind: Real, Size: t.Size / 2}

		if c.Raw != nil {
			return "", false
		}

		re, ok1 := realText(c.Real, part)
		im, ok2 := realText(c.Imag, part)

		if c.literal {
			re, im, ok1, ok2 = literalText(c.exactRe, part), literalText(c.exactIm, part), true, true
		}

		return "(" + re + ", " + im + ")", ok1 && ok2

	case Character:
		return characterText(c.Text), true

	case Record:
		st := t.Struct
		if st == nil || !st.Derived || len(c.Parts) != len(st.Fields) {
			return "", false
		}

		parts := make([]string, len(c.Parts))

		for i, p := range c.Parts {
			f := st.Fields[i]
			ft := f.Type

			if f.Struct != nil {
				ft = recordType(f.Struct)
			}

			var (
				text string
				ok   bool
			)

			switch p := p.(type) {
			case *Const:
				text, ok = w.constText(p, ft)
			case *ArrayConst:
				text, ok = w.arrayConstText(p, ft, f.Dims)
			}

			if !ok {
				return "", false
			}

			parts[i] = text
		}

		return w.typeName(st) + "(" + strings.Join(parts, ", ") + ")", true
	}

	return "", false
}

// rawMatches reports whether a REAL constant's bytes are its value's.
func rawMatches(raw []byte, v vaxfloat.Value, t Type) bool {
	f := realFormatOf(t)

	return string(floatBytes(f, v)) == string(raw)
}

// realFormatOf is the floating format of a REAL type of the text: REAL(8) is
// written in decimal, which the reader rounds to its module's format, so D
// here is only a stand-in for 8 bytes' digits; realText checks the module's
// own format.
func realFormatOf(t Type) vaxfloat.Format {
	switch t.Size {
	case 8:
		return vaxfloat.D
	case 16:
		return vaxfloat.H
	}

	return vaxfloat.F
}

// realText is a REAL value of type t in decimal, with its kind: the fewest
// digits that round back to the same value in every format of its size (D and
// G for REAL(8)), so the reader, which rounds to its module's format, gets the
// same bits.
func realText(v vaxfloat.Value, t Type) (string, bool) {
	formats := []vaxfloat.Format{realFormatOf(t)}
	if t.Size == 8 {
		formats = []vaxfloat.Format{vaxfloat.D, vaxfloat.G}
	}

	kind := "_" + strconv.Itoa(t.Size)

	for prec := 1; prec <= 40; prec++ {
		text := v.Text('e', prec)

		back, err := vaxfloat.Parse(text)
		if err != nil {
			return "", false
		}

		same := true

		for _, f := range formats {
			want, err1 := vaxfloat.Round(f, v)
			got, err2 := vaxfloat.Round(f, back)

			if err1 == nil && (err2 != nil || got.Cmp(want) != 0) {
				same = false
			}
		}

		if same {
			return strings.ToUpper(text) + kind, true
		}
	}

	return "", false
}

// literalText is a REAL literal's exact value as it was written, in the fewest
// digits that read back as the same exact value, with its kind: a literal keeps
// its digits' extra precision in an operation of a higher one (section
// 2.3.1.2), so the text gives its digits, not its rounded value's.
func literalText(exact vaxfloat.Value, t Type) string {
	return strings.ToUpper(exact.Text('e', -1)) + "_" + strconv.Itoa(t.Size)
}

// characterText is a character constant's text: between apostrophes, each
// apostrophe doubled, a character outside printing ASCII written CHAR(n) and
// joined by //.
func characterText(s string) string {
	if s == "" {
		return "''"
	}

	var parts []string

	var run strings.Builder

	flush := func() {
		if run.Len() > 0 {
			parts = append(parts, "'"+run.String()+"'")
			run.Reset()
		}
	}

	for i := range len(s) {
		c := s[i]

		switch {
		case c == '\'':
			run.WriteString("''")
		case c >= ' ' && c <= '~':
			run.WriteByte(c)
		default:
			flush()
			parts = append(parts, "CHAR("+strconv.Itoa(int(c))+")")
		}
	}

	flush()

	return strings.Join(parts, "//")
}

// arrayConstText is an array constant of element type t and dimensions dims as
// a constructor, under RESHAPE for more than one dimension.
func (w *moduleWriter) arrayConstText(c *ArrayConst, t Type, dims []Dim) (string, bool) {
	elems := make([]string, len(c.Elems))

	for i, e := range c.Elems {
		text, ok := w.constText(e, t)
		if !ok {
			return "", false
		}

		elems[i] = text
	}

	// An empty constructor would need its type, which Fortran 90 can't
	// write; but a zero-sized constant's declared shape, dimsText's (1:0),
	// takes a scalar of its type for its no elements (parameter.go), so the
	// value is a zero of that type.
	if len(elems) == 0 {
		return w.constText(ZeroConst(t, diag.Position{}), t)
	}

	text := "(/ " + strings.Join(elems, ", ") + " /)"

	if len(dims) <= 1 {
		return text, true
	}

	extents := make([]string, len(dims))
	for i, d := range dims {
		extents[i] = strconv.Itoa(d.Extent())
	}

	return "RESHAPE(" + text + ", (/ " + strings.Join(extents, ", ") + " /))", true
}

// fortranText is a specification expression written back as Fortran from the
// syntax tree (finding 7): an adjustable bound or a length a module
// procedure's dummy arguments use. Every operation is parenthesized, so the
// text means what the tree does whatever the operators' precedence.
func fortranText(e ast.Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case *ast.IntConst:
		if e.Radix == 8 {
			return `"` + e.Text
		}

		return e.Text
	case *ast.RealConst:
		return e.Text
	case *ast.LogicalConst:
		if e.Value {
			return ".TRUE."
		}

		return ".FALSE."
	case *ast.CharConst:
		return characterText(e.Value)
	case *ast.KindConst:
		return fortranText(e.X) + "_" + e.Kind
	case *ast.ComplexConst:
		return "(" + fortranText(e.Re) + ", " + fortranText(e.Im) + ")"
	case *ast.Name:
		return e.Name
	case *ast.Apply:
		return e.Name + "(" + fortranListText(e.Args) + ")"
	case *ast.Unary:
		return "(" + e.Op.String() + fortranText(e.X) + ")"
	case *ast.Binary:
		return "(" + fortranText(e.X) + e.Op.String() + fortranText(e.Y) + ")"
	case *ast.DefinedOp:
		if e.Y == nil {
			return "(" + e.Op + " " + fortranText(e.X) + ")"
		}

		return "(" + fortranText(e.X) + " " + e.Op + " " + fortranText(e.Y) + ")"
	case *ast.Paren:
		return "(" + fortranText(e.X) + ")"
	case *ast.Substring:
		return fortranText(e.X) + "(" + fortranText(e.Lower) + ":" + fortranText(e.Upper) + ")"
	case *ast.FieldRef:
		text := fortranText(e.X) + "%" + e.Field
		if e.Args != nil {
			text += "(" + fortranListText(e.Args) + ")"
		}

		return text
	case *ast.KeywordArg:
		return e.Name + "=" + fortranText(e.X)
	case *ast.Triplet:
		text := fortranText(e.Lower) + ":" + fortranText(e.Upper)
		if e.Stride != nil {
			text += ":" + fortranText(e.Stride)
		}

		return text
	case *ast.ArrayConstructor:
		return "(/ " + fortranListText(e.Values) + " /)"
	}

	return ast.DumpExpr(e)
}

// fortranListText is a list of expressions, separated by commas.
func fortranListText(list []ast.Expr) string {
	parts := make([]string, len(list))
	for i, e := range list {
		parts[i] = fortranText(e)
	}

	return strings.Join(parts, ", ")
}
