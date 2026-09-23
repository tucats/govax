// Package asm implements the MACRO-32-ish assembler and disassembler shared
// by the console's ASM/DISASM commands. See docs/PHASE-11.md.
package asm

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"

	"github.com/tucats/govax/internal/cpu"
)

// defaultOrigin is the P0 deposit/PC location a fresh assembly starts at,
// matching initialization.c's vax.console.deposit/p0_deposit default
// (0x200) — the first 512 bytes of a real VAX's low memory are reserved
// (interrupt/exception vectors, restart parameter block, ROM scratch), so
// ordinary code and data start just past them.
const defaultOrigin = 0x200

// defaultS0Base is the initial S0 deposit location, matching
// initialization.c's vax.console.s0_deposit — the base of S0 (system)
// virtual address space.
const defaultS0Base = 0x80000000

// Assembler holds everything one assembly (and any .INCLUDE files it pulls
// in) shares: the symbol table, output image, and location-counter/region
// state. Matches the reference tool's vax.assembler/vax.console fields,
// minus the console-interactive and live-VM-specific pieces Phase 11's
// batch-oriented Assemble doesn't need (see docs/PHASE-11.md).
type Assembler struct {
	table   *cpu.Table
	symbols *symbolTable
	image   *image

	// deposit is the current location counter (vax.console.deposit).
	deposit uint32
	// origin is the configured P0 base, used by Bytes() to know where the
	// assembled program's bytes start.
	origin uint32
	// regionIsS0 says which of p0Deposit/s0Deposit is "the other one" —
	// .REGION swaps the active deposit counter with its saved
	// counterpart, matching asm_pseudo.c's case 27.
	regionIsS0 bool
	p0Deposit  uint32
	s0Deposit  uint32
	// s0Origin is the configured S0 base, the S0 counterpart of origin —
	// see SetS0Origin.
	s0Origin uint32

	// curEntry is the active local-symbol scope name (vax.assembler.cur_entry).
	curEntry string
	tempSeq  int

	// caseBase is the running .CASE block's base address, or 0 when no
	// .CASE block is active (vax.assembler.case_base).
	caseBase uint32

	// lastSymbol is the most recently looked-up-or-defined symbol
	// (vax.console.last_symbol) — asm_operand.c's DISP(Rn) rewrite needs
	// to mutate the forward-reference fixup it just created.
	lastSymbol *symbol

	// entrySeen/entryAddr record whether .END named an explicit start
	// address (vax.assembler.flags & ASM_ENTRY), for callers that load the
	// assembled image and want to know where to start execution.
	entrySeen bool
	entryAddr uint32

	radix       int // 16 (default) or 10; see numericLiteral.
	microkernel bool
	scbb        uint32
	verbose     bool
	memSize     uint32

	// p1VectorBase/p1VectorEnd record the [base, end) byte range .P1VECTOR
	// deposited its trampolines across (real min/max address seen in
	// internal/p1vector's table, not a fixed constant), so depositAsmImage
	// (internal/console/asm.go) knows what to copy into live memory — the
	// same role a.scbb plays for .SCB/.VECTOR. p1VectorSet distinguishes
	// "never ran .P1VECTOR" from a coincidental zero range.
	p1VectorBase uint32
	p1VectorEnd  uint32
	p1VectorSet  bool

	// prints accumulates .PRINT output, one entry per statement — see
	// pseudoPrint's doc comment for why this is captured rather than
	// written to stdout directly.
	prints []string

	// includeResolver, when non-nil, resolves a .INCLUDE file name to its
	// source text. .INCLUDE reports an error if this is nil.
	includeResolver func(name string) (string, error)
	includeDepth    int

	// stop is set by .END to unwind out of the (possibly nested, via
	// .INCLUDE/.IF) line-processing loop.
	stop bool
}

// New returns an Assembler ready to assemble source, using the built-in VAX
// instruction table.
func New(verbose bool) *Assembler {
	a := &Assembler{
		table:     cpu.Instructions(),
		symbols:   newSymbolTable(),
		image:     newImage(),
		deposit:   defaultOrigin,
		origin:    defaultOrigin,
		p0Deposit: defaultOrigin,
		s0Deposit: defaultS0Base,
		s0Origin:  defaultS0Base,
		radix:     16,
		verbose:   verbose,
	}
	a.seedBuiltinSymbols()

	return a
}

// Prints returns the text of every .PRINT statement assembled so far, in
// order — see pseudoPrint's doc comment for why .PRINT is captured here
// rather than written to stdout.
func (a *Assembler) Prints() []string { return a.prints }

// SetOrigin sets the initial P0 deposit location (default 0x200, matching
// the reference tool). Only meaningful before Assemble is called.
func (a *Assembler) SetOrigin(addr uint32) {
	a.origin = addr
	a.deposit = addr
	a.p0Deposit = addr
}

// SetSCBB sets the SCB base register value .SCB/.VECTOR compute their
// target address from (default 0, matching a freshly reset VAX's SCBB
// privileged register).
func (a *Assembler) SetSCBB(v uint32) { a.scbb = v }

// SetMicrokernel enables (or disables) the .MICROKERNEL-gated pseudo-ops
// (.SCB/.VECTOR/.SHIM/.REGION/.P1VECTOR) without requiring a ".MICROKERNEL"
// statement in the source — useful for assembling a fragment that assumes
// it's already running in microkernel context.
func (a *Assembler) SetMicrokernel(enabled bool) { a.microkernel = enabled }

// SetVerbose sets the value the VERBOSE() expression function reports.
func (a *Assembler) SetVerbose(v bool) { a.verbose = v }

// SetMemSize sets the value the PMEMSIZE() expression function reports.
func (a *Assembler) SetMemSize(size uint32) { a.memSize = size }

// SetRadix sets the default numeric-literal radix (16 or 10); panics on any
// other value, matching this being a programming error, not a runtime one.
func (a *Assembler) SetRadix(radix int) {
	if radix != 10 && radix != 16 {
		panic("asm: radix must be 10 or 16")
	}

	a.radix = radix
}

// SetIncludeResolver installs the function .INCLUDE uses to resolve a file
// name to source text. Without one, .INCLUDE reports an error.
func (a *Assembler) SetIncludeResolver(resolve func(name string) (string, error)) {
	a.includeResolver = resolve
}

// Entry reports the address .END named as the program's start address (an
// expression after END, e.g. "END MAIN"), and whether one was given.
func (a *Assembler) Entry() (uint32, bool) { return a.entryAddr, a.entrySeen }

// TakeEntry is Entry, plus clearing the "an entry was named" flag it
// reports -- matching asm_pseudo.c's own ASM_ENTRY flag, which console.c's
// post-command hook clears (`vax.assembler.flags &= ~ASM_ENTRY`) the moment
// it fires the one-shot "CALL __ENTRY" this flag triggers. Since a bare
// ".END" with no name never sets the flag in the first place (see case 11
// in asm_pseudo.c) but doesn't clear it either, a persistent Assembler
// reused across several "ASM <file>" commands (internal/console/asm.go)
// needs this one-shot consumption so an earlier file's ".END name" doesn't
// spuriously re-trigger on a later, entry-less file.
func (a *Assembler) TakeEntry() (uint32, bool) {
	addr, ok := a.entryAddr, a.entrySeen
	a.entrySeen = false

	return addr, ok
}

// Origin returns the configured P0 base address (see SetOrigin), the
// address Bytes()'s returned span starts at.
func (a *Assembler) Origin() uint32 { return a.origin }

// Deposit returns the current active location counter (vax.console.deposit)
// — whichever of the P0/S0 counters .REGION has made active. A live
// console session (internal/console/asm.go) mirrors this into its own
// shared "current address" register (Console.DepositAddr) after every
// statement, matching the reference tool's own single shared field.
func (a *Assembler) Deposit() uint32 { return a.deposit }

// HasUnresolvedSymbols reports whether any symbol assembled so far still has
// pending forward references, matching check_unresolved_symbols(0) — the
// reference tool's interactive bare-END warns with this when ASM_WARNFORWARD
// (on by default) is set; see docs/PHASE-19.md.
func (a *Assembler) HasUnresolvedSymbols() bool { return a.hasUnresolvedSymbols() }

// BeginInteractive prepares the Assembler for a fresh interactive REPL
// session (the console's bare "ASM" command, docs/PHASE-19.md): clears the
// "assembly stopped" flag a previous interactive session's own END may have
// left set, exactly mirroring Assemble's own reset on every top-level call
// for the same reason — a persistent session (internal/console/asm.go's
// asmSession) is reused across multiple ASM invocations, batch or
// interactive, in a row.
func (a *Assembler) BeginInteractive() { a.stop = false }

// AssembleLine assembles one interactively-typed statement — the console's
// bare "ASM" REPL mode (docs/PHASE-19.md) — depositing directly into this
// Assembler's own image/symbol table exactly like one line of Assemble's own
// per-line loop. Reports done=true once a bare or dotted END statement has
// stopped assembly (matching assemble()'s single-statement entry point in
// the reference tool, which the interactive console prompt calls once per
// line read instead of pre-splitting a whole file).
func (a *Assembler) AssembleLine(line string) (done bool, err error) {
	line = preprocessLine(line)
	if line == "" {
		return a.stop, nil
	}

	if err := a.assembleStatement(line); err != nil {
		return false, err
	}

	return a.stop, nil
}

// Bytes returns the assembled P0-region program: the contiguous span from
// the configured origin to the final P0 deposit location. Addresses in
// that range that were never written (alignment padding, a forward .BASE
// jump) read back as zero.
//
// A microkernel-style source that spends most of its time in S0 (via
// .REGION) — kernel.asm, for instance — may leave the P0 region almost
// empty; use S0Origin/S0End with BytesRange to read that data instead, or
// ByteAt for a single address anywhere in the sparse image.
func (a *Assembler) Bytes() []byte {
	return a.image.Bytes(a.origin, a.p0End())
}

// BytesRange returns the contiguous span [from, to) from the assembled
// image, zero-filling any address never written — the general form of
// Bytes, for reading a region other than "the P0 program" (S0 data, an SCB
// entry, ...).
func (a *Assembler) BytesRange(from, to uint32) []byte {
	return a.image.Bytes(from, to)
}

// S0Origin returns the configured S0 base address: 0x80000000 by default,
// matching initialization.c's vax.console.s0_deposit, or whatever
// SetS0Origin last configured.
func (a *Assembler) S0Origin() uint32 { return a.s0Origin }

// SetS0Origin sets the initial S0 deposit location (default 0x80000000).
// Only meaningful before Assemble is called. A standalone assembly with no
// live VM backing it (e.g. internal/asm's own fixture tests) has no reason
// to move this, but a live console session (internal/console/asm.go) does:
// VMINIT's page tables, privileged stacks, and scratch pages already
// occupy low S0 addresses starting at the literal default, so depositing a
// program there would corrupt the running page table it's mapped through.
func (a *Assembler) SetS0Origin(addr uint32) {
	a.s0Origin = addr
	a.s0Deposit = addr

	if a.regionIsS0 {
		a.deposit = addr
	}
}

// S0End returns the final S0 deposit location — the S0 counterpart to
// Bytes' implicit P0 range end.
func (a *Assembler) S0End() uint32 {
	if a.regionIsS0 {
		return a.deposit
	}

	return a.s0Deposit
}

// ByteAt returns the single byte at addr in the assembled image (0 if
// nothing was ever deposited there).
func (a *Assembler) ByteAt(addr uint32) byte { return a.image.loadByte(addr) }

// P1VectorRange reports the [base, end) byte range .P1VECTOR deposited its
// trampolines across, and whether .P1VECTOR has run at all this assembly —
// see p1VectorBase's own doc comment.
func (a *Assembler) P1VectorRange() (base, end uint32, ok bool) {
	return a.p1VectorBase, a.p1VectorEnd, a.p1VectorSet
}

// p0End returns the final P0 deposit location, whichever counter — the
// active one, or the saved one from the last .REGION switch — currently
// holds it.
func (a *Assembler) p0End() uint32 {
	if a.regionIsS0 {
		return a.p0Deposit
	}

	return a.deposit
}

// Assemble assembles source (a full program, or one .INCLUDE-able
// fragment) statement by statement, in one pass, matching the reference
// tool's assemble()/console_dispatch() loop (one uppercased, comment-
// stripped line per statement — see preprocessLine). Returns the P0-region
// bytes assembled so far (see Bytes) unless a statement fails, in which
// case the error identifies the 1-based source line.
func (a *Assembler) Assemble(source string) ([]byte, error) {
	// A prior top-level Assemble call on this same Assembler (the console's
	// ASM command reuses one instance across multiple files -- see
	// internal/console/asm.go) may have left a.stop set by its own .END;
	// clear it here so this new source doesn't immediately no-op on its
	// first line. assembleLines itself must NOT do this reset, since
	// .INCLUDE calls it directly mid-assembly and relies on a still-set
	// a.stop (an .END inside an included file) unwinding the includer too.
	a.stop = false
	if err := a.assembleLines(source); err != nil {
		return nil, err
	}

	return a.Bytes(), nil
}

// assembleLines is Assemble's error-returning core, shared with .INCLUDE
// (which needs to report a failure without re-wrapping Bytes()).
func (a *Assembler) assembleLines(source string) error {
	a.includeDepth++
	defer func() { a.includeDepth-- }()

	if a.includeDepth > 64 {
		return vmserrors.New(vmserrors.VAX_INCLUDEDEPTH)
	}

	for i, raw := range strings.Split(source, "\n") {
		if a.stop {
			return nil
		}

		line := preprocessLine(raw)
		if line == "" {
			continue
		}

		if err := a.assembleStatement(line); err != nil {
			return &Error{Line: i + 1, Err: err}
		}
	}

	return nil
}

// Error identifies the 1-based source line a statement-level failure
// occurred on. .INCLUDE failures nest an inner *Error naming the line
// within the included file, itself wrapped with the line of the .INCLUDE
// statement, courtesy of assembleStatement's dot-pseudo-op handling.
type Error struct {
	Line int
	Err  error
}

func (e *Error) Error() string {
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// preprocessLine strips a trailing ";" comment and uppercases everything
// outside single- or double-quoted regions, matching parse.c's uppercase()
// — called once per line by the reference tool's console read loop, ahead
// of both ordinary command dispatch and assembler-mode dispatch, so every
// parser downstream of it can assume mnemonics/pseudo-ops/labels already
// arrived in uppercase while quoted string contents kept their original
// case.
func preprocessLine(line string) string {
	b := []byte(line)
	inDouble, inSingle := false, false
	out := b[:0]

	for i := 0; i < len(b); i++ {
		ch := b[i]

		if inDouble && ch == '"' && i > 0 && b[i-1] == '\\' {
			out = append(out, ch)

			continue
		}

		if ch == '\'' {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		}

		if !inSingle && !inDouble && ch == ';' {
			break
		}

		if !inSingle && !inDouble && ch >= 'a' && ch <= 'z' {
			ch -= 32
		}

		out = append(out, ch)
	}

	return strings.TrimRight(string(out), " \t\r")
}

// assembleStatement assembles one preprocessed line: an optional label,
// then either a pseudo-op or a real instruction — matching assemble()'s
// per-statement flow in asm.c (minus the interactive ASM-mode-toggle and
// END-command special cases, which Phase 11's batch Assemble doesn't need:
// a bare "END" always just ends the current assembleLines call).
func (a *Assembler) assembleStatement(line string) error {
	c := newCursor(line)
	c.skipBlanks()

	if c.atEnd() || c.peek() == '#' { // GNU-style leading "#" comment line
		return nil
	}

	if err := a.parseLabel(c); err != nil {
		return err
	}

	c.skipBlanks()

	if c.atEnd() {
		return nil
	}

	handled, err := a.assemblePseudo(c)
	if err != nil {
		return err
	}

	if handled {
		return nil
	}

	return a.assembleOpcode(c)
}

// parseLabel consumes a leading "NAME:" or "NAME::" label, if present, and
// defines it at the current deposit location — matching asm_label(). A
// line with no colon before the first blank/end is left untouched.
func (a *Assembler) parseLabel(c *cursor) error {
	c.skipBlanks()
	start := c.pos
	i := c.pos

	for i < len(c.s) {
		ch := c.s[i]
		if isBlank(ch) {
			return nil
		}

		if ch == ':' {
			break
		}

		i++
	}

	if i >= len(c.s) {
		return nil
	}

	name := c.s[start:i]
	flags := SymLabel

	end := i + 1
	if end < len(c.s) && c.s[end] == ':' {
		end++
		flags |= SymPermanent
	}

	c.pos = end

	return a.setSymbol(name, a.deposit, flags, true)
}
