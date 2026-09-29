package asm

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// addrFixup/dispFixup/branchFixup pick the fixup kind matching a scale (1,
// 2, or 4 bytes), for the three K_ADDR_*/K_DISP_*/K_BRANCH_* families.
func addrFixup(scale int) fixupKind {
	switch scale {
	case 1:
		return fixAddrB
	case 2:
		return fixAddrW
	default:
		return fixAddrL
	}
}

func dispFixup(scale int) fixupKind {
	switch scale {
	case 1:
		return fixDispB
	case 2:
		return fixDispW
	default:
		return fixDispL
	}
}

func branchFixup(scale int) fixupKind {
	switch scale {
	case 1:
		return fixBranchB
	case 2:
		return fixBranchW
	default:
		return fixBranchL
	}
}

// storeScaled writes value at addr using the given byte width (1, 2, or 4),
// matching the reference tool's repeated "switch on vax.assembler.scale"
// blocks throughout asm_operand.c.
func (a *Assembler) storeScaled(addr uint32, value uint32, scale int) error {
	switch scale {
	case 1:
		return a.image.storeByte(addr, byte(value))

	case 2:
		return a.image.storeWord(addr, uint16(value))

	case 4:
		return a.image.storeLongword(addr, value)
	}

	return vmserrors.New(vmserrors.VAX_BADSCALE, scale)
}

// assembleOperand assembles one instruction operand at the current deposit
// location, matching asm_operand(). inst/opIndex identify the operand's
// access kind, data type (int/float short-literal interpretation) and
// scale (byte width) from the instruction table.
func (a *Assembler) assembleOperand(c *cursor, inst *cpu.Instruction, opIndex int) error {
	return a.assembleOperandRec(c, inst, opIndex, false)
}

// assembleOperandRec is assembleOperand's recursive core: parsingIndex is
// true only for the recursive call that parses an operand's base address
// after a "[Rx]" index prefix has already been detected and encoded (see
// the lookahead block below) — matching asm_operand.c's ASM_PARSING_IDX
// flag, which stops that recursive call from scanning for (and re-encoding)
// its own index prefix.
func (a *Assembler) assembleOperandRec(c *cursor, inst *cpu.Instruction, opIndex int, parsingIndex bool) error {
	access := inst.Access[opIndex]
	dtype := inst.Type
	scale := inst.Scale[opIndex]

	// The operand syntax for indexed mode is "BASE[Rx]": the index appears
	// textually after the base, but its addressing-mode byte must be
	// written to memory before the base operand's own encoding. So: look
	// ahead (without consuming) for a "[" before the next comma/end; if
	// found, write the index byte now, then recurse to parse the whole
	// operand text (including the trailing "[Rx]", which the recursive
	// call's main loop below just skips over once it gets there) as the
	// base operand.
	if !parsingIndex {
		var indexMode byte

		save := c.pos
		foundIndex := false

		for !c.atEnd() && c.peek() != ',' {
			if c.peek() == '[' {
				c.next()

				reg, err := parseRegister(c, 0)
				if err != nil {
					return err
				}

				indexMode = 0x40 | byte(reg)
				foundIndex = true

				break
			}

			c.next()
		}

		c.pos = save

		if foundIndex {
			if err := a.emitByte(indexMode); err != nil {
				return err
			}
			if err := a.assembleOperandRec(c, inst, opIndex, true); err != nil {
				return err
			}
			// The recursive call above parses the base ("(Rn)", "@#addr",
			// a displacement mode, ...); most of its own code paths
			// return as soon as the base itself is fully consumed,
			// leaving the trailing "[Rx]" text this function's own
			// lookahead already turned into a byte still unconsumed in
			// the cursor. Skip over it here, once, regardless of which
			// base-mode branch the recursive call actually took, rather
			// than teaching each one individually to check for it (the
			// issue this replaces: only the few branches that happened to
			// fall through to the "already at '['" special case below
			// consumed it; every other base mode -- e.g. "(Rn)[Rx]",
			// exercised for real by testdata/asm/kernel.asm's own
			// EXE$DISPATCH -- silently left it in place, corrupting
			// whatever operand parsing came next).
			c.skipBlanks()

			if c.peek() == '[' {
				c.next()

				for !c.atEnd() && c.peek() != ']' && c.peek() != ',' {
					c.next()
				}

				if c.peek() == ']' {
					c.next()
				}
			}

			return nil
		}
	}

	c.skipBlanks()

	if c.atEnd() || c.peek() == ',' {
		return nil
	}

	if c.peek() == '[' {
		// The index prefix for this operand was already encoded above;
		// just skip over the "[Rx]" text.
		c.next()

		for !c.atEnd() && c.peek() != ']' && c.peek() != ',' {
			c.next()
		}

		if c.peek() == ']' {
			c.next()
		}

		return nil
	}

	// OP_IM: implicit immediate, e.g. CHMK/XFC's operand — requires the
	// "#" notation, then behaves like OP_BR minus the PC-relative offset
	// conversion.
	if access == cpu.AccessImmediate {
		if c.peek() != '#' {
			return vmserrors.New(vmserrors.VAX_NEEDHASH)
		}

		c.next()
	}

	if access == cpu.AccessBranch || access == cpu.AccessImmediate {
		return a.assembleBranchOrImplicit(c, access, scale)
	}

	ch := c.next()

	// "#value": short literal (S^#n) if it fits and isn't forward
	// referenced, else immediate literal (I^#n) — decided once the value
	// is known, but the '#' and value are only parsed here, once.
	constant := litNone

	var (
		litValue      uint32  // the short-literal value/table-index (int or float)
		litFloat      float64 // the parsed float, valid when dtype is float
		litWasForward bool
	)

	if ch == '#' {
		loc := a.pc()
		fx := addrFixup(scale)

		if dtype == cpu.ShortLiteralInt {
			v, wasForward, err := a.exprValue(c, loc, fx)
			if err != nil {
				return err
			}

			litValue, litWasForward = v, wasForward
		} else {
			f, err := a.parseFloat(c)
			if err != nil {
				return err
			}

			litFloat = f

			if idx, ok := cpu.FindShortFloat(f); ok {
				litValue = uint32(idx)
			} else {
				litValue = 0xFFFFFFFF
			}
		}

		if litWasForward || litValue >= 64 {
			constant = litImmediate

			if litWasForward {
				// The fixup we just queued assumed the value starts right
				// at `loc` (the short-literal layout); since this turns
				// out to need a leading 0x8F mode byte first, shift it
				// forward by one byte — matching asm_operand.c's own late
				// correction of vax.console.last_symbol->forward->location.
				a.lastFixup.location++
			}
		} else {
			constant = litShort
		}
	}

	// S^#literal: either just decided above, or spelled out explicitly.
	if constant == litShort || (ch == 'S' && c.peek() == '^') {
		if constant != litShort {
			c.next() // '^'

			if c.next() != '#' {
				return vmserrors.New(vmserrors.VAX_BADSHORTLIT)
			}

			if dtype == cpu.ShortLiteralInt {
				// An expression like any other literal; the reference
				// tool read only hex digits here, whatever the radix.
				v, err := a.exprNoForward(c)
				if err != nil {
					return err
				}

				if v >= 64 {
					return vmserrors.New(vmserrors.VAX_SHORTRANGE)
				}

				litValue = v
			} else {
				f, err := a.parseFloat(c)
				if err != nil {
					return err
				}

				idx, ok := cpu.FindShortFloat(f)
				if !ok {
					return vmserrors.New(vmserrors.VAX_BADSHORTFLOAT)
				}

				litValue = uint32(idx)
			}
		}

		if err := a.emitByte(byte(litValue)); err != nil {
			return err
		}

		return nil
	}

	// Rn / AP / FP / SP / PC: register mode.
	if ch == 'R' || ch == 'S' || ch == 'A' || ch == 'F' || ch == 'P' {
		save := c.pos

		if reg, err := parseRegister(c, ch); err == nil {
			mode := byte(0x50) | byte(reg)
			if err := a.emitByte(mode); err != nil {
				return err
			}

			return nil
		}

		c.pos = save
	}

	// -(Rn): autodecrement. Any other leading '-' starts a negative
	// value, e.g. the displacement in "-4(FP)".
	if ch == '-' && c.peekAt(blankRun(c)) == '(' {
		c.skipBlanks()
		c.next()

		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		if err := a.emitByte(byte(0x70) | byte(reg)); err != nil {
			return err
		}

		c.skipBlanks()

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}

		return nil
	}

	// @(Rn)+ / @(Rn): autoincrement deferred, or (with no "+") the
	// zero-displacement byte-deferred form asm_operand.c encodes for it.
	if ch == '@' && c.peek() == '(' {
		c.next()

		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		c.skipBlanks()

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}

		mode := byte(0x90)

		hasPlus := c.peek() == '+'
		if !hasPlus {
			mode = 0xB0
		} else {
			c.next()
		}

		mode |= byte(reg)

		if err := a.emitByte(mode); err != nil {
			return err
		}

		if mode >= 0xB0 {
			if err := a.emitByte(0); err != nil {
				return err
			}
		}

		return nil
	}

	// (Rn) / (Rn)+: register deferred, or autoincrement.
	if ch == '(' {
		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		c.skipBlanks()

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}

		mode := byte(0x60)

		if c.peek() == '+' {
			mode = 0x80

			c.next()
		}

		if err := a.emitByte(mode | byte(reg)); err != nil {
			return err
		}

		return nil
	}

	// I^#constant: immediate literal, either just decided above via "#",
	// or spelled out explicitly.
	if constant == litImmediate || (ch == 'I' && c.peek() == '^') {
		if constant != litImmediate {
			c.next() // '^'

			if c.next() != '#' {
				return vmserrors.New(vmserrors.VAX_BADIMMLIT)
			}
		}

		if err := a.emitByte(0x8F); err != nil {
			return err
		}

		if dtype == cpu.ShortLiteralFloat {
			if constant != litImmediate {
				f, err := a.parseFloat(c)
				if err != nil {
					return err
				}

				litFloat = f
			}

			return a.storeImmediateFloat(scale, litFloat)
		}

		if constant != litImmediate {
			loc := a.pc()

			v, _, err := a.exprValue(c, loc, addrFixup(scale))
			if err != nil {
				return err
			}

			litValue = v
		}

		return a.storeImmediateInt(scale, litValue)
	}

	// @#address: absolute.
	if ch == '@' && c.peek() == '#' {
		c.next()

		if err := a.emitByte(0x9F); err != nil {
			return err
		}

		return a.storeAddrValue(c)
	}

	deferred := byte(0)
	if ch == '@' {
		deferred = 0x10
		ch = c.next()
	}

	// @(Rn): byte-displacement-deferred with an implied zero displacement
	// (no explicit displacement given at all).
	if deferred != 0 && ch == '(' {
		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		mode := byte(0xB0) | byte(reg)

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}

		if err := a.emitByte(mode); err != nil {
			return err
		}
		if err := a.emitByte(0); err != nil {
			return err
		}

		return nil
	}

	// B^address / B^displacement(Rn): byte relative [deferred].
	if ch == 'B' && c.peek() == '^' {
		c.next()

		return a.assembleDisplacement(c, deferred, 1, 0xAF, 0xA0)
	}
	// W^address / W^displacement(Rn): word relative [deferred].
	if ch == 'W' && c.peek() == '^' {
		c.next()

		return a.assembleDisplacement(c, deferred, 2, 0xCF, 0xC0)
	}
	// L^address / L^displacement(Rn): long relative [deferred].
	if ch == 'L' && c.peek() == '^' {
		c.next()

		return a.assembleDisplacement(c, deferred, 4, 0xEF, 0xE0)
	}

	// Fallback: a bare value or symbol, with no size prefix. As in
	// MACRO-32, "address" is relative mode (PC + displacement), "@address"
	// is relative deferred, and "value(Rn)"/"@value(Rn)" is displacement
	// [deferred] mode. The reference tool assembled a bare address as
	// absolute (@#) and ignored a leading "@" entirely; see
	// docs/DEVIATIONS.md.
	c.pos-- // put back `ch`; the value parser needs the whole token.

	return a.assembleBareOperand(c, deferred)
}

// litKind mirrors asm_operand.c's ASM_LIT_NONE/SHORT/IMMEDIATE local flag.
type litKind int

const (
	litNone litKind = iota
	litShort
	litImmediate
)

// storeImmediateInt writes an I^# immediate literal's integer data (1, 2,
// or 4 bytes), matching asm_operand.c's size-based store.
func (a *Assembler) storeImmediateInt(scale int, value uint32) error {
	if err := a.emitScaled(value, scale); err != nil {
		return err
	}

	return nil
}

// storeImmediateFloat writes an I^# immediate literal's F_FLOAT (scale 4)
// or D_FLOAT (scale 8) data.
//
// asm_operand.c's own D_FLOAT case advances the deposit pointer by 4 mid-
// branch (after writing the first longword) and *again* by the full scale
// (8) in the shared code path every branch falls through to, over-
// advancing by 4 bytes — a plain arithmetic double-count, not an ISA
// fidelity question (VAX D_FLOAT is unambiguously 8 bytes), and not one
// any testdata/asm fixture exercises (none use an 8-byte float immediate
// literal). Per docs/CLAUDE.md's bug-fixing policy this is fixed here
// rather than replicated: write exactly 8 bytes and advance by 8.
func (a *Assembler) storeImmediateFloat(scale int, f float64) error {
	bits, overflow := cpu.EncodeFloat(scale, f)
	if overflow {
		return vmserrors.New(vmserrors.VAX_FLOATRANGE)
	}

	if err := a.image.storeLongword(a.pc(), uint32(bits)); err != nil {
		return err
	}

	if scale == 8 {
		if err := a.image.storeLongword(a.pc()+4, uint32(bits>>32)); err != nil {
			return err
		}
	}

	a.advance(uint32(scale))

	return nil
}

// storeAddrValue parses an absolute address's 4-byte value and writes it,
// used by the "@#address" case.
func (a *Assembler) storeAddrValue(c *cursor) error {
	loc := a.pc()

	value, _, err := a.exprValue(c, loc, fixAddrL)
	if err != nil {
		return err
	}

	if err := a.emitLongword(value); err != nil {
		return err
	}

	return nil
}

// assembleBranchOrImplicit handles an OP_BR (branch displacement) or OP_IM
// (implicit immediate) operand: a bare value/expression stored directly at
// the current location, sized by scale — no addressing-mode byte at all.
// For OP_BR, the parsed value names the branch's absolute destination and
// is converted to a PC-relative displacement before storing.
func (a *Assembler) assembleBranchOrImplicit(c *cursor, access cpu.AccessKind, scale int) error {
	var fx fixupKind
	if access == cpu.AccessBranch {
		fx = branchFixup(scale)
	} else {
		fx = fixAddrL
	}

	loc := a.pc()

	value, _, err := a.exprValue(c, loc, fx)
	if err != nil {
		return err
	}

	if access == cpu.AccessBranch {
		value = value - a.pc() - uint32(scale)
	}

	if err := a.emitScaled(value, scale); err != nil {
		return err
	}

	return nil
}

// fitsSigned reports whether v fits in a size-byte signed field.
func fitsSigned(v int64, size int) bool {
	switch size {
	case 1:
		return v >= -128 && v <= 127
	case 2:
		return v >= -32768 && v <= 32767
	}

	return v >= -2147483648 && v <= 2147483647
}

// sizeModeBase maps a displacement size (1, 2, 4) to the high nibble of its
// addressing-mode byte: byte (A), word (C) or longword (E) displacement.
func sizeModeBase(size int) byte {
	switch size {
	case 1:
		return 0xA0
	case 2:
		return 0xC0
	}

	return 0xE0
}

// assembleBareOperand assembles an operand given as a bare expression,
// optionally "@"-deferred and optionally followed by "(Rn)", choosing the
// displacement size the way MACRO-32 does for a value it already knows:
// the smallest of byte, word, or longword that holds it. A forward
// reference always gets a longword displacement, since its value (and so
// the size it needs) isn't known until later and there's no linker pass to
// shrink it; MACRO-32 itself defaults these to a word.
func (a *Assembler) assembleBareOperand(c *cursor, deferred byte) error {
	modeAddr := a.pc()
	loc := modeAddr + 1 // the displacement follows the mode byte.

	value, wasForward, err := a.exprValue(c, loc, fixBranchL)
	if err != nil {
		return err
	}

	reg := byte(0x0F) // PC: relative mode
	haveReg := false

	c.skipBlanks()

	if c.peek() == '(' {
		c.next()

		r, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		c.skipBlanks()

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}

		reg = byte(r)
		haveReg = true
	}

	size := 4
	disp := value

	switch {
	case wasForward && haveReg:
		// A displacement from Rn is the symbol's own value, not a
		// distance from here.
		a.lastFixup.kind = fixAddrL

	case wasForward:
		// Relative: the fixBranchL fixup queued above already measures
		// from the end of a longword displacement.

	case haveReg:
		for _, size = range []int{1, 2, 4} {
			if fitsSigned(int64(int32(value)), size) {
				break
			}
		}

	default:
		for _, size = range []int{1, 2, 4} {
			disp = value - (loc + uint32(size))
			if fitsSigned(int64(int32(disp)), size) {
				break
			}
		}
	}

	if err := a.image.storeByte(modeAddr, sizeModeBase(size)|deferred|reg); err != nil {
		return err
	}

	if err := a.storeScaled(loc, disp, size); err != nil {
		return err
	}

	a.setPC(loc + uint32(size))

	return nil
}

// assembleDisplacement handles the B^/W^/L^ operand forms: "X^address"
// (relative: the mode byte's register field is PC, and the stored value is
// the distance from the end of the displacement to address) or
// "X^displacement(Rn)" (the value itself, added to Rn), each optionally
// "@"-deferred. size is the displacement field's byte width; relMode/
// dispMode are the mode bytes' high nibble|0xF and high-nibble-only forms
// (e.g. 0xAF/0xA0 for byte).
//
// The reference tool stored the target address itself for "X^address",
// rather than a displacement to it, and a forward reference's fixup was
// off by the displacement's own size; both are fixed here.
func (a *Assembler) assembleDisplacement(c *cursor, deferred byte, size int, relMode, dispMode byte) error {
	loc := a.pc() + 1 // the mode byte comes first; the displacement follows it.

	value, wasForward, err := a.exprValue(c, loc, branchFixup(size))
	if err != nil {
		return err
	}

	mode := relMode | deferred

	c.skipBlanks()

	haveReg := c.peek() == '('
	if haveReg {
		c.next()

		r, err := parseRegister(c, 0)
		if err != nil {
			return err
		}

		mode = dispMode | deferred | byte(r)

		c.skipBlanks()

		if c.next() != ')' {
			return vmserrors.New(vmserrors.VAX_BADMODE)
		}
	}

	disp := value

	switch {
	case wasForward && haveReg:
		a.lastFixup.kind = addrFixup(size)

	case wasForward:
		// The branch-style fixup already measures from the end of the
		// displacement field.

	case haveReg:
		// A displacement may be written signed or as its unsigned bit
		// pattern (e.g. B^0FF(R1)).
		if !fitsSigned(int64(int32(value)), size) && (size == 4 || value>>(8*uint(size)) != 0) {
			return vmserrors.New(vmserrors.VAX_DATARANGE, "displacement", int32(value))
		}

	default:
		disp = value - (loc + uint32(size))
		if !fitsSigned(int64(int32(disp)), size) {
			return vmserrors.New(vmserrors.VAX_DATARANGE, "displacement", int32(disp))
		}
	}

	if err := a.emitByte(mode); err != nil {
		return err
	}

	if err := a.emitScaled(disp, size); err != nil {
		return err
	}

	return nil
}
