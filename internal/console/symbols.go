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

// Get looks up a symbol by name (case-insensitive).
func (t *SymbolTable) Get(name string) (uint32, bool) {
	s, ok := t.m[strings.ToUpper(name)]
	if !ok {
		return 0, false
	}
	return s.Value, true
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
