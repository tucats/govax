package console

import (
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/vax"
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
	return c.EvalWholeIn(c.Radix, text)
}

// EvalWholeIn is EvalWhole with the radix a number without a prefix is read
// in: the debugger has a radix of its own (SET RADIX).
func (c *Console) EvalWholeIn(radix int, text string) (uint32, error) {
	return c.EvalWholeMode(radix, text, false)
}

// EvalWholeMode is EvalWholeIn with the evaluator's Value mode chosen: true
// gives an expression's value, as EVALUATE shows it (a data label is its
// contents), and false its location, as EXAMINE and DEPOSIT take it (a
// data label is its address).
func (c *Console) EvalWholeMode(radix int, text string, value bool) (uint32, error) {
	ev := c.Evaluator()
	ev.Radix = radix
	ev.Value = value

	v, rest, err := ev.Eval(text)
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

// ImageActive reports whether an image started by RUN is running, from its
// start until it exits (or the machine is reset).
func (c *Console) ImageActive() bool { return c.imageActive }

// StatusText is a status value's message line, such as "%SYSTEM-S-NORMAL,
// normal successful completion".
func (c *Console) StatusText(status uint32) string {
	if c.RTL == nil {
		return ""
	}

	return c.RTL.StatusText(status)
}

// RoutineEntry reports whether addr is the entry point of a routine in a
// loaded image's debug symbol table, one that starts with a two-byte entry
// mask (a CALLS/CALLG routine, as opposed to a JSB subroutine). The
// debugger sets a breakpoint "at routine X" *after* that mask: the mask is
// data (which registers the routine saves), not an instruction, so a stop
// there would be somewhere no instruction starts.
func (c *Console) RoutineEntry(addr uint32) bool {
	prog := c.debugImageAt(addr)
	if prog == nil {
		return false
	}

	r, _, ok := prog.RoutineAt(addr)

	return ok && r.Address == addr && !r.NoCall
}

// RoutineExtent returns the first address and the size in bytes of the
// routine in a loaded image's debug symbol table that holds addr. ok is
// false where addr is in no routine the debugger knows. SET BREAK/RETURN
// uses it to know which RET instructions belong to a routine.
func (c *Console) RoutineExtent(addr uint32) (start, size uint32, ok bool) {
	prog := c.debugImageAt(addr)
	if prog == nil {
		return 0, 0, false
	}

	r, _, found := prog.RoutineAt(addr)
	if !found {
		return 0, 0, false
	}

	return r.Address, r.Size, true
}

// LineStart reports whether pc is the first byte of a source line's code in
// a loaded image's line-number table. SET BREAK/LINE stops at each such
// address: the first instruction of every line the program runs.
func (c *Console) LineStart(pc uint32) bool {
	prog := c.debugImageAt(pc)
	if prog == nil {
		return false
	}

	line, _, ok := prog.LineAt(pc)

	return ok && line.Address == pc
}

// DebugProgramAt returns the debug symbol table of the loaded image that
// holds addr, or nil when addr is in no image or its image has none. The
// debugger names data and registers' scopes from it.
func (c *Console) DebugProgramAt(addr uint32) *dbgsym.Program { return c.debugImageAt(addr) }

// DebugPrograms returns the debug symbol table of each loaded image that
// has one, main image first.
func (c *Console) DebugPrograms() []*dbgsym.Program {
	var out []*dbgsym.Program

	for _, icb := range c.ICBList {
		if icb.Debug != nil {
			out = append(out, icb.Debug)
		}
	}

	return out
}

// ReadBytes reads n bytes of the machine's memory starting at virtual
// address addr, as the console's EXAMINE does: through kernel-mode
// translation, whatever mode the program last ran in. A page that can't
// be read is the debugger's %DEBUG-E-NOACCESSR for the first address that
// failed.
func (c *Console) ReadBytes(addr, n uint32) ([]byte, error) {
	if err := c.requireInit(); err != nil {
		return nil, err
	}

	out := make([]byte, n)

	for i := range out {
		b, err := c.loadSized(addr+uint32(i), SizeByte)
		if err != nil {
			return nil, vmserrors.New(vmserrors.DBG_NOACCESSR, addr+uint32(i))
		}

		out[i] = byte(b)
	}

	return out, nil
}

// WriteBytes stores data into the machine's memory at virtual address
// addr, as DEPOSIT does (through kernel-mode translation, a byte at a
// time). It doesn't move the console's current deposit address.
func (c *Console) WriteBytes(addr uint32, data []byte) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	return c.withKernelMode(func() error {
		for i, b := range data {
			if err := c.Mem.StoreByte(c.CPU, addr+uint32(i), b); err != nil {
				return err
			}
		}

		return nil
	})
}

// FormatPTE is a page table entry as EXAMINE/PTE shows it: the entry's
// value and its fields (valid, protection, modified, owner, and the
// physical address of its page frame).
func (c *Console) FormatPTE(v uint32) string { return c.formatOne(0, SizePTE, v) }

// SetAccessMode switches the CPU to an access mode (kernel, executive,
// supervisor, or user), which also selects that mode's stack pointer; with
// interruptStack it switches to the interrupt stack instead, whatever the
// mode. It is SET MODE's access-mode half, which the debugger took over.
func (c *Console) SetAccessMode(mode vax.AccessMode, interruptStack bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Engine.SetModeStack(mode, interruptStack)

	return nil
}

// AccessModeName is the name of the CPU's current access mode (KERNEL,
// EXEC, SUPER, or USER), or "" before INIT has created the machine.
func (c *Console) AccessModeName() string {
	if c.requireInit() != nil {
		return ""
	}

	return modeNames[c.CPU.PSL().CurMod()]
}
