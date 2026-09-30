package asm

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the tools a macro uses to look at its own arguments
// (docs/PHASE-28.md, subtask 2): the argument attribute directives .NARG,
// .NCHR, and .NTYPE, and the string operators %LENGTH, %LOCATE, and
// %EXTRACT. A macro uses them to decide, with conditional assembly, what
// to generate. For example, VMS's $PUSHADR macro, which every system
// service macro ($QIOW_S and friends) uses to push an argument's address,
// uses .NTYPE and %LOCATE to tell "#0" (push a zero) from "BUFFER" (push
// BUFFER's address) from "(R1)+" (push the address with PUSHAL, not
// PUSHAB, so R1 steps by a longword).
//
// The directives assign a value to a symbol:
//
//	.NARG   COUNT           ; COUNT = how many positional arguments the
//	                        ; call of this macro gave
//	.NCHR   LEN, <ABC>      ; LEN = 3, the string's length
//	.NTYPE  MODE, (R2)+     ; MODE = ^X82, the operand's addressing mode
//
// The string operators are replaced by their result wherever they appear
// in a line of a macro's (or repeat block's) expansion, before the line is
// assembled:
//
//	%LENGTH(<ABCDE>)            becomes 5
//	%LOCATE(<D>,<ABCDEF>)       becomes 3 (a string's first character is
//	                            position 0; not found gives the length)
//	%EXTRACT(2,3,<ABCDEF>)      becomes CDE
//
// They're evaluated one line at a time, as each expanded line is about to
// be assembled, because a line may use a symbol an earlier line of the
// same expansion set (the manual's RESERVE example sets XX with %LOCATE,
// then uses it in %EXTRACT).

// pseudoNarg assembles .NARG symbol: symbol is set to the number of
// positional arguments the innermost macro call gave, null ones included
// and keyword ones not (the manual's CNT_ARG example).
func (a *Assembler) pseudoNarg(c *cursor) error {
	f := a.innermost(sourceMacro)
	if f == nil {
		return vmserrors.New(vmserrors.VAX_NOTINMACRO, ".NARG")
	}

	name, err := attributeSymbol(c, ".NARG")
	if err != nil {
		return err
	}

	return a.setSymbol(name, uint32(f.expansion.positional), SymNone, false)
}

// pseudoNchr assembles .NCHR symbol,<string>: symbol is set to the
// string's length. The string is read like a macro argument, so it needs
// brackets only if it holds a separator.
func (a *Assembler) pseudoNchr(c *cursor) error {
	name, err := attributeSymbol(c, ".NCHR")
	if err != nil {
		return err
	}

	skipComma(c)

	text, err := scanArgument(c)
	if err != nil {
		return err
	}

	return a.setSymbol(name, uint32(len(text)), SymNone, false)
}

// pseudoNtype assembles .NTYPE symbol,operand: symbol is set to the
// operand's addressing mode (see operandType). No operand gives 0.
func (a *Assembler) pseudoNtype(c *cursor) error {
	name, err := attributeSymbol(c, ".NTYPE")
	if err != nil {
		return err
	}

	skipComma(c)

	mode, err := a.operandType(strings.TrimSpace(c.rest()))
	if err != nil {
		return err
	}

	c.pos = len(c.s)

	return a.setSymbol(name, mode, SymNone, false)
}

// attributeSymbol reads the symbol an attribute directive assigns.
func attributeSymbol(c *cursor, directive string) (string, error) {
	c.skipBlanks()

	name := scanName(c)
	if name == "" {
		return "", vmserrors.New(vmserrors.VAX_BADKEYWORD, directive, c.rest())
	}

	return name, nil
}

// skipComma skips blanks and one optional comma, and the blanks after it.
func skipComma(c *cursor) {
	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
		c.skipBlanks()
	}
}

// .NTYPE's values for the operand forms that use the PC (register 15).
// The manual (.NTYPE) gives these their own mode numbers, instead of the
// instruction encoding's: 0 for a short literal (S^#), 1 for immediate
// (I^#), 2 for absolute (@#), and 3 for general (G^).
const (
	ntypeLiteral   = 0x00
	ntypeImmediate = 0x1F
	ntypeAbsolute  = 0x2F
	ntypeGeneral   = 0x3F
)

// operandType returns .NTYPE's value for operand, an instruction operand
// as it would be written in an instruction: the addressing mode in bits
// 4-7 and the register in bits 0-3, as the operand's mode byte encodes
// them, except for the PC-based forms (see ntypeLiteral). An indexed
// operand, "BASE[Rx]", gives 16 bits: the base operand's mode in the high
// byte and the index's (^X4x) in the low byte.
//
// Nothing is assembled: the mode is worked out from the operand's syntax,
// choosing the displacement size and literal form the way operand
// assembly does (see assembleOperandRec): a displacement or a "#" value
// that isn't known yet gets the default size, or an immediate.
func (a *Assembler) operandType(operand string) (uint32, error) {
	if operand == "" {
		return 0, nil
	}

	// An index: "BASE[Rx]".
	if strings.HasSuffix(operand, "]") {
		if open := strings.LastIndexByte(operand, '['); open > 0 {
			reg, err := parseRegister(newCursor(operand[open+1:len(operand)-1]), 0)
			if err != nil {
				return 0, err
			}

			base, err := a.operandType(operand[:open])
			if err != nil {
				return 0, err
			}

			return base<<8 | 0x40 | uint32(reg), nil
		}
	}

	c := newCursor(operand)

	switch {
	case strings.HasPrefix(operand, "S^#"):
		return ntypeLiteral, nil

	case strings.HasPrefix(operand, "I^#"):
		return ntypeImmediate, nil

	case strings.HasPrefix(operand, "@#"):
		return ntypeAbsolute, nil

	case strings.HasPrefix(operand, "G^"):
		return ntypeGeneral, nil

	case strings.HasPrefix(operand, "#"):
		// A short literal if its value is known now and fits in six
		// bits, as operand assembly decides.
		c.next()

		if x, err := a.exprKnown(c); err == nil && x.known() && x.v < 64 {
			return ntypeLiteral, nil
		}

		return ntypeImmediate, nil
	}

	if reg, err := parseRegister(newCursor(operand), 0); err == nil && registerOnly(operand) {
		return 0x50 | uint32(reg), nil // register
	}

	deferred := uint32(0)

	if c.peek() == '@' {
		c.next()

		deferred = 0x10
	}

	rest := c.rest()

	switch {
	case strings.HasPrefix(rest, "-(") && strings.HasSuffix(rest, ")") && deferred == 0:
		reg, err := parseRegister(newCursor(rest[2:len(rest)-1]), 0)

		return 0x70 | uint32(reg), err // autodecrement

	case strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")+"):
		reg, err := parseRegister(newCursor(rest[1:len(rest)-2]), 0)

		return 0x80 | deferred | uint32(reg), err // autoincrement [deferred]

	case strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")"):
		reg, err := parseRegister(newCursor(rest[1:len(rest)-1]), 0)
		if deferred != 0 {
			// @(Rn) is byte displacement deferred, 0 from Rn.
			return 0xB0 | uint32(reg), err
		}

		return 0x60 | uint32(reg), err // register deferred
	}

	return a.displacementType(rest, deferred)
}

// registerOnly reports whether operand is just a register name.
func registerOnly(operand string) bool {
	c := newCursor(operand)
	if _, err := parseRegister(c, 0); err != nil {
		return false
	}

	return c.atEnd()
}

// displacementType returns .NTYPE's value for a displacement or relative
// mode operand, text (without any "@", whose bit deferred holds): an
// optional B^, W^, or L^, an expression, and an optional "(Rn)" (none
// means the PC: relative mode).
func (a *Assembler) displacementType(text string, deferred uint32) (uint32, error) {
	size := 0

	if len(text) > 2 && text[1] == '^' {
		switch text[0] {
		case 'B':
			size = 1
		case 'W':
			size = 2
		case 'L':
			size = 4
		}

		if size != 0 {
			text = text[2:]
		}
	}

	reg := uint32(0xF)
	relative := true

	if strings.HasSuffix(text, ")") {
		if open := strings.LastIndexByte(text, '('); open > 0 {
			r, err := parseRegister(newCursor(text[open+1:len(text)-1]), 0)
			if err == nil {
				reg, relative = uint32(r), false
				text = text[:open]
			}
		}
	}

	if size == 0 {
		size = a.displacementSize(text, relative)
	}

	return uint32(sizeModeBase(size)) | deferred | reg, nil
}

// displacementSize is the displacement size operand assembly would choose
// for expression text with no B^/W^/L^ (see relativeOperand and
// registerDisplacement): the smallest that holds a value known now, else
// the default.
func (a *Assembler) displacementSize(text string, relative bool) int {
	x, err := a.exprKnown(newCursor(text))

	if relative {
		if err == nil {
			target, ok := a.knownTarget(x)

			// A label in another psect is measured by its offset, as
			// if it were in this one: real MACRO's .NTYPE gives byte
			// relative mode for a label at offset 0 of another psect,
			// seen from offset 0x78 (testdata/mar/macros/usermac.mar),
			// though an instruction's displacement to it is the
			// linker's, at the default size.
			if !ok && !x.known() && x.x.op == rBase && x.x.sect != nil && x.x.sect.relocatable {
				target, ok = x.x.v, true
			}

			if ok {
				// The displacement is measured from the end of the
				// field, after a one-byte mode byte; try each size.
				for _, size := range []int{1, 2} {
					if fitsSigned(int64(int32(target-(a.pc()+1+uint32(size)))), size) {
						return size
					}
				}
			}
		}

		if a.dialect == DialectMACRO {
			return a.defaultDisp
		}

		return 4
	}

	if err == nil && x.known() {
		for _, size := range []int{1, 2} {
			if fitsSigned(int64(int32(x.v)), size) {
				return size
			}
		}

		return 4
	}

	if a.dialect == DialectMACRO {
		return 2
	}

	return 4
}

// stringOperators returns line, one line of a macro's or repeat block's
// expansion, with each %LENGTH, %LOCATE, and %EXTRACT replaced by its
// result. The comment, after a ";" outside angle brackets, is left alone.
func (a *Assembler) stringOperators(line string) (string, error) {
	if !strings.Contains(line, "%") {
		return line, nil
	}

	var out strings.Builder

	depth := 0

	for i := 0; i < len(line); {
		ch := line[i]

		switch {
		case ch == '<':
			depth++
		case ch == '>' && depth > 0:
			depth--
		case ch == ';' && depth == 0:
			out.WriteString(line[i:])

			return out.String(), nil
		}

		if ch != '%' {
			out.WriteByte(ch)

			i++

			continue
		}

		name, args, end, ok, err := operatorCall(line, i)
		if err != nil {
			return "", err
		}

		if !ok {
			out.WriteByte(ch)

			i++

			continue
		}

		result, err := a.stringOperator(name, args)
		if err != nil {
			return "", err
		}

		out.WriteString(result)
		
		i = end
	}

	return out.String(), nil
}

// operatorArgs is how many arguments each string operator takes: at least
// min, at most max.
var operatorArgs = map[string]struct{ min, max int }{
	"LENGTH":  {1, 1},
	"LOCATE":  {2, 3},
	"EXTRACT": {3, 3},
}

// operatorCall reads a string operator's call starting at line[i] (a
// "%"): its name, its arguments (delimiters removed), and the index just
// past its ")". It reports ok=false if line[i] doesn't start one.
func operatorCall(line string, i int) (name string, args []string, end int, ok bool, err error) {
	j := i + 1
	for j < len(line) && isMacroNameChar(line[j]) {
		j++
	}

	name = strings.ToUpper(line[i+1 : j])

	counts, known := operatorArgs[name]
	if !known || j >= len(line) || line[j] != '(' {
		return "", nil, 0, false, nil
	}

	c := newCursor(line)
	c.pos = j + 1

	for {
		c.skipBlanks()

		arg, err := scanOperatorArgument(c)
		if err != nil {
			return "", nil, 0, false, err
		}

		args = append(args, arg)

		c.skipBlanks()

		switch c.next() {
		case ',':
			continue
		case ')':
			if len(args) < counts.min || len(args) > counts.max {
				return "", nil, 0, false, vmserrors.New(vmserrors.VAX_BADOPERATOR, "%"+name)
			}

			return name, args, c.pos, true, nil
		}

		return "", nil, 0, false, vmserrors.New(vmserrors.VAX_NOCLOSE, ")")
	}
}

// scanOperatorArgument reads one string operator argument: a delimited
// string (<...> or ^x...x), or text up to a "," or ")" not inside
// parentheses (an argument substituted into the call may be an operand
// such as "(R1)+").
func scanOperatorArgument(c *cursor) (string, error) {
	switch {
	case c.peek() == '<':
		end, ok := matchBracket(c.s, c.pos)
		if !ok {
			return "", vmserrors.New(vmserrors.VAX_NOCLOSE, ">")
		}

		text := c.s[c.pos+1 : end]
		c.pos = end + 1

		return text, nil

	case c.peek() == '^' && c.peekAt(1) != 0:
		return scanArgument(c)
	}

	start, depth := c.pos, 0

	for !c.atEnd() {
		switch c.peek() {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return strings.TrimSpace(c.s[start:c.pos]), nil
			}

			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(c.s[start:c.pos]), nil
			}
		}

		c.next()
	}

	return strings.TrimSpace(c.s[start:c.pos]), nil
}

// stringOperator evaluates one string operator.
func (a *Assembler) stringOperator(name string, args []string) (string, error) {
	switch name {
	case "LENGTH":
		return strconv.Itoa(len(args[0])), nil

	case "LOCATE":
		from := 0

		if len(args) == 3 {
			n, err := a.operatorNumber(args[2])
			if err != nil {
				return "", err
			}

			from = n
		}

		s := args[1]
		if from < len(s) {
			if k := strings.Index(s[from:], args[0]); k >= 0 {
				return strconv.Itoa(from + k), nil
			}
		}

		return strconv.Itoa(len(s)), nil
	}

	// EXTRACT
	start, err := a.operatorNumber(args[0])
	if err != nil {
		return "", err
	}

	length, err := a.operatorNumber(args[1])
	if err != nil {
		return "", err
	}

	s := args[2]
	if start >= len(s) || length == 0 {
		return "", nil
	}

	return s[start:min(start+length, len(s))], nil
}

// operatorNumber reads a string operator's position or length argument:
// an unsigned decimal number, or an absolute symbol already defined.
func (a *Assembler) operatorNumber(text string) (int, error) {
	if n, err := strconv.Atoi(text); err == nil && n >= 0 {
		return n, nil
	}

	name := strings.ToUpper(text)

	sym, ok := a.symbols.find(name)
	if !ok || !sym.defined() || sym.sect != nil {
		return 0, vmserrors.New(vmserrors.VAX_UNDEFSYM, name)
	}

	return int(int32(sym.value)), nil
}
