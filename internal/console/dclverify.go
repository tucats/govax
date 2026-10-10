package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Verification (docs/PHASE-50 - DCL command procedures.md, subtask 15):
// SET [NO]VERIFY, SET [NO]PREFIX, and the echoing of a command
// procedure's lines, as the DCL Dictionary (SET VERIFY, SET PREFIX,
// F$VERIFY) and the User's Manual (13.5.5) describe them.
//
// There are two settings. Procedure verification (Console.Verify) shows
// each command line of a procedure as DCL reads it: after the first
// phase of symbol substitution, so 'NAME' is shown replaced and &NAME
// and the symbols of an expression are not (the Dictionary's SET VERIFY
// example 5). Image verification (Console.verifyImage) shows each data
// line given to what reads the procedure's input (the debugger, the
// interactive assembler). Both are off at first, and a change lasts:
// it isn't undone when the procedure that made it ends.
//
// A line is shown as it is in the file: its "$", its indentation, its
// label, and its comment. A line whose scan fails isn't shown, only the
// message (VMS 7.3's log of testdata/dcl50, which TestProbe50Verify
// replays line for line). Comment lines are shown too, and a command
// continued over several records is shown a record at a time (the User's
// Manual's 13.5.5.1 example), unless substitution changed it, when it is
// shown as one line (unconfirmed against VMS: DCL may keep the records
// even then). A CALL's ENDSUBROUTINE is shown as the command that ends
// the subroutine. The lines of an IF block's branch that doesn't run
// aren't shown, except the ENDIF or ELSE that ends it (both unconfirmed).
// Never shown are the terminal's commands, an ON action, and a debugger
// command procedure's lines (the debugger has its own SET OUTPUT
// VERIFY).
//
// DCL does one thing in a comment: it performs F$VERIFY between
// apostrophes, 'F$VERIFY(0)' (the Dictionary's F$VERIFY). As that happens
// in the first phase, before the line is shown, "$ ! 'F$VERIFY(0)'" turns
// verification off without being shown itself, the usual way for a
// procedure to keep its lines out of sight.
//
// SET PREFIX gives a $FAO control string whose result goes in front of
// each verified command line, the time with "(!5%T) ", and blanks as long
// as it in front of the records that continue one. A directive that
// takes an argument gets 0. Data lines get no prefix.

// maxPrefixLength is the longest prefix control string, and the longest
// prefix it may make (the Dictionary's SET PREFIX).
const maxPrefixLength = 64

// pendingEcho is the line a command procedure is about to run, for
// verification: its records, as they are in the file. The procedure loop
// sets it (Console.echo) before dispatching the line, and the dispatcher
// shows it once the line has been scanned, or drops it.
type pendingEcho struct {
	records []string
}

// verifyKeywords are SET VERIFY's keywords: which setting each changes,
// and to what.
var verifyKeywords = map[string]struct{ image, on bool }{
	"PROCEDURE":   {false, true},
	"NOPROCEDURE": {false, false},
	"IMAGE":       {true, true},
	"NOIMAGE":     {true, false},
}

// SetVerify is SET VERIFY [=([NO]PROCEDURE, [NO]IMAGE)] and, with off,
// SET NOVERIFY: with no keywords, both settings are turned on (or off);
// a keyword changes only its own. One SET VERIFY doesn't have is
// CLI_IVKEYW.
func (c *Console) SetVerify(off bool, keywords []string) error {
	if len(keywords) == 0 {
		c.Verify, c.verifyImage = !off, !off

		return nil
	}

	procedure, image := c.Verify, c.verifyImage

	for _, k := range keywords {
		key := strings.ToUpper(strings.TrimSpace(k))

		v, ok := verifyKeywords[key]
		if !ok {
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
		}

		on := v.on && !off
		if v.image {
			image = on
		} else {
			procedure = on
		}
	}

	c.Verify, c.verifyImage = procedure, image

	return nil
}

// SetPrefix is SET PREFIX "string" (and SET NOPREFIX, with string ""):
// the control string for verified lines' prefix. One longer than 64
// characters is CLI_IVVALU (unconfirmed against VMS).
func (c *Console) SetPrefix(control string) error {
	if len(control) > maxPrefixLength {
		return vmserrors.New(vmserrors.CLI_IVVALU)
	}

	c.verifyPrefix = control

	return nil
}

// setVerifyCommand is SET VERIFY's and SET NOVERIFY's handler: the
// keywords, given after "=" alone or as a list in parentheses.
func setVerifyCommand(c *Console, r *dcl.Result) error {
	var keywords []string

	for _, item := range r.List("KEYWORDS") {
		for _, k := range strings.Split(strings.Trim(item, "()"), ",") {
			if k = strings.TrimSpace(k); k != "" {
				keywords = append(keywords, k)
			}
		}
	}

	return c.SetVerify(r.Negated("WHAT"), keywords)
}

// prefix is the prefix of a verified command line: the SET PREFIX
// control string, formatted, at most 64 characters. Before there is a
// process to format it, the control string is used as it is.
func (c *Console) prefix() string {
	if c.verifyPrefix == "" {
		return ""
	}

	s := c.verifyPrefix

	if c.RTL != nil {
		zeros := make([]corevms.FAOValue, 20)

		formatted, status := c.RTL.FormatFAOValues(c.verifyPrefix, zeros)
		if status == 0 {
			s = formatted
		}
	}

	return s[:min(len(s), maxPrefixLength)]
}

// showVerified writes records, a verified command line's, with the
// prefix before the first and blanks as long as it before the rest.
func (c *Console) showVerified(records []string) {
	prefix := c.prefix()
	pad := strings.Repeat(" ", len(prefix))

	for i, record := range records {
		if i == 0 {
			fmt.Fprintln(c.Out, prefix+record)
		} else {
			fmt.Fprintln(c.Out, pad+record)
		}
	}
}

// echoLine shows the pending line (Console.echo), if procedure
// verification is on, and forgets it. label is the label taken off the
// front of the line ("" for none), before the rest of the line as it
// was read, and after that rest once scanned: when scanning changed
// nothing, the records are shown as they are; otherwise the line is
// shown as its first record starts ("$" and indentation), then the label
// and the scanned line.
func (c *Console) echoLine(label, before, after string) {
	e := c.echo
	c.echo = nil

	if e == nil || !c.Verify || len(e.records) == 0 {
		return
	}

	if before == after {
		c.showVerified(e.records)

		return
	}

	first := e.records[0]
	head := first[:len(first)-len(strings.TrimLeft(strings.TrimPrefix(first, "$"), " \t"))]

	c.showVerified([]string{head + label + after})
}

// dropEcho forgets the pending line without showing it: a line of an IF
// branch that doesn't run.
func (c *Console) dropEcho() { c.echo = nil }

// comments handles the comment (and blank) lines a procedure passed
// over before its next line: F$VERIFY between apostrophes is performed
// in each, and then, with procedure verification on, the line is shown.
func (c *Console) comments(records []string) {
	for _, record := range records {
		c.verifyInComment(record, &c.dclSymbols)

		if c.Verify {
			c.showVerified([]string{record})
		}
	}
}

// showData shows a data line given to what reads the procedure's input,
// if image verification is on.
func (c *Console) showData(record string) {
	if c.verifyImage {
		fmt.Fprintln(c.Out, record)
	}
}

// verifyInComment performs each F$VERIFY call between apostrophes in
// text, a comment ('F$VERIFY(0)', the name abbreviated or not; the
// closing apostrophe must be there). Nothing else in a comment is
// processed, and an error in the call is ignored (unconfirmed against
// VMS).
func (c *Console) verifyInComment(text string, symbols *dclSymbolTable) {
	for i := strings.IndexByte(text, '\''); i >= 0; {
		rest := text[i+1:]
		j := len(rest) - len(strings.TrimLeft(rest, " \t"))

		name := j
		for name < len(rest) && isDCLNameChar(rest[name]) {
			name++
		}

		paren := name
		for paren < len(rest) && (rest[paren] == ' ' || rest[paren] == '\t') {
			paren++
		}

		if fn := rest[j:name]; len(fn) > 2 && lexicalName(fn) == "F$VERIFY" && paren < len(rest) && rest[paren] == '(' {
			if end, ok := matchingParen(rest, paren); ok && end+1 < len(rest) && rest[end+1] == '\'' {
				_, _ = evaluateDCLExpression(upcaseOutsideQuotes(rest[j:end+1]), symbols, c)
				rest = rest[end+2:]
			}
		}

		k := strings.IndexByte(rest, '\'')
		if k < 0 {
			return
		}

		text, i = rest, k
	}
}
