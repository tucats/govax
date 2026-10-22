package dclshell

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// A command procedure's process-permanent files (docs/PHASE-71 -
// process-permanent files.md, subtask 6).
//
// DCL opens a command procedure as a process-permanent file, and its
// /OUTPUT file as another, and defines SYS$INPUT and SYS$OUTPUT in
// supervisor mode, CONFINE, as their strings, with the TERMINAL attribute
// (the Guide to OpenVMS File Applications, 6.6). What VMS 7.1 showed
// (testdata/ppf71):
//
//   - At level 0 DCL defines SYS$OUTPUT as the executive-mode SYS$OUTPUT's
//     own string, and no SYS$INPUT: the executive one is found.
//   - A procedure's SYS$INPUT is one entry, reused for a nested
//     procedure, whose string names the nested procedure's device: DCL
//     keeps one procedure file open.
//   - Each /OUTPUT file is an entry of its own, a VFC print file with
//     output access only.
//
// govax's procedure entry is a stream DCL's reader gives out
// (procedureSource.readData): an image's SYS$INPUT reads the data records
// after the command that ran it, by LIB$GET_INPUT, an indirect $OPEN and
// $GET of SYS$INPUT, or TYPE SYS$INPUT, and the DECK and EOD checks stay
// in the reader. Which level's records it reads is the current one's.
// The names follow the levels: defineProcedureNames sets them from the
// stack as it is, when a level starts or ends, and when VMINIT makes a new
// process 1 under a procedure (vax.init), which takes DCL's entries over
// (adoptProcedureFiles).

// The FAB$B_FAC and FAB$B_RAT values of DCL's opens.
var (
	fabFACGet = byte(vmsdef.Symbols["FAB$M_GET"])
	fabFACPut = byte(vmsdef.Symbols["FAB$M_PUT"])
	fabFACExe = byte(vmsdef.Symbols["FAB$M_EXE"])
	fabRATPRN = byte(vmsdef.Symbols["FAB$M_PRN"])
)

// openProcedureInput makes the procedure entry, if the process has none
// yet: its records are the current level's data records (procedureData).
// Its access is GET and EXE, as $DISPLAY showed VMS 7.1's, its
// characteristics a disk file's that is read, not written.
func (c *Interpreter) openProcedureInput() error {
	if c.RTL == nil || c.procedureInput != nil {
		return nil
	}

	stream := &rms.RecordStream{Next: c.procedureData(c.Shell), Taken: c.showProcedureData(c.Shell)}

	e, err := c.RTL.AddPermanentStream(stream, fabFACGet|fabFACExe,
		rms.PermanentOptions{RAT: fabRATPRN, SQO: true, Characteristics: rms.FileCharacteristics(fabFACGet)})
	if err != nil {
		return err
	}

	c.procedureInput = e

	return nil
}

// procedureData returns the procedure entry's record source for shell's
// DCL: the next data record of its innermost level. A debugger command
// procedure's level has no data records.
func (c *Interpreter) procedureData(shell *Shell) func() ([]byte, bool) {
	return func() ([]byte, bool) {
		level := shell.currentLevel()
		if level == nil || level.source.plain {
			return nil, false
		}

		record, ok := level.source.readData()
		if !ok {
			return nil, false
		}

		return []byte(record), true
	}
}

// showProcedureData returns what the procedure entry tells of each record
// a reader gets: it is shown with image verification on (showData).
func (c *Interpreter) showProcedureData(shell *Shell) func([]byte) {
	return func(record []byte) {
		// An image may read while another process's DCL is the
		// console's: the record is shown with this one put back for the
		// moment (subcli.go).
		restore := c.useShell(shell)
		c.showData(string(record))
		restore()
	}
}

// closeProcedureInput closes the procedure entry once no procedure is
// running.
func (c *Interpreter) closeProcedureInput() error {
	if c.procedureInput == nil || c.RTL == nil || c.dclLevel() != nil {
		return nil
	}

	e := c.procedureInput
	c.procedureInput = nil

	return c.RTL.ClosePermanentFile(e)
}

// dclLevel is the innermost level running a DCL procedure, not a
// debugger command procedure: the one SYS$INPUT names.
func (s *Shell) dclLevel() *commandLevel {
	for i := len(s.levels) - 1; i >= 0; i-- {
		if !s.levels[i].source.plain {
			return s.levels[i]
		}
	}

	return nil
}

// defineProcedureNames defines SYS$INPUT and SYS$OUTPUT in supervisor
// mode as the levels make them now: SYS$OUTPUT the innermost /OUTPUT
// file's string, or at level 0 DCL's own (corevms's DefineCLIOutput);
// SYS$INPUT the procedure entry's, naming the innermost DCL procedure's
// device, or none at level 0.
func (c *Interpreter) defineProcedureNames() {
	env := c.RTL
	if env == nil {
		return
	}

	defined := false

	for i := len(c.levels) - 1; i >= 0 && !defined; i-- {
		if out := c.levels[i].output; out != nil && out.entry != nil {
			c.defineLevelName("SYS$OUTPUT", out.entry.EquivalenceString())

			defined = true
		}
	}

	if !defined {
		_ = env.DefineCLIOutput()
	}

	level := c.dclLevel()
	if level == nil || c.procedureInput == nil {
		_, _ = c.Logicals.DeleteExact(lnm.ProcessTableName, "SYS$INPUT", lnm.Supervisor)

		return
	}

	device := level.source.name
	if level.source.device != "" {
		device = env.PermanentDevice(strings.TrimPrefix(level.source.device, "_"))
	}

	c.defineLevelName("SYS$INPUT", lnm.PermanentFileString(c.procedureInput.IFI(), device))
	c.procedureInput.Describe(level.source.file)
}

// defineLevelName defines name as DCL defines a procedure's files' names:
// in the process table, supervisor mode, CONFINE, as value, TERMINAL.
func (c *Interpreter) defineLevelName(name, value string) {
	_, _ = c.Logicals.Define(lnm.ProcessTableName, name, lnm.Supervisor, lnm.AttrConfine,
		[]lnm.Equivalence{{Value: value, Attrs: lnm.AttrTerminal}})
}

// openOutputEntry makes an /OUTPUT file a process-permanent file: f, a
// file OpenDCLFile opened, or with f nil the null device. Its access is
// PUT alone, its record attributes PRN, and its device text the volume's
// device ("_GOVAX$DUA1:") or the host file's name.
func (c *Interpreter) openOutputEntry(f *rms.DCLFile) (*procedureOutput, error) {
	env := c.RTL
	opts := rms.PermanentOptions{RAT: fabRATPRN, SQO: true}

	var (
		e   *rms.PermanentEntry
		err error
	)

	switch {
	case f == nil:
		opts.Device = env.PermanentDevice("NLA0")
		e, err = env.AddPermanentStream(&rms.RecordStream{}, fabFACPut, opts)
	case f.Device == "":
		opts.Device = f.Name
		e, err = env.AddPermanentDCLFile(f, fabFACPut, opts)
	default:
		opts.Device = env.PermanentDevice(f.Device)
		e, err = env.AddPermanentDCLFile(f, fabFACPut, opts)
	}

	if err != nil {
		if f != nil {
			_ = f.Close()
		}

		return nil, err
	}

	out := &procedureOutput{entry: e, env: env}

	w := &recordWriter{
		put: func(record []byte) uint32 {
			sts, err := out.env.PutPermanent(out.entry, string(record))
			if err != nil {
				return rmsWER
			}

			return sts
		},
		done: func() error { return out.env.ClosePermanentFile(out.entry) },
	}

	out.Writer, out.close = w, w.close

	return out, nil
}

// rmsWER is RMS$_WER, a record that couldn't be written.
var rmsWER = vmsdef.Symbols["RMS$_WER"]

// adoptProcedureFiles is the Hooks.ProcessOneStarted step for a new
// process 1 (VMINIT in vax.init): the procedure entry, each level's
// /OUTPUT file, and the files OPEN has open, still open in the process 1
// VMINIT replaced, become entries of the new one's, and the names are
// defined again.
func (c *Interpreter) adoptProcedureFiles(env *corevms.Environment) {
	restore := c.useShell(c.home)
	defer restore()

	var errs []error

	if e := c.procedureInput; e != nil {
		adopted, err := env.AdoptPermanentFile(e)
		errs = append(errs, err)
		c.procedureInput = adopted
	}

	for _, level := range c.levels {
		out := level.output
		if out == nil || out.entry == nil {
			continue
		}

		adopted, err := env.AdoptPermanentFile(out.entry)
		errs = append(errs, err)

		if adopted != nil {
			out.env, out.entry = env, adopted
		}
	}

	errs = append(errs, c.adoptOpenFiles(env))

	if err := errors.Join(errs...); err != nil {
		c.Printf("%%GOVAX-W-PPF, keeping the command procedures' files: %v\n", err)
	}

	c.defineProcedureNames()
}

// adoptOpenFiles makes the files OPEN has open entries of env's, a new
// process 1's, and defines again each logical name that still leads to
// its file, as the new entry's string (its IFI may differ).
func (c *Interpreter) adoptOpenFiles(env *corevms.Environment) error {
	var errs []error

	files := c.dclFiles
	c.dclFiles = map[*rms.PermanentEntry]*dclOpenFile{}

	for old, f := range files {
		named := false
		if e, err := c.Logicals.Translate(lnm.ProcessTableName, f.logical, lnm.Supervisor, 0); err == nil &&
			len(e.Equivalences) > 0 && e.Equivalences[0].Value == old.EquivalenceString() {
			named = true
		}

		adopted, err := env.AdoptPermanentFile(old)
		if err != nil {
			errs = append(errs, err)
			c.dclFiles[old] = f

			continue
		}

		f.entry, f.env = adopted, env
		c.dclFiles[adopted] = f

		if named {
			c.defineLevelName(f.logical, adopted.EquivalenceString())
		}
	}

	return errors.Join(errs...)
}
