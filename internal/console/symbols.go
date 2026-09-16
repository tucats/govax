package console

import (
	"sort"
	"strings"
)

// SymbolKind classifies a Symbol, matching the categories
// reference/eVAX/eVAX/Source/Console/console_show.c's SHOW SYMBOL and
// console_clear.c's CLEAR SYMBOL distinguish (system symbols like
// CONSOLE$SCRATCH survive a VMINIT/CLEAR SYMBOL/ALL that wipes user-defined
// ones; see console_vminit.c's "doesn't include reserved symbols with a $
// character" comment).
type SymbolKind int

const (
	SymbolUser SymbolKind = iota
	SymbolSystem
)

// Symbol is one entry in a SymbolTable.
type Symbol struct {
	Name  string
	Value uint32
	Kind  SymbolKind
	// IsEntry marks a symbol defined by .ENTRY (or a .SHIM stub) --
	// SYM_ENTRY in the C reference. Independent of Kind (an entry point
	// can be either a user or a system symbol): SHOW SYMBOL displays it,
	// and Disassemble/traceStep consult it (via EntryAt) to recognize a
	// routine's register-save mask word instead of misdecoding it as an
	// instruction (matching decode_opcode.c's own SYM_ENTRY scan).
	IsEntry bool
}

// SymbolTable is the console's symbol table — a simplified, map-based
// replacement for the C source's SYMBOL/FSYMBOL linked lists (struct
// SYMBOL's forward-reference-patching machinery has no purpose here since
// this port's expression evaluator, unlike the inline assembler, never
// forward-references a symbol before it's defined; see expr.go).
type SymbolTable struct {
	m map[string]*Symbol
}

// NewSymbolTable returns an empty SymbolTable.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{m: map[string]*Symbol{}}
}

// Set defines or redefines a symbol.
func (t *SymbolTable) Set(name string, value uint32, kind SymbolKind) {
	t.m[strings.ToUpper(name)] = &Symbol{Name: strings.ToUpper(name), Value: value, Kind: kind}
}

// SetEntry defines or redefines a symbol with IsEntry set, matching .ENTRY
// (or a .SHIM stub) — see Symbol.IsEntry.
func (t *SymbolTable) SetEntry(name string, value uint32, kind SymbolKind) {
	t.m[strings.ToUpper(name)] = &Symbol{Name: strings.ToUpper(name), Value: value, Kind: kind, IsEntry: true}
}

// Get looks up a symbol by name (case-insensitive).
func (t *SymbolTable) Get(name string) (uint32, bool) {
	s, ok := t.m[strings.ToUpper(name)]
	if !ok {
		return 0, false
	}
	return s.Value, true
}

// Find returns the full Symbol record for name (case-insensitive), or
// (nil, false) if undefined -- unlike Get, which only reports the value,
// for a caller (SHOW SYMBOL) that also needs Kind.
func (t *SymbolTable) Find(name string) (*Symbol, bool) {
	s, ok := t.m[strings.ToUpper(name)]

	return s, ok
}

// FindByValue returns the name of a symbol (in All's sorted order, for
// determinism, when more than one matches) whose value equals v, or ("",
// false) if none — a simplified stand-in for find_label's SYM_LABEL/
// SYM_ENTRY-kind-filtered reverse lookup: this port's SymbolKind doesn't
// distinguish a label/entry point from any other kind of symbol (see
// docs/PHASE-16.md sub-phase 1c), so every symbol is a candidate here.
func (t *SymbolTable) FindByValue(v uint32) (string, bool) {
	for _, s := range t.All() {
		if s.Value == v {
			return s.Name, true
		}
	}

	return "", false
}

// EntryAt returns the name of an IsEntry symbol whose value equals addr (in
// All's sorted order, for determinism, when more than one matches), or
// ("", false) if none — matching decode_opcode.c's own linear SYM_ENTRY
// scan by PC. Used by Disassemble/traceStep to recognize a routine's
// register-save mask word at its .ENTRY address instead of decoding it as
// an instruction.
func (t *SymbolTable) EntryAt(addr uint32) (string, bool) {
	for _, s := range t.All() {
		if s.IsEntry && s.Value == addr {
			return s.Name, true
		}
	}

	return "", false
}

// Delete removes one symbol by name.
func (t *SymbolTable) Delete(name string) {
	delete(t.m, strings.ToUpper(name))
}

// ClearAll removes every user symbol, matching CLEAR SYMBOL/ALL — system
// symbols are untouched.
func (t *SymbolTable) ClearAll() {
	for k, s := range t.m {
		if s.Kind == SymbolUser {
			delete(t.m, k)
		}
	}
}

// All returns every symbol, sorted by name, for SHOW SYMBOL.
func (t *SymbolTable) All() []*Symbol {
	out := make([]*Symbol, 0, len(t.m))
	for _, s := range t.m {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
