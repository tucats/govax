package dclshell

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dcl"
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
// label, and its comment, after the first phase's substitutions. A line
// whose scan fails isn't shown, only the message. Comment lines are
// shown too, and a command continued over several records is shown a
// record at a time, each with its own substitutions. A CALL shows its
// subroutine's SUBROUTINE line as the subroutine starts, and its
// ENDSUBROUTINE as it ends. The lines of an IF block's branch that
// doesn't run aren't shown, but for the ELSE or ENDIF that ends it, and
// the THEN of a block whose IF was false isn't shown either. (VMS 7.3's
// logs of testdata/dcl50, which TestProbe50Verify and TestProbe50VOracle
// replay line for line.) Never shown are the terminal's commands and a
// debugger command procedure's lines (the debugger has its own SET
// OUTPUT VERIFY). An ON action isn't shown (VMS 7.3).
//
// DCL does one thing in a comment: it performs F$VERIFY between
// apostrophes, 'F$VERIFY(0)' (the Dictionary's F$VERIFY), and the line
// is shown with the call replaced by its value ("$ ! 0"). As that
// happens in the first phase, before the line is shown, "$ ! 'F$VERIFY(0)'"
// turns verification off without being shown itself, the usual way for a
// procedure to keep its lines out of sight.
//
// SET PREFIX gives a $FAO control string whose result goes in front of
// each verified command line, the time with "(!5%T) ", and blanks as long
// as it in front of the records that continue one. Only the time's and
// date's directives may take an argument (validPrefix). Data lines get no
// prefix.

// maxPrefixLength is the longest prefix control string, and the longest
// prefix it may make (the Dictionary's SET PREFIX).
const maxPrefixLength = 64

// pendingEcho is the line a command procedure is about to run, for
// verification: its records, as they are in the file. The procedure loop
// sets it (Console.echo) before dispatching the line, and the dispatcher
// shows it once the line has been scanned, or drops it.
type pendingEcho struct {
	records []string

	// line is the command line the records make, and joins the offsets
	// in it of the blanks that join one record to the next.
	line  string
	joins []int
}

// verifyKeywords are SET VERIFY's keywords: which setting each changes,
// and to what. Any unique abbreviation will do (VMS 7.3 took PROC, IMAG,
// even IMA).
var verifyKeywords = map[string]struct{ image, on bool }{
	"PROCEDURE":   {false, true},
	"NOPROCEDURE": {false, false},
	"IMAGE":       {true, true},
	"NOIMAGE":     {true, false},
}

// verifyKeyword returns the SET VERIFY keyword k abbreviates, if it
// abbreviates exactly one.
func verifyKeyword(k string) (string, bool) {
	if _, ok := verifyKeywords[k]; ok {
		return k, true
	}

	found := ""

	for name := range verifyKeywords {
		if strings.HasPrefix(name, k) {
			if found != "" {
				return "", false
			}

			found = name
		}
	}

	return found, found != ""
}

// SetVerify is SET VERIFY [=([NO]PROCEDURE, [NO]IMAGE)] and, with off,
// SET NOVERIFY: with no keywords, both settings are turned on (or off);
// a keyword changes only its own. A keyword SET VERIFY doesn't have is
// CLI_IVKEYW; SET NOVERIFY with keywords is CLI_CONFLICT, with word, the
// command's NOVERIFY as it was typed, as its segment (VMS 7.3).
func (c *Interpreter) SetVerify(off bool, word string, keywords []string) error {
	if len(keywords) == 0 {
		c.Verify, c.verifyImage = !off, !off

		return nil
	}

	if off {
		return vmserrors.NewSegment(vmserrors.CLI_CONFLICT, word)
	}

	procedure, image := c.Verify, c.verifyImage

	for _, k := range keywords {
		key := strings.ToUpper(strings.TrimSpace(k))

		name, ok := verifyKeyword(key)
		if !ok {
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
		}

		v := verifyKeywords[name]
		if v.image {
			image = v.on
		} else {
			procedure = v.on
		}
	}

	c.Verify, c.verifyImage = procedure, image

	return nil
}

// SetPrefix is SET PREFIX "string" (and SET NOPREFIX, with string ""):
// the control string for verified lines' prefix. One longer than 64
// characters is CLI_IVVALU (unconfirmed against VMS); one with a
// directive the prefix can't use is CLI_INVPREFIX, and the prefix stays
// as it was (VMS 7.3).
func (c *Interpreter) SetPrefix(control string) error {
	if len(control) > maxPrefixLength {
		return vmserrors.New(vmserrors.CLI_IVVALU)
	}

	if !validPrefix(control) {
		return vmserrors.New(vmserrors.CLI_INVPREFIX)
	}

	c.verifyPrefix = control

	return nil
}

// validPrefix reports whether control is a control string SET PREFIX
// takes. VMS 7.3 took "(!5%T) " and refused "[!UL|!AS|!XL] " and
// "!%D !"; govax's rule (unconfirmed beyond those) is that the only
// directives that take an argument may be the time's and date's, !%T and
// !%D (their argument being 0, the current time), with a field width,
// and that the rest must take none: !/, !_, !^, !!, and !n*c. A "!" at
// the end is malformed.
func validPrefix(control string) bool {
	for i := 0; i < len(control); i++ {
		if control[i] != '!' {
			continue
		}

		i++
		for i < len(control) && control[i] >= '0' && control[i] <= '9' {
			i++
		}

		if i >= len(control) {
			return false
		}

		switch rest := control[i:]; {
		case strings.ContainsRune("/_^!", rune(rest[0])):
		case rest[0] == '*' && len(rest) > 1:
			i++
		case strings.HasPrefix(rest, "%T"), strings.HasPrefix(rest, "%D"):
			i++
		default:
			return false
		}
	}

	return true
}

// setVerifyCommand is SET VERIFY's and SET NOVERIFY's handler: the
// keywords, given after "=" alone or as a list in parentheses. line is
// the command line, from which SET's keyword is taken as it was typed.
func setVerifyCommand(c *Interpreter, r *dcl.Result, line string) error {
	var keywords []string

	for _, item := range r.List("KEYWORDS") {
		for _, k := range strings.Split(strings.Trim(item, "()"), ",") {
			if k = strings.TrimSpace(k); k != "" {
				keywords = append(keywords, k)
			}
		}
	}

	return c.SetVerify(r.Negated("WHAT"), setKeywordWord(line), keywords)
}

// setKeywordWord returns the word after SET's verb in line, as typed:
// up to a blank, "=", "/", or "(".
func setKeywordWord(line string) string {
	_, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	rest = strings.TrimLeft(rest, " \t")

	if end := strings.IndexAny(rest, " \t=/("); end >= 0 {
		rest = rest[:end]
	}

	return strings.ToUpper(rest)
}

// prefix is the prefix of a verified command line: the SET PREFIX
// control string, formatted, at most 64 characters. Before there is a
// process to format it, the control string is used as it is.
func (c *Interpreter) prefix() string {
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
func (c *Interpreter) showVerified(records []string) {
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

// joinMark stands, while a continued command is scanned, where one of
// its records was joined to the next, so that the scanned line can be
// shown a record at a time again.
const joinMark = "\x00"

// newPendingEcho returns the pending echo of a command line whose
// records are records: the line they make, joined as procedureSource
// joins them, and the offsets in it of the blanks that join them.
func newPendingEcho(records []string) *pendingEcho {
	line := strings.TrimSpace(strings.TrimPrefix(records[0], "$"))

	var joins []int

	for _, record := range records[1:] {
		head, _ := continuation(line)
		joins = append(joins, len(head))
		line = head + " " + strings.TrimSpace(record)
	}

	return &pendingEcho{records: records, line: line, joins: joins}
}

// markJoins returns line, the part of whole after its label, with
// joinMark where the pending line's records were joined, so its scan
// keeps them apart; line itself if there is no pending line, it has
// one record, or whole isn't the line it was made from.
func (c *Interpreter) markJoins(whole, line string) string {
	e := c.echo
	if e == nil || len(e.joins) == 0 || whole != e.line {
		return line
	}

	skip := len(whole) - len(line)
	b := []byte(line)

	for _, at := range e.joins {
		if at -= skip; at < 0 || at >= len(b) || b[at] != ' ' {
			return line
		}

		b[at] = joinMark[0]
	}

	return string(b)
}

// echoLine shows the pending line (Console.echo), if procedure
// verification is on, and forgets it. label is the label taken off the
// front of the line ("" for none), before the rest of the line as it
// was read, and after that rest once scanned, both with joinMark where
// records were joined (markJoins). When scanning changed nothing, the
// records are shown as they are. Otherwise each record is shown as it
// is but for its part of the line, which is shown scanned: VMS 7.3
// shows a continued command a record at a time, each substituted.
func (c *Interpreter) echoLine(label, before, after string) {
	e := c.echo
	c.echo = nil

	if e == nil || !c.Verify || len(e.records) == 0 {
		return
	}

	if before == after {
		c.showVerified(e.records)

		return
	}

	if shown, ok := scannedRecords(e.records, label, before, after); ok {
		c.showVerified(shown)

		return
	}

	// Where the records can't be matched up, the line is shown whole, as
	// its first record starts ("$" and indentation).
	first := e.records[0]
	head := first[:len(first)-len(strings.TrimLeft(strings.TrimPrefix(first, "$"), " \t"))]

	c.showVerified([]string{head + label + strings.ReplaceAll(after, joinMark, " ")})
}

// scannedRecords returns records with each one's part of the line
// replaced by that part once scanned: before and after split at
// joinMark, one part per record. ok is false if they don't match up.
func scannedRecords(records []string, label, before, after string) ([]string, bool) {
	was := strings.Split(before, joinMark)
	now := strings.Split(after, joinMark)

	if len(was) != len(records) || len(now) != len(records) {
		return nil, false
	}

	shown := make([]string, len(records))

	for i, record := range records {
		part, start := was[i], len(record)-len(strings.TrimLeft(record, " \t"))
		if i == 0 {
			part = label + part
			start = len(record) - len(strings.TrimLeft(strings.TrimPrefix(record, "$"), " \t"))
		}

		if !strings.HasPrefix(record[start:], part) {
			return nil, false
		}

		text := now[i]
		if i == 0 {
			text = label + text
		}

		shown[i] = record[:start] + text + record[start+len(part):]
	}

	return shown, true
}

// dropEcho forgets the pending line without showing it: a line of an IF
// branch that doesn't run.
func (c *Interpreter) dropEcho() { c.echo = nil }

// comments handles the comment (and blank) lines a procedure passed
// over before its next line: F$VERIFY between apostrophes is performed
// in each, and then, with procedure verification on, the line is shown,
// the call replaced by its value.
func (c *Interpreter) comments(records []string) {
	for _, record := range records {
		shown := c.verifyInComment(record, &c.dclSymbols)

		if c.Verify {
			c.showVerified([]string{shown})
		}
	}
}

// showData shows a data line given to what reads the procedure's input,
// if image verification is on.
func (c *Interpreter) showData(record string) {
	if c.verifyImage {
		fmt.Fprintln(c.Out, record)
	}
}

// verifyInComment performs each F$VERIFY call between apostrophes in
// text, a comment ('F$VERIFY(0)', the name abbreviated or not), and
// returns text with each call, apostrophes and all, replaced by its
// value, as VMS 7.3 shows it ("$ ! 0"). The closing apostrophe may be
// missing (VMS 7.3 still performed the call). Nothing else in a comment
// is processed, and a call with an error is left as it is (unconfirmed
// against VMS).
func (c *Interpreter) verifyInComment(text string, symbols *dclSymbolTable) string {
	var out strings.Builder

	for {
		i := strings.IndexByte(text, '\'')
		if i < 0 {
			out.WriteString(text)

			return out.String()
		}

		out.WriteString(text[:i])

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
			if end, ok := matchingParen(rest, paren); ok {
				if v, err := evaluateDCLExpression(upcaseOutsideQuotes(rest[j:end+1]), symbols, c); err == nil {
					if end++; end < len(rest) && rest[end] == '\'' {
						end++
					}

					out.WriteString(v.String())
					text = rest[end:]

					continue
				}
			}
		}

		out.WriteByte('\'')
		text = rest
	}
}
