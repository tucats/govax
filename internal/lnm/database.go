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
// process directory and the process-private tables cataloged there (its
// process table among them), plus the shareable tables every process on
// the system sees, which are cataloged in the system directory: the
// system table, the group tables, and the job tables.
//
// The shareable half is held once, in a sharedTables, and every process's
// Database points to it (docs/PHASE-45.md, subtask 3). NewDatabase makes
// the first process's view, along with the shareable tables; NewProcessView
// makes another process's view of the same ones. A name one process
// defines in its process table is seen by no other process; one it
// defines in its job table is seen by every process in its job, and one
// in the system table by every process.
type Database struct {
	ProcessDirectory *Table
	SystemDirectory  *Table

	// GroupTableName is this process's group table, LNM$GROUP_gggggg
	// with its UIC group in six octal digits.
	GroupTableName string

	// JobTableName is this process's job table, LNM$JOB_xxxxxxxx: on VMS
	// the eight hexadecimal digits are the address of the job's job
	// information block; govax numbers the job tables (NewJobTable).
	JobTableName string

	// Trace, when non-nil, receives a line for each define, delete, and
	// translation (the console wires it to SET DEBUG LOGICALS).
	Trace func(format string, args ...any)

	// tables lists the process's private tables, its directory included,
	// in creation order.
	tables []*Table

	// shared holds the tables every process shares.
	shared *sharedTables
}

// sharedTables is the shareable half of the logical name database: the
// system directory and every table cataloged in it, in creation order
// (the directory first), and the counters that keep the names the
// database makes up unique system-wide.
type sharedTables struct {
	tables []*Table

	// nextID numbers the default LNM$xxxx names CreateTable gives a table
	// created without a name.
	nextID uint32

	// nextJob numbers the job tables (NewJobTable).
	nextJob uint32
}

// NewDatabase returns the first process's Database, whose UIC is uic
// ([group,member], group in the high word), and with it the shareable
// tables every later process's view (NewProcessView) shares. It holds
// the standard VMS directories and tables:
//
//	LNM$PROCESS_DIRECTORY (process-private)
//	    LNM$PROCESS_DIRECTORY  names this directory
//	    LNM$PROCESS_TABLE      names the process table
//	    LNM$PROCESS            = "LNM$PROCESS_TABLE"
//	    LNM$JOB                = "LNM$JOB_xxxxxxxx"
//	    LNM$GROUP              = "LNM$GROUP_gggggg"
//	LNM$SYSTEM_DIRECTORY (shareable)
//	    LNM$SYSTEM_DIRECTORY   names this directory
//	    LNM$SYSTEM_TABLE       names the system table
//	    LNM$GROUP_gggggg       names the group table
//	    LNM$JOB_xxxxxxxx       names the job table
//	    LNM$SYSTEM             = "LNM$SYSTEM_TABLE"
//	    LNM$FILE_DEV           = "LNM$PROCESS", "LNM$JOB", "LNM$GROUP", "LNM$SYSTEM"
//	    LNM$DCL_LOGICAL        = "LNM$FILE_DEV"
//	    LNM$DIRECTORIES        = "LNM$PROCESS_DIRECTORY", "LNM$SYSTEM_DIRECTORY"
//	    LNM$TEMPORARY_MAILBOX  = "LNM$JOB"
//	    LNM$PERMANENT_MAILBOX  = "LNM$SYSTEM"
//
// This follows the User's Manual's tables 11-1, 11-2 and 11-4, minus the
// cluster tables govax doesn't have. The tables are kernel mode, and the
// directory logical names are executive mode, so they're visible to a
// translation at any access mode. None of these tables can be deleted.
func NewDatabase(uic uint32) *Database {
	sh := &sharedTables{}

	sys := newTable(SystemDirectoryName, Kernel, nil)
	sys.Directory = true
	sys.Shareable = true
	sys.permanent = true

	// The system directory names itself.
	(&Database{SystemDirectory: sys, shared: sh}).catalog(sys, sys, Kernel, 0)

	db := &Database{SystemDirectory: sys, shared: sh}
	db.newPermanentTable(SystemTableName, sys)

	for _, n := range []struct {
		name string
		eqv  []string
	}{
		{"LNM$SYSTEM", []string{SystemTableName}},
		{"LNM$FILE_DEV", []string{"LNM$PROCESS", "LNM$JOB", "LNM$GROUP", "LNM$SYSTEM"}},
		{"LNM$DCL_LOGICAL", []string{"LNM$FILE_DEV"}},
		{"LNM$DIRECTORIES", []string{ProcessDirectoryName, SystemDirectoryName}},
		// Where $CREMBX puts mailbox logical names (docs/PHASE-26.md
		// subtask 29): a temporary mailbox's in the job table, so every
		// process in the job can find it.
		{"LNM$TEMPORARY_MAILBOX", []string{"LNM$JOB"}},
		{"LNM$PERMANENT_MAILBOX", []string{"LNM$SYSTEM"}},
	} {
		sys.add(directoryName(n.name, n.eqv))
	}

	// The first process's group table, then its job's, so the system
	// directory lists them in that order.
	db.newPermanentTable(groupTableName(uic), sys)

	return db.NewProcessView(uic, db.NewJobTable())
}

// groupTableName is the name of the group table of a process whose UIC
// is uic: LNM$GROUP_gggggg, with the group in six octal digits.
func groupTableName(uic uint32) string {
	return fmt.Sprintf("LNM$GROUP_%06o", uic>>16)
}

// NewJobTable creates a new job's logical name table in the system
// directory, kernel mode and permanent (no user deletes it: it lasts as
// long as its job), and returns its name, LNM$JOB_xxxxxxxx. The number
// stands for the job information block's address that VMS uses; govax's
// are 80000100, 80000200, and so on (a choice, not VMS's).
func (db *Database) NewJobTable() string {
	db.shared.nextJob++

	name := fmt.Sprintf("LNM$JOB_%08X", 0x80000000+db.shared.nextJob<<8)
	db.newPermanentTable(name, db.SystemDirectory)

	return name
}

// NewProcessView returns a new process's view of db's shareable tables:
// a process directory and process table of its own, with the process
// whose UIC is uic in the job whose table is jobTable (NewJobTable's
// name: its creator's, for a subprocess). Its group's table is created
// if no process of the group has had one yet. The view starts with
// db's Trace.
func (db *Database) NewProcessView(uic uint32, jobTable string) *Database {
	v := &Database{
		SystemDirectory: db.SystemDirectory,
		GroupTableName:  groupTableName(uic),
		JobTableName:    jobTable,
		Trace:           db.Trace,
		shared:          db.shared,
	}

	v.ProcessDirectory = newTable(ProcessDirectoryName, Kernel, nil)
	v.ProcessDirectory.Directory = true
	v.ProcessDirectory.permanent = true
	v.catalog(v.ProcessDirectory, v.ProcessDirectory, Kernel, 0)

	v.newPermanentTable(ProcessTableName, v.ProcessDirectory)

	if !v.SystemDirectory.hasName(v.GroupTableName) {
		v.newPermanentTable(v.GroupTableName, v.SystemDirectory)
	}

	for _, n := range []struct{ name, eqv string }{
		{"LNM$PROCESS", ProcessTableName},
		{"LNM$JOB", jobTable},
		{"LNM$GROUP", v.GroupTableName},
	} {
		v.ProcessDirectory.add(directoryName(n.name, []string{n.eqv}))
	}

	return v
}

// directoryName returns an executive-mode logical name for a directory,
// translating to the table names eqv.
func directoryName(name string, eqv []string) *Entry {
	e := &Entry{Name: name, Mode: Executive}
	for _, v := range eqv {
		e.Equivalences = append(e.Equivalences, Equivalence{Value: v})
	}

	return e
}

func (db *Database) newPermanentTable(name string, parent *Table) {
	t := newTable(name, Kernel, parent)
	t.permanent = true

	db.catalog(t, db.directoryFor(parent), Kernel, 0)
}

// catalog records t in its table list, the shared one for a shareable
// table, and enters its name in dir.
func (db *Database) catalog(t, dir *Table, mode Mode, attrs uint32) {
	e := &Entry{Name: t.Name, Mode: mode, Attrs: attrs | AttrTable, Target: t}
	t.catalog = e
	dir.add(e)

	list := db.listFor(t)
	*list = append(*list, t)
}

// listFor returns the table list t belongs in: the shared one for a
// shareable table, the process's own otherwise.
func (db *Database) listFor(t *Table) *[]*Table {
	if t.Shareable {
		return &db.shared.tables
	}

	return &db.tables
}

// directoryFor returns the directory a table with `parent` parent is
// cataloged in: the system directory for a shareable table, the process
// directory otherwise ($CRELNT's partab description).
func (db *Database) directoryFor(parent *Table) *Table {
	if parent.Shareable {
		return db.SystemDirectory
	}

	return db.ProcessDirectory
}

// Tables returns every table the process sees, directories included: its
// private ones and then the shareable ones, each in creation order.
func (db *Database) Tables() []*Table {
	return append(append([]*Table(nil), db.tables...), db.shared.tables...)
}

// Children returns the tables whose parent is t, in creation order (SHOW
// LOGICAL/STRUCTURE). A table's children are shareable exactly when it
// is, so they're all in its own list.
func (db *Database) Children(t *Table) []*Table {
	var out []*Table

	for _, c := range *db.listFor(t) {
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
