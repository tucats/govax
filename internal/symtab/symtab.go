// Package symtab is a symbol table that can be searched both ways: by
// name, as an assembler or an expression evaluator does, and by address,
// as a disassembler or debugger does when it shows "START+2" instead of
// 00000402. The assembler (internal/asm), the console's own table, and an
// image's debug symbols (internal/dbgsym) all keep their symbols in it,
// so one lookup serves them all (docs/PHASE-41.md).
//
// It's a leaf package: it imports nothing from govax.
package symtab

import (
	"sort"
	"strings"
)

// Flags are a symbol's attributes. A symbol can have several: a console
// symbol set with SET/ENTRY/LABEL is both an entry point and a label.
type Flags uint32

const (
	// Entry marks a routine's entry point (.ENTRY): its address holds the
	// routine's register-save mask, not an instruction.
	Entry Flags = 1 << iota
	// Label marks a name for a code address that isn't an entry point.
	Label
	// Data marks the name of data: a longword, a string, an array.
	Data
	// Literal marks a constant (LIMIT = 10): its value isn't an
	// address.
	Literal
	// Psect marks a program section's name; its value is the section's
	// base address.
	Psect
	// Module marks a module's name.
	Module
	// Global marks a symbol known outside its module.
	Global
	// System marks a symbol the system defined rather than the user (in
	// the console, one whose name has a "$", or that govax defined).
	System
	// Permanent marks a symbol that survives clearing the temporary ones
	// (the console's SET/PERMANENT, MACRO's "::").
	Permanent
	// Builtin marks one of the assembler's predefined symbols (PTE$K_*,
	// OPC$_*, ...).
	Builtin
)

// Symbol is one entry in a Table.
type Symbol struct {
	// Name is the symbol's name as given. Lookups ignore case.
	Name string
	// Value is the symbol's value: for most symbols an address.
	Value uint32
	Flags Flags
	// Scope is where the symbol is defined, when that matters: an image's
	// debug symbols are scoped by module and routine ("DBGDIS\START").
	// "" for none.
	Scope string
	// Size is the size in bytes of what the symbol names (a routine's
	// code, a psect, an array), or 0 if unknown.
	Size uint32
}

// Has reports whether s has every flag in f.
func (s *Symbol) Has(f Flags) bool { return s.Flags&f == f }

// IsEntry reports whether s is a routine's entry point.
func (s *Symbol) IsEntry() bool { return s.Has(Entry) }

// IsLabel reports whether s is a label.
func (s *Symbol) IsLabel() bool { return s.Has(Label) }

// IsSystem reports whether s was defined by the system.
func (s *Symbol) IsSystem() bool { return s.Has(System) }

// IsPermanent reports whether s survives clearing temporary symbols.
func (s *Symbol) IsPermanent() bool { return s.Has(Permanent) }

// IsBuiltin reports whether s is one of the assembler's predefined
// symbols.
func (s *Symbol) IsBuiltin() bool { return s.Has(Builtin) }

// Table holds symbols by name, with an index by address built when it's
// first needed after a change. The zero value isn't usable; call New.
type Table struct {
	byName map[string]*Symbol

	// byValue is every symbol, sorted by value and then name; nil when a
	// change has made it stale.
	byValue []*Symbol
}

// New returns an empty Table.
func New() *Table {
	return &Table{byName: map[string]*Symbol{}}
}

func key(name string) string { return strings.ToUpper(name) }

// Set defines s, replacing any symbol of the same name (ignoring case).
// The table keeps its own copy.
func (t *Table) Set(s Symbol) {
	t.byName[key(s.Name)] = &s
	t.byValue = nil
}

// Get returns the symbol named name (ignoring case).
func (t *Table) Get(name string) (*Symbol, bool) {
	s, ok := t.byName[key(name)]

	return s, ok
}

// Delete removes the symbol named name, if there is one.
func (t *Table) Delete(name string) {
	if _, ok := t.byName[key(name)]; ok {
		delete(t.byName, key(name))
		t.byValue = nil
	}
}

// DeleteIf removes every symbol drop reports true for, and returns how
// many it removed.
func (t *Table) DeleteIf(drop func(*Symbol) bool) int {
	n := 0

	for k, s := range t.byName {
		if drop(s) {
			delete(t.byName, k)

			n++
		}
	}

	if n > 0 {
		t.byValue = nil
	}

	return n
}

// Len returns the number of symbols.
func (t *Table) Len() int { return len(t.byName) }

// All returns every symbol, sorted by name.
func (t *Table) All() []*Symbol {
	out := make([]*Symbol, 0, len(t.byName))

	for _, s := range t.byName {
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out
}

// index returns the symbols sorted by value, then by name, building the
// index if a change has made it stale.
func (t *Table) index() []*Symbol {
	if t.byValue == nil {
		t.byValue = make([]*Symbol, 0, len(t.byName))

		for _, s := range t.byName {
			t.byValue = append(t.byValue, s)
		}

		sort.Slice(t.byValue, func(i, j int) bool {
			a, b := t.byValue[i], t.byValue[j]
			if a.Value != b.Value {
				return a.Value < b.Value
			}

			return a.Name < b.Name
		})
	}

	return t.byValue
}

// At returns the first symbol, in name order, whose value is v and that
// match accepts (a nil match accepts any symbol).
func (t *Table) At(v uint32, match func(*Symbol) bool) (*Symbol, bool) {
	idx := t.index()

	// sort.Search finds the first symbol whose value is at least v;
	// those with value v follow it in name order.
	for i := sort.Search(len(idx), func(i int) bool { return idx[i].Value >= v }); i < len(idx) && idx[i].Value == v; i++ {
		if match == nil || match(idx[i]) {
			return idx[i], true
		}
	}

	return nil, false
}

// Nearest returns the symbol match accepts (a nil match accepts any)
// with the greatest value at or below v, and v's offset from it: what a
// disassembler shows as NAME+offset. Of several with that value, it's
// the first in name order.
func (t *Table) Nearest(v uint32, match func(*Symbol) bool) (*Symbol, uint32, bool) {
	idx := t.index()

	// The first symbol whose value is above v; everything before it is
	// a candidate, the nearest last.
	end := sort.Search(len(idx), func(i int) bool { return idx[i].Value > v })

	for i := end - 1; i >= 0; i-- {
		s := idx[i]
		if match != nil && !match(s) {
			continue
		}

		// Of several at this value, prefer the first by name.
		for i > 0 && idx[i-1].Value == s.Value {
			if prev := idx[i-1]; match == nil || match(prev) {
				s = prev
			}

			i--
		}

		return s, v - s.Value, true
	}

	return nil, 0, false
}
