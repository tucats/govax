package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// SYS$INPUT from a command procedure (docs/PHASE-50 - DCL command
// procedures.md, subtask 16).
//
// In VMS, an image that a command procedure runs reads the procedure
// itself as its SYS$INPUT: the records after the command that ran it, up
// to the next record that starts with "$", which is the image's end of
// file (User's Manual, 13.1.3 and 14.1). govax runs an image to its end
// inside the command that runs it, so the image pulls its lines from the
// procedure while the command runs: the procedure is a process-permanent
// file whose stream reads the level's next data record (readData) each
// time the image wants more, and SYS$INPUT names it (dclppf.go, Phase
// 71). Every way an image reads SYS$INPUT reads it: RMS's indirect $OPEN
// and $GET, LIB$GET_INPUT, and the input shims, through a logical name for
// SYS$INPUT too. A $QIO read reaches the device the name leads to, not
// the records, as on VMS. So does the console's TYPE SYS$INPUT
// (typeInput), by name or through logical names (IsInputStream).
//
// With image verification on (SET VERIFY=IMAGE), each record is shown as
// the image reads it, before the image does anything with it: VMS 7.3
// showed each line TYPE SYS$INPUT read, then typed it (testdata/dcl50's
// round 2, section V6).
//
// DECK and EOD (the DCL Dictionary): DECK makes the records after it data
// even when they start with "$", up to its end of file indicator, the
// record "$ EOD" by default, or a record that starts with DECK/DOLLARS's
// string; the indicator is then the default again, any "$" record. An
// EOD record read by an image ends its input and is used up, so the next
// read gets the data after it ("several input files ... without
// intervening commands"). The DECK and indicator records aren't shown by
// verification. DECK must come where the image reads (the manual's
// examples put it right after the command): read by DCL as a command it
// is refused (CLI$_IVVALU, VMS 7.3). A deck an image started and didn't
// read to its end goes on when DCL reads again: its records are data
// (procedureSource.Read; unconfirmed).
//
// The data lines nothing reads. After an image, DCL skips the records up
// to the next "$" record (even a blank one), with one CLI$_SKPDAT
// warning. After one of
// DCL's own commands, which don't read SYS$INPUT, DCL reads the next
// record as a command, "$" or not: VMS 7.3 read a data line after SET
// VERIFY as a command, %DCL-W-IVVERB with \DATA\ (section V6). Which of
// govax's commands VMS performs in DCL itself, rather than by running an
// image, is dclOwnCommands.

// maxInputRecord is the longest line a console command reads from
// SYS$INPUT: RMS's largest record.
const maxInputRecord = 32767

// dclOwnCommands are the console commands, by the grammar's name for the
// syntax that ran them (dcl.Result.Active), that VMS's DCL performs
// itself rather than by running an image (the DCL Dictionary lists
// them): after one of them a data line is read as a command, not
// skipped. Every other command counts as an image, as most of VMS's do
// (TYPE, DIRECTORY, RUN, ...); govax's own machine commands count as
// images too (unconfirmed: VMS has none of them). Symbol assignments,
// DELETE/SYMBOL, and @ are DCL's own as well, and never reach the
// grammar; nor does a command the grammar refuses, or an unknown verb,
// which run no image. VMS 7.3 agreed for each of these govax has
// (testdata/dcl50's round 3, section I1), WRITE among them; WAIT and SET
// SYMBOL, which govax doesn't have yet, were DCL's own there too, and so
// were OPEN, READ, and CLOSE in round 4 (F8); INQUIRE is taken to be
// (unconfirmed).
var dclOwnCommands = map[string]bool{
	"ASSIGN":           true,
	"CALL":             true,
	"CLOSE":            true,
	"CONTINUE":         true,
	"DEASSIGN":         true,
	"DECK":             true,
	"DEFINE":           true,
	"ELSE":             true,
	"ENDIF":            true,
	"ENDSUBROUTINE":    true,
	"EOD":              true,
	"EXIT":             true,
	"GOSUB":            true,
	"GOTO":             true,
	"IF":               true,
	"INQUIRE":          true,
	"ON":               true,
	"OPEN":             true,
	"QUIT":             true,
	"ATTACH":           true,
	"READ":             true,
	"RETURN":           true,
	"SET_CONTROL":      true,
	"SET_DEFAULT":      true,
	"SET_ON":           true,
	"SET_PREFIX":       true,
	"SET_PROMPT":       true,
	"SET_VERIFY":       true,
	"SHOW_DEFAULT":     true,
	"SHOW_SYM":         true,
	"SHOW_TRANSLATION": true,
	"SPAWN":            true,
	"STOP":             true,
	"SUBROUTINE":       true,
	"THEN":             true,
	"WAIT":             true,
	"WRITE":            true,
}

// procedureDeck is a DECK in effect: its end of file indicator, "" for
// the default, an EOD command.
type procedureDeck struct {
	marker string
}

// ends reports whether record is the deck's end of file indicator.
func (d *procedureDeck) ends(record string) bool {
	if d.marker == "" {
		rest, ok := strings.CutPrefix(record, "$")

		return ok && isEODCommand(rest)
	}

	return strings.HasPrefix(record, d.marker)
}

// isEODCommand reports whether command, a "$" record's text after the
// "$", is the EOD command.
func isEODCommand(command string) bool {
	verb, _ := readCommandVerb(strings.TrimSpace(command))
	verb, _, _ = strings.Cut(verb, "!")

	return strings.EqualFold(verb, "EOD")
}

// parseDeck reads command, a "$" record's text after the "$", as a DECK
// command: ok says it is one, and marker is its /DOLLARS string ("" for
// the default indicator, EOD). /DOLLARS may be abbreviated, and its
// string is read as a parameter is (dclWord: uppercased unless quoted).
func parseDeck(command string) (marker string, ok bool) {
	verb, rest := readCommandVerb(strings.TrimSpace(command))
	if !strings.EqualFold(verb, "DECK") {
		return "", false
	}

	rest = strings.TrimLeft(rest, " \t")
	if rest == "" || rest[0] == '!' {
		return "", true
	}

	// Anything but DECK's one qualifier makes the record a command,
	// which DCL's DECK rejects.
	if rest[0] != '/' {
		return "", false
	}

	qualifier, value, hasValue := strings.Cut(rest[1:], "=")
	if word, _, _ := dclWord(qualifier, 0, true); word == "" || !strings.HasPrefix("DOLLARS", word) {
		return "", false
	}

	if hasValue {
		marker, _, _ = dclWord(value, 0, true)
	}

	return marker, true
}

// readData reads the next record of the procedure's input for an image
// reading it as SYS$INPUT: ok is false at the input's end of file, a
// record that starts with "$". DECK and EOD records are handled as the
// DCL Dictionary describes them (above): a DECK record is used up and
// starts a deck, and the indicator that ends a deck, or an EOD record,
// is used up and ends this input. Any other "$" record stays, for DCL to
// read as its next command.
func (p *procedureSource) readData() (record string, ok bool) {
	for p.next < p.end {
		record = p.records[p.next]

		if p.deck != nil {
			p.next++

			if p.deck.ends(record) {
				p.deck = nil

				return "", false
			}

			return record, true
		}

		command, isCommand := strings.CutPrefix(record, "$")
		if !isCommand {
			p.next++

			return record, true
		}

		if marker, isDeck := parseDeck(command); isDeck {
			p.next++
			p.deck = &procedureDeck{marker: marker}

			continue
		}

		if isEODCommand(command) {
			p.next++
		}

		return "", false
	}

	return "", false
}

// IsInputStream reports whether a file specification names SYS$INPUT
// (with or without its colon), itself or through logical names: the
// process's input, which TYPE reads as lines rather than as a file. It
// does when the name leads to the process-permanent file SYS$INPUT leads
// to (so INFILE, defined as SYS$INPUT, is the input stream too: the DCL
// Dictionary's DECK example 2 types a procedure's data through such a
// name).
func (c *Interpreter) IsInputStream(spec string) bool {
	e, ok := c.permanentFile(strings.TrimSpace(spec))
	if !ok {
		return false
	}

	input, _ := c.permanentFile("SYS$INPUT")

	return e == input
}

// typeInput is TYPE SYS$INPUT: it types each line of the process's
// input until its end of file. In a command procedure that is the data
// records after the command (an image's SYS$INPUT, dclppf.go), each
// shown first with image verification on; at the terminal it is what is
// typed, up to Ctrl/Z.
func (c *Interpreter) typeInput() error {
	if c.RTL == nil {
		return nil
	}

	for c.Running() {
		line, ok := c.RTL.ReadLine(maxInputRecord)
		if !ok {
			break
		}

		c.Printf("%s\n", line)
	}

	return nil
}

// deckCommand is DECK read by DCL as a command, with no image reading
// the data after it: VMS 7.3 refused it with CLI$_IVVALU, a warning,
// with or without /DOLLARS, and started no deck (testdata/dcl50's I3),
// so the lines after it are what they would have been without it. At
// the terminal it is refused the same way (unconfirmed).
func (c *Interpreter) deckCommand(id int64, r *dcl.Result) error {
	return vmserrors.New(vmserrors.CLI_IVVALU)
}

// eodCommand is EOD read by DCL as a command, with no image reading the
// data it would end: it does nothing, and leaves $STATUS as it was (VMS
// 7.3, testdata/dcl50's I3).
func (c *Interpreter) eodCommand(id int64, r *dcl.Result) error {
	c.keepStatus()

	return nil
}

// TypeDCL is TYPE of spec when it names something only DCL has: SYS$INPUT
// (or a logical name for it), typed a line at a time (typeInput), or a
// logical name DCL's OPEN defined, typed from where READ has got to
// (typeOpenFile). handled is false for anything else.
func (c *Interpreter) TypeDCL(spec string) (handled bool, err error) {
	if c.IsInputStream(spec) {
		return true, c.typeInput()
	}

	return c.typeOpenFile(spec), nil
}
