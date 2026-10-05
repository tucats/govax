package console

import (
	"strings"

	"github.com/tucats/govax/internal/rms"
)

// Source lines for the debugger (docs/PHASE-42.md, subtask 8).
//
// A program linked /DEBUG carries, in its debug symbol table, the name of
// each module's source file and a map from the module's *listing lines* to
// the file's *records* (internal/dbgsym, source.go). After a breakpoint or
// a step the VMS debugger shows the source line at the PC. This file finds
// that line: it works out which line the PC is in, which file and record
// that line is, locates the file, and returns the record's text.
//
// The file name the DST records is the one the program was assembled from,
// on the machine that assembled it ("DUA1:[SRC]PROG.MAR;1"). The file may
// have moved since, or never have existed on this machine, so the debugger
// looks for it in several places, in order:
//
//  1. the name as recorded, which works when the file is still there (a
//     host path, or a name on a mounted volume);
//  2. each directory of the SET SOURCE list, with the recorded file's name
//     and type (in capitals as recorded, then in lower case);
//  3. the current default directory, again with the file's name and type.
//
// A file that can't be found, or a line it doesn't have, means there is no
// source to show, which isn't an error: the location is shown alone.

// SetSourceDirs makes dirs the SET SOURCE search list. Each is a host
// directory ("testdata/dbg") or a VMS directory specification
// ("DUA0:[SRC]"). The files read so far are forgotten, since a different
// list may now find different files.
func (c *Console) SetSourceDirs(dirs []string) {
	c.sourceDirs = dirs
	c.sourceCache = nil
}

// SourceDirs returns the SET SOURCE search list.
func (c *Console) SourceDirs() []string { return c.sourceDirs }

// SourceLine returns the source line that holds the instruction at pc: its
// number in the listing and its text, with the tabs still in it. ok is
// false when pc isn't in code the debug symbol table has lines for, or the
// source file can't be found or doesn't have the record.
func (c *Console) SourceLine(pc uint32) (n int, text string, ok bool) {
	prog := c.debugImageAt(pc)
	if prog == nil {
		return 0, "", false
	}

	line, mod, found := prog.LineAt(pc)
	if !found {
		return 0, "", false
	}

	file, record, found := mod.SourceOf(line.Line)
	if !found {
		return 0, "", false
	}

	records := c.sourceRecords(file.Spec)
	if record < 1 || record > len(records) {
		return 0, "", false
	}

	// The number shown is the listing's, which is the line table's: the
	// record's own position in the file would differ only for a module
	// built from several files, and VMS shows the listing's.
	return line.Line, records[record-1], true
}

// sourceRecords returns the lines of the source file whose recorded name
// is spec, finding it as the file's comment describes, or nil if it can't
// be found. The answer, including a failure, is remembered until the
// search list changes.
func (c *Console) sourceRecords(spec string) []string {
	if records, seen := c.sourceCache[spec]; seen {
		return records
	}

	records := c.findSource(spec)

	if c.sourceCache == nil {
		c.sourceCache = map[string][]string{}
	}

	c.sourceCache[spec] = records

	return records
}

// findSource looks for the file in each place, in order, and returns the
// lines of the first one that can be read.
func (c *Console) findSource(spec string) []string {
	s := c.ContainerSession
	if s == nil {
		return nil
	}

	candidates := []string{spec}

	// VMS file names have no case, but a host file system's may: the
	// program's source is probably "prog.mar" where the debug symbols say
	// "PROG.MAR", so each place is tried both ways.
	names := []string{baseFileName(spec)}
	if lower := strings.ToLower(names[0]); lower != names[0] {
		names = append(names, lower)
	}

	for _, dir := range c.sourceDirs {
		for _, name := range names {
			candidates = append(candidates, joinDirectory(dir, name))
		}
	}

	candidates = append(candidates, names...)

	for _, name := range candidates {
		loc, err := s.Locate(name, false)
		if err != nil {
			continue
		}

		lines, _, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			continue
		}

		records := make([]string, len(lines))
		for i, l := range lines {
			records[i] = string(l)
		}

		return records
	}

	return nil
}

// baseFileName is a file specification's name and type without its device,
// directory, or version: "DUA1:[SRC]PROG.MAR;1" is "PROG.MAR", and
// "/home/me/prog.mar" is "prog.mar".
func baseFileName(spec string) string {
	if i := strings.LastIndexAny(spec, ":]>/\\"); i >= 0 {
		spec = spec[i+1:]
	}

	if i := strings.IndexByte(spec, ';'); i >= 0 {
		spec = spec[:i]
	}

	return spec
}

// joinDirectory puts a file name in a directory given either way: a VMS
// directory specification ends in ":", "]", or ">" and takes the name as
// it is, and a host directory needs a separator between them.
func joinDirectory(dir, name string) string {
	if dir == "" {
		return name
	}

	switch dir[len(dir)-1] {
	case ':', ']', '>', '/', '\\':
		return dir + name
	}

	return dir + "/" + name
}
