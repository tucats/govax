package console

import (
	"strings"

	"github.com/tucats/govax/internal/symtab"
)

// SymbolKind says who defined a symbol.
type SymbolKind int

const (
	SymbolUser SymbolKind = iota
	SymbolSystem
	SymbolDCL // Used by symbols created in the console via DCL ":=" syntax
)

// Symbol is one entry in a SymbolTable: a symtab.Symbol, whose flags hold
// the console's attributes (Phase 41 moved the table onto internal/symtab,
// so the disassembler can look console symbols up by address):
//
//   - System: defined by the system, not the user (SymbolSystem).
//   - Entry: defined by .ENTRY (or a .SHIM stub) or SET/ENTRY -- SYM_ENTRY
//     in the C reference. SHOW SYMBOL displays it, and Disassemble/
//     traceStep consult it (via EntryAt) to recognize a routine's
//     register-save mask word instead of misdecoding it as an instruction.
//   - Permanent: defined with SET/PERMANENT -- SYM_PERMANENT in the C
//     reference. CLEAR SYMBOL/TEMPORARY (ClearTemporary) removes every
//     user symbol without it.
//   - Label: defined with SET/LABEL -- SYM_LABEL in the C reference,
//     tracked for SHOW SYMBOL's attribute display.
//   - Builtin: one of the assembler's predefined system symbols
//     (asm.BuiltinSymbols: PTE$K_*, VAX$PR_*, OPC$_*, ...), which the
//     console resolves but doesn't keep in its own table. Only SHOW
//     SYMBOL's listings build such Symbols (see Console.listSymbols).
type Symbol = symtab.Symbol

// SymbolTable is the console's symbol table — a simplified replacement
// for the C source's SYMBOL/FSYMBOL linked lists (struct SYMBOL's
// forward-reference-patching machinery has no purpose here since this
// port's expression evaluator, unlike the inline assembler, never
// forward-references a symbol before it's defined; see expr.go). Names
// are stored uppercased.
type SymbolTable struct {
	t *symtab.Table
}

// NewSymbolTable returns an empty SymbolTable.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{t: symtab.New()}
}

// kindFlags is the flag a SymbolKind sets.
func kindFlags(kind SymbolKind) symtab.Flags {
	if kind == SymbolSystem {
		return symtab.System
	}

	return 0
}

// Set defines or redefines a symbol.
func (t *SymbolTable) Set(name string, value uint32, kind SymbolKind) {
	t.t.Set(Symbol{Name: strings.ToUpper(name), Value: value, Flags: kindFlags(kind)})
}

// SetEntry defines or redefines a symbol that is an entry point, matching
// .ENTRY (or a .SHIM stub).
func (t *SymbolTable) SetEntry(name string, value uint32, kind SymbolKind) {
	t.t.Set(Symbol{Name: strings.ToUpper(name), Value: value, Flags: kindFlags(kind) | symtab.Entry})
}

// SetQualified defines or redefines a user symbol with the /PERMANENT,
// /ENTRY, /LABEL attributes SET's qualifiers give (setcommand.go's
// SET_SYMBOL).
func (t *SymbolTable) SetQualified(name string, value uint32, permanent, entry, label bool) {
	var flags symtab.Flags

	if permanent {
		flags |= symtab.Permanent
	}

	if entry {
		flags |= symtab.Entry
	}

	if label {
		flags |= symtab.Label
	}

	t.t.Set(Symbol{Name: strings.ToUpper(name), Value: value, Flags: flags})
}

// Get looks up a symbol by name (case-insensitive).
func (t *SymbolTable) Get(name string) (uint32, bool) {
	s, ok := t.t.Get(name)
	if !ok {
		return 0, false
	}

	return s.Value, true
}

// Find returns the full Symbol record for name (case-insensitive), or
// (nil, false) if undefined -- unlike Get, which only reports the value,
// for a caller (SHOW SYMBOL) that also needs its attributes.
func (t *SymbolTable) Find(name string) (*Symbol, bool) {
	return t.t.Get(name)
}

// FindByValue returns the name of a symbol (the first by name, for
// determinism, when more than one matches) whose value equals v, or ("",
// false) if none — a simplified stand-in for find_label's SYM_LABEL/
// SYM_ENTRY-kind-filtered reverse lookup: every symbol is a candidate
// here (see docs/PHASE-16.md sub-phase 1c).
func (t *SymbolTable) FindByValue(v uint32) (string, bool) {
	if s, ok := t.t.At(v, nil); ok {
		return s.Name, true
	}

	return "", false
}

// EntryAt returns the name of an entry-point symbol whose value equals
// addr (the first by name when more than one matches), or ("", false) if
// none. Used by Disassemble/traceStep to recognize a routine's 
// register-save mask word at its .ENTRY address instead of decoding
// it as an instruction.
func (t *SymbolTable) EntryAt(addr uint32) (string, bool) {
	if s, ok := t.t.At(addr, (*Symbol).IsEntry); ok {
		return s.Name, true
	}

	return "", false
}

// Delete removes one symbol by name.
func (t *SymbolTable) Delete(name string) {
	t.t.Delete(name)
}

// ClearAll removes every user symbol, matching CLEAR SYMBOL/ALL — system
// symbols are untouched.
func (t *SymbolTable) ClearAll() {
	t.t.DeleteIf(func(s *Symbol) bool { return !s.IsSystem() })
}

// ClearTemporary removes every non-permanent user symbol, matching CLEAR
// SYMBOL/TEMPORARY — a permanent one (SET/PERMANENT) survives, as does 
// every system symbol. Returns the count removed, matching CLEAR SYMBOL/ALL's
// own report convention (seeClearSymbol, misc.go).
func (t *SymbolTable) ClearTemporary() int {
	return t.t.DeleteIf(func(s *Symbol) bool { return !s.IsSystem() && !s.IsPermanent() })
}

// All returns every symbol, sorted by name, for SHOW SYMBOL.
func (t *SymbolTable) All() []*Symbol {
	return t.t.All()
}

// Table returns the symtab.Table the console's symbols are kept in, for a
// caller that looks them up by address (the disassembler).
func (t *SymbolTable) Table() *symtab.Table {
	return t.t
}
