package asm

import (
	"fmt"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements macro libraries (docs/PHASE-28.md, subtask 5):
// .MCALL, .LIBRARY, and MACRO's automatic search of its libraries for an
// undefined opcode.
//
// A macro library (.MLB) holds one module per macro, keyed by the macro's
// name; the module is the macro's source, .MACRO line to .ENDM, one record
// per line. VMS's system macros ($EXIT_S, $FAB, ...) are the modules of
// SYS$LIBRARY:STARLET.MLB. Using a library macro means defining it from
// that source (loadLibraryMacro) and then calling it like any other.
//
// The libraries are searched in MACRO's order: those .LIBRARY named, the
// last named first, then the caller's (SetMacroLibraries), in the order
// given. The caller puts /LIBRARY's files, the last first, and then
// STARLET.MLB there.

// MacroLibrary is a macro library the assembler can search.
type MacroLibrary interface {
	// Macro returns the source lines of the library's module for the
	// macro name (uppercase), and whether the library has one.
	Macro(name string) (lines []string, found bool, err error)
}

// lbrMacros is a MacroLibrary read from a librarian file.
type lbrMacros struct {
	l *lbr.Library
}

// NewMacroLibrary returns l, a macro library read by internal/lbr, as a
// MacroLibrary. It's an error if l isn't a macro library.
func NewMacroLibrary(l *lbr.Library) (MacroLibrary, error) {
	if l.Type != lbr.TypeMacro {
		return nil, fmt.Errorf("not a macro library: %s library", l.Type)
	}

	return lbrMacros{l: l}, nil
}

// Macro implements MacroLibrary.
func (m lbrMacros) Macro(name string) ([]string, bool, error) {
	rfa, ok := m.l.Lookup(name)
	if !ok {
		return nil, false, nil
	}

	mod, err := m.l.Module(rfa)
	if err != nil {
		return nil, false, err
	}

	lines := make([]string, len(mod.Records))
	for i, r := range mod.Records {
		lines[i] = string(r)
	}

	return lines, true, nil
}

// SetMacroLibraries sets the libraries searched after the ones .LIBRARY
// names, in the order they're searched.
func (a *Assembler) SetMacroLibraries(libs ...MacroLibrary) {
	a.libraries = libs
}

// SetLibraryResolver sets the function that opens the macro library a
// .LIBRARY directive names, as written (the file type defaults to .MLB,
// which is the resolver's to apply).
func (a *Assembler) SetLibraryResolver(resolve func(name string) (MacroLibrary, error)) {
	a.libraryResolver = resolve
}

// searchLibraries returns the source of the macro name from the first
// library, in search order, that has it.
func (a *Assembler) searchLibraries(name string) ([]string, bool, error) {
	search := make([]MacroLibrary, 0, len(a.dotLibraries)+len(a.libraries))

	for k := len(a.dotLibraries) - 1; k >= 0; k-- {
		search = append(search, a.dotLibraries[k])
	}

	search = append(search, a.libraries...)

	for _, lib := range search {
		lines, found, err := lib.Macro(name)
		if err != nil {
			return nil, false, vmserrors.Wrap(vmserrors.VAX_LIBREAD, err, name)
		}

		if found {
			return lines, true, nil
		}
	}

	return nil, false, nil
}

// loadLibraryMacro defines the macro name from the libraries, replacing
// any definition it has, and returns it, or nil if no library has it.
func (a *Assembler) loadLibraryMacro(name string) (*macroDef, error) {
	lines, found, err := a.searchLibraries(name)
	if err != nil || !found {
		return nil, err
	}

	// The module's lines are assembled as a source of their own, so its
	// .MACRO line (continuation lines and all) and .ENDM go through the
	// same path as a definition in the program. They are assembled in
	// the middle of the statement that needs the macro, so that
	// statement's comment, continuation state, and text as written are
	// kept.
	comment, continued, cased := a.comment, a.continued, a.cased
	a.continued = ""

	err = a.runSource(&sourceFrame{kind: sourceLibrary, name: name}, lines)

	a.comment, a.continued, a.cased = comment, continued, cased

	if err != nil {
		return nil, err
	}

	m, ok := a.macros[name]
	if !ok {
		return nil, vmserrors.New(vmserrors.VAX_UNDEFMACRO, name)
	}

	return m, nil
}

// assembleLibraryCall assembles the statement at c if its first word is
// a macro in a library: MACRO's automatic search, made for a name that
// isn't a directive, a macro already defined, or an opcode. It reports
// handled=false, leaving c untouched, if no library has the name.
func (a *Assembler) assembleLibraryCall(c *cursor) (handled bool, err error) {
	if len(a.libraries) == 0 && len(a.dotLibraries) == 0 {
		return false, nil
	}

	save := c.pos
	c.skipBlanks()

	name := scanName(c)
	if name == "" || a.isOpcode(name) || (!c.atEnd() && !isBlank(c.peek()) && c.peek() != ',') {
		c.pos = save

		return false, nil
	}

	m, err := a.loadLibraryMacro(name)
	if err != nil || m == nil {
		c.pos = save

		return err != nil, err
	}

	return true, a.expandMacro(m, c)
}

// isOpcode reports whether name is an instruction's mnemonic.
func (a *Assembler) isOpcode(name string) bool {
	if realName, ok := opcodeAliases[name]; ok {
		name = realName
	}

	return a.table.ByName(name) != nil
}

// pseudoMcall assembles .MCALL macro-name-list: each macro is defined from
// the libraries, even one already defined, or with an opcode's name
// (which is how a library macro replaces an instruction). A name no
// library has is an error.
func (a *Assembler) pseudoMcall(c *cursor) error {
	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.atEnd() {
			return nil
		}

		name := scanName(c)
		if name == "" {
			return vmserrors.New(vmserrors.VAX_MACRONAME, c.rest())
		}

		m, err := a.loadLibraryMacro(name)
		if err != nil {
			return err
		}

		if m == nil {
			return vmserrors.New(vmserrors.VAX_UNDEFMACRO, name)
		}
	}
}

// pseudoLibrary assembles .LIBRARY /file-spec/: the macro library the
// delimited string names is searched, ahead of those named before it
// and the caller's.
func (a *Assembler) pseudoLibrary(c *cursor) error {
	name, err := readDelimited(c)
	if err != nil {
		return err
	}

	c.skipBlanks()

	if !c.atEnd() {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	if a.libraryResolver == nil {
		return vmserrors.New(vmserrors.VAX_NOLIBRESOLVER, name)
	}

	lib, err := a.libraryResolver(name)
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_LIBRARY, err, name)
	}

	a.dotLibraries = append(a.dotLibraries, lib)

	return nil
}
