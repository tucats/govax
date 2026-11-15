package corevms

import (
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// DCL's logical name commands, held to the services' rules
// (docs/planning/PHASE-94 - privileges.md, Decision 5).
//
// VMS's DCL makes DEFINE, ASSIGN, DEASSIGN, and CREATE/NAME_TABLE by
// calling $CRELNM, $DELLNM, and $CRELNT from supervisor mode, DCL's own,
// and shows what the service returned (%SYSTEM-F-NOPRIV alone, VMS 7.1:
// testdata/priv94's NAMES sections). govax's DCL is Go, so these methods
// are the services' checks at supervisor mode, made for the DCL running
// in env's process:
//
//   - a name's mode inner to supervisor (/EXECUTIVE_MODE) is kept only
//     with SYSNAM, and otherwise made supervisor without complaint
//     (writeModeAt);
//   - a shareable table is written by its protection, or past it by
//     SYSNAM for the system table and GRPNAM for the process's group
//     table (lnmTableAccessAt);
//   - a shareable table's name goes in LNM$SYSTEM_DIRECTORY, which the
//     process must be able to write.
//
// DEFINE looks its table up from supervisor mode, so a table made at user
// mode is NOLOGTAB to it (testdata/priv94: DEFINE/TABLE=PRIV94_OT, a
// user-mode table, was NOLOGTAB for everyone); DEASSIGN and SHOW LOGICAL
// look from user mode, as $DELLNM and $TRNLNM do.

// dclMode is the access mode DCL's commands run in.
const dclMode = registers.Supervisor

// DefineName is DCL's DEFINE and ASSIGN: lognam defined at mode (made
// supervisor when it's inner and the process lacks SYSNAM) in the first
// table tabnam designates from supervisor mode, with the strings eqv, if
// the process may write that table. superseded reports a definition at
// the same mode replaced. The errors are VMSErrors of $CRELNM's statuses.
func (env *Environment) DefineName(tabnam, lognam string, mode lnm.Mode, eqv []lnm.Equivalence) (superseded bool, err error) {
	mode = env.writeModeAt(mode, dclMode)

	tables, err := env.Logicals.ResolveTables(tabnam, lnm.Supervisor)
	if err != nil {
		return false, err
	}

	if st := env.lnmTableAccessAt(tables[0], accessWrite, dclMode); st != 0 {
		return false, vmserrors.New(st)
	}

	superseded, err = env.Logicals.DefineIn(tables[0], lognam, mode, 0, eqv)
	env.traceLogicals("DEFINE(%s,%s) [%s], %v", tabnam, lognam, mode, err)

	return superseded, err
}

// DeassignName is DCL's DEASSIGN: $DELLNM's deletion of lognam (every
// name, when it's empty) at mode, maximized at supervisor, and outer
// modes, from the table of those tabnam designates that has it, if the
// process may write that table. A name not at mode is SS$_NOLOGNAM first.
func (env *Environment) DeassignName(tabnam, lognam string, mode lnm.Mode) error {
	mode = env.writeModeAt(mode, dclMode)

	t, err := env.Logicals.DeletionTable(tabnam, lognam, mode)
	if err != nil {
		return err
	}

	if st := env.lnmTableAccessAt(t, accessWrite, dclMode); st != 0 {
		return vmserrors.New(st)
	}

	_, err = env.Logicals.Delete(tabnam, lognam, mode)

	return err
}

// CreateNameTable is DCL's CREATE/NAME_TABLE: $CRELNT's making of table
// tabnam under partab at mode (supervisor when it's inner and the process
// lacks SYSNAM), owned by the process, with protection and quota (in
// bytes, 0 for none). A table under a shareable parent takes write access
// to LNM$SYSTEM_DIRECTORY, where its name goes, whatever the parent
// (testdata/priv94, round 2's cases 18 to 29).
func (env *Environment) CreateNameTable(tabnam, partab string, mode lnm.Mode, protection uint16, quota uint32) (lnm.CreateResult, error) {
	mode = env.writeModeAt(mode, dclMode)

	if parent := env.firstTable(partab); parent != nil && parent.Shareable {
		if st := env.lnmTableAccessAt(env.Logicals.SystemDirectory, accessWrite, dclMode); st != 0 {
			return 0, vmserrors.New(st)
		}
	}

	_, result, err := env.Logicals.CreateTableWith(tabnam, partab, mode, 0,
		lnm.TableProfile{Owner: env.Process.UIC, Protection: protection, Quota: quota})

	return result, err
}

// MayReadTable reports whether the process may read names in table t, by
// its protection (SYSPRV, READALL, and BYPASS count): SHOW LOGICAL passes
// over a table it may not read with %SYSTEM-W-IVLOGTAB (VMS 7.1,
// testdata/priv94's PRIV94_PT).
func (env *Environment) MayReadTable(t *lnm.Table) bool {
	return env.lnmTableAccessAt(t, accessRead, dclMode) == 0
}

// TranslateName is $TRNLNM's lookup for DCL (F$TRNLNM): one level of
// translation of lognam in the tables tabnam designates that the process
// may read, names at a mode outer to mode ignored; SS$_NOPRIV when only a
// table it may not read could have had it.
func (env *Environment) TranslateName(tabnam, lognam string, mode lnm.Mode, attr uint32) (*lnm.Entry, error) {
	return env.Logicals.TranslateWhere(tabnam, lognam, mode, attr, env.MayReadTable)
}
