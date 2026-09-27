package lnm

import (
	"fmt"

	"github.com/tucats/govax/internal/vmserrors"
)

// Names of the tables every Database starts with.
const (
	ProcessDirectoryName = "LNM$PROCESS_DIRECTORY"
	SystemDirectoryName  = "LNM$SYSTEM_DIRECTORY"
	ProcessTableName     = "LNM$PROCESS_TABLE"
	SystemTableName      = "LNM$SYSTEM_TABLE"
)

// Database is one process's view of the logical name tables: its own
// process directory and process table, plus the shareable system
// directory, system table, and its UIC group's table. govax has a single
// process, so one Database holds all of them.
type Database struct {
	ProcessDirectory *Table
	SystemDirectory  *Table

	// GroupTableName is this process's group table, LNM$GROUP_gggggg
	// with its UIC group in six octal digits.
	GroupTableName string

	// Trace, when non-nil, receives a line for each define, delete, and
	// translation (the console wires it to SET DEBUG LOGICALS).
	Trace func(format string, args ...any)

	// tables lists every live table, directories included, in creation
	// order.
	tables []*Table

	// nextID numbers the default LNM$xxxx names CreateTable gives a table
	// created without a name.
	nextID uint32
}

// NewDatabase returns a Database holding the standard VMS directories and
// tables for a process whose UIC is uic ([group,member], group in the
// high word):
//
//	LNM$PROCESS_DIRECTORY (process-private)
//	    LNM$PROCESS_DIRECTORY  names this directory
//	    LNM$PROCESS_TABLE      names the process table
//	    LNM$PROCESS            = "LNM$PROCESS_TABLE"
//	    LNM$GROUP              = "LNM$GROUP_gggggg"
//	LNM$SYSTEM_DIRECTORY (shareable)
//	    LNM$SYSTEM_DIRECTORY   names this directory
//	    LNM$SYSTEM_TABLE       names the system table
//	    LNM$GROUP_gggggg       names the group table
//	    LNM$SYSTEM             = "LNM$SYSTEM_TABLE"
//	    LNM$FILE_DEV           = "LNM$PROCESS", "LNM$GROUP", "LNM$SYSTEM"
//	    LNM$DCL_LOGICAL        = "LNM$FILE_DEV"
//	    LNM$DIRECTORIES        = "LNM$PROCESS_DIRECTORY", "LNM$SYSTEM_DIRECTORY"
//
// This follows the User's Manual's tables 11.1, 11.2 and 11.4, minus the
// job and cluster tables govax doesn't have (so LNM$FILE_DEV has no
// LNM$JOB element). The tables are kernel mode, and the directory logical
// names are executive mode, so they're visible to a translation at any
// access mode. None of these tables can be deleted.
func NewDatabase(uic uint32) *Database {
	db := &Database{GroupTableName: fmt.Sprintf("LNM$GROUP_%06o", uic>>16)}

	db.ProcessDirectory = db.newDirectory(ProcessDirectoryName, false)
	db.SystemDirectory = db.newDirectory(SystemDirectoryName, true)

	db.newPermanentTable(ProcessTableName, db.ProcessDirectory)
	db.newPermanentTable(SystemTableName, db.SystemDirectory)
	db.newPermanentTable(db.GroupTableName, db.SystemDirectory)

	names := []struct {
		dir  *Table
		name string
		eqv  []string
	}{
		{db.ProcessDirectory, "LNM$PROCESS", []string{ProcessTableName}},
		{db.ProcessDirectory, "LNM$GROUP", []string{db.GroupTableName}},
		{db.SystemDirectory, "LNM$SYSTEM", []string{SystemTableName}},
		{db.SystemDirectory, "LNM$FILE_DEV", []string{"LNM$PROCESS", "LNM$GROUP", "LNM$SYSTEM"}},
		{db.SystemDirectory, "LNM$DCL_LOGICAL", []string{"LNM$FILE_DEV"}},
		{db.SystemDirectory, "LNM$DIRECTORIES", []string{ProcessDirectoryName, SystemDirectoryName}},
	}

	for _, n := range names {
		e := &Entry{Name: n.name, Mode: Executive}
		for _, v := range n.eqv {
			e.Equivalences = append(e.Equivalences, Equivalence{Value: v})
		}

		n.dir.add(e)
	}

	return db
}

func (db *Database) newDirectory(name string, shareable bool) *Table {
	t := newTable(name, Kernel, nil)
	t.Directory = true
	t.Shareable = shareable
	t.permanent = true

	db.catalog(t, t, Kernel, 0)

	return t
}

func (db *Database) newPermanentTable(name string, parent *Table) {
	t := newTable(name, Kernel, parent)
	t.permanent = true

	db.catalog(t, db.directoryFor(parent), Kernel, 0)
}

// catalog records t in the table list and enters its name in dir.
func (db *Database) catalog(t, dir *Table, mode Mode, attrs uint32) {
	e := &Entry{Name: t.Name, Mode: mode, Attrs: attrs | AttrTable, Target: t}
	t.catalog = e
	dir.add(e)
	db.tables = append(db.tables, t)
}

// directoryFor returns the directory a table with parent parent is
// cataloged in: the system directory for a shareable table, the process
// directory otherwise ($CRELNT's partab description).
func (db *Database) directoryFor(parent *Table) *Table {
	if parent.Shareable {
		return db.SystemDirectory
	}

	return db.ProcessDirectory
}

// Tables returns every table, directories included, in creation order.
func (db *Database) Tables() []*Table {
	return append([]*Table(nil), db.tables...)
}

// Children returns the tables whose parent is t, in creation order (SHOW
// LOGICAL/STRUCTURE).
func (db *Database) Children(t *Table) []*Table {
	var out []*Table

	for _, c := range db.tables {
		if c.Parent == t {
			out = append(out, c)
		}
	}

	return out
}

func (db *Database) tracef(format string, args ...any) {
	if db.Trace != nil {
		db.Trace(format, args...)
	}
}

func status(code uint32) error {
	return vmserrors.New(code)
}

// ResolveTables translates tabnam, a table name or a logical name that
// iteratively translates to table names, into the list of tables it
// designates, in search order. This is how every service's tabnam
// argument is interpreted:
//
//   - The name is looked up in the process directory and then the system
//     directory. VMS defines LNM$DIRECTORIES as that list; the search
//     order is fixed here rather than read from it, since resolving
//     LNM$DIRECTORIES itself needs a directory to look in.
//   - A table-name entry designates its table. Any other entry is
//     translated one more level, each equivalence string in order, so a
//     search list of tables (LNM$FILE_DEV) yields each of its tables.
//   - Names and tables at an access mode less privileged than mode are
//     ignored ($TRNLNM's acmode rule). User considers everything.
//   - An equivalence string that doesn't name anything in a directory is
//     skipped, so one stale element doesn't disable a whole search list.
//
// Errors: SS$_IVLOGNAM for a name that is empty or longer than 255
// characters, SS$_NOLOGTAB when tabnam isn't in either directory,
// SS$_IVLOGTAB when it is but leads to no table, and SS$_TOOMANYLNAM when
// the translation goes more than MaxDepth levels deep (which is also how
// a circular definition ends). A table reached twice is listed once.
func (db *Database) ResolveTables(tabnam string, mode Mode) ([]*Table, error) {
	if tabnam == "" || len(tabnam) > MaxNameLength {
		return nil, status(vmserrors.SS_IVLOGNAM)
	}

	var out []*Table

	seen := map[*Table]bool{}

	found, err := db.resolveTableName(tabnam, mode, 0, &out, seen)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, status(vmserrors.SS_NOLOGTAB)
	}

	if len(out) == 0 {
		return nil, status(vmserrors.SS_IVLOGTAB)
	}

	return out, nil
}

func (db *Database) resolveTableName(name string, mode Mode, depth int, out *[]*Table, seen map[*Table]bool) (bool, error) {
	if depth > MaxDepth {
		return false, status(vmserrors.SS_TOOMANYLNAM)
	}

	e := db.ProcessDirectory.lookup(name, mode, false)
	if e == nil {
		e = db.SystemDirectory.lookup(name, mode, false)
	}

	if e == nil {
		return false, nil
	}

	if e.IsTable() {
		if e.Target.Mode <= mode && !seen[e.Target] {
			seen[e.Target] = true
			*out = append(*out, e.Target)
		}

		return true, nil
	}

	for _, eqv := range e.Equivalences {
		if _, err := db.resolveTableName(eqv.Value, mode, depth+1, out, seen); err != nil {
			return false, err
		}
	}

	return true, nil
}

// Translate performs one level of translation of lognam, searching the
// tables tabnam designates (see ResolveTables) in order and returning
// the first match — $TRNLNM's lookup. The caller reads the equivalence
// strings, attributes, and containing table from the returned Entry; no
// further translation of an equivalence string is done here.
//
// mode is $TRNLNM's acmode: names at a less privileged mode are ignored,
// and of those left in a table the outermost wins. attr may contain
// AttrCaseBlind to match lognam without regard to letter case.
//
// Errors: those of ResolveTables, SS$_IVLOGNAM for an empty or too-long
// lognam, SS$_BADPARAM for an attr bit other than AttrCaseBlind, and
// SS$_NOLOGNAM when no table has the name.
func (db *Database) Translate(tabnam, lognam string, mode Mode, attr uint32) (*Entry, error) {
	if lognam == "" || len(lognam) > MaxNameLength {
		return nil, status(vmserrors.SS_IVLOGNAM)
	}

	if attr&^AttrCaseBlind != 0 {
		return nil, status(vmserrors.SS_BADPARAM)
	}

	tables, err := db.ResolveTables(tabnam, mode)
	if err != nil {
		return nil, err
	}

	for _, t := range tables {
		if e := t.lookup(lognam, mode, attr&AttrCaseBlind != 0); e != nil {
			db.tracef("translate %s in %s: found in %s [%s]", lognam, tabnam, t.Name, e.Mode)

			return e, nil
		}
	}

	db.tracef("translate %s in %s: no match", lognam, tabnam)

	return nil, status(vmserrors.SS_NOLOGNAM)
}

// DefineProcessNames defines the process-permanent names VMS's LOGINOUT
// gives an interactive process, all equated to its terminal, the
// physical device name terminal (for example "_TTA0:"):
//
//	SYS$INPUT, SYS$OUTPUT, SYS$ERROR, SYS$COMMAND   executive mode, TERMINAL
//	TT                                              supervisor mode
//
// All go in the process table. Real VMS prefixes the SYS$ names'
// equivalence strings with a hidden 4-byte ESC/IFI header naming the
// process-permanent file; govax doesn't.
func (db *Database) DefineProcessNames(terminal string) error {
	for _, name := range []string{"SYS$INPUT", "SYS$OUTPUT", "SYS$ERROR", "SYS$COMMAND"} {
		eqv := []Equivalence{{Value: terminal, Attrs: AttrTerminal}}
		if _, err := db.Define(ProcessTableName, name, Executive, 0, eqv); err != nil {
			return err
		}
	}

	_, err := db.Define(ProcessTableName, "TT", Supervisor, 0, []Equivalence{{Value: terminal}})

	return err
}
