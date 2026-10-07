package debugger

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// The commands that show or change the machine's own state: registers, the
// processor status longword, page tables, the translation buffer, the
// system control block, the stacks, and so on (docs/PHASE-42.md, subtask
// 11). Before this subtask they were console commands; they are the
// debugger's now, since looking at the machine is what a debugger is for.
//
// Most are govax's own (the VMS debugger has no SHOW PTE or SHOW SCB, for
// example), and keep the syntax and output the console gave them, so each
// handler here just asks the console, which still owns the code that
// reads the machine (the Show*, Set*, and Clear* methods of
// console.Console). What changes is only that numbers they take are read
// in the debugger's input radix (hexadecimal unless SET RADIX says
// otherwise) rather than the console's.
//
// The VMS debugger's own commands among them are SHOW CALLS, SHOW STACK
// (stack.go), SHOW MODE, and SHOW RADIX (modes.go), and CANCEL MODE.

// bindMachine binds the machine-state SHOW, SET, and CANCEL commands.
func (d *Dispatcher) bindMachine() {
	g := d.Grammar
	c := d.Debugger.Console

	// The SHOW commands that take nothing: each prints one table.
	for name, show := range map[string]func() error{
		"SHOW_REG":     c.ShowRegisters,
		"SHOW_PSL":     c.ShowPSL,
		"SHOW_CPU":     c.ShowCPU,
		"SHOW_CLOCK":   c.ShowClock,
		"SHOW_BASE":    c.ShowBase,
		"SHOW_MEMORY":  c.ShowMemory,
		"SHOW_TB":      c.ShowTB,
		"SHOW_REGIONS": c.ShowRegions,
		"SHOW_SHIM":    c.ShowShim,
		"SHOW_FAULT":   c.ShowFault,
		"SHOW_MODE":    d.Debugger.ShowMode,
		"SHOW_RADIX":   d.Debugger.ShowRadix,
	} {
		g.Bind(name, func(id int64, r *dcl.Result) error { return show() })
	}

	g.Bind("SHOW_PAGE", func(id int64, r *dcl.Result) error {
		address, err := d.Debugger.evalText(r.String("ADDRESS"))
		if err != nil {
			return err
		}

		// The console reads the expression in its own radix, so hand it the
		// value as a number with an explicit hexadecimal prefix.
		return c.ShowPage(fmt.Sprintf("^X%X", address), r.Present("WRITE"))
	})

	g.Bind("SHOW_SCB", func(id int64, r *dcl.Result) error {
		if r.Present("ALL") {
			return c.ShowSCBAll()
		}

		return c.ShowSCB()
	})

	g.Bind("SHOW_CALLS", func(id int64, r *dcl.Result) error { return d.Debugger.showCalls(r.String("COUNT")) })
	g.Bind("SHOW_STACK", func(id int64, r *dcl.Result) error { return d.Debugger.showStack(r.String("COUNT")) })

	// SHOW SP is the current stack; the others are each mode's own.
	for name, stack := range map[string]struct {
		kind    console.StackKind
		current bool
	}{
		"SHOW_SP":  {console.StackKSP, true},
		"SHOW_KSP": {console.StackKSP, false},
		"SHOW_ESP": {console.StackESP, false},
		"SHOW_SSP": {console.StackSSP, false},
		"SHOW_USP": {console.StackUSP, false},
		"SHOW_ISP": {console.StackISP, false},
	} {
		g.Bind(name, func(id int64, r *dcl.Result) error {
			count, err := d.Debugger.optionalNumber(r.String("COUNT"))
			if err != nil {
				return err
			}

			return c.ShowStack(stack.kind, stack.current, count, r.Present("ALL"))
		})
	}

	// SHOW R0, SHOW PC, SHOW IPL, ...: a register keyword with nothing
	// after it reaches the bare verb.
	g.Bind("SHOW", func(id int64, r *dcl.Result) error {
		return c.ShowRegisterOrPrivReg(r.Keyword("WHAT"))
	})

	g.Bind("SET_PSL", func(id int64, r *dcl.Result) error {
		for _, clause := range splitTop(r.String("FIELDS"), ',') {
			field, v, err := d.Debugger.fieldAssignment(clause, vmserrors.CLI_INVSETPSL)
			if err != nil {
				return err
			}

			if err := c.SetPSLField(field, v); err != nil {
				return err
			}
		}

		return nil
	})
	g.Bind("SET_PTE", d.setPTE)
	g.Bind("SET_FAULT_HISTORY", func(id int64, r *dcl.Result) error {
		return c.SetFaultHistory(int(r.Int("COUNT")))
	})
	g.Bind("SET_VM", func(id int64, r *dcl.Result) error { return c.SetVM(!r.Negated("WHAT")) })
	g.Bind("SET_BASE", func(id int64, r *dcl.Result) error {
		address, err := d.Debugger.evalText(r.String("ADDRESS"))
		if err != nil {
			return err
		}

		return c.SetBase(address)
	})

	g.Bind("CANCEL_MODE", func(id int64, r *dcl.Result) error { return d.Debugger.cancelMode() })
	g.Bind("CANCEL_TB", func(id int64, r *dcl.Result) error { return c.ClearTB() })
	g.Bind("CANCEL_MEMORY", func(id int64, r *dcl.Result) error {
		return vmserrors.New(vmserrors.DBG_SYNTAX, "MEMORY")
	})

	g.Bind("CANCEL_INTERRUPT", func(id int64, r *dcl.Result) error {
		if r.Present("ALL") {
			return c.ClearAllInterrupts()
		}

		code, err := d.Debugger.optionalNumber(r.String("CODE"))
		if err != nil {
			return err
		}

		return c.ClearInterrupt(code)
	})
}

// evalText is evalWhole for a command's whole parameter, which may have
// blanks around it.
func (d *Debugger) evalText(text string) (uint32, error) {
	return d.evalWhole(strings.TrimSpace(text))
}

// optionalNumber is the value of a parameter that may be left out (a
// count, an interrupt number): 0 when it is.
func (d *Debugger) optionalNumber(text string) (uint32, error) {
	if strings.TrimSpace(text) == "" {
		return 0, nil
	}

	return d.evalText(text)
}

// showCalls runs SHOW CALLS [count]: a row per call frame, the VMS
// debugger's table (the console draws it). With no frame to show, as
// after the image has exited, it is %DEBUG-E-NOCALLS.
func (d *Debugger) showCalls(countText string) error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	if d.imageExited {
		return vmserrors.New(vmserrors.DBG_NOCALLS)
	}

	// The console reads the count in hexadecimal, so hand it the value
	// the debugger worked out in its own input radix.
	count, err := d.optionalNumber(countText)
	if err != nil {
		return err
	}

	text := ""
	if count != 0 {
		text = fmt.Sprintf("%X", count)
	}

	err = c.ShowCalls(text, true)
	if errors.Is(err, vmserrors.New(vmserrors.CLI_NOFRAMES)) {
		return vmserrors.New(vmserrors.DBG_NOCALLS)
	}

	return err
}

// fieldAssignment splits one "field=value" clause of SET PSL or SET PTE
// at its first "=" and evaluates the value; a clause with no "=" is
// reported with status bad.
func (d *Debugger) fieldAssignment(clause string, bad uint32) (string, uint32, error) {
	field, value, ok := strings.Cut(clause, "=")
	if !ok {
		return "", 0, vmserrors.New(bad, clause)
	}

	v, err := d.evalText(value)
	if err != nil {
		return "", 0, err
	}

	return strings.TrimSpace(field), v, nil
}

// setPTE runs SET PTE address [TO address] field=value[,field=value...]
// (govax's own): the same changes to the page table entry of the page
// holding the address or, with TO, of every page from the first address
// through the second.
func (d *Dispatcher) setPTE(id int64, r *dcl.Result) error {
	dbg := d.Debugger
	text := strings.TrimSpace(r.String("CHANGES"))

	// The address is an expression that runs up to the first blank
	// outside parentheses; what follows is "TO address" and the changes.
	first, rest, _ := strings.Cut(text, " ")

	addr, err := dbg.evalText(first)
	if err != nil {
		return err
	}

	addrs := []uint32{addr}
	changes := strings.TrimSpace(rest)

	if word, tail, _ := strings.Cut(changes, " "); strings.EqualFold(word, "TO") {
		second, after, _ := strings.Cut(strings.TrimSpace(tail), " ")

		last, err := dbg.evalText(second)
		if err != nil {
			return err
		}

		changes = strings.TrimSpace(after)
		from, to := addr&^0x1FF, last&^0x1FF

		if to < from {
			return vmserrors.New(vmserrors.CLI_BADRANGE)
		}

		addrs = addrs[:0]
		for a := from; a <= to; a += 512 {
			addrs = append(addrs, a)
		}
	}

	if changes == "" {
		return vmserrors.New(vmserrors.CLI_BADSETSYNTAX, changes)
	}

	for _, a := range addrs {
		for _, clause := range splitTop(changes, ',') {
			if clause = strings.TrimSpace(clause); clause == "" {
				continue
			}

			field, v, err := dbg.fieldAssignment(clause, vmserrors.CLI_BADPTEFIELD)
			if err != nil {
				return err
			}

			if err := dbg.Console.SetPTE(a, field, v); err != nil {
				return err
			}
		}
	}

	return nil
}
