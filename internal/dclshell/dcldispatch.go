package dclshell

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// DispatchConsole parses and executes one console command line -- DCL
// symbols and the console grammar -- whatever mode the front end is in.
// Dispatch calls it when no debugger session is active; XFC$CONSOLE_CMD
// (Console.ConsoleCommand) calls it always.
func (c *Interpreter) DispatchConsole(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	// Interactive assembler mode, as in Dispatch, for the callers that
	// come straight here.
	if c.machine.InAssemblerMode() {
		return c.machine.AssembleLine(line)
	}

	// The lines of an IF block's branch that doesn't run are passed
	// over, unread (dclif.go).
	// One that ends the passing over (its ENDIF, or the ELSE where the
	// branch that runs starts) is shown with verification on; the rest
	// aren't (dclverify.go).
	skipping, whole := c.flow().skip.active, line

	line, run := c.skipLine(line)
	if !run {
		if skipping && !c.flow().skip.active {
			c.echoLine("", whole, whole)
		} else {
			c.dropEcho()
		}

		return nil
	}

	// The line is one command, whose status becomes $STATUS
	// (dclstatus.go). An image it ran may have left a command for DCL to
	// run next (dclchain.go), a line of its own.
	err := c.statusOf(func() error {
		// A label marks the line (dcllabel.go), and the command after a
		// block's IF must be THEN (dclif.go).
		whole := line

		line, err := c.takeLabel(line)
		if err != nil || line == "" {
			c.echoLine("", whole, whole)

			return err
		}

		if c.awaitThen(line) {
			if c.passedBeforeThen() {
				c.echoLine("", whole, whole)
			} else {
				c.dropEcho()
			}

			c.keepStatus()

			return nil
		}

		// The THEN of a block whose IF was false isn't shown (VMS 7.3).
		quiet := c.thenOfFalseIf(line)

		// DCL's first phase: symbols between apostrophes are replaced
		// before anything reads the line (dclsubst.go). A procedure's
		// line is shown then, with procedure verification on
		// (dclverify.go).
		// A line whose scan fails isn't shown: its message is (VMS 7.3,
		// testdata/dcl50).
		// A continued command is scanned with its records' joins
		// marked, so it can be shown a record at a time.
		label := ""
		if strings.HasSuffix(whole, line) {
			label = whole[:len(whole)-len(line)]
		}

		marked := c.markJoins(whole, line)

		scanned, err := c.substituteApostrophes(marked)
		if err != nil {
			c.dropEcho()

			return err
		}

		if quiet {
			c.dropEcho()
		} else {
			c.echoLine(label, marked, scanned)
		}

		line = strings.ReplaceAll(scanned, joinMark, " ")

		return c.dispatchCommand(line)
	})

	return c.runChained(err)
}

// dispatchCommand executes a console command line that has been through
// apostrophe substitution: DCL's second phase (the first word as a
// symbol, then ampersand substitution) and then the console's grammar.
// IF's THEN command comes back here, so its apostrophes aren't
// substituted again.
func (c *Interpreter) dispatchCommand(line string) error {
	return c.dispatchParsed(line, true)
}

// dispatchParsed is dispatchCommand, with verbSymbol saying whether the
// first word may be a symbol: not in an alias's value, which DCL doesn't
// look up again (VMS 7.3 gave IVVERB for an alias of an alias,
// testdata/dcl50).
func (c *Interpreter) dispatchParsed(line string, verbSymbol bool) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	// "@file" runs a command procedure (procedure.go). DCL reads it
	// before any verb: its /OUTPUT qualifier must touch the file name,
	// and anything after a blank is a parameter, which the grammar's
	// qualifiers don't model.
	if text, ok := strings.CutPrefix(line, "@"); ok {
		if err := c.leaveControlY(false); err != nil {
			return err
		}

		return c.atCommand(text)
	}

	// A command starts with a letter (or "$" or "_", which can start a
	// symbol's name). "%" and a digit start one too, as far as DCL is
	// concerned: VMS 7.3 gave IVVERB, \%%\ and \1ABC\, for lines "%%"
	// and "1ABC", where a line starting with an apostrophe, ".", or "("
	// was NOCOMD (testdata/dcl50's rounds 3, 1, and 6).
	if !isDCLSymbolStart(line[0]) && line[0] != '%' && !isDecimalDigit(line[0]) {
		return vmserrors.New(vmserrors.CLI_NOCOMD)
	}

	// A symbol assignment, DELETE/SYMBOL, or a command whose first word
	// is a DCL symbol (a foreign command or an alias): DCL looks for a
	// symbol before a verb (dclsym.go).
	if handled, err := c.dclSymbolLine(line, verbSymbol); handled {
		return err
	}

	line = c.dclSymbols.substituteAmpersands(line, true)

	// A "!" outside quotes starts a comment, which runs to the end of the
	// line ("SHOW SYMBOL X ! why"); the grammar never sees it. (An
	// &SYMBOL's value is a quoted token by now, so a "!" in it stays.)
	line = strings.TrimRight(uncomment(line), " \t")
	if line == "" {
		return nil
	}

	if c.machine.CPU != nil && c.machine.CPU.DebugEnabled(registers.DebugDCL) {
		fmt.Fprintf(c.machine.CPU.DebugWriter(), "DEBUG(DCL): parsing %q\n", line)
	}

	r, err := c.parseCommand(line)
	if errors.Is(err, dcl.ErrCanceled) {
		c.SetCommandStatus(ssNormal|stsInhibitMsg, nil)

		return nil
	}

	if err != nil {
		return verbError(err)
	}

	// A subprocess's DCL doesn't work the machine itself (subcli.go).
	if processOneCommands[r.Active] {
		if err := c.ProcessOneOnly(strings.ToUpper(r.Verb)); err != nil {
			return err
		}
	}

	// At the Ctrl/Y level, a command that runs an image ends the
	// interrupted procedure first (dclcontroly.go).
	if err := c.leaveControlY(dclOwnCommands[r.Active]); err != nil {
		return err
	}

	c.line = line

	// Whether the command ran an image, as VMS would run one for it,
	// decides what a procedure does with the data lines after it
	// (dclinput.go). It is known when the command has run, after any
	// procedure or command it ran in turn.
	if !dclOwnCommands[r.Active] {
		defer func() { c.imageRan = true }()
	}

	return c.Grammar.Dispatch(r)
}

// parseCommand parses a command line. One typed at the terminal asks
// there for a required parameter it doesn't give ("$ SHOW" asks
// "_What: "), and the reply completes the line; Ctrl/Z at the prompt
// drops the command (dcl.ErrCanceled) with $STATUS SS$_NORMAL, marked as
// shown (%X10000001, VMS's console). A command read from a command
// procedure isn't prompted for: its missing parameter is an error, as
// on VMS. Nor is one with no terminal input to read (a test's console).
func (c *Interpreter) parseCommand(line string) (*dcl.Result, error) {
	if !c.readingTerminal() || (c.subCLI == nil && c.In == nil) {
		return c.Grammar.Parse(line)
	}

	return c.Grammar.ParsePrompting(line, c.readTerminal)
}

// readingTerminal says whether the command being run was read from the
// terminal: at command level 0, or typed at the Ctrl/Y level (which
// leaves the interrupted procedure's levels in place), not in a
// procedure run from it.
func (c *Interpreter) readingTerminal() bool {
	if prompt := c.controlYLevelActive; prompt != nil {
		return len(c.levels) == prompt.depth
	}

	return c.currentLevel() == nil
}

// verbError is DCL's message for a command whose verb the grammar
// doesn't know (CLI_IVVERB) or can't choose (CLI_ABVERB), with the verb
// as its segment, as VMS shows them; any other error is returned as it
// is.
func verbError(err error) error {
	var ve vmserrors.VMSError
	if !errors.As(err, &ve) || len(ve.Arguments) != 2 || ve.Arguments[0] != "verb" {
		return err
	}

	verb, _ := ve.Arguments[1].(string)

	switch ve.Status {
	case vmserrors.CLI_UNRECOGNIZED:
		return vmserrors.NewSegment(vmserrors.CLI_IVVERB, verb)
	case vmserrors.CLI_AMBIGUOUS:
		return vmserrors.NewSegment(vmserrors.CLI_ABVERB, verb)
	}

	return err
}

// readCommandVerb reads the leading command word (up to whitespace, '/',
// ',', or '='), matching read_verb — including its "a leading '@' is a
// token by itself" special case (a command procedure). DCL symbol
// lookup (dclsym.go) uses it, and IF and SET PTE use it for their THEN
// and TO words.
func readCommandVerb(s string) (verb, rest string) {
	if strings.HasPrefix(s, "@") {
		return "@", s[1:]
	}

	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '/', ',', '=':
			return s[:i], s[i:]
		}

		i++
	}

	return s, ""
}

// BindDCL binds DCL's own verbs in g, the console's grammar: the flow of
// a procedure (EXIT, ON, GOTO, IF/THEN/ELSE/ENDIF, GOSUB/RETURN,
// CALL/SUBROUTINE), DECK/EOD, DCL's file I/O, WAIT, CONTINUE, QUIT,
// LOGOUT, STOP, SPAWN, MCR, SHOW SYMBOL, and DCL's SET commands (VERIFY,
// PREFIX, CONTROL, ON, PROMPT). The console binds the rest.
func (c *Interpreter) BindDCL(g *dcl.Grammar) {
	// EXIT ends the command procedure it's in, or govax at the terminal;
	// QUIT ends govax wherever it is (procedure.go).
	g.Bind("EXIT", c.exitCommand)
	g.Bind("ON", c.onCommand)
	// Labels, IF blocks, GOSUB, and CALL (dcllabel.go, dclif.go,
	// dclcall.go).
	g.Bind("GOTO", c.gotoCommand)
	g.Bind("THEN", c.thenCommand)
	g.Bind("ELSE", c.elseCommand)
	g.Bind("ENDIF", c.endifCommand)
	g.Bind("GOSUB", c.gosubCommand)
	g.Bind("RETURN", c.returnCommand)
	g.Bind("CALL", c.callCommand)
	g.Bind("SUBROUTINE", c.subroutineCommand)
	g.Bind("ENDSUBROUTINE", c.endsubroutineCommand)
	// DECK and EOD: data lines for an image, "$" or not (dclinput.go).
	g.Bind("DECK", c.deckCommand)
	g.Bind("EOD", c.eodCommand)
	// DCL's file I/O (dclfile.go).
	g.Bind("OPEN", c.openCommand)
	g.Bind("CLOSE", c.closeCommand)
	g.Bind("READ", c.readCommand)
	g.Bind("WRITE", c.writeCommand)
	g.Bind("INQUIRE", c.inquireCommand)
	g.Bind("WAIT", c.waitCommand)
	g.Bind("CONTINUE", func(id int64, r *dcl.Result) error { return c.continueCommand() })
	g.Bind("QUIT", func(id int64, r *dcl.Result) error { return c.Quit() })
	g.Bind("LOGOUT", func(id int64, r *dcl.Result) error { return c.logoutCommand() })
	g.Bind("MCR", func(id int64, r *dcl.Result) error {
		return c.runForeign(r.String("IMAGE"), r.String("TEXT"))
	})
	g.Bind("SHOW_SYM", func(id int64, r *dcl.Result) error {
		cmd := symbolCommand{all: r.Present("ALL"), name: strings.ToUpper(strings.TrimSpace(r.String("SYMBOL")))}

		switch {
		case r.Present("LOCAL"):
			cmd.scope = scopeLocal
		case r.Present("GLOBAL"):
			cmd.scope = scopeGlobal
		}

		return c.showDCLSymbols(cmd)
	})
	g.Bind("STOP", c.stopCommand)
	g.Bind("IF", c.ifCommand)
	g.Bind("SPAWN", func(id int64, r *dcl.Result) error {
		return c.Spawn(SpawnOptions{
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
	g.Bind("ATTACH", func(id int64, r *dcl.Result) error {
		return c.Attach(r.String("PROCESS"), r.String("IDENTIFICATION"))
	})
	g.Bind("SET_VERIFY", func(id int64, r *dcl.Result) error { return setVerifyCommand(c, r, c.line) })
	g.Bind("SET_PREFIX", func(id int64, r *dcl.Result) error {
		if r.Negated("WHAT") {
			return c.SetPrefix("")
		}

		return c.SetPrefix(r.String("TEXT"))
	})
	g.Bind("SET_CONTROL", func(id int64, r *dcl.Result) error { return c.setControlCommand(r) })
	g.Bind("SET_ON", func(id int64, r *dcl.Result) error { return c.SetOn(!r.Negated("WHAT")) })
	// SET PROMPT="text" changes the console's prompt.
	g.Bind("SET_PROMPT", func(id int64, r *dcl.Result) error { return c.SetPrompt(r.String("TEXT")) })
}

// isDecimalDigit reports whether ch is a decimal digit.
func isDecimalDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
