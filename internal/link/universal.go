package link

import (
	"bytes"
	"sort"

	"github.com/tucats/govax/internal/obj"
)

// This file is a shareable image's universal symbols: the symbols other
// images may be linked against, which its global symbol table (GST)
// lists (the Linker manual, 4.2; testdata/link68's probe, docs/PHASE-68 -
// link shareable images.md, subtask 6). A symbol is universal when an
// options file names it (UNIVERSAL=), or when a module's .TRANSFER
// redefines it (TIR$C_OPR_REDEF), as a transfer vector's entries do.
//
// A universal symbol's GST value is an offset in the image, flagged
// relocatable (GSY$V_REL), which the image activator adds the image's
// base to: a program linked against the image stores that offset in its
// G^ cells. A .TRANSFER'd symbol's value there is its transfer vector
// entry, the location of the .TRANSFER, so that the routine can move
// within the image without relinking the programs that call it. Inside
// the image, references to it still reach the routine itself.

// noteUniversals marks the symbols UNIVERSAL= names, once pass 1 has
// read every module. Each is a reference too, so one nothing defines is
// reported with the link's other undefined symbols, and the image is
// still made, as real LINK's E1 was. An executable has no universal
// symbols, and its link takes UNIVERSAL= silently (E8).
func (l *linker) noteUniversals() {
	if !l.opts.Shareable {
		return
	}

	for _, name := range l.opts.Universals {
		g := l.refer(name, "")
		g.universal, g.strongRef = true, true
	}
}

// redefine is .TRANSFER's OPR_REDEF in pass 2: in a shareable image,
// the symbol becomes universal, with the location as its value outside
// the image. An executable link ignores it, as real LINK does
// (testdata/link68's E2).
func (x *machine) redefine(name string) {
	if !x.l.opts.Shareable {
		return
	}

	g := x.l.refer(name, "")
	g.universal, g.vectored, g.vector = true, true, x.loc
}

// universalValue is universal symbol g's value in the global symbol
// table: its transfer vector entry when a .TRANSFER gave it one, else its
// own value.
func (g *global) universalValue() uint32 {
	if g.vectored {
		return g.vector
	}

	return g.value
}

// universalSymbols returns the universal symbols the shareable image's
// global symbol table lists, in its order. A universal symbol nothing
// defined isn't listed.
//
// The order is real LINK's in all the probe's shareable images (SHR1,
// SHR1A, SHR2, SHR3, SHR4, SHR5, SHR6), by a rule that is unconfirmed:
// absolute symbols first, then the program sections in address order,
// each with its entry points first, then its other symbols, each group
// in descending order of value, which is the reverse of the order the
// modules defined them in. SHR1's SHR1_CODE gives SHR1_SHOW, SHR1_ADD,
// and SHR1_BASE (entry points at offsets ^X22, 7, and 0), then SHR1_JSB
// and SHR1_ORIGIN (^X16 and 0), and its other psects' symbols come
// before and after, by their psects' addresses. A symbol's place is
// its own definition's: a .TRANSFER'd routine is ordered by the routine,
// not its vector. The order of equal values of a kind, and of symbols
// several modules define in one psect, isn't known: those are by the
// reverse of their definitions.
func (l *linker) universalSymbols() []*global {
	var out []*global

	for _, g := range l.symbols {
		if g.universal && g.defined && g.image == "" {
			out = append(out, g)
		}
	}

	// place is a symbol's psect address, below every psect's for an
	// absolute symbol.
	place := func(g *global) (bool, uint32) {
		if g.contrib == nil {
			return false, 0
		}

		return true, g.contrib.psect.base
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]

		ra, pa := place(a)
		rb, pb := place(b)

		switch {
		case ra != rb:
			return !ra
		case pa != pb:
			return pa < pb
		case a.entry != b.entry:
			return a.entry
		case a.value != b.value:
			return a.value > b.value
		}

		return a.defSeq > b.defSeq
	})

	return out
}

// gstEntry returns the GSD entry the global symbol table gives symbol g.
// In an executable's table (linked /DEBUG) every symbol is absolute,
// with its final value. In a shareable image's, each is universal, and
// one in the image's sections is relocatable, an offset in the image,
// unless its cluster has a base of its own: a based image's symbols are
// all absolute (SHR3's).
func (l *linker) gstEntry(g *global) *obj.Symbol {
	s := &obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF, Value: g.value, Name: g.name}

	if l.opts.Shareable {
		s.Flags |= obj.SymUNI
		s.Value = g.universalValue()

		if c := g.contrib; c != nil && (c.psect.cluster == nil || !c.psect.cluster.based) {
			s.Flags |= obj.SymREL
		}
	}

	if g.entry {
		s.Type, s.Mask = obj.GSDEntry, g.mask
	}

	return s
}

// warned says the link reported a warning: an undefined symbol, or a
// reference to one.
func (l *linker) warned() bool {
	for _, m := range l.messages {
		if m.Severity == 'W' {
			return true
		}
	}

	for _, m := range l.modules {
		for _, msg := range m.messages {
			if msg.Severity == 'W' {
				return true
			}
		}
	}

	return false
}

// SymbolTable returns the image's global symbol table as an object
// module's records, which LINK/SYMBOL_TABLE writes as a symbol table
// file (.STB): real LINK's SHR1.STB holds SHR1.EXE's table record for
// record. An executable's is the table it would have linked /DEBUG: its
// global symbols, absolute (what real LINK writes for an executable is
// unconfirmed).
func (img *Image) SymbolTable() ([][]byte, error) {
	gst := img.l.gst
	if gst == nil {
		var err error
		if gst, _, err = img.l.globalSymbolTable(); err != nil {
			return nil, err
		}
	}

	return obj.ReadRecords(bytes.NewReader(gst))
}
