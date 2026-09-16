package console

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// Evaluator parses console address/value expressions — a small, from-scratch
// stand-in for the real assembler's asm_expr/asm_expr2/asm_expr3/asm_hex/
// asm_dec (reference/eVAX/eVAX/Source/Assembler/asm_expr.c, asm_value.c),
// which is Phase 11's scope, not this one's (see doc.go). It supports the
// subset console commands actually need: numeric literals in the current
// radix (or a "^D"/"^X"/"^O"/"^B" prefix override, matching asm_hex/
// asm_dec's radix-prefix handling), symbol names, "." for the current
// deposit address, parenthesized sub-expressions, +/-, *//, and the C
// source's =/<>/<=/</>=/> comparison operators (each yielding 0 or 1) at the
// same precedence levels asm_expr's three-tier grammar uses. Register names
// and indirect (@) register/PSL references are not supported — EXAMINE/
// DEPOSIT special-case a bare register name themselves before ever calling
// the evaluator (matching console_exam.c's own register short-circuit), and
// no other console command in scope needs them.
type Evaluator struct {
	Symbols *SymbolTable
	Radix   int    // 8, 10, or 16 — the default for a prefix-less numeric literal
	Here    uint32 // value of "." (the current deposit address)
}

// Eval parses a leading expression from s and returns its value and
// whatever text remains unconsumed.
//
// TODO - This item does not yet support string pooling in the console.
// When the microkernel is valid, a string constants like "Hello" should
// store the string value in the console string pool area and create a
// descriptor to that string, and then use the VAX address of the
// descriptor as the result of the operaiton. This was supported in the
// C "reference" version of eVAX but has not yet been ported here.
func (e *Evaluator) Eval(s string) (uint32, string, error) {
	return e.parseCompare(s)
}

func (e *Evaluator) parseCompare(s string) (uint32, string, error) {
	v1, rest, err := e.parseAddSub(s)
	if err != nil {
		return 0, "", err
	}

	for {
		trimmed := strings.TrimLeft(rest, " \t")

		op, opLen := compareOp(trimmed)
		if op == "" {
			return v1, rest, nil
		}

		v2, r2, err := e.parseAddSub(trimmed[opLen:])
		if err != nil {
			return 0, "", err
		}

		var result uint32

		switch op {
		case "=":
			result = boolToUint32(v1 == v2)

		case "<>":
			result = boolToUint32(v1 != v2)

		case "<=":
			result = boolToUint32(int32(v1) <= int32(v2))

		case "<":
			result = boolToUint32(int32(v1) < int32(v2))

		case ">=":
			result = boolToUint32(int32(v1) >= int32(v2))

		case ">":
			result = boolToUint32(int32(v1) > int32(v2))
		}

		v1, rest = result, r2
	}
}

func compareOp(s string) (op string, length int) {
	if len(s) == 0 {
		return "", 0
	}

	if strings.HasPrefix(s, "<>") {
		return "<>", 2
	}

	if strings.HasPrefix(s, "<=") {
		return "<=", 2
	}

	if strings.HasPrefix(s, ">=") {
		return ">=", 2
	}

	switch s[0] {
	case '=':
		return "=", 1

	case '<':
		return "<", 1

	case '>':
		return ">", 1
	}

	return "", 0
}

func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}

func (e *Evaluator) parseAddSub(s string) (uint32, string, error) {
	v1, rest, err := e.parseMulDiv(s)
	if err != nil {
		return 0, "", err
	}

	for {
		trimmed := strings.TrimLeft(rest, " \t")
		if trimmed == "" || (trimmed[0] != '+' && trimmed[0] != '-') {
			return v1, rest, nil
		}

		op := trimmed[0]

		v2, r2, err := e.parseMulDiv(trimmed[1:])
		if err != nil {
			return 0, "", err
		}

		if op == '+' {
			v1 = v1 + v2
		} else {
			v1 = v1 - v2
		}

		rest = r2
	}
}

func (e *Evaluator) parseMulDiv(s string) (uint32, string, error) {
	v1, rest, err := e.parseAtom(s)
	if err != nil {
		return 0, "", err
	}

	for {
		trimmed := strings.TrimLeft(rest, " \t")
		if trimmed == "" || (trimmed[0] != '*' && trimmed[0] != '/') {
			return v1, rest, nil
		}

		op := trimmed[0]

		v2, r2, err := e.parseAtom(trimmed[1:])
		if err != nil {
			return 0, "", err
		}

		if op == '*' {
			v1 = v1 * v2
		} else {
			if v2 == 0 {
				return 0, "", vmserrors.New(vmserrors.CLI_DIVZERO)
			}

			v1 = v1 / v2
		}

		rest = r2
	}
}

func (e *Evaluator) parseAtom(s string) (uint32, string, error) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	if s[0] == '(' {
		v, rest, err := e.parseCompare(s[1:])
		if err != nil {
			return 0, "", err
		}

		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, ")") {
			return 0, "", vmserrors.New(vmserrors.CLI_NEEDPAREN)
		}

		return v, rest[1:], nil
	}

	if s[0] == '.' && !isDigit(peekByte(s, 1)) {
		return e.Here, s[1:], nil
	}

	if isSymbolStart(s[0]) {
		i := 1
		for i < len(s) && isSymbolChar(s[i]) {
			i++
		}

		name, rest := s[:i], s[i:]

		if strings.EqualFold(name, "DEFINED") {
			return e.parseDefined(rest)
		}

		v, ok := e.Symbols.Get(name)
		if !ok {
			return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
		}

		return v, rest, nil
	}

	return e.parseNumber(s)
}

// parseDefined parses DEFINED("SYMBOL")'s parenthesized, double-quoted
// argument and reports 1 if the named console symbol exists, 0 otherwise —
// matching asm_function()'s DEFINED case (see internal/asm/functions.go's
// callDefined, the assembler's own equivalent), needed here for the IF
// console verb (cmdIf in dispatch.go), which vax.init uses: "IF
// DEFINED(\"CONSOLE$ARG_FILE\") THEN SET NOVERBOSE".
func (e *Evaluator) parseDefined(s string) (uint32, string, error) {
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "(") {
		return 0, "", vmserrors.New(vmserrors.CLI_DEFARG)
	}

	s = strings.TrimLeft(s[1:], " \t")
	if !strings.HasPrefix(s, `"`) {
		return 0, "", vmserrors.New(vmserrors.CLI_DEFQUOTE)
	}

	s = s[1:]

	end := strings.IndexByte(s, '"')
	if end < 0 {
		return 0, "", vmserrors.New(vmserrors.CLI_UNTERMSTR)
	}

	name := s[:end]

	s = strings.TrimLeft(s[end+1:], " \t")
	if !strings.HasPrefix(s, ")") {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDPAREN)
	}

	_, ok := e.Symbols.Get(name)

	return boolToUint32(ok), s[1:], nil
}

func peekByte(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}

	return 0
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isSymbolStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '$'
}

func isSymbolChar(ch byte) bool {
	return isSymbolStart(ch) || isDigit(ch)
}

// parseNumber parses a numeric literal, matching asm_hex/asm_dec's
// radix-prefix handling: "^D" forces decimal, "^X"/"0X" forces hex, "^O"
// forces octal, "^B" forces binary; with no prefix, digits are read in the
// evaluator's default Radix.
func (e *Evaluator) parseNumber(s string) (uint32, string, error) {
	radix := e.Radix

	if len(s) >= 2 && s[0] == '^' {
		switch s[1] {
		case 'D', 'd':
			radix, s = 10, s[2:]

		case 'X', 'x':
			radix, s = 16, s[2:]

		case 'O', 'o':
			radix, s = 8, s[2:]

		case 'B', 'b':
			radix, s = 2, s[2:]

		default:
			return 0, "", vmserrors.New(vmserrors.CLI_BADRADIXPREFIX, s[1])
		}
	} else if len(s) >= 2 && s[0] == '0' && (s[1] == 'X' || s[1] == 'x') {
		radix, s = 16, s[2:]
	}

	var v uint32

	i := 0
	for i < len(s) {
		d, ok := digitValue(s[i])
		if !ok || d >= radix {
			break
		}

		v = v*uint32(radix) + uint32(d)
		i++
	}

	if i == 0 {
		return 0, "", vmserrors.New(vmserrors.CLI_BADNUMBER, s)
	}

	return v, s[i:], nil
}

func digitValue(ch byte) (int, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0'), true

	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10, true

	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10, true
	}

	return 0, false
}
