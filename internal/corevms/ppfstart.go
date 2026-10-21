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
//     showed; the input entry's is GET (unconfirmed).
//   - VMS numbered process 1's input entry 1 and its output entry 3; govax
//     numbers them lowest free first, so its output entry is 2 (the
//     oracle masks the numbers).
//
// For now only a terminal is opened as an entry: a SYS$ name given a
// mailbox, NL:, or a file is defined as that name, and reached as before
// (recdevice.go, outfile.go), until command procedures and subprocesses
// open their files as entries too.

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
		func(string) (string, bool) { return device, true })
}

// openProcessFiles is process startup's step for SYS$INPUT, SYS$OUTPUT,
// and SYS$ERROR, given input, output, and errorOutput (each "" when not
// given, when the name isn't defined): a value terminal says names a
// terminal device is opened as an entry, shared as this file's opening
// comment says, and the name defined as its string with attrs, and
// SYS$COMMAND with SYS$INPUT's; anything else defines the name as given.
func (env *Environment) openProcessFiles(input, output, errorOutput string, attrs uint32,
	terminal func(value string) (device string, ok bool),
) error {
	open := func(value string, fac byte) *rms.PermanentEntry {
		if value == "" {
			return nil
		}

		device, ok := terminal(value)
		if !ok {
			return nil
		}

		return env.openTerminalEntry(device, fac)
	}

	in := open(input, fabFACGet)
	out := open(output, fabFACGet|fabFACPut)

	errEntry := out
	if !sameDevice(errorOutput, output) {
		errEntry = open(errorOutput, fabFACGet|fabFACPut)
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

// terminalDevice returns the device a SYS$ name's value names ("TTA0"),
// and whether it is a terminal. A value that is itself a process-permanent
// file string (a creator's, naming an entry of the creator's) is taken as
// its device.
func (env *Environment) terminalDevice(value string) (string, bool) {
	device, st := env.deviceName(lnm.DisplayValue(value))
	if st != 0 {
		return "", false
	}

	d, found := env.Devices.Find(device)

	return device, found && d.DevClass == iodev.DeviceClassTT
}

// openTerminalEntry opens a process-permanent file on the terminal device
// ("TTA0"), with access fac, and returns its entry; nil if the process's
// table is full.
func (env *Environment) openTerminalEntry(device string, fac byte) *rms.PermanentEntry {
	// A handle with no console writer isn't the terminal to RMS: a process
	// with nowhere to write writes nowhere.
	var console io.Writer = io.Discard
	if env.consoleOut != nil {
		console = env.consoleOut
	}

	e, err := env.files.AddPermanent(&rms.FileHandle{Console: console, Access: fac},
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
