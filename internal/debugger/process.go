package debugger

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The debugger's view of the processes (docs/PHASE-44.md, "Future work";
// done in the multiprocessing program's close-out, docs/PHASE-48.md).
//
// The VMS debugger's SET PROCESS and SHOW PROCESS are for programs that
// run in several processes. govax's debugger is the machine's, and the
// machine runs every process, so here SET PROCESS chooses which
// process's context the debugger looks at, the "visible" process: the
// CPU moves to it (corevms.System.SwitchCPU, as when a run stops in a
// process other than the console's), and EXAMINE, DEPOSIT, SHOW
// REGISTERS, and the rest then see its registers and its P0 and P1
// space. Running is unchanged: STEP and breakpoints belong to process 1
// (Decision 11 in docs/PHASE-43.md), and GO, STEP, and CALL give the CPU
// back to process 1 first, the visible process keeping its place in the
// scheduler.
//
//	SET PROCESS [/VISIBLE] [pid | name]   (no name: process 1)
//	SHOW PROCESS                          every process, the visible one
//	                                      marked
//
// A pid is hexadecimal (00000302, or 302); anything else is a process
// name in process 1's UIC group, as $GETJPI looks one up. A process that
// isn't there is %SYSTEM-W-NONEXPR. The VMS debugger numbers processes
// and shows each one's state and location; SHOW PROCESS's layout here,
// with the PID, name, state, and PC, is govax's own (unconfirmed).

// bindProcess binds SET PROCESS and SHOW PROCESS.
func (d *Dispatcher) bindProcess() {
	dbg := d.Debugger

	d.Grammar.Bind("SET_PROCESS", func(id int64, r *dcl.Result) error { return dbg.setProcess(r.String("PROCESS")) })
	d.Grammar.Bind("SHOW_PROCESS", func(id int64, r *dcl.Result) error { return dbg.showProcess() })
}

// setProcess is SET PROCESS spec.
func (d *Debugger) setProcess(spec string) error {
	c := d.Console
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NEEDRTL, "SET PROCESS")
	}

	env, err := d.findProcess(strings.TrimSpace(spec))
	if err != nil {
		return err
	}

	return c.ShowProcessContext(env)
}

// findProcess is the process spec names: process 1 for "", a PID in
// hexadecimal, or a name in process 1's group.
func (d *Debugger) findProcess(spec string) (*corevms.Environment, error) {
	sys := d.Console.RTL

	if spec == "" {
		return sys, nil
	}

	if pid, err := strconv.ParseUint(spec, 16, 32); err == nil {
		if env, ok := sys.FindProcess(uint32(pid)); ok {
			return env, nil
		}
	}

	name := strings.ToUpper(strings.Trim(spec, `"`))
	if env, ok := sys.FindProcessName(sys.Process.UIC>>16, name); ok {
		return env, nil
	}

	return nil, vmserrors.New(vmserrors.SS_NONEXPR)
}

// showProcess is SHOW PROCESS: a line for each process, in PID order,
// the visible one (the one the CPU holds) marked with "*":
//
//	  Pid    Process Name    State  PC
//	*00000301 SYSTEM          CUR    00000406
//	 00000302 SYSTEM_1        LEF    000005A2
//
// The PC is the CPU's for the visible process and the one saved in its
// hardware PCB for each of the others.
func (d *Debugger) showProcess() error {
	c := d.Console
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NEEDRTL, "SHOW PROCESS")
	}

	sys := c.RTL.System
	current := sys.Current()

	c.Printf("  Pid    Process Name    State  PC\n")

	for _, env := range sys.Processes() {
		mark := " "
		if env == current {
			mark = "*"
		}

		c.Printf("%s%08X %-15s %-6s %08X\n", mark, env.Process.PID, env.Process.Name, sys.ProcessState(env), c.ProcessPC(env))
	}

	return nil
}
