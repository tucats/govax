package asm

import (
	"fmt"
	"math"

	"github.com/tucats/govax/internal/vmserrors"
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
	// SymLocalLabel marks a MACRO-32 local label ("1$", "20$", ...),
	// stored under a block-qualified name (see localLabelName). Symbols
	// leaves these out: they mean nothing outside their own block.
	SymLocalLabel
	// SymExternal marks a symbol a MACRO-dialect assembly referred to but
	// never defined, which the linker must supply (see finish).
	SymExternal
	// SymGlobal marks a symbol known outside the module: defined by
	// "::", "==", or .ENTRY, or named by .GLOBAL, .EXTERNAL, or .WEAK.
	SymGlobal
	// SymWeak marks a symbol named by .WEAK.
	SymWeak
	// SymUndefined marks a symbol .GLOBAL, .EXTERNAL, .WEAK, or .MASK
	// named before (or without) defining it: it has attributes, but no
	// value yet.
	SymUndefined
	// SymExtern marks a symbol named by .EXTERNAL, which a listing's
	// symbol table doesn't mark global (G) while it's undefined, though
	// .EXTERNAL also sets SymGlobal.
	SymExtern
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
	// fixDispB/W/L are an operand's PC-relative displacement (relative
	// or relative deferred mode), measured, like a branch's, from the end
	// of the field. They differ from the branch kinds only in the MACRO
	// dialect, where a displacement to a label defined later in the same
	// psect is left to the linker, as real MACRO leaves it (see
	// completeFixup).
	fixDispB
	fixDispW
	fixDispL
	fixBranchB
	fixBranchW
	fixBranchL
	fixCaseW
	// fixAddress is a longword address, position independent: the object
	// language's STO_PIDR, which .ADDRESS and .ASCID's pointer use.
	fixAddress
	// fixPICR is a G^ general mode operand, five bytes starting at the
	// mode byte, which the linker writes as relative or absolute mode:
	// the object language's STO_PICR. It's always left to the linker.
	fixPICR
	// fixSignedB/W are a MACRO-dialect displacement mode field, a signed
	// byte or word: the object language's STO_SB/STO_SW.
	fixSignedB
	fixSignedW
)

// fixupSize returns the byte width a fixup kind writes; branch fixups use
// this to know how much to subtract from the displacement (VAX branch
// displacements are relative to the byte following the displacement field).
func fixupSize(k fixupKind) int64 {
	switch k {
	case fixAddrB, fixDispB, fixBranchB, fixSignedB:
		return 1
	case fixAddrW, fixDispW, fixBranchW, fixCaseW, fixSignedW:
		return 2
	case fixAddrL, fixDispL, fixBranchL, fixAddress:
		return 4
	case fixPICR:
		return 5
	}

	return 0
}

// fixup is one value the assembler couldn't finish when it read it: a
// location in section sect to store expr's value in, once every symbol
// expr uses is defined (see rexpr). expr is whatever an operand or data
// item can say: "SYM", "SYM+8", "B-A", "2*SYM", "SYM-.". In the console
// dialect the last symbol's definition completes it; in the MACRO
// dialect one using a relocatable section's base or an external symbol
// becomes a relocation for the linker instead.
type fixup struct {
	sect     *section
	location uint32
	kind     fixupKind
	expr     *rexpr
	pending  int    // how many of expr's symbols are still undefined
	base     uint32 // fixCaseW: the .CASE block's base address
	// prefix is how many bytes of the operand specifier come before the
	// field: its addressing mode byte, and an index byte before that.
	// Real MACRO stores them after the value's stack program.
	prefix int
	// stmt is the statement that queued the fixup, and dead says a later
	// statement stored over its field, so it's never applied (see claim).
	stmt int
	dead bool
	// line and where are the line that queued the fixup (when it's
	// listed) and its place in the source, in the MACRO dialect, for a
	// value the fixup finds out of range (see fixupRange).
	line  *listLine
	where func(error) error
}

// symbol is one entry in the assembler's symbol table.
type symbol struct {
	name  string
	value uint32
	// sect is the relocatable section a label is in, whose base value is
	// relative to, or nil for an absolute value (every value, in the
	// console dialect).
	sect    *section
	flags   SymFlag
	forward []*fixup // pending fixups using this symbol, most-recent first; nil once defined
	// mask is an .ENTRY symbol's register save mask.
	mask uint16
	// absSect is the named absolute psect a MACRO-dialect label was
	// defined in, whose index its GSD record carries, as real MACRO's
	// does (docs/PHASE-27.md, subtask 11's log). It's nil otherwise.
	absSect *section

	// What a listing's symbol table needs (listclose.go). firstSect is
	// the section that was current when a symbol was first named without
	// being defined (referred to, or declared by .GLOBAL and the like),
	// which the table shows for an external symbol. referenced says an
	// expression used the symbol, and suppressed that its last
	// definition was made under .ENABLE SUPPRESSION: a suppressed symbol
	// that's never referenced isn't listed.
	firstSect  *section
	referenced bool
	suppressed bool

	// A MACRO-dialect label's definition: the line it was defined on
	// (listed, if a listing was asked for), so that a later definition
	// can report it out of phase (see outOfPhase).
	defLine  *listLine
	defWhere func(error) error
}

// defined reports whether s has a value: it isn't waiting on a forward
// reference's definition, and isn't only declared or external.
func (s *symbol) defined() bool {
	return len(s.forward) == 0 && s.flags&(SymUndefined|SymExternal) == 0
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
	if strings.ContainsRune(name, '@') {
		s.flags |= SymLocalLabel
	} else if strings.ContainsRune(name, '$') {
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

// isLocalLabel reports whether name is a MACRO-32 local label: one or
// more decimal digits followed by a single "$" (e.g. "1$", "30$").
func isLocalLabel(name string) bool {
	n := len(name)
	if n < 2 || name[n-1] != '$' {
		return false
	}

	for i := 0; i < n-1; i++ {
		if !isDigit(name[i]) {
			return false
		}
	}

	return true
}

// localLabelName returns the symbol-table name a local label is stored
// under in the current local label block. The "@" can't appear in a
// source symbol, so these never collide with an ordinary name.
func (a *Assembler) localLabelName(name string) string {
	return fmt.Sprintf("%s@%d", name, a.localBlock)
}

// closeLocalBlock ends the current local label block, matching MACRO-32:
// a block runs from one ordinary label to the next, so every ordinary
// label definition (and .ENTRY/.SCOPE/.SHIM/.REGION/.END) calls this
// before defining its own name. A local label referenced in the block but
// never defined there is an error now, since no later definition can
// resolve it.
func (a *Assembler) closeLocalBlock() error {
	if err := a.checkLocalBlock(); err != nil {
		return err
	}

	a.localBlock++
	a.localUsed = false

	return nil
}

// checkLocalBlock reports a local label referenced in the current block
// but never defined there. A block .SAVE_PSECT LOCAL_BLOCK saved isn't
// checked, since .RESTORE_PSECT returns to it.
func (a *Assembler) checkLocalBlock() error {
	if !a.localUsed || a.blockSaved(a.localBlock) {
		return nil
	}

	suffix := fmt.Sprintf("@%d", a.localBlock)

	for key, s := range a.symbols.byName {
		if len(s.forward) != 0 && strings.HasSuffix(key, suffix) {
			return vmserrors.New(vmserrors.VAX_UNDEFSYM, strings.TrimSuffix(key, suffix))
		}
	}

	return nil
}

// endLocalBlock ends the local label block at a user-defined label or a
// .PSECT, unless .ENABLE LOCAL_BLOCK is holding it open (the MACRO
// manual, §3.4).
func (a *Assembler) endLocalBlock() error {
	if a.localBlockHeld {
		return nil
	}

	return a.closeLocalBlock()
}

// resolvedName applies scopeName using the assembler's current entry scope,
// generating one on demand if needed. A local label ("n$") is qualified
// by the current local label block instead.
func (a *Assembler) resolvedName(name string) (resolved string, wasLocal bool) {
	if isLocalLabel(name) {
		a.localUsed = true

		return a.localLabelName(name), false
	}

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
// placeholder value of 0 is returned and a fixup of kind fx for the bare
// symbol is queued at location, to be patched in when the symbol is later
// defined via setSymbol. (Expressions go through exprValue instead, which
// queues one fixup for the whole expression.)
func (a *Assembler) getSymbol(name string, allowForward bool, location uint32, fx fixupKind) (value uint32, wasForward bool, err error) {
	resolved, local := a.resolvedName(name)

	sym, found := a.symbols.find(resolved)
	if found && local {
		sym.flags |= SymLocal
	}

	if found {
		sym.referenced = true
	}

	a.xrefSymbol(resolved)

	switch {
	case found && sym.defined():
		return sym.value, false, nil

	case (!found || a.dialect == DialectMACRO) && !allowForward:
		return 0, false, vmserrors.New(vmserrors.VAX_UNDEFSYM, name)

	case found && !allowForward:
		return sym.value, false, nil
	}

	a.queueFixup(location, fx, &rexpr{op: rSym, key: resolved})

	return 0, true, nil
}

// queueFixup records a fixup of kind fx at location in the current
// section for the value t, attaching it to each symbol t uses that isn't
// defined yet (creating the symbol, which the expression evaluator
// doesn't do, so an expression rejected partway through leaves no
// undefined symbol behind that would look defined with value 0). It
// becomes a.lastFixup, for the operand parsers that adjust a fixup after
// deciding the operand's final layout. A fixup waiting on no symbol (one
// using only a relocatable section's base) completes at the end of the
// statement (see flushReady).
func (a *Assembler) queueFixup(location uint32, fx fixupKind, t *rexpr) {
	f := &fixup{sect: a.cur, location: location, kind: fx, expr: t, base: a.caseBase, stmt: a.stmt}

	if a.dialect == DialectMACRO {
		f.line, f.where = a.listCur, a.where()
	}

	t.leaves(func(leaf *rexpr) {
		sym, found := a.symbols.find(leaf.key)
		if !found {
			sym = a.symbols.create(leaf.key)
			sym.firstSect = a.cur
		}

		sym.referenced = true
		leaf.sym = sym

		if len(sym.forward) == 0 || sym.forward[0] != f {
			sym.forward = append([]*fixup{f}, sym.forward...)
			f.pending++
		}
	})

	a.lastFixup = f

	if f.pending == 0 {
		a.ready = append(a.ready, f)
	}
}

// setSymbol defines (or redefines) name's value, applying flags and
// resolving any pending forward references against value — matching
// set_symbol(). If unique is true and the symbol already has a fully
// resolved definition, that's a duplicate-definition error (used by
// labels/.ENTRY/.SCOPE/.SHIM, which each require a fresh name).
func (a *Assembler) setSymbol(name string, value uint32, flags SymFlag, unique bool) error {
	return a.setSymbolIn(name, nil, value, flags, unique)
}

// defineHere defines name as the current location, as a label does: an
// address in an absolute section, or an offset in a relocatable one.
// Before any .PSECT, a label goes in . BLANK . (see useBlankPsect).
func (a *Assembler) defineHere(name string, flags SymFlag, unique bool) error {
	if a.dialect == DialectMACRO {
		a.useBlankPsect()
	}

	var sect *section
	if a.cur.relocatable {
		sect = a.cur
	}

	if err := a.setSymbolIn(name, sect, a.pc(), flags, unique); err != nil {
		return err
	}

	if a.dialect == DialectMACRO && sect == nil && a.cur.index != 0 {
		if resolved, _ := a.resolvedName(name); resolved != "" {
			if sym, ok := a.symbols.find(resolved); ok {
				sym.absSect = a.cur
			}
		}
	}

	return nil
}

// setSymbolIn is setSymbol for a value relative to the base of the
// relocatable section sect (nil for an absolute value).
func (a *Assembler) setSymbolIn(name string, sect *section, value uint32, flags SymFlag, unique bool) error {
	resolved, local := a.resolvedName(name)
	if local {
		flags |= SymLocal
	}

	sym, found := a.symbols.find(resolved)
	if unique && found && sym.defined() {
		// Real MACRO reports a label defined again on both lines and
		// goes on, the later definition giving its value (errors.lis).
		if a.dialect == DialectMACRO {
			a.outOfPhase(sym, name)
		}

		if err := a.recoverable(vmserrors.New(vmserrors.VAX_DUPSYM, name)); err != nil {
			return err
		}
	}

	if !found {
		sym = a.symbols.create(resolved)
	}

	if unique && a.dialect == DialectMACRO {
		sym.defLine, sym.defWhere = a.listCur, a.where()
	}

	a.xrefDefine(xrefSymbols, resolved)

	sym.value = value
	sym.sect = sect
	sym.absSect = nil
	sym.flags = sym.flags&^SymUndefined | flags
	sym.suppressed = a.enabled&enableSuppression != 0

	waiting := sym.forward
	sym.forward = nil

	for _, f := range waiting {
		f.pending--
		if f.pending > 0 {
			continue
		}

		if err := a.completeFixup(f); err != nil {
			return err
		}
	}

	return nil
}

// outOfPhase reports sym, a label being defined again, on the line that
// last defined it. Real MACRO finds it there in its second pass: by then
// the label has the later definition's value, which isn't the location
// of that line ("Symbol out of phase", errors.lis).
func (a *Assembler) outOfPhase(sym *symbol, name string) {
	if sym.defWhere == nil {
		return
	}

	err := vmserrors.New(vmserrors.VAX_OUTOFPHASE, name)
	a.errs = append(a.errs, sym.defWhere(err))

	if l := sym.defLine; l != nil {
		l.addNote(listNote{err: err, sect: l.sect, loc: l.loc, column: -1})
	}
}

// fixupRange reports err, fp's value (or displacement) v found too big
// for its field once it was known. In the MACRO dialect it's reported on
// the line that queued fixup, as real MACRO reports it there in its
// second pass, and fixupRange returns nil, so the field is stored
// truncated, as real MACRO stores it: a branch's out of range
// displacement is "Branch destination out of range" (BRDESTRANG,
// errors.lis), and any other value is truncated data. In the console
// dialect it returns err.
func (a *Assembler) fixupRange(fp *fixup, err error, v int64) error {
	if a.dialect != DialectMACRO || fp.where == nil {
		return err
	}

	if isBranch(fp.kind) {
		err = vmserrors.New(vmserrors.VAX_BRANCHRANGE, v)
	}

	a.errs = append(a.errs, fp.where(err))

	if l := fp.line; l != nil {
		l.addNote(listNote{err: err, sect: fp.sect, loc: fp.location, column: -1})
	}

	return nil
}

// applyFixup stores one fixup's value, now that it's known, matching
// set_symbol()'s fixup switch. fixCaseW measures the offset from the
// .CASE block's base rather than from the fixup's own location.
func (a *Assembler) applyFixup(fp *fixup, value uint32) error {
	img := fp.sect.img
	disp := int64(value) - int64(fp.location)

	if isDisplacement(fp.kind) {
		disp -= fixupSize(fp.kind)
	}

	switch fp.kind {
	case fixCaseW:
		d := int64(int32(value - fp.base))
		if d < math.MinInt16 || d > math.MaxInt16 {
			if err := a.fixupRange(fp, vmserrors.New(vmserrors.VAX_FWDWORD, d), d); err != nil {
				return err
			}
		}

		return img.storeWord(fp.location, uint16(int16(d)))

	case fixAddrB, fixSignedB:
		// A value, signed or unsigned, as .BYTE takes it.
		if v := int64(int32(value)); v < -128 || v > 0xFF {
			if err := a.fixupRange(fp, vmserrors.New(vmserrors.VAX_FWDBYTE, v), v); err != nil {
				return err
			}
		}

		return img.storeByte(fp.location, byte(value))

	case fixDispB, fixBranchB:
		if disp < -128 || disp > 127 {
			if err := a.fixupRange(fp, vmserrors.New(vmserrors.VAX_FWDBYTE, disp), disp); err != nil {
				return err
			}
		}

		return img.storeByte(fp.location, byte(int8(disp)))

	case fixAddrW, fixSignedW:
		if v := int64(int32(value)); v < -32768 || v > 0xFFFF {
			if err := a.fixupRange(fp, vmserrors.New(vmserrors.VAX_FWDWORD, v), v); err != nil {
				return err
			}
		}

		return img.storeWord(fp.location, uint16(value))

	case fixDispW, fixBranchW:
		if disp < -32768 || disp > 32767 {
			if err := a.fixupRange(fp, vmserrors.New(vmserrors.VAX_FWDWORD, disp), disp); err != nil {
				return err
			}
		}

		return img.storeWord(fp.location, uint16(int16(disp)))

	case fixAddrL, fixAddress:
		disp = int64(int32(value))

		fallthrough

	case fixDispL, fixBranchL:
		return img.storeLongword(fp.location, uint32(int32(disp)))
	}

	return vmserrors.New(vmserrors.VAX_INTERNAL, fmt.Sprintf("unhandled fixup kind %d", fp.kind))
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

// SymbolInfo is one entry returned by Symbols(): a symbol's value plus
// whether it was defined by .ENTRY (or a .SHIM stub) — SymEntry — so a
// caller merging these into its own symbol table (the console's ASM
// command) can preserve that attribute instead of losing it, which
// previously left the disassembler with no way to recognize a routine's
// register-save mask word (see decode_opcode.c's own SYM_ENTRY scan).
type SymbolInfo struct {
	Value uint32
	Entry bool
}

// Symbols returns every symbol this assembly itself defined (labels,
// .ENTRY points, .SET values, .SHIM stubs, ...) with no pending forward
// references — excluding the fixed set seeded by seedBuiltinSymbols at
// construction time (see SymBuiltin). Used by the console's ASM command
// (Phase 12) to merge a freshly assembled program's own symbol table into
// Console.Symbols once its bytes have been deposited into live memory.
func (a *Assembler) Symbols() map[string]SymbolInfo {
	out := make(map[string]SymbolInfo)

	for name, s := range a.symbols.byName {
		if s.flags&(SymBuiltin|SymLocalLabel) != 0 || !s.defined() {
			continue
		}

		out[name] = SymbolInfo{Value: s.value, Entry: s.flags&SymEntry != 0}
	}

	return out
}
