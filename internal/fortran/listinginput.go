package fortran

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/listing"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// listingInput gathers what the listing shows from what the passes that ran left.
// tree and prog are nil if the source pass stopped the compilation, and maps is
// empty if there was any error.
func listingInput(opts Options, file File, program *source.Program, messages []diag.Message,
	tree *ast.File, prog *sema.Program, maps []codegen.StorageMap,
) listing.Input {
	in := listing.Input{
		Lines:    program.Lines,
		Messages: messages,
		Show:     opts.Show,
		Title:    ModuleName(file.Name),
		File:     file.Spec,
		Version:  opts.Version,
		Compiled: opts.Compiled,
	}

	if tree != nil {
		in.Headings = listingHeadings(tree)
	}

	if prog != nil {
		in.Units = listingUnits(prog)

		if opts.CrossReference {
			in.CrossRefs = listingCrossRefs(prog, program)
		}
	}

	for _, m := range maps {
		in.Maps = append(in.Maps, listing.Map{Unit: m.Routine, Lines: m.Lines})
	}

	return in
}

// referenceKinds is the letter after a line number that says what the line does to
// a name: a line that only uses it has none.
var referenceKinds = map[sema.RefKind]string{
	sema.RefDefined:  "D",
	sema.RefAssigned: "A",
}

// listingCrossRefs is each unit's cross reference, as text. A mention on an included
// line has that line's include depth before its number, as the listing's source lines
// do, since line 1 of an INCLUDE file isn't line 1 of the file that includes it.
func listingCrossRefs(prog *sema.Program, program *source.Program) []listing.CrossRef {
	// make's third argument is the slice's capacity: room for every unit,
	// so the appends below never have to grow it.
	crossRefs := make([]listing.CrossRef, 0, len(prog.Units))

	// A mention of a name records its line's serial number (each line read,
	// counting INCLUDE files' lines, has its own), so build a map from that
	// number to the line's include depth once, rather than searching the
	// lines for every mention.
	depths := map[int]int{}
	for _, line := range program.Lines {
		depths[line.Serial] = line.Depth
	}

	for _, u := range prog.Units {
		crossRef := listing.CrossRef{Unit: u.Name}

		for _, name := range u.CrossReference() {
			row := listing.CrossRow{Name: name.Name, What: name.What}
			for _, ref := range name.Refs {
				text := strconv.Itoa(ref.Line) + referenceKinds[ref.Kind]
				if depth := depths[ref.Serial]; depth > 0 {
					text = strconv.Itoa(depth) + ":" + text
				}

				row.Refs = append(row.Refs, text)
			}

			crossRef.Rows = append(crossRef.Rows, row)
		}

		crossRefs = append(crossRefs, crossRef)
	}

	return crossRefs
}

// listingUnits says where each unit starts: at its first line, or past its
// predecessor's last statement, which takes in the comments before it.
func listingUnits(prog *sema.Program) []listing.Unit {
	units := make([]listing.Unit, 0, len(prog.Units))
	start := 0

	// Comment lines between two units belong to the second: they usually
	// describe it. So each unit after the first starts on the line after
	// the last statement (END) of the one before.
	for _, u := range prog.Units {
		units = append(units, listing.Unit{Line: start, Name: u.Name})

		if last := lastStatement(u.AST); last != nil {
			start = last.Base().Source.Last + 1
		}
	}

	return units
}

// lastStatement is the unit's last statement, nil if it has none.
func lastStatement(u *ast.Unit) ast.Stmt {
	if n := len(u.Body); n > 0 {
		return u.Body[n-1]
	}

	return u.Header
}

// listingHeadings finds the file's TITLE and SUBTITLE directives that give a string.
// A directive without one changes nothing (section 10.3.3).
func listingHeadings(tree *ast.File) []listing.Heading {
	// walk is a recursive function literal. A function literal can't refer
	// to itself by a name given in its own := statement, since the name
	// doesn't exist until the statement ends, so walk is declared first
	// with var and assigned after; the literal then calls walk freely.
	var (
		headings []listing.Heading
		walk     func(list []ast.Stmt)
	)

	walk = func(list []ast.Stmt) {
		for _, s := range list {
			// A type switch: s is an ast.Stmt, an interface, and each case
			// asks whether the value inside it is of that concrete type.
			// Within a case, s is that type, so its fields can be used.
			// Directives may sit inside a STRUCTURE declaration (and its
			// UNIONs and MAPs), whose statements nest, so walk goes in.
			switch s := s.(type) {
			case *ast.DirectiveStmt:
				if (s.Name == "TITLE" || s.Name == "SUBTITLE") && s.HasText {
					headings = append(headings, listing.Heading{Line: s.Source.First, Subtitle: s.Name == "SUBTITLE", Text: s.Text})
				}
			case *ast.StructureStmt:
				walk(s.Body)
			case *ast.UnionStmt:
				walk(s.Body)
			case *ast.MapStmt:
				walk(s.Body)
			}
		}
	}

	for _, u := range tree.Units {
		walk(u.Prologue)
		walk(u.Body)
	}

	walk(tree.Trailing)

	return headings
}
