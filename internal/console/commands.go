package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vmserrors"
)

const (
	romTOKEN   = "ROM"
	nvramTOKEN = "NVRAM"
	zeroTOKEN  = "ZERO"
	bootTOKEN  = "BOOT"
	timeTOKEN  = "TIME"
	printTOKEN = "PRINT"
	helpTOKEN  = "HELP"
	ifTOKEN    = "IF"
	asmTOKEN   = "ASM"
	runTOKEN   = "RUN"
)

// bindConsoleCommands binds the console's former fixed commands, which
// docs/PHASE-37.md moved onto the DCL grammar (console.dcl's Phase 37
// block). The grammar divides each command line into its qualifiers and
// parameters; an $expression parameter arrives as text, and its handler
// evaluates it with the console's expression evaluator.
func (d *Dispatcher) bindConsoleCommands() {
	g := d.Grammar

	g.Bind("STOP", d.Console.stopCommand)

	g.Bind(zeroTOKEN, func(id int64, r *dcl.Result) error { return d.Console.Zero() })
	g.Bind(bootTOKEN, notImplemented(bootTOKEN, "device/RTL support"))
	g.Bind(romTOKEN, notImplemented(romTOKEN, "device support"))

	g.Bind(timeTOKEN, func(id int64, r *dcl.Result) error {
		return d.Console.Time(r.String("COMMAND"), d.Dispatch)
	})

	g.Bind(printTOKEN, func(id int64, r *dcl.Result) error {
		return d.Console.Print(r.List("ITEMS"))
	})

	// A "/" starts a new word, as it does in DCL, so HELP SHOW
	// SYMBOL/ALL finds the same "/ALL" topic as HELP SHOW SYMBOL /ALL.
	g.Bind(helpTOKEN, func(id int64, r *dcl.Result) error {
		return d.Console.Help(d.Help, strings.Fields(strings.ReplaceAll(r.String("TOPIC"), "/", " /")))
	})

	g.Bind(ifTOKEN, d.Console.ifCommand)

	g.Bind(runTOKEN, d.runCommand)
	g.Bind("SPAWN", func(id int64, r *dcl.Result) error {
		return d.Console.Spawn(SpawnOptions{
			Command:        r.String("COMMAND"),
			Input:          r.String("INPUT"),
			Output:         r.String("OUTPUT"),
			Process:        r.String("PROCESS"),
			Prompt:         r.String("PROMPT"),
			NoWait:         r.Negated("WAIT"),
			NoSymbols:      r.Negated("SYMBOLS"),
			NoLogicalNames: r.Negated("LOGICAL_NAMES"),
		})
	})

	g.Bind(asmTOKEN, d.asmCommand)
	g.Bind("SAVE", d.saveCommand)
	g.Bind("LOAD", d.loadCommand)

	d.bindSetCommands()
}

// asmCommand implements ASM: the batch "ASM file" form (Console.Assemble)
// when a file is named, or AssembleBegin's interactive mode
// (docs/PHASE-19.md) for a bare "ASM".
func (d *Dispatcher) asmCommand(id int64, r *dcl.Result) error {
	// Once the microkernel is in place ASM is the debugger's command.
	if d.Console.KernelPlaced() {
		return vmserrors.NewSegment(vmserrors.CLI_IVVERB, "ASM")
	}

	if !r.Present("FILE") {
		return d.Console.AssembleBegin()
	}

	entryAddr, hasEntry, err := d.Console.Assemble(r.String("FILE"))
	if err != nil {
		return err
	}

	if hasEntry {
		// An .END-named entry address auto-invokes "CALL __ENTRY"
		// (no arguments) once the file finishes assembling.
		return d.Console.Call(entryAddr, false)
	}

	return nil
}

// romOrNVRAM returns which of /ROM and /NVRAM a SAVE or LOAD command was
// given, and its file; one of them is required, and so is the file unless
// noError allows a default.
func romOrNVRAM(r *dcl.Result, noError bool) (kind, file string, err error) {
	switch {
	case r.Present("ROM"):
		kind = romTOKEN
	case r.Present("NVRAM"):
		kind = nvramTOKEN
	default:
		return "", "", vmserrors.New(vmserrors.CLI_NEEDROMNVRAM)
	}

	file = r.String("FILE")
	if file == "" && !noError {
		return "", "", vmserrors.New(vmserrors.CLI_NEEDFILENAME, kind)
	}

	return kind, file, nil
}

// saveCommand implements SAVE/ROM file and SAVE/NVRAM file (rom.go). The
// plain .VAX-file SAVE isn't implemented; see rom.go's doc comment.
func (d *Dispatcher) saveCommand(id int64, r *dcl.Result) error {
	// check `if (!vax_init) return VAX_NOVAX;` before doing anything
	// else -- ROM/NVRAM live on Engine.Memory(), which
	// doesn't exist until INIT has allocated a machine.
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	kind, file, err := romOrNVRAM(r, false)
	if err != nil {
		return err
	}

	if kind == "ROM" {
		return d.Console.SaveROM(file)
	}

	return d.Console.SaveNVRAM(file)
}

// loadCommand implements LOAD/ROM and LOAD/NVRAM [/NOERROR] [file].
func (d *Dispatcher) loadCommand(id int64, r *dcl.Result) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	noError := r.Present("ERROR") && r.Negated("ERROR")

	kind, file, err := romOrNVRAM(r, noError)
	if err != nil {
		return err
	}

	if kind == "ROM" {
		return d.Console.LoadROM(file, noError)
	}

	return d.Console.LoadNVRAM(file, noError)
}

// runOptions applies RUN's qualifiers to defaults, whose RunInits is
// Console.DefaultRunInits, which /INIT or /NOINIT overrides.
func runOptions(r *dcl.Result, defaultRunInits bool) machine.RunOptions {
	opts := machine.RunOptions{RunInits: defaultRunInits}

	if r.Present("INIT") {
		opts.RunInits = !r.Negated("INIT")
	}

	if r.Present("DEBUG") {
		opts.Debug = machine.DebugOn

		if r.Negated("DEBUG") {
			opts.Debug = machine.DebugOff
		}
	}

	opts.NoExecute = r.Present("EXECUTE") && r.Negated("EXECUTE")
	opts.Host = r.ParamPresent("FILE", "HOST")

	return opts
}

// runCommand implements RUN (Console.Run): activates a VMS image.
func (d *Dispatcher) runCommand(id int64, r *dcl.Result) error {
	opts := runOptions(r, d.Console.DefaultRunInits())

	// The one-shot command's RUN gives its image the rest of govax's
	// command line (RunCommandLine).
	opts.CommandLine, d.Console.runCommandLine = d.Console.runCommandLine, ""

	return d.Console.Run(r.String("FILE"), opts)
}

// notImplemented is the handler of a verb the console doesn't implement
// yet: it reports what the verb is waiting on.
func notImplemented(name, dependency string) dcl.Handler {
	return func(id int64, r *dcl.Result) error {
		return vmserrors.New(vmserrors.CLI_NEEDDEP, name, dependency)
	}
}

// ShowRegions prints the P0/P1/S0 image-activation region limits.
func (c *Console) ShowRegions() error {
	if err := c.RequireInit(); err != nil {
		return err
	}

	return c.ShowRegionsOf(c.RTL)
}

// StartDebugger is the console's DEBUG command: it starts a debugger
// session on the machine as it stands. From here until the debugger's
// EXIT, command lines go to the debugger's grammar, not the console's
// (Dispatcher.Dispatch routes them).
func (c *Console) StartDebugger() error {
	if err := c.processOneOnly("DEBUG"); err != nil {
		return err
	}

	if c.Debugger == nil {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	if err := c.RequireInit(); err != nil {
		return err
	}

	// Already in a session (DEBUG from a command file the debugger is
	// reading): nothing to start.
	if c.Debugger.Active() {
		return nil
	}

	return c.Debugger.Start(machine.Activation{Kind: machine.ActivateAttach})
}

// StopProcess is STOP: it deletes a process and its subprocesses at once,
// as the operator with every privilege would. The process is named, as
// for SHOW PROCESS, by name (in process 1's UIC group) or by /IDENTIFICATION
// (hexadecimal; the PID wins if both are given). A process that isn't
// there is %SYSTEM-W-NONEXPR. Process 1 is the console's own, so STOP
// of it is refused (%SYSTEM-F-NOPRIV): govax has no logging out, and
// EXIT ends the session. (STOP with neither a name nor an
// identification ends command procedures instead: stopCommand.)
//
// The deleted process's files are closed, and its termination message
// goes to its termination mailbox if it had one, with the final status
// SS$_ABORT. If the CPU was in the process (a run stopped there), it is
// returned to process 1.
func (c *Interpreter) StopProcess(name, pid string) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	env := c.RTL

	switch {
	case pid != "":
		v, err := strconv.ParseUint(strings.TrimSpace(pid), 16, 32)
		if err != nil {
			return vmserrors.New(vmserrors.SS_IVIDENT)
		}

		var found bool
		if env, found = c.RTL.FindProcess(uint32(v)); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}

	case name != "":
		var found bool
		if env, found = c.RTL.FindProcessName(c.RTL.Process.UICGroup(), name); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}
	}

	if env == c.RTL {
		return vmserrors.New(vmserrors.SS_NOPRIV)
	}

	c.RTL.DeleteNow(env)

	return c.machine.ReturnToProcessOne()
}

// The console is internal/debugger's Console: the machine it debugs, and
// the console's command line.

// DebugMachine is the machine the debugger works on.
func (c *Console) DebugMachine() *machine.Machine { return c.Machine }

// ConsoleDispatch is the console's Dispatcher.Dispatch, which routes a
// line to the debugger's grammar or the console's; nil before there is a
// Dispatcher.
func (c *Console) ConsoleDispatch() func(line string) error {
	if c.Dispatcher == nil {
		return nil
	}

	return c.Dispatcher.Dispatch
}

// IsConsoleVerb reports whether word is a verb of the console's grammar.
func (c *Console) IsConsoleVerb(word string) bool {
	return c.Dispatcher != nil && c.Dispatcher.Grammar.HasVerb(word)
}

// Run is RUN (Interpreter.Run): in a subprocess's DCL it runs the image
// in that process, in process 1's on the machine.
func (c *Console) Run(fn string, opts machine.RunOptions) error {
	return c.Interpreter.Run(fn, opts)
}
