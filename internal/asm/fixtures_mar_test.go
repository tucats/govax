package asm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// realObject reads testdata/mar/vax/name.obj, the object real MACRO made
// from testdata/mar/name.mar.
func realObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "testdata", "mar", "vax", name+".obj"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	raw, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// objectSummary describes what an object module declares, one line each,
// in a form an assembly can be described in too: the module name and
// version, each psect (name, index, alignment, flags, and allocation),
// and each global symbol (defined or referred to, weak, psect, value, and
// entry mask).
type objectSummary struct {
	module  string
	psects  []string
	symbols []string
}

func summarizeObject(m *obj.Module) objectSummary {
	var s objectSummary

	index := 0

	for _, rec := range m.Records {
		switch r := rec.(type) {
		case *obj.MainHeader:
			s.module = r.Name + " " + r.Version

		case *obj.GSD:
			for _, sub := range r.Subrecords {
				switch e := sub.(type) {
				case *obj.Psect:
					s.psects = append(s.psects, fmt.Sprintf("%d %s align=%d flags=%03X alloc=%d", index, e.Name, e.Align, e.Flags, e.Alloc))
					index++

				case *obj.Symbol:
					s.symbols = append(s.symbols, globalLine(e.Name, e.Defined(), e.Flags&obj.SymWEAK != 0, int(e.Psect), e.Value, e.GSDType() == obj.GSDEntry, e.Mask))
				}
			}
		}
	}

	sort.Strings(s.symbols)

	return s
}

func globalLine(name string, defined, weak bool, psect int, value uint32, entry bool, mask uint16) string {
	line := name

	if weak {
		line += " weak"
	}

	if !defined {
		return line + " ref"
	}

	line += fmt.Sprintf(" def %d:%X", psect, value)

	if entry {
		line += fmt.Sprintf(" mask=%04X", mask)
	}

	return line
}

// summarizeAssembly describes an assembly the way summarizeObject does an
// object module.
func summarizeAssembly(a *Assembler) objectSummary {
	var s objectSummary

	name, _ := a.Title()
	s.module = name + " " + a.Ident()

	for _, sect := range a.sections {
		s.psects = append(s.psects, fmt.Sprintf("%d %s align=%d flags=%03X alloc=%d", sect.index, sect.name, sect.align, sect.flags, sect.hi))
	}

	for name, sym := range a.symbols.byName {
		switch {
		case sym.flags&SymExternal != 0:
			s.symbols = append(s.symbols, globalLine(name, false, sym.flags&SymWeak != 0, 0, 0, false, 0))

		case sym.flags&SymGlobal != 0:
			psect := 0
			if sym.sect != nil {
				psect = sym.sect.index
			}

			s.symbols = append(s.symbols, globalLine(name, true, sym.flags&SymWeak != 0, psect, sym.value, sym.flags&SymEntry != 0, sym.mask))
		}
	}

	sort.Strings(s.symbols)

	return s
}

// awaitingOperandEncoding are the fixtures whose operands need subtask 7
// of docs/PHASE-27.md: G^ general mode (extern, hello), and MACRO's
// displacement sizes, which change CODE's allocation (branch).
var awaitingOperandEncoding = map[string]bool{"branch": true, "extern": true, "hello": true}

// TestFixtureLadderDeclarations assembles each testdata/mar fixture and
// checks that it declares what real MACRO's object for it does: the
// module name and version, each psect's index, attributes, and
// allocation, and the global symbols it defines and refers to.
func TestFixtureLadderDeclarations(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		t.Run(name, func(t *testing.T) {
			if awaitingOperandEncoding[name] {
				t.Skip("needs subtask 7's operand encoding")
			}

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssemble(t, string(src))

			got, want := summarizeAssembly(a), summarizeObject(realObject(t, name))

			if got.module != want.module {
				t.Errorf("module = %q, want %q", got.module, want.module)
			}

			compareLines(t, "psects", got.psects, want.psects)
			compareLines(t, "symbols", got.symbols, want.symbols)
		})
	}
}

func compareLines(t *testing.T, what string, got, want []string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n%s\nwant:\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
