package dclcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the console's logical-name commands (docs/PHASE-25.md):
// DEFINE and ASSIGN, DEASSIGN, CREATE/NAME_TABLE, SHOW LOGICAL and SHOW
// TRANSLATION, all working on the shared lnm.Database in c.Logicals.
// The DCL bindings that call these are in dispatch.go.

// dclLogicalName is the logical name SHOW LOGICAL and SHOW TRANSLATION
// search by default (LNM$FILE_DEV's tables, as on VMS).
const dclLogicalName = "LNM$DCL_LOGICAL"

// DefineLogicalName implements DEFINE and ASSIGN: it defines name in the
// first table designated, at mode, with one equivalence string per
// element of values, each carrying attrs (AttrConcealed, AttrTerminal).
// When log is set and an existing definition was replaced, it prints
// DCL's %DCL-I-SUPERSEDE message. Removing ASSIGN's trailing colon is the
// caller's job.
func (c *Context) DefineLogicalName(table, name string, values []string, mode lnm.Mode, attrs uint32, log bool) error {
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
// table (DEASSIGN/TABLE=LNM$PROCESS_DIRECTORY TAX). It leaves $STATUS
// SS$_NORMAL.
func (c *Context) DeassignLogicalName(table, name string, mode lnm.Mode, all bool) error {
	switch {
	case all && name != "":
		return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, name)

	case !all && name == "":
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "LOG_NAME")
	}

	if _, err := c.Logicals.Delete(table, name, mode); err != nil {
		return err
	}

	// $DELLNM's status, SS$_NORMAL: VMS 7.3's DEASSIGN left %X00000001
	// (testdata/dcl50's F11).
	c.SetStatus(ssNormal)

	return nil
}

// CreateNameTable implements CREATE/NAME_TABLE: it creates table as a
// child of the first table parent designates (LNM$PROCESS_DIRECTORY, for
// a process-private table, unless the caller says otherwise). When log
// is set and an existing table of that name was replaced, it prints
// %DCL-I-SUPERSEDE.
func (c *Context) CreateNameTable(table, parent string, mode lnm.Mode, log bool) error {
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
func (d logicalDisplay) logicalTables(specs []string) ([]*lnm.Table, error) {
	var out []*lnm.Table

	seen := map[*lnm.Table]bool{}

	for _, spec := range specs {
		tables, err := d.db.ResolveTables(spec, lnm.User)
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
//     with a blank line before and after it, then each name in it,
//     alphabetically.
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
//     1  "WORK4" = "$255$DUA17:" (LNM$SYSTEM_TABLE)
//
//     A name that isn't found prints %SHOW-S-NOTRAN.
//
// Its status is SS$_NORMAL, or SHOW$_NOTRAN when a name had no
// translation, each with STS$M_INHIB_MSG set, as VMS's SHOW image
// returned them (testdata/mp/probe4 and probe5).
func (c *Context) ShowLogical(names, tables []string, full bool) error {
	untranslated, err := logicalDisplay{db: c.Logicals, out: c.Printf}.show(names, tables, full)
	if err != nil {
		return err
	}

	if untranslated {
		c.SetStatus(showNotran | stsInhibitMsg)
	} else {
		c.SetStatus(ssNormal | stsInhibitMsg)
	}

	return nil
}

// showNotran is SHOW$_NOTRAN, SHOW LOGICAL's status for a name with no
// translation (VMS 7.3's $STATUS, %X10788019 with STS$M_INHIB_MSG;
// testdata/mp/probe5/vax, step 10). govax's tables have no SHOW$
// facility, so the value is VMS's.
const showNotran = 0x00788019

// logicalDisplay is SHOW LOGICAL's display of one process's logical
// names (db), written through out.
type logicalDisplay struct {
	db  *lnm.Database
	out func(format string, args ...any)
}

// show is SHOW LOGICAL (see Context.ShowLogical). untranslated reports
// that a name (not a wildcard) had no translation: %SHOW-S-NOTRAN.
func (d logicalDisplay) show(names, tables []string, full bool) (untranslated bool, err error) {
	if len(tables) == 0 {
		tables = []string{dclLogicalName}
	}

	searched, err := d.logicalTables(tables)
	if err != nil {
		return false, err
	}

	if len(names) == 0 {
		d.listLogicalTables(searched, "*", full)

		return false, nil
	}

	for _, name := range names {
		if lnm.HasWildcards(name) {
			d.listLogicalTables(searched, name, full)

			continue
		}

		if !d.showLogicalName(searched, name, full) {
			d.out("%%SHOW-S-NOTRAN, no translation for logical name %s\n", name)

			untranslated = true
		}
	}

	return untranslated, nil
}

// listLogicalTables prints each table's header after a blank line, as
// VMS's SHOW LOGICAL has it (testdata/mp/probe4/vax/probe4.log), and its
// names that match pattern after another blank line; a table with none
// has no second blank line (VMS 7.3, testdata/dcl50's R5). With full,
// the header adds the table's
// access mode and a line about its protection, as VMS 7.3's SHOW
// LOGICAL/FULL of the process table did (testdata/mp/probe5/vax, step
// 12): "(LNM$PROCESS_TABLE)<tab>[kernel]", then three tabs and "[no
// protection information]". (govax keeps no table protection, so every
// table says so; unconfirmed for the shareable tables.)
func (d logicalDisplay) listLogicalTables(tables []*lnm.Table, pattern string, full bool) {
	for _, t := range tables {
		if full {
			d.out("\n(%s)\t[%s]\n\t\t\t[no protection information]\n", t.Name, t.Mode)
		} else {
			d.out("\n(%s)\n", t.Name)
		}

		entries := t.Entries()
		shown := false

		for j, e := range entries {
			if !lnm.Match(pattern, e.Name) {
				continue
			}

			if !shown {
				d.out("\n")

				shown = true
			}

			// A name defined at more than one mode is ambiguous without
			// its mode, so VMS shows the mode then even without /FULL.
			multi := j > 0 && entries[j-1].Name == e.Name ||
				j+1 < len(entries) && entries[j+1].Name == e.Name

			d.printLogicalEntry("  ", e, full || multi, full, "")
		}
	}
}

// showLogicalName prints name's definition from each of tables that has
// it, in search order, each followed by its iterative translations: VMS
// shows a process SYS$SHARE, then the system's (testdata/link68's log,
// R4). It reports whether any table had name.
func (d logicalDisplay) showLogicalName(tables []*lnm.Table, name string, full bool) bool {
	shown := false

	for _, t := range tables {
		e, err := d.db.Translate(t.Name, name, lnm.User, 0)
		if err != nil {
			continue
		}

		d.printLogicalChain(tables, e, 0, []string{e.Name}, full)

		shown = true
	}

	return shown
}

// findLogical returns name's entry in the first of tables that has it.
func (d logicalDisplay) findLogical(tables []*lnm.Table, name string) *lnm.Entry {
	for _, t := range tables {
		if e, err := d.db.Translate(t.Name, name, lnm.User, 0); err == nil {
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
func (d logicalDisplay) printLogicalChain(tables []*lnm.Table, e *lnm.Entry, level int, chain []string, full bool) {
	// The name is indented three blanks, the room a level's number and
	// two blanks take on the lines below it (VMS 7.3, testdata/dcl50's
	// R5: `   "R6B" = "R6A"`, then `1  "R6A" = "plain"`).
	prefix := "   "
	if level > 0 {
		prefix = fmt.Sprintf("%d  ", level)
	}

	d.printLogicalEntry(prefix, e, full, full, e.Table.Name)

	if level+1 >= lnm.MaxDepth {
		return
	}

	for _, q := range e.Equivalences {
		if _, ok := lnm.ParsePermanentFile(q.Value); ok || q.Attrs&lnm.AttrTerminal != 0 {
			continue
		}

		next := strings.TrimSuffix(q.Value, ":")
		if next == "" || containsString(chain, next) {
			continue
		}

		if ne := d.findLogical(tables, next); ne != nil {
			d.printLogicalChain(tables, ne, level+1, append(chain[:len(chain):len(chain)], next), full)
		}
	}
}

// printLogicalEntry prints one name in SHOW LOGICAL's format:
//
//	prefix"NAME" [mode] = "VALUE" [attributes] (TABLE)
//	<tab>= "VALUE2" [attributes]
//
// (a search list's further values follow a tab, as VMS 7.3 wrote them).
// A process-permanent file's value shows without its prefix
// (lnm.DisplayValue), as VMS shows it.
// showMode adds the "[mode]" tag, showAttrs the equivalence attributes,
// and table (when not "") the trailing "(TABLE)". A table-name entry,
// which has no equivalence strings, shows as [table] = "".
func (d logicalDisplay) printLogicalEntry(prefix string, e *lnm.Entry, showMode, showAttrs bool, table string) {
	var b strings.Builder

	fmt.Fprintf(&b, "%s%q", prefix, e.Name)

	if showMode {
		tags := []string{e.Mode.String()}
		if showAttrs {
			tags = append(tags, nameAttrTags(e.Attrs)...)
		}

		fmt.Fprintf(&b, " [%s]", strings.Join(tags, ","))
	}

	if e.IsTable() {
		b.WriteString(" [table]")
	}

	first := lnm.Equivalence{}
	if len(e.Equivalences) > 0 {
		first = e.Equivalences[0]
	}

	fmt.Fprintf(&b, " = %q%s", lnm.DisplayValue(first.Value), attrTags(first.Attrs, showAttrs))

	if table != "" {
		fmt.Fprintf(&b, " (%s)", table)
	}

	d.out("%s\n", b.String())

	for _, q := range e.Equivalences[min(1, len(e.Equivalences)):] {
		d.out("\t= %q%s\n", lnm.DisplayValue(q.Value), attrTags(q.Attrs, showAttrs))
	}
}

// nameAttrTags are SHOW LOGICAL/FULL's words for a name's own
// attributes, after its mode: "[super,confine]" for a name DCL's OPEN
// defined, as VMS 7.3 showed it (testdata/dcl50's F1). Without /FULL only
// the mode shows (VMS 7.1, testdata/ppf71: "[super]"). Where no_alias
// goes among them is unconfirmed.
func nameAttrTags(attrs uint32) []string {
	var tags []string

	if attrs&lnm.AttrNoAlias != 0 {
		tags = append(tags, "no_alias")
	}

	if attrs&lnm.AttrConfine != 0 {
		tags = append(tags, "confine")
	}

	return tags
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
func (c *Context) ShowLogicalStructure() {
	for _, dir := range []*lnm.Table{c.Logicals.ProcessDirectory, c.Logicals.SystemDirectory} {
		c.printTableTree(dir, 0)
	}
}

func (c *Context) printTableTree(t *lnm.Table, depth int) {
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
// A name that isn't found prints %SHOW-S-NOTRAN. A process-permanent
// file's value shows without its prefix.
func (c *Context) ShowTranslation(table, name string) error {
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
		value = lnm.DisplayValue(e.Equivalences[0].Value)
	}

	c.Printf("%s = %q (%s)\n", e.Name, value, e.Table.Name)

	return nil
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
