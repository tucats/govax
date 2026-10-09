package console

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Command procedures and command levels (docs/PHASE-50 - DCL command
// procedures.md), as the OpenVMS User's Manual describes them.
//
// A command level is an input stream for the command interpreter. The
// terminal is command level 0. "@file" runs the command procedure in file
// one level deeper: it pushes a level onto the console's input stack,
// with the file as its source and a local symbol table of its own, runs
// the file's commands, and pops the level when the file ends, at an
// EXIT, or when a command fails with an error. The level's local symbols
// go with it, so "=" and ":=" (local) and "==" and ":==" (global) differ
// in how long a symbol lasts, not only in which table holds it.
//
//	@filespec[/OUTPUT=filespec] [p1 [p2 ... p8]]
//
// The procedure's parameters are the local symbols P1 to P8 at its level,
// "" for those not given. /OUTPUT sends what the procedure writes to a
// file instead of the terminal.
//
// The stack is explicit (Console.levels), not just Go's call stack. The
// procedure runs synchronously: whoever dispatched "@file" (the prompt,
// another procedure, TIME, a symbol's value, the debugger's @, cmd/govax
// running vax.init) gets control back when the procedure ends. The
// commands that act on the current level find it there: EXIT now, and
// later GOTO, GOSUB, block IF, ON, and F$ENVIRONMENT. A level's source
// keeps its file's records and a cursor, so a later GOTO can move the
// cursor (Seek, Rewind) and the level's loop reads on from there.

// maxCommandLevels is how many command levels may be active above level
// 0, the terminal: DCL's limit of 32 nested levels (the User's Manual's
// glossary, "command level").
const maxCommandLevels = 32

// maxProcedureParameters is how many parameters a command procedure takes:
// P1 to P8 (User's Manual, 14.2).
const maxProcedureParameters = 8

// commandLevel is one command level above the terminal: a command
// procedure being run.
type commandLevel struct {
	// source is where the level's commands come from.
	source *procedureSource

	// exiting is set by EXIT: the level ends after the command that's
	// running now.
	exiting bool

	// output is the level's /OUTPUT file, nil without one. restoreOut is
	// the Console.Out it replaced, put back when the level ends.
	output     *procedureOutput
	restoreOut io.Writer

	// sysCommand is the terminal: level 0's Console.Out. redirected says
	// this level's output, or an outer level's, goes to an /OUTPUT file,
	// so an error message goes to sysCommand too, as DCL sends one both
	// to the file and to the terminal (User's Manual, 13.6.4).
	sysCommand io.Writer
	redirected bool
}

// currentLevel returns the innermost command level, or nil at the
// terminal (level 0).
func (c *Console) currentLevel() *commandLevel {
	if len(c.levels) == 0 {
		return nil
	}

	return c.levels[len(c.levels)-1]
}

// CommandLevel is the current command level: 0 at the terminal, 1 in a
// command procedure run from it, and so on.
func (c *Console) CommandLevel() int { return len(c.levels) }

// procedureSource is a command procedure's text: its records (lines),
// read whole when the procedure starts, and a cursor at the next one to
// read.
type procedureSource struct {
	// name is the file that was read, for messages.
	name string

	// records are the file's lines, as they are in the file.
	records []string

	// next is the index of the next record Read looks at.
	next int

	// plain says the file is a debugger command procedure, whose lines
	// are all commands, rather than a DCL one, whose commands start with
	// "$".
	plain bool
}

// newProcedureSource returns the source for a procedure whose text is
// text: one record per line, whether lines end in LF, CR LF, or CR.
func newProcedureSource(name, text string) *procedureSource {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSuffix(text, "\n")

	var records []string
	if text != "" {
		records = strings.Split(text, "\n")
	}

	return &procedureSource{name: name, records: records}
}

// Read returns the next line of the procedure, and false at its end.
//
// In a DCL procedure (User's Manual, 13.1 to 13.3), a record whose first
// character is "$" is a command line: Read returns the command, without
// the "$", joined with the records after it while it ends in a hyphen
// (continuation lines have no "$"). A "$" line that is empty or starts
// with "!" is a comment, and is skipped. Any other record is a data line,
// for whatever reads the procedure's input (data is true): Read returns
// it as it is.
//
// In a debugger command procedure (plain), every record is a command,
// with the same continuations, and blank lines and "!" comments are
// skipped; data is always false.
func (p *procedureSource) Read() (line string, data bool, ok bool) {
	for p.next < len(p.records) {
		record := p.records[p.next]
		p.next++

		if p.plain {
			line = strings.TrimSpace(record)
		} else if rest, isCommand := strings.CutPrefix(record, "$"); isCommand {
			line = strings.TrimSpace(rest)
		} else {
			return record, true, true
		}

		if line == "" || line[0] == '!' {
			continue
		}

		for {
			head, more := continuation(line)
			if !more || p.next >= len(p.records) {
				break
			}

			line = head + " " + strings.TrimSpace(p.records[p.next])
			p.next++
		}

		return line, false, true
	}

	return "", false, false
}

// Position returns the cursor: where the next Read starts.
func (p *procedureSource) Position() int { return p.next }

// Seek moves the cursor to pos, a value Position returned (or 0, the
// first record).
func (p *procedureSource) Seek(pos int) { p.next = min(max(pos, 0), len(p.records)) }

// Rewind moves the cursor back to the first record.
func (p *procedureSource) Rewind() { p.Seek(0) }

// continuation reports whether line goes on in the next record: whether
// its last character, outside quotes and before any comment, is a
// hyphen. It returns the line without the hyphen and the comment.
func continuation(line string) (string, bool) {
	end := len(line)
	quoted := false

	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"':
			quoted = !quoted
		case line[i] == '!' && !quoted:
			end = i
		}

		if end < len(line) {
			break
		}
	}

	text := strings.TrimRight(line[:end], " \t")
	if quoted || !strings.HasSuffix(text, "-") {
		return line, false
	}

	return strings.TrimRight(text[:len(text)-1], " \t"), true
}

// procedureCommand is what an @ command says: the procedure's file, its
// /OUTPUT file, and its parameters. host says the file is on the host
// whatever the default device (RunHostProcedure), and plain that it's a
// debugger command procedure (RunDebuggerProcedure).
type procedureCommand struct {
	file      string
	host      bool
	plain     bool
	output    string
	hasOutput bool
	params    []string
}

// parseProcedureCommand reads an @ command's text after the "@". The file
// specification comes first, ended by a blank or a "/". Qualifiers count
// only where they follow it directly: DCL takes a qualifier after a blank
// for a parameter (User's Manual, 13.6.4). The rest are the parameters,
// read by DCL's rules (dclWord).
func parseProcedureCommand(text string) (procedureCommand, error) {
	var cmd procedureCommand

	file, i, _ := dclWord(text, 0, true)
	if file == "" {
		return cmd, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "file")
	}

	cmd.file = file

	for i < len(text) && text[i] == '/' {
		j := i + 1
		for j < len(text) && text[j] != '=' && text[j] != '/' && text[j] != ' ' && text[j] != '\t' {
			j++
		}

		name := strings.ToUpper(text[i+1 : j])
		if name == "" || !strings.HasPrefix("OUTPUT", name) {
			return cmd, vmserrors.New(vmserrors.CLI_UNRECOGNIZED, "qualifier", "/"+name)
		}

		cmd.hasOutput = true
		i = j

		// DCL allows blanks around the "=".
		if k := len(text) - len(strings.TrimLeft(text[i:], " \t")); k < len(text) && text[k] == '=' {
			i = k
		}

		if i < len(text) && text[i] == '=' {
			cmd.output, i, _ = dclWord(text, i+1, true)
		}

		if cmd.output == "" {
			return cmd, vmserrors.New(vmserrors.CLI_NEEDQUALIFIERVALUE, "OUTPUT")
		}
	}

	for {
		word, next, present := dclWord(text, i, false)
		if !present {
			break
		}

		cmd.params = append(cmd.params, word)
		i = next
	}

	if len(cmd.params) > maxProcedureParameters {
		return cmd, vmserrors.New(vmserrors.CLI_MAXPARM)
	}

	return cmd, nil
}

// dclWord reads the word of text that starts at i, after any blanks, by
// DCL's rules for a parameter: it ends at a blank or tab outside quotes
// (or at a "/", with stopAtSlash), and a "!" outside quotes ends the
// text. Outside quotes, letters are uppercased; quoted text keeps its
// case and blanks, the quotes are removed, and "" inside quotes is one
// quote. So "" alone is an empty word, which is still present. It
// returns the word, where the text after it starts, and whether there
// was a word at all.
func dclWord(text string, i int, stopAtSlash bool) (string, int, bool) {
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}

	if i >= len(text) || text[i] == '!' {
		return "", len(text), false
	}

	var b strings.Builder

	quoted := false

	for ; i < len(text); i++ {
		ch := text[i]

		if quoted {
			switch {
			case ch == '"' && i+1 < len(text) && text[i+1] == '"':
				b.WriteByte('"')

				i++
			case ch == '"':
				quoted = false
			default:
				b.WriteByte(ch)
			}

			continue
		}

		switch {
		case ch == '"':
			quoted = true
		case ch == ' ' || ch == '\t' || ch == '!' || (stopAtSlash && ch == '/'):
			return b.String(), i, true
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch - ('a' - 'A'))
		default:
			b.WriteByte(ch)
		}
	}

	return b.String(), i, true
}

// atCommand is the @ command: text is what follows the "@".
func (d *Dispatcher) atCommand(text string) error {
	cmd, err := parseProcedureCommand(text)
	if err != nil {
		return err
	}

	return d.Console.runProcedure(cmd, d.Dispatch)
}

// RunProcedure runs the DCL command procedure in file, with no
// parameters, at a new command level, sending each of its lines to
// dispatch: what "@file" does, for callers outside the console's own
// dispatcher.
func (c *Console) RunProcedure(file string, dispatch func(string) error) error {
	return c.runProcedure(procedureCommand{file: file}, dispatch)
}

// RunHostProcedure is RunProcedure for a host file, whatever the default
// device: govax's own startup procedure, vax.init, found through the
// search path (c.Paths) even when SET DEFAULT is on a volume.
func (c *Console) RunHostProcedure(file string, dispatch func(string) error) error {
	return c.runProcedure(procedureCommand{file: file, host: true}, dispatch)
}

// RunDebuggerProcedure is the debugger's @: a debugger command
// procedure, whose lines are all commands, with no "$" (the VMS
// debugger's format). It runs at a new command level, like any other.
func (c *Console) RunDebuggerProcedure(file string, dispatch func(string) error) error {
	return c.runProcedure(procedureCommand{file: file, plain: true}, dispatch)
}

// runProcedure runs the procedure cmd names at a new command level, with
// its parameters as P1 to P8, until its last command, an EXIT, or a
// command that fails with an error or severe error. The last is DCL's
// default action, ON ERROR THEN EXIT (User's Manual, 13.8): the message
// is shown, the procedure ends, and the @ command fails with the same
// status, with its message marked as shown so no level shows it again. A
// failure with a warning, success, or informational status is shown, and
// the procedure goes on.
func (c *Console) runProcedure(cmd procedureCommand, dispatch func(string) error) (err error) {
	if len(c.levels) >= maxCommandLevels {
		return vmserrors.New(vmserrors.CLI_MAXDEPTH, maxCommandLevels)
	}

	source, err := c.openProcedure(cmd.file, cmd.host)
	if err != nil {
		return err
	}

	source.plain = cmd.plain

	level := &commandLevel{source: source, sysCommand: c.Out}
	if outer := c.currentLevel(); outer != nil {
		level.sysCommand, level.redirected = outer.sysCommand, outer.redirected
	}

	if cmd.hasOutput {
		out, err := c.openProcedureOutput(cmd.output)
		if err != nil {
			return err
		}

		level.output, level.restoreOut, level.redirected = out, c.Out, true
		c.Out = out
	}

	c.levels = append(c.levels, level)
	c.dclSymbols.push()

	defer func() {
		if closeErr := c.endLevel(level); err == nil {
			err = closeErr
		}
	}()

	locals := c.dclSymbols.local()

	for i := range maxProcedureParameters {
		name := fmt.Sprintf("P%d", i+1)

		value := ""
		if i < len(cmd.params) {
			value = cmd.params[i]
		}

		locals[name] = dclSymbol{name: name, minLength: len(name), value: value}
	}

	// skipping is set while DCL skips a run of data lines.
	skipping := false

	for !level.exiting && c.Running() {
		line, data, ok := source.Read()
		if !ok {
			break
		}

		var status error

		switch {
		case data && !c.readingInput():
			// Nothing reads the procedure's input, so DCL skips the data,
			// with one warning for each run of data lines.
			if !skipping {
				status = vmserrors.New(vmserrors.CLI_SKPDAT)
			}

			skipping = true

		case data:
			skipping = false
			status = dispatch(line)

		default:
			skipping = false

			// What was reading the procedure's input has come to its end.
			if !source.plain {
				status = c.endInput(dispatch)
			}

			if status == nil {
				status = dispatch(line)
			}
		}

		if status == nil {
			continue
		}

		var ve vmserrors.VMSError
		if errors.As(status, &ve) && ve.Status == vmserrors.VAX_QUIT {
			c.quit = true

			return nil
		}

		if !vmserrors.MessageInhibited(status) {
			c.procedureMessage(level, status)
		}

		if endsProcedure(status) {
			return vmserrors.InhibitMessage(status)
		}
	}

	return nil
}

// readingInput reports whether something other than DCL reads the
// console's input now: the debugger, or the interactive assembler. In a
// DCL procedure, its data lines are their input, as a VMS image run from
// a procedure reads the procedure's data lines (User's Manual, 14.1): a
// procedure that starts the debugger gives it its commands that way.
func (c *Console) readingInput() bool {
	return c.InDebugger() || c.InAssemblerMode()
}

// endInput ends the input of whatever reads it (readingInput), as a VMS
// image reading a procedure's data lines comes to end of file at the
// next command line: the debugger EXITs, and the interactive assembler
// takes it as .END, as each does for Ctrl/Z at its prompt.
func (c *Console) endInput(dispatch func(string) error) error {
	switch {
	case c.InAssemblerMode():
		return dispatch(".END")
	case c.InDebugger():
		return dispatch("EXIT")
	}

	return nil
}

// endLevel pops level, the innermost command level: its local symbols
// go, and its /OUTPUT file, if it has one, is closed and the console's
// output put back.
func (c *Console) endLevel(level *commandLevel) error {
	c.dclSymbols.pop()
	c.levels = c.levels[:len(c.levels)-1]

	if level.output == nil {
		return nil
	}

	c.Out = level.restoreOut

	return level.output.close()
}

// procedureMessage shows the message for a command in a procedure that
// failed: on the procedure's output, and on the terminal too when that
// output is an /OUTPUT file.
func (c *Console) procedureMessage(level *commandLevel, err error) {
	text := "%" + err.Error() + "\n"

	fmt.Fprint(c.Out, text)

	if level.redirected && level.sysCommand != nil {
		fmt.Fprint(level.sysCommand, text)
	}
}

// endsProcedure reports whether a command's failure ends the procedure
// it's in, by DCL's default action: an error or a severe error does, a
// warning doesn't. A failure that isn't a VMS status is an error.
func endsProcedure(err error) bool {
	var ve vmserrors.VMSError
	if !errors.As(err, &ve) {
		return true
	}

	severity := ve.SeverityCode()

	return severity == vmserrors.StatusError || severity == vmserrors.StatusSevere
}

// Exit is the console's EXIT: in a command procedure it ends the
// procedure, and the level above goes on (User's Manual, 13.7.1); at the
// terminal it ends govax, as QUIT does.
func (c *Console) Exit() error {
	if level := c.currentLevel(); level != nil {
		level.exiting = true

		return nil
	}

	return c.Quit()
}

// openProcedure reads the command procedure file names. Where the file is,
// on the host (always, with host) or on a mounted volume, follows
// rms.Session.Locate's rules,
// and a name with no file type gets .COM (User's Manual, 13.1.1). A host
// file is looked for as every console file is, through c.Paths (the
// search path, then govax's built-in files), with .COM and then as given.
func (c *Console) openProcedure(file string, host bool) (*procedureSource, error) {
	loc, err := c.ContainerSession.Locate(file, host)
	if err != nil {
		return nil, fileFailure(err, file)
	}

	if loc.Host {
		var firstErr error

		for _, name := range procedureHostNames(loc.Name) {
			data, err := c.Paths.ReadFile(name)
			if err == nil {
				return newProcedureSource(name, string(data)), nil
			}

			if firstErr == nil {
				firstErr = err
			}
		}

		var notFound *respath.NotFoundError
		if errors.As(firstErr, &notFound) {
			return nil, vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, firstErr, file)
		}

		return nil, fileFailure(firstErr, file)
	}

	loc = withDefaultType(loc, "COM")

	records, found, err := c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return nil, fileFailure(err, file)
	}

	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	return &procedureSource{name: found.Name, records: lines}, nil
}

// procedureHostNames are the host names a procedure's file name may mean,
// in the order they're tried: a name with no extension with .COM added
// (in the name's case), then as given. DCL uppercases a name that isn't
// quoted, so a name with no lowercase letters is tried in lowercase too,
// after each of those: "@VAX.INIT" finds vax.init.
func procedureHostNames(name string) []string {
	names := []string{name}
	if filepath.Ext(name) == "" {
		names = []string{name + "." + matchCase("", "com", filepath.Base(name)), name}
	}

	if strings.ToUpper(name) != name {
		return names
	}

	for _, n := range names {
		names = append(names, strings.ToLower(n))
	}

	return names
}

// procedureOutput is a command procedure's /OUTPUT file: what the
// procedure writes goes to it, and close finishes it.
type procedureOutput struct {
	io.Writer

	close func() error
}

// openProcedureOutput opens the /OUTPUT file name names: a host file,
// written as the procedure writes, or a file on a mounted volume, written
// when the procedure ends (a volume file is written whole). A name with
// no file type gets .LIS. The null device, NL:, discards the output.
func (c *Console) openProcedureOutput(name string) (*procedureOutput, error) {
	if isNullDevice(name) {
		return &procedureOutput{Writer: io.Discard, close: func() error { return nil }}, nil
	}

	loc, err := c.ContainerSession.Locate(name, false)
	if err != nil {
		return nil, fileFailure(err, name)
	}

	loc = withDefaultType(loc, "LIS")

	if loc.Host {
		f, err := os.Create(loc.Name)
		if err != nil {
			return nil, fileFailure(err, name)
		}

		return &procedureOutput{Writer: f, close: f.Close}, nil
	}

	var buf bytes.Buffer

	session := c.ContainerSession

	return &procedureOutput{Writer: &buf, close: func() error {
		text := strings.TrimSuffix(buf.String(), "\n")

		var records [][]byte
		if text != "" {
			for _, line := range strings.Split(text, "\n") {
				records = append(records, []byte(line))
			}
		}

		if _, err := session.CreateRecordFile(loc, rms.TextRecords, records); err != nil {
			return fileFailure(err, name)
		}

		return nil
	}}, nil
}

// isNullDevice reports whether name is the null device, NL: (or NLA0:,
// with or without the "_" of a physical name).
func isNullDevice(name string) bool {
	switch strings.TrimPrefix(strings.ToUpper(name), "_") {
	case "NL:", "NLA0:":
		return true
	}

	return false
}
