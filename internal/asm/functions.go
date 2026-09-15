package asm

import "fmt"

// callFunction recognizes an expression function call — name already
// consumed, c positioned right after it — matching asm_function(). Reports
// matched=false (with value/err both zero) if name isn't a known function
// name, so the caller falls back to treating it as a plain symbol
// reference.
//
// Only the functions testdata/asm's fixtures actually use are implemented
// (DEFINED, VERBOSE); MKVALID/VMVALID are cheap to include for parity.
// asm_function()'s PMEMSIZE/LONG/WORD/BYTE/ULONG/UWORD/UBYTE/FILEEXISTS are
// omitted: the first needs a configurable memory size with no fixture that
// reads it, and the rest need live VAX memory or host filesystem access
// that has no meaning for a batch assembler with no attached machine.
func (a *Assembler) callFunction(name string, c *cursor) (value uint32, matched bool, err error) {
	switch name {
	case "DEFINED":
		return a.callDefined(c)
	case "VERBOSE":
		skipEmptyArgs(c)

		return boolToU32(a.verbose), true, nil
	case "MKVALID":
		skipEmptyArgs(c)

		return boolToU32(a.microkernel), true, nil
	case "VMVALID":
		skipEmptyArgs(c)

		return 1, true, nil
	}

	return 0, false, nil
}

// callDefined implements DEFINED("SYMBOL"): 1 if the named symbol is
// defined (with no pending forward references), 0 otherwise.
func (a *Assembler) callDefined(c *cursor) (uint32, bool, error) {
	c.skipBlanks()

	if c.peek() != '(' {
		return 0, true, fmt.Errorf("DEFINED() requires an argument")
	}

	c.next()
	c.skipBlanks()

	if c.peek() != '"' {
		return 0, true, fmt.Errorf("DEFINED() requires a quoted symbol name")
	}

	c.next()

	start := c.pos

	for !c.atEnd() && c.peek() != '"' {
		c.pos++
	}

	name := c.s[start:c.pos]

	if c.peek() == '"' {
		c.next()
	}

	c.skipBlanks()

	if c.peek() == ')' {
		c.next()
	}

	sym, ok := a.symbols.find(name)
	defined := ok && len(sym.forward) == 0

	return boolToU32(defined), true, nil
}

// skipEmptyArgs consumes an optional "()" argument list for a zero-
// argument function call (VERBOSE, MKVALID, VMVALID all accept either
// "NAME" or "NAME()").
func skipEmptyArgs(c *cursor) {
	save := c.pos

	c.skipBlanks()

	if c.peek() != '(' {
		c.pos = save

		return
	}

	c.next()
	c.skipBlanks()
	if c.peek() == ')' {
		c.next()

		return
	}

	c.pos = save
}
