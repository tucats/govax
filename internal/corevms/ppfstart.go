package corevms

import (
	"io"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The process-permanent files a process starts with (docs/PHASE-71 -
// process-permanent files.md, subtask 5).
//
// LOGINOUT opens SYS$INPUT, SYS$OUTPUT, SYS$ERROR, and SYS$COMMAND as
// process-permanent files (the Guide to OpenVMS File Applications, 6.6),
// kept in RMS's table of the process's files (rms's ppf.go), and defines
// each name, in executive mode, as the string that leads to its entry:
// ESC, 0, the entry's IFI word, and the device's name with the node's
// ("_GOVAX$TTA0:"). What VMS 7.1 showed of them (testdata/ppf71):
//
//   - SYS$INPUT and SYS$COMMAND share one entry when they name the same
//     device, and so do SYS$OUTPUT and SYS$ERROR.
//   - The names are TERMINAL; at the console terminal they are also
//     CONCEALED, in a subprocess not.
//   - Each entry's word carries the PRN record attribute of the open that
//     made it, and the files are sequential only (SQO).
//   - The output entry's access is GET and PUT, as $DISPLAY of SYS$OUTPUT
//     showed. A terminal's input entry has PUT too: a FORTRAN program
//     writes its unit 5 by a $CREATE of SYS$INPUT and $PUTs to it, which
//     VMS 7.1 allowed at the terminal (testdata/legacy72's DUNGEON run;
//     Phase 72). Another input entry's access is GET, as a procedure's
//     was (testdata/ppf71, I9).
//   - VMS numbered process 1's input entry 1 and its output entry 3; govax
//     numbers them lowest free first, so its output entry is 2 (the
//     oracle masks the numbers).
//
// A created process's values may name a terminal, a mailbox or NL:, or a
// file (a SPAWN's /INPUT and /OUTPUT, $CREPRC's input and output): each is
// opened as an entry of the process's own (openStartupEntry), as VMS 7.1's
// subprocess's log and NL: were (testdata/ppf71: SYS$OUTPUT and SYS$ERROR
// one entry on the log, SYS$INPUT and SYS$COMMAND one on _NLA0:).

// fabRATPRN is FAB$M_PRN, the print file record attribute LOGINOUT's
// files carry.
var fabRATPRN = byte(vmsdef.Symbols["FAB$M_PRN"])

// OpenTerminalFiles opens the process's terminal as its four
// process-permanent files, as LOGINOUT does for an interactive process,
// and defines SYS$INPUT, SYS$OUTPUT, SYS$ERROR, and SYS$COMMAND (executive
// mode, CONCEALED and TERMINAL) as their strings: process 1's, at the
// console terminal. The terminal needn't be in the device table yet
// (vax.init defines TTA0 after VMINIT makes process 1).
func (env *Environment) OpenTerminalFiles() error {
	device := strings.Trim(env.Process.Terminal, "_:")
	terminal := "_" + device + ":"

	return env.openProcessFiles(terminal, terminal, terminal, lnm.AttrConcealed|lnm.AttrTerminal,
		func(_ string, fac byte) *rms.PermanentEntry { return env.openTerminalEntry(device, fac) })
}

// openProcessFiles is process startup's step for SYS$INPUT, SYS$OUTPUT,
// and SYS$ERROR, given input, output, and errorOutput (each "" when not
// given, when the name isn't defined): open makes each value an entry,
// shared as this file's opening comment says, and the name is defined as
// its string with attrs, and SYS$COMMAND with SYS$INPUT's. A value open
// can't make an entry of (nil) defines the name as given.
func (env *Environment) openProcessFiles(input, output, errorOutput string, attrs uint32,
	open func(value string, fac byte) *rms.PermanentEntry,
) error {
	entry := func(value string, fac byte) *rms.PermanentEntry {
		if value == "" {
			return nil
		}

		return open(value, fac)
	}

	in := entry(input, fabFACGet)
	out := entry(output, fabFACGet|fabFACPut)

	errEntry := out
	if !sameDevice(errorOutput, output) {
		errEntry = entry(errorOutput, fabFACGet|fabFACPut)
	}

	names := []startupName{
		{"SYS$INPUT", input, in},
		{"SYS$OUTPUT", output, out},
		{"SYS$ERROR", errorOutput, errEntry},
	}

	if in != nil {
		names = append(names, startupName{"SYS$COMMAND", input, in})
	}

	for _, n := range names {
		if n.value == "" {
			continue
		}

		eqv := lnm.Equivalence{Value: n.value}
		if n.entry != nil {
			eqv = lnm.Equivalence{Value: n.entry.EquivalenceString(), Attrs: attrs}
		}

		if _, err := env.Logicals.Define(lnm.ProcessTableName, n.name, lnm.Executive, 0, []lnm.Equivalence{eqv}); err != nil {
			return err
		}
	}

	return nil
}

// startupName is one of the SYS$ names process startup defines: its
// value as given, and the entry opened for it, if any.
type startupName struct {
	name, value string
	entry       *rms.PermanentEntry
}

// sameDevice reports whether two of the SYS$ names' values name the same
// thing, the case of their letters aside.
func sameDevice(a, b string) bool {
	return strings.EqualFold(lnm.DisplayValue(a), lnm.DisplayValue(b))
}

// defineTerminalName defines TT for a created process, in executive mode,
// as the physical name of the device its SYS$INPUT value names, with no
// attributes: VMS 7.1's subprocess with /INPUT=NL: had TT "_NLA0:"
// (testdata/ppf71; a terminal's or a file's device is unconfirmed). An
// input that names no device defines nothing.
func (env *Environment) defineTerminalName(input string) error {
	if input == "" {
		return nil
	}

	device, st := env.deviceName(lnm.DisplayValue(input))
	if st != 0 {
		return nil
	}

	if _, found := env.Devices.Find(device); !found {
		return nil
	}

	_, err := env.Logicals.Define(lnm.ProcessTableName, "TT", lnm.Executive, 0,
		[]lnm.Equivalence{{Value: "_" + device + ":"}})

	return err
}

// openStartupEntry opens value, one of a created process's SYS$ names'
// values, as a process-permanent file with access fac (GET for SYS$INPUT,
// GET and PUT for the output names) and returns its entry: a terminal
// (openTerminalEntry), a mailbox or NL: (a record stream on a channel of
// the process's own, in executive mode), or a file, opened as DCL's OPEN
// opens one (an output file made new, a VFC print file, as VMS 7.1's
// subprocess log was; an input file read from its start). A value that is
// itself a process-permanent file string (a creator's, naming an entry of
// the creator's) is taken as its device. nil when the value can't be
// opened: the name is then defined as given, and an input file that isn't
// there is the CLI's failure (OpenCommandInput).
func (env *Environment) openStartupEntry(value string, fac byte) *rms.PermanentEntry {
	spec := lnm.DisplayValue(value)

	if device, st := env.deviceName(spec); st == 0 {
		if d, found := env.Devices.Find(device); found {
			switch d.DevClass {
			case iodev.DeviceClassTT:
				return env.openTerminalEntry(device, fac)
			case iodev.DeviceClassMailbox:
				return env.openRecordEntry(device, fac)
			}
		}
	}

	if env.Session == nil {
		return nil
	}

	loc, err := env.Session.Locate(spec, false)
	if err != nil {
		return nil
	}

	access := rms.DCLAccess{Read: true}
	if fac&fabFACPut != 0 {
		access = rms.DCLAccess{Write: true, ShareRead: true}
	}

	f, err := env.Session.OpenDCLFile(loc, access)
	if err != nil {
		return nil
	}

	// The open's own access: an output file only writes, so its device
	// characteristics have no input, as VMS's log's had none.
	fac = access.FAC()

	opts := rms.PermanentOptions{Device: f.Name, RAT: fabRATPRN, SQO: true}
	if f.Device != "" {
		opts.Device = env.PermanentDevice(f.Device)
	}

	e, err := env.files.AddDCLFile(f, fac, opts)
	if err != nil {
		_ = f.Close()

		return nil
	}

	return e
}

// openRecordEntry opens a mailbox or NL: (device, "MBA5" or "NLA0") as a
// process-permanent file with access fac, its device text without the
// node's name, as VMS 7.1's "_NLA0:" was (a mailbox's is unconfirmed).
func (env *Environment) openRecordEntry(device string, fac byte) *rms.PermanentEntry {
	dev, found, st := env.OpenRecordDevice(device, fac)
	if !found || st != ssNormal {
		return nil
	}

	e, err := env.files.AddPermanent(&rms.FileHandle{Device: dev, Access: fac},
		rms.PermanentOptions{Device: "_" + device + ":", RAT: fabRATPRN, SQO: true})
	if err != nil {
		dev.Close()

		return nil
	}

	return e
}

// openTerminalEntry opens a process-permanent file on the terminal device
// ("TTA0"), with access fac and PUT (a terminal is written through its
// input entry too: see this file's opening comment), and returns its
// entry; nil if the process's table is full.
func (env *Environment) openTerminalEntry(device string, fac byte) *rms.PermanentEntry {
	// A handle with no console writer isn't the terminal to RMS: a process
	// with nowhere to write writes nowhere.
	var console io.Writer = io.Discard
	if env.consoleOut != nil {
		console = env.consoleOut
	}

	e, err := env.files.AddPermanent(&rms.FileHandle{Console: console, Access: fac | fabFACPut},
		rms.PermanentOptions{Device: "_" + env.NodeName + "$" + device + ":", RAT: fabRATPRN, SQO: true})
	if err != nil {
		return nil
	}

	return e
}

// permanentEntry returns the process-permanent file name leads to, by
// RMS's translation of a file specification (through an alias too), and
// whether it leads to one of the process's.
func (env *Environment) permanentEntry(name string) (*rms.PermanentEntry, bool) {
	specs, err := env.Logicals.TranslateFileSpec(name, lnm.User)
	if err != nil || len(specs) == 0 || specs[0].Permanent == nil {
		return nil, false
	}

	return env.files.Permanent(specs[0].Permanent.IFI)
}

// putPermanent writes record to e as one record, as LIB$PUT_OUTPUT's $PUT
// does, and returns SS$_NORMAL, as LIB$PUT_OUTPUT does for a record
// written (VMS 7.1, testdata/lib57), or $PUT's failure.
func (env *Environment) putPermanent(e *rms.PermanentEntry, record string) (uint32, error) {
	sts, err := env.rmsContext().PutPermanent(e, []byte(record))
	if err != nil {
		return 0, err
	}

	if sts&1 != 0 {
		return ssNormal, nil
	}

	return sts, nil
}

// putPermanentNoWait is putPermanent for a $PUTMSG line: a mailbox's
// record is left pending on its stream rather than waited for
// (rms.Context.PutPermanentNoWait), and the status is SS$_NORMAL.
func (env *Environment) putPermanentNoWait(e *rms.PermanentEntry, record string) (uint32, error) {
	sts, err := env.rmsContext().PutPermanentNoWait(e, []byte(record))
	if err != nil {
		return 0, err
	}

	if sts&1 != 0 {
		return ssNormal, nil
	}

	return sts, nil
}

// shareOutput makes shared, the creator's process-permanent file
// (ProcessStartup.SharedOutput), the process's SYS$OUTPUT too: an entry
// of the process's own on the same stream, which the process's rundown
// doesn't close (rms.FileTable.Share), and SYS$OUTPUT defined as its
// string, TERMINAL, in executive mode. nil shares nothing.
func (env *Environment) shareOutput(shared *rms.PermanentEntry) error {
	if shared == nil {
		return nil
	}

	e, err := env.files.Share(shared)
	if err != nil {
		return err
	}

	_, err = env.Logicals.Define(lnm.ProcessTableName, "SYS$OUTPUT", lnm.Executive, 0,
		[]lnm.Equivalence{{Value: e.EquivalenceString(), Attrs: lnm.AttrTerminal}})

	return err
}
