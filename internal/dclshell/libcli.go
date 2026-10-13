package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// What the LIB$ CLI routines ask of a process's DCL (docs/PHASE-57):
// each process with a CLI gets a shellCLI bound to its own Shell
// (Environment.CLI), set by newProcessOne for process 1 and by
// cliHost.Start for a subprocess. A subprocess's image calls it on the
// engine's goroutine while the subprocess's interpreter waits for the
// image (subcli.go's strict handoff), so it never races the interpreter.

var (
	libNoSuchSym = vmsdef.LibrarySymbols["LIB$_NOSUCHSYM"]
	libAmbSymDef = vmsdef.LibrarySymbols["LIB$_AMBSYMDEF"]
)

// shellCLI is one process's DCL as corevms.CLI: its Shell, and the
// interpreter all the processes' DCL share.
type shellCLI struct {
	c     *Interpreter
	shell *Shell
}

var _ corevms.CLI = shellCLI{}

// Symbol is corevms.CLI's: name looked up as DCL looks a symbol up.
func (s shellCLI) Symbol(name string) (string, uint32, uint32) {
	sym, table, ok := s.shell.dclSymbols.find(name)
	if !ok {
		return "", 0, libNoSuchSym
	}

	return sym.value, table, ssNormal
}

// SetSymbol is corevms.CLI's: a string symbol in the table given.
func (s shellCLI) SetSymbol(name, value string, table uint32) uint32 {
	return s.shell.dclSymbols.define(name, value, table == corevms.CLIGlobalSymbol)
}

// DeleteSymbol is corevms.CLI's: the symbol name means, from the table
// given.
func (s shellCLI) DeleteSymbol(name string, table uint32) uint32 {
	t := s.shell.dclSymbols.local()
	if table == corevms.CLIGlobalSymbol {
		t = s.shell.dclSymbols.globals()
	}

	sym, ok := t.lookup(name)
	if !ok {
		return libNoSuchSym
	}

	delete(t, sym.name)

	return ssNormal
}

// find returns the symbol word means, searching the tables in DCL's
// order (searchOrder), and which kind of table it was in:
// LIB$K_CLI_LOCAL_SYM for any command level's local table,
// LIB$K_CLI_GLOBAL_SYM for the global one.
func (t *dclSymbolTable) find(word string) (dclSymbol, uint32, bool) {
	for i, table := range t.searchOrder() {
		if sym, ok := table.lookup(word); ok {
			if i < len(t.levels) {
				return sym, corevms.CLILocalSymbol, true
			}

			return sym, corevms.CLIGlobalSymbol, true
		}
	}

	return dclSymbol{}, 0, false
}

// define makes name (uppercase; an "*" in it marks the shortest
// abbreviation) a string symbol with value, in the global table or the
// current command level's local one, replacing a symbol of the same full
// name there: LIB$SET_SYMBOL. It returns SS$_NORMAL, or LIB$_AMBSYMDEF
// when some abbreviation the new symbol allows would also be one another
// symbol of that table allows (both "AB*CDEF" and "ABC", say), which
// would make that abbreviation mean two symbols. DCL's own assignments
// don't check that; which names VMS finds ambiguous is unconfirmed (a
// probe case).
func (t *dclSymbolTable) define(name, value string, global bool) uint32 {
	full := strings.Replace(name, "*", "", 1)

	minLength := len(full)
	if star := strings.IndexByte(name, '*'); star > 0 {
		minLength = star
	}

	table := t.local()
	if global {
		table = t.globals()
	}

	for _, sym := range table {
		if sym.name != full && commonPrefix(sym.name, full) >= max(minLength, sym.minLength) {
			return libAmbSymDef
		}
	}

	table[full] = dclSymbol{name: full, minLength: minLength, value: value, global: global}

	return ssNormal
}

// commonPrefix returns the length of the longest prefix a and b share.
func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}

	return n
}

// Chain is corevms.CLI's: text is run once the image has exited
// (dclchain.go).
func (s shellCLI) Chain(text string, program bool) {
	s.shell.chained = &chainedCommand{text: text, program: program}
}

// ControlKeys is corevms.CLI's: SET CONTROL for the keys enable names,
// then SET NOCONTROL for those disable names, each a LIB$M_CLI_CTRLT and
// LIB$M_CLI_CTRLY mask. It returns the keys enabled before, in the same
// form. Disabling Ctrl/Y cancels the ON CONTROL_Y of the command level
// that ran the image, as SET NOCONTROL=Y does (unconfirmed, a probe
// case).
func (s shellCLI) ControlKeys(enable, disable uint32) uint32 {
	var old uint32

	if s.shell.control.t {
		old |= corevms.CLICtrlT
	}

	if !s.shell.control.noY {
		old |= corevms.CLICtrlY
	}

	s.shell.setControl(enable&corevms.CLICtrlT != 0, enable&corevms.CLICtrlY != 0, true)
	s.shell.setControl(disable&corevms.CLICtrlT != 0, disable&corevms.CLICtrlY != 0, false)

	if s.c != nil {
		s.c.noteControlKeys()
	}

	return old
}
