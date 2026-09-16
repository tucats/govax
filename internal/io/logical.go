package io

// Logical-name attribute bits, matching Headers/logicals.h — only the ones
// this package's own Set/Get logic inspects (LNM_M_TABLE, unconditionally
// stamped onto every table header by set_logical regardless of the attr a
// caller passes it; LNM_M_TERMINAL, get_logical's own recursion guard).
const (
	LNMTable    = 0x00000008
	LNMTerminal = 0x00000200
)

// LogicalName is one DEFINE/LOGICAL-created name within a table, the Go
// equivalent of one non-header logical_names.c struct LNM node.
type LogicalName struct {
	Name  string
	Value string
	Attr  uint32
}

// logicalTable is one logical-name table (e.g. "LNM$FILE_DEV"), the Go
// equivalent of a header struct LNM node (whose own `tables` field points
// at its list of name entries). The C source reuses a single struct for
// both the table-header and name-entry roles; this port splits that into
// this unexported type and the exported LogicalName, which is clearer in
// Go and loses no behavior this package's Set/Get ever relied on — see
// Get's own doc comment for the one C code path this split can't (and, on
// inspection, doesn't need to) reproduce.
type logicalTable struct {
	attr  uint32
	names map[string]*LogicalName
	order []string // insertion order, for a stable Show listing
}

// LogicalNameTable is the Go equivalent of logical_names.c's tables list
// of table headers.
type LogicalNameTable struct {
	tables map[string]*logicalTable
	order  []string
}

// NewLogicalNameTable returns an empty logical name table (matching
// tables == 0L) with no default tables seeded — call InitLogicals to
// match the C source's own startup behavior (see its doc comment).
func NewLogicalNameTable() *LogicalNameTable {
	return &LogicalNameTable{tables: map[string]*logicalTable{}}
}

// InitLogicals seeds the default logical name tables, matching
// logical_names.c's init_logicals — which init_symbols.c's
// init_system_symbols calls once, unconditionally, as part of one-time
// process startup (independent of the console's own INIT/vax_init state);
// internal/console.New calls this the same way, once per Console.
func (t *LogicalNameTable) InitLogicals() {
	t.Set("LNM$TABLE", "LNM$FILE_DEV", "<TABLE>", LNMTable)

	t.Set("LNM$FILE_DEV", "SYS$COMMAND", "TTA0:", 0)
	t.Set("LNM$FILE_DEV", "SYS$INPUT", "TTA0:", 0)
	t.Set("LNM$FILE_DEV", "SYS$OUTPUT", "TTA0:", 0)
	t.Set("LNM$FILE_DEV", "SYS$ERROR", "TTA0:", 0)
}

// HasTable reports whether a table named name has been created, letting a
// caller (e.g. internal/rtl's SYS$TRNLNM) distinguish "no such table" from
// "table exists, but no such name in it" — two different SS_ status codes
// on the real service that Get's own single ok bool can't tell apart.
func (t *LogicalNameTable) HasTable(name string) bool {
	_, ok := t.tables[name]

	return ok
}

func (t *LogicalNameTable) table(name string, create bool) *logicalTable {
	if tb, ok := t.tables[name]; ok {
		return tb
	}

	if !create {
		return nil
	}
	
	tb := &logicalTable{attr: LNMTable, names: map[string]*LogicalName{}}
	t.tables[name] = tb
	t.order = append(t.order, name)

	return tb
}

// Set defines name's value within table (auto-creating the table if it
// doesn't exist yet, matching set_logical — whose table header always
// gets attr = LNM_M_TABLE regardless of the attr parameter, since that's
// a literal assignment in the C source's table-creation branch, not the
// caller-supplied value). A name's own attr is fixed at creation time
// only: redefining an existing name's value leaves its attr unchanged,
// matching set_logical's "if (!lnm) { ...; lnm->attr = attr; }" structure
// (the assignment lives inside the creation branch alone).
func (t *LogicalNameTable) Set(table, name, value string, attr uint32) *LogicalName {
	tb := t.table(table, true)

	ln, ok := tb.names[name]
	if !ok {
		ln = &LogicalName{Name: name, Attr: attr}
		tb.names[name] = ln
		tb.order = append(tb.order, name)
	}

	ln.Value = value

	return ln
}

// Get resolves logname within tabnam (defaulting to "LNM$ROOT" when tabnam
// is empty, matching get_logical), applying attr as an exact-match filter
// on the found name's own attr when attr is nonzero (get_logical's own
// "if (attr) if (lnm->attr != attr) continue" — a single equality test,
// not a bitmask subset test, replicated as literally specified).
//
// Not ported: get_logical's recursive LNM$TABLE fallback for a table name
// with no direct header ("if (!lnm_t && !(attr & LNM_M_TERMINAL)) lnm_t =
// get_logical(\"LNM$TABLE\", tabnam, LNM_M_TERMINAL);"). Traced rather than
// guessed at: every table this port ever seeds or defines is registered as
// a *name entry* inside "LNM$TABLE" with attr LNM_M_TABLE (via Set's own
// unconditional table-header stamping, or InitLogicals' literal LNM_M_TABLE
// argument) — never LNM_M_TERMINAL — so the recursive call's own attr
// filter (which requires the found entry's attr to equal LNM_M_TERMINAL
// exactly) can never match anything in this codebase, in the C source or
// here. A no-op path isn't worth reproducing as inert complexity; if a
// future caller ever needs LNM_M_TERMINAL-tagged table aliasing, revisit
// then with a concrete use case in hand.
func (t *LogicalNameTable) Get(tabnam, logname string, attr uint32) (*LogicalName, bool) {
	if tabnam == "" {
		tabnam = "LNM$ROOT"
	}

	tb, ok := t.tables[tabnam]
	if !ok {
		return nil, false
	}

	ln, ok := tb.names[logname]
	if !ok {
		return nil, false
	}

	if attr != 0 && ln.Attr != attr {
		return nil, false
	}

	return ln, true
}

// Delete removes name from table, reporting whether it existed. This has
// no direct C-source equivalent: logical_names.c defines get/set but no
// delete (SYS$DELLNM is listed in p1_vector.c's system-service jump table
// but its routine pointer is 0 — never actually implemented). Added
// because docs/PHASE-09.md's own deliverable names "define/translate/
// delete" as this package's expected test coverage, and it's a natural,
// low-risk CRUD-completeness addition for a data structure this port
// already owns outright — the same kind of Go-native addition as Phase
// 08's DEPOSIT command.
func (t *LogicalNameTable) Delete(table, name string) bool {
	tb, ok := t.tables[table]
	if !ok {
		return false
	}

	if _, ok := tb.names[name]; !ok {
		return false
	}

	delete(tb.names, name)

	for i, n := range tb.order {
		if n == name {
			tb.order = append(tb.order[:i], tb.order[i+1:]...)

			break
		}
	}

	return true
}

// LogicalNameEntry pairs a LogicalName with the table it was found in —
// AllMatching's element type, since a bare *LogicalName doesn't carry its
// own table's name.
type LogicalNameEntry struct {
	Table string
	Name  *LogicalName
}

// AllMatching returns every logical name matching the optional table/name
// filters (either "" means "no filter on that field"), in table-then-name
// insertion order — matching show_logical's own nested-loop enumeration.
func (t *LogicalNameTable) AllMatching(table, name string) []LogicalNameEntry {
	var out []LogicalNameEntry

	for _, tname := range t.order {
		if table != "" && tname != table {
			continue
		}

		tb := t.tables[tname]
		for _, nname := range tb.order {
			if name != "" && nname != name {
				continue
			}

			out = append(out, LogicalNameEntry{Table: tname, Name: tb.names[nname]})
		}
	}

	return out
}
