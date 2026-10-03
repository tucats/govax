package console

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the console's logical-name commands (docs/PHASE-25.md):
// DEFINE and ASSIGN, DEASSIGN, CREATE/NAME_TABLE, SHOW LOGICAL and SHOW
// TRANSLATION, all working on the shared lnm.Database in c.Logicals.
// The DCL bindings that call these are in dispatch.go.

// consoleTerminal is the physical device name the console's own
// terminal has: what SYS$INPUT, SYS$OUTPUT, SYS$ERROR, SYS$COMMAND, and
// TT translate to.
const consoleTerminal = "_TTA0:"

// dclLogicalName is the logical name SHOW LOGICAL and SHOW TRANSLATION
// search by default (LNM$FILE_DEV's tables, as on VMS).
const dclLogicalName = "LNM$DCL_LOGICAL"

// newLogicals returns the console's logical-name database: the standard
// VMS directories and tables for corevms.NominalUIC (the same UIC the RTL
// reports for this process), plus the process-permanent terminal names.
func newLogicals() *lnm.Database {
	db := lnm.NewDatabase(corevms.NominalUIC)
	if err := db.DefineProcessNames(consoleTerminal); err != nil {
		// Only reachable if the fixed names above were invalid.
		panic(err)
	}

	return db
}

// traceLogicals is the logical-name database's Trace hook: it writes
// each line to the CPU's debug writer while SET DEBUG LOGICALS is on.
func (c *Console) traceLogicals(format string, args ...any) {
	if c.CPU != nil && c.CPU.DebugEnabled(vax.DebugLogicals) {
		fmt.Fprintf(c.CPU.DebugWriter(), "DEBUG: LNM: "+format+"\n", args...)
	}
}

// bindLogicalCommands binds the logical-name commands' grammar entries
// (internal/bootdata/files/console.dcl) to c.
func bindLogicalCommands(g *dcl.Grammar, c *Console) {
	define := func(assign bool) dcl.Handler {
		return func(id int64, r *dcl.Result) error {
			name := r.String("NAME")
			if assign {
				// ASSIGN removes one trailing colon; DEFINE keeps it.
				name = strings.TrimSuffix(name, ":")
			}

			var attrs uint32

			for _, a := range r.List("TRANSLATION_ATTRIBUTES") {
				switch a {
				case "CONCEALED":
					attrs |= lnm.AttrConcealed

				case "TERMINAL":
					attrs |= lnm.AttrTerminal
				}
			}

			return c.DefineLogicalName(logicalTableQualifier(r), name, r.List("VALUE"),
				logicalModeQualifier(r), attrs, !r.Negated("LOG"))
		}
	}

	g.Bind("DEFINE", define(false))
	g.Bind("ASSIGN", define(true))

	g.Bind("DEASSIGN", func(id int64, r *dcl.Result) error {
		// Like ASSIGN, DEASSIGN removes one trailing colon.
		name := strings.TrimSuffix(r.String("NAME"), ":")

		return c.DeassignLogicalName(logicalTableQualifier(r), name, logicalModeQualifier(r), r.Present("ALL"))
	})

	// A bare CREATE makes a file from terminal input on VMS, which govax
	// doesn't do; only /NAME_TABLE and /DIRECTORY (create.go) are here.
	g.Bind("CREATE", func(id int64, r *dcl.Result) error {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "/DIRECTORY or /NAME_TABLE")
	})

	g.Bind("CREATE_DIRECTORY", func(id int64, r *dcl.Result) error {
		req := CreateDirectoryRequest{
			Directories: r.List("DIRECTORIES"),
			OwnerUIC:    r.String("OWNER_UIC"),
			Protection:  r.List("PROTECTION"),
			Allocation:  int(r.Int("ALLOCATION")),
			Log:         r.Present("LOG") && !r.Negated("LOG"),
		}

		if r.Present("VERSION_LIMIT") {
			n := int(r.Int("VERSION_LIMIT"))
			req.VersionLimit = &n
		}

		return c.CreateDirectory(req)
	})

	g.Bind("CREATE_NAME_TABLE", func(id int64, r *dcl.Result) error {
		parent := r.String("PARENT_TABLE")
		if parent == "" {
			parent = lnm.ProcessDirectoryName
		}

		return c.CreateNameTable(r.String("TABLE"), parent, logicalModeQualifier(r), !r.Negated("LOG"))
	})

	g.Bind("SHOW_LOGICAL", func(id int64, r *dcl.Result) error {
		if r.Present("STRUCTURE") {
			c.ShowLogicalStructure()

			return nil
		}

		var tables []string

		switch {
		case r.Present("PROCESS"):
			tables = []string{"LNM$PROCESS"}

		case r.Present("GROUP"):
			tables = []string{"LNM$GROUP"}

		case r.Present("SYSTEM"):
			tables = []string{"LNM$SYSTEM"}

		case r.Present("TABLE"):
			tables = r.List("TABLE")
		}

		return c.ShowLogical(r.List("NAME"), tables, r.Present("FULL"))
	})

	g.Bind("SHOW_TRANSLATION", func(id int64, r *dcl.Result) error {
		return c.ShowTranslation(r.String("TABLE"), r.String("NAME"))
	})
}

// logicalTableQualifier returns the table a DEFINE/ASSIGN/DEASSIGN
// command names: /PROCESS, /GROUP, /SYSTEM, or /TABLE= (the grammar lets
// only one through), defaulting to the process table.
func logicalTableQualifier(r *dcl.Result) string {
	switch {
	case r.Present("GROUP"):
		return "LNM$GROUP"

	case r.Present("SYSTEM"):
		return "LNM$SYSTEM"

	case r.Present("TABLE"):
		return r.String("TABLE")

	default:
		return "LNM$PROCESS"
	}
}

// logicalModeQualifier returns the access mode /USER_MODE,
// /SUPERVISOR_MODE, or /EXECUTIVE_MODE selects; supervisor by default.
func logicalModeQualifier(r *dcl.Result) lnm.Mode {
	switch {
	case r.Present("USER_MODE"):
		return lnm.User

	case r.Present("EXECUTIVE_MODE"):
		return lnm.Executive

	default:
		return lnm.Supervisor
	}
}

// DefineLogicalName implements DEFINE and ASSIGN: it defines name in the
// first table designated, at mode, with one equivalence string per
// element of values, each carrying attrs (AttrConcealed, AttrTerminal).
// When log is set and an existing definition was replaced, it prints
// DCL's %DCL-I-SUPERSEDE message. Removing ASSIGN's trailing colon is the
// caller's job.
func (c *Console) DefineLogicalName(table, name string, values []string, mode lnm.Mode, attrs uint32, log bool) error {
	eqv := make([]lnm.Equivalence, len(values))
	for i, v := range values {
		eqv[i] = lnm.Equivalence{Value: v, Attrs: attrs}
	}

	superseded, err := c.Logicals.Define(table, name, mode, 0, eqv)
	if err != nil {
		return err
	}

	if superseded && log {
		c.Printf("%%DCL-I-SUPERSEDE, previous value of %s has been superseded\n", name)
	}

	return nil
}

// DeassignLogicalName implements DEASSIGN: it deletes name from the
// first table designated, at mode and any less privileged mode, or
// with all set every such name in that table. Process-permanent names
// are executive mode, so a default (supervisor-mode) DEASSIGN leaves
// them alone, as on VMS. A table name in a directory table deletes that
// table (DEASSIGN/TABLE=LNM$PROCESS_DIRECTORY TAX).
func (c *Console) DeassignLogicalName(table, name string, mode lnm.Mode, all bool) error {
	switch {
	case all && name != "":
		return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, name)

	case !all && name == "":
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "LOG_NAME")
	}

	_, err := c.Logicals.Delete(table, name, mode)

	return err
}

// CreateNameTable implements CREATE/NAME_TABLE: it creates table as a
// child of the first table parent designates (LNM$PROCESS_DIRECTORY, for
// a process-private table, unless the caller says otherwise). When log
// is set and an existing table of that name was replaced, it prints
// %DCL-I-SUPERSEDE.
func (c *Console) CreateNameTable(table, parent string, mode lnm.Mode, log bool) error {
	_, result, err := c.Logicals.CreateTable(table, parent, mode, 0)
	if err != nil {
		return err
	}

	if result == lnm.TableSuperseded && log {
		c.Printf("%%DCL-I-SUPERSEDE, previous value of %s has been superseded\n", table)
	}

	return nil
}

// logicalTables resolves each of specs (table names, or logical names
// that translate to tables, such as LNM$DCL_LOGICAL) into one list of
// tables in search order, listing a table reached twice only once.
func (c *Console) logicalTables(specs []string) ([]*lnm.Table, error) {
	var out []*lnm.Table

	seen := map[*lnm.Table]bool{}

	for _, spec := range specs {
		tables, err := c.Logicals.ResolveTables(spec, lnm.User)
		if err != nil {
			return nil, err
		}

		for _, t := range tables {
			if !seen[t] {
				seen[t] = true

				out = append(out, t)
			}
		}
	}

	return out, nil
}

// ShowLogical implements SHOW LOGICAL, in the VMS output format. tables
// are the table specs to search (LNM$DCL_LOGICAL when empty); full is
// /FULL, which adds each name's access mode and each equivalence
// string's translation attributes.
//
//   - With no names, every searched table is listed: a "(TABLE)" header,
//     then each name in it, alphabetically.
//
//   - A name with "*" or "%" wildcards lists the matching names the same
//     way, under the header of every table searched.
//
//   - Any other name is looked up in the tables in order and shown for
//     the first table that has it, followed by the iterative translation
//     of each equivalence string that is itself a logical name, one
//     numbered level at a time:
//
//     "MYDISK" = "WORK4" (LNM$PROCESS_TABLE)
//     1 "WORK4" = "$255$DUA17:" (LNM$SYSTEM_TABLE)
//
//     A name that isn't found prints %SHOW-S-NOTRAN.
func (c *Console) ShowLogical(names, tables []string, full bool) error {
	if len(tables) == 0 {
		tables = []string{dclLogicalName}
	}

	searched, err := c.logicalTables(tables)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		c.listLogicalTables(searched, "*", full)

		return nil
	}

	for _, name := range names {
		if lnm.HasWildcards(name) {
			c.listLogicalTables(searched, name, full)

			continue
		}

		if !c.showLogicalName(searched, name, full) {
			c.Printf("%%SHOW-S-NOTRAN, no translation for logical name %s\n", name)
		}
	}

	return nil
}

// listLogicalTables prints each table's header and its names that match
// pattern, with a blank line between tables.
func (c *Console) listLogicalTables(tables []*lnm.Table, pattern string, full bool) {
	for i, t := range tables {
		if i > 0 {
			c.Printf("\n")
		}

		c.Printf("(%s)\n", t.Name)

		entries := t.Entries()

		for j, e := range entries {
			if !lnm.Match(pattern, e.Name) {
				continue
			}

			// A name defined at more than one mode is ambiguous without
			// its mode, so VMS shows the mode then even without /FULL.
			multi := j > 0 && entries[j-1].Name == e.Name ||
				j+1 < len(entries) && entries[j+1].Name == e.Name

			c.printLogicalEntry("  ", e, full || multi, full, "")
		}
	}
}

// showLogicalName prints name's definition from the first of tables that
// has it, then its iterative translations. It reports whether any table
// had name.
func (c *Console) showLogicalName(tables []*lnm.Table, name string, full bool) bool {
	e := c.findLogical(tables, name)
	if e == nil {
		return false
	}

	c.printLogicalChain(tables, e, 0, []string{e.Name}, full)

	return true
}

// findLogical returns name's entry in the first of tables that has it.
func (c *Console) findLogical(tables []*lnm.Table, name string) *lnm.Entry {
	for _, t := range tables {
		if e, err := c.Logicals.Translate(t.Name, name, lnm.User, 0); err == nil {
			return e
		}
	}

	return nil
}

// printLogicalChain prints e at level, then, for each equivalence string
// that isn't TERMINAL and (less one trailing colon) is itself a defined
// logical name, that name's own chain at level+1. chain holds the names
// already printed on this path, so a circular definition stops instead
// of repeating; levels stop at lnm.MaxDepth.
func (c *Console) printLogicalChain(tables []*lnm.Table, e *lnm.Entry, level int, chain []string, full bool) {
	prefix := "  "
	if level > 0 {
		prefix = fmt.Sprintf("%-2d", level)
	}

	c.printLogicalEntry(prefix, e, full, full, e.Table.Name)

	if level+1 >= lnm.MaxDepth {
		return
	}

	for _, q := range e.Equivalences {
		if q.Attrs&lnm.AttrTerminal != 0 {
			continue
		}

		next := strings.TrimSuffix(q.Value, ":")
		if next == "" || containsString(chain, next) {
			continue
		}

		if ne := c.findLogical(tables, next); ne != nil {
			c.printLogicalChain(tables, ne, level+1, append(chain[:len(chain):len(chain)], next), full)
		}
	}
}

// printLogicalEntry prints one name in SHOW LOGICAL's format:
//
//	prefix"NAME" [mode] = "VALUE" [attributes] (TABLE)
//	        = "VALUE2" [attributes]
//
// showMode adds the "[mode]" tag, showAttrs the equivalence attributes,
// and table (when not "") the trailing "(TABLE)". A table-name entry,
// which has no equivalence strings, shows as [table] = "".
func (c *Console) printLogicalEntry(prefix string, e *lnm.Entry, showMode, showAttrs bool, table string) {
	var b strings.Builder

	fmt.Fprintf(&b, "%s%q", prefix, e.Name)

	if showMode {
		fmt.Fprintf(&b, " [%s]", e.Mode)
	}

	if e.IsTable() {
		b.WriteString(" [table]")
	}

	first := lnm.Equivalence{}
	if len(e.Equivalences) > 0 {
		first = e.Equivalences[0]
	}

	fmt.Fprintf(&b, " = %q%s", first.Value, attrTags(first.Attrs, showAttrs))

	if table != "" {
		fmt.Fprintf(&b, " (%s)", table)
	}

	c.Printf("%s\n", b.String())

	for _, q := range e.Equivalences[min(1, len(e.Equivalences)):] {
		c.Printf("        = %q%s\n", q.Value, attrTags(q.Attrs, showAttrs))
	}
}

// attrTags returns SHOW LOGICAL/FULL's " [concealed,terminal]" suffix for
// an equivalence string's attributes, or "" when there are none or show
// is off.
func attrTags(attrs uint32, show bool) string {
	if !show {
		return ""
	}

	var tags []string

	if attrs&lnm.AttrConcealed != 0 {
		tags = append(tags, "concealed")
	}

	if attrs&lnm.AttrTerminal != 0 {
		tags = append(tags, "terminal")
	}

	if len(tags) == 0 {
		return ""
	}

	return " [" + strings.Join(tags, ",") + "]"
}

// ShowLogicalStructure implements SHOW LOGICAL/STRUCTURE: each directory
// and, indented beneath it, the tables descended from it.
func (c *Console) ShowLogicalStructure() {
	for _, dir := range []*lnm.Table{c.Logicals.ProcessDirectory, c.Logicals.SystemDirectory} {
		c.printTableTree(dir, 0)
	}
}

func (c *Console) printTableTree(t *lnm.Table, depth int) {
	c.Printf("%s(%s)\n", strings.Repeat("    ", depth), t.Name)

	for _, child := range c.Logicals.Children(t) {
		if child != t {
			c.printTableTree(child, depth+1)
		}
	}
}

// ShowTranslation implements SHOW TRANSLATION: one level of translation
// of name ($TRNLNM's lookup, no iteration), searching table
// (LNM$DCL_LOGICAL when ""), shown as
//
//	NAME = "VALUE" (TABLE)
//
// A name that isn't found prints %SHOW-S-NOTRAN.
func (c *Console) ShowTranslation(table, name string) error {
	if table == "" {
		table = dclLogicalName
	}

	e, err := c.Logicals.Translate(table, name, lnm.User, 0)
	if err != nil {
		var ve vmserrors.VMSError
		if errors.As(err, &ve) && ve.Status == vmserrors.SS_NOLOGNAM {
			c.Printf("%%SHOW-S-NOTRAN, no translation for logical name %s\n", name)

			return nil
		}

		return err
	}

	value := ""
	if len(e.Equivalences) > 0 {
		value = e.Equivalences[0].Value
	}

	c.Printf("%s = %q (%s)\n", e.Name, value, e.Table.Name)

	return nil
}

// imageRundown does what VMS does at image exit. For logical names
// (User's Manual §11.3.5, §11.4) it deletes the user-mode names in the
// process table, such as those DEFINE/USER_MODE made for the image that
// just ended. The RTL's own per-image state (user-mode device
// allocations, docs/PHASE-26.md) is cleaned up by Environment.ImageRundown.
func (c *Console) imageRundown() {
	if n, err := c.Logicals.Delete(lnm.ProcessTableName, "", lnm.User); err == nil && n > 0 {
		c.traceLogicals("image rundown deleted %d user-mode name(s)", n)
	}

	if c.RTL != nil {
		c.RTL.ImageRundown()
	}
}

// logicalNameFailure returns the status carried by err when a file
// command failed because its spec's logical names couldn't be translated
// (SS$_TOOMANYLNAM: a circular definition, or too many levels), so it is
// reported as that rather than as a bad file specification; nil
// otherwise.
func logicalNameFailure(err error) error {
	var lne *rms.LogicalNameError
	if errors.As(err, &lne) {
		return lne.Err
	}

	return nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}

	return false
}
