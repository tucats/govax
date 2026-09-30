// Package asm implements the MACRO-32-ish assembler and disassembler shared
// by the console's ASM/DISASM commands. See docs/PHASE-11.md.
package asm

import (
	"bytes"
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

	// dialect is the source language being assembled (see Dialect).
	dialect Dialect

	// p0 and s0 are the console dialect's two absolute sections, based at
	// the configured P0 origin (SetOrigin) and S0 origin (SetS0Origin).
	// cur is the one output goes to; .REGION switches it, matching
	// asm_pseudo.c's case 27, which swapped the active deposit counter
	// (vax.console.deposit) with its saved counterpart.
	p0  *section
	s0  *section
	cur *section
	// sections is every section, in the order they were defined.
	sections []*section
	// implicitAbs says a MACRO-dialect assembly is still in . ABS .
	// because nothing has chosen a psect yet (see useBlankPsect).
	implicitAbs bool
	// psectStack is .SAVE_PSECT's context stack.
	psectStack []psectContext

	// The MACRO dialect's module identification: .TITLE's module name
	// and comment, and .IDENT's string.
	title     string
	titleText string
	ident     string

	// enabled holds the .ENABLE/.DISABLE functions in effect (see
	// enableArgs), and defaultDisp the .DEFAULT DISPLACEMENT size in
	// bytes.
	enabled     enableFlags
	defaultDisp int
	// localBlockHeld says .ENABLE LOCAL_BLOCK is holding the local label
	// block open across labels and .PSECTs (see endLocalBlock).
	localBlockHeld bool

	// warnings holds the warnings assembly produced, each an *Error
	// naming its line, and line the line being assembled.
	warnings []error
	line     int
	// messages holds the MACRO dialect's .PRINT messages, and comment
	// the ";" comment of the statement being assembled (see
	// message.go).
	messages []string
	comment  string

	// relocs holds the values left for the linker (MACRO dialect only),
	// and ready the fixups waiting for the end of their statement to
	// become relocations (see flushReady).
	relocs []relocation
	ready  []*fixup
	// events is the MACRO dialect's output, in source order (see
	// outEvent).
	events []outEvent
	// cased is the statement statement last returned, with its case as
	// written (see preprocessCase).
	cased string
	// stmt numbers statements as they're assembled, so a store can tell
	// the fixups and relocations of an earlier statement it overwrites
	// (see claim).
	stmt int

	// curEntry is the active local-symbol scope name (vax.assembler.cur_entry).
	curEntry string
	tempSeq  int

	// localBlock numbers the current MACRO-32 local label block (see
	// closeLocalBlock); localUsed says whether any "n$" label has been
	// defined or referenced in it.
	localBlock int
	localUsed  bool

	// caseBase is the running .CASE block's base address, or 0 when no
	// .CASE block is active (vax.assembler.case_base).
	caseBase uint32

	// lastFixup is the fixup most recently queued (vax.console.last_symbol's
	// head fixup in the reference tool): the operand parsers adjust it once
	// they've decided the operand's final layout (see queueFixup).
	lastFixup *fixup

	// entrySeen/entryAddr record whether .END named an explicit start
	// address (vax.assembler.flags & ASM_ENTRY), for callers that load the
	// assembled image and want to know where to start execution.
	entrySeen bool
	entryAddr uint32
	// entrySect is the relocatable psect entryAddr is an offset in, for
	// a MACRO-dialect transfer address in one.
	entrySect *section

	// radix is the radix of a number with no radix operator: decimal,
	// as in MACRO-32, except inside a ^X<...>, ^O<...>, or ^B<...> group
	// (see unaryOperator). The reference tool's was hexadecimal, and
	// followed the console's SET RADIX.
	radix       int
	microkernel bool
	scbb        uint32
	verbose     bool
	memSize     uint32

	// p1VectorBase/p1VectorEnd record the [base, end) byte range .P1VECTOR
	// deposited its trampolines across (real min/max address seen in
	// internal/vmsdef's P1VectorTable, not a fixed constant), so depositAsmImage
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

	// sources is the source stack (see source.go): the program's text,
	// then each .INCLUDE file and macro expansion being assembled,
	// innermost last.
	sources []*sourceFrame

	// macros is the macro table, keyed by macro name (see macros.go), and
	// defining the definition whose lines are being collected, or nil.
	macros   map[string]*macroDef
	// libraries are the macro libraries searched after the ones
	// .LIBRARY names (dotLibraries, in the order named), and
	// libraryResolver opens those (see maclib.go).
	libraries       []MacroLibrary
	dotLibraries    []MacroLibrary
	libraryResolver func(name string) (MacroLibrary, error)
	defining *definition
	// createdLabel is the number of the next created local label a
	// macro call makes up (see bind).
	createdLabel int

	// errs collects the statement errors of a MACRO-dialect assembly,
	// which goes on after an error so that one assembly reports them all
	// (docs/PHASE-27.md, subtask 10). The console dialect stops at the
	// first error, as eVAX's assembler does.
	errs []error

	// stop is set by .END to unwind out of the (possibly nested, via
	// .INCLUDE/.IF) line-processing loop.
	stop bool

	// continued holds a statement continued onto the next line (see
	// statement), already preprocessed, without its trailing "-".
	continued string

	// cond holds the open conditional assembly blocks (see
	// conditional.go), innermost last.
	cond []condFrame
}

// New returns an Assembler ready to assemble source, using the built-in VAX
// instruction table.
func New(verbose bool) *Assembler {
	a := &Assembler{
		table:   cpu.Instructions(),
		symbols: newSymbolTable(),
		image:   newImage(),
		radix:   10,
		verbose: verbose,

		createdLabel: firstCreatedLabel,
	}
	a.p0 = a.newSection("P0", false, a.image, defaultOrigin)
	a.s0 = a.newSection("S0", false, a.image, defaultS0Base)
	a.cur = a.p0
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
	a.p0.base = addr
	a.p0.loc = 0
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
func (a *Assembler) Origin() uint32 { return a.p0.base }

// Deposit returns the current active location counter (vax.console.deposit)
// — whichever of the P0/S0 counters .REGION has made active. A live
// console session (internal/console/asm.go) mirrors this into its own
// shared "current address" register (Console.DepositAddr) after every
// statement, matching the reference tool's own single shared field.
func (a *Assembler) Deposit() uint32 { return a.pc() }

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
func (a *Assembler) BeginInteractive() {
	a.stop = false
	a.continued = ""
	a.cond = nil
	a.defining = nil
}

// AssembleLine assembles one interactively-typed statement — the console's
// bare "ASM" REPL mode (docs/PHASE-19.md) — depositing directly into this
// Assembler's own image/symbol table exactly like one line of Assemble's own
// per-line loop. Reports done=true once a bare or dotted END statement has
// stopped assembly (matching assemble()'s single-statement entry point in
// the reference tool, which the interactive console prompt calls once per
// line read instead of pre-splitting a whole file).
func (a *Assembler) AssembleLine(line string) (done bool, err error) {
	// A macro definition typed at the prompt collects the lines that
	// follow it, up to its .ENDM.
	if a.defining != nil {
		return false, a.collectDefinition(line)
	}

	line, ok := a.statement(line)
	if !ok || line == "" {
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
//
// In the MACRO dialect it returns the current psect's contents instead,
// with zeros where a relocation's value goes.
func (a *Assembler) Bytes() []byte {
	if a.dialect == DialectMACRO {
		return a.cur.img.Bytes(0, a.cur.hi)
	}

	return a.image.Bytes(a.p0.base, a.p0End())
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
func (a *Assembler) S0Origin() uint32 { return a.s0.base }

// SetS0Origin sets the initial S0 deposit location (default 0x80000000).
// Only meaningful before Assemble is called. A standalone assembly with no
// live VM backing it (e.g. internal/asm's own fixture tests) has no reason
// to move this, but a live console session (internal/console/asm.go) does:
// VMINIT's page tables, privileged stacks, and scratch pages already
// occupy low S0 addresses starting at the literal default, so depositing a
// program there would corrupt the running page table it's mapped through.
func (a *Assembler) SetS0Origin(addr uint32) {
	a.s0.base = addr
	a.s0.loc = 0
}

// S0End returns the final S0 deposit location — the S0 counterpart to
// Bytes' implicit P0 range end.
func (a *Assembler) S0End() uint32 { return a.s0.addr() }

// ByteAt returns the single byte at addr in the assembled image (0 if
// nothing was ever deposited there).
func (a *Assembler) ByteAt(addr uint32) byte { return a.image.loadByte(addr) }

// P1VectorRange reports the [base, end) byte range .P1VECTOR deposited its
// trampolines across, and whether .P1VECTOR has run at all this assembly —
// see p1VectorBase's own doc comment.
func (a *Assembler) P1VectorRange() (base, end uint32, ok bool) {
	return a.p1VectorBase, a.p1VectorEnd, a.p1VectorSet
}

// p0End returns the final P0 deposit location, whether or not P0 is the
// section .REGION has made active.
func (a *Assembler) p0End() uint32 { return a.p0.addr() }

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
	a.continued = ""
	a.cond = nil
	a.errs = nil
	a.defining = nil
	a.createdLabel = firstCreatedLabel
	a.dotLibraries = nil

	err := a.assembleLines(source)

	if err == nil && len(a.cond) > 0 {
		err = vmserrors.New(vmserrors.VAX_NOENDC, len(a.cond))
	}

	if err == nil && a.dialect == DialectMACRO {
		err = a.finish()
	}

	if err != nil {
		a.errs = append(a.errs, err)
	}

	switch len(a.errs) {
	case 0:
		return a.Bytes(), nil
	case 1:
		return nil, a.errs[0]
	default:
		return nil, &Errors{List: a.errs}
	}
}

// Errors is every error a MACRO-dialect assembly found, in source order
// (the errors found when assembly ends, such as undefined symbols,
// last). An assembly with one error returns that error alone.
type Errors struct {
	List []error
}

// Error lists the errors, one per line.
func (e *Errors) Error() string {
	lines := make([]string, len(e.List))
	for i, err := range e.List {
		lines[i] = err.Error()
	}

	return strings.Join(lines, "\n")
}

// Unwrap returns the errors, for errors.Is and errors.As.
func (e *Errors) Unwrap() []error { return e.List }

// statement preprocesses one source line, joining MACRO-32 continuation
// lines: a statement whose last character before any comment is "-"
// continues on the next line. It reports false while a statement is
// still being continued. The reference tool had no continuation lines.
// The statement's comment (its last line's) is kept in a.comment.
func (a *Assembler) statement(raw string) (string, bool) {
	// Already-preprocessed text is unchanged by preprocessing it again,
	// and a.continued is kept with its case (see preprocessCase).
	line, cased, comment := preprocessCase(a.continued + raw)
	a.continued = ""
	a.comment = comment

	if strings.HasSuffix(line, "-") {
		a.continued = strings.TrimSuffix(cased, "-")

		return "", false
	}

	a.cased = cased

	return line, true
}

// caseCursor returns a cursor at c's position in the statement as
// written (see preprocessCase), or c itself if c isn't reading the
// statement statement last returned.
func (a *Assembler) caseCursor(c *cursor) *cursor {
	if len(a.cased) != len(c.s) || !strings.EqualFold(a.cased, c.s) {
		return c
	}

	return &cursor{s: a.cased, pos: c.pos}
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
//
// A string directive's operands (.ASCIC/.ASCID/.ASCII/.ASCIZ) are
// scanned by preprocessStrings instead, since MACRO-32 lets any printing
// character delimit their strings (.ASCII /text/). The reference tool
// also opened a single-quoted region at an apostrophe inside a
// double-quoted string ("don't"), so the rest of the line, comment
// included, was neither uppercased nor stripped.
func preprocessLine(line string) string {
	out, _ := preprocessComment(line)

	return out
}

// preprocessComment is preprocessLine, also returning the text of the
// line's ";" comment as written, without the ";" (the message of .ERROR,
// .WARN, and .PRINT; see message.go). A string directive's or .TITLE's
// comment isn't returned, since neither statement has a use for it.
func preprocessComment(line string) (string, string) {
	out, _, comment := preprocessCase(line)

	return out, comment
}

// preprocessCase is preprocessComment, also returning the preprocessed
// text with its case as written: the same text, character for character,
// except that the letters preprocessing uppercases keep their case. A
// macro call's arguments are read from it (see caseCursor), since real
// MACRO passes them as written, and one may land in a string.
func preprocessCase(line string) (string, string, string) {
	b := []byte(line)
	inDouble, inSingle := false, false
	out := make([]byte, 0, len(b))
	raw := make([]byte, 0, len(b))
	comment := ""

	// same appends what out gained since n to raw too.
	same := func(n int) { raw = append(raw, out[n:]...) }

	for i := 0; i < len(b); i++ {
		ch := b[i]

		if inDouble && ch == '"' && i > 0 && b[i-1] == '\\' {
			out = append(out, ch)

			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		}

		if !inSingle && !inDouble {
			if ch == ';' {
				comment = string(b[i+1:])

				break
			}

			if n := stringDirectiveAt(b, i); n > 0 {
				start := len(out)
				out = append(out, bytes.ToUpper(b[i:i+n])...)
				out = preprocessStrings(out, b[i+n:])
				same(start)

				break
			}

			if n := titleDirectiveAt(b, i); n > 0 {
				start := len(out)
				out = append(out, preprocessTitle(b[i:], n)...)
				same(start)

				break
			}

			if n := asciiOperatorAt(b, i); n > 0 {
				start := len(out)
				out = append(out, '^', 'A')
				out = append(out, b[i+2:i+n]...)
				same(start)
				i += n - 1

				continue
			}

			raw = append(raw, ch)

			if ch >= 'a' && ch <= 'z' {
				ch -= 32
			}

			out = append(out, ch)

			continue
		}

		raw = append(raw, ch)
		out = append(out, ch)
	}

	return strings.TrimRight(string(out), " \t\r"), strings.TrimRight(string(raw), " \t\r"), comment
}

// stringDirectiveAt returns the length of a string directive's name
// (".ASCIC", ".ASCID", ".ASCII", or ".ASCIZ", in any case, the "." being
// optional as for every directive here; see assemblePseudo), or of
// ".IDENT" or ".LIBRARY", whose operands are delimited strings too,
// starting at b[i] as a whole word, or 0 if there isn't one.
func stringDirectiveAt(b []byte, i int) int {
	if i > 0 && !isBlank(b[i-1]) && b[i-1] != ':' {
		return 0
	}

	n := 0
	if b[i] == '.' {
		n = 1
	}

	switch {
	case n == 1 && i+6 <= len(b) && strings.EqualFold(string(b[i+1:i+6]), "IDENT"):
		n += 5
	case n == 1 && i+8 <= len(b) && strings.EqualFold(string(b[i+1:i+8]), "LIBRARY"):
		n += 7
	case i+n+5 > len(b) || !strings.EqualFold(string(b[i+n:i+n+4]), "ASCI") || !strings.ContainsRune("CDIZcdiz", rune(b[i+n+4])):
		return 0
	default:
		n += 5
	}

	if i+n < len(b) && !isBlank(b[i+n]) {
		return 0
	}

	return n
}

// preprocessTitle preprocesses a .TITLE, .SUBTITLE, or .SBTTL statement,
// b, whose name is n bytes long: the name, and .TITLE's module name, are
// uppercased, and the text after them is kept as written, up to a ";"
// comment.
func preprocessTitle(b []byte, n int) []byte {
	out := bytes.ToUpper(b[:n])
	rest := b[n:]

	if bytes.EqualFold(out, []byte(".TITLE")) {
		i := 0
		for i < len(rest) && isBlank(rest[i]) {
			i++
		}

		for i < len(rest) && !isBlank(rest[i]) && rest[i] != ';' {
			i++
		}

		out = append(out, bytes.ToUpper(rest[:i])...)
		rest = rest[i:]
	}

	if semi := bytes.IndexByte(rest, ';'); semi >= 0 {
		rest = rest[:semi]
	}

	return append(out, rest...)
}

// preprocessStrings appends a string directive's operands to out: each
// delimited string is copied as written, each <expression> is processed
// as a line of its own, and a ";" between them starts the comment.
func preprocessStrings(out, b []byte) []byte {
	for i := 0; i < len(b); i++ {
		ch := b[i]

		switch {
		case ch == ';':
			return out

		case isBlank(ch):
			out = append(out, ch)

		case ch == '<':
			// An expression, up to its matching '>', processed like the
			// rest of a line (a character literal or ^A keeps its case).
			depth, j := 0, i

			for ; j < len(b); j++ {
				if b[j] == '<' {
					depth++
				} else if b[j] == '>' {
					depth--
					if depth == 0 {
						break
					}
				}
			}

			if j < len(b) {
				j++
			}

			out = append(out, preprocessLine(string(b[i:j]))...)
			i = j - 1

		case isStringDelimiter(ch):
			out = append(out, ch)

			for i++; i < len(b) && b[i] != ch; i++ {
				if b[i] == '\\' && i+1 < len(b) {
					out = append(out, b[i])
					i++
				}

				out = append(out, b[i])
			}

			if i < len(b) {
				out = append(out, ch)
			}

		default:
			out = append(out, bytes.ToUpper(b[i:i+1])...)
		}
	}

	return out
}

// asciiOperatorAt returns the length of a MACRO-32 ^A/text/ operator
// starting at b[i], through its closing delimiter, or 0 if there isn't
// one there.
func asciiOperatorAt(b []byte, i int) int {
	if i+3 >= len(b) || b[i] != '^' || (b[i+1] != 'A' && b[i+1] != 'a') {
		return 0
	}

	q := b[i+2]
	if isBlank(q) || q == ';' {
		return 0
	}

	for j := i + 3; j < len(b); j++ {
		if b[j] == q {
			return j - i + 1
		}
	}

	return 0
}

// isStringDelimiter reports whether ch can delimit a string directive's
// string: MACRO-32 accepts any printing character but blank, '=', ';', or
// '<' (letters and digits too, though its manual advises against them).
func isStringDelimiter(ch byte) bool {
	if ch <= ' ' || ch > '~' {
		return false
	}

	return !strings.ContainsRune("<;=", rune(ch))
}

// assembleStatement assembles one preprocessed line: an optional label,
// then either a pseudo-op or a real instruction — matching assemble()'s
// per-statement flow in asm.c (minus the interactive ASM-mode-toggle and
// END-command special cases, which Phase 11's batch Assemble doesn't need:
// a bare "END" always just ends the current assembleLines call).
func (a *Assembler) assembleStatement(line string) error {
	if err := a.assembleStatementBody(line); err != nil {
		return err
	}

	return a.flushReady()
}

// assembleStatementBody is assembleStatement before its fixups are
// flushed.
func (a *Assembler) assembleStatementBody(line string) error {
	a.stmt++

	if a.skipping() {
		return a.skippedStatement(line)
	}

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

	if handled, err := a.assembleAssignment(c); handled || err != nil {
		return err
	}

	handled, err := a.assemblePseudo(c)
	if err != nil {
		return err
	}

	if handled {
		return nil
	}

	// A macro call; macros come before instructions, so a macro may
	// replace an instruction of the same name.
	if handled, err := a.assembleMacroCall(c); handled || err != nil {
		return err
	}

	// A name that's none of the above may be a library's macro.
	if handled, err := a.assembleLibraryCall(c); handled || err != nil {
		return err
	}

	if err := a.assembleOpcode(c); err != nil {
		return err
	}

	// The reference tool ignored anything after the last operand it
	// expected, so "MOVL R0, R1 R2" assembled as "MOVL R0, R1".
	c.skipBlanks()

	if !c.atEnd() {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.s[c.pos:])
	}

	return nil
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
		flags |= SymPermanent | SymGlobal
	}

	c.pos = end

	if !isLocalLabel(name) {
		if err := a.endLocalBlock(); err != nil {
			return err
		}
	} else if flags&SymGlobal != 0 && a.dialect == DialectMACRO {
		return vmserrors.New(vmserrors.VAX_NOTGLOBAL, name)
	}

	return a.defineHere(name, flags, true)
}

// assembleAssignment handles a MACRO-32 direct assignment statement:
// "NAME = expression" or "NAME == expression" (MACRO-32's global form),
// or ". = expression" to move the location counter. Reports
// handled=false, leaving c untouched, if the statement isn't one.
func (a *Assembler) assembleAssignment(c *cursor) (handled bool, err error) {
	save := c.pos

	var name string

	if c.peek() == '.' && !isSymbolChar(c.peekAt(1)) {
		c.next()

		name = "."
	} else {
		name = scanName(c)
	}

	c.skipBlanks()

	// A symbol may start with "." (".LEN = 1"), but a directive's name
	// is never a symbol: ".ASCII =abc=" is the directive, with "=" as its
	// string's delimiter.
	_, directive := directives[strings.TrimPrefix(name, ".")]

	if name == "" || c.peek() != '=' || (name != "." && strings.HasPrefix(name, ".") && directive) {
		c.pos = save

		return false, nil
	}

	c.next()

	flags := SymNone

	if c.peek() == '=' {
		c.next()

		flags = SymGlobal
	}

	x, err := a.exprKnown(c)
	if err != nil {
		return true, err
	}

	if x.known() {
		if name == "." {
			a.setPC(x.v)

			return true, nil
		}

		return true, a.setSymbol(name, x.v, flags, false)
	}

	// A relocatable value: a label plus or minus a constant (the MACRO
	// manual, §3.5). "." can only move within its own section.
	sect, offset, ok := x.x.simpleRelocatable()
	
	switch {
	case !ok, name == "." && sect != a.cur:
		return true, vmserrors.New(vmserrors.VAX_RELEXPR)

	case name == ".":
		a.setPC(offset)

		return true, nil
	}

	return true, a.setSymbolIn(name, sect, offset, flags, false)
}
