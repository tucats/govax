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
