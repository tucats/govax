package console

import (
	"io"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/lnm"
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
// procedure while the command runs: each command line of a procedure is
// dispatched with process 1's input stream redirected to a
// procedureInput (withProcedureInput), which reads the level's next data
// record each time the image wants more. Every way an image reads its
// input reads it: a terminal $QIO read, RMS's $GET of SYS$INPUT,
// LIB$GET_INPUT, and the input shims; a program that opens a logical
// name for SYS$INPUT gets there through RMS's translation, to the
// terminal. So does the console's TYPE SYS$INPUT (typeInput), by name or
// through logical names (isInputStream).
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

// procedureInput is a command level's data records as a stream of lines:
// process 1's input (SYS$INPUT) while a procedure's command runs. Each
// Read that finds nothing left of the last line reads the next record
// (readData), so an image takes records as it reads them, and those it
// doesn't read stay in the procedure. It is a corevms.TerminalSource
// that never waits.
type procedureInput struct {
	c      *Interpreter
	source *procedureSource

	// shell is the DCL whose procedure it is: an image may read while
	// another process's DCL is the console's, and the record is shown with
	// this one put back for the moment (subcli.go).
	shell *Shell

	// rest is what is left of the line Read last read, "\n" and all.
	rest []byte

	// closed says the command the stream was made for has ended: a
	// process that kept it (a subprocess started meanwhile) is at its
	// end of file.
	closed bool
}

func (in *procedureInput) Read(b []byte) (int, error) {
	if len(in.rest) == 0 {
		if in.closed {
			return 0, io.EOF
		}

		record, ok := in.source.readData()
		if !ok {
			return 0, io.EOF
		}

		restore := in.c.useShell(in.shell)
		in.c.showData(record)
		restore()

		in.rest = []byte(record + "\n")
	}

	n := copy(b, in.rest)
	in.rest = in.rest[n:]

	return n, nil
}

// Ready says a read never waits: the records are at hand.
func (in *procedureInput) Ready() bool { return true }

// withProcedureInput runs fn, a line of source's procedure, with process
// 1's input the procedure's data records (procedureInput), as an image
// the line runs reads them, and puts the terminal back afterward. A
// debugger command procedure's lines are all commands, so it has no
// data; its lines run with the input as it is.
func (c *Interpreter) withProcedureInput(source *procedureSource, fn func() error) error {
	env := c.RTL
	if env == nil || source.plain {
		return fn()
	}

	in := &procedureInput{c: c, source: source, shell: c.Shell}
	restore := env.RedirectInput(in)

	defer func() {
		in.closed = true

		restore()
	}()

	return fn()
}

// isInputStream reports whether a file specification names SYS$INPUT
// (with or without its colon), itself or through logical names: the
// process's input, which TYPE reads as lines rather than as a file
// (processPermanentFile).
func (c *Interpreter) isInputStream(spec string) bool {
	return c.processPermanentFile(spec) == "SYS$INPUT"
}

// processPermanentNames are the process-permanent files every process
// has, which DCL's READ and WRITE use without an OPEN (dclfile.go).
var processPermanentNames = map[string]bool{
	"SYS$INPUT": true, "SYS$OUTPUT": true, "SYS$ERROR": true, "SYS$COMMAND": true,
}

// processPermanentFile returns which of the process-permanent files
// (processPermanentNames) a file specification names, with or without
// its colon, itself or through logical names; "" for none. RMS
// translates a file specification's logical name again and again, as far
// as LNM$C_MAXDEPTH levels, until nothing translates or an equivalence
// is terminal (lnm.TranslateFileSpec), and the process-permanent names
// are among those names: so INFILE, defined as SYS$INPUT, is the input
// stream too (the DCL Dictionary's DECK example 2 types a procedure's
// data through such a name). Each step takes a name's first equivalence
// string; one with anything after the name and its colon (a directory, a
// file name) is a file, not a process-permanent one.
func (c *Interpreter) processPermanentFile(spec string) string {
	name := strings.ToUpper(strings.TrimSpace(spec))

	for range lnm.MaxDepth {
		name = strings.TrimSuffix(name, ":")
		if processPermanentNames[name] {
			return name
		}

		if c.Logicals == nil || name == "" || !isDCLName(name) {
			return ""
		}

		e, err := c.Logicals.Translate(lnm.FileDevName, name, lnm.User, 0)
		if err != nil || len(e.Equivalences) == 0 {
			return ""
		}

		eqv := e.Equivalences[0]
		name = strings.ToUpper(strings.TrimSpace(eqv.Value))

		if eqv.Attrs&lnm.AttrTerminal != 0 {
			name = strings.TrimSuffix(name, ":")
			if processPermanentNames[name] {
				return name
			}

			return ""
		}
	}

	return ""
}

// isDCLName reports whether name could be a logical name: letters,
// digits, "$", "_", and "-" only.
func isDCLName(name string) bool {
	for i := range len(name) {
		ch := name[i]
		if !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '$' && ch != '_' && ch != '-' {
			return false
		}
	}

	return true
}

// typeInput is TYPE SYS$INPUT: it types each line of the process's
// input until its end of file. In a command procedure that is the data
// records after the command (an image's SYS$INPUT, procedureInput), each
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
