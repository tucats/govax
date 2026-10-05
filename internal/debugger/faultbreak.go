package debugger

import "github.com/tucats/govax/internal/cpu"

// This file implements the console-facing surface of fault-kind breakpoints
// (SET BREAKPOINT/FAULT, CLEAR BREAKPOINT/FAULT[/ALL]) — the state and
// delivery-time check itself live on cpu.Engine (see cpu/faultbreak.go's
// own doc comment for why), so this is thin wiring plus the merged display
// ShowBreakpoints needs (console_show.c's own SHOW BREAK prints address and
// fault breakpoints in one unified list, distinguished by an 'F' marker —
// see that function's own doc comment).

// AddFaultBreakpoint implements SET BREAKPOINT/FAULT <code>
// (console_set.c:727-729's BREAK_FAULT case): code is evaluated as an
// expression (this port's own Evaluator, used uniformly for every other
// address-shaped SET BREAKPOINT argument, rather than porting asm_value's
// own mixed radix/no-forward-reference conventions exactly — a console-
// command-scope convenience, not an ISA-fidelity question). Setting an
// already-armed code is a no-op, matching cpu.Engine.SetFaultBreakpoint.
func (d *Debugger) AddFaultBreakpoint(codeExpr string) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	code, _, err := d.Console.Evaluator().Eval(codeExpr)
	if err != nil {
		return err
	}

	d.Console.Engine.SetFaultBreakpoint(cpu.Exception(code))

	return nil
}

// RemoveFaultBreakpoint implements CLEAR BREAKPOINT/FAULT <code>, matching
// console_clear.c's case 108.
func (d *Debugger) RemoveFaultBreakpoint(codeExpr string) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	code, _, err := d.Console.Evaluator().Eval(codeExpr)
	if err != nil {
		return err
	}

	d.Console.Engine.RemoveFaultBreakpoint(cpu.Exception(code))

	return nil
}

// ClearAllFaultBreakpoints implements CLEAR BREAKPOINT/FAULT/ALL, matching
// console_clear.c's case 109.
func (d *Debugger) ClearAllFaultBreakpoints() error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	d.Console.Engine.ClearFaultBreakpoints()

	return nil
}
