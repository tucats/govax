package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// bindSetCommands binds SET's syntaxes (console.dcl's set_types and
// set_* entries, docs/PHASE-37.md). Each SET keyword redirects the parse
// into its own syntax, so each form has its own handler; the Console
// methods that do the work are in set.go.
func (d *Dispatcher) bindSetCommands() {
	g := d.Grammar
	c := d.Console

	g.Bind("SET_RADIX", func(id int64, r *dcl.Result) error {
		n, ok := parseRadixArg(r.String("RADIX"))
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADRADIXVAL, r.String("RADIX"))
		}

		return c.SetRadix(n)
	})

	g.Bind("SET_DEBUG", func(id int64, r *dcl.Result) error { return c.SetDebug(r.List("FLAGS")) })

	g.Bind("SET_VERBOSE", func(id int64, r *dcl.Result) error {
		if r.Negated("WHAT") {
			return c.SetNoVerbose()
		}

		return c.SetVerbose()
	})

	g.Bind("SET_QUANTUM", func(id int64, r *dcl.Result) error { return c.SetQuantum(int(r.Int("COUNT"))) })

	// SET DEFAULT (docs/PHASE-23.md, subtask 3) establishes the
	// operator's default device and directory for DIRECTORY, DELETE,
	// PURGE, COPY, and TYPE; Session.SetDefault checks its syntax.
	g.Bind("SET_DEFAULT", func(id int64, r *dcl.Result) error { return c.cmd().SetDefault(r.String("SPEC")) })

}

// radixNames are SET RADIX's keywords, each with its base. A keyword may
// be abbreviated to as few as three letters (HEX, DEC, OCT).
var radixNames = []struct {
	name string
	base int
}{
	{"HEXADECIMAL", 16},
	{"DECIMAL", 10},
	{"OCTAL", 8},
}

// parseRadixArg reads SET RADIX's argument: a keyword of radixNames,
// whole or abbreviated (SET RADIX HEXADECIMAL, HEX, HEXA), or a bare
// number, which SetRadix then checks is 8, 10, or 16.
func parseRadixArg(s string) (int, bool) {
	word := strings.ToUpper(strings.TrimSpace(s))

	for _, r := range radixNames {
		if len(word) >= 3 && strings.HasPrefix(r.name, word) {
			return r.base, true
		}
	}

	n, err := strconv.Atoi(word)
	if err != nil {
		return 0, false
	}

	return n, true
}
