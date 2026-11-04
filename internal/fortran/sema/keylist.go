package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// An indexed file's keys (section 9.1.15). OPEN's KEY= names each key by the
// bytes it occupies in a record, its type, and its direction:
//
//	KEY=(1:10, 11:14:INTEGER:DESCENDING)
//
// A key's place in the list is its number, the one KEYID= gives later: the
// first is the primary key, 0.

// maxOpenKeys is how many keys one OPEN can list. The manual gives the number
// and leaves it at that: "individual OPEN statements only allow up to 85 key
// definitions". A file with more keys is made some other way, with FDL.
const maxOpenKeys = 85

// KeyDef is one key of OPEN's KEY=: its first and last bytes, numbered from 1,
// each an INTEGER*4 expression whose value may not be known until the program
// runs; and whether it is an INTEGER key rather than a CHARACTER one, and
// DESCENDING rather than ASCENDING.
type KeyDef struct {
	First, Last Expr
	Integer     bool
	Descending  bool
}

// keyList analyzes KEY='s list. What it can tell now it checks now, by the
// section's rules; bounds not known until the program runs are FTNRTL's to
// check. It returns false if it reported anything.
func (a *analyzer) keyList(spec *ast.IOSpec) ([]*KeyDef, bool) {
	list := spec.Value.(*ast.KeyList)
	ok := true

	if len(list.Keys) > maxOpenKeys {
		a.report("KEYLISTLONG", spec.Pos, maxOpenKeys, len(list.Keys))

		ok = false
	}

	defs := make([]*KeyDef, 0, len(list.Keys))

	for _, field := range list.Keys {
		def := &KeyDef{}

		switch field.Type {
		case "", "CHARACTER":
		case "INTEGER":
			def.Integer = true
		default:
			a.report("KEYFIELDTYPE", field.Pos, field.Type)

			ok = false
		}

		switch field.Dir {
		case "", "ASCENDING":
		case "DESCENDING":
			def.Descending = true
		default:
			a.report("KEYFIELDDIR", field.Pos, field.Dir)

			ok = false
		}

		def.First = a.keyBound(field.Start, spec.Keyword)
		def.Last = a.keyBound(field.End, spec.Keyword)

		if def.First == nil || def.Last == nil || !a.keyFits(field, def) {
			ok = false
		}

		defs = append(defs, def)
	}

	return defs, ok
}

// keyBound is one of a key's two byte positions, a number converted to
// INTEGER*4 as OPEN's other numbers are; nil if it is none.
func (a *analyzer) keyBound(e ast.Expr, keyword string) Expr {
	v := a.value(e)

	switch t := v.Type(); t.Kind {
	case Invalid:
		return nil
	case Integer, Real:
		return a.convert(v, IntegerType)
	default:
		a.report("KEYNUMBER", e.Position(), keyword, article(t.String())+" value")

		return nil
	}
}

// keyFits checks a key whose bounds are both constants against the section's
// rules: 1 <= e1 <= e2, at most 255 bytes, and an INTEGER key 2 or 4 bytes
// long. (e2's other limit, the record's length, depends on RECL's units, which
// depend on FORM=, so it waits for FTNRTL.)
func (a *analyzer) keyFits(field *ast.KeyField, def *KeyDef) bool {
	first, isConst := def.First.(*Const)
	if !isConst {
		return true
	}

	last, isConst := def.Last.(*Const)
	if !isConst {
		return true
	}

	length := int(last.Int) - int(first.Int) + 1

	switch {
	case first.Int < 1 || length < 1:
		a.report("KEYBOUNDS", field.Pos, first.Int, last.Int)
	case length > 255:
		a.report("KEYLENGTH", field.Pos, first.Int, last.Int, length)
	case def.Integer && length != 2 && length != 4:
		a.report("KEYINTEGER", field.Pos, first.Int, last.Int, length)
	default:
		return true
	}

	return false
}
