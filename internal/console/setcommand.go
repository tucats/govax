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

	g.Bind("SET_BREAKPOINT", func(id int64, r *dcl.Result) error {
		addr, err := d.evalWhole(r.String("ADDRESS"))
		if err != nil {
			return err
		}

		c.AddBreakpoint(addr)

		return nil
	})
	g.Bind("SET_BREAK_TEMPORARY", func(id int64, r *dcl.Result) error {
		addr, err := d.evalWhole(r.String("ADDRESS"))
		if err != nil {
			return err
		}

		c.AddTemporaryBreakpoint(addr)

		return nil
	})
	g.Bind("SET_BREAK_INSTRUCTION", func(id int64, r *dcl.Result) error {
		return c.AddInstructionBreakpoint(r.String("OPCODE"))
	})
	g.Bind("SET_BREAK_FAULT", func(id int64, r *dcl.Result) error {
		return c.AddFaultBreakpoint(r.String("FAULT"))
	})

	g.Bind("SET_STEP", func(id int64, r *dcl.Result) error { return c.SetStepMode(r.String("MODE")) })

	// SET TRACE, or SET NOTRACE: the keyword's NO is the parameter's.
	g.Bind("SET_TRACE", func(id int64, r *dcl.Result) error {
		c.SetTrace(!r.Negated("WHAT"))

		return nil
	})

	g.Bind("SET_DEBUG", func(id int64, r *dcl.Result) error { return c.SetDebug(r.List("FLAGS")) })

	g.Bind("SET_PSL", func(id int64, r *dcl.Result) error {
		for _, clause := range r.List("FIELDS") {
			field, v, err := d.fieldAssignment(clause, vmserrors.CLI_INVSETPSL)
			if err != nil {
				return err
			}

			if err := c.SetPSLField(field, v); err != nil {
				return err
			}
		}

		return nil
	})

	g.Bind("SET_MODE", func(id int64, r *dcl.Result) error { return c.SetMode(r.String("MODE")) })
	g.Bind("SET_PTE", d.setPTECommand)

	g.Bind("SET_FAULT_HISTORY", func(id int64, r *dcl.Result) error {
		return c.SetFaultHistory(int(r.Int("COUNT")))
	})

	g.Bind("SET_VM", func(id int64, r *dcl.Result) error { return c.SetVM(!r.Negated("WHAT")) })

	g.Bind("SET_BASE", func(id int64, r *dcl.Result) error {
		addr, err := d.evalWhole(r.String("ADDRESS"))
		if err != nil {
			return err
		}

		return c.SetBase(addr)
	})

	g.Bind("SET_VERBOSE", func(id int64, r *dcl.Result) error {
		if r.Negated("WHAT") {
			return c.SetNoVerbose()
		}

		return c.SetVerbose()
	})
	g.Bind("SET_VERIFY", func(id int64, r *dcl.Result) error { return c.SetVerify() })

	g.Bind("SET_QUANTUM", func(id int64, r *dcl.Result) error { return c.SetQuantum(int(r.Int("COUNT"))) })
	g.Bind("SET_UIQUANTUM", func(id int64, r *dcl.Result) error { return c.SetUIQuantum(int(r.Int("COUNT"))) })

	// SET DEFAULT (docs/PHASE-23.md, subtask 3) establishes the
	// operator's default device and directory for DIRECTORY, DELETE,
	// PURGE, COPY, and TYPE; Session.SetDefault checks its syntax.
	g.Bind("SET_DEFAULT", func(id int64, r *dcl.Result) error { return c.SetDefault(r.String("SPEC")) })
}

// parseRadixArg accepts either console_set.c's own HEX/HEXA/16/DEC/DECI/10
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

// fieldAssignment splits one "field=value" clause of SET PSL or SET PTE
// at its first "=" and evaluates the value; a clause with no "=" is
// reported with status bad.
func (d *Dispatcher) fieldAssignment(clause string, bad uint32) (string, uint32, error) {
	field, value, ok := strings.Cut(clause, "=")
	if !ok {
		return "", 0, vmserrors.New(bad, clause)
	}

	v, err := d.evalWhole(strings.TrimSpace(value))
	if err != nil {
		return "", 0, err
	}

	return strings.TrimSpace(field), v, nil
}

// setPTECommand implements SET PTE/SET PAGE: "address [TO address]
// field=value[,...]", matching console_set.c's setpte_multiple/setpte/
// parse_pte_changes. A TO range applies the same changes to every
// 512-byte page from the first address to the second, inclusive.
func (d *Dispatcher) setPTECommand(id int64, r *dcl.Result) error {
	addr1, err := d.evalWhole(r.String("ADDRESS"))
	if err != nil {
		return err
	}

	addrs := []uint32{addr1}
	changes := strings.TrimSpace(r.String("CHANGES"))

	if word, rest := readCommandVerb(changes); strings.EqualFold(word, "TO") && strings.HasPrefix(rest, " ") {
		addr2, tail, err := d.Console.Evaluator().Eval(rest)
		if err != nil {
			return err
		}

		changes = strings.TrimSpace(tail)

		a1, a2 := addr1&^0x1FF, addr2&^0x1FF
		if a2 < a1 {
			return vmserrors.New(vmserrors.CLI_BADRANGE)
		}

		addrs = addrs[:0]
		for a := a1; a <= a2; a += 512 {
			addrs = append(addrs, a)
		}
	}

	if changes == "" {
		return vmserrors.New(vmserrors.CLI_BADSETSYNTAX, changes)
	}

	for _, addr := range addrs {
		for _, clause := range strings.Split(changes, ",") {
			if clause = strings.TrimSpace(clause); clause == "" {
				continue
			}

			field, v, err := d.fieldAssignment(clause, vmserrors.CLI_BADPTEFIELD)
			if err != nil {
				return err
			}

			if err := d.Console.SetPTE(addr, field, v); err != nil {
				return err
			}
		}
	}

	return nil
}
