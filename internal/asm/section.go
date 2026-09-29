package asm

import "github.com/tucats/govax/internal/vmserrors"

// section is one of the assembler's location counters: a named region of
// output with its own current location. Every byte the assembler emits goes
// to the current section, at its location, which then advances past it.
//
// This is the core's location model, shared by both dialects (see
// docs/PHASE-27.md, "Program sections become the core's location model").
// In the console dialect, every section is absolute: it has a fixed base
// address, so a location is also an absolute VAX address right away and
// output goes straight to the sparse image. There are two, P0 (at the
// configured origin) and S0 (at the configured S0 origin), and .REGION
// switches between them.
//
// In the MACRO dialect, the sections are MACRO-32's program sections
// (psects). A relocatable one's base is decided by the linker, so its base
// here is 0, its locations are offsets, and a value using its base is
// left to the linker (see rexpr and relocation).
type section struct {
	name string
	// index is the section's position in Assembler.sections: in the MACRO
	// dialect, its psect number in the object module.
	index int
	// relocatable says the linker decides the section's base.
	relocatable bool
	// img holds the section's contents. The console dialect's sections all
	// share the Assembler's one absolute image; each MACRO psect has its
	// own, indexed by offset.
	img *image
	// base is the absolute address of the section's offset 0 (always 0 for
	// a relocatable section).
	base uint32
	// loc is the current location, as an offset from base. .BASE, ". =",
	// and .SCB can move the location anywhere in the address space, even
	// below base; the arithmetic is modulo 2^32, so base+loc is still the
	// address that was set.
	loc uint32
	// hi is the highest location reached: a psect's allocation.
	hi uint32
	// flags are a MACRO psect's attributes, as the object language's
	// GPS$M_ bits (see psectAttributes), and align its alignment, as a
	// power of two.
	flags uint32
	align uint32
}

// addr returns the section's current location as an absolute address.
func (s *section) addr() uint32 { return s.base + s.loc }

// pc returns the current location counter (vax.console.deposit in the
// reference tool) as an absolute address.
func (a *Assembler) pc() uint32 { return a.cur.addr() }

// setPC moves the current location counter to the absolute address addr.
func (a *Assembler) setPC(addr uint32) {
	a.cur.loc = addr - a.cur.base
	a.cur.mark()
}

// advance moves the current location counter forward n bytes without
// writing anything; bytes never written read back as zero.
func (a *Assembler) advance(n uint32) {
	a.cur.loc += n
	a.cur.mark()
}

// mark records the current location in hi.
func (s *section) mark() {
	if s.loc > s.hi {
		s.hi = s.loc
	}
}

// newSection adds a section to the assembly.
func (a *Assembler) newSection(name string, relocatable bool, img *image, base uint32) *section {
	s := &section{name: name, index: len(a.sections), relocatable: relocatable, img: img, base: base}
	a.sections = append(a.sections, s)

	return s
}

// findSection returns the section named name, or nil.
func (a *Assembler) findSection(name string) *section {
	for _, s := range a.sections {
		if s.name == name {
			return s
		}
	}

	return nil
}

// Default psect names (the MACRO manual, .PSECT): symbols defined before
// any code or data go in the absolute one, and code or data before the
// first named .PSECT goes in the blank one. Each is nine characters: the
// manual prints ". ABS .", but real MACRO's objects and listings have two
// blanks on each side of ABS, and one on each side of BLANK.
const (
	absPsect   = ".  ABS  ."
	blankPsect = ". BLANK ."
)

// macroSections replaces the console dialect's P0 and S0 sections with
// MACRO-32's default absolute psect, . ABS ., where assembly starts.
// . BLANK . is only defined once something uses it (see useBlankPsect),
// as real MACRO does: it's missing from the object of a module that
// never uses it, and the psect numbers after it move down one.
func (a *Assembler) macroSections() {
	a.sections = nil
	a.cur = a.newSection(absPsect, false, newImage(), 0)
	a.implicitAbs = true
}

// output prepares the current section for code or data to be stored at
// its location: before any .PSECT, it moves from . ABS . to . BLANK .,
// and in the MACRO dialect it's an error to store anything in an
// absolute psect, which only defines symbols.
func (a *Assembler) output() error {
	if a.dialect != DialectMACRO {
		return nil
	}

	a.useBlankPsect()

	if !a.cur.relocatable {
		return vmserrors.New(vmserrors.VAX_ABSDATA, a.cur.name)
	}

	return nil
}

// emitByte stores b at the current location and advances past it.
func (a *Assembler) emitByte(b byte) error {
	if err := a.output(); err != nil {
		return err
	}

	if err := a.cur.img.storeByte(a.pc(), b); err != nil {
		return err
	}

	a.advance(1)

	return nil
}

// emitBytes stores each of bs in turn, as emitByte does.
func (a *Assembler) emitBytes(bs ...byte) error {
	for _, b := range bs {
		if err := a.emitByte(b); err != nil {
			return err
		}
	}

	return nil
}

// emitWord stores w at the current location and advances past it.
func (a *Assembler) emitWord(w uint16) error {
	if err := a.output(); err != nil {
		return err
	}

	if err := a.cur.img.storeWord(a.pc(), w); err != nil {
		return err
	}

	a.advance(2)

	return nil
}

// emitLongword stores l at the current location and advances past it.
func (a *Assembler) emitLongword(l uint32) error {
	if err := a.output(); err != nil {
		return err
	}

	if err := a.cur.img.storeLongword(a.pc(), l); err != nil {
		return err
	}

	a.advance(4)

	return nil
}

// emitScaled stores value at the current location in scale bytes (1, 2, or
// 4) and advances past it.
func (a *Assembler) emitScaled(value uint32, scale int) error {
	if err := a.output(); err != nil {
		return err
	}

	if err := a.storeScaled(a.pc(), value, scale); err != nil {
		return err
	}

	a.advance(uint32(scale))

	return nil
}
