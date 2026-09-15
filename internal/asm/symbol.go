package asm

import (
	"fmt"
	"math"
	"strings"
)

// SymFlag records characteristics of a symbol, matching asm_symbols.c's
// SYM_* bit flags (the subset that affects assembly, as opposed to display
// formatting only).
type SymFlag uint32

const (
	SymNone SymFlag = 0
	// SymLabel marks a symbol defined by a "NAME:" label or a .SCOPE.
	SymLabel SymFlag = 1 << iota
	// SymEntry marks a symbol defined by .ENTRY or a .SHIM stub.
	SymEntry
	// SymPermanent marks a symbol that survives .CLEAR (set by a "::"
	// label or .SET/PERMANENT).
	SymPermanent
	// SymLocal marks a symbol whose name was scope-adjusted (a leading
	// "_" name rewritten under the current .ENTRY/.SCOPE), for parity with
	// the C source's SYM_LOCAL bookkeeping.
	SymLocal
	// SymSystem marks a symbol whose name contains '$' — the C source
	// keeps these in a separate "system symbols" table; this port keeps
	// one table but still records the distinction.
	SymSystem
	// SymBuiltin marks a symbol seeded by seedBuiltinSymbols at
	// construction time (XFC$/PTE$/EXC$/OPC$_... system constants), as
	// opposed to one this assembly itself defined — distinct from
	// SymPermanent, which a user program can also set via "::"/.SET
	// PERMANENT. Symbols (below) uses this to return only a program's own
	// symbols.
	SymBuiltin
)

// fixupKind says how a pending forward reference's value should be written
// once the symbol resolves — matching vax.h's K_ADDR_*/K_DISP_*/K_BRANCH_*/
// K_CASE_W constants (see asm_symbols.c's set_symbol()).
type fixupKind int

const (
	// fixNone means forward references are not permitted in this context
	// (K_NOFORWARD): an undefined symbol is an immediate error.
	fixNone fixupKind = iota
	fixAddrB
	fixAddrW
	fixAddrL
	fixDispB
	fixDispW
	fixDispL
	fixBranchB
	fixBranchW
	fixBranchL
	fixCaseW
)

// fixupSize returns the byte width a fixup kind writes; branch fixups use
// this to know how much to subtract from the displacement (VAX branch
// displacements are relative to the byte following the displacement field).
func fixupSize(k fixupKind) int64 {
	switch k {
	case fixAddrB, fixDispB, fixBranchB:
		return 1
	case fixAddrW, fixDispW, fixBranchW, fixCaseW:
		return 2
	case fixAddrL, fixDispL, fixBranchL:
		return 4
	}
	return 0
}

// forwardRef is one pending fixup: a location in the output image that
// needs to be patched once its symbol's value becomes known.
type forwardRef struct {
	location uint32
	kind     fixupKind
}

// symbol is one entry in the assembler's symbol table.
type symbol struct {
	name    string
	value   uint32
	flags   SymFlag
	forward []forwardRef // pending fixups, most-recent first; nil once resolved
}

// symbolTable holds every symbol defined during an assembly. Unlike the C
// source's single sorted linked list (split only by the SYM_SYSTEM flag for
// display purposes), this is a plain map — nothing in Phase 11's scope
// needs the sorted-traversal order that only mattered for the SHOW SYMBOL
// console command.
type symbolTable struct {
	byName map[string]*symbol
}

func newSymbolTable() *symbolTable {
	return &symbolTable{byName: make(map[string]*symbol)}
}

func (t *symbolTable) find(name string) (*symbol, bool) {
	s, ok := t.byName[name]
	return s, ok
}

func (t *symbolTable) create(name string) *symbol {
	s := &symbol{name: name}
	if strings.ContainsRune(name, '$') {
		s.flags |= SymSystem
	}
	t.byName[name] = s
	return s
}

// clear removes name from the table, matching CLEAR SYMBOL.
func (t *symbolTable) clear(name string) bool {
	if _, ok := t.byName[name]; !ok {
		return false
	}
	delete(t.byName, name)
	return true
}

// clearAll removes every symbol, matching .CLEAR with no name.
func (t *symbolTable) clearAll() { t.byName = make(map[string]*symbol) }

// scopeName rewrites a leading-underscore "local" name to be scoped under
// entry, matching asm_symbols.c's scope_name(): a name starting with a
// single "_" (not "__") is rewritten to "<entry>_<rest>"; anything else is
// returned unchanged. Reports whether scoping was applied.
func scopeName(name, entry string) (string, bool) {
	if len(name) < 2 || name[0] != '_' || name[1] == '_' {
		return name, false
	}
	return entry + "_" + name[1:], true
}

// entryScope returns the current scope prefix for local ("_name") symbols,
// generating an anonymous numbered scope the first time it's needed since
// the last scopeSymbols() call — matching scope_name()'s fallback when no
// .ENTRY/.SCOPE name is active.
func (a *Assembler) entryScope() string {
	if a.curEntry == "" {
		a.tempSeq++
		a.curEntry = fmt.Sprintf("__%d", a.tempSeq)
	}
	return a.curEntry
}

// scopeSymbols closes out the current local-symbol scope, matching
// scope_symbols() — called at .ENTRY/.SCOPE/.SHIM/.BASE/.END boundaries so
// the next "_name" reference starts a fresh scope.
func (a *Assembler) scopeSymbols() { a.curEntry = "" }

// resolvedName applies scopeName using the assembler's current entry scope,
// generating one on demand if needed.
func (a *Assembler) resolvedName(name string) (resolved string, wasLocal bool) {
	if len(name) < 2 || name[0] != '_' || name[1] == '_' {
		return name, false
	}
	return a.entryScope() + "_" + name[1:], true
}

// getSymbol resolves name to its current value, matching get_symbol().
//
// If the symbol is already fully resolved (defined, with no pending forward
// references) its value is returned directly. Otherwise: if allowForward is
// false, an undefined symbol is an error (K_NOFORWARD); if true, a
// placeholder value of 0 is used (creating the symbol if necessary) and a
// fixup of kind fx is queued at location, to be patched in when the symbol
// is later defined via setSymbol.
//
// Forward references are always prepended to the symbol's fixup list — some
// callers (asm_operand.c's late DISP(Rn) mode rewrite) rely on being able to
// find "the fixup just created for the last symbol referenced" at the head
// of that list, tracked here as a.lastSymbol.
func (a *Assembler) getSymbol(name string, allowForward bool, location uint32, fx fixupKind) (value uint32, wasForward bool, err error) {
	resolved, local := a.resolvedName(name)

	sym, found := a.symbols.find(resolved)
	if found && local {
		sym.flags |= SymLocal
	}

	switch {
	case found && len(sym.forward) == 0:
		a.lastSymbol = sym
		return sym.value, false, nil
	case !found && !allowForward:
		return 0, false, fmt.Errorf("undefined symbol %q", name)
	case found && !allowForward:
		a.lastSymbol = sym
		return sym.value, false, nil
	}

	if !found {
		sym = a.symbols.create(resolved)
		if fx == fixCaseW {
			sym.value = a.caseBase
		}
	}

	sym.forward = append([]forwardRef{{location: location, kind: fx}}, sym.forward...)
	a.lastSymbol = sym

	return sym.value, true, nil
}

// setSymbol defines (or redefines) name's value, applying flags and
// resolving any pending forward references against value — matching
// set_symbol(). If unique is true and the symbol already has a fully
// resolved definition, that's a duplicate-definition error (used by
// labels/.ENTRY/.SCOPE/.SHIM, which each require a fresh name).
func (a *Assembler) setSymbol(name string, value uint32, flags SymFlag, unique bool) error {
	resolved, local := a.resolvedName(name)
	if local {
		flags |= SymLocal
	}

	sym, found := a.symbols.find(resolved)
	if unique && found && len(sym.forward) == 0 {
		return fmt.Errorf("duplicate symbol definition %q", name)
	}
	if !found {
		sym = a.symbols.create(resolved)
	}

	a.lastSymbol = sym
	ivalue := sym.value
	sym.value = value
	sym.flags |= flags

	for _, fp := range sym.forward {
		if err := a.applyFixup(fp, value, ivalue); err != nil {
			return err
		}
	}
	sym.forward = nil

	return nil
}

// applyFixup patches one pending forward reference now that its symbol's
// value is known, matching set_symbol()'s fixup switch. ivalue is the
// symbol's value immediately before this definition (used only by
// fixCaseW, which measures the offset from the .CASE block's base rather
// than from the fixup's own location).
func (a *Assembler) applyFixup(fp forwardRef, value, ivalue uint32) error {
	disp := int64(value) - int64(fp.location)
	switch fp.kind {
	case fixBranchB, fixBranchW, fixBranchL:
		disp -= fixupSize(fp.kind)
	}

	switch fp.kind {
	case fixCaseW:
		d := int64(value) - int64(ivalue)
		if d < math.MinInt16 || d > math.MaxInt16 {
			return fmt.Errorf("forward reference displacement %d out of word range", d)
		}
		return a.image.storeWord(fp.location, uint16(int16(d)))

	case fixAddrB:
		disp = int64(int8(value))
		fallthrough
	case fixDispB, fixBranchB:
		if disp < -128 || disp > 127 {
			return fmt.Errorf("forward reference displacement %d out of byte range", disp)
		}
		return a.image.storeByte(fp.location, byte(int8(disp)))

	case fixAddrW:
		disp = int64(int32(value))
		fallthrough
	case fixDispW, fixBranchW:
		if disp < -32768 || disp > 32767 {
			return fmt.Errorf("forward reference displacement %d out of word range", disp)
		}
		return a.image.storeWord(fp.location, uint16(int16(disp)))

	case fixAddrL:
		disp = int64(int32(value))
		fallthrough
	case fixDispL, fixBranchL:
		return a.image.storeLongword(fp.location, uint32(int32(disp)))
	}

	return fmt.Errorf("internal: unhandled fixup kind %d", fp.kind)
}

// hasUnresolvedSymbols reports whether any symbol still has pending forward
// references, matching check_unresolved_symbols(0).
func (a *Assembler) hasUnresolvedSymbols() bool {
	for _, s := range a.symbols.byName {
		if len(s.forward) != 0 {
			return true
		}
	}
	return false
}

// Symbols returns every symbol this assembly itself defined (labels,
// .ENTRY points, .SET values, .SHIM stubs, ...) with no pending forward
// references — excluding the fixed set seeded by seedBuiltinSymbols at
// construction time (see SymBuiltin). Used by the console's ASM command
// (Phase 12) to merge a freshly assembled program's own symbol table into
// Console.Symbols once its bytes have been deposited into live memory.
func (a *Assembler) Symbols() map[string]uint32 {
	out := make(map[string]uint32)
	for name, s := range a.symbols.byName {
		if s.flags&SymBuiltin != 0 || len(s.forward) != 0 {
			continue
		}
		out[name] = s.value
	}
	return out
}
