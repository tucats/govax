package rms

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
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
// its caller (internal/console/directory.go's Console.Directory) to print
// via Console.Printf, the same "rms computes, console prints" split
// Session.DefaultString/Console.ShowDefault already established.

// DirectoryOptions selects which extra columns DIRECTORY prints alongside
// each file's bare name, matching ods2's own cmdDirectory qualifiers.
// Full implies all three of the others, exactly as it does there.
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

	// Versions, when above zero, is DIRECTORY/VERSIONS=n: only the n
	// highest versions of each file name are listed. Zero lists every
	// version the specification matches.
	Versions int
}

// columns reports which of the optional columns a listing shows: /FULL
// implies every one.
func (o DirectoryOptions) columns() DirectoryOptions {
	if o.Full {
		return DirectoryOptions{Full: true, File: true, Size: true, Date: true, Owner: true, Protection: true}
	}

	return o
}

// Directory lists every file matching specText (an operator-typed file
// specification, possibly wildcarded, possibly partial — resolved against
// the session's current Default exactly like any other command in this
// phase) on its resolved volume, formatted as VMS's own DIRECTORY command
// would print it: one "Directory device:[dir]" header per directory a
// wildcard reached, one line per file beneath it, and a final file/block
// count summary.
//
// An empty specText lists everything in the current default directory —
// the same "*.*;*" (every name, every type, every version) ods2's own
// cmdDirectory defaults to when no file spec is typed at all.
//
// Directory never treats "nothing matched" as an error (an empty, all-
// zero listing with "Total of 0 file(s)." is exactly what real VMS prints
// for a DIRECTORY that matches nothing) — the only errors it can return
// come from specText itself being malformed, or its device not being
// mounted at all (both from the shared resolveVolume helper), or a
// genuine failure opening a matched file's header while gathering
// size/date detail.
func (s *Session) Directory(specText string, opts DirectoryOptions) (string, error) {
	if specText == "" {
		specText = "*.*;*"
	}

	cols := opts.columns()

	var b strings.Builder

	totalFiles := 0

	var totalBlocks uint32

	// A search list lists each of its elements in turn (User's Manual
	// §11.7.1), under that element's own "Directory" headers.
	err := s.eachSpec(specText, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return err
		}

		for _, group := range groupMatchesByDir(matches) {
			dir := strings.Join(group.dirs, ".")
			if dir == "" {
				dir = "000000" // the MFD, which VMS writes [000000]
			}

			fmt.Fprintf(&b, "\nDirectory %s:[%s]\n\n", r.Display, dir)

			for _, m := range newestVersions(group.matches, opts.Versions) {
				line, blocks, err := formatDirectoryEntry(vol, m, cols)
				if err != nil {
					return err
				}

				fmt.Fprintln(&b, line)

				totalFiles++
				totalBlocks += blocks
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("directory: %w", err)
	}

	fmt.Fprintf(&b, "\nTotal of %d file(s)", totalFiles)

	if cols.Size {
		fmt.Fprintf(&b, ", %d block(s)", totalBlocks)
	}

	fmt.Fprintln(&b, ".")

	return b.String(), nil
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
// name.type and its version number (e.g. "FOO.TXT;3") — real VMS's own
// convention, and ods2's own default (its Session.Delim, which this
// project has no equivalent user-configurable setting for, so this is
// simply written as a literal here rather than threaded through as a
// parameter no caller in this project would ever vary).
const directoryVersionDelim = ';'

// formatDirectoryEntry renders one line of DIRECTORY output for m, and
// also returns its size in blocks (0 if size information wasn't
// requested), which the caller accumulates into a grand total.
//
// Like ods2's own formatDirectoryEntry, this doesn't attempt real VMS's
// exact column-aligned, multiple-files-per-line layout for a bare listing
// — see docs/PHASE-23.md's own open question on this — it always prints
// one file per line, with any requested extra detail appended after it.
func formatDirectoryEntry(vol *volume.Volume, m filespec.Match, cols DirectoryOptions) (string, uint32, error) {
	var line strings.Builder

	fmt.Fprintf(&line, "%-30s", m.ShortName(directoryVersionDelim))

	if !cols.File && !cols.Size && !cols.Date && !cols.Owner && !cols.Protection && !cols.Full {
		return line.String(), 0, nil
	}

	f, err := vol.OpenFID(m.Fid)
	if err != nil {
		return "", 0, fmt.Errorf("opening %s.%s: %w", m.Name, m.Type, err)
	}

	h := f.Header
	blocks := h.RecordAttributes.HighestBlock

	if cols.File {
		fmt.Fprintf(&line, "  %-16s", m.Fid.String())
	}

	if cols.Size {
		fmt.Fprintf(&line, "  %5d", blocks)
	}

	if cols.Date {
		if ident, err := h.Ident(); err == nil {
			fmt.Fprintf(&line, "  %s", ident.RevisionDate.String())
		}
	}

	if cols.Owner {
		fmt.Fprintf(&line, "  %-16s", h.Owner.String())
	}

	if cols.Protection {
		fmt.Fprintf(&line, "  %s", directoryProtection(h.FileProtection))
	}

	if cols.Full {
		fmt.Fprintf(&line, "  %s", h.RecordAttributes.Format)
	}

	return line.String(), blocks, nil
}

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
