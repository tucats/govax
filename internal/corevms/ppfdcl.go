package corevms

import (
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The process-permanent files DCL opens (docs/PHASE-71 - process-permanent
// files.md, subtask 6): a command procedure's input and its /OUTPUT file,
// opened in the process's own table, as LOGINOUT's are (ppfstart.go), and
// reached by the names DCL defines in supervisor mode.

// AddPermanentStream makes stream, a record stream DCL keeps in Go (a
// procedure's data records, an /OUTPUT file on the host or NL:), one of
// the process's process-permanent files, with access fac.
func (env *Environment) AddPermanentStream(stream *rms.RecordStream, fac byte,
	opts rms.PermanentOptions,
) (*rms.PermanentEntry, error) {
	return env.files.AddPermanent(&rms.FileHandle{Device: stream, Access: fac}, opts)
}

// AddPermanentDCLFile makes f, a file DCL opened, one of the process's
// process-permanent files, with access fac (rms.FileTable.AddDCLFile).
func (env *Environment) AddPermanentDCLFile(f *rms.DCLFile, fac byte,
	opts rms.PermanentOptions,
) (*rms.PermanentEntry, error) {
	return env.files.AddDCLFile(f, fac, opts)
}

// AdoptPermanentFile makes e, an entry of an earlier process 1's, one of
// env's (rms.FileTable.Adopt): DCL's command procedure files, still open
// when VMINIT made a new process 1.
func (env *Environment) AdoptPermanentFile(e *rms.PermanentEntry) (*rms.PermanentEntry, error) {
	return env.files.Adopt(e)
}

// PermanentEntryFor returns the process-permanent file an IFI word names
// (rms.FileTable.Permanent), and whether the process has it open.
func (env *Environment) PermanentEntryFor(ifi uint16) (*rms.PermanentEntry, bool) {
	return env.files.Permanent(ifi)
}

// ClosePermanentFile closes e, one of the process's process-permanent
// files.
func (env *Environment) ClosePermanentFile(e *rms.PermanentEntry) error {
	_, err := env.files.ClosePermanent(e.IFI())

	return err
}

// PutPermanent writes record to e as one record, as LIB$PUT_OUTPUT does
// (putPermanent): DCL's own output to a procedure's /OUTPUT file.
func (env *Environment) PutPermanent(e *rms.PermanentEntry, record string) (uint32, error) {
	return env.putPermanent(e, record)
}

// PermanentDevice is device's name as a process-permanent file string
// carries it ("_GOVAX$DUA1:"): the physical name with the node's.
func (env *Environment) PermanentDevice(device string) string {
	if env.NodeName == "" {
		return "_" + device + ":"
	}

	return "_" + env.NodeName + "$" + device + ":"
}

// DefineCLIOutput is DCL's first step at command level 0: SYS$OUTPUT
// defined in supervisor mode, CONFINE, as the string of the
// process-permanent file the executive-mode SYS$OUTPUT names, with its
// attributes (VMS 7.1: the same entry, at the terminal and in a
// subprocess). When that SYS$OUTPUT names none, a supervisor-mode
// SYS$OUTPUT (a command procedure's /OUTPUT file's) is deleted instead.
func (env *Environment) DefineCLIOutput() error {
	e, err := env.Logicals.Translate(lnm.ProcessTableName, "SYS$OUTPUT", lnm.Executive, 0)
	if err != nil || len(e.Equivalences) == 0 || !isPermanentString(e.Equivalences[0].Value) {
		_, _ = env.Logicals.DeleteExact(lnm.ProcessTableName, "SYS$OUTPUT", lnm.Supervisor)

		return nil
	}

	eqv := e.Equivalences[0]

	_, err = env.Logicals.Define(lnm.ProcessTableName, "SYS$OUTPUT", lnm.Supervisor, lnm.AttrConfine, []lnm.Equivalence{eqv})

	return err
}

// isPermanentString reports whether value is a process-permanent file
// string.
func isPermanentString(value string) bool {
	_, ok := lnm.ParsePermanentFile(value)

	return ok
}

// rmsRER is RMS$_RER, a record that couldn't be read.
var rmsRER = vmsdef.Symbols["RMS$_RER"]

// GetPermanent reads the next record from e, as DCL's READ of a file it
// didn't open itself does (rms.Context.GetPermanent): the record and
// RMS$_NORMAL, or the status for why there is none. A stream that would
// have to wait for its record is RMS$_RER.
func (env *Environment) GetPermanent(e *rms.PermanentEntry) ([]byte, uint32) {
	record, sts, err := env.rmsContext().GetPermanent(e)
	if err != nil {
		return nil, rmsRER
	}

	return record, sts
}
