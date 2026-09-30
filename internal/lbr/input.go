package lbr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/obj"
)

// This file turns input files into modules as LIBRARIAN's input routines
// do (librar/lis/inputmac.lis and inputobj.lis): a macro source file into
// one module per macro, and an object file into one module per object
// module, with the global symbols each defines.

// Errors from reading input files.
var (
	// ErrNoMacro is LIB$_NOMACFOUND: a macro source file holds no .MACRO.
	ErrNoMacro = errors.New("no macro definition found")
	// ErrNoEndm is LIB$_NOMTCHENDM: a macro isn't finished by its file.
	ErrNoEndm = errors.New("no matching .ENDM")
)

// The directives the macro scan looks for, in inputmac's macro_names
// order.
const (
	kwMacro = iota
	kwRepeat
	kwRept
	kwIRP
	kwIRPC
	kwEndm
	kwEndr
	kwWarn
	kwError
	kwPrint
	kwIIF
)

var macroKeywords = []string{".MACRO", ".REPEAT", ".REPT", ".IRP", ".IRPC", ".ENDM", ".ENDR", ".WARN", ".ERROR", ".PRINT", ".IIF"}

// maxMacroNest is LIB$C_MAXNEST: how deeply .MACROs may nest.
const maxMacroNest = blockSize / 8

// MacroModules reads a macro source file's lines as LIBRARY/MACRO does:
// each outermost .MACRO through its matching .ENDM is a module named for
// the macro, and lines outside macros are skipped. squeeze (the default,
// /SQUEEZE) drops trailing blanks and comments from every line but the
// .MACRO line and .ERROR, .WARN, .PRINT, and .IIF lines, and drops lines
// the squeezing empties; an empty line stays, as an empty record. Like
// LIBRARIAN, it takes a comment to start at the line's last semicolon.
//
// Mismatched .ENDM names and repeat blocks are reported as warnings. An
// error ends the file: the modules read before it are still returned.
func (b *Builder) MacroModules(lines []string, squeeze bool) ([]*Entry, []error, error) {
	var (
		out   []*Entry
		warns []error
	)

	keepCase := b.flags[0]&IndexNoCaseEnt != 0

	name := func(tok string) (string, error) {
		if len(tok) > b.KeySize {
			return "", fmt.Errorf("macro name %s is longer than %d characters", tok, b.KeySize)
		}

		if tok == "" {
			return "", fmt.Errorf("a .MACRO names no macro")
		}

		if keepCase {
			return tok, nil
		}

		return upcase(tok), nil
	}

	for i := 0; ; {
		for ; i < len(lines); i++ {
			if kw, _ := scanMacroLine(lines[i]); kw == kwMacro {
				break
			}
		}

		if i == len(lines) {
			if len(out) == 0 {
				return nil, warns, ErrNoMacro
			}

			return out, warns, nil
		}

		_, tok := scanMacroLine(lines[i])

		top, err := name(tok)
		if err != nil {
			return out, warns, err
		}

		names := []string{top}
		repeats := 0
		e := &Entry{Name: top, Records: [][]byte{[]byte(lines[i])}}
		done := false

		for i++; i < len(lines) && !done; i++ {
			line := lines[i]
			kw, tok := scanMacroLine(line)

			switch kw {
			case kwMacro:
				if len(names)+1 >= maxMacroNest {
					return out, warns, fmt.Errorf("macro %s: .MACROs nest too deeply", top)
				}

				n, err := name(tok)
				if err != nil {
					return out, warns, err
				}

				names = append(names, n)

			case kwRepeat, kwRept, kwIRP, kwIRPC:
				repeats++

			case kwEndm:
				if tok == "" && repeats > 0 {
					repeats-- // an .ENDM ending a repeat block
				} else {
					if tok != "" && !keepCase {
						tok = upcase(tok)
					}

					if want := names[len(names)-1]; tok != "" && tok != want {
						warns = append(warns, fmt.Errorf(".ENDM %s ends macro %s", tok, want))
					}

					names = names[:len(names)-1]
				}

			case kwEndr:
				repeats--
			}

			if len(names) == 0 {
				done = true

				switch {
				case repeats > 0:
					warns = append(warns, fmt.Errorf("macro %s: %d repeat blocks have no .ENDR", top, repeats))
				case repeats < 0:
					warns = append(warns, fmt.Errorf("macro %s: too many .ENDRs", top))
				}
			}

			if squeeze && line != "" && (kw < kwWarn || kw > kwIIF) {
				if line = squeezeLine(line); line == "" {
					continue
				}
			}

			e.Records = append(e.Records, []byte(line))
		}

		if !done {
			return out, warns, fmt.Errorf("macro %s: %w", top, ErrNoEndm)
		}

		out = append(out, e)
	}
}

// scanMacroLine finds the directive a macro source line starts with, if
// it's one the macro scan cares about, and the word after it (inputmac's
// scan_line). A label is skipped; an assignment is nothing. It returns -1
// for anything else. As in inputmac, p is the last character read.
func scanMacroLine(line string) (int, string) {
	p := -1

	// skipBlanks reads past blanks and tabs, and reports whether it
	// reached something other than a comment or the end of the line.
	skipBlanks := func() bool {
		for p+1 < len(line) {
			p++

			switch line[p] {
			case ';':
				return false
			case ' ', '\t':
			default:
				return true
			}
		}

		return false
	}

	// scanWord returns the word starting at p, whatever its first
	// character: that and the symbol characters after it. It leaves p on
	// the character that ends the word, or on the line's last character.
	scanWord := func() string {
		start := p

		for p+1 < len(line) {
			p++

			if !isSymbolChar(line[p]) {
				return line[start:p]
			}
		}

		return line[start:]
	}

	for {
		if !skipBlanks() {
			return -1, ""
		}

		first := scanWord()
		last := line[p]

		if !skipBlanks() {
			return keyword(first), ""
		}

		if last == ':' {
			p-- // a label: scan what follows it

			continue
		}

		if line[p] == '=' {
			return -1, ""
		}

		return keyword(first), scanWord()
	}
}

func keyword(tok string) int {
	tok = upcase(tok)
	for i, k := range macroKeywords {
		if tok == k {
			return i
		}
	}

	return -1
}

func isSymbolChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '$' || c == '_'
}

// upcase upper-cases ASCII letters only, as the librarian does.
func upcase(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}

		return r
	}, s)
}

// squeezeLine is /SQUEEZE (elim_trail_blnk): trailing blanks go, then
// everything from the line's last semicolon on, then the blanks before it.
func squeezeLine(line string) string {
	line = strings.TrimRight(line, " \t")
	if i := strings.LastIndexByte(line, ';'); i >= 0 {
		line = strings.TrimRight(line[:i], " \t")
	}

	return line
}

// ObjectModules splits an object file's records into modules, one per
// module header through end of module, as LIBRARY/OBJECT does. Each
// module's index 2 keys are the global symbols it defines: symbol
// definitions that aren't weak, and entry points and procedures. Its
// header's user data holds MHD$M_OBJTIR if it has TIR records,
// MHD$M_SELSRC if selective is set (/SELECTIVE_SEARCH), and its ident.
func (b *Builder) ObjectModules(records [][]byte, selective bool) ([]*Entry, error) {
	if b.Type != TypeObject {
		return nil, fmt.Errorf("lbr: object modules go only in an object library")
	}

	var (
		out   []*Entry
		start int
	)

	for i, r := range records {
		if len(r) == 0 {
			return nil, fmt.Errorf("record %d is empty", i+1)
		}

		if t := obj.RecordType(r[0]); t != obj.RecEOM && t != obj.RecEOMW {
			continue
		}

		e, err := b.objectModule(records[start:i+1], selective)
		if err != nil {
			return nil, fmt.Errorf("object module %d: %w", len(out)+1, err)
		}

		out = append(out, e)
		start = i + 1
	}

	if start < len(records) || len(out) == 0 {
		return nil, fmt.Errorf("the object file doesn't end with an end of module record")
	}

	return out, nil
}

func (b *Builder) objectModule(records [][]byte, selective bool) (*Entry, error) {
	m, err := obj.Decode(records)
	if err != nil {
		return nil, err
	}

	h, ok := m.Records[0].(*obj.MainHeader)
	if !ok {
		return nil, fmt.Errorf("the first record isn't a module header")
	}

	if h.Name == "" || len(h.Name) > b.KeySize {
		return nil, fmt.Errorf("module name %q isn't 1 to %d characters", h.Name, b.KeySize)
	}

	e := &Entry{Name: h.Name, Records: records, UserData: make([]byte, b.userSize)}

	if selective {
		e.UserData[0] |= mhdSelectiveSearch
	}

	for _, rec := range m.Records {
		if rec.RecordType() == obj.RecTIR {
			e.UserData[0] |= mhdObjectTIR
		}
	}

	ident := h.Version[:min(len(h.Version), obj.MaxNameLength, b.userSize-2)]
	e.UserData[1] = byte(len(ident))
	copy(e.UserData[2:], ident)

	seen := map[string]bool{}

	for _, s := range m.Symbols() {
		if !indexedSymbol(s) || seen[s.Name] {
			continue
		}

		if s.Name == "" || len(s.Name) > b.KeySize {
			return nil, fmt.Errorf("module %s: symbol name %q isn't 1 to %d characters", h.Name, s.Name, b.KeySize)
		}

		seen[s.Name] = true
		e.Symbols = append(e.Symbols, s.Name)
	}

	return e, nil
}

// indexedSymbol reports whether LIBRARIAN puts a GSD symbol subrecord in
// the global symbol index: a symbol definition that isn't weak, or an
// entry point or procedure; never a module-local one.
func indexedSymbol(s *obj.Symbol) bool {
	switch s.Type {
	case obj.GSDLocalSymbol, obj.GSDLocalEntry, obj.GSDLocalProc:
		return false
	case obj.GSDSymbol, obj.GSDSymbolW, obj.GSDSymbolV, obj.GSDSymbolM:
		return s.Flags&obj.SymDEF != 0 && s.Flags&obj.SymWEAK == 0
	}

	return true
}
