package console

import (
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// bindConsoleCommands binds the console's former fixed commands, which
// docs/PHASE-37.md moved onto the DCL grammar (console.dcl's Phase 37
// block). The grammar divides each command line into its qualifiers and
// parameters; an $expression parameter arrives as text, and its handler
// evaluates it with the console's expression evaluator.
func (d *Dispatcher) bindConsoleCommands() {
	g := d.Grammar

	g.Bind("ZERO", func(id int64, r *dcl.Result) error { return d.Console.Zero() })
	g.Bind("BOOT", notImplemented("BOOT", "device/RTL support"))
	g.Bind("ROM", notImplemented("ROM", "device support"))

	g.Bind("TIME", func(id int64, r *dcl.Result) error {
		return d.Console.Time(r.String("COMMAND"), d.Dispatch)
	})

	g.Bind("PRINT", func(id int64, r *dcl.Result) error {
		return d.Console.Print(r.List("ITEMS"))
	})

	// A "/" starts a new word, as it does in DCL, so HELP SHOW
	// SYMBOL/ALL finds the same "/ALL" topic as HELP SHOW SYMBOL /ALL.
	g.Bind("HELP", func(id int64, r *dcl.Result) error {
		return d.Console.Help(d.Help, strings.Fields(strings.ReplaceAll(r.String("TOPIC"), "/", " /")))
	})

	g.Bind("IF", d.ifCommand)

	g.Bind("STEP", d.stepCommand)
	g.Bind("EXECUTE", func(id int64, r *dcl.Result) error {
		addr, err := d.optionalAddress(r, "ADDRESS")
		if err != nil {
			return err
		}

		return d.Console.Execute(addr)
	})
	g.Bind("CALL", d.callCommand)
	g.Bind("RUN", d.runCommand)

	g.Bind("EXAMINE", d.examineCommand)
	g.Bind("DEPOSIT", d.depositCommand)
	g.Bind("DISASSEMBLE", d.disassembleCommand)

	g.Bind("ASM", d.asmCommand)
	g.Bind("INCLUDE", func(id int64, r *dcl.Result) error {
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
	if !r.Present("FILE") {
		return d.Console.AssembleBegin()
	}

	entryAddr, hasEntry, err := d.Console.Assemble(r.String("FILE"))
	if err != nil {
		return err
	}

	if hasEntry {
		// console.c's own post-command hook: a .END-named entry address
		// auto-invokes "CALL __ENTRY" (no arguments) once the file
		// finishes assembling.
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
		kind = "ROM"
	case r.Present("NVRAM"):
		kind = "NVRAM"
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
	// console_save.c checks `if (!vax_init) return VAX_NOVAX;` before
	// doing anything else -- ROM/NVRAM live on Engine.Memory(), which
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
	// console_load.c's same vax_init check as saveCommand's.
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

// examineSize returns the size qualifier EXAMINE or DEPOSIT was given,
// SizeLongword by default (exam.go).
func examineSize(r *dcl.Result) ExamSize {
	switch {
	case r.Present("BYTE"):
		return SizeByte
	case r.Present("WORD"):
		return SizeWord
	case r.Present("ASCII"):
		return SizeASCII
	case r.Present("PTE"):
		return SizePTE
	default:
		return SizeLongword
	}
}

// isRegisterName reports whether text names a register EXAMINE and
// DEPOSIT handle themselves (console_exam.c's register short-circuit),
// rather than an address expression.
func isRegisterName(text string) bool {
	_, ok := registerNames[strings.ToUpper(text)]

	return ok
}

// examineCommand implements EXAMINE[/size] [start [end]] (console_exam.c):
// one item at start, every item from start to end, or, with no address,
// the item at the current deposit address. start may be a register name.
func (d *Dispatcher) examineCommand(id int64, r *dcl.Result) error {
	sz := examineSize(r)

	if !r.Present("START") {
		return d.Console.Examine("", d.Console.DepositAddr, 1, sz)
	}

	start := r.String("START")
	if isRegisterName(start) && !r.Present("END") {
		return d.Console.Examine(start, 0, 1, sz)
	}

	addr, err := d.evalWhole(start)
	if err != nil {
		return err
	}

	count := uint32(1)

	if r.Present("END") {
		end, err := d.evalWhole(r.String("END"))
		if err != nil {
			return err
		}

		if end < addr {
			return vmserrors.New(vmserrors.CLI_BADRANGE)
		}

		count = (end-addr)/sizeBytes(sz) + 1
	}

	return d.Console.Examine("", addr, count, sz)
}

// depositCommand implements DEPOSIT[/size] target[=]value: target is a
// register name or an address expression.
func (d *Dispatcher) depositCommand(id int64, r *dcl.Result) error {
	sz := examineSize(r)

	val, err := d.evalWhole(r.String("VALUE"))
	if err != nil {
		return err
	}

	target := r.String("TARGET")
	if isRegisterName(target) {
		return d.Console.Deposit(target, 0, sz, val)
	}

	addr, err := d.evalWhole(target)
	if err != nil {
		return err
	}

	return d.Console.Deposit("", addr, sz, val)
}

// disassembleCommand implements DISASSEMBLE [start [end]]
// (console_disasm.c): start defaults to the current deposit address, and
// end to start (one instruction).
func (d *Dispatcher) disassembleCommand(id int64, r *dcl.Result) error {
	start := d.Console.DepositAddr

	if r.Present("START") {
		v, err := d.evalWhole(r.String("START"))
		if err != nil {
			return err
		}

		start = v
	}

	end := start

	if r.Present("END") {
		v, err := d.evalWhole(r.String("END"))
		if err != nil {
			return err
		}

		end = v
	}

	return d.Console.Disassemble(start, end)
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
	v, rest, err := d.Console.Evaluator().Eval(text)
	if err != nil {
		return 0, err
	}

	if extra := strings.TrimSpace(rest); extra != "" {
		return 0, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	return v, nil
}

// stepCommand implements STEP [/INTO|/IN|/INSTRUCTION|/OVER|/RETURN]
// [address] (console_step.c). With no qualifier, the mode is SET STEP's
// (Console.StepMode, docs/PHASE-18.md); the address, when given, is where
// stepping starts.
func (d *Dispatcher) stepCommand(id int64, r *dcl.Result) error {
	mode := d.Console.StepMode

	switch {
	case r.Present("INTO"):
		mode = StepInto
	case r.Present("OVER"):
		mode = StepOver
	case r.Present("RETURN"):
		mode = StepReturn
	}

	addr, err := d.optionalAddress(r, "ADDRESS")
	if err != nil {
		return err
	}

	return d.Console.Step(addr, mode)
}

// callCommand implements CALL [/STEP] routine[(argument[,argument...])]
// (Console.Call; console_call's argument-list syntax). The argument list
// follows the routine directly ("F(1,2)", part of the ROUTINE expression)
// or after a blank ("F (1,2)", the ARGUMENTS parameter).
func (d *Dispatcher) callCommand(id int64, r *dcl.Result) error {
	ev := d.Console.Evaluator()

	addr, list, err := ev.Eval(r.String("ROUTINE"))
	if err != nil {
		return err
	}

	list = strings.TrimSpace(list + " " + r.String("ARGUMENTS"))

	var args []uint32

	if list != "" {
		if !strings.HasPrefix(list, "(") {
			return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, list)
		}

		if args, list, err = d.callArguments(list[1:]); err != nil {
			return err
		}

		// Nothing may follow the argument list.
		if extra := strings.TrimSpace(list); extra != "" {
			return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
		}
	}

	return d.Console.Call(addr, r.Present("STEP"), args...)
}

// callArguments evaluates a CALL argument list, s being what follows its
// "(": expressions separated by commas, up to the closing ")". It returns
// the values and what follows the ")".
func (d *Dispatcher) callArguments(s string) ([]uint32, string, error) {
	var args []uint32

	ev := d.Console.Evaluator()

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

// runOptions applies RUN's qualifiers to defaults, whose RunInits is
// Console.DefaultRunInits (console_run.c's run_inits = vax.debug &
// DBG_LIBINIT), which /INIT or /NOINIT overrides.
func runOptions(r *dcl.Result, defaultRunInits bool) RunOptions {
	opts := RunOptions{RunInits: defaultRunInits}

	if r.Present("INIT") {
		opts.RunInits = !r.Negated("INIT")
	}

	opts.Step = r.Present("STEP")
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

// ifCommand implements IF expression [THEN] command (console_if,
// reference/eVAX/eVAX/Source/Console/console_include.c): when the
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
