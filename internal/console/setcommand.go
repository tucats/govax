package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// bindSetCommands binds SET's syntaxes (console.dcl's set_types and
// set_* entries, docs/PHASE-37.md). Each SET keyword redirects the parse
// into its own syntax, so each form has its own handler; the Console
// methods that do the work are in set.go.
func (d *Dispatcher) bindSetCommands() {
	g := d.Grammar
	c := d.Console

	g.Bind("SET_SYMBOL", func(id int64, r *dcl.Result) error {
		val, err := d.evalWhole(r.String("VALUE"))
		if err != nil {
			return err
		}

		return c.SetSymbolQualified(r.String("NAME"), val, r.Present("PERMANENT"), r.Present("ENTRY"), r.Present("LABEL"))
	})

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
	g.Bind("SET_VERIFY", func(id int64, r *dcl.Result) error { return setVerifyCommand(c, r) })
	g.Bind("SET_PREFIX", func(id int64, r *dcl.Result) error {
		if r.Negated("WHAT") {
			return c.SetPrefix("")
		}

		return c.SetPrefix(r.String("TEXT"))
	})
	g.Bind("SET_ON", func(id int64, r *dcl.Result) error { return c.SetOn(!r.Negated("WHAT")) })

	g.Bind("SET_QUANTUM", func(id int64, r *dcl.Result) error { return c.SetQuantum(int(r.Int("COUNT"))) })
	g.Bind("SET_UIQUANTUM", func(id int64, r *dcl.Result) error { return c.SetUIQuantum(int(r.Int("COUNT"))) })

	// SET DEFAULT (docs/PHASE-23.md, subtask 3) establishes the
	// operator's default device and directory for DIRECTORY, DELETE,
	// PURGE, COPY, and TYPE; Session.SetDefault checks its syntax.
	g.Bind("SET_DEFAULT", func(id int64, r *dcl.Result) error { return c.SetDefault(r.String("SPEC")) })

	// SET PROMPT="text" changes the console's prompt.
	g.Bind("SET_PROMPT", func(id int64, r *dcl.Result) error { return c.SetPrompt(r.String("TEXT")) })
}

// parseRadixArg accepts either HEX/HEXA/16/DEC/DECI/10
// keyword forms or a bare number (this port's own pre-existing, more
// lenient numeric form, kept for backward compatibility — SetRadix itself
// rejects anything but 8/10/16 either way).
func parseRadixArg(s string) (int, bool) {
	switch strings.ToUpper(s) {
	case "HEX", "HEXA":
		return 16, true
	case "DEC", "DECI":
		return 10, true
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}

	return n, true
}

