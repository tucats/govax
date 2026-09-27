package lnm

import "sort"

// Equivalence is one equivalence string of a logical name, with its own
// translation attributes (AttrConcealed, AttrTerminal).
type Equivalence struct {
	Value string
	Attrs uint32
}

// Entry is one logical name at one access mode within one table.
type Entry struct {
	Name string
	Mode Mode

	// Attrs holds the name's own attributes: AttrNoAlias, AttrConfine,
	// AttrCrelog, and AttrTable for a table-name entry.
	Attrs uint32

	// Equivalences are the name's equivalence strings, by index. A
	// table-name entry has none.
	Equivalences []Equivalence

	// Table is the table this entry is in.
	Table *Table

	// Target is the table a table-name entry names; nil otherwise.
	Target *Table
}

// IsTable reports whether e is the name of a logical name table.
func (e *Entry) IsTable() bool {
	return e.Target != nil
}

// Table is one logical name table, or one of the two directory tables.
type Table struct {
	Name string
	Mode Mode

	// Parent is the table's parent; nil only for a directory.
	Parent *Table

	// Directory marks LNM$PROCESS_DIRECTORY and LNM$SYSTEM_DIRECTORY,
	// whose names are restricted to table-name syntax.
	Directory bool

	// Shareable is true for tables cataloged in LNM$SYSTEM_DIRECTORY
	// (the system and group tables, and their descendants), false for
	// process-private ones.
	Shareable bool

	// Attrs holds AttrConfine when set.
	Attrs uint32

	// catalog is the directory entry that names this table.
	catalog *Entry

	// permanent marks the tables created at startup, which can't be
	// deleted or superseded.
	permanent bool

	// names maps each logical name (case-sensitive) to its entries, one
	// per access mode.
	names map[string][]*Entry
}

func newTable(name string, mode Mode, parent *Table) *Table {
	t := &Table{Name: name, Mode: mode, Parent: parent, names: map[string][]*Entry{}}
	if parent != nil {
		t.Shareable = parent.Shareable
	}

	return t
}

// Entries returns every entry in t, sorted by name and, for one name,
// from the outermost access mode inward — the order SHOW LOGICAL lists
// them in.
func (t *Table) Entries() []*Entry {
	var out []*Entry
	for _, list := range t.names {
		out = append(out, list...)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}

		return out[i].Mode > out[j].Mode
	})

	return out
}

// lookup returns the entry for name that a translation at mode sees: of
// the entries at mode or a more privileged one, the outermost. A name at
// a less privileged mode than mode is ignored, as $TRNLNM specifies;
// passing User considers every entry.
func (t *Table) lookup(name string, mode Mode, caseBlind bool) *Entry {
	var best *Entry

	consider := func(list []*Entry) {
		for _, e := range list {
			if e.Mode <= mode && (best == nil || e.Mode > best.Mode) {
				best = e
			}
		}
	}

	if !caseBlind {
		consider(t.names[name])

		return best
	}

	for n, list := range t.names {
		if equalFoldASCII(n, name) {
			consider(list)
		}
	}

	return best
}

// hasName reports whether any entry named name exists in t, at any mode.
func (t *Table) hasName(name string) bool {
	return len(t.names[name]) > 0
}

// atMode returns the entry for name at exactly mode, or nil.
func (t *Table) atMode(name string, mode Mode) *Entry {
	for _, e := range t.names[name] {
		if e.Mode == mode {
			return e
		}
	}

	return nil
}

func (t *Table) add(e *Entry) {
	e.Table = t
	t.names[e.Name] = append(t.names[e.Name], e)
}

func (t *Table) remove(e *Entry) {
	list := t.names[e.Name]
	for i, x := range list {
		if x == e {
			list = append(list[:i], list[i+1:]...)

			break
		}
	}

	if len(list) == 0 {
		delete(t.names, e.Name)
	} else {
		t.names[e.Name] = list
	}
}

// equalFoldASCII compares a and b ignoring ASCII letter case, matching
// LNM$M_CASE_BLIND (which VMS applies to the DEC Multinational character
// set's letters as well; govax only handles ASCII).
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if upper(a[i]) != upper(b[i]) {
			return false
		}
	}

	return true
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}

	return c
}
