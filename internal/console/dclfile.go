package console

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// DCL's file I/O (docs/PHASE-50 - DCL command procedures.md, subtask 17):
// OPEN, READ, WRITE, CLOSE, and INQUIRE, as the DCL Dictionary describes
// them, with what VMS 7.3 showed in testdata/dcl50's probes.
//
//	$ OPEN/READ IN DATA.DAT
//	$ LOOP:
//	$ READ/END_OF_FILE=DONE IN RECORD
//	$ WRITE SYS$OUTPUT "Read: ", RECORD
//	$ GOTO LOOP
//	$ DONE:
//	$ CLOSE IN
//
// OPEN opens a file (rms.Session.OpenDCLFile: on a mounted volume, or on
// the host) and defines a logical name for it in the process table, in
// supervisor mode, CONFINE, equated to the volume's physical device name
// with the node's (TERMINAL: "_GOVAX$DUA0", as VMS 7.3's was
// "_SIMVAX$DUA1", testdata/dcl50's F1; a host file's is its path). The
// logical name is the way to the file: READ, WRITE, CLOSE, and TYPE reach
// it through the name, and once the name is deassigned they can't (VMS
// 7.3, F11), though the file stays open. A file stays open until CLOSE
// closes it, whatever command level opened it: a procedure that ends with
// a file open leaves it open (DCL's files are process-permanent), and
// govax closes what is left when it ends. A name OPEN already has a file
// for gets no other, with no message (the DCL Dictionary; VMS 7.3's
// READ then read the first file, F3).
//
// READ name symbol reads the file's next record into a local symbol of
// the current command level, as it is: no uppercasing, no compression,
// no substitution. WRITE name item[,...] writes one record, the values
// of DCL expressions joined; without /SYMBOL, an item longer than 255
// characters is CLI$_TKNOVF (F10). CLOSE closes the file and deassigns
// its logical name. TYPE of the name types the file's records from where
// READ has got to, and leaves it at the end (VMS 7.3, F2).
//
// The process-permanent files need no OPEN (processPermanentFile, by name
// or through logical names, but not with a colon: VMS 7.3's WRITE
// SYS$OUTPUT: was CLI$_UNDFIL, F6): READ SYS$INPUT reads a command
// procedure's next record, whatever it is (a "$" record, or an EOD, VMS
// 7.3 showed: testdata/dcl50's I6), or the terminal at level 0; READ
// SYS$COMMAND always reads the terminal, prompting with /PROMPT's string
// ("Data:" and a tab without one, Q6); WRITE SYS$OUTPUT writes to the
// procedure's output, and WRITE SYS$ERROR and SYS$COMMAND to the terminal
// (even under /OUTPUT, Q11).
//
// A failure is an RMS status. OPEN's is shown as DCL shows a procedure
// it can't open (VMS 7.3, F3):
//
//	%DCL-E-OPENIN, error opening DUA0:[WORK]NOSUCH.DAT; as input
//	-RMS-E-FNF, file not found
//
// (OPENOUT, "as output", for a new file only), with $STATUS the RMS
// status, its message marked shown (bit 28), even when /ERROR goes to its
// label. READ's and WRITE's are the RMS message alone, and $STATUS the
// RMS status without bit 28 (F2, F4, F5). /ERROR=label goes to the label
// instead, with $STATUS the status and no message, and the level's ON
// action not taken; READ's /END_OF_FILE=label does the same for RMS$_EOF.
// A READ that fails leaves its symbol as it was. Success leaves $STATUS
// %X00000001 after OPEN (and INQUIRE), CLI$_NORMAL after READ (and an
// OPEN passed over), RMS$_NORMAL after WRITE and CLOSE. A READ, WRITE, or
// CLOSE of a name OPEN didn't define is CLI$_UNDFIL, which CLOSE/NOLOG
// doesn't show, its status CLI$_NORMAL then.
//
// INQUIRE symbol [prompt] asks the terminal (SYS$COMMAND) for a value,
// prompting with the prompt, or the symbol's name, and ": "
// (/NOPUNCTUATION leaves that out); the reply is substituted ('X') and
// then treated as a string assignment's value is (dclText), and goes in
// a local symbol, or with /GLOBAL a global one. With procedure
// verification on, the substituted reply is shown on the procedure's
// output (Q1, Q11); at Ctrl/Z the value is "", and nothing is shown.
//
// Not here: indexed files (READ's /INDEX, /KEY, /MATCH, and /DELETE),
// record locking (/NOLOCK is accepted, and nothing is locked), READ's
// /TIME_OUT (accepted; a read waits), WRITE's 1024-byte record buffer, and
// an image's access to a file DCL has open through its logical name.

// The RMS statuses DCL's file I/O reports.
var (
	rmsNormalStatus = vmsdef.Symbols["RMS$_NORMAL"]
	rmsEOFStatus    = vmsdef.Symbols["RMS$_EOF"]
	rmsFACStatus    = vmsdef.Symbols["RMS$_FAC"]
	rmsRTBStatus    = vmsdef.Symbols["RMS$_RTB"]
)

// openSuccess is the status OPEN and INQUIRE leave: %X00000001 (VMS 7.3,
// testdata/dcl50's F1 and Q1).
const openSuccess = 1

// maxWriteItem is the longest value a WRITE item may have without
// /SYMBOL (the DCL Dictionary; VMS 7.3's TKNOVF, F10).
const maxWriteItem = 255

// maxReadRecord is the longest record READ reads: 2048 bytes (the DCL
// Dictionary). A longer one is cut to this length, with RMS$_RTB.
const maxReadRecord = 2048

// dclReadPrompt is READ's prompt at the terminal without /PROMPT: "Data:"
// and a tab, as VMS 7.3 showed it (testdata/dcl50's Q6; the DCL
// Dictionary says "DATA:").
const dclReadPrompt = "Data:\t"

// dclStream is an open DCL file's records: an rms.DCLFile, or the null
// device (nullStream).
type dclStream interface {
	Get() ([]byte, uint32)
	Put(record []byte) uint32
	Update(record []byte) uint32
	Close() error
}

// nullStream is NL:, opened by OPEN: every read is at its end of file,
// and what is written goes nowhere.
type nullStream struct{}

func (nullStream) Get() ([]byte, uint32) { return nil, rmsEOFStatus }
func (nullStream) Put([]byte) uint32     { return rmsNormalStatus }
func (nullStream) Update([]byte) uint32  { return rmsNormalStatus }
func (nullStream) Close() error          { return nil }

// dclOpenFile is a file OPEN has open: its logical name and the
// equivalence string OPEN gave it, the file's full name, and its records.
type dclOpenFile struct {
	logical     string
	equivalence string
	name        string
	stream      dclStream
}

// dclFileName is a logical name as OPEN, READ, WRITE, and CLOSE take it:
// uppercase, without a colon after it.
func dclFileName(text string) string {
	return strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(text)), ":")
}

// openFile returns the file OPEN has open under the logical name, if any:
// one whose logical name is still OPEN's, in the process table in
// supervisor mode, equated to what OPEN equated it to. A file whose name
// was deassigned (or redefined) is out of reach, though still open
// (orphanFile).
func (c *Interpreter) openFile(name string) (*dclOpenFile, bool) {
	key := dclFileName(name)

	f, ok := c.dclFiles[key]
	if !ok {
		return nil, false
	}

	if c.Logicals != nil {
		e, err := c.Logicals.Translate(lnm.ProcessTableName, key, lnm.Supervisor, 0)
		if err != nil || e.Mode != lnm.Supervisor || len(e.Equivalences) == 0 || e.Equivalences[0].Value != f.equivalence {
			c.orphanFile(key, f)

			return nil, false
		}
	}

	return f, true
}

// orphanFile takes f, whose logical name is gone, out of reach: it stays
// open, as on VMS, until govax ends.
func (c *Interpreter) orphanFile(key string, f *dclOpenFile) {
	delete(c.dclFiles, key)
	c.dclOrphans = append(c.dclOrphans, f)
}

// rmsStatusFailure is the failure for a record operation that ended with
// RMS status v: its message alone, and v as $STATUS, without bit 28 (VMS
// 7.3, testdata/dcl50's F2 and F4). RMS$_RTB's message has the record's
// length, n.
func rmsStatusFailure(v uint32, n int) error {
	text := messageText(v)
	if v == rmsRTBStatus {
		text = strings.Replace(text, "!UL", strconv.Itoa(n), 1)
	}

	return vmserrors.ConditionError{Status: v, Text: text}
}

// branch is an /ERROR or /END_OF_FILE label taken for a command that
// failed with condition value v: the procedure goes on at the label
// (Goto), with $STATUS v and no message, and the level's ON action isn't
// taken. A label that isn't there is Goto's failure (CLI$_USGOTO), as is
// a branch at the terminal.
func (c *Interpreter) branch(label string, v uint32, cause error) error {
	if err := c.Goto(label); err != nil {
		return err
	}

	c.branchStatus(v, cause)

	return nil
}

// failOrBranch reports a command's failure, condition value v with err
// its message: to the label, if there is one (branch), or as err.
func (c *Interpreter) failOrBranch(label string, v uint32, err error) error {
	if label != "" {
		return c.branch(label, v, err)
	}

	return err
}

// qualifierLabel is the label an /ERROR or /END_OF_FILE qualifier names,
// "" without the qualifier.
func qualifierLabel(r *dcl.Result, name string) (string, error) {
	if !r.Present(name) {
		return "", nil
	}

	return labelParameter(r.String(name))
}

// terminalOut is where the terminal's prompts go: the terminal itself
// (SYS$COMMAND), even while a procedure's output goes to an /OUTPUT
// file.
func (c *Interpreter) terminalOut() io.Writer {
	if level := c.currentLevel(); level != nil && level.redirected && level.sysCommand != nil {
		return level.sysCommand
	}

	return c.Out
}

// readTerminal shows prompt on the terminal and reads a line from it:
// SYS$COMMAND, which a command procedure's data doesn't replace. ok is
// false at its end of file (Ctrl/Z).
func (c *Interpreter) readTerminal(prompt string) (string, bool) {
	if out := c.terminalOut(); out != nil && prompt != "" {
		fmt.Fprint(out, prompt)
	}

	return c.readReply()
}

// readRecord reads the procedure's next record, whatever it is: READ
// SYS$INPUT's record in a command procedure. ok is false at the
// procedure's end of file (a CALL subroutine's, its ENDSUBROUTINE).
func (p *procedureSource) readRecord() (string, bool) {
	if p.next >= p.end {
		return "", false
	}

	record := p.records[p.next]
	p.next++

	return record, true
}

// openCommand is OPEN[/READ][/WRITE][/APPEND][/SHARE[=option]]
// [/ERROR=label] logical-name filespec.
func (c *Interpreter) openCommand(id int64, r *dcl.Result) error {
	label, err := qualifierLabel(r, "ERROR")
	if err != nil {
		return err
	}

	logical, spec := dclFileName(r.String("LOGICAL_NAME")), strings.TrimSpace(r.String("FILESPEC"))
	if logical == "" || spec == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	access := rms.DCLAccess{
		Read:   r.Present("READ") && !r.Negated("READ"),
		Write:  r.Present("WRITE") && !r.Negated("WRITE"),
		Append: r.Present("APPEND") && !r.Negated("APPEND"),
	}

	// VMS 7.3 named /WRITE as the conflict (testdata/dcl50's F3).
	if access.Write && access.Append {
		return vmserrors.NewSegment(vmserrors.CLI_CONFLICT, "WRITE")
	}

	if r.Present("SHARE") && !r.Defaulted("SHARE") {
		switch option := strings.ToUpper(strings.TrimSpace(r.String("SHARE"))); {
		case option == "":
			access.ShareWrite = true
		case strings.HasPrefix("READ", option):
			access.ShareRead = true
		case strings.HasPrefix("WRITE", option):
			access.ShareWrite = true
		default:
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, option)
		}
	}

	// The DCL Dictionary: a logical name that already names a file OPEN
	// opened gets no other, with no message.
	if _, open := c.openFile(logical); open {
		c.setCommandStatus(cliNormal, nil)

		return nil
	}

	file, equivalence, err := c.openDCLFile(spec, r.ParamPresent("FILESPEC", "HOST"), access)
	if err != nil {
		var fe *rms.DCLFileError
		if errors.As(err, &fe) {
			return c.failOrBranch(label, fe.Status|stsInhibitMsg, openFailure(fe, access))
		}

		return err
	}

	eqv := []lnm.Equivalence{{Value: equivalence, Attrs: lnm.AttrTerminal}}
	if _, err := c.Logicals.Define(lnm.ProcessTableName, logical, lnm.Supervisor, lnm.AttrConfine, eqv); err != nil {
		_ = file.stream.Close()

		return err
	}

	file.logical, file.equivalence = logical, equivalence

	if c.dclFiles == nil {
		c.dclFiles = map[string]*dclOpenFile{}
	}

	c.dclFiles[logical] = file
	c.setCommandStatus(openSuccess, nil)

	return nil
}

// openDCLFile opens the file spec names for OPEN, host being an explicit
// /HOST on it, and returns it with the equivalence string its logical
// name gets: the device's physical name with the node's, without a
// colon ("_GOVAX$DUA0"). A file on the host whose name has no type gets
// .DAT, as name processing gives a volume file; NL: is the null device.
func (c *Interpreter) openDCLFile(spec string, host bool, access rms.DCLAccess) (*dclOpenFile, string, error) {
	if !host && isNullDevice(spec) {
		return &dclOpenFile{name: "_NLA0:", stream: nullStream{}}, c.physicalDevice("NLA0"), nil
	}

	loc, err := c.ContainerSession.Locate(spec, host)
	if err != nil {
		return nil, "", dclcmd.FileFailure(err, spec)
	}

	if loc.Host {
		loc = loc.WithDefaultType("DAT")
	}

	f, err := c.ContainerSession.OpenDCLFile(loc, access)
	if err != nil {
		return nil, "", err
	}

	equivalence := f.Name
	if !loc.Host {
		equivalence = c.physicalDevice(f.Device)
	}

	return &dclOpenFile{name: f.Name, stream: f}, equivalence, nil
}

// physicalDevice is device's physical name with the node's, as VMS 7.3
// equated OPEN's logical name to it: "_GOVAX$DUA0".
func (c *Interpreter) physicalDevice(device string) string {
	if c.RTL == nil || c.RTL.NodeName == "" {
		return "_" + device
	}

	return "_" + c.RTL.NodeName + "$" + device
}

// openFailure is OPEN's message for a file it couldn't open, as DCL
// shows @'s: OPENIN, or OPENOUT for a new file, then the RMS status's
// message and its secondary status's, the status marked shown.
func openFailure(fe *rms.DCLFileError, access rms.DCLAccess) error {
	head := "DCL-E-OPENIN, error opening " + fe.Name + " as input"
	if access.Output() {
		head = "DCL-E-OPENOUT, error opening " + fe.Name + " as output"
	}

	text := head + "\n" + dclcmd.ConditionLine(fe.Status)
	if fe.STV != 0 {
		text += "\n" + dclcmd.ConditionLine(fe.STV)
	}

	return vmserrors.ConditionError{Status: fe.Status | stsInhibitMsg, Text: text}
}

// closeCommand is CLOSE[/ERROR=label][/[NO]LOG] logical-name.
func (c *Interpreter) closeCommand(id int64, r *dcl.Result) error {
	label, err := qualifierLabel(r, "ERROR")
	if err != nil {
		return err
	}

	logical := dclFileName(r.String("LOGICAL_NAME"))
	if logical == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	f, open := c.openFile(logical)
	if !open {
		undefined := vmserrors.New(vmserrors.CLI_UNDFIL)

		switch {
		case label != "":
			return c.branch(label, conditionValue(undefined), undefined)
		case r.Negated("LOG"):
			c.setCommandStatus(cliNormal, nil)

			return nil
		}

		return undefined
	}

	delete(c.dclFiles, logical)
	_, _ = c.Logicals.Delete(lnm.ProcessTableName, logical, lnm.Supervisor)

	if err := f.stream.Close(); err != nil {
		return c.failOrBranch(label, vmsdef.Symbols["RMS$_DEV"], vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, f.name))
	}

	c.setCommandStatus(rmsNormalStatus, nil)

	return nil
}

// readCommand is READ[/END_OF_FILE=label][/ERROR=label][/PROMPT=string]
// logical-name symbol-name.
func (c *Interpreter) readCommand(id int64, r *dcl.Result) error {
	errorLabel, err := qualifierLabel(r, "ERROR")
	if err != nil {
		return err
	}

	eofLabel, err := qualifierLabel(r, "END_OF_FILE")
	if err != nil {
		return err
	}

	logical, symbol := dclFileName(r.String("LOGICAL_NAME")), strings.ToUpper(strings.TrimSpace(r.String("SYMBOL")))
	if logical == "" || symbol == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	if !isDCLSymbolName(symbol) {
		return vmserrors.NewSegment(vmserrors.CLI_IVSYMB, symbol)
	}

	prompt := dclReadPrompt
	if r.Present("PROMPT") {
		prompt = r.String("PROMPT")
	}

	record, sts, err := c.readDCLRecord(r.String("LOGICAL_NAME"), prompt)
	if err != nil {
		return c.failOrBranch(errorLabel, conditionValue(err), err)
	}

	switch {
	case sts == rmsEOFStatus && eofLabel != "":
		return c.branch(eofLabel, sts, nil)
	case sts == rmsRTBStatus:
		c.dclSymbols.local()[symbol] = dclSymbol{name: symbol, minLength: len(symbol), value: record[:maxReadRecord]}

		return c.failOrBranch(errorLabel, sts, rmsStatusFailure(sts, len(record)))
	case sts&1 == 0:
		return c.failOrBranch(errorLabel, sts, rmsStatusFailure(sts, 0))
	}

	c.dclSymbols.local()[symbol] = dclSymbol{name: symbol, minLength: len(symbol), value: record}
	c.setCommandStatus(cliNormal, nil)

	return nil
}

// readDCLRecord reads READ's record from the file logical names (as
// typed): one OPEN opened, or a process-permanent file (see the top of this file). It
// returns the record and RMS$_NORMAL, or the RMS status for why there is
// none (RMS$_EOF at the end of the file, RMS$_RTB with the whole record
// for one too long), or err for a name that has no file.
func (c *Interpreter) readDCLRecord(logical, prompt string) (string, uint32, error) {
	var (
		record string
		sts    = rmsNormalStatus
	)

	if f, open := c.openFile(logical); open {
		data, s := f.stream.Get()
		record, sts = string(data), s
	} else {
		switch ppf := c.processPermanentName(logical); {
		case ppf == "":
			return "", 0, vmserrors.New(vmserrors.CLI_UNDFIL)
		case ppf == "SYS$INPUT" && c.currentLevel() != nil && !c.currentLevel().source.plain:
			line, ok := c.currentLevel().source.readRecord()
			if !ok {
				return "", rmsEOFStatus, nil
			}

			record = line
		default:
			line, ok := c.readTerminal(prompt)
			if !ok {
				return "", rmsEOFStatus, nil
			}

			record = line
		}
	}

	if sts&1 != 0 && len(record) > maxReadRecord {
		sts = rmsRTBStatus
	}

	return record, sts, nil
}

// isDCLSymbolName reports whether name can be a DCL symbol's name: 1 to
// 255 letters, digits, "$", and "_", not starting with a digit (the DCL
// Dictionary's READ).
func isDCLSymbolName(name string) bool {
	if name == "" || len(name) > 255 || !isDCLSymbolStart(name[0]) {
		return false
	}

	for i := 1; i < len(name); i++ {
		if !isDCLSymbolChar(name[i]) {
			return false
		}
	}

	return true
}

// writeCommand is WRITE[/ERROR=label][/SYMBOL][/UPDATE] logical-name
// expression[,...].
func (c *Interpreter) writeCommand(id int64, r *dcl.Result) error {
	label, err := qualifierLabel(r, "ERROR")
	if err != nil {
		return err
	}

	logical, data := dclFileName(r.String("LOGICAL_NAME")), strings.TrimSpace(r.String("DATA"))
	if logical == "" || data == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	record, err := c.writeRecord(data, r.Present("SYMBOL"))
	if err != nil {
		return err
	}

	sts := rmsNormalStatus

	if f, open := c.openFile(logical); open {
		if r.Present("UPDATE") {
			sts = f.stream.Update([]byte(record))
		} else {
			sts = f.stream.Put([]byte(record))
		}
	} else {
		switch c.processPermanentName(r.String("LOGICAL_NAME")) {
		case "":
			undefined := vmserrors.New(vmserrors.CLI_UNDFIL)

			return c.failOrBranch(label, conditionValue(undefined), undefined)
		case "SYS$OUTPUT":
			c.Printf("%s\n", record)
		case "SYS$INPUT":
			if c.currentLevel() != nil {
				sts = rmsFACStatus
			} else {
				fmt.Fprintf(c.terminalOut(), "%s\n", record)
			}
		default:
			fmt.Fprintf(c.terminalOut(), "%s\n", record)
		}
	}

	if sts&1 == 0 {
		return c.failOrBranch(label, sts, rmsStatusFailure(sts, 0))
	}

	c.setCommandStatus(rmsNormalStatus, nil)

	return nil
}

// processPermanentName is processPermanentFile for READ and WRITE's
// logical-name parameter, which VMS 7.3 didn't take with a colon after it
// (WRITE SYS$OUTPUT: was CLI$_UNDFIL, testdata/dcl50's F6).
func (c *Interpreter) processPermanentName(name string) string {
	if strings.HasSuffix(strings.TrimSpace(name), ":") {
		return ""
	}

	return c.processPermanentFile(name)
}

// writeRecord is WRITE's record: the values of its data's expressions,
// separated by commas, one after another, an integer's in decimal.
// Without symbol (/SYMBOL), an item's value may be no longer than 255
// characters: a longer one is CLI$_TKNOVF, with the first 255 as its
// segment, as VMS 7.3 showed it (testdata/dcl50's F10).
func (c *Interpreter) writeRecord(data string, symbol bool) (string, error) {
	var b strings.Builder

	for _, item := range splitDCLList(data) {
		v, err := evaluateDCLExpression(item, &c.dclSymbols, c)
		if err != nil {
			return "", err
		}

		text := v.String()
		if !symbol && len(text) > maxWriteItem {
			return "", vmserrors.NewSegment(vmserrors.CLI_TKNOVF, text[:maxWriteItem])
		}

		b.WriteString(text)
	}

	return b.String(), nil
}

// splitDCLList splits text at the commas that separate a list's items:
// those outside quotes and parentheses.
func splitDCLList(text string) []string {
	var (
		items  []string
		depth  int
		quoted bool
		start  int
	)

	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case ch == '"':
			quoted = !quoted
		case quoted:
		case ch == '(':
			depth++
		case ch == ')':
			depth = max(depth-1, 0)
		case ch == ',' && depth == 0:
			items = append(items, strings.TrimSpace(text[start:i]))
			start = i + 1
		}
	}

	return append(items, strings.TrimSpace(text[start:]))
}

// inquireCommand is INQUIRE[/GLOBAL|/LOCAL][/[NO]PUNCTUATION] symbol
// [prompt].
func (c *Interpreter) inquireCommand(id int64, r *dcl.Result) error {
	symbol := strings.ToUpper(strings.TrimSpace(r.String("SYMBOL")))
	if symbol == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	if !isDCLSymbolName(symbol) {
		return vmserrors.NewSegment(vmserrors.CLI_IVSYMB, symbol)
	}

	prompt := dclText(r.String("PROMPT"), false)
	if prompt == "" {
		prompt = symbol
	}

	if !r.Negated("PUNCTUATION") {
		prompt += ": "
	}

	reply, ok := c.readTerminal(prompt)

	reply, err := c.substituteApostrophes(reply)
	if err != nil {
		return err
	}

	// Procedure verification shows the reply, substituted, on the
	// procedure's output (VMS 7.3, testdata/dcl50's Q1 and Q11).
	if ok && c.Verify && c.currentLevel() != nil {
		fmt.Fprintln(c.Out, reply)
	}

	table := c.dclSymbols.local()
	if r.Present("GLOBAL") {
		table = c.dclSymbols.globals()
	}

	table[symbol] = dclSymbol{name: symbol, minLength: len(symbol), value: dclText(reply, false), global: r.Present("GLOBAL")}
	c.setCommandStatus(openSuccess, nil)

	return nil
}

// typeOpenFile is TYPE of a logical name OPEN defined: it types the
// file's records from where READ has got to, through the same stream, so
// a READ after it is at the end of the file, as VMS 7.3's was
// (testdata/dcl50's F2). ok is false for a name that isn't one.
func (c *Interpreter) typeOpenFile(spec string) (ok bool) {
	f, open := c.openFile(spec)
	if !open {
		return false
	}

	for c.Running() {
		record, sts := f.stream.Get()
		if sts&1 == 0 {
			break
		}

		c.Printf("%s\n", record)
	}

	return true
}

// closeDCLFiles closes every file OPEN left open, those whose logical
// names are gone too, as the process's rundown does on VMS: what they
// wrote goes to their volumes.
func (c *Interpreter) closeDCLFiles() {
	for logical, f := range c.dclFiles {
		_ = f.stream.Close()

		delete(c.dclFiles, logical)
	}

	for _, f := range c.dclOrphans {
		_ = f.stream.Close()
	}

	c.dclOrphans = nil
}
