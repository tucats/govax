package asm

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
)

// parseRegister parses a register specifier at the cursor — Rn (0-15), or
// the AP/FP/SP/PC mnemonics — matching asm_reg(). first, if non-zero, is a
// character already consumed by the caller that turned out to be the first
// character of the register name (asm_reg's `ch` parameter); pass 0 to have
// this function read it itself.
func parseRegister(c *cursor, first byte) (vax.Reg, error) {
	ch := first
	if ch == 0 {
		c.skipBlanks()
		ch = c.next()
	}

	switch ch {
	case 'R':
		n, ok := parseSimpleDecimal(c)
		if !ok || n < 0 || n > 15 {
			return 0, fmt.Errorf("invalid register specification")
		}
		if isSymbolChar(c.peek()) {
			return 0, fmt.Errorf("invalid register specification")
		}
		return vax.Reg(n), nil

	case 'A':
		if c.next() != 'P' || isSymbolChar(c.peek()) {
			return 0, fmt.Errorf("invalid register specification")
		}
		return vax.AP, nil

	case 'F':
		if c.next() != 'P' || isSymbolChar(c.peek()) {
			return 0, fmt.Errorf("invalid register specification")
		}
		return vax.FP, nil

	case 'S':
		if c.next() != 'P' || isSymbolChar(c.peek()) {
			return 0, fmt.Errorf("invalid register specification")
		}
		return vax.SP, nil

	case 'P':
		if c.next() != 'C' || isSymbolChar(c.peek()) {
			return 0, fmt.Errorf("invalid register specification")
		}
		return vax.PC, nil
	}

	return 0, fmt.Errorf("invalid register specification")
}

// parseSimpleDecimal reads an unsigned decimal integer with no sign, radix
// prefix, or symbol fallback — the register-number half of asm_dec(), used
// only by parseRegister for the digits after "R".
func parseSimpleDecimal(c *cursor) (int, bool) {
	start := c.pos
	for isDigit(c.peek()) {
		c.pos++
	}
	if c.pos == start {
		return 0, false
	}
	n := 0
	for _, ch := range []byte(c.s[start:c.pos]) {
		n = n*10 + int(ch-'0')
	}
	return n, true
}
