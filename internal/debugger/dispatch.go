package debugger

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// Dispatcher parses one debugger command line with the debugger's DCL
// grammar (debug.dcl) and runs the handler bound to the verb it names.
// It is to the debugger what console.Dispatcher is to the console: the
// same grammar interpreter (internal/console/dcl), a different grammar.
type Dispatcher struct {
	Debugger *Debugger
	Grammar  *dcl.Grammar
	Help     *console.Help
}

// newDispatcher returns a Dispatcher for d with every command this
// subtask implements bound to its handler.
func newDispatcher(d *Debugger, g *dcl.Grammar, help *console.Help) *Dispatcher {
	disp := &Dispatcher{Debugger: d, Grammar: g, Help: help}
	disp.bind()

	return disp
}

// bind binds each verb in debug.dcl to its handler. The names are the
// grammar's, so a verb in debug.dcl with no handler here reports "no
// handler bound" when used, rather than doing nothing.
func (d *Dispatcher) bind() {
	g := d.Grammar

	// EXIT and QUIT end the session. They differ on VMS only in whether
	// the program's exit handlers run, and govax has none to run yet.
	end := func(id int64, r *dcl.Result) error {
		d.Debugger.End()

		return nil
	}

	g.Bind("EXIT", end)
	g.Bind("QUIT", end)

	// A "/" starts a new word, as it does in DCL, so HELP SET BREAK
	// /AFTER finds the same topic as HELP SET BREAK/AFTER.
	g.Bind("HELP", func(id int64, r *dcl.Result) error {
		words := strings.Fields(strings.ReplaceAll(r.String("TOPIC"), "/", " /"))

		return d.Debugger.Console.Help(d.Help, words)
	})

	// @file reads debugger commands from a file. Each line goes through
	// the console dispatcher's routing, not straight back here: if one of
	// them ends the session (EXIT), the lines after it are console
	// commands, as they are when a script is fed to the console.
	g.Bind("INCLUDE", func(id int64, r *dcl.Result) error {
		c := d.Debugger.Console

		dispatch := d.Dispatch
		if c.Dispatcher != nil {
			dispatch = c.Dispatcher.Dispatch
		}

		return c.Include(r.String("FILE"), dispatch)
	})
}

// Dispatch parses and runs one command line. Blank lines and comments
// (a line starting with "!") do nothing.
func (d *Dispatcher) Dispatch(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return nil
	}

	r, err := d.Grammar.Parse(line)
	if err != nil {
		return d.parseError(line, err)
	}

	return d.Grammar.Dispatch(r)
}

// parseError turns a failure to parse a command into what the VMS
// debugger says: %DEBUG-E-SYNTAX, naming the first word it couldn't take.
// The probe (testdata/dbgcmd/vax/errors.dlg) shows that for an unknown
// verb, keyword, or qualifier alike.
//
// If that word is a console command (DIRECTORY, MACRO, ...), the
// debugger doesn't have it, but the console does, so govax adds a hint
// that EXIT returns there. VMS prints no such hint. The two lines are
// printed here and the error is returned marked as already displayed,
// so the front end doesn't print the first one again.
//
// An error that isn't a bad word (a missing parameter, say) is returned
// unchanged, for now; the subtasks that add commands choose their
// messages.
func (d *Dispatcher) parseError(line string, err error) error {
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_UNRECOGNIZED)) &&
		!errors.Is(err, vmserrors.New(vmserrors.CLI_AMBIGUOUS)) {
		return err
	}

	// The grammar's error carries the word as its second argument (the
	// first says whether it was a verb or a qualifier). It is the
	// uppercased text, as the VMS debugger shows its word.
	word := strings.ToUpper(firstWord(line))
	isVerb := true

	var ve vmserrors.VMSError
	if errors.As(err, &ve) && len(ve.Arguments) >= 2 {
		isVerb = fmt.Sprint(ve.Arguments[0]) == "verb"
		word = qualifierAsTyped(line, fmt.Sprint(ve.Arguments[1]), isVerb)
	}

	syntax := vmserrors.New(vmserrors.DBG_SYNTAX, word)
	c := d.Debugger.Console

	// Only an unknown *verb* can be a console command.
	if isVerb && c.Dispatcher != nil && c.Dispatcher.Grammar.HasVerb(firstWord(line)) {
		c.Printf("%%%s\n", syntax)
		c.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_CONSOLECOMMAND, strings.ToUpper(firstWord(line))))

		return vmserrors.InhibitMessage(syntax)
	}

	return syntax
}

// qualifierAsTyped returns the word as the user typed it. The grammar
// reports a qualifier without its "NO" prefix, because it tries a
// qualifier both ways (/NOSUCH is /SUCH negated), but the VMS debugger
// names the word whole: '/NOSUCHQUAL' is reported as NOSUCHQUAL. A verb
// is reported as the grammar has it.
func qualifierAsTyped(line, word string, isVerb bool) string {
	if isVerb {
		return word
	}

	upper := strings.ToUpper(line)

	// Is "/WORD" in the line, as a whole word? If not, it was typed
	// with a NO in front.
	for i := 0; ; {
		at := strings.Index(upper[i:], "/"+word)
		if at < 0 {
			break
		}

		end := i + at + 1 + len(word)
		if end == len(upper) || !isWordByte(upper[end]) {
			return word
		}

		i = end
	}

	return "NO" + word
}

// isWordByte reports whether b can be part of a qualifier's name.
func isWordByte(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '$'
}
