package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Structure declarations (section 4.15) are the one place FORTRAN's
// statements nest: STRUCTURE ... END STRUCTURE holds field declarations
// (type statements, RECORD, PARAMETER), nested structures, and unions;
// UNION ... END UNION holds only maps; MAP ... END MAP holds what a
// structure does. A type statement inside one declares a field, not a
// variable, so the parser gathers the statements into their declaration
// (its Body) rather than leaving them in the unit's flat list, where
// semantic analysis would take them for declarations of variables.
//
// blockStack is the declarations open at the current statement,
// innermost last.
type blockStack struct {
	messages *diag.List
	open     []openBlock
}

// openBlock is one open declaration: the statement that opened it, its
// kind (Structure, Union, or Map), and the body its statements go into.
type openBlock struct {
	stmt ast.Stmt
	kind lex.Kind
	body *[]ast.Stmt

	// quiet is set for a declaration already reported as misplaced,
	// which is opened only so that its END matches it.
	quiet bool
}

// blockEnds is the statement that ends each kind of declaration.
var blockEnds = map[lex.Kind]lex.Kind{
	lex.Structure: lex.EndStructure,
	lex.Union:     lex.EndUnion,
	lex.Map:       lex.EndMap,
}

// isBlockEnd reports whether k ends a declaration.
func isBlockEnd(k lex.Kind) bool {
	return k == lex.EndStructure || k == lex.EndUnion || k == lex.EndMap
}

// take puts s into the innermost open declaration and reports true, or
// reports false if no declaration is open.
func (b *blockStack) take(s ast.Stmt) bool {
	if len(b.open) == 0 {
		return false
	}

	top := b.open[len(b.open)-1]
	k := s.Base().Kind

	switch {
	case isBlockEnd(k):
		b.end(s)

		return true

	case k == lex.Map && top.kind != lex.Union:
		b.messages.Report("NOTINBLOCK", s.Position(), "a MAP declaration", "a UNION declaration")
	case k == lex.Union && top.kind == lex.Union:
		b.messages.Report("NOTINBLOCK", s.Position(), "a UNION declaration", "a STRUCTURE or MAP declaration")
	case k == lex.Structure && top.kind == lex.Union:
		b.messages.Report("NOTINBLOCK", s.Position(), "a STRUCTURE declaration", "a STRUCTURE or MAP declaration")
	case k == lex.Map, k == lex.Union, k == lex.Structure, k == lex.Directive:
	case !isField(k):
		b.messages.Report("FIELDSTMT", s.Position(), aStatement(k))
	case top.kind == lex.Union:
		b.messages.Report("NOTINBLOCK", s.Position(), aStatement(k), "a MAP declaration")
	}

	*top.body = append(*top.body, s)
	b.push(s)

	return true
}

// isField reports whether statements of kind k may declare a structure's
// fields: type statements, RECORD, and PARAMETER (section 4.15).
func isField(k lex.Kind) bool {
	return k.TypeKeyword() || k == lex.Record || k == lex.Parameter
}

// outside checks a statement outside every declaration, opening the
// declaration s starts, if it starts one. A UNION or MAP here, a
// declaration's end, or a %FILL field outside a structure is an error,
// and becomes a BadStmt.
func (b *blockStack) outside(s ast.Stmt) ast.Stmt {
	k := s.Base().Kind

	switch {
	case k == lex.Union, k == lex.Map:
		b.messages.Report("NOTINBLOCK", s.Position(), "a "+k.String()+" declaration", "a STRUCTURE declaration")
		b.push(s)
		b.open[len(b.open)-1].quiet = true

		return &ast.BadStmt{StmtBase: *s.Base()}

	case isBlockEnd(k):
		b.messages.Report("NOBLOCK", s.Position(), k.String(), opener(k).String())

		return &ast.BadStmt{StmtBase: *s.Base()}

	case k == lex.Structure:
		b.push(s)
	}

	if t, ok := s.(*ast.TypeStmt); ok {
		for _, d := range t.Entities {
			if d.Name == "%FILL" {
				b.messages.Report("FILLNAME", d.Pos)

				return &ast.BadStmt{StmtBase: t.StmtBase}
			}
		}
	}

	return s
}

// opener is the kind of declaration a declaration's end, k, ends.
func opener(k lex.Kind) lex.Kind {
	for open, end := range blockEnds {
		if end == k {
			return open
		}
	}

	return lex.Unknown
}

// push opens the declaration s starts, if it is one. A declaration
// whose statement had an error is still opened, its statements going
// nowhere, so that its END isn't reported again.
func (b *blockStack) push(s ast.Stmt) {
	var body *[]ast.Stmt

	switch s := s.(type) {
	case *ast.StructureStmt:
		body = &s.Body
	case *ast.UnionStmt:
		body = &s.Body
	case *ast.MapStmt:
		body = &s.Body
	case *ast.BadStmt:
		if _, ok := blockEnds[s.Kind]; !ok {
			return
		}

		body = &[]ast.Stmt{}
	default:
		return
	}

	b.open = append(b.open, openBlock{stmt: s, kind: s.Base().Kind, body: body})
}

// end closes the declaration that the END STRUCTURE, END UNION, or END
// MAP statement s ends: the innermost one of its kind, closing (and
// reporting) any opened inside it and not closed. An end with no
// declaration of its kind open is NOBLOCK, kept in the innermost one.
func (b *blockStack) end(s ast.Stmt) {
	k := s.Base().Kind
	want := opener(k)

	for i := len(b.open) - 1; i >= 0; i-- {
		if b.open[i].kind != want {
			continue
		}

		for j := len(b.open) - 1; j > i; j-- {
			b.unclosed(b.open[j])
		}

		*b.open[i].body = append(*b.open[i].body, s)
		b.open = b.open[:i]

		return
	}

	b.messages.Report("NOBLOCK", s.Position(), k.String(), want.String())

	top := b.open[len(b.open)-1]
	*top.body = append(*top.body, s)
}

// closeAll closes every open declaration, at a unit's END or the next
// unit's header, reporting each.
func (b *blockStack) closeAll() {
	for i := len(b.open) - 1; i >= 0; i-- {
		b.unclosed(b.open[i])
	}

	b.open = nil
}

// unclosed reports a declaration with no end (NOBLOCKEND).
func (b *blockStack) unclosed(o openBlock) {
	if o.quiet {
		return
	}

	b.messages.Report("NOBLOCKEND", o.stmt.Position(), o.kind.String(), blockEnds[o.kind].String())
}

// aStatement is "a CONTINUE statement", "an INTEGER statement".
func aStatement(k lex.Kind) string {
	return article(k.String()) + " " + k.String() + " statement"
}
