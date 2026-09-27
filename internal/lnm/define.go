package lnm

import "github.com/tucats/govax/internal/vmserrors"

// Define creates logical name lognam at access mode mode in the first
// table tabnam designates, with equivalence strings eqv (indexes 0 to
// len(eqv)-1) — $CRELNM. attr may contain AttrNoAlias and AttrConfine
// (and AttrCrelog, for the $CRELOG wrapper). mode is the mode actually
// used; "maximizing" the caller's mode with a requested one is the
// caller's job.
//
// It reports superseded when an existing definition of lognam at the same
// mode in the same table was replaced (SS$_SUPERSEDE rather than
// SS$_NORMAL). Following $CRELNM:
//
//   - A name at a more privileged mode with AttrNoAlias blocks the define
//     (SS$_DUPLNAM).
//   - Defining with AttrNoAlias deletes the name's definitions at outer
//     modes in the table.
//   - A name in a table with AttrConfine gets AttrConfine too.
//   - mode can't be more privileged than the table's mode (SS$_NOPRIV).
//   - A name defined in a directory table must be 1-31 letters, digits,
//     "$" or "_" (SS$_IVLOGNAM).
//
// Superseding a table-name entry deletes that table and its descendants;
// the tables created at startup can't be superseded (SS$_NOPRIV).
//
// Other errors: SS$_IVLOGNAM for a name or equivalence string that is
// empty or longer than 255 characters, SS$_BADPARAM for no equivalence
// strings, more than 128, or an attribute bit that doesn't belong, and
// those of ResolveTables.
func (db *Database) Define(tabnam, lognam string, mode Mode, attr uint32, eqv []Equivalence) (superseded bool, err error) {
	if lognam == "" || len(lognam) > MaxNameLength {
		return false, status(vmserrors.SS_IVLOGNAM)
	}

	if attr&^(AttrNoAlias|AttrConfine|AttrCrelog) != 0 {
		return false, status(vmserrors.SS_BADPARAM)
	}

	if len(eqv) == 0 || len(eqv) > MaxEquivalences {
		return false, status(vmserrors.SS_BADPARAM)
	}

	for _, q := range eqv {
		if q.Value == "" || len(q.Value) > MaxNameLength {
			return false, status(vmserrors.SS_IVLOGNAM)
		}

		if q.Attrs&^(AttrConcealed|AttrTerminal) != 0 {
			return false, status(vmserrors.SS_BADPARAM)
		}
	}

	tables, err := db.ResolveTables(tabnam, User)
	if err != nil {
		return false, err
	}

	t := tables[0]

	if t.Directory && !validTableName(lognam) {
		return false, status(vmserrors.SS_IVLOGNAM)
	}

	if mode < t.Mode {
		return false, status(vmserrors.SS_NOPRIV)
	}

	superseded, err = db.clearForNewName(t, lognam, mode, attr&AttrNoAlias != 0, nil)
	if err != nil {
		return false, err
	}

	attr |= t.Attrs & AttrConfine

	t.add(&Entry{
		Name:         lognam,
		Mode:         mode,
		Attrs:        attr,
		Equivalences: append([]Equivalence(nil), eqv...),
	})

	db.tracef("define %s [%s] in %s = %v (superseded=%v)", lognam, mode, t.Name, eqv, superseded)

	return superseded, nil
}

// clearForNewName makes room in t for a new entry named name at mode:
// it fails with SS$_DUPLNAM if an inner-mode entry has AttrNoAlias,
// removes the entry at mode itself (reporting that as superseded), and,
// when noAlias is set, removes outer-mode entries too. keep, if not nil,
// is a table whose deletion is refused with SS$_PARENT_DEL ($CRELNT
// superseding its own parent). Nothing is removed unless every removal
// is allowed.
func (db *Database) clearForNewName(t *Table, name string, mode Mode, noAlias bool, keep *Table) (bool, error) {
	var (
		doomed     []*Entry
		superseded bool
	)

	for _, e := range t.names[name] {
		switch {
		case e.Mode < mode:
			if e.Attrs&AttrNoAlias != 0 {
				return false, status(vmserrors.SS_DUPLNAM)
			}

		case e.Mode == mode:
			doomed = append(doomed, e)
			superseded = true

		case noAlias:
			doomed = append(doomed, e)
		}
	}

	for _, e := range doomed {
		if err := db.checkRemovable(e, keep); err != nil {
			return false, err
		}
	}

	for _, e := range doomed {
		db.removeEntry(e)
	}

	return superseded, nil
}

// Delete deletes logical names from the tables tabnam designates —
// $DELLNM. mode is the mode actually used; names at mode and at outer
// (less privileged) modes are deleted.
//
//   - With lognam, the tables are searched in order for the first one
//     containing lognam at any mode, and that table's definitions of it
//     at mode or outer are deleted. SS$_NOLOGNAM if no table has the
//     name, or the one that does has it only at inner modes.
//   - With lognam empty, every name at mode or outer is deleted from the
//     first table. The manual limits this to "the first table in the list
//     whose access mode is equal to or less privileged than the caller's",
//     but the standard tables are kernel mode, so taken literally no
//     caller could ever empty the process table (DEASSIGN/ALL). The
//     per-name mode test already keeps inner-mode names safe, so the
//     table's own mode isn't checked.
//
// Deleting a table-name entry deletes the table and its descendants. The
// tables created at startup can't be deleted (SS$_NOPRIV); nothing is
// deleted if any requested deletion is refused. It returns how many
// entries were deleted (not counting entries inside deleted tables).
//
// Other errors: SS$_IVLOGNAM for a lognam longer than 255 characters, and
// those of ResolveTables.
func (db *Database) Delete(tabnam, lognam string, mode Mode) (int, error) {
	if len(lognam) > MaxNameLength {
		return 0, status(vmserrors.SS_IVLOGNAM)
	}

	tables, err := db.ResolveTables(tabnam, User)
	if err != nil {
		return 0, err
	}

	var doomed []*Entry

	if lognam != "" {
		for _, t := range tables {
			if !t.hasName(lognam) {
				continue
			}

			for _, e := range t.names[lognam] {
				if e.Mode >= mode {
					doomed = append(doomed, e)
				}
			}

			break
		}
	} else {
		for _, e := range tables[0].Entries() {
			if e.Mode >= mode {
				doomed = append(doomed, e)
			}
		}
	}

	if lognam != "" && len(doomed) == 0 {
		return 0, status(vmserrors.SS_NOLOGNAM)
	}

	for _, e := range doomed {
		if err := db.checkRemovable(e, nil); err != nil {
			return 0, err
		}
	}

	for _, e := range doomed {
		db.tracef("delete %s [%s] from %s", e.Name, e.Mode, e.Table.Name)
		db.removeEntry(e)
	}

	return len(doomed), nil
}

// checkRemovable reports whether e may be removed: a table-name entry
// for a permanent table can't be (SS$_NOPRIV), nor one whose removal
// would delete keep, i.e. keep is that table or a descendant of it
// (SS$_PARENT_DEL).
func (db *Database) checkRemovable(e *Entry, keep *Table) error {
	if !e.IsTable() {
		return nil
	}

	if e.Target.permanent {
		return status(vmserrors.SS_NOPRIV)
	}

	for t := keep; t != nil; t = t.Parent {
		if t == e.Target {
			return status(vmserrors.SS_PARENT_DEL)
		}
	}

	return nil
}

// removeEntry removes e from its table; if e names a table, that table
// and its descendants are deleted too.
func (db *Database) removeEntry(e *Entry) {
	e.Table.remove(e)

	if e.IsTable() {
		db.dropTable(e.Target)
	}
}

// dropTable deletes t and, first, each of its descendants along with the
// directory entries naming them. t's own directory entry is the caller's
// to remove.
func (db *Database) dropTable(t *Table) {
	for _, c := range db.Children(t) {
		if c.catalog != nil {
			c.catalog.Table.remove(c.catalog)
		}

		db.dropTable(c)
	}

	for i, x := range db.tables {
		if x == t {
			db.tables = append(db.tables[:i], db.tables[i+1:]...)

			break
		}
	}
}

// validTableName reports whether name may appear in a directory table:
// 1 to 31 characters, each a letter, digit, "$" or "_".
func validTableName(name string) bool {
	if name == "" || len(name) > MaxTableNameLength {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]

		ok := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '$' || c == '_'
		if !ok {
			return false
		}
	}

	return true
}
