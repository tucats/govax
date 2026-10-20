package rms

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/vmstime"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 5: DIRECTORY, the
// operator-console command that lists the files on a mounted volume. Its
// behavioral reference is the sibling ods2 module's own
// cmd/ods2/internal/session/directory.go (read-only, per this phase's own
// "behavioral spec, not code to link against" framing — see docs/
// PHASE-23.md's "Why this phase looks different from most others"); this
// implementation follows that file's shape closely (the same grouping-by-
// directory, the same /FULL/FILE/SIZE/DATE qualifier meanings) without
// being a literal port, since ods2's own session.Session type lives in an
// internal/ package this module isn't allowed to import.
//
// # A note for a reader new to Go
//
// Directory below returns its listing as a plain string rather than
// writing to an io.Writer the way ods2's own cmdDirectory writes straight
// to s.Stdout. That's deliberate: internal/rms is not allowed to know
// about internal/console.Console's own output stream (see this package's
// other files — InitializeContainer, Session.SetDefault — none of which do
// any printing themselves either), so it hands the finished text back to
// its caller (internal/dclcmd/directory.go's Console.Directory) to print
// via Console.Printf, the same "rms computes, console prints" split
// Session.DefaultString/Console.ShowDefault already established.

// DirectoryOptions selects what DIRECTORY shows of each file, by its
// qualifiers. Full implies the others but Versions and the layout ones.
type DirectoryOptions struct {
	Full bool
	File bool
	Size bool
	Date bool

	// Owner and Protection (docs/PHASE-34.md) add each file's owner UIC
	// and protection, as VMS's DIRECTORY/OWNER and /PROTECTION do; /FULL
	// shows both.
	Owner      bool
	Protection bool

	// SizeMode is /SIZE's value: SizeUsed (the default, and /SIZE=USED),
	// SizeAllocation, or SizeAll ("used/allocated").
	SizeMode SizeMode

	// Versions, when above zero, is DIRECTORY/VERSIONS=n: only the n
	// highest versions of each file name are listed. Zero lists every
	// version the specification matches.
	Versions int

	// NoHeading is /NOHEADING: no "Directory" lines, and each file by its
	// full name, one to a line; NoTrailing is /NOTRAILING: no totals.
	NoHeading, NoTrailing bool

	// OwnerName writes an owner's UIC (group in the high word) as /OWNER
	// and /FULL show it, "[SYSTEM]"; nil writes it as numbers, "[1,4]".
	OwnerName func(uic uint32) string
}

// SizeMode is which of a file's sizes DIRECTORY/SIZE shows.
type SizeMode int

const (
	SizeUsed SizeMode = iota
	SizeAllocation
	SizeAll
)

// columns reports which of the optional columns a listing shows: /FULL
// implies every one.
func (o DirectoryOptions) columns() DirectoryOptions {
	if o.Full {
		o.File, o.Size, o.Date, o.Owner, o.Protection = true, true, true, true, true
		o.SizeMode = SizeAll
	}

	return o
}

// brief reports whether the listing is DIRECTORY's brief one: names
// alone, in columns.
func (o DirectoryOptions) brief() bool {
	return !o.Full && !o.File && !o.Size && !o.Date && !o.Owner && !o.Protection
}

// The brief layout's columns: each name is padded to a column's width,
// four columns to a line, as VMS 7.3's DIRECTORY laid them out
// (testdata/dcl50: ".LOG;1              PROBE50.LOG;1       "). A name
// too long for one column takes as many as it needs (unconfirmed).
const (
	directoryColumnWidth = 20
	directoryColumns     = 4
)

// directoryNameWidth is the width of the name field before the other
// columns (/SIZE, /DATE, ...), as VMS 7.3 laid them out
// ("ASM35.COM;1                1   2-OCT-2026 16:25:34.51"); a longer
// name is on a line of its own, the columns on the next (unconfirmed).
const directoryNameWidth = 19

// Directory lists every file matching specText (an operator-typed file
// specification, possibly wildcarded, possibly partial — resolved against
// the session's current Default exactly like any other command in this
// phase) on its resolved volume, laid out as VMS 7.3's DIRECTORY lays it
// out (testdata/dcl50, testdata/mar's logs): for each directory a
// wildcard reached, a "Directory device:[dir]" heading, its files, and
// "Total of n files." (with blocks when sizes are shown); "Grand total of
// n directories, ..." after more than one.
//
// An empty specText lists everything in the current default directory —
// the same "*.*;*" (every name, every type, every version) ods2's own
// cmdDirectory defaults to when no file spec is typed at all.
//
// A specification that matches nothing returns found false, with nothing
// listed (DIRECTORY's NOFILES warning is the caller's). Errors come from
// specText itself being malformed, or its device not being mounted at all
// (both from the shared resolveVolume helper), or a genuine failure
// opening a matched file's header while gathering detail.
func (s *Session) Directory(specText string, opts DirectoryOptions) (string, bool, error) {
	if specText == "" {
		specText = "*.*;*"
	}

	cols := opts.columns()

	var (
		b                     strings.Builder
		dirs, files           int
		totalUsed, totalAlloc uint32
	)

	// A search list lists each of its elements in turn (User's Manual
	// §11.7.1), under that element's own "Directory" headers.
	err := s.eachSpec(specText, func(vol *volume.Volume, r resolvedSpec) error {
		// A name without a version lists every version, as VMS 7.3's
		// DIRECTORY did (testdata/dcl50's round 4).
		spec := r.Spec
		if spec.Version == "" {
			spec.Version = "*"
		}

		matches, err := filespec.Glob(vol, spec)
		if err != nil {
			return err
		}

		for _, group := range groupMatchesByDir(matches) {
			dirName := r.Display + ":" + dirSpec{Elems: group.dirs}.String()
			listed := newestVersions(group.matches, opts.Versions)

			if len(listed) == 0 {
				continue
			}

			if !opts.NoHeading {
				fmt.Fprintf(&b, "\nDirectory %s\n\n", dirName)
			}

			var used, alloc uint32

			brief := newBriefColumns(&b)

			for _, m := range listed {
				name := m.ShortName(directoryVersionDelim)
				if opts.NoHeading {
					name = dirName + name
				}

				if opts.brief() && !opts.NoHeading {
					brief.add(name)

					continue
				}

				if opts.brief() {
					fmt.Fprintln(&b, name)

					continue
				}

				a, err := matchAttributes(vol, m)
				if err != nil {
					return err
				}

				used += a.Used()
				alloc += a.Allocated

				if opts.Full {
					writeFullEntry(&b, name, m, a, opts)

					continue
				}

				writeColumnsEntry(&b, name, m, a, cols, opts)
			}

			brief.end()

			dirs++
			files += len(listed)
			totalUsed += used
			totalAlloc += alloc

			if !opts.NoTrailing {
				if !opts.Full {
					b.WriteString("\n")
				}

				fmt.Fprintf(&b, "Total of %s%s.\n", countText(len(listed), "file"), blocksText(cols, used, alloc))
			}
		}

		return nil
	})
	if err != nil {
		return "", false, fmt.Errorf("directory: %w", err)
	}

	if dirs > 1 && !opts.NoTrailing {
		fmt.Fprintf(&b, "\nGrand total of %s, %s%s.\n", countText(dirs, "directory"), countText(files, "file"), blocksText(cols, totalUsed, totalAlloc))
	}

	return b.String(), files > 0, nil
}

// countText is "1 file", "2 files", "2 directories".
func countText(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	if strings.HasSuffix(noun, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(noun, "y"))
	}

	return fmt.Sprintf("%d %ss", n, noun)
}

// blocksText is a total's blocks, when sizes are shown: ", 7 blocks.",
// ", 74/90 blocks." for /SIZE=ALL and /FULL.
func blocksText(cols DirectoryOptions, used, alloc uint32) string {
	if !cols.Size {
		return ""
	}

	switch cols.SizeMode {
	case SizeAll:
		return fmt.Sprintf(", %d/%d block%s", used, alloc, pluralS(alloc))
	case SizeAllocation:
		return fmt.Sprintf(", %d block%s", alloc, pluralS(alloc))
	}

	return fmt.Sprintf(", %d block%s", used, pluralS(used))
}

// pluralS is "s" unless n is 1.
func pluralS(n uint32) string {
	if n == 1 {
		return ""
	}

	return "s"
}

// briefColumns lays names out in the brief listing's columns.
type briefColumns struct {
	b    *strings.Builder
	used int // columns used on the current line
}

func newBriefColumns(b *strings.Builder) *briefColumns { return &briefColumns{b: b} }

// add writes name in the next column(s), starting a new line when it
// doesn't fit on this one.
func (c *briefColumns) add(name string) {
	span := (len(name) + directoryColumnWidth) / directoryColumnWidth

	if c.used > 0 && c.used+span > directoryColumns {
		c.b.WriteString("\n")
		c.used = 0
	}

	fmt.Fprintf(c.b, "%-*s", span*directoryColumnWidth, name)

	c.used += span
}

// end finishes the last line.
func (c *briefColumns) end() {
	if c.used > 0 {
		c.b.WriteString("\n")
		c.used = 0
	}
}

// matchAttributes reads the header of the file m found on vol.
func matchAttributes(vol *volume.Volume, m filespec.Match) (FileAttributes, error) {
	f, err := vol.OpenFID(m.Fid)
	if err != nil {
		return FileAttributes{}, fmt.Errorf("opening %s.%s: %w", m.Name, m.Type, err)
	}

	return headerAttributes(f.Header), nil
}

// writeColumnsEntry writes one file's line with the columns cols asks
// for: the name (on a line of its own when it's too long for its field),
// then the file ID, size, date, owner, and protection.
func writeColumnsEntry(b *strings.Builder, name string, m filespec.Match, a FileAttributes, cols, opts DirectoryOptions) {
	if len(name) > directoryNameWidth {
		fmt.Fprintf(b, "%s\n%-*s", name, directoryNameWidth, "")
	} else {
		fmt.Fprintf(b, "%-*s", directoryNameWidth, name)
	}

	if cols.File {
		fmt.Fprintf(b, " %-18s", m.Fid.String())
	}

	if cols.Size {
		switch cols.SizeMode {
		case SizeAll:
			fmt.Fprintf(b, "%9d/%-7d", a.Used(), a.Allocated)
		case SizeAllocation:
			fmt.Fprintf(b, "%9d", a.Allocated)
		default:
			fmt.Fprintf(b, "%9d", a.Used())
		}
	}

	if cols.Date {
		fmt.Fprintf(b, "  %s", directoryTime(a.Created))
	}

	if cols.Owner {
		fmt.Fprintf(b, "  %-20s", ownerText(a.Owner, opts))
	}

	if cols.Protection {
		fmt.Fprintf(b, "  %s", directoryProtection(a.Protection))
	}

	b.WriteString("\n")
}

// writeFullEntry writes one file's DIRECTORY/FULL block, as VMS 7.3 laid
// it out (testdata/dcl50's round 4, testdata/mar's listings), and the
// blank line after it.
func writeFullEntry(b *strings.Builder, name string, m filespec.Match, a FileAttributes, opts DirectoryOptions) {
	optional := func(t uint64, none string) string {
		if t == 0 {
			return none
		}

		return directoryTime(t)
	}

	sizes := fmt.Sprintf("Size: %12d/%d", a.Used(), a.Allocated)

	limit := versionLimitText(a)

	fmt.Fprintf(b, "%-30sFile ID:  %-22s\n", name, m.Fid.String())
	fmt.Fprintf(b, "%-30sOwner:    %s\n", sizes, ownerText(a.Owner, opts))
	fmt.Fprintf(b, "Created:   %s\n", directoryTime(a.Created))
	fmt.Fprintf(b, "Revised:   %s (%d)\n", directoryTime(a.Revised), a.Revision)
	fmt.Fprintf(b, "Expires:   %s\n", optional(a.Expires, "<None specified>"))
	fmt.Fprintf(b, "Backup:    %s\n", optional(a.Backup, "<No backup recorded>"))
	b.WriteString("Effective: <None specified>\n")
	b.WriteString("Recording: <None specified>\n")
	fmt.Fprintf(b, "File organization:  %s\n", organizationName(a.Organization))
	b.WriteString("Shelved state:      Online \n")
	fmt.Fprintf(b, "File attributes:    Allocation: %d, Extend: %d, Global buffer count: %d, %s%s\n",
		a.Allocated, a.Extend, a.GlobalBuffers, limit, characteristicsText(a.Characteristics))
	fmt.Fprintf(b, "Record format:      %s\n", recordFormatText(a))
	fmt.Fprintf(b, "Record attributes:  %s\n", recordAttributesText(a.RecordAttributes))
	b.WriteString("RMS attributes:     None\n")
	b.WriteString("Journaling enabled: None\n")
	fmt.Fprintf(b, "File protection:    %s\n", fullProtection(a.Protection))
	b.WriteString("Access Cntrl List:  None\n\n")
}

// characteristicWords are the words /FULL's file attributes line adds for
// a file's characteristics, in the order VMS 7.1 shows them (by bit, as
// far as the examples go): testdata/pg64's page file shows ", Backups
// disabled, MoveFile disabled", and testdata/backup69's tree shows
// "Contiguous best try", "Contiguous", and "Directory file" (a directory
// says ", Contiguous, Directory file"). The other characteristics' words
// aren't known, so they show nothing.
var characteristicWords = []struct {
	bit  uint32
	text string
}{
	{ondisk.FchNoBackup, "Backups disabled"},
	{ondisk.FchContigB, "Contiguous best try"},
	{ondisk.FchContig, "Contiguous"},
	{ondisk.FchDirectory, "Directory file"},
	{fchNoMove, "MoveFile disabled"},
}

// characteristicsText is what /FULL's file attributes line adds for a
// file's characteristics, each after a comma (characteristicWords).
func characteristicsText(fch uint32) string {
	var b strings.Builder

	for _, w := range characteristicWords {
		if fch&w.bit != 0 {
			b.WriteString(", ")
			b.WriteString(w.text)
		}
	}

	return b.String()
}

// versionLimitText is the version limit's part of /FULL's file attributes
// line. A directory's limit is the default for the files in it, and VMS
// 7.1 shows a directory without one as 32767, the largest version number
// (testdata/backup69's tree).
func versionLimitText(a FileAttributes) string {
	switch {
	case a.Directory():
		limit := a.VersionLimit
		if limit == 0 {
			limit = ondisk.NoVersionLimit
		}

		return fmt.Sprintf("Default version limit: %d", limit)

	case a.VersionLimit == 0:
		return "No version limit"
	}

	return fmt.Sprintf("Version limit: %d", a.VersionLimit)
}

// ownerText is an owner's UIC as opts writes it.
func ownerText(uic uint32, opts DirectoryOptions) string {
	if opts.OwnerName != nil {
		return opts.OwnerName(uic)
	}

	return fmt.Sprintf("[%o,%o]", uic>>16, uic&0xFFFF)
}

// directoryTime writes a VMS time as DIRECTORY does, $ASCTIM's
// "dd-MMM-yyyy hh:mm:ss.cc", a one-digit day after a blank
// (" 5-OCT-2026 07:00:17.69").
func directoryTime(t uint64) string {
	tm := vmstime.VMSTime(int64(t)).Time()

	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d.%02d", tm.Day(), strings.ToUpper(tm.Month().String()[:3]),
		tm.Year(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond()/10_000_000)
}

// organizationName is /FULL's file organization.
func organizationName(org uint8) string {
	switch org {
	case 1:
		return "Relative"
	case 2:
		return "Indexed"
	}

	return "Sequential"
}

// recordFormatText is /FULL's record format: VMS 7.3's "VFC, 2 byte
// header, maximum 0 bytes, longest 25 bytes", "Variable length, ...",
// "Stream_LF, ..." (the maximum is the record size limit, the longest
// the longest record written). Fixed, undefined, and the other streams
// are written by the same pattern (unconfirmed).
func recordFormatText(a FileAttributes) string {
	sizes := fmt.Sprintf("maximum %d bytes, longest %d bytes", a.RecordSize, a.LongestRecord)

	switch ondisk.RecordFormat(a.Format) {
	case ondisk.RecordFormatFixed:
		return fmt.Sprintf("Fixed length %d byte records", a.RecordSize)
	case ondisk.RecordFormatVariable:
		return "Variable length, " + sizes
	case ondisk.RecordFormatVFC:
		return fmt.Sprintf("VFC, %d byte header, %s", a.FixedControl, sizes)
	case ondisk.RecordFormatStreamCRLF:
		return "Stream, " + sizes
	case ondisk.RecordFormatStreamLF:
		return "Stream_LF, " + sizes
	case ondisk.RecordFormatStreamCR:
		return "Stream_CR, " + sizes
	}

	return "Undefined, " + sizes
}

// recordAttributesText is /FULL's record attributes: its carriage
// control, as VMS 7.3 wrote "Print file carriage control" and "None";
// "Carriage return carriage control" and "Fortran carriage control"
// (unconfirmed), and ", Non-spanned" for FAB$M_BLK (unconfirmed).
func recordAttributesText(rat uint8) string {
	var text string

	switch {
	case rat&ondisk.AttrFortranCC != 0:
		text = "Fortran carriage control"
	case rat&ondisk.AttrImpliedCC != 0:
		text = "Carriage return carriage control"
	case rat&ondisk.AttrPrintCC != 0:
		text = "Print file carriage control"
	}

	if rat&ondisk.AttrNoSpan != 0 {
		if text == "" {
			return "Non-spanned"
		}

		text += ", Non-spanned"
	}

	if text == "" {
		return "None"
	}

	return text
}

// fullProtection is /FULL's protection: "System:RWED, Owner:RWED,
// Group:RE, World:".
func fullProtection(mask uint16) string {
	categories := []string{"System", "Owner", "Group", "World"}
	parts := make([]string, len(categories))

	for i, name := range categories {
		parts[i] = name + ":" + ondisk.ProtectionAccess(mask, i)
	}

	return strings.Join(parts, ", ")
}

// dirGroup is every filespec.Match Glob found in one directory, keyed by
// that directory's path — the same shape as ods2's own dirGroup
// (directory.go), reproduced here rather than imported since that type is
// private to ods2's internal/ session package.
type dirGroup struct {
	dirs    []string
	matches []filespec.Match
}

// groupMatchesByDir groups Glob's results by which directory they were
// found in (relevant when a wildcarded directory component matched more
// than one directory) and returns the groups sorted by directory path, so
// DIRECTORY's output is stable and reproducible across runs rather than
// depending on Glob's own internal traversal order.
func groupMatchesByDir(matches []filespec.Match) []dirGroup {
	var order []string

	index := make(map[string]*dirGroup)

	for _, m := range matches {
		key := strings.Join(m.Dirs, ".")

		g, ok := index[key]
		if !ok {
			g = &dirGroup{dirs: m.Dirs}

			index[key] = g

			order = append(order, key)
		}

		g.matches = append(g.matches, m)
	}

	sort.Strings(order)

	groups := make([]dirGroup, len(order))
	for i, key := range order {
		groups[i] = *index[key]
	}

	return groups
}

// newestVersions returns the matches (all in one directory) that are
// among the keep highest versions of their own name.type, in their
// original order; a keep of zero or less returns matches unchanged.
func newestVersions(matches []filespec.Match, keep int) []filespec.Match {
	if keep <= 0 {
		return matches
	}

	// Gather each name.type's versions, then find the lowest version
	// that still makes the cut: a file is listed if its version is at
	// least that one.
	versions := make(map[string][]uint16)

	for _, m := range matches {
		key := m.Name + "." + m.Type
		versions[key] = append(versions[key], m.Version)
	}

	lowest := make(map[string]uint16, len(versions))

	for key, list := range versions {
		sort.Slice(list, func(i, j int) bool { return list[i] > list[j] })

		if len(list) > keep {
			lowest[key] = list[keep-1]
		}
	}

	kept := make([]filespec.Match, 0, len(matches))

	for _, m := range matches {
		if floor, ok := lowest[m.Name+"."+m.Type]; ok && m.Version < floor {
			continue
		}

		kept = append(kept, m)
	}

	return kept
}

// directoryVersionDelim is the character DIRECTORY prints between a file's
// name.type and its version number (e.g. "FOO.TXT;3").
const directoryVersionDelim = ';'

// directoryProtection writes a protection mask the way VMS's DIRECTORY
// /PROTECTION does: the access each of system, owner, group, and world is
// granted, in that order, "(RWED,RWED,RE,)".
func directoryProtection(mask uint16) string {
	parts := make([]string, 4)
	for i := range parts {
		parts[i] = ondisk.ProtectionAccess(mask, i)
	}

	return "(" + strings.Join(parts, ",") + ")"
}
