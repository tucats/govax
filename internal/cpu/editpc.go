package cpu

// EDITPC: Edit Packed to Character String (docs/PHASE-35.md, subtask 13).
//
//	EDITPC srclen.rw, srcaddr.ab, pattern.ab, dstaddr.ab
//
// The pattern is a list of one-byte pattern operators (the manual's EO$
// operators), some followed by a one-byte operand, ending with EO$END.
// They move the source's digits into the destination as characters,
// suppressing or protecting leading zeros, placing a fixed or floating
// sign, inserting characters, and blanking a zero result: the editing of
// a COBOL PICTURE clause. The destination's length is whatever the pattern
// produces.
//
// The instruction keeps two character registers, the fill character
// (initially a space) and the sign character (initially a space, or "-"
// for a negative source), a significance flag (whether a nonzero digit
// has been seen, or significance has been set), and the condition codes:
// N, the source's sign (cleared at the end for -0); Z, whether every
// digit so far is zero; V, whether a nonzero digit was lost; and C, the
// significance flag.
//
// A source length over 31, a pattern operator the manual reserves, a
// pattern that asks for more digits than the source has, or one that
// ends with digits left over, is a reserved operand. VMS's run of the
// Phase 35 probe (testdata/insn35) shows the characters produced before
// such an abort stay in the destination, so govax writes them too. govax
// doesn't model PSL<FPD>, so the registers aren't given the manual's
// part-way state (its note 11); they're left as they were.
//
// After a normal end: R0 = srclen, R1 = srcaddr, R2 = 0, R3 = the address
// of the EO$END operator, R4 = 0, R5 = the address just past the
// destination. V set and PSL<DV> set is a decimal overflow trap.

// Pattern operators (Table 9-2 of the manual). The 8x, 9x, and Ax ones
// carry a repeat count, 1 to 15, in their low nibble.
const (
	eoEnd         = 0x00
	eoEndFloat    = 0x01
	eoClearSignif = 0x02
	eoSetSignif   = 0x03
	eoStoreSign   = 0x04
	eoLoadFill    = 0x40
	eoLoadSign    = 0x41
	eoLoadPlus    = 0x42
	eoLoadMinus   = 0x43
	eoInsert      = 0x44
	eoBlankZero   = 0x45
	eoReplaceSign = 0x46
	eoAdjustInput = 0x47
	eoFill        = 0x80
	eoMove        = 0x90
	eoFloat       = 0xA0
)

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x38}), emulEditpc)
}

// editState is EDITPC's state while it runs the pattern.
type editState struct {
	digits   []byte // the source digit nibbles not yet used
	zeros    int    // leading zeros EO$ADJUST_INPUT asked to supply first
	fill     byte
	sign     byte
	neg      bool // the source's sign is minus (N)
	signif   bool // significance (C)
	zero     bool // every digit so far is zero (Z)
	overflow bool // a nonzero digit was lost (V)
	out      []byte
}

// nextDigit returns the next source digit: a supplied leading zero first,
// then the source's own. ok is false if there are none left.
func (s *editState) nextDigit() (byte, bool) {
	if s.zeros > 0 {
		s.zeros--

		return 0, true
	}

	if len(s.digits) == 0 {
		return 0, false
	}

	d := s.digits[0]
	s.digits = s.digits[1:]

	return d, true
}

// digitsLeft returns how many digits the source still has to give.
func (s *editState) digitsLeft() int { return s.zeros + len(s.digits) }

func emulEditpc(e *Engine, d *Decoded) error {
	length, err := e.decimalLength(d, 0)
	if err != nil {
		return err
	}

	src := d.Operands[1].Addr
	pattern := d.Operands[2].Addr
	dst := d.Operands[3].Addr

	p, err := e.readPacked(src, length)
	if err != nil {
		return err
	}

	s := &editState{digits: p.digits, fill: ' ', sign: ' ', neg: p.neg, zero: true}
	if p.neg {
		s.sign = '-'
	}

	// abort writes what the pattern has produced so far and returns the
	// reserved operand exception.
	abort := func() error {
		if err := e.storeBytes(dst, s.out); err != nil {
			return err
		}

		return &Fault{Code: ExcReservedOp}
	}

	for {
		op, err := e.mem.LoadByte(e.cpu, pattern)
		if err != nil {
			return err
		}

		// The operand byte of the operators that take one.
		var arg byte
		if op >= eoLoadFill && op <= eoAdjustInput {
			if arg, err = e.mem.LoadByte(e.cpu, pattern+1); err != nil {
				return err
			}
		}

		count := int(op & 0xF)

		switch {
		case op == eoEnd:
			if s.digitsLeft() > 0 {
				return abort()
			}

			if err := e.storeBytes(dst, s.out); err != nil {
				return err
			}

			e.setRegisters(uint32(length), src, 0, pattern, 0, dst+uint32(len(s.out)))

			psl := e.cpu.PSL()
			psl.SetN(s.neg && !s.zero) // -0 isn't negative
			psl.SetZ(s.zero)
			psl.SetV(s.overflow)
			psl.SetC(s.signif)
			e.cpu.SetPSL(psl)

			return e.decimalOverflowTrap(s.overflow)

		case op == eoEndFloat:
			if !s.signif {
				s.out = append(s.out, s.sign)
				s.signif = true
			}

		case op == eoClearSignif:
			s.signif = false

		case op == eoSetSignif:
			s.signif = true

		case op == eoStoreSign:
			s.out = append(s.out, s.sign)

		case op == eoLoadFill:
			s.fill = arg

		case op == eoLoadSign:
			s.sign = arg

		case op == eoLoadPlus:
			if !s.neg {
				s.sign = arg
			}

		case op == eoLoadMinus:
			if s.neg {
				s.sign = arg
			}

		case op == eoInsert:
			if s.signif {
				s.out = append(s.out, arg)
			} else {
				s.out = append(s.out, s.fill)
			}

		case op == eoBlankZero:
			// The manual makes a length of zero, or one reaching back
			// before the destination, UNPREDICTABLE; govax blanks what
			// it can.
			if s.zero {
				for i := max(len(s.out)-int(arg), 0); i < len(s.out); i++ {
					s.out[i] = s.fill
				}
			}

		case op == eoReplaceSign:
			if s.zero {
				if i := len(s.out) - int(arg); i >= 0 && i < len(s.out) {
					s.out[i] = s.fill
				}
			}

		case op == eoAdjustInput:
			// Fewer source digits than arg: supply leading zeros. More:
			// discard the extra leading ones, a nonzero one being an
			// overflow (and significant).
			left := s.digitsLeft()

			if left < int(arg) {
				s.zeros += int(arg) - left
			}

			for ; left > int(arg); left-- {
				if d, _ := s.nextDigit(); d != 0 {
					s.overflow = true
					s.signif = true
					s.zero = false
				}
			}

		case op&0xF0 == eoFill && count > 0:
			for i := 0; i < count; i++ {
				s.out = append(s.out, s.fill)
			}

		case op&0xF0 == eoMove && count > 0:
			// The manual: a count beyond the digits left is a reserved
			// operand. VMS moves the digits there are first.
			for i := 0; i < count; i++ {
				digit, ok := s.nextDigit()
				if !ok {
					return abort()
				}

				if digit != 0 {
					s.signif = true
					s.zero = false
				}

				if s.signif {
					s.out = append(s.out, '0'+digit)
				} else {
					s.out = append(s.out, s.fill)
				}
			}

		case op&0xF0 == eoFloat && count > 0:
			for i := 0; i < count; i++ {
				digit, ok := s.nextDigit()
				if !ok {
					return abort()
				}

				if digit != 0 {
					if !s.signif {
						s.out = append(s.out, s.sign)
						s.signif = true
					}

					s.zero = false
				}

				if s.signif {
					s.out = append(s.out, '0'+digit)
				} else {
					s.out = append(s.out, s.fill)
				}
			}

		default:
			// 05-3F, 48-7F, 80, 90, A0, and B0-FF are reserved.
			return abort()
		}

		pattern++
		if op >= eoLoadFill && op <= eoAdjustInput {
			pattern++
		}
	}
}
