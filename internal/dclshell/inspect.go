package dclshell

// What the interpreter shows of its state, for the console's tests and
// tools: symbols, open files, verification, SET CONTROL, and expression
// evaluation as DCL does it.

// Symbol is the value of the DCL symbol name, local or global, as DCL
// looks it up, and true; false if there is none.
func (c *Interpreter) Symbol(name string) (string, bool) {
	sym, ok := c.dclSymbols.lookup(name)

	return sym.value, ok
}

// DeleteSymbol is DELETE/SYMBOL with args (its qualifiers and name).
func (c *Interpreter) DeleteSymbol(args string) error { return c.dclSymbols.delete(args) }

// SymbolLevels is the number of symbol tables kept: the global one's
// level and one local table per command level.
func (c *Interpreter) SymbolLevels() int { return len(c.dclSymbols.levels) }

// EvaluateExpression evaluates text as a DCL expression, upcased outside
// its quotes as DCL reads a command, returning its value as text and
// whether it is an integer.
func (c *Interpreter) EvaluateExpression(text string) (value string, integer bool, err error) {
	v, err := evaluateDCLExpression(upcaseOutsideQuotes(text), &c.dclSymbols, c)

	return v.String(), v.integer, err
}

// DCLFileOpen reports whether name is the logical name of a file DCL's
// OPEN has open.
func (c *Interpreter) DCLFileOpen(name string) bool {
	_, open := c.openFile(name)

	return open
}

// OrphanedDCLFiles is the number of files DCL's OPEN has open whose
// logical names are gone.
func (c *Interpreter) OrphanedDCLFiles() int {
	n := 0

	for _, f := range c.dclFiles {
		if g, open := c.openFile(f.logical); !open || g != f {
			n++
		}
	}

	return n
}

// VerifyImage is image verification (SET VERIFY=IMAGE), and VerifyPrefix
// SET PREFIX's control string.
func (c *Interpreter) VerifyImage() bool { return c.verifyImage }

// VerifyPrefix is SET PREFIX's control string.
func (c *Interpreter) VerifyPrefix() string { return c.verifyPrefix }

// ControlTEnabled is SET CONTROL's Ctrl/T setting.
func (c *Interpreter) ControlTEnabled() bool { return c.control.t }
