package asm

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
// switches between them. MACRO-32's relocatable program sections, whose
// base the linker decides, will be sections too.
type section struct {
	name string
	// base is the absolute address of the section's offset 0.
	base uint32
	// loc is the current location, as an offset from base. .BASE, ". =",
	// and .SCB can move the location anywhere in the address space, even
	// below base; the arithmetic is modulo 2^32, so base+loc is still the
	// address that was set.
	loc uint32
}

// addr returns the section's current location as an absolute address.
func (s *section) addr() uint32 { return s.base + s.loc }

// pc returns the current location counter (vax.console.deposit in the
// reference tool) as an absolute address.
func (a *Assembler) pc() uint32 { return a.cur.addr() }

// setPC moves the current location counter to the absolute address addr.
func (a *Assembler) setPC(addr uint32) { a.cur.loc = addr - a.cur.base }

// advance moves the current location counter forward n bytes without
// writing anything; bytes never written read back as zero.
func (a *Assembler) advance(n uint32) { a.cur.loc += n }

// emitByte stores b at the current location and advances past it.
func (a *Assembler) emitByte(b byte) error {
	if err := a.image.storeByte(a.pc(), b); err != nil {
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
	if err := a.image.storeWord(a.pc(), w); err != nil {
		return err
	}

	a.advance(2)

	return nil
}

// emitLongword stores l at the current location and advances past it.
func (a *Assembler) emitLongword(l uint32) error {
	if err := a.image.storeLongword(a.pc(), l); err != nil {
		return err
	}

	a.advance(4)

	return nil
}

// emitScaled stores value at the current location in scale bytes (1, 2, or
// 4) and advances past it.
func (a *Assembler) emitScaled(value uint32, scale int) error {
	if err := a.storeScaled(a.pc(), value, scale); err != nil {
		return err
	}

	a.advance(uint32(scale))

	return nil
}
