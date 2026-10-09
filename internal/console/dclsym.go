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
// These are not the machine's symbol table (symbols.go, which ASM and
// EXAMINE use, and the debugger's SHOW SYMBOL and CANCEL SYMBOL show and
// remove): they're a separate table, as DCL's symbols are separate from
// an image's.
//
// What's here, and what isn't:
//
//   - ":=" and ":==" assign a string: the rest of the line, with DCL's
//     usual treatment (dclText). "=" and "==" assign an expression's value;
//     the console evaluates only a quoted string or a decimal integer.
//   - DCL has local (":=", "=") and global (":==", "==") symbols, kept in
//     separate tables (dclSymbolTable): one global table, and a local
//     table for each command level. The terminal is level 0, and each
//     command procedure (@file, procedure.go) runs a level deeper, its
//     local symbols, P1 to P8 among them, gone when it ends.
//   - An "*" in the name, as in "DIR*ECTORY", marks how short an
//     abbreviation of the name still means the symbol.
//   - DELETE/SYMBOL [/LOCAL | /GLOBAL] [/ALL] [name] removes one, or all
//     of a table's.
//   - DCL defaults a foreign command's image to SYS$SYSTEM:; the console
//     finds it as RUN finds an image instead (unconfirmed nuance: govax
//     has no SYS$SYSTEM to default to).
//   - SHOW SYMBOL [/LOCAL | /GLOBAL] [/ALL] [name] shows them
//     (ShowDCLSymbols), as DCL's SHOW SYMBOL does.
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

	// global says it was assigned with ":==" or "==", so it's in the
	// global table, not a local one (":=" or "="); SHOW SYMBOL shows
	// which by its "==" or "=".
	global bool

	// integer says "=" or "==" gave it an integer value, which SHOW
	// SYMBOL shows as a number, not a string.
	integer bool
}

// dclSymbols is one table of DCL symbols, by full name.
type dclSymbols map[string]dclSymbol

// dclSymbolTable is a command interpreter's DCL symbols: the global
// table, and the local table of each command level. levels[0] is the
// interactive level (the terminal's); a command procedure runs one
// level deeper (push and pop), and its local symbols go when it ends. A name is looked
// up in the current level's local table, then the levels outside it, and
// then the global table (DCL User's Guide, "Symbol Tables"). The zero
// value is an empty table, ready to use.
type dclSymbolTable struct {
	// global is the global symbol table.
	global dclSymbols

	// levels are the local symbol tables, outermost (the interactive
	// level) first; the last is the current command level's.
	levels []dclSymbols
}

// symbolScope is which of a dclSymbolTable's tables a SHOW SYMBOL or
// DELETE/SYMBOL is about: /LOCAL's, /GLOBAL's, or (neither qualifier)
// the command's default.
type symbolScope int

const (
	// scopeDefault is neither /LOCAL nor /GLOBAL.
	scopeDefault symbolScope = iota

	// scopeLocal is /LOCAL: the current command level's local table.
	scopeLocal

	// scopeGlobal is /GLOBAL: the global table.
	scopeGlobal
)

// push starts a new command level's local table, empty, for a command
// procedure (procedure.go); pop ends it. Only the input stack pushes and
// pops, so the symbol levels above level 0 are its levels, one for one.
func (t *dclSymbolTable) push() {
	t.local()
	t.levels = append(t.levels, dclSymbols{})
}

// pop discards the current command level's local table, and every
// symbol in it. Level 0's, the terminal's, is never popped.
func (t *dclSymbolTable) pop() {
	if len(t.levels) > 1 {
		t.levels = t.levels[:len(t.levels)-1]
	}
}

// local returns the current command level's local table, making the
// interactive level's if there's none yet.
func (t *dclSymbolTable) local() dclSymbols {
	if len(t.levels) == 0 {
		t.levels = []dclSymbols{{}}
	}

	return t.levels[len(t.levels)-1]
}

// globals returns the global table, making it if there's none yet.
func (t *dclSymbolTable) globals() dclSymbols {
	if t.global == nil {
		t.global = dclSymbols{}
	}

	return t.global
}

// searchOrder returns the tables a name is looked up in, in order: the
// local tables from the current level out, then the global table.
func (t *dclSymbolTable) searchOrder() []dclSymbols {
	tables := make([]dclSymbols, 0, len(t.levels)+1)

	for i := len(t.levels) - 1; i >= 0; i-- {
		tables = append(tables, t.levels[i])
	}

	return append(tables, t.global)
}

// lookup returns the symbol word means, searching the tables in DCL's
// order (searchOrder).
func (t *dclSymbolTable) lookup(word string) (dclSymbol, bool) {
	for _, table := range t.searchOrder() {
		if sym, ok := table.lookup(word); ok {
			return sym, true
		}
	}

	return dclSymbol{}, false
}

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
// operator op and the text after it (see splitAssignment): in the global
// table for ":==" or "==", otherwise in the current level's local table.
// A local and a global symbol may have the same name; the local one is
// found first.
func (t *dclSymbolTable) assign(name, op, text string) error {
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

	global := strings.HasSuffix(op, "==")

	table := t.local()
	if global {
		table = t.globals()
	}

	table[full] = dclSymbol{name: full, minLength: minLength, value: value, global: global, integer: integer}

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

// deleteSymbols is DELETE/SYMBOL: rest is its qualifiers and the name of
// the symbol to delete.
func (c *Console) deleteSymbols(rest string) error {
	return c.dclSymbols.delete(rest)
}

// symbolCommand is what SHOW SYMBOL's and DELETE/SYMBOL's qualifiers and
// parameter say: the table (/LOCAL or /GLOBAL), /ALL, and the name.
type symbolCommand struct {
	scope symbolScope
	all   bool
	name  string
}

// parseSymbolCommand reads the qualifiers and the symbol name of a SHOW
// SYMBOL or DELETE/SYMBOL command line (its text after the verb), as
// DCL does: each qualifier may be abbreviated to its first letter,
// /SYMBOL (DELETE's) to two. A qualifier it doesn't know, a second name,
// or /LOCAL with /GLOBAL is CLI_EXPSYN.
func parseSymbolCommand(rest string) (symbolCommand, error) {
	var cmd symbolCommand

	local, global := false, false

	for _, field := range strings.Fields(strings.ReplaceAll(rest, "/", " /")) {
		word, isQualifier := strings.CutPrefix(strings.ToUpper(field), "/")

		switch {
		case !isQualifier && cmd.name == "":
			cmd.name = strings.ToUpper(field)
		case !isQualifier:
			return cmd, vmserrors.New(vmserrors.CLI_EXPSYN, field)
		case word != "" && strings.HasPrefix("LOCAL", word):
			local = true
		case word != "" && strings.HasPrefix("GLOBAL", word):
			global = true
		case word != "" && strings.HasPrefix("ALL", word):
			cmd.all = true
		case len(word) >= 2 && strings.HasPrefix("SYMBOL", word):
			// DELETE's /SYMBOL.
		default:
			return cmd, vmserrors.New(vmserrors.CLI_EXPSYN, field)
		}
	}

	switch {
	case local && global:
		return cmd, vmserrors.New(vmserrors.CLI_EXPSYN, "/LOCAL/GLOBAL")
	case local:
		cmd.scope = scopeLocal
	case global:
		cmd.scope = scopeGlobal
	}

	return cmd, nil
}

// delete is DELETE/SYMBOL on t, given the command's text after the verb
// (rest). The table is the current level's local one unless /GLOBAL says
// the global one (DCL's default is /LOCAL). The name may be any
// abbreviation the symbol allows; /ALL, with no name, deletes every
// symbol in the table. Unconfirmed against VMS: that a name given with
// /ALL is an error (CLI_EXPSYN).
func (t *dclSymbolTable) delete(rest string) error {
	cmd, err := parseSymbolCommand(rest)
	if err != nil {
		return err
	}

	table := t.local()
	if cmd.scope == scopeGlobal {
		table = t.globals()
	}

	switch {
	case cmd.all && cmd.name != "":
		return vmserrors.New(vmserrors.CLI_EXPSYN, cmd.name)
	case cmd.all:
		clear(table)

		return nil
	case cmd.name == "":
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "symbol")
	}

	sym, ok := table.lookup(cmd.name)
	if !ok {
		return vmserrors.New(vmserrors.CLI_UNDEFSYM, cmd.name)
	}

	delete(table, sym.name)

	return nil
}

// clone returns a copy of t, every table of it: what a spawned
// subprocess's CLI starts with (subcli.go), as LIB$SPAWN copies both
// the local and the global symbols. The subprocess starts at its own
// interactive level, so the local symbols of every level are copied
// into its one level, an inner level's replacing an outer one's.
func (t *dclSymbolTable) clone() dclSymbolTable {
	c := dclSymbolTable{global: dclSymbols{}, levels: []dclSymbols{{}}}

	for name, sym := range t.global {
		c.global[name] = sym
	}

	for _, level := range t.levels {
		for name, sym := range level {
			c.levels[0][name] = sym
		}
	}

	return c
}

// showDCLSymbols is the console's SHOW SYMBOL, as DCL's: cmd is the
// command's qualifiers and symbol name (see show). Each symbol is a line
//
//	FO*RTH == "$DUA0:[TOOLS]FORTH.EXE"
//	COUNT = 42   Hex = 0000002A  Octal = 00000000052
//
// "==" is a global symbol's, "=" a local one's, and an "*" marks the
// shortest abbreviation. A quote in a string value is doubled.
// Unconfirmed against VMS: the integer line's spacing.
func (c *Console) showDCLSymbols(cmd symbolCommand) error {
	shown, err := c.dclSymbols.show(cmd)
	if err != nil {
		return err
	}

	for _, sym := range shown {
		c.Printf("%s\n", sym.showLine())
	}

	return nil
}

// show returns the symbols SHOW SYMBOL shows for cmd, by DCL's rules:
//
//   - A name (or an abbreviation the symbol allows) is looked up in the
//     table /LOCAL or /GLOBAL names, or, with neither, in DCL's search
//     order (the local tables from the current level out, then the
//     global table); the first symbol found is shown.
//   - A name with the wildcards "*" and "%" shows every symbol it
//     matches in that table, or, with neither qualifier, in the current
//     level's local table and then the global table, each in name order.
//   - /ALL shows every symbol in the table /LOCAL or /GLOBAL names, the
//     current level's local table with neither; a name given too limits
//     it to the symbols the name matches.
//
// A name that finds nothing is CLI_UNDEFSYM; no name and no /ALL is
// CLI_MISSINGPARAMETER. /ALL of an empty table shows nothing.
// Unconfirmed against VMS: which tables a wildcard name searches with
// neither qualifier (the outer levels' local tables aren't searched),
// and that a name may be given with /ALL.
func (t *dclSymbolTable) show(cmd symbolCommand) ([]dclSymbol, error) {
	var tables []dclSymbols

	switch cmd.scope {
	case scopeLocal:
		tables = []dclSymbols{t.local()}
	case scopeGlobal:
		tables = []dclSymbols{t.global}
	case scopeDefault:
		switch {
		case cmd.all:
			tables = []dclSymbols{t.local()}
		case lnm.HasWildcards(cmd.name):
			tables = []dclSymbols{t.local(), t.global}
		default:
			tables = t.searchOrder()
		}
	}

	if cmd.name == "" && !cmd.all {
		return nil, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "symbol")
	}

	var shown []dclSymbol

	if cmd.all || lnm.HasWildcards(cmd.name) {
		for _, table := range tables {
			shown = append(shown, table.matching(cmd.name)...)
		}
	} else {
		for _, table := range tables {
			if sym, ok := table.lookup(cmd.name); ok {
				shown = []dclSymbol{sym}

				break
			}
		}
	}

	if len(shown) == 0 && cmd.name != "" {
		return nil, vmserrors.New(vmserrors.CLI_UNDEFSYM, cmd.name)
	}

	return shown, nil
}

// matching returns the symbols in t whose names pattern (in upper case,
// with VMS wildcards) matches, every one for an empty pattern, in name
// order.
func (t dclSymbols) matching(pattern string) []dclSymbol {
	var shown []dclSymbol

	for _, sym := range t {
		if pattern == "" || lnm.Match(pattern, sym.name) {
			shown = append(shown, sym)
		}
	}

	sort.Slice(shown, func(i, j int) bool { return shown[i].name < shown[j].name })

	return shown
}

// showLine is sym's line in SHOW SYMBOL.
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
