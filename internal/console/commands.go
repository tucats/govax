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
