package lnm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// testUIC is [123,45] (octal), so the group table is LNM$GROUP_000123.
const testUIC = 0o123<<16 | 0o45

const groupTable = "LNM$GROUP_000123"

// jobTable is the first job's table, and fileDev what LNM$FILE_DEV
// resolves to in the first process's view.
const (
	jobTable = "LNM$JOB_80000100"
	fileDev  = ProcessTableName + "," + jobTable + "," + groupTable + "," + SystemTableName
)

// wantStatus fails t unless err is a VMSError carrying code.
func wantStatus(t *testing.T, err error, code uint32) {
	t.Helper()

	var ve vmserrors.VMSError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want VMS status %d", err, code)
	}

	if ve.Status != code {
		t.Fatalf("status = %d (%v), want %d (%v)", ve.Status, ve, code, vmserrors.New(code))
	}
}

func mustDefine(t *testing.T, db *Database, tabnam, lognam string, mode Mode, values ...string) {
	t.Helper()

	eqv := make([]Equivalence, 0, len(values))
	for _, v := range values {
		eqv = append(eqv, Equivalence{Value: v})
	}

	if _, err := db.Define(tabnam, lognam, mode, 0, eqv); err != nil {
		t.Fatalf("Define(%s, %s): %v", tabnam, lognam, err)
	}
}

func tableNames(tables []*Table) string {
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name)
	}

	return strings.Join(names, ",")
}

func mustResolve(t *testing.T, db *Database, tabnam string) string {
	t.Helper()

	tables, err := db.ResolveTables(tabnam, User)
	if err != nil {
		t.Fatalf("ResolveTables(%s): %v", tabnam, err)
	}

	return tableNames(tables)
}

func mustTranslate(t *testing.T, db *Database, tabnam, lognam string) *Entry {
	t.Helper()

	e, err := db.Translate(tabnam, lognam, User, 0)
	if err != nil {
		t.Fatalf("Translate(%s, %s): %v", tabnam, lognam, err)
	}

	return e
}

func TestNewDatabase_standardTables(t *testing.T) {
	db := NewDatabase(testUIC)

	if db.GroupTableName != groupTable {
		t.Errorf("GroupTableName = %s, want %s", db.GroupTableName, groupTable)
	}

	if db.JobTableName != jobTable {
		t.Errorf("JobTableName = %s, want %s", db.JobTableName, jobTable)
	}

	cases := map[string]string{
		"LNM$PROCESS":        ProcessTableName,
		ProcessTableName:     ProcessTableName,
		"LNM$JOB":            jobTable,
		jobTable:             jobTable,
		"LNM$GROUP":          groupTable,
		"LNM$SYSTEM":         SystemTableName,
		SystemTableName:      SystemTableName,
		"LNM$FILE_DEV":       fileDev,
		"LNM$DCL_LOGICAL":    fileDev,
		"LNM$DIRECTORIES":    ProcessDirectoryName + "," + SystemDirectoryName,
		ProcessDirectoryName: ProcessDirectoryName,
		SystemDirectoryName:  SystemDirectoryName,

		"LNM$TEMPORARY_MAILBOX": jobTable,
		"LNM$PERMANENT_MAILBOX": SystemTableName,
	}

	for tabnam, want := range cases {
		if got := mustResolve(t, db, tabnam); got != want {
			t.Errorf("ResolveTables(%s) = %s, want %s", tabnam, got, want)
		}
	}

	if got := len(db.Tables()); got != 6 {
		t.Errorf("%d tables, want 6", got)
	}

	proc := db.ProcessDirectory.lookup(ProcessTableName, User, false).Target
	if proc.Shareable || proc.Parent != db.ProcessDirectory {
		t.Errorf("process table: shareable=%v parent=%v", proc.Shareable, proc.Parent.Name)
	}

	sys := db.SystemDirectory.lookup(SystemTableName, User, false).Target
	if !sys.Shareable || sys.Parent != db.SystemDirectory {
		t.Errorf("system table: shareable=%v parent=%v", sys.Shareable, sys.Parent.Name)
	}

	job := db.SystemDirectory.lookup(jobTable, User, false).Target
	if !job.Shareable || job.Parent != db.SystemDirectory {
		t.Errorf("job table: shareable=%v parent=%v", job.Shareable, job.Parent.Name)
	}

	if got := tableNames(db.Children(db.SystemDirectory)); got != SystemTableName+","+groupTable+","+jobTable {
		t.Errorf("Children(system directory) = %s", got)
	}
}

func TestResolveTables_errors(t *testing.T) {
	db := NewDatabase(testUIC)

	_, err := db.ResolveTables("", User)
	wantStatus(t, err, vmserrors.SS_IVLOGNAM)

	_, err = db.ResolveTables(strings.Repeat("X", 256), User)
	wantStatus(t, err, vmserrors.SS_IVLOGNAM)

	_, err = db.ResolveTables("NO_SUCH_TABLE", User)
	wantStatus(t, err, vmserrors.SS_NOLOGTAB)

	// A directory name that leads to nothing that is a table.
	mustDefine(t, db, ProcessDirectoryName, "NOT_A_TABLE", Supervisor, "NOWHERE")

	_, err = db.ResolveTables("NOT_A_TABLE", User)
	wantStatus(t, err, vmserrors.SS_IVLOGTAB)

	// A circular definition ends at the depth limit.
	mustDefine(t, db, ProcessDirectoryName, "LOOP_A", Supervisor, "LOOP_B")
	mustDefine(t, db, ProcessDirectoryName, "LOOP_B", Supervisor, "LOOP_A")

	_, err = db.ResolveTables("LOOP_A", User)
	wantStatus(t, err, vmserrors.SS_TOOMANYLNAM)
}

func TestResolveTables_depthLimit(t *testing.T) {
	db := NewDatabase(testUIC)

	// T0 -> T1 -> ... -> Tn -> LNM$PROCESS_TABLE: MaxDepth levels of
	// indirection above the table name resolve; one more doesn't.
	chain := func(n int) string {
		prev := ProcessTableName

		for i := n; i >= 1; i-- {
			name := "T" + strings.Repeat("X", n) + "_" + string(rune('A'+i))
			mustDefine(t, db, ProcessDirectoryName, name, Supervisor, prev)
			prev = name
		}

		return prev
	}

	if got := mustResolve(t, db, chain(MaxDepth)); got != ProcessTableName {
		t.Errorf("depth %d resolved to %s", MaxDepth, got)
	}

	_, err := db.ResolveTables(chain(MaxDepth+1), User)
	wantStatus(t, err, vmserrors.SS_TOOMANYLNAM)
}

// TestResolveTables_redefineProcess is the User's Manual §11.9.2 example:
// adding a private table to the front of LNM$PROCESS also adds it to
// every search list built on LNM$PROCESS, such as LNM$FILE_DEV.
func TestResolveTables_redefineProcess(t *testing.T) {
	db := NewDatabase(testUIC)

	if _, _, err := db.CreateTable("APPLICATION_NAMES", "LNM$PROCESS_TABLE", Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	mustDefine(t, db, ProcessDirectoryName, "LNM$PROCESS", Supervisor, "APPLICATION_NAMES", ProcessTableName)

	want := "APPLICATION_NAMES," + fileDev
	if got := mustResolve(t, db, "LNM$FILE_DEV"); got != want {
		t.Errorf("LNM$FILE_DEV = %s, want %s", got, want)
	}

	// The executive-mode definition is still there underneath.
	tables, err := db.ResolveTables("LNM$FILE_DEV", Executive)
	if err != nil {
		t.Fatal(err)
	}

	if got := tableNames(tables); got != fileDev {
		t.Errorf("LNM$FILE_DEV at exec = %s", got)
	}
}

func TestResolveTables_skipsStaleSearchListElement(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, SystemDirectoryName, "LNM$FILE_DEV", Supervisor, "LNM$PROCESS", "LNM$NOSUCH", "LNM$SYSTEM")

	if got := mustResolve(t, db, "LNM$FILE_DEV"); got != ProcessTableName+","+SystemTableName {
		t.Errorf("LNM$FILE_DEV = %s", got)
	}
}

func TestTranslate_searchOrder(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, "LNM$SYSTEM", "DISK", Executive, "DUA2:")

	if e := mustTranslate(t, db, "LNM$FILE_DEV", "DISK"); e.Table.Name != SystemTableName {
		t.Errorf("found in %s, want system table", e.Table.Name)
	}

	mustDefine(t, db, "LNM$GROUP", "DISK", Supervisor, "DUA1:")

	if e := mustTranslate(t, db, "LNM$FILE_DEV", "DISK"); e.Table.Name != groupTable {
		t.Errorf("found in %s, want group table", e.Table.Name)
	}

	mustDefine(t, db, "LNM$PROCESS", "DISK", Supervisor, "DUA0:")

	e := mustTranslate(t, db, "LNM$FILE_DEV", "DISK")
	if e.Table.Name != ProcessTableName || e.Equivalences[0].Value != "DUA0:" {
		t.Errorf("found %q in %s, want DUA0: in the process table", e.Equivalences[0].Value, e.Table.Name)
	}

	// Naming one table searches only that one.
	if e := mustTranslate(t, db, "LNM$SYSTEM", "DISK"); e.Equivalences[0].Value != "DUA2:" {
		t.Errorf("LNM$SYSTEM DISK = %s", e.Equivalences[0].Value)
	}

	_, err := db.Translate("LNM$FILE_DEV", "NOT_DEFINED", User, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)
}

// TestTranslate_accessModes follows $TRNLNM's acmode rules, using the
// User's Manual's example of one name at two modes in the process table.
func TestTranslate_accessModes(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, "LNM$PROCESS", "ACCOUNTS", Supervisor, "DISK1:[ACCOUNTS]CURRENT.DAT")
	mustDefine(t, db, "LNM$PROCESS", "ACCOUNTS", Executive, "DISK1:[JANE.ACCOUNTS]OBSOLETE.DAT")

	cases := []struct {
		mode Mode
		want string
	}{
		{User, "DISK1:[ACCOUNTS]CURRENT.DAT"},       // outermost wins
		{Supervisor, "DISK1:[ACCOUNTS]CURRENT.DAT"}, // super is allowed
		{Executive, "DISK1:[JANE.ACCOUNTS]OBSOLETE.DAT"},
	}

	for _, c := range cases {
		e, err := db.Translate("LNM$FILE_DEV", "ACCOUNTS", c.mode, 0)
		if err != nil {
			t.Fatalf("mode %s: %v", c.mode, err)
		}

		if got := e.Equivalences[0].Value; got != c.want {
			t.Errorf("mode %s: %s, want %s", c.mode, got, c.want)
		}
	}

	// Neither definition is visible at kernel mode.
	_, err := db.Translate(ProcessTableName, "ACCOUNTS", Kernel, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	// Table names are filtered the same way: LNM$FILE_DEV is an
	// executive-mode name, so a kernel-mode lookup can't use it.
	_, err = db.Translate("LNM$FILE_DEV", "ACCOUNTS", Kernel, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGTAB)
}

func TestTranslate_caseBlind(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, "LNM$PROCESS", "Mixed_Case", Supervisor, "X")

	_, err := db.Translate("LNM$FILE_DEV", "MIXED_CASE", User, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	if _, err := db.Translate("LNM$FILE_DEV", "MIXED_CASE", User, AttrCaseBlind); err != nil {
		t.Errorf("case-blind: %v", err)
	}

	_, err = db.Translate("LNM$FILE_DEV", "X", User, AttrTerminal)
	wantStatus(t, err, vmserrors.SS_BADPARAM)

	_, err = db.Translate("LNM$FILE_DEV", "", User, 0)
	wantStatus(t, err, vmserrors.SS_IVLOGNAM)
}

func TestDefine_searchListAndAttributes(t *testing.T) {
	db := NewDatabase(testUIC)

	eqv := []Equivalence{
		{Value: "[JONES.HISTORY]"},
		{Value: "[JONES.WORKFILES]", Attrs: AttrTerminal},
	}

	if _, err := db.Define("LNM$PROCESS", "GETTYSBURG", Supervisor, AttrNoAlias, eqv); err != nil {
		t.Fatal(err)
	}

	// The caller's slice isn't retained.
	eqv[0].Value = "CHANGED"

	e := mustTranslate(t, db, "LNM$FILE_DEV", "GETTYSBURG")
	if len(e.Equivalences) != 2 || e.Equivalences[0].Value != "[JONES.HISTORY]" || e.Equivalences[1].Attrs != AttrTerminal {
		t.Errorf("equivalences = %+v", e.Equivalences)
	}

	if e.Attrs != AttrNoAlias || e.Mode != Supervisor || e.IsTable() {
		t.Errorf("entry attrs=%#x mode=%s table=%v", e.Attrs, e.Mode, e.IsTable())
	}
}

func TestDefine_validation(t *testing.T) {
	db := NewDatabase(testUIC)

	one := []Equivalence{{Value: "X"}}
	many := make([]Equivalence, MaxEquivalences+1)

	for i := range many {
		many[i] = Equivalence{Value: "X"}
	}

	cases := []struct {
		name   string
		tabnam string
		lognam string
		attr   uint32
		eqv    []Equivalence
		code   uint32
	}{
		{"empty name", "LNM$PROCESS", "", 0, one, vmserrors.SS_IVLOGNAM},
		{"long name", "LNM$PROCESS", strings.Repeat("N", 256), 0, one, vmserrors.SS_IVLOGNAM},
		{"no equivalences", "LNM$PROCESS", "N", 0, nil, vmserrors.SS_BADPARAM},
		{"too many equivalences", "LNM$PROCESS", "N", 0, many, vmserrors.SS_BADPARAM},
		{"empty equivalence", "LNM$PROCESS", "N", 0, []Equivalence{{}}, vmserrors.SS_IVLOGNAM},
		{"long equivalence", "LNM$PROCESS", "N", 0, []Equivalence{{Value: strings.Repeat("V", 256)}}, vmserrors.SS_IVLOGNAM},
		{"bad name attribute", "LNM$PROCESS", "N", AttrTerminal, one, vmserrors.SS_BADPARAM},
		{"bad translation attribute", "LNM$PROCESS", "N", 0, []Equivalence{{Value: "X", Attrs: AttrNoAlias}}, vmserrors.SS_BADPARAM},
		{"no such table", "NO_SUCH_TABLE", "N", 0, one, vmserrors.SS_NOLOGTAB},
		{"directory name syntax", ProcessDirectoryName, "BAD NAME", 0, one, vmserrors.SS_IVLOGNAM},
		{"directory name length", ProcessDirectoryName, strings.Repeat("D", 32), 0, one, vmserrors.SS_IVLOGNAM},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := db.Define(c.tabnam, c.lognam, Supervisor, c.attr, c.eqv)
			wantStatus(t, err, c.code)
		})
	}

	// 255-character names and exactly 128 equivalences are allowed.
	if _, err := db.Define("LNM$PROCESS", strings.Repeat("N", 255), Supervisor, 0, many[:MaxEquivalences]); err != nil {
		t.Errorf("limits: %v", err)
	}
}

func TestDefine_supersede(t *testing.T) {
	db := NewDatabase(testUIC)

	one := func(v string) []Equivalence { return []Equivalence{{Value: v}} }

	if sup, err := db.Define("LNM$PROCESS", "PAY", Supervisor, 0, one("A")); err != nil || sup {
		t.Fatalf("first define: superseded=%v err=%v", sup, err)
	}

	if sup, err := db.Define("LNM$PROCESS", "PAY", Supervisor, 0, one("B")); err != nil || !sup {
		t.Fatalf("second define: superseded=%v err=%v", sup, err)
	}

	// A different mode is a separate definition, not a supersede.
	if sup, err := db.Define("LNM$PROCESS", "PAY", Executive, 0, one("C")); err != nil || sup {
		t.Fatalf("exec define: superseded=%v err=%v", sup, err)
	}

	if got := mustTranslate(t, db, "LNM$PROCESS", "PAY").Equivalences[0].Value; got != "B" {
		t.Errorf("PAY = %s, want B", got)
	}

	if n := len(db.ProcessDirectory.lookup(ProcessTableName, User, false).Target.names["PAY"]); n != 2 {
		t.Errorf("%d PAY entries, want 2", n)
	}
}

func TestDefine_noAlias(t *testing.T) {
	db := NewDatabase(testUIC)

	one := []Equivalence{{Value: "X"}}

	// An outer-mode name is deleted by an inner NO_ALIAS definition.
	mustDefine(t, db, "LNM$PROCESS", "N", User, "OUTER")

	if _, err := db.Define("LNM$PROCESS", "N", Executive, AttrNoAlias, one); err != nil {
		t.Fatal(err)
	}

	if e := mustTranslate(t, db, "LNM$PROCESS", "N"); e.Mode != Executive {
		t.Errorf("N is at %s, want exec", e.Mode)
	}

	// ... and blocks later outer-mode definitions.
	_, err := db.Define("LNM$PROCESS", "N", Supervisor, 0, one)
	wantStatus(t, err, vmserrors.SS_DUPLNAM)
}

func TestDefine_tableRules(t *testing.T) {
	db := NewDatabase(testUIC)

	tab, _, err := db.CreateTable("SUPER_TABLE", ProcessTableName, Supervisor, AttrConfine)
	if err != nil {
		t.Fatal(err)
	}

	// A name can't be more privileged than its table.
	_, err = db.Define("SUPER_TABLE", "N", Executive, 0, []Equivalence{{Value: "X"}})
	wantStatus(t, err, vmserrors.SS_NOPRIV)

	// A CONFINE table's names are CONFINE.
	mustDefine(t, db, "SUPER_TABLE", "N", User, "X")

	if e := tab.lookup("N", User, false); e.Attrs&AttrConfine == 0 {
		t.Errorf("N attrs = %#x, want CONFINE", e.Attrs)
	}

	// A search list of tables defines into the first.
	mustDefine(t, db, "LNM$FILE_DEV", "FIRST", Supervisor, "X")

	if e := mustTranslate(t, db, "LNM$FILE_DEV", "FIRST"); e.Table.Name != ProcessTableName {
		t.Errorf("FIRST defined in %s", e.Table.Name)
	}
}

func TestDelete_byName(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, "LNM$PROCESS", "N", User, "U")
	mustDefine(t, db, "LNM$PROCESS", "N", Supervisor, "S")
	mustDefine(t, db, "LNM$PROCESS", "N", Executive, "E")
	mustDefine(t, db, "LNM$SYSTEM", "N", Executive, "SYS")

	// Supervisor mode deletes the supervisor and user definitions in the
	// first table that has the name, and leaves the rest.
	n, err := db.Delete("LNM$FILE_DEV", "N", Supervisor)
	if err != nil || n != 2 {
		t.Fatalf("Delete = %d, %v; want 2, nil", n, err)
	}

	if e := mustTranslate(t, db, "LNM$FILE_DEV", "N"); e.Equivalences[0].Value != "E" {
		t.Errorf("N = %s, want E", e.Equivalences[0].Value)
	}

	// Only an inner-mode definition is left in that table.
	_, err = db.Delete("LNM$FILE_DEV", "N", Supervisor)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	_, err = db.Delete("LNM$FILE_DEV", "NEVER_DEFINED", User)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	_, err = db.Delete("LNM$FILE_DEV", strings.Repeat("N", 256), User)
	wantStatus(t, err, vmserrors.SS_IVLOGNAM)
}

func TestDelete_allInTable(t *testing.T) {
	db := NewDatabase(testUIC)

	mustDefine(t, db, "LNM$PROCESS", "A", Supervisor, "1")
	mustDefine(t, db, "LNM$PROCESS", "B", User, "2")
	mustDefine(t, db, "LNM$PROCESS", "C", Executive, "3")

	n, err := db.Delete("LNM$PROCESS", "", Supervisor)
	if err != nil || n != 2 {
		t.Fatalf("Delete = %d, %v; want 2, nil", n, err)
	}

	proc := db.ProcessDirectory.lookup(ProcessTableName, User, false).Target
	if got := len(proc.Entries()); got != 1 {
		t.Errorf("%d entries left, want 1 (C)", got)
	}
}

func TestDelete_permanentTablesProtected(t *testing.T) {
	db := NewDatabase(testUIC)

	// A user-mode delete can't reach the kernel-mode table name at all.
	_, err := db.Delete(ProcessDirectoryName, ProcessTableName, User)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	// A kernel-mode one can, but the table is protected.
	_, err = db.Delete(ProcessDirectoryName, ProcessTableName, Kernel)
	wantStatus(t, err, vmserrors.SS_NOPRIV)

	// Deleting everything in a directory fails as a whole.
	before := len(db.ProcessDirectory.Entries())

	_, err = db.Delete(ProcessDirectoryName, "", Kernel)
	wantStatus(t, err, vmserrors.SS_NOPRIV)

	if after := len(db.ProcessDirectory.Entries()); after != before {
		t.Errorf("process directory went from %d to %d entries", before, after)
	}

	// Superseding one isn't allowed either.
	_, err = db.Define(ProcessDirectoryName, ProcessTableName, Kernel, 0, []Equivalence{{Value: "X"}})
	wantStatus(t, err, vmserrors.SS_NOPRIV)
}

func TestDelete_tableDeletesSubtables(t *testing.T) {
	db := NewDatabase(testUIC)

	for _, c := range []struct{ name, parent string }{
		{"APP", ProcessTableName},
		{"APP_CHILD", "APP"},
		{"APP_GRANDCHILD", "APP_CHILD"},
		{"OTHER", ProcessTableName},
	} {
		if _, _, err := db.CreateTable(c.name, c.parent, Supervisor, 0); err != nil {
			t.Fatalf("CreateTable(%s): %v", c.name, err)
		}
	}

	mustDefine(t, db, "APP_GRANDCHILD", "N", Supervisor, "X")

	// APP's directory entry is supervisor mode, out of a user-mode
	// delete's reach.
	_, err := db.Delete(ProcessDirectoryName, "APP", User)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	if _, err := db.Delete(ProcessDirectoryName, "APP", Supervisor); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"APP", "APP_CHILD", "APP_GRANDCHILD"} {
		_, err := db.ResolveTables(name, User)
		wantStatus(t, err, vmserrors.SS_NOLOGTAB)
	}

	if got := mustResolve(t, db, "OTHER"); got != "OTHER" {
		t.Errorf("OTHER = %s", got)
	}

	if got := len(db.Tables()); got != 7 {
		t.Errorf("%d tables, want the 6 standard ones plus OTHER", got)
	}
}

func TestCreateTable_placement(t *testing.T) {
	db := NewDatabase(testUIC)

	priv, res, err := db.CreateTable("PRIVATE", ProcessTableName, Supervisor, 0)
	if err != nil || res != TableCreated {
		t.Fatalf("PRIVATE: %v, %v", res, err)
	}

	if priv.Shareable || priv.Parent.Name != ProcessTableName || db.ProcessDirectory.atMode("PRIVATE", Supervisor) == nil {
		t.Errorf("PRIVATE: shareable=%v parent=%s", priv.Shareable, priv.Parent.Name)
	}

	shared, _, err := db.CreateTable("SHARED", "LNM$SYSTEM", Executive, AttrConfine)
	if err != nil {
		t.Fatal(err)
	}

	if !shared.Shareable || shared.Attrs != 0 || db.SystemDirectory.atMode("SHARED", Executive) == nil {
		t.Errorf("SHARED: shareable=%v attrs=%#x", shared.Shareable, shared.Attrs)
	}

	// CONFINE is inherited from a process-private parent.
	if _, _, err := db.CreateTable("CONFINED", ProcessTableName, Supervisor, AttrConfine); err != nil {
		t.Fatal(err)
	}

	child, _, err := db.CreateTable("CONFINED_CHILD", "CONFINED", Supervisor, 0)
	if err != nil {
		t.Fatal(err)
	}

	if child.Attrs&AttrConfine == 0 {
		t.Errorf("CONFINED_CHILD didn't inherit CONFINE")
	}

	// A default name.
	anon, _, err := db.CreateTable("", ProcessTableName, Supervisor, 0)
	if err != nil || anon.Name != "LNM$0001" {
		t.Errorf("default name = %v, %v", anon, err)
	}
}

func TestCreateTable_existingNames(t *testing.T) {
	db := NewDatabase(testUIC)

	first, _, err := db.CreateTable("APP", ProcessTableName, Supervisor, 0)
	if err != nil {
		t.Fatal(err)
	}

	mustDefine(t, db, "APP", "N", Supervisor, "X")

	// CREATE_IF keeps the existing table.
	same, res, err := db.CreateTable("APP", ProcessTableName, Supervisor, AttrCreateIf)
	if err != nil || res != TableExisted || same != first {
		t.Fatalf("CREATE_IF: %v, %v, same=%v", res, err, same == first)
	}

	// Without it, the table is replaced and its names are gone.
	_, res, err = db.CreateTable("APP", ProcessTableName, Supervisor, 0)
	if err != nil || res != TableSuperseded {
		t.Fatalf("supersede: %v, %v", res, err)
	}

	_, err = db.Translate("APP", "N", User, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	// A table can't supersede its own parent.
	if _, _, err := db.CreateTable("APP_CHILD", "APP", Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	_, _, err = db.CreateTable("APP", "APP_CHILD", Supervisor, 0)
	wantStatus(t, err, vmserrors.SS_PARENT_DEL)

	// ... or a standard table.
	_, _, err = db.CreateTable(ProcessTableName, ProcessDirectoryName, Kernel, 0)
	wantStatus(t, err, vmserrors.SS_NOPRIV)
}

func TestCreateTable_validation(t *testing.T) {
	db := NewDatabase(testUIC)

	if _, _, err := db.CreateTable("SUPER", ProcessTableName, Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, tabnam, partab string
		mode                 Mode
		attr                 uint32
		code                 uint32
	}{
		{"bad table name", "BAD NAME", ProcessTableName, Supervisor, 0, vmserrors.SS_IVLOGTAB},
		{"long table name", strings.Repeat("T", 32), ProcessTableName, Supervisor, 0, vmserrors.SS_IVLOGTAB},
		{"no parent", "T", "", Supervisor, 0, vmserrors.SS_IVLOGNAM},
		{"missing parent", "T", "NO_SUCH_TABLE", Supervisor, 0, vmserrors.SS_NOLOGTAB},
		{"bad attribute", "T", ProcessTableName, Supervisor, AttrTerminal, vmserrors.SS_BADPARAM},
		{"more privileged than parent", "T", "SUPER", Executive, 0, vmserrors.SS_NOPRIV},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := db.CreateTable(c.tabnam, c.partab, c.mode, c.attr)
			wantStatus(t, err, c.code)
		})
	}
}

func TestTrace(t *testing.T) {
	var lines []string

	db := NewDatabase(testUIC)

	db.Trace = func(format string, args ...any) { lines = append(lines, format) }

	mustDefine(t, db, "LNM$PROCESS", "N", Supervisor, "X")
	mustTranslate(t, db, "LNM$FILE_DEV", "N")

	if len(lines) != 2 {
		t.Errorf("%d trace lines, want 2", len(lines))
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"SYS$OUTPUT", "SYS$OUTPUT", true},
		{"SYS$OUTPUT", "SYS$INPUT", false},
		{"SYS$*", "SYS$OUTPUT", true},
		{"SYS$*", "SYS$", true},
		{"*", "", true},
		{"*PUT", "SYS$OUTPUT", true},
		{"*PUT", "SYS$OUTPUTX", false},
		{"S*$*T", "SYS$OUTPUT", true},
		{"%%", "TT", true},
		{"%%", "T", false},
		{"TT%", "TT", false},
		{"*A*B*", "XAYBZ", true},
		{"*A*B*", "XBYAZ", false},
		{"sys$*", "SYS$OUTPUT", false},
	}

	for _, c := range cases {
		if got := Match(c.pattern, c.name); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}

	if !HasWildcards("A*") || !HasWildcards("A%") || HasWildcards("A") {
		t.Error("HasWildcards")
	}
}

func TestMode_String(t *testing.T) {
	for m, want := range map[Mode]string{Kernel: "kernel", Executive: "exec", Supervisor: "super", User: "user"} {
		if got := m.String(); got != want {
			t.Errorf("%d.String() = %s, want %s", m, got, want)
		}
	}
}

func TestDefineProcessNames(t *testing.T) {
	db := NewDatabase(testUIC)
	if err := db.DefineProcessNames("_TTA0:"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"SYS$INPUT", "SYS$OUTPUT", "SYS$ERROR", "SYS$COMMAND", "TT"} {
		e := mustTranslate(t, db, "LNM$FILE_DEV", name)
		if e.Table.Name != ProcessTableName || e.Equivalences[0].Value != "_TTA0:" {
			t.Errorf("%s = %+v in %s", name, e.Equivalences, e.Table.Name)
		}

		wantMode, wantAttrs := Executive, AttrTerminal
		if name == "TT" {
			wantMode, wantAttrs = Supervisor, 0
		}

		if e.Mode != wantMode || e.Equivalences[0].Attrs != wantAttrs {
			t.Errorf("%s: mode %s attrs %#x, want %s %#x", name, e.Mode, e.Equivalences[0].Attrs, wantMode, wantAttrs)
		}
	}
}

// TestProcessView_sharing: a second process's view (a subprocess, in
// the first's job) has a process table of its own, and shares the job,
// group, and system tables: a name defined in the subprocess's process
// table isn't seen by its parent, and one in the job table is
// (docs/PHASE-45.md, subtask 3).
func TestProcessView_sharing(t *testing.T) {
	parent := NewDatabase(testUIC)
	child := parent.NewProcessView(testUIC, parent.JobTableName)

	if child.ProcessDirectory == parent.ProcessDirectory || child.SystemDirectory != parent.SystemDirectory {
		t.Fatal("the views share a process directory, or don't share the system directory")
	}

	if got := mustResolve(t, child, "LNM$FILE_DEV"); got != fileDev {
		t.Errorf("child's LNM$FILE_DEV = %s, want %s", got, fileDev)
	}

	mustDefine(t, child, "LNM$PROCESS", "MINE", Supervisor, "CHILD")
	mustDefine(t, child, "LNM$JOB", "OURS", Supervisor, "SHARED")
	mustDefine(t, parent, "LNM$PROCESS", "MINE", Supervisor, "PARENT")

	if e := mustTranslate(t, parent, "LNM$FILE_DEV", "MINE"); e.Equivalences[0].Value != "PARENT" {
		t.Errorf("the parent sees MINE = %s, want its own", e.Equivalences[0].Value)
	}

	if e := mustTranslate(t, child, "LNM$FILE_DEV", "MINE"); e.Equivalences[0].Value != "CHILD" {
		t.Errorf("the child sees MINE = %s, want its own", e.Equivalences[0].Value)
	}

	if e := mustTranslate(t, parent, "LNM$FILE_DEV", "OURS"); e.Table.Name != jobTable {
		t.Errorf("the parent finds the job's OURS in %s, want %s", e.Table.Name, jobTable)
	}

	// Deleting the child's process names leaves the parent's.
	if _, err := child.Delete(ProcessTableName, "MINE", Supervisor); err != nil {
		t.Fatal(err)
	}

	mustTranslate(t, parent, ProcessTableName, "MINE")

	// A process-private table belongs to its process; a shareable one,
	// to everyone.
	if _, _, err := child.CreateTable("CHILD_TABLE", "LNM$PROCESS_TABLE", Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	if _, _, err := child.CreateTable("COMMON_TABLE", "LNM$SYSTEM_TABLE", Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	_, err := parent.ResolveTables("CHILD_TABLE", User)
	wantStatus(t, err, vmserrors.SS_NOLOGTAB)

	if got := mustResolve(t, parent, "COMMON_TABLE"); got != "COMMON_TABLE" {
		t.Errorf("the parent's COMMON_TABLE = %s", got)
	}

	if got := len(parent.Tables()); got != 7 {
		t.Errorf("the parent sees %d tables, want its 6 and COMMON_TABLE", got)
	}
}

// TestProcessView_jobsAndGroups: a process in another job has another
// job table, and a process in a new UIC group gets its group's table,
// which a later process of the group shares.
func TestProcessView_jobsAndGroups(t *testing.T) {
	first := NewDatabase(testUIC)

	const otherUIC = 0o200<<16 | 1

	detached := first.NewProcessView(otherUIC, first.NewJobTable())
	if detached.JobTableName != "LNM$JOB_80000200" || detached.GroupTableName != "LNM$GROUP_000200" {
		t.Fatalf("second job: %s, %s", detached.JobTableName, detached.GroupTableName)
	}

	mustDefine(t, first, "LNM$JOB", "JOBNAME", Supervisor, "FIRST")

	_, err := detached.Translate("LNM$FILE_DEV", "JOBNAME", User, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	mustDefine(t, detached, "LNM$GROUP", "GROUPNAME", Supervisor, "G200")

	sibling := first.NewProcessView(otherUIC, first.JobTableName)
	if e := mustTranslate(t, sibling, "LNM$FILE_DEV", "GROUPNAME"); e.Table.Name != "LNM$GROUP_000200" {
		t.Errorf("GROUPNAME found in %s", e.Table.Name)
	}

	if e := mustTranslate(t, sibling, "LNM$FILE_DEV", "JOBNAME"); e.Equivalences[0].Value != "FIRST" {
		t.Errorf("JOBNAME = %s", e.Equivalences[0].Value)
	}

	_, err = first.Translate("LNM$FILE_DEV", "GROUPNAME", User, 0)
	wantStatus(t, err, vmserrors.SS_NOLOGNAM)

	// Default table names are unique across processes.
	a, _, err := first.CreateTable("", "LNM$SYSTEM_TABLE", Supervisor, 0)
	if err != nil {
		t.Fatal(err)
	}

	b, _, err := detached.CreateTable("", "LNM$SYSTEM_TABLE", Supervisor, 0)
	if err != nil {
		t.Fatal(err)
	}

	if a.Name == b.Name {
		t.Errorf("two processes' default table names are both %s", a.Name)
	}
}

// TestDeleteJobTable: deleting a job's table, when its job ends, takes
// its names and any table created under it, and leaves other jobs'
// tables alone.
func TestDeleteJobTable(t *testing.T) {
	first := NewDatabase(testUIC)
	second := first.NewProcessView(testUIC, first.NewJobTable())

	mustDefine(t, second, "LNM$JOB", "JOBNAME", Supervisor, "SECOND")
	mustDefine(t, first, "LNM$JOB", "JOBNAME", Supervisor, "FIRST")

	if _, _, err := second.CreateTable("CHILD_TABLE", second.JobTableName, Supervisor, 0); err != nil {
		t.Fatal(err)
	}

	before := len(first.Tables())

	if !first.DeleteJobTable(second.JobTableName) {
		t.Fatalf("DeleteJobTable(%s) found no table", second.JobTableName)
	}

	if got := len(first.Tables()); got != before-2 {
		t.Errorf("%d tables after the deletion, want %d (the job table and its child gone)", got, before-2)
	}

	for _, name := range []string{second.JobTableName, "CHILD_TABLE"} {
		if _, err := first.ResolveTables(name, User); err == nil {
			t.Errorf("table %s is still there", name)
		}
	}

	if e := mustTranslate(t, first, "LNM$JOB", "JOBNAME"); e.Equivalences[0].Value != "FIRST" {
		t.Errorf("the first job's JOBNAME = %s", e.Equivalences[0].Value)
	}

	if first.DeleteJobTable(second.JobTableName) || first.DeleteJobTable(first.GroupTableName) {
		t.Error("DeleteJobTable deleted a table that isn't a live job table")
	}
}
