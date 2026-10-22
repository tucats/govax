package dclshell

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
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
// the host) as a process-permanent file (docs/PHASE-71 - process-permanent
// files.md, subtask 7), and defines a logical name for it in the process
// table, in supervisor mode, CONFINE, TERMINAL, as the file's
// process-permanent string: ESC, 0, its IFI word, and the volume's
// physical device name with the node's, without a colon ("_GOVAX$DUA0",
// as VMS 7.1's was "_SIMVAX$DUA1", testdata/ppf71; a host file's is its
// path), which SHOW LOGICAL shows. The word carries the record attributes
// of DCL's open: none for OPEN/READ (%X8005), PRN for an open that writes
// (%X8106). The logical name is the way to the file: READ, WRITE, CLOSE,
// and TYPE reach it through the name (by the IFI in its string), and once
// the name is deassigned they can't (VMS 7.3, F11), though the file stays
// open. An image reaches it the same way, an indirect $OPEN or $CREATE of
// the name (rms's ppfaccess.go), and reads or writes the one stream DCL
// does, in turn (VMS 7.1, testdata/ppf71's part F). A file stays open
// until CLOSE closes it, whatever command level opened it: a procedure
// that ends with a file open leaves it open, and govax closes what is
// left when it ends. A name OPEN already has a file for gets no other,
// with no message (the DCL Dictionary; VMS 7.3's READ then read the first
// file, F3).
//
// READ name symbol reads the file's next record into a local symbol of
// the current command level, as it is: no uppercasing, no compression,
// no substitution. WRITE name item[,...] writes one record, the values
// of DCL expressions joined; without /SYMBOL, an item longer than 255
// characters is CLI$_TKNOVF (F10). CLOSE closes the file and deassigns
// its logical name. TYPE of the name types the file's records from where
// READ has got to, and leaves it at the end (VMS 7.3, F2).
//
// The other process-permanent files need no OPEN: READ and WRITE find
// them as they find OPEN's, by the IFI in the string a name leads to,
// through logical names too, but not with a colon (VMS 7.3's WRITE
// SYS$OUTPUT: was CLI$_UNDFIL, F6). READ SYS$INPUT in a command procedure
// reads the procedure's next record, whatever it is (a "$" record, or an
// EOD, VMS 7.3 showed: testdata/dcl50's I6); READ of the terminal's
// entries (SYS$INPUT at level 0, SYS$COMMAND) reads the terminal,
// prompting with /PROMPT's string ("Data:" and a tab without one, Q6);
// WRITE SYS$OUTPUT writes to the procedure's /OUTPUT file or the
// terminal, and WRITE SYS$ERROR and SYS$COMMAND to the terminal (even
// under /OUTPUT, Q11). A name that leads to no entry of the process's
// (a subprocess's SYS$OUTPUT that names a file: subtask 8 makes those
// entries) is still found by its name (processPermanentFile).
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
// /TIME_OUT (accepted; a read waits), and WRITE's 1024-byte record
// buffer.

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

// dclOpenFile is a file OPEN has open: its logical name, the file's full
// name, its records as DCL reads and writes them, and the
// process-permanent file it is (entry, in env's table), whose stream is
// the same one, so DCL and an image share its position.
type dclOpenFile struct {
	logical string
	name    string
	stream  dclStream
	entry   *rms.PermanentEntry
	env     *corevms.Environment
}

// dclFileName is a logical name as OPEN, READ, WRITE, and CLOSE take it:
// uppercase, without a colon after it.
func dclFileName(text string) string {
	return strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(text)), ":")
}

// openFile returns the file OPEN has open that the logical name leads to
// (dclFileName's form of it), if any: by the IFI in the process-permanent
// string the name translates to. A file whose name was deassigned (or
// redefined) is out of reach, though still open.
func (c *Interpreter) openFile(name string) (*dclOpenFile, bool) {
	e, ok := c.permanentFile(dclFileName(name))
	if !ok {
		return nil, false
	}

	f, ok := c.dclFiles[e]

	return f, ok
}

// permanentFile returns the process-permanent file of the process's that
// name leads to, by RMS's translation of a file specification, and
// whether there is one. A name that leads to more than the file's device
// ("SYS$INPUT:X.DAT", or a name for it) names a file, not the
// process-permanent one, to DCL.
func (c *Interpreter) permanentFile(name string) (*rms.PermanentEntry, bool) {
	if c.RTL == nil || c.Logicals == nil || name == "" {
		return nil, false
	}

	specs, err := c.Logicals.TranslateFileSpec(name, lnm.User)
	if err != nil || len(specs) == 0 || specs[0].Permanent == nil {
		return nil, false
	}

	if pf := specs[0].Permanent; strings.TrimSuffix(specs[0].Spec, ":") != strings.TrimSuffix(pf.Device, ":") {
		return nil, false
	}

	return c.RTL.PermanentEntryFor(specs[0].Permanent.IFI)
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
// false at its end of file (Ctrl/Z). A subprocess's DCL reads its
// process's terminal, as it reads its commands (subcli.go's
// readTerminal).
func (c *Interpreter) readTerminal(prompt string) (string, bool) {
	if cli := c.subCLI; cli != nil {
		return cli.readTerminal(prompt)
	}

	if out := c.terminalOut(); out != nil && prompt != "" {
		fmt.Fprint(out, prompt)
	}

	return c.ReadReply()
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
		c.SetCommandStatus(CLINormal, nil)

		return nil
	}

	if c.RTL == nil {
		return errors.New("OPEN needs a process to open the file in")
	}

	file, err := c.openDCLFile(spec, r.ParamPresent("FILESPEC", "HOST"), access)
	if err != nil {
		var fe *rms.DCLFileError
		if errors.As(err, &fe) {
			return c.failOrBranch(label, fe.Status|stsInhibitMsg, openFailure(fe, access))
		}

		return err
	}

	eqv := []lnm.Equivalence{{Value: file.entry.EquivalenceString(), Attrs: lnm.AttrTerminal}}
	if _, err := c.Logicals.Define(lnm.ProcessTableName, logical, lnm.Supervisor, lnm.AttrConfine, eqv); err != nil {
		_ = file.close()

		return err
	}

	file.logical = logical

	if c.dclFiles == nil {
		c.dclFiles = map[*rms.PermanentEntry]*dclOpenFile{}
	}

	c.dclFiles[file.entry] = file
	c.SetCommandStatus(openSuccess, nil)

	return nil
}

// openDCLFile opens the file spec names for OPEN, host being an explicit
// /HOST on it, as a process-permanent file of the process's (openEntry).
// A file on the host whose name has no type gets .DAT, as name
// processing gives a volume file; NL: is the null device.
func (c *Interpreter) openDCLFile(spec string, host bool, access rms.DCLAccess) (*dclOpenFile, error) {
	if !host && isNullDevice(spec) {
		return c.openEntry(&dclOpenFile{name: "_NLA0:", stream: nullStream{}}, nil, access)
	}

	loc, err := c.ContainerSession.Locate(spec, host)
	if err != nil {
		return nil, dclcmd.FileFailure(err, spec)
	}

	if loc.Host {
		loc = loc.WithDefaultType("DAT")
	}

	f, err := c.ContainerSession.OpenDCLFile(loc, access)
	if err != nil {
		return nil, err
	}

	return c.openEntry(&dclOpenFile{name: f.Name, stream: f}, f, access)
}

// openEntry makes file, which OPEN opened (f; nil for the null device), a
// process-permanent file, its access OPEN's FAB$B_FAC, its record
// attributes PRN for an open that writes and none for one that reads
// (VMS 7.1's words, testdata/ppf71; /APPEND's and /READ/WRITE's
// unconfirmed), and its device text the device's physical name with the
// node's, without a colon ("_GOVAX$DUA0"), or a host file's path. On a
// failure the file is closed.
func (c *Interpreter) openEntry(file *dclOpenFile, f *rms.DCLFile, access rms.DCLAccess) (*dclOpenFile, error) {
	fac := access.FAC()

	var opts rms.PermanentOptions
	if access.Write || access.Append {
		opts.RAT = fabRATPRN
	}

	var err error

	switch {
	case f == nil:
		opts.Device = c.physicalDevice("NLA0")
		file.entry, err = c.RTL.AddPermanentStream(&rms.RecordStream{}, fac, opts)
	case f.Device == "":
		opts.Device = f.Name
		file.entry, err = c.RTL.AddPermanentDCLFile(f, fac, opts)
	default:
		opts.Device = c.physicalDevice(f.Device)
		file.entry, err = c.RTL.AddPermanentDCLFile(f, fac, opts)
	}

	if err != nil {
		_ = file.stream.Close()

		return nil, err
	}

	file.env = c.RTL

	return file, nil
}

// close closes f's process-permanent file, and with it the file: f's
// stream is the entry's.
func (f *dclOpenFile) close() error {
	return f.env.ClosePermanentFile(f.entry)
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
			return c.branch(label, ConditionValue(undefined), undefined)
		case r.Negated("LOG"):
			c.SetCommandStatus(CLINormal, nil)

			return nil
		}

		return undefined
	}

	delete(c.dclFiles, f.entry)
	_, _ = c.Logicals.Delete(lnm.ProcessTableName, logical, lnm.Supervisor)

	if err := f.close(); err != nil {
		return c.failOrBranch(label, vmsdef.Symbols["RMS$_DEV"], vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, f.name))
	}

	c.SetCommandStatus(rmsNormalStatus, nil)

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
		return c.failOrBranch(errorLabel, ConditionValue(err), err)
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
	c.SetCommandStatus(CLINormal, nil)

	return nil
}

// readDCLRecord reads READ's record from the file logical names (as
// typed): one OPEN opened, or another process-permanent file (see the top
// of this file). It returns the record and RMS$_NORMAL, or the RMS status
// for why there is none (RMS$_EOF at the end of the file, RMS$_RTB with
// the whole record for one too long), or err for a name that has no
// file.
func (c *Interpreter) readDCLRecord(logical, prompt string) (string, uint32, error) {
	var (
		record string
		sts    = rmsNormalStatus
	)

	switch f, e, ppf := c.dclTarget(logical); {
	case f != nil:
		data, s := f.stream.Get()
		record, sts = string(data), s
	case e != nil && e == c.procedureInput, e == nil && ppf == "SYS$INPUT":
		level := c.currentLevel()
		if level == nil || level.source.plain {
			return c.readTerminalRecord(prompt)
		}

		line, ok := level.source.readRecord()
		if !ok {
			return "", rmsEOFStatus, nil
		}

		record = line
	case e != nil && !e.IsTerminal():
		data, s := c.RTL.GetPermanent(e)
		record, sts = string(data), s
	case e != nil, ppf != "":
		return c.readTerminalRecord(prompt)
	default:
		return "", 0, vmserrors.New(vmserrors.CLI_UNDFIL)
	}

	if sts&1 != 0 && len(record) > maxReadRecord {
		sts = rmsRTBStatus
	}

	return record, sts, nil
}

// readTerminalRecord is READ of the terminal: a line, prompted for,
// RMS$_EOF at its end of file (Ctrl/Z).
func (c *Interpreter) readTerminalRecord(prompt string) (string, uint32, error) {
	line, ok := c.readTerminal(prompt)
	if !ok {
		return "", rmsEOFStatus, nil
	}

	if len(line) > maxReadRecord {
		return line, rmsRTBStatus, nil
	}

	return line, rmsNormalStatus, nil
}

// dclTarget is what READ and WRITE find for the logical name they were
// given, as typed: a file OPEN opened (f, with its entry e), another of
// the process's process-permanent files (e alone; not with a colon after
// the name), or, for a name that leads to no entry, which of the four
// process-permanent names it leads to by name (ppf; processPermanentFile).
func (c *Interpreter) dclTarget(logical string) (f *dclOpenFile, e *rms.PermanentEntry, ppf string) {
	if f, open := c.openFile(logical); open {
		return f, f.entry, ""
	}

	if strings.HasSuffix(strings.TrimSpace(logical), ":") {
		return nil, nil, ""
	}

	if e, ok := c.permanentFile(strings.TrimSpace(logical)); ok {
		return nil, e, ""
	}

	return nil, nil, c.processPermanentFile(logical)
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

	switch f, e, ppf := c.dclTarget(r.String("LOGICAL_NAME")); {
	case f != nil && r.Present("UPDATE"):
		sts = f.stream.Update([]byte(record))
	case f != nil:
		sts = f.stream.Put([]byte(record))
	case e != nil && e == c.levelOutput(), e == nil && ppf == "SYS$OUTPUT":
		// The procedure's own output, where DCL's lines go too.
		c.Printf("%s\n", record)
	case e != nil && e.IsTerminal():
		fmt.Fprintf(c.terminalOut(), "%s\n", record)
	case e != nil:
		if sts, err = c.RTL.PutPermanent(e, record); err != nil {
			sts = rmsWER
		}
	case ppf == "SYS$INPUT" && c.currentLevel() != nil:
		sts = rmsFACStatus
	case ppf != "":
		fmt.Fprintf(c.terminalOut(), "%s\n", record)
	default:
		undefined := vmserrors.New(vmserrors.CLI_UNDFIL)

		return c.failOrBranch(label, ConditionValue(undefined), undefined)
	}

	if sts&1 == 0 {
		return c.failOrBranch(label, sts, rmsStatusFailure(sts, 0))
	}

	c.SetCommandStatus(rmsNormalStatus, nil)

	return nil
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
	c.SetCommandStatus(openSuccess, nil)

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

// CloseDCLFiles closes every file OPEN left open, those whose logical
// names are gone too, as the process's rundown does on VMS: what they
// wrote goes to their volumes.
func (c *Interpreter) CloseDCLFiles() {
	for e, f := range c.dclFiles {
		_ = f.close()

		delete(c.dclFiles, e)
	}
}

// levelOutput is the /OUTPUT file of the innermost level that has one,
// the entry the supervisor-mode SYS$OUTPUT names (defineProcedureNames);
// nil when there is none.
func (c *Interpreter) levelOutput() *rms.PermanentEntry {
	for i := len(c.levels) - 1; i >= 0; i-- {
		if out := c.levels[i].output; out != nil && out.entry != nil {
			return out.entry
		}
	}

	return nil
}
