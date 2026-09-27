package lnm

import (
	"fmt"

	"github.com/tucats/govax/internal/vmserrors"
)

// CreateResult says what CreateTable did, matching $CRELNT's three
// success statuses.
type CreateResult int

const (
	// TableCreated: a new table was created (SS$_LNMCREATED).
	TableCreated CreateResult = iota

	// TableExisted: AttrCreateIf was given and the name already existed
	// at that mode, so nothing changed (SS$_NORMAL).
	TableExisted

	// TableSuperseded: a new table was created, replacing an existing
	// directory entry of the same name and mode (SS$_SUPERSEDE).
	TableSuperseded
)

// CreateTable creates a logical name table named tabnam at access mode
// mode, whose parent is the first table partab designates — $CRELNT.
// An empty tabnam gets a unique default name of the form LNM$xxxx. mode
// is the mode actually used (the caller does the "maximizing").
//
// The new table is shareable, and cataloged in LNM$SYSTEM_DIRECTORY, when
// its parent is shareable; otherwise it is process-private and cataloged
// in LNM$PROCESS_DIRECTORY. attr may contain:
//
//   - AttrCreateIf: if the directory already has tabnam at mode, change
//     nothing and return TableExisted, with the existing table (or nil if
//     that entry isn't a table).
//   - AttrNoAlias: delete tabnam's directory entries at outer modes, and
//     block later definitions of it at outer modes.
//   - AttrConfine: for a process-private table. It is inherited from the
//     parent regardless of attr, and ignored for a shareable table.
//
// Without AttrCreateIf, an existing entry at mode is superseded, deleting
// the table it names and that table's descendants — unless that would
// delete the new table's own parent (SS$_PARENT_DEL) or a table created
// at startup (SS$_NOPRIV). An inner-mode entry with AttrNoAlias blocks
// creation (SS$_DUPLNAM).
//
// Other errors: SS$_IVLOGTAB for a tabnam that isn't 1-31 letters,
// digits, "$" or "_"; SS$_IVLOGNAM for a partab that isn't 1-31
// characters; SS$_BADPARAM for any other attr bit; SS$_NOPRIV for a mode
// more privileged than the parent's; and those of ResolveTables for
// partab.
func (db *Database) CreateTable(tabnam, partab string, mode Mode, attr uint32) (*Table, CreateResult, error) {
	if attr&^(AttrCreateIf|AttrNoAlias|AttrConfine) != 0 {
		return nil, 0, status(vmserrors.SS_BADPARAM)
	}

	if partab == "" || len(partab) > MaxTableNameLength {
		return nil, 0, status(vmserrors.SS_IVLOGNAM)
	}

	if tabnam == "" {
		tabnam = db.defaultTableName()
	}

	if !validTableName(tabnam) {
		return nil, 0, status(vmserrors.SS_IVLOGTAB)
	}

	parents, err := db.ResolveTables(partab, User)
	if err != nil {
		return nil, 0, err
	}

	parent := parents[0]

	if mode < parent.Mode {
		return nil, 0, status(vmserrors.SS_NOPRIV)
	}

	dir := db.directoryFor(parent)

	if attr&AttrCreateIf != 0 {
		if e := dir.atMode(tabnam, mode); e != nil {
			db.tracef("create table %s [%s]: already exists", tabnam, mode)

			return e.Target, TableExisted, nil
		}
	}

	superseded, err := db.clearForNewName(dir, tabnam, mode, attr&AttrNoAlias != 0, parent)
	if err != nil {
		return nil, 0, err
	}

	t := newTable(tabnam, mode, parent)
	if !t.Shareable {
		t.Attrs = (parent.Attrs | attr) & AttrConfine
	}

	db.catalog(t, dir, mode, attr&AttrNoAlias)

	result := TableCreated
	if superseded {
		result = TableSuperseded
	}

	db.tracef("create table %s [%s] in %s, parent %s", tabnam, mode, dir.Name, parent.Name)

	return t, result, nil
}

// defaultTableName returns an LNM$xxxx name (xxxx in hexadecimal) that
// neither directory holds yet.
func (db *Database) defaultTableName() string {
	for {
		db.nextID++

		name := fmt.Sprintf("LNM$%04X", db.nextID)
		if !db.ProcessDirectory.hasName(name) && !db.SystemDirectory.hasName(name) {
			return name
		}
	}
}
