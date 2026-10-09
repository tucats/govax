package console

import (
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

const (
	romTOKEN     = "ROM"
	nvramTOKEN   = "NVRAM"
	zeroTOKEN    = "ZERO"
	bootTOKEN    = "BOOT"
	timeTOKEN    = "TIME"
	printTOKEN   = "PRINT"
	helpTOKEN    = "HELP"
	ifTOKEN      = "IF"
	asmTOKEN     = "ASM"
	includeTOKEN = "INCLUDE"
	runTOKEN     = "RUN"
)

// bindConsoleCommands binds the console's former fixed commands, which
// docs/PHASE-37.md moved onto the DCL grammar (console.dcl's Phase 37
// block). The grammar divides each command line into its qualifiers and
// parameters; an $expression parameter arrives as text, and its handler
// evaluates it with the console's expression evaluator.
func (d *Dispatcher) bindConsoleCommands() {
	g := d.Grammar

	g.Bind("STOP", func(id int64, r *dcl.Result) error {
		return d.Console.StopProcess(r.String("PROCESS_NAME"), r.String("IDENTIFICATION"))
	})

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

	g.Bind(ifTOKEN, d.ifCommand)

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
	g.Bind(includeTOKEN, func(id int64, r *dcl.Result) error {
		return d.Console.Include(r.String("FILE"), d.Dispatch)
	})
	g.Bind("INCLUDE_COMMAND_LINE", func(id int64, r *dcl.Result) error {
		return d.Console.IncludeCommandLine(d.Dispatch)
	})
	g.Bind("SAVE", d.saveCommand)
	g.Bind("LOAD", d.loadCommand)

	d.bindSetCommands()
}

// asmCommand implements ASM: the batch "ASM file" form (Console.Assemble)
// when a file is named, or AssembleBegin's interactive mode
// (docs/PHASE-19.md) for a bare "ASM".
func (d *Dispatcher) asmCommand(id int64, r *dcl.Result) error {
	// Once the microkernel is in place ASM is the debugger's command.
	if d.Console.kernelPlaced {
		return vmserrors.New(vmserrors.CLI_UNRECOGNIZED, "verb", "ASM")
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
	if err := d.Console.requireInit(); err != nil {
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
	if err := d.Console.requireInit(); err != nil {
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

// symbolicSetting is the setting that gives DISASSEMBLE's default for
// /SYMBOLIC.
const symbolicSetting = "vax.disassemble.symbolic"

// symbolicDefault is whether DISASSEMBLE is /SYMBOLIC when the command
// doesn't say: the vax.disassemble.symbolic setting, or true when it
// isn't set, as the debugger's SET MODE SYMBOLIC is its default.
func symbolicDefault() bool {
	if settings.Get(symbolicSetting) == "" {
		return true
	}

	return settings.GetBool(symbolicSetting)
}

// optionalAddress evaluates the $expression parameter name, or returns nil
// if the command line didn't give it.
func (d *Dispatcher) optionalAddress(r *dcl.Result, name string) (*uint32, error) {
	if !r.Present(name) {
		return nil, nil
	}

	v, err := d.evalWhole(r.String(name))
	if err != nil {
		return nil, err
	}

	return &v, nil
}

// evalWhole evaluates one $expression parameter's text, which must be
// one whole expression.
func (d *Dispatcher) evalWhole(text string) (uint32, error) {
	return d.Console.EvalWhole(text)
}

// runOptions applies RUN's qualifiers to defaults, whose RunInits is
// Console.DefaultRunInits, which /INIT or /NOINIT overrides.
func runOptions(r *dcl.Result, defaultRunInits bool) RunOptions {
	opts := RunOptions{RunInits: defaultRunInits}

	if r.Present("INIT") {
		opts.RunInits = !r.Negated("INIT")
	}

	if r.Present("DEBUG") {
		opts.Debug = DebugOn

		if r.Negated("DEBUG") {
			opts.Debug = DebugOff
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

// ifCommand implements IF expression [THEN] command: when the
// expression is nonzero, the command is dispatched as a command line of
// its own (so it can be any console command); otherwise it isn't run.
// vax.init uses IF DEFINED("CONSOLE$ARG_FILE") THEN SET NOVERBOSE (see
// expr.go's DEFINED()). THEN is optional, as console_if allows.
func (d *Dispatcher) ifCommand(id int64, r *dcl.Result) error {
	v, rest, err := d.Console.Evaluator().Eval(r.String("CONDITION"))
	if err != nil {
		return err
	}

	if extra := strings.TrimSpace(rest); extra != "" {
		return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	command := r.String("COMMAND")
	if then, tail := readCommandVerb(command); strings.EqualFold(then, "THEN") {
		command = strings.TrimSpace(tail)
	}

	if v == 0 {
		return nil
	}

	return d.Dispatch(command)
}

// SymbolicDefault is whether the debugger's instruction display names
// addresses from the debug symbols when nothing says otherwise: the
// vax.disassemble.symbolic setting, true when it isn't set. It seeds the
// debugger's SET MODE SYMBOLIC.
func SymbolicDefault() bool { return symbolicDefault() }
