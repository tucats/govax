package asm

// cursor is a read position into one preprocessed source line (comment
// stripped, already uppercased outside quoted regions — see
// Assembler.preprocessLine). It plays the role of the C source's `char **`
// sliding pointer into a NUL-terminated line buffer, without the unsafety:
// isend(*p) becomes c.atEnd().
type cursor struct {
	s   string
	pos int
}

func newCursor(s string) *cursor { return &cursor{s: s} }

func (c *cursor) atEnd() bool { return c.pos >= len(c.s) }

// peek returns the byte at the cursor without consuming it, or 0 at end of
// line — matching the C source's convention of a NUL line terminator.
func (c *cursor) peek() byte {
	if c.atEnd() {
		return 0
	}

	return c.s[c.pos]
}

// peekAt returns the byte off positions ahead of the cursor, or 0 if that
// position is outside the line.
func (c *cursor) peekAt(off int) byte {
	i := c.pos + off
	if i < 0 || i >= len(c.s) {
		return 0
	}

	return c.s[i]
}

// next consumes and returns the current byte, or 0 at end of line.
func (c *cursor) next() byte {
	if c.atEnd() {
		return 0
	}

	ch := c.s[c.pos]
	c.pos++
	
	return ch
}

// skip advances the cursor past n bytes (clamped to the end of line).
func (c *cursor) skip(n int) {
	c.pos += n
	if c.pos > len(c.s) {
		c.pos = len(c.s)
	}
}

func isBlank(ch byte) bool { return ch == ' ' || ch == '\t' }

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isUpperAlpha(ch byte) bool { return ch >= 'A' && ch <= 'Z' }

// isSymbolChar matches the characters of a symbol/name token, used by
// symbol names, register names, mnemonics and pseudo-op names alike:
// letters, digits, '_', '$', and '.'. The C source had no '.'; MACRO-32
// allows it anywhere in a symbol, even first (the MACRO manual, chapter
// 3), as VMS's system macros' own symbols do. A "." standing alone is
// the location counter, not a symbol (see exprAtom).
func isSymbolChar(ch byte) bool {
	return isUpperAlpha(ch) || isDigit(ch) || ch == '_' || ch == '$' || ch == '.'
}

// isSymbolStart reports whether a symbol can start at c: a letter, '_',
// '$', or a '.' followed by another symbol character that isn't a digit
// (".5" is a number, and "." alone the location counter).
func isSymbolStart(c *cursor) bool {
	ch := c.peek()
	if ch == '.' {
		next := c.peekAt(1)

		return isSymbolChar(next) && !isDigit(next)
	}

	return isUpperAlpha(ch) || ch == '_' || ch == '$'
}

// blankRun returns how many blanks start at the cursor, without consuming
// them.
func blankRun(c *cursor) int {
	n := 0
	for isBlank(c.peekAt(n)) {
		n++
	}

	return n
}

func (c *cursor) skipBlanks() {
	for isBlank(c.peek()) {
		c.pos++
	}
}

// rest returns the unconsumed remainder of the line.
func (c *cursor) rest() string { return c.s[c.pos:] }
