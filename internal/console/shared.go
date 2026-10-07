package console

import (
	"sort"
	"sync"

	"github.com/tucats/govax/internal/vmsdef"
)

// Naming references to shareable images (docs/PHASE-41.md, subtask 10).
//
// A program reaches a shareable image's routine through a G^ reference:
// MACRO writes CALLS #1,G^LIB$PUT_OUTPUT, and LINK, finding the routine in
// LIBRTL.EXE, turns the operand into @L^cell, a longword in the image's
// fixup section that holds the routine's offset in LIBRTL's transfer
// vector. The image activator (imageFixup) overwrites the offset with the
// routine's address: LIBRTL's load base plus the offset when a real
// LIBRTL.EXE is loaded, or else the SHIM$LIBRTL_<offset> stub govax
// stands in with (shim.go).
//
// The VMS debugger names the operand by the cell's own address
// (CALLS S^#01,@L^SUB2+0F0, the GST's nearest global), not the routine,
// even with LIBRTL's symbols on hand. So the disassembler does the same
// by default, and naming the routine (G^LIB$PUT_OUTPUT) is an option, as
// naming constants is (Decision 5): the console is a disasm.CellNamer.

// fixupCell is what a G^ fixup cell asked for: a shareable image, by the
// name the image's dependency list gives it, and an offset there.
type fixupCell struct {
	Image  string
	Offset uint32
}

// addCell records that the longword at addr is a G^ fixup cell for
// offset in the shareable image named image.
func (icb *ICB) addCell(addr uint32, image string, offset uint32) {
	if icb.Cells == nil {
		icb.Cells = map[uint32]fixupCell{}
	}

	icb.Cells[addr] = fixupCell{Image: image, Offset: offset}
}

// Cell implements disasm.CellNamer: if addr is a loaded image's G^ fixup
// cell, it returns the name of the routine (or data) the cell's reference
// asked for, LIB$PUT_OUTPUT. The name comes from the reference itself, not
// from what the cell now holds, so it's the same whether a real shareable
// image or a shim answers the call.
func (c *Console) Cell(addr uint32) (string, bool) {
	for _, icb := range c.images().ICBList {
		if cell, ok := icb.Cells[addr]; ok {
			return sharedName(cell.Image, cell.Offset)
		}
	}

	return "", false
}

// sharedSymbolizer is a disasm.Symbolizer for addresses in the shareable
// images a program loaded: an address that is one of an image's universal
// symbols (a transfer-vector entry, say) is named by it. It doesn't name
// shim stubs, which the console's own symbol table already does
// (ensureShims), or anything in the main image.
type sharedSymbolizer struct {
	c *Console
}

// Symbolize returns the universal symbol at addr in a loaded shareable
// image, if there is one.
func (s sharedSymbolizer) Symbolize(addr uint32) (string, bool) {
	main := s.c.findMainICB()

	for _, icb := range s.c.images().ICBList {
		if icb == main || addr < icb.Base || addr > icb.End {
			continue
		}

		if name, ok := sharedName(icb.Name, addr-icb.Base); ok {
			return name, true
		}
	}

	return "", false
}

// sharedName names offset in the shareable image called image: the
// routine a shim stands in for there (shimTable), else the universal
// symbol VMS's image defines at that offset (vmsdef.ImageSymbols, as LINK
// read them from the image).
func sharedName(image string, offset uint32) (string, bool) {
	for _, e := range shimTable {
		if e.library == image && e.offset == offset {
			return e.name, true
		}
	}

	name, ok := imageSymbolNames()[fixupCell{Image: image, Offset: offset}]

	return name, ok
}

var (
	imageSymbolsOnce   sync.Once
	imageSymbolsByAddr map[fixupCell]string
)

// imageSymbolNames is vmsdef.ImageSymbols turned around: each shareable
// image's symbols by offset. Where two names share an offset, the first in
// alphabetical order wins, so the answer doesn't depend on map order.
// Built on first use.
func imageSymbolNames() map[fixupCell]string {
	imageSymbolsOnce.Do(func() {
		names := make([]string, 0, len(vmsdef.ImageSymbols))
		for name := range vmsdef.ImageSymbols {
			names = append(names, name)
		}

		sort.Strings(names)

		imageSymbolsByAddr = make(map[fixupCell]string, len(names))

		for _, name := range names {
			sym := vmsdef.ImageSymbols[name]
			if sym.Image == "" {
				continue // an absolute value, such as a LIB$_ condition
			}

			key := fixupCell{Image: sym.Image, Offset: sym.Value}
			if _, taken := imageSymbolsByAddr[key]; !taken {
				imageSymbolsByAddr[key] = name
			}
		}
	})

	return imageSymbolsByAddr
}
