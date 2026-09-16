package console

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
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

	// Mem/CPU back a quoted-string literal's writes into the console
	// string pool (parseQuotedString) — nil whenever this Evaluator is
	// used only for address/value arithmetic that never touches memory.
	Mem *vm.Memory
	CPU *vax.CPU
}

// Eval parses a leading expression from s and returns its value and
// whatever text remains unconsumed.
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

	if s[0] == '"' {
		return e.parseQuotedString(s)
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

// parseQuotedString parses a double-quoted string literal, matching
// asm_expr3's own '"' case (reference/eVAX/eVAX/Source/Assembler/
// asm_expr.c): it appends the string's bytes into the console string pool
// (CONSOLE$STRINGPOOL_BASE/_SIZE, reserved by VMInit -- see vminit.go),
// builds a VAX string descriptor for it, links the descriptor onto the
// pool's singly linked chain (the same chain ShowString walks via
// CONSOLE$STRINGPOOL_BASE's head-of-chain longword), and returns the
// descriptor's address as the expression's value.
//
// The pool layout, per string, mirrors the C source exactly: a 4-byte
// "link" cell (initially zero, later overwritten with this string's own
// descriptor address by whichever *next* call retroactively points back to
// it -- or, for the very first string, by this call pointing back into
// CONSOLE$STRINGPOOL_BASE's head cell), immediately followed by the raw
// string bytes, then (4-byte aligned) a 12-byte descriptor: length,
// pointer-to-data, and a next-descriptor link that starts zeroed (chain
// terminator) and is reused as the *following* string's own link cell.
//
// Matching the C source, a missing closing quote is not an error -- the
// literal simply runs to the end of the input (or an embedded '\n'), same
// as asm_expr3's own `!isend(*p)` loop guard.
func (e *Evaluator) parseQuotedString(s string) (uint32, string, error) {
	if e.Mem == nil {
		return 0, "", vmserrors.New(vmserrors.CLI_NOVAX)
	}

	poolStart, ok := e.Symbols.Get("CONSOLE$STRINGPOOL")
	if !ok {
		return 0, "", vmserrors.New(vmserrors.CLI_NOPOOL)
	}

	poolSize, ok := e.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		return 0, "", vmserrors.New(vmserrors.CLI_NOPOOLSIZE)
	}

	poolBase, ok := e.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		return 0, "", vmserrors.New(vmserrors.CLI_NOSTRINGPOOL)
	}

	s = s[1:] // consume the opening quote

	linkage := poolStart
	n := poolStart

	if err := e.Mem.StoreLongword(e.CPU, n, 0); err != nil {
		return 0, "", err
	}

	n += 4

	var size uint32

	for len(s) > 0 && s[0] != '"' && s[0] != '\n' {
		if n > poolBase+poolSize+16 {
			return 0, "", vmserrors.New(vmserrors.CLI_SPOOLOVF)
		}

		ch := s[0]
		s = s[1:]

		if ch == '\\' && len(s) > 0 {
			esc := s[0]
			s = s[1:]

			switch esc {
			case 'n':
				ch = '\n'
			case 'r':
				ch = '\r'
			case 't':
				ch = '\t'
			default:
				ch = esc
			}
		}

		if err := e.Mem.StoreByte(e.CPU, n, ch); err != nil {
			return 0, "", err
		}

		size++
		n++
	}

	if n&3 != 0 {
		n = (n &^ 3) + 4
	}

	if err := e.Mem.StoreLongword(e.CPU, n, size); err != nil {
		return 0, "", err
	}

	if err := e.Mem.StoreLongword(e.CPU, n+4, poolStart+4); err != nil {
		return 0, "", err
	}

	if err := e.Mem.StoreLongword(e.CPU, linkage, n); err != nil {
		return 0, "", err
	}

	if err := e.Mem.StoreLongword(e.CPU, n+8, 0); err != nil {
		return 0, "", err
	}

	e.Symbols.Set("CONSOLE$STRINGPOOL", n+8, SymbolSystem)

	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}

	return n, s, nil
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
