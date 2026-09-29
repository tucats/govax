package asm

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// MACRO-32 conditional assembly: .IF condition argument(s) ... .ENDC, the
// subconditionals .IF_FALSE/.IF_TRUE/.IF_TRUE_FALSE (.IFF/.IFT/.IFTF)
// inside a block, and the one-line .IIF. The reference tool instead had
// ".IF expression [THEN] statement", assembling the rest of the line if
// the expression was nonzero; that form is gone.

// maxCondDepth is how deeply conditional blocks may nest, as in MACRO-32.
const maxCondDepth = 31

// condFrame is one open conditional assembly block.
type condFrame struct {
	outer bool // whether the enclosing code is being assembled
	test  bool // the result of the block's .IF test
	on    bool // whether the current part of the block is assembled
}

// skipping reports whether statements are currently being left out by
// an unsatisfied conditional.
func (a *Assembler) skipping() bool {
	return len(a.cond) > 0 && !a.cond[len(a.cond)-1].on
}

// skippedStatement handles a statement while skipping: only the
// conditional directives are looked at (to track nesting and the
// subconditionals); everything else, labels included, is left out.
func (a *Assembler) skippedStatement(line string) error {
	c := newCursor(line)
	c.skipBlanks()

	// A label is left out along with its statement.
	if i := strings.IndexByte(c.rest(), ':'); i > 0 && !strings.ContainsAny(c.rest()[:i], " \t") {
		c.skip(i + 1)

		if c.peek() == ':' {
			c.next()
		}

		c.skipBlanks()
	}

	if c.peek() == '.' {
		c.next()
	}

	switch name := scanName(c); name {
	case "IF":
		if len(a.cond) >= maxCondDepth {
			return vmserrors.New(vmserrors.VAX_CONDDEPTH, maxCondDepth)
		}

		// Not evaluated: the block is left out whatever its test.
		a.cond = append(a.cond, condFrame{})

	case "IF_FALSE", "IFF", "IF_TRUE", "IFT", "IF_TRUE_FALSE", "IFTF", "ENDC":
		return a.subconditional(name)
	}

	return nil
}

// pseudoIf assembles .IF condition argument(s), opening a conditional
// assembly block that runs to the matching .ENDC.
func (a *Assembler) pseudoIf(c *cursor) error {
	if len(a.cond) >= maxCondDepth {
		return vmserrors.New(vmserrors.VAX_CONDDEPTH, maxCondDepth)
	}

	test, err := a.condition(c)
	if err != nil {
		return err
	}

	c.skipBlanks()

	if !c.atEnd() {
		return vmserrors.New(vmserrors.VAX_EXTRATEXT, c.rest())
	}

	a.cond = append(a.cond, condFrame{outer: true, test: test, on: test})

	return nil
}

// subconditional handles .ENDC and the subconditional directives, which
// switch the rest of the block on or off by the block's own test result.
// Inside a block whose enclosing code is left out, they leave it out too.
func (a *Assembler) subconditional(name string) error {
	if len(a.cond) == 0 {
		return vmserrors.New(vmserrors.VAX_NOCOND, "."+name)
	}

	top := &a.cond[len(a.cond)-1]

	switch name {
	case "ENDC":
		a.cond = a.cond[:len(a.cond)-1]

	case "IF_FALSE", "IFF":
		top.on = top.outer && !top.test

	case "IF_TRUE", "IFT":
		top.on = top.outer && top.test

	default: // IF_TRUE_FALSE, IFTF
		top.on = top.outer
	}

	return nil
}

// pseudoIif assembles .IIF condition [,]argument(s), statement: the
// statement is assembled if the condition is met.
func (a *Assembler) pseudoIif(c *cursor) error {
	test, err := a.condition(c)
	if err != nil {
		return err
	}

	c.skipBlanks()

	if c.next() != ',' {
		return vmserrors.New(vmserrors.VAX_BADCOND, "missing \",\" before the .IIF statement")
	}

	if !test {
		return nil
	}

	return a.assembleStatement(c.rest())
}

// condTests maps each condition test's long and short names to its
// canonical short name.
var condTests = map[string]string{
	"EQUAL": "EQ", "EQ": "EQ",
	"NOT_EQUAL": "NE", "NE": "NE",
	"GREATER": "GT", "GT": "GT",
	"LESS_EQUAL": "LE", "LE": "LE",
	"LESS_THAN": "LT", "LT": "LT",
	"GREATER_EQUAL": "GE", "GE": "GE",
	"DEFINED": "DF", "DF": "DF",
	"NOT_DEFINED": "NDF", "NDF": "NDF",
	"BLANK": "B", "B": "B",
	"NOT_BLANK": "NB", "NB": "NB",
	"IDENTICAL": "IDN", "IDN": "IDN",
	"DIFFERENT": "DIF", "DIF": "DIF",
}

// condition reads a condition test and its arguments and reports whether
// the condition is met. The test is separated from its arguments by a
// comma or blanks. An expression argument may not use a symbol not yet
// defined.
func (a *Assembler) condition(c *cursor) (bool, error) {
	c.skipBlanks()

	name := scanName(c)

	test, ok := condTests[name]
	if !ok {
		return false, vmserrors.New(vmserrors.VAX_BADCOND, "unknown condition test \""+name+"\"")
	}

	c.skipBlanks()

	if c.peek() == ',' {
		c.next()
		c.skipBlanks()
	}

	switch test {
	case "DF", "NDF":
		defined, err := a.definedTest(c)

		return defined == (test == "DF"), err

	case "B", "NB":
		arg, err := condArgument(c)

		return (strings.TrimSpace(arg) == "") == (test == "B"), err

	case "IDN", "DIF":
		first, err := condArgument(c)
		if err != nil {
			return false, err
		}

		c.skipBlanks()

		if c.peek() == ',' {
			c.next()
		}

		second, err := condArgument(c)

		return (first == second) == (test == "IDN"), err
	}

	v, err := a.exprNoForward(c)
	if err != nil {
		return false, err
	}

	n := int32(v)

	switch test {
	case "EQ":
		return n == 0, nil
	case "NE":
		return n != 0, nil
	case "GT":
		return n > 0, nil
	case "LE":
		return n <= 0, nil
	case "LT":
		return n < 0, nil
	}

	return n >= 0, nil // GE
}

// definedTest reads the DEFINED/NOT_DEFINED argument: a symbol, or
// symbols joined by & (all defined) and ! (either defined), applied left
// to right, and reports whether it is defined.
func (a *Assembler) definedTest(c *cursor) (bool, error) {
	result := false
	op := byte(0)

	for {
		c.skipBlanks()

		name, ok := scanLocalLabel(c)
		if !ok {
			name = scanName(c)
		}

		if name == "" {
			return false, vmserrors.New(vmserrors.VAX_BADCOND, "missing symbol name")
		}

		resolved, _ := a.resolvedName(name)
		sym, found := a.symbols.find(resolved)
		defined := found && len(sym.forward) == 0

		switch op {
		case '&':
			result = result && defined
		case '!':
			result = result || defined
		default:
			result = defined
		}

		c.skipBlanks()

		if c.peek() != '&' && c.peek() != '!' {
			return result, nil
		}

		op = c.next()
	}
}

// condArgument reads a BLANK/IDENTICAL-style argument: text in angle
// brackets, or a run of characters up to a comma or blank.
func condArgument(c *cursor) (string, error) {
	c.skipBlanks()

	if c.peek() != '<' {
		start := c.pos
		for !c.atEnd() && c.peek() != ',' && !isBlank(c.peek()) {
			c.next()
		}

		return c.s[start:c.pos], nil
	}

	c.next()

	start, depth := c.pos, 1

	for ; !c.atEnd(); c.next() {
		switch c.peek() {
		case '<':
			depth++
		case '>':
			depth--
		}

		if depth == 0 {
			arg := c.s[start:c.pos]
			c.next()

			return arg, nil
		}
	}

	return "", vmserrors.New(vmserrors.VAX_NOCLOSE, ">")
}
