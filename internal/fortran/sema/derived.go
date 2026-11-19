package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Fortran 90's derived types (the standard's 4.4; docs/planning/PHASE-105 -
// Fortran 90 derived types.md). A derived type is a Structure (structure.go)
// with the Derived mark: STRUCTURE and TYPE are two front doors to one house.
//
//	TYPE POINT              STRUCTURE /POINT/
//	  SEQUENCE                REAL X, Y
//	  REAL :: X, Y          END STRUCTURE
//	END TYPE POINT
//
//	TYPE(POINT) :: P        RECORD /POINT/ P
//	P%X = 1.0               P.X = 1.0
//
// Both make a Structure of two REAL fields, 8 bytes, and an object of it is a
// Symbol whose Type is of the kind Record and names the structure (its
// Struct). From there a record and a derived type's object are compiled by
// the same code: field references (record.go), moves, arguments, and the
// storage map. What differs is checked at the doors:
//
//   - a definition holds only component declarations (type statements whose
//     only attribute is DIMENSION, or POINTER, which is Phase 107's), SEQUENCE,
//     and PRIVATE (a module's, Phase 106), never a UNION, a MAP, %FILL, a RECORD
//     statement, or a nested definition (typeDefinition);
//   - a type's name is a local name of class 1 (14.1.2): no variable,
//     constant, or procedure of its unit may have it (checkTypeNames), and it
//     can't be an intrinsic type's name;
//   - TYPE(t) names only a derived type, and RECORD /s/ only a structure;
//   - the layout: a SEQUENCE type's components are placed as a structure's
//     fields are, packed unless /ALIGN=RECORDS=NATURAL, but a type without
//     SEQUENCE is naturally aligned unless /NOALIGN or /ALIGN=RECORDS=PACKED
//     was given explicitly (the author's answer 1; derivedLayout).
//
// IMPLICIT TYPE(t) (letters) may name a type its unit defines after the
// IMPLICIT statement, as the standard's statement order has it, so a name
// given such a type by its letter waits until the declarations are read
// (implicitObject, resolveImplicitObjects).

// derivedLayout reports whether a derived type is naturally aligned:
// SEQUENCE by the setting where it is defined, as a STRUCTURE; any other
// unless the setting says PACKED explicitly.
func (a *analyzer) derivedLayout(sequence bool) bool {
	if sequence {
		return a.align.Records == Natural
	}

	return !a.align.RecordsGiven || a.align.Records != Packed
}

// typeDefinition reads a derived type's definition (R422 to R429) into a
// Structure, entered in the unit's table of structures under its name.
func (a *analyzer) typeDefinition(s *ast.TypeDefStmt) {
	a.noteOther(whatType, s.Name, s.NamePos, RefDefined)

	switch {
	case ast.TypeNames[s.Name] != 0:
		a.report("TYPENAME", s.NamePos, s.Name)
	case s.Access != "" && a.unit.Kind != ast.ModuleUnit:
		a.report("TYPEACCESS", s.AccessPos, s.Access)
	}

	st := &Structure{Name: s.Name, Pos: s.NamePos, byName: map[string]*Field{}, open: true, Derived: true}

	if old := a.unit.structs[s.Name]; old != nil {
		a.report("DUPTYPE", s.NamePos, s.Name, old.Pos.Line)
	} else {
		a.unit.structs[s.Name] = st
	}

	a.unit.Structures = append(a.unit.Structures, st)

	st.Sequence = a.typeHeader(s)
	st.Natural = a.derivedLayout(st.Sequence)
	b := &structBuilder{a: a, s: st, boundary: 1}

	a.structDepth++

	for _, c := range s.Body {
		switch c := c.(type) {
		case *ast.TypeStmt:
			b.declares = true
			b.componentStatement(c)

		case *ast.BadStmt:
			b.declares = true

		case *ast.SequenceStmt, *ast.EndTypeStmt:
			// typeHeader's and endType's.

		case *ast.AccessStmt:
			if c.Access != "PRIVATE" || len(c.Items) > 0 {
				a.report("TYPESTMT", c.Position(), aStatement(c))
			}

		case *ast.DirectiveStmt:
			a.directive(c)

		default:
			a.report("TYPESTMT", c.Position(), aStatement(c))
		}
	}

	a.structDepth--

	st.Size, st.Boundary, st.open = b.offset, b.boundary, false

	if st.Natural {
		st.Size = alignUp(st.Size, st.Boundary)
	}

	a.warnFields(st)

	if !b.declares {
		a.report("TYPEEMPTY", s.NamePos, s.Name)
	}

	if st.Sequence {
		for _, f := range st.Fields {
			if f.Struct != nil && !f.Struct.Sequence {
				a.report("SEQCOMPONENT", f.Pos, s.Name, f.Name, f.Struct.Name)
			}
		}
	}

	a.endType(s)
}

// typeHeader checks a definition's SEQUENCE and PRIVATE statements, which come
// once each before its components (R422), and reports whether it is SEQUENCE.
// PRIVATE is allowed only in a module (Phase 106).
func (a *analyzer) typeHeader(s *ast.TypeDefStmt) bool {
	var sequence, private ast.Stmt

	components := false

	for _, c := range s.Body {
		var seen *ast.Stmt

		switch c := c.(type) {
		case *ast.TypeStmt, *ast.BadStmt:
			components = true

			continue

		case *ast.SequenceStmt:
			seen = &sequence

		case *ast.AccessStmt:
			if c.Access != "PRIVATE" || len(c.Items) > 0 {
				continue
			}

			seen = &private

			if a.unit.Kind != ast.ModuleUnit {
				a.report("TYPEACCESS", c.Position(), "PRIVATE")
			}

		default:
			continue
		}

		switch {
		case *seen != nil:
			a.report("TYPETWICE", c.Position(), statementName(c), s.Name, (*seen).Position().Line)
		case components:
			a.report("TYPEORDER", c.Position(), statementName(c))
		default:
			*seen = c
		}
	}

	return sequence != nil
}

// endType checks the name on a definition's END TYPE, which must be the type's
// if it is given (R429).
func (a *analyzer) endType(s *ast.TypeDefStmt) {
	if len(s.Body) == 0 {
		return
	}

	if e, ok := s.Body[len(s.Body)-1].(*ast.EndTypeStmt); ok && e.Name != "" && e.Name != s.Name {
		a.report("ENDTYPENAME", e.NamePos, e.Name, s.Name)
	}
}

// componentStatement declares a component for each entity of a type statement
// in a definition (R425): an intrinsic type's, or another derived type's
// already defined, with constant bounds and lengths. The statement's only
// attribute may be DIMENSION, whose bounds an entity without its own takes;
// POINTER is Phase 107's. A component's default initial value is Fortran 95's
// (the author's answer 3).
func (b *structBuilder) componentStatement(s *ast.TypeStmt) {
	a := b.a

	var dims []ast.Bound

	for _, at := range s.Attrs {
		switch at.Name {
		case "DIMENSION":
			dims = at.Dims
		case "POINTER":
			a.notImplForm(at.Pos, "a POINTER component")

			return
		default:
			a.report("TYPEATTR", at.Pos, at.Name)
		}
	}

	for _, d := range s.Entities {
		if d.Value != nil || d.Init != nil {
			pos := d.ValuePos
			if d.Init != nil {
				pos = d.InitPos
			}

			a.notImplForm(pos, "a component's default initial value")
		}

		f := &Field{Name: d.Name, Pos: d.Pos}

		var ok bool

		if s.Type.Derived != "" {
			f.Struct, ok = b.componentType(s.Type, d)
		} else {
			f.Type, ok = b.componentIntrinsic(s, d)
		}

		decl := d
		if d.Dims == nil && dims != nil {
			copied := *d
			copied.Dims = dims
			decl = &copied
		}

		if !ok || !b.fieldDims(f, decl) {
			f.Struct, f.Type, f.Dims = nil, invalid, nil
		}

		b.add(f)
	}
}

// componentIntrinsic is the type of the component d of an intrinsic type, as a
// structure's field's: a CHARACTER one's length must be constant, and not *.
func (b *structBuilder) componentIntrinsic(s *ast.TypeStmt, d *ast.Declarator) (Type, bool) {
	l, pos := s.Type.Len, s.Type.Pos
	if d.HasLength() {
		l, pos = d.Len, d.Pos
	}

	t, ok := b.a.typeOf(s.Type, l, pos)
	if ok && t.Kind == Character && t.Size == 0 {
		b.a.report("FIELDLEN", d.Pos, d.Name)

		return invalid, false
	}

	return t, ok
}

// componentType is the derived type of a component d of that type, which must
// be defined already, and not be the type being defined (that only a POINTER
// component may name, Phase 107).
func (b *structBuilder) componentType(spec ast.TypeSpec, d *ast.Declarator) (*Structure, bool) {
	if d.HasLength() {
		b.a.report("NOLENGTH", d.Pos, spec.Text())

		return nil, false
	}

	if spec.Derived == b.s.Name {
		b.a.report("TYPESELF", spec.DerivedPos, spec.Derived)

		return nil, false
	}

	st, ok := b.a.derivedStructure(spec.Derived, spec.DerivedPos)

	return st, ok
}

// derivedStructure is the derived type a TYPE(name) names: one defined before
// it in its unit or a host (an interface body has no host), never a STRUCTURE.
func (a *analyzer) derivedStructure(name string, pos diag.Position) (*Structure, bool) {
	st := a.unit.structure(name)

	switch {
	case st == nil:
		a.report("NOTYPEDEF", pos, name)

		return nil, false

	case !st.Derived:
		a.report("TYPESTRUCT", pos, name)

		return nil, false

	case st.open:
		a.report("TYPESELF", pos, name)

		return nil, false
	}

	a.noteOther(whatType, name, pos, RefUsed)

	return st, true
}

// derivedType is the type a TYPE(name) declaration gives its entities, which
// have no length of their own.
func (a *analyzer) derivedType(spec ast.TypeSpec, l ast.Len, pos diag.Position) (Type, bool) {
	if l.HasLength() {
		a.report("NOLENGTH", pos, spec.Text())

		return invalid, false
	}

	st, ok := a.derivedStructure(spec.Derived, spec.DerivedPos)
	if !ok {
		return invalid, false
	}

	return recordType(st), true
}

// implicitObject gives s, a name entered with IMPLICIT TYPE(t)'s type by its
// first letter, that type: at once if the unit's declarations have been read,
// else when they have (resolveImplicitObjects), since the type may be defined
// after the IMPLICIT statement.
func (a *analyzer) implicitObject(s *Symbol) {
	if !a.unit.typesKnown {
		a.unit.pendingObjects = append(a.unit.pendingObjects, s)

		return
	}

	r := a.scope.rules()
	c := s.Name[0] - 'A'
	name, pos := r.derived[c], r.derivedPos[c]

	if a.unit.implicitFailed[name] {
		s.Type, s.broken = invalid, true

		return
	}

	st, ok := a.derivedStructure(name, pos)
	if !ok {
		if a.unit.implicitFailed == nil {
			a.unit.implicitFailed = map[string]bool{}
		}

		a.unit.implicitFailed[name] = true
		s.Type, s.broken = invalid, true

		return
	}

	s.Type, s.Struct = recordType(st), st
}

// resolveImplicitObjects gives the names entered while the declarations were
// read their IMPLICIT TYPE(t) types, and a TYPE(t) FUNCTION statement's value
// its type, now that every type is defined. A name a declaration has typed
// since keeps that type.
func (a *analyzer) resolveImplicitObjects() {
	a.unit.typesKnown = true

	// TYPE(POINT) FUNCTION F's type (header).
	if spec := a.unit.headerType; spec != nil {
		s := a.unit.Result

		if st, ok := a.derivedStructure(spec.Derived, spec.DerivedPos); ok {
			s.Type, s.Struct = recordType(st), st
		} else {
			s.Type, s.broken = invalid, true
		}
	}

	for _, s := range a.unit.pendingObjects {
		if !s.Typed && s.Struct == nil && s.Type.Kind == Record {
			a.implicitObject(s)
		}
	}

	a.unit.pendingObjects = nil
}

// checkTypeNames reports each name of the unit's derived types that also
// names something else in the unit (14.1.2: a type's name is a local name of
// class 1, as a variable's is): a variable, a constant, a procedure, the unit
// itself.
func (a *analyzer) checkTypeNames() {
	for _, st := range a.unit.Structures {
		if !st.Derived || a.unit.structs[st.Name] != st {
			continue
		}

		if s := a.unit.scope.byName[st.Name]; s != nil && !s.broken {
			a.report("TYPECLASH", s.Pos, st.Name, st.Pos.Line, a.describe(s))
		}
	}
}
