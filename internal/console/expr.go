package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/asm"
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
// the evaluator (matching console_exam.c's own register short-circuit).
//
// Phase 42 adds what the VMS debugger's address expressions use: a register
// name (R0 to R11, AP, FP, SP, PC, PSL, and the debugger's %R0 spelling) is
// its contents, so R1+4 and SP-8 work; and a "." or "@" in front of an
// operand means "the contents of": .PC is the address the PC holds, and
// .COUNT is the longword stored at COUNT. A "." with nothing after it that
// could start an operand is still the current location.
type Evaluator struct {
	Symbols *SymbolTable
	Radix   int    // 8, 10, or 16 — the default for a prefix-less numeric literal
	Here    uint32 // value of "." (the current deposit address)

	// Mem/CPU back a quoted-string literal's writes into the console
	// string pool (parseQuotedString) — nil whenever this Evaluator is
	// used only for address/value arithmetic that never touches memory.
	Mem *vm.Memory
	CPU *vax.CPU

	// Load reads the longword at an address for "." and "@" (the contents
	// of a memory operand). The console sets it to read through kernel-mode
	// translation, as EXAMINE does; nil leaves those operators without
	// memory to read, and they fail.
	Load func(addr uint32) (uint32, error)

	// Value makes the evaluator give an expression's value as the
	// debugger's EVALUATE does, not a location's address as EXAMINE and
	// DEPOSIT take it. A data label is then the data's contents (EVALUATE
	// WATCHL is what is stored at WATCHL, where EXAMINE WATCHL means the
	// location itself), and a register after "." or "@" is dereferenced
	// (.R2 is what is at the address R2 holds), as any other operand is.
	Value bool

	// LoadSized reads size bytes (1, 2, or 4) at an address, for a data
	// label's contents in Value mode. Nil reads a longword with Load.
	LoadSized func(addr, size uint32) (uint32, error)

	// Debug resolves the names the console's table doesn't have from the
	// loaded images' debug symbol tables (Phase 41): a symbol, a
	// debugger path name (FORTH\NEXT, DBGDIS\START\LOOP), or a line
	// (%LINE 120, DBGSUB\%LINE 14). Nil for none.
	Debug DebugNames
}

// DebugNames looks names up in a debugger's symbol table, for the
// evaluator.
type DebugNames interface {
	// Lookup returns the value of the symbol path names: NAME,
	// MODULE\NAME, or MODULE\ROUTINE\NAME.
	Lookup(path string) (uint32, bool)

	// Line returns the address of line n's first instruction. scope is
	// the path written before %LINE (DBGSUB, DBGDIS\START), or "" for
	// the module the program is in.
	Line(scope string, n int) (uint32, bool)
}

// DebugData is what a DebugNames that knows the type of its data symbols
// adds: the size of the data a name labels, so that the evaluator in Value
// mode can fetch its contents.
type DebugData interface {
	// DataSize returns the size in bytes of the data the symbol path
	// names, up to a longword (a larger datum is read as its first four
	// bytes). ok is false for anything that isn't a data symbol.
	DataSize(path string) (uint32, bool)

	// Element returns the address and size of element index of the array
	// the symbol path labels (BUFFER[2]); the index counts from the
	// array's own lower bound. ok is false for anything but an array.
	Element(path string, index int32) (addr, size uint32, ok bool)
}

// Eval parses a leading expression from s and returns its value and
// whatever text remains unconsumed.
func (e *Evaluator) Eval(s string) (uint32, string, error) {
	return e.parseLogical(s)
}

// keywordAt reports which of words (upper case) s starts with, ignoring
// case and only where the word is a whole name: "MOD 3" starts with MOD,
// but "MODE" doesn't. It returns the word, or "" for none. The VMS
// debugger spells its comparison and logical operators this way (EQL,
// NEQ, AND, ...), which a symbol name could also begin with, hence the
// whole-name test.
func keywordAt(s string, words ...string) string {
	for _, w := range words {
		if len(s) >= len(w) && strings.EqualFold(s[:len(w)], w) &&
			(len(s) == len(w) || !isSymbolChar(s[len(w)])) {
			return w
		}
	}

	return ""
}

// parseLogical is the lowest-precedence level of an expression: the
// bitwise operators AND, OR, and XOR, applied left to right to
// comparisons. (The debugger's language is the language of the program
// being debugged, MACRO here, whose own operators are &, !, and \; the
// keyword forms are what the debugger's expression evaluator takes.)
func (e *Evaluator) parseLogical(s string) (uint32, string, error) {
	v1, rest, err := e.parseCompare(s)
	if err != nil {
		return 0, "", err
	}

	for {
		trimmed := strings.TrimLeft(rest, " \t")

		op := keywordAt(trimmed, "AND", "OR", "XOR")
		if op == "" {
			return v1, rest, nil
		}

		v2, r2, err := e.parseCompare(trimmed[len(op):])
		if err != nil {
			return 0, "", err
		}

		switch op {
		case "AND":
			v1 &= v2
		case "OR":
			v1 |= v2
		default:
			v1 ^= v2
		}

		rest = r2
	}
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

	// The debugger's word forms are the same comparisons as the symbols.
	if w := keywordAt(s, "EQL", "NEQ", "LSS", "LEQ", "GTR", "GEQ"); w != "" {
		return map[string]string{
			"EQL": "=", "NEQ": "<>", "LSS": "<", "LEQ": "<=", "GTR": ">", "GEQ": ">=",
		}[w], len(w)
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

		// "*", "/", MOD, and "@" (an arithmetic shift, as MACRO's: 1@4 is
		// 1 shifted left four places, and a negative count shifts right).
		op, opLen := "", 0

		switch {
		case trimmed == "":
			return v1, rest, nil
		case trimmed[0] == '*' || trimmed[0] == '/' || trimmed[0] == '@':
			op, opLen = trimmed[:1], 1
		case keywordAt(trimmed, "MOD") != "":
			op, opLen = "MOD", 3
		default:
			return v1, rest, nil
		}

		v2, r2, err := e.parseAtom(trimmed[opLen:])
		if err != nil {
			return 0, "", err
		}

		switch op {
		case "*":
			v1 *= v2
		case "@":
			v1 = shiftLeft(v1, int32(v2))
		default: // "/" and MOD
			if v2 == 0 {
				return 0, "", vmserrors.New(vmserrors.CLI_DIVZERO)
			}

			if op == "/" {
				v1 /= v2
			} else {
				v1 %= v2
			}
		}

		rest = r2
	}
}

// shiftLeft shifts v by count bits: left for a positive count, and
// arithmetically right (keeping the sign) for a negative one, as the VAX's
// ASHL instruction and MACRO's @ operator do.
func shiftLeft(v uint32, count int32) uint32 {
	switch {
	case count >= 32:
		return 0
	case count >= 0:
		return v << uint(count)
	case count <= -32:
		return uint32(int32(v) >> 31)
	default:
		return uint32(int32(v) >> uint(-count))
	}
}

func (e *Evaluator) parseAtom(s string) (uint32, string, error) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	if s[0] == '(' {
		v, rest, err := e.parseLogical(s[1:])
		if err != nil {
			return 0, "", err
		}

		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, ")") {
			return 0, "", vmserrors.New(vmserrors.CLI_NEEDPAREN)
		}

		return v, rest[1:], nil
	}

	// NOT is the one's complement of what follows (NOT 0 is FFFFFFFF).
	if w := keywordAt(s, "NOT"); w != "" {
		v, rest, err := e.parseAtom(s[len(w):])
		if err != nil {
			return 0, "", err
		}

		return ^v, rest, nil
	}

	// ".X" and "@X" are the contents of X: the register's value, or the
	// longword in memory at X's address. A "." with nothing operand-like
	// after it is the current location.
	if (s[0] == '.' || s[0] == '@') && startsOperand(peekByte(s, 1)) {
		return e.parseContents(s[1:])
	}

	if s[0] == '.' && !isDigit(peekByte(s, 1)) {
		return e.Here, s[1:], nil
	}

	if s[0] == '%' && lineKeyword(s) == 0 {
		i := 1
		for i < len(s) && isSymbolChar(s[i]) {
			i++
		}

		if v, ok := registerValue(e, s[1:i]); ok {
			return v, s[i:], nil
		}

		return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, s[:i])
	}

	if s[0] == '"' {
		return e.parseQuotedString(s)
	}

	if n := lineKeyword(s); n > 0 {
		return e.parseLine("", s[n:])
	}

	if isSymbolStart(s[0]) {
		i := 1
		for i < len(s) && isSymbolChar(s[i]) {
			i++
		}

		// A debugger path name goes on through each backslash:
		// MODULE\NAME, MODULE\ROUTINE\NAME, or MODULE\%LINE n.
		for i+1 < len(s) && s[i] == '\\' {
			if n := lineKeyword(s[i+1:]); n > 0 {
				return e.parseLine(s[:i], s[i+1+n:])
			}

			if !isSymbolStart(s[i+1]) {
				break
			}

			i += 2
			for i < len(s) && isSymbolChar(s[i]) {
				i++
			}
		}

		name, rest := s[:i], s[i:]

		if strings.EqualFold(name, "DEFINED") {
			return e.parseDefined(rest)
		}

		v, ok := e.lookupSymbol(name)
		if !ok {
			return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
		}

		// NAME[n] is an element of an array.
		if strings.HasPrefix(rest, "[") {
			return e.parseElement(name, rest[1:])
		}

		if e.Value {
			contents, isData, err := e.dataContents(name, v)
			if err != nil {
				return 0, "", err
			}

			if isData {
				return contents, rest, nil
			}
		}

		return v, rest, nil
	}

	return e.parseNumber(s)
}

// parseElement evaluates the subscript of NAME[subscript], s being what
// follows the "[": the address of that element of the array NAME labels,
// or, in Value mode, the element's contents.
func (e *Evaluator) parseElement(name, s string) (uint32, string, error) {
	dd, ok := e.Debug.(DebugData)
	if !ok {
		return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
	}

	index, rest, err := e.parseLogical(s)
	if err != nil {
		return 0, "", err
	}

	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, "]") {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDPAREN)
	}

	addr, size, ok := dd.Element(name, int32(index))
	if !ok {
		return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, name+"["+strconv.Itoa(int(int32(index)))+"]")
	}

	if !e.Value {
		return addr, rest[1:], nil
	}

	v, err := e.loadValue(addr, size)

	return v, rest[1:], err
}

// loadValue reads size bytes (at most a longword's worth) at addr in the
// way Value mode does.
func (e *Evaluator) loadValue(addr, size uint32) (uint32, error) {
	switch {
	case e.LoadSized != nil:
		return e.LoadSized(addr, size)
	case e.Load != nil:
		return e.Load(addr)
	}

	return 0, vmserrors.New(vmserrors.CLI_NOVAX)
}

// dataContents is a data label's value in Value mode: the contents at
// address addr, sized by the label's data type. isData is false when name
// isn't a data label (a register, a routine, a constant), and its value
// is unchanged.
func (e *Evaluator) dataContents(name string, addr uint32) (v uint32, isData bool, err error) {
	dd, ok := e.Debug.(DebugData)
	if !ok || e.Symbols == nil {
		return 0, false, nil
	}

	// A name the console's own table or a register defines wins over the
	// debug symbols, as in lookupSymbol.
	if _, found := e.Symbols.Get(name); found {
		return 0, false, nil
	}

	if _, found := registerValue(e, name); found {
		return 0, false, nil
	}

	size, ok := dd.DataSize(name)
	if !ok {
		return 0, false, nil
	}

	v, err = e.loadValue(addr, size)

	return v, true, err
}

// startsOperand reports whether ch can begin the operand a "." or "@"
// takes: a name, a number, a parenthesis, or a "%" register. (A "." before
// a digit is a number's own point, and is left to the number.)
func startsOperand(ch byte) bool {
	return isSymbolStart(ch) || isDigit(ch) || ch == '(' || ch == '%'
}

// parseContents evaluates the operand after a "." or "@" and returns its
// contents. A register's contents are its value, which is what the bare
// register name already gives; anything else is an address, and the
// contents are the longword stored there.
func (e *Evaluator) parseContents(s string) (uint32, string, error) {
	addr, rest, err := e.parseAtom(s)
	if err != nil {
		return 0, "", err
	}

	if !e.Value && e.isRegister(s) {
		return addr, rest, nil
	}

	if e.Load == nil {
		return 0, "", vmserrors.New(vmserrors.CLI_NOVAX)
	}

	v, err := e.Load(addr)
	if err != nil {
		return 0, "", err
	}

	return v, rest, nil
}

// isRegister reports whether the operand at the start of s is a register
// name (with or without the debugger's "%"), and so evaluates to the
// register's contents. A console symbol of the same name wins over a
// register, as it does in lookupSymbol.
func (e *Evaluator) isRegister(s string) bool {
	t := strings.TrimPrefix(s, "%")

	i := 0
	for i < len(t) && isSymbolChar(t[i]) {
		i++
	}

	name := t[:i]
	if _, ok := registerValue(e, name); !ok {
		return false
	}

	if t == s && e.Symbols != nil {
		if _, isSym := e.Symbols.Get(name); isSym {
			return false
		}
	}

	return true
}

// registerValue returns the contents of the register named name: a
// general register, or the PSL. ok is false when name is neither.
func registerValue(e *Evaluator, name string) (uint32, bool) {
	name = strings.ToUpper(name)

	if name == "PSL" {
		if e.CPU == nil {
			return 0, true
		}

		return uint32(e.CPU.PSL()), true
	}

	r, ok := registerNames[name]
	if !ok {
		return 0, false
	}

	if e.CPU == nil {
		return 0, true
	}

	return e.CPU.GPR(r), true
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
// console verb (ifCommand in commands.go), which vax.init uses: "IF
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

	_, ok := e.lookupSymbol(name)

	return boolToUint32(ok), s[1:], nil
}

// lookupSymbol resolves name from the console's symbol table, then from
// the assembler's predefined system symbols (asm.BuiltinSymbol: PTE$K_*,
// VAX$PR_*, XFC$*, OPC$_*, ...), which the C source kept in that same
// table. A console symbol of the same name wins.
//
// After those come the loaded images' debug symbol tables (e.Debug), and a
// path name (one with a backslash) is looked for only there.
func (e *Evaluator) lookupSymbol(name string) (uint32, bool) {
	if !strings.Contains(name, `\`) {
		if v, ok := e.Symbols.Get(name); ok {
			return v, true
		}

		if v, ok := registerValue(e, name); ok {
			return v, true
		}

		if v, ok := asm.BuiltinSymbol(name); ok {
			return v, true
		}
	}

	if e.Debug == nil {
		return 0, false
	}

	return e.Debug.Lookup(name)
}

// lineKeyword returns the length of the debugger's %LINE keyword when s
// starts with it, else 0.
func lineKeyword(s string) int {
	const keyword = "%LINE"

	if len(s) < len(keyword) || !strings.EqualFold(s[:len(keyword)], keyword) {
		return 0
	}

	if len(s) > len(keyword) && isSymbolChar(s[len(keyword)]) {
		return 0
	}

	return len(keyword)
}

// parseLine parses the line number after %LINE, in s, and returns the
// address of the line's first instruction from the debug symbol tables.
// scope is the path before it ("" for none). A line number is decimal,
// whatever the radix, as the debugger takes it.
func (e *Evaluator) parseLine(scope, s string) (uint32, string, error) {
	s = strings.TrimLeft(s, " \t")

	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}

	if i == 0 {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, "", vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	if e.Debug != nil {
		if v, ok := e.Debug.Line(scope, n); ok {
			return v, s[i:], nil
		}
	}

	name := "%LINE " + s[:i]
	if scope != "" {
		name = scope + `\` + name
	}

	return 0, "", vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
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
