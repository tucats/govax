package console

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
)

// Names from the loaded images' debug symbol tables (docs/PHASE-41.md,
// subtask 11): imageNames resolves a typed name to an address for the
// expression evaluator, and consoleSymbolizer names an address for the
// disassembler, as the VMS debugger names them.

// imageNames is the evaluator's DebugNames: the debug symbol tables of
// every image RUN loaded (ICB.Debug), main image first.
type imageNames struct {
	c *Console
}

// Lookup returns the value of a symbol or path name in the first image
// whose debug symbol table has it: a routine, label, data symbol, or
// constant in a module's DST, else a global in its GST.
func (n imageNames) Lookup(path string) (uint32, bool) {
	for _, icb := range n.c.ICBList {
		if icb.Debug == nil {
			continue
		}

		if s, ok := icb.Debug.Lookup(path); ok {
			return s.Value, true
		}

		if g := icb.Debug.Globals; g != nil && !strings.Contains(path, `\`) {
			if s, ok := g.Get(path); ok {
				return s.Value, true
			}
		}
	}

	return 0, false
}

// Line returns the address of line n's first instruction. With a scope
// (DBGSUB, or DBGDIS\START), the line is its module's, the first
// component naming the module (line numbers are a module's, so a
// routine in the path doesn't narrow them). With none, it's the line in
// the module holding the PC, as the debugger takes %LINE in the current
// scope, or else the first module, in load order, with code on that line
// (govax's choice when the PC is in no module: unconfirmed).
func (n imageNames) Line(scope string, line int) (uint32, bool) {
	if scope != "" {
		module, _, _ := strings.Cut(scope, `\`)

		for _, icb := range n.c.ICBList {
			if icb.Debug == nil {
				continue
			}

			if m, ok := icb.Debug.ModuleNamed(module); ok {
				return m.AddressOfLine(line)
			}
		}

		return 0, false
	}

	pc := n.c.CPU.GPR(vax.PC)

	for _, icb := range n.c.ICBList {
		if icb.Debug == nil {
			continue
		}

		if m, ok := icb.Debug.ModuleAt(pc); ok {
			if addr, ok := m.AddressOfLine(line); ok {
				return addr, true
			}
		}
	}

	for _, icb := range n.c.ICBList {
		if icb.Debug == nil {
			continue
		}

		for _, m := range icb.Debug.Modules {
			if addr, ok := m.AddressOfLine(line); ok {
				return addr, true
			}
		}
	}

	return 0, false
}

// consoleSymbolizer names addresses for DISASSEMBLE/SYMBOLIC, both an
// instruction's location and the addresses its operands refer to, in
// radix (16 or 10, for the offsets):
//
//  1. An address inside an image with a debug symbol table is named by
//     that table alone, by the debugger's rules (dbgsym.Program.Symbolize):
//     routine, label, data, array element, line, routine plus offset, and
//     the nearest global.
//  2. Any other address is named only by an exact match: a global some
//     image's GST gives that value (@#SYS$OPEN, the system service's
//     vector), a console symbol (the ASM command's labels, the shims), or
//     a universal symbol of a loaded shareable image (sharedSymbolizer).
//
// Rule 2's exactness is govax's choice: the debugger names an address
// past an image by its nearest global too, but the console's table holds
// constants as well as addresses, and NAME+offset from either would name
// unrelated addresses. Of the console's symbols, only those that are
// addresses name one (namesAddress), and page 0, never mapped, is never
// named by them.
type consoleSymbolizer struct {
	c     *Console
	radix int
}

// Symbolize implements disasm.Symbolizer.
func (s consoleSymbolizer) Symbolize(addr uint32) (string, bool) {
	if p := s.c.debugImageAt(addr); p != nil {
		return p.Symbolize(addr, s.radix)
	}

	for _, icb := range s.c.ICBList {
		if icb.Debug == nil || icb.Debug.Globals == nil {
			continue
		}

		if g, ok := icb.Debug.Globals.At(addr, nil); ok {
			return g.Name, true
		}
	}

	if addr >= vaxPageSize {
		if sym, ok := s.c.Symbols.Table().At(addr, namesAddress); ok {
			return sym.Name, true
		}
	}

	return sharedSymbolizer{c: s.c}.Symbolize(addr)
}

// lineName names addr by its line alone, MOD\ROUTINE\%LINE n, when an
// image's line table has a line starting there: how DISASSEMBLE names
// the first instruction of a range typed as a %LINE, as the debugger does.
func (s consoleSymbolizer) lineName(addr uint32) (string, bool) {
	if p := s.c.debugImageAt(addr); p != nil {
		return p.LineName(addr, s.radix)
	}

	return "", false
}

// namesAddress passes the console symbols that may name an address: an
// entry point or label (the ASM command's .ENTRY routines, the shims), or
// a user's own symbol. The system symbols that are neither are the
// constants kernel.asm and VMS define (SS$_ACCVIO, CONSOLE$STRINGPOOL_SIZE),
// and the assembler's predefined symbols are constants too.
func namesAddress(sym *Symbol) bool {
	if sym.IsBuiltin() {
		return false
	}

	return sym.IsEntry() || sym.IsLabel() || !sym.IsSystem()
}

// vaxPageSize is the size of a VAX page, in bytes.
const vaxPageSize = 0x200
