package console

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
)

// DCL symbols and foreign commands (docs/PHASE-34.md, 2026-10-04).
//
// A DCL symbol is a name for a string, defined at the console prompt by
// an assignment statement:
//
//	FORTH :== $DUA0:[000000]FORTH.EXE
//	DIR*ECTORY :== DIRECTORY/SIZE
//	GREETING == "Hello there"
//
// When a command's first word is a symbol, DCL replaces the word with the
// symbol's value before reading the command (DCL User's Guide, "Symbol
// Substitution"). A value that starts with "$" makes the symbol a foreign
// command: the rest of the value names an image, which runs as RUN would
// run it, and the rest of the command line is the text the image reads
// with LIB$GET_FOREIGN. Any other value is the start of a command, so a
// symbol is also an abbreviation or an alias for a command.
//
// These are not the console's VAX symbols (symbols.go, which ASM, CALL,
// and EXAMINE use, and SHOW SYMBOL lists): they're a separate table, as
// DCL's symbols are separate from an image's.
//
// What's here, and what isn't:
//
//   - ":=" and ":==" assign a string: the rest of the line, with DCL's
//     usual treatment (dclText). "=" and "==" assign an expression's value;
//     the console evaluates only a quoted string or a decimal integer.
//   - DCL has local (":=", "=") and global (":==", "==") symbols. The
//     console has no command procedures for local symbols to be local to,
//     so it keeps one table, which every form assigns.
//   - An "*" in the name, as in "DIR*ECTORY", marks how short an
//     abbreviation of the name still means the symbol.
//   - DELETE/SYMBOL name removes one (/GLOBAL and /LOCAL are accepted and
//     mean nothing more).
//   - DCL defaults a foreign command's image to SYS$SYSTEM:; the console
//     finds it as RUN finds an image instead (unconfirmed nuance: govax
//     has no SYS$SYSTEM to default to).
//   - SHOW SYMBOL/DCL [name] shows them (ShowDCLSymbols), as DCL's SHOW
//     SYMBOL does; the console's own SHOW SYMBOL, without /DCL, shows
//     its VAX symbols.
//   - Not here: apostrophe substitution ('SYMBOL') inside a command.

// maxSymbolDepth is how many times symbol substitution may rewrite one
// command: enough for an alias of an alias, short of an alias of itself.
const maxSymbolDepth = 16

// dclSymbol is one DCL symbol.
type dclSymbol struct {
	// name is the symbol's full name, uppercase, without any "*".
	name string

	// minLength is the length of the shortest abbreviation of name that
	// means this symbol: where an "*" was, or all of name.
	minLength int

	// value is the string the symbol stands for.
	value string

	// global says it was assigned with ":==" or "==" (a global symbol,
	// to DCL), not ":=" or "=" (a local one); the console keeps them in
	// one table, but SHOW SYMBOL/DCL shows which.
	global bool

	// integer says "=" or "==" gave it an integer value, which SHOW
	// SYMBOL/DCL shows as a number, not a string.
	integer bool
}

// dclSymbols is the console's table of DCL symbols, by full name.
type dclSymbols map[string]dclSymbol

// lookup returns the symbol word names: the one whose full name it is, or
// else the one it's an allowed abbreviation of.
func (t dclSymbols) lookup(word string) (dclSymbol, bool) {
	word = strings.ToUpper(word)

	if sym, ok := t[word]; ok {
		return sym, true
	}

	// Names are checked in order, so that which symbol an abbreviation
	// means never depends on map order.
	names := make([]string, 0, len(t))
	for name := range t {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		sym := t[name]
		if len(word) >= sym.minLength && strings.HasPrefix(name, word) {
			return sym, true
		}
	}

	return dclSymbol{}, false
}

// DCLSymbol returns the value of the DCL symbol name means, and whether
// there is one.
func (c *Console) DCLSymbol(name string) (string, bool) {
	sym, ok := c.dclSymbols.lookup(name)

	return sym.value, ok
}

// dclSymbolLine handles line if it's a symbol assignment, a DELETE/SYMBOL,
// or a command whose first word is a DCL symbol, reporting whether it
// was one of those.
func (d *Dispatcher) dclSymbolLine(line string) (bool, error) {
	if name, op, value, ok := splitAssignment(line); ok {
		return true, d.Console.assignSymbol(name, op, value)
	}

	verb, rest := readCommandVerb(line)

	if isDeleteSymbol(verb, rest) {
		return true, d.Console.deleteSymbols(rest)
	}

	sym, ok := d.Console.dclSymbols.lookup(verb)
	if !ok {
		return false, nil
	}

	if d.symbolDepth >= maxSymbolDepth {
		return true, vmserrors.New(vmserrors.CLI_SYMDEPTH, sym.name)
	}

	if image, foreign := strings.CutPrefix(sym.value, "$"); foreign {
		return true, d.runForeign(image, rest)
	}

	d.symbolDepth++
	defer func() { d.symbolDepth-- }()

	return true, d.Dispatch(sym.value + rest)
}

// runForeign runs a foreign command: the image named by spec (a symbol's
// value after its "$"), given the rest of the command line, after DCL's
// treatment of it, as its command text.
func (d *Dispatcher) runForeign(spec, rest string) error {
	spec = strings.Trim(strings.TrimSpace(spec), `"`)
	if spec == "" {
		return vmserrors.New(vmserrors.CLI_NOFILE)
	}

	opts := RunOptions{RunInits: d.Console.DefaultRunInits(), CommandLine: dclText(rest, true)}

	return d.Console.Run(spec, opts)
}

// splitAssignment splits line, if it's a symbol assignment, into the
// symbol's name (as written, "*" and all), the operator (":=", ":==",
// "=", or "=="), and the text after the operator.
func splitAssignment(line string) (name, op, value string, ok bool) {
	i := 0
	for i < len(line) && isDCLSymbolChar(line[i]) {
		i++
	}

	name = line[:i]
	if name == "" || !isDCLSymbolStart(name[0]) {
		return "", "", "", false
	}

	rest := strings.TrimLeft(line[i:], " \t")

	for _, o := range []string{":==", ":=", "==", "="} {
		if strings.HasPrefix(rest, o) {
			return name, o, rest[len(o):], true
		}
	}

	return "", "", "", false
}

// isDCLSymbolStart reports whether ch can start a symbol name: a letter, "$",
// or "_".
func isDCLSymbolStart(ch byte) bool {
	return ch == '$' || ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

// isDCLSymbolChar reports whether ch can be in a symbol name as written: a
// letter, digit, "$", "_", or the "*" that marks an abbreviation.
func isDCLSymbolChar(ch byte) bool {
	return isDCLSymbolStart(ch) || (ch >= '0' && ch <= '9') || ch == '*'
}

// assignSymbol defines (or redefines) the symbol written as name in the
// console's table.
func (c *Console) assignSymbol(name, op, text string) error {
	return c.dclSymbols.assign(name, op, text)
}

// assign defines (or redefines) the symbol written as name in t, with
// operator op and the text after it (see splitAssignment).
func (t *dclSymbols) assign(name, op, text string) error {
	full := strings.ToUpper(strings.Replace(name, "*", "", 1))
	if strings.Contains(full, "*") {
		return vmserrors.New(vmserrors.CLI_EXPSYN, name)
	}

	minLength := len(full)
	if star := strings.IndexByte(name, '*'); star > 0 {
		minLength = star
	}

	var (
		value   string
		integer bool
	)

	if strings.HasPrefix(op, ":") {
		value = dclText(text, false)
	} else {
		v, isInt, err := symbolExpression(text)
		if err != nil {
			return err
		}

		value, integer = v, isInt
	}

	if *t == nil {
		*t = dclSymbols{}
	}

	(*t)[full] = dclSymbol{
		name: full, minLength: minLength, value: value,
		global: strings.HasSuffix(op, "=="), integer: integer,
	}

	return nil
}

// symbolExpression evaluates the expression an "=" or "==" assignment
// gives: a quoted string (with "" for a quote inside it) or a decimal
// integer, whose value is its decimal string (integer is then true).
func symbolExpression(text string) (value string, integer bool, err error) {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, `"`) {
		s, n, ok := quotedString(text)
		if !ok {
			return "", false, vmserrors.New(vmserrors.CLI_UNTERMSTR)
		}

		if strings.TrimSpace(text[n:]) != "" {
			return "", false, vmserrors.New(vmserrors.CLI_EXPSYN, text)
		}

		return s, false, nil
	}

	v, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return "", false, vmserrors.New(vmserrors.CLI_EXPSYN, text)
	}

	return strconv.FormatInt(v, 10), true, nil
}

// quotedString reads the quoted string text starts with, returning its
// contents (a doubled quote inside it is one quote), the length of text
// it took up, and whether it was closed.
func quotedString(text string) (string, int, bool) {
	var b strings.Builder

	for i := 1; i < len(text); i++ {
		if text[i] != '"' {
			b.WriteByte(text[i])

			continue
		}

		if i+1 < len(text) && text[i+1] == '"' {
			b.WriteByte('"')

			i++

			continue
		}

		return b.String(), i + 1, true
	}

	return "", len(text), false
}

// dclText is DCL's treatment of the text of a command's parameters, or of
// a string assignment's value: outside quotes, letters are uppercased,
// each run of blanks and tabs becomes one blank, and a "!" starts a
// comment that ends the text; leading and trailing blanks are dropped.
// Quoted text is kept as written. keepQuotes keeps the quotes themselves
// (a foreign command's text, which the image parses); otherwise they're
// removed, and a doubled quote inside them is one quote (a string
// assignment's value).
//
// Unconfirmed against VMS: that a foreign command's text keeps its quotes
// and its quoted text's case, as LIB$GET_FOREIGN returns it.
func dclText(text string, keepQuotes bool) string {
	var b strings.Builder

	quoted, blank := false, false

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if !quoted {
			if ch == ' ' || ch == '\t' {
				blank = true

				continue
			}

			if ch == '!' {
				break
			}

			if blank && b.Len() > 0 {
				b.WriteByte(' ')
			}

			blank = false
		}

		switch {
		case ch == '"' && quoted && i+1 < len(text) && text[i+1] == '"':
			// A doubled quote inside quotes is a quote.
			if keepQuotes {
				b.WriteString(`""`)
			} else {
				b.WriteByte('"')
			}

			i++

		case ch == '"':
			quoted = !quoted

			if keepQuotes {
				b.WriteByte(ch)
			}

		case !quoted && ch >= 'a' && ch <= 'z':
			b.WriteByte(ch - ('a' - 'A'))

		default:
			b.WriteByte(ch)
		}
	}

	return b.String()
}

// isDeleteSymbol reports whether a command whose verb and rest these are
// is DELETE/SYMBOL: the verb an abbreviation of DELETE (at least "DEL"),
// followed by /SYMBOL (at least "/SY").
func isDeleteSymbol(verb, rest string) bool {
	v := strings.ToUpper(verb)
	if len(v) < 3 || !strings.HasPrefix("DELETE", v) {
		return false
	}

	qual := strings.ToUpper(strings.TrimLeft(rest, " \t"))
	if !strings.HasPrefix(qual, "/") {
		return false
	}

	word, _, _ := strings.Cut(qual[1:], "/")
	word, _, _ = strings.Cut(word, " ")

	return len(word) >= 2 && strings.HasPrefix("SYMBOL", word)
}

// deleteSymbols is DELETE/SYMBOL: rest is its qualifiers, then the name
// of the symbol to delete (all of it, or an allowed abbreviation).
func (c *Console) deleteSymbols(rest string) error {
	return c.dclSymbols.delete(rest)
}

// delete is DELETE/SYMBOL on t (deleteSymbols).
func (t dclSymbols) delete(rest string) error {
	name := ""

	for _, field := range strings.Fields(strings.ReplaceAll(rest, "/", " /")) {
		if !strings.HasPrefix(field, "/") {
			name = field
		}
	}

	sym, ok := t.lookup(name)
	if name == "" || !ok {
		return vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
	}

	delete(t, sym.name)

	return nil
}

// clone returns a copy of t: what a spawned subprocess's CLI starts with
// (subcli.go).
func (t dclSymbols) clone() dclSymbols {
	c := make(dclSymbols, len(t))
	for name, sym := range t {
		c[name] = sym
	}

	return c
}

// ShowDCLSymbols is SHOW SYMBOL/DCL [name]: the DCL symbol name means (an
// abbreviation it allows will do), the ones a wildcard name matches, or
// with no name every one, in name order, as DCL's SHOW SYMBOL shows them:
//
//	FO*RTH == "$DUA0:[TOOLS]FORTH.EXE"
//	COUNT = 42   Hex = 0000002A  Octal = 00000000052
//
// "==" is a global symbol's, "=" a local one's (see assignSymbol), and an
// "*" marks the shortest abbreviation. A quote in a string value is
// doubled. Unconfirmed against VMS: the integer line's spacing.
func (c *Console) ShowDCLSymbols(name string) error {
	name = strings.ToUpper(strings.TrimSpace(name))

	shown := c.dclSymbols.matching(name)
	if len(shown) == 0 {
		if name == "" {
			c.Printf("No DCL symbols are defined\n")

			return nil
		}

		return vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
	}

	for _, sym := range shown {
		c.Printf("%s\n", sym.showLine())
	}

	return nil
}

// matching returns the symbols SHOW SYMBOL shows for name (in upper
// case): the one it means (an abbreviation it allows will do), the ones
// a wildcard name matches, or with no name every one, in name order.
func (t dclSymbols) matching(name string) []dclSymbol {
	if name != "" && !lnm.HasWildcards(name) {
		if sym, ok := t.lookup(name); ok {
			return []dclSymbol{sym}
		}

		return nil
	}

	var shown []dclSymbol

	for _, sym := range t {
		if name == "" || lnm.Match(name, sym.name) {
			shown = append(shown, sym)
		}
	}

	sort.Slice(shown, func(i, j int) bool { return shown[i].name < shown[j].name })

	return shown
}

// showLine is sym's line in SHOW SYMBOL/DCL.
func (sym dclSymbol) showLine() string {
	name := sym.name
	if sym.minLength < len(name) {
		name = name[:sym.minLength] + "*" + name[sym.minLength:]
	}

	op := "="
	if sym.global {
		op = "=="
	}

	if sym.integer {
		v, _ := strconv.ParseInt(sym.value, 10, 32)

		return fmt.Sprintf("  %s %s %d   Hex = %08X  Octal = %011o", name, op, v, uint32(v), uint32(v))
	}

	return fmt.Sprintf("  %s %s \"%s\"", name, op, strings.ReplaceAll(sym.value, `"`, `""`))
}
