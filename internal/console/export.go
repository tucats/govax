package console

import (
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the surface the debugger (internal/debugger) reaches the
// console through. The debugger runs the program, but the console still
// owns what makes the machine a VMS one: image activation and rundown, the
// condition handling facility (CHF), the symbol tables and the loaded
// images' debug symbols, and the machine's lifetime. Like
// internal/coreos's export.go, the exported names the debugger needs and
// that have no other reason to be public are gathered here, so the
// boundary is visible in one place. (The engine, CPU, memory, output
// stream, and expression evaluator are exported fields and methods of
// Console already.)

// RequireInit reports the error a command gets when the machine hasn't
// been created yet (before INIT).
func (c *Console) RequireInit() error { return c.requireInit() }

// LocationText is pc as STEP's "Stepped to" and a breakpoint's "Break at"
// show it: a symbolic name where the program has one, else the address
// in 8 hex digits.
func (c *Console) LocationText(pc uint32) string { return c.locationText(pc) }

// TraceStep prints the instruction about to execute at pc when tracing is
// on (SET TRACE), or when force is true, and returns a function to call
// once the instruction has run, to print what it changed. See traceStep.
func (c *Console) TraceStep(pc uint32, force bool) func() { return c.traceStep(pc, force) }

// ExceptionName is the name of an exception (fault, trap, or interrupt)
// vector's code, as the console words it.
func (c *Console) ExceptionName(code cpu.Exception) string { return exceptionName(code) }

// EvalWhole evaluates text, which must be one whole expression: anything
// left over after the expression is an error. It is how a command's
// address or value parameter becomes a number.
func (c *Console) EvalWhole(text string) (uint32, error) {
	v, rest, err := c.Evaluator().Eval(text)
	if err != nil {
		return 0, err
	}

	if extra := strings.TrimSpace(rest); extra != "" {
		return 0, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	return v, nil
}

// ParseCall evaluates the parameters of a CALL command: the routine's
// address, and the arguments to pass it. In CALL's syntax the argument
// list "(a,b,...)" follows the routine either directly ("F(1,2)", part of
// the routine expression, which the evaluator leaves unconsumed in its
// remainder) or after a blank ("F (1,2)", which the grammar delivers as a
// separate parameter, arguments).
func (c *Console) ParseCall(routine, arguments string) (addr uint32, args []uint32, err error) {
	addr, list, err := c.Evaluator().Eval(routine)
	if err != nil {
		return 0, nil, err
	}

	list = strings.TrimSpace(list + " " + arguments)
	if list == "" {
		return addr, nil, nil
	}

	if !strings.HasPrefix(list, "(") {
		return 0, nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, list)
	}

	args, list, err = c.callArguments(list[1:])
	if err != nil {
		return 0, nil, err
	}

	// Nothing may follow the argument list.
	if extra := strings.TrimSpace(list); extra != "" {
		return 0, nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	return addr, args, nil
}

// callArguments evaluates a CALL argument list, s being what follows its
// "(": expressions separated by commas, up to the closing ")". It returns
// the values and what follows the ")".
func (c *Console) callArguments(s string) ([]uint32, string, error) {
	var args []uint32

	ev := c.Evaluator()

	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, ")") {
			return args, s[1:], nil
		}

		if s == "" {
			return nil, "", vmserrors.New(vmserrors.CLI_INCOMPLETEARGS)
		}

		if len(args) > 0 {
			if !strings.HasPrefix(s, ",") {
				return nil, "", vmserrors.New(vmserrors.CLI_NEEDCOMMA)
			}

			s = s[1:]
		}

		v, rest, err := ev.Eval(s)
		if err != nil {
			return nil, "", err
		}

		args = append(args, v)
		s = rest
	}
}
