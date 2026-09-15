package asm

import (
	"fmt"

	"github.com/tucats/govax/internal/cpu"
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
	return fmt.Errorf("unsupported operand scale %d", scale)
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
		save := c.pos
		foundIndex := false
		var indexMode byte

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
			if err := a.image.storeByte(a.deposit, indexMode); err != nil {
				return err
			}
			a.deposit++
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
			// bug this replaces: only the few branches that happened to
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
			return fmt.Errorf("implicit immediate operand requires '#'")
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
	var litValue uint32  // the short-literal value/table-index (int or float)
	var litFloat float64 // the parsed float, valid when dtype is float
	var litWasForward bool

	if ch == '#' {
		loc := a.deposit
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
				if a.lastSymbol != nil && len(a.lastSymbol.forward) > 0 {
					a.lastSymbol.forward[0].location++
				}
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
				return fmt.Errorf("invalid short literal syntax")
			}
			if dtype == cpu.ShortLiteralInt {
				v, err := a.hexDigits(c)
				if err != nil {
					return err
				}
				if v >= 64 {
					return fmt.Errorf("short literal out of range")
				}
				litValue = v
			} else {
				f, err := a.parseFloat(c)
				if err != nil {
					return err
				}
				idx, ok := cpu.FindShortFloat(f)
				if !ok {
					return fmt.Errorf("value is not a valid short float literal")
				}
				litValue = uint32(idx)
			}
		}
		if err := a.image.storeByte(a.deposit, byte(litValue)); err != nil {
			return err
		}
		a.deposit++
		return nil
	}

	// Rn / AP / FP / SP / PC: register mode.
	if ch == 'R' || ch == 'S' || ch == 'A' || ch == 'F' || ch == 'P' {
		save := c.pos
		if reg, err := parseRegister(c, ch); err == nil {
			mode := byte(0x50) | byte(reg)
			if err := a.image.storeByte(a.deposit, mode); err != nil {
				return err
			}
			a.deposit++
			return nil
		}
		c.pos = save
	}

	// -(Rn): autodecrement.
	if ch == '-' {
		c.skipBlanks()
		if c.next() != '(' {
			return fmt.Errorf("invalid addressing mode")
		}
		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}
		if err := a.image.storeByte(a.deposit, byte(0x70)|byte(reg)); err != nil {
			return err
		}
		a.deposit++
		c.skipBlanks()
		if c.next() != ')' {
			return fmt.Errorf("invalid addressing mode")
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
			return fmt.Errorf("invalid addressing mode")
		}

		mode := byte(0x90)
		hasPlus := c.peek() == '+'
		if !hasPlus {
			mode = 0xB0
		} else {
			c.next()
		}
		mode |= byte(reg)

		if err := a.image.storeByte(a.deposit, mode); err != nil {
			return err
		}
		a.deposit++

		if mode >= 0xB0 {
			if err := a.image.storeByte(a.deposit, 0); err != nil {
				return err
			}
			a.deposit++
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
			return fmt.Errorf("invalid addressing mode")
		}

		mode := byte(0x60)
		if c.peek() == '+' {
			mode = 0x80
			c.next()
		}
		if err := a.image.storeByte(a.deposit, mode|byte(reg)); err != nil {
			return err
		}
		a.deposit++
		return nil
	}

	// I^#constant: immediate literal, either just decided above via "#",
	// or spelled out explicitly.
	if constant == litImmediate || (ch == 'I' && c.peek() == '^') {
		if constant != litImmediate {
			c.next() // '^'
			if c.next() != '#' {
				return fmt.Errorf("invalid immediate literal syntax")
			}
		}

		if err := a.image.storeByte(a.deposit, 0x8F); err != nil {
			return err
		}
		a.deposit++

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
			loc := a.deposit
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
		if err := a.image.storeByte(a.deposit, 0x9F); err != nil {
			return err
		}
		a.deposit++
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
			return fmt.Errorf("invalid addressing mode")
		}
		if err := a.image.storeByte(a.deposit, mode); err != nil {
			return err
		}
		a.deposit++
		if err := a.image.storeByte(a.deposit, 0); err != nil {
			return err
		}
		a.deposit++
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

	// Fallback: a bare value or symbol, treated as an absolute address —
	// unless followed by "(Rn)", in which case it's rewritten (mode byte
	// and, if the value was forward-referenced, the fixup kind) into a
	// long-displacement operand: Rn + <value>, where <value> is used as a
	// raw additive displacement rather than an address of its own.
	c.pos-- // put back `ch`; the value parser needs the whole token.

	modeAddr := a.deposit
	if err := a.image.storeByte(a.deposit, 0x9F); err != nil {
		return err
	}
	a.deposit++

	loc := a.deposit
	value, wasForward, err := a.exprValue(c, loc, fixAddrL)
	if err != nil {
		return err
	}

	if c.peek() == '(' {
		c.next()
		reg, err := parseRegister(c, 0)
		if err != nil {
			return err
		}
		mode := byte(0xE0) | deferred | byte(reg)
		if err := a.image.storeByte(modeAddr, mode); err != nil {
			return err
		}
		c.skipBlanks()
		if c.next() != ')' {
			return fmt.Errorf("invalid addressing mode")
		}
		if wasForward && a.lastSymbol != nil && len(a.lastSymbol.forward) > 0 {
			a.lastSymbol.forward[0].kind = fixDispL
		}
	}

	if err := a.image.storeLongword(a.deposit, value); err != nil {
		return err
	}
	a.deposit += 4

	return nil
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
	if err := a.storeScaled(a.deposit, value, scale); err != nil {
		return err
	}
	a.deposit += uint32(scale)
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
		return fmt.Errorf("floating literal out of range")
	}
	if err := a.image.storeLongword(a.deposit, uint32(bits)); err != nil {
		return err
	}
	if scale == 8 {
		if err := a.image.storeLongword(a.deposit+4, uint32(bits>>32)); err != nil {
			return err
		}
	}
	a.deposit += uint32(scale)
	return nil
}

// storeAddrValue parses an absolute address's 4-byte value and writes it,
// used by the "@#address" case.
func (a *Assembler) storeAddrValue(c *cursor) error {
	loc := a.deposit
	value, _, err := a.exprValue(c, loc, fixAddrL)
	if err != nil {
		return err
	}
	if err := a.image.storeLongword(a.deposit, value); err != nil {
		return err
	}
	a.deposit += 4
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

	loc := a.deposit
	value, _, err := a.exprValue(c, loc, fx)
	if err != nil {
		return err
	}

	if access == cpu.AccessBranch {
		value = value - a.deposit - uint32(scale)
	}

	if err := a.storeScaled(a.deposit, value, scale); err != nil {
		return err
	}
	a.deposit += uint32(scale)

	return nil
}

// assembleDisplacement handles the B^/W^/L^ relative-or-displacement
// operand forms: "X^address" (relative — the mode byte's low nibble is
// 0xF, meaning "use PC") or "X^displacement(Rn)" (displacement from Rn),
// each optionally "@"-deferred. size is the displacement field's byte
// width; relMode/dispMode are the mode bytes' high nibble|0xF and
// high-nibble-only forms (e.g. 0xAF/0xA0 for byte).
func (a *Assembler) assembleDisplacement(c *cursor, deferred byte, size int, relMode, dispMode byte) error {
	loc := a.deposit + 1 // the mode byte comes first; the displacement follows it.
	value, _, err := a.exprValue(c, loc, dispFixup(size))
	if err != nil {
		return err
	}

	mode := relMode | deferred
	haveReg := c.peek() == '('
	var reg byte
	if haveReg {
		c.next()
		r, err := parseRegister(c, 0)
		if err != nil {
			return err
		}
		reg = byte(r)
		mode = dispMode | deferred | reg
		c.skipBlanks()
		if c.next() != ')' {
			return fmt.Errorf("invalid addressing mode")
		}
	}

	if err := a.image.storeByte(a.deposit, mode); err != nil {
		return err
	}
	a.deposit++

	if err := a.storeScaled(a.deposit, value, size); err != nil {
		return err
	}
	a.deposit += uint32(size)

	return nil
}
