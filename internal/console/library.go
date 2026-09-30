package console

import (
	"fmt"
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/PHASE-28.md subtask 7's LIBRARY command, in
// the style of VMS's LIBRARIAN:
//
//	LIBRARY library[/HOST] [input[,...][/HOST]]
//	        [/CREATE] [/INSERT | /REPLACE] [/DELETE=(module,...)]
//	        [/EXTRACT=(module,...) [/OUTPUT=file]]
//	        [/LIST[=file] [/FULL] [/NAMES] [/WIDTH=n]]
//	        [/MACRO | /OBJECT] [/[NO]SQUEEZE] [/SELECTIVE_SEARCH] [/LOG]
//
// It creates or changes a macro library (.MLB) or an object library
// (.OLB) with internal/lbr, extracts modules from one, and lists one as
// LIBRARY/LIST does. /MACRO or /OBJECT picks the type of a new library (an
// object library by default, as on VMS) and the library's default file
// type; an existing library keeps its own type, and naming the other is an
// error.
//
// Input files are inserted in order: macro source (.MAR), one module per
// macro, or object files (.OBJ), one module per object module. With no
// operation named, they replace modules of the same names, as LIBRARIAN's
// default /REPLACE does; with /INSERT or /CREATE, a module already in the
// library is a warning and isn't changed. /DELETE= is done first, and
// /EXTRACT= and /LIST last, on the library as changed. /DELETE= and
// /EXTRACT= names may hold the wildcards * and %.
//
// Every file can be a host file or a file on a mounted ODS-2 volume, by
// the rules MACRO and LINK use (rms.Session.Locate): an input file's bare
// name is found beside the input before it, and /OUTPUT= and a /LIST= file
// beside the library, with the types OBJ or MAR (by the library's type)
// and LIS.
//
// Unlike LIBRARIAN, which updates a library in place, LIBRARY builds the
// whole library in memory and writes it only when every step has
// succeeded: an error leaves the library as it was. A host library is
// replaced; a library on a volume gets a new version.

// LibraryOptions is one LIBRARY command.
type LibraryOptions struct {
	// Library is the library file, and LibraryHost an explicit /HOST on
	// it. Inputs are the input files, and InputHost an explicit /HOST on
	// them.
	Library     string
	LibraryHost bool
	Inputs      []string
	InputHost   bool

	// Create is /CREATE, Insert /INSERT, and Replace /REPLACE.
	Create  bool
	Insert  bool
	Replace bool

	// Delete and Extract are /DELETE= and /EXTRACT='s module names, and
	// Output /OUTPUT='s file for the extracted modules.
	Delete  []string
	Extract []string
	Output  string

	// List is /LIST, to ListFile, or the console when it's empty. Full is
	// /FULL, Names /NAMES, and Width /WIDTH= (0 is 80 on the console, 132
	// in a file).
	List     bool
	ListFile string
	Full     bool
	Names    bool
	Width    int

	// Macro is /MACRO and Object /OBJECT. NoSqueeze is /NOSQUEEZE, Selective
	// /SELECTIVE_SEARCH, and Log /LOG.
	Macro     bool
	Object    bool
	NoSqueeze bool
	Selective bool
	Log       bool
}

// library is one LIBRARY command in progress.
type library struct {
	c    *Console
	opts LibraryOptions
	now  uint64

	lib     *lbr.Library // the library as read, or as written
	b       *lbr.Builder // the library being changed, if it is
	found   rms.FileLocation
	changed bool
	logs    []func(name string) error // /LOG's messages, which name the library written
}

// Library runs one LIBRARY command.
func (c *Console) Library(opts LibraryOptions) error {
	l := &library{c: c, opts: opts, now: vmsdef.Time(time.Now())}

	return l.run()
}

func (l *library) run() error {
	s := l.c.ContainerSession
	opts := l.opts

	typ, libType := lbr.TypeObject, "OLB"
	if opts.Macro {
		typ, libType = lbr.TypeMacro, "MLB"
	}

	loc, err := s.Locate(opts.Library, opts.LibraryHost)
	if err != nil {
		return fileFailure(err, opts.Library)
	}

	loc = withDefaultType(loc, libType)

	edit := len(opts.Inputs) > 0 || len(opts.Delete) > 0

	switch {
	case (opts.Insert || opts.Replace) && len(opts.Inputs) == 0:
		return l.failed(fmt.Errorf("/INSERT and /REPLACE need input files"), loc.Name)
	case !opts.Create && !edit && len(opts.Extract) == 0 && !opts.List:
		return l.failed(fmt.Errorf("nothing to do: name input files, or /CREATE, /DELETE, /EXTRACT, or /LIST"), loc.Name)
	}

	if opts.Create {
		if l.b, err = lbr.Create(typ); err != nil {
			return l.failed(err, loc.Name)
		}

		l.b.Created, l.b.Updated = l.now, l.now
		l.found, l.changed = loc, true
	} else {
		if err := l.open(loc); err != nil {
			return err
		}

		if edit {
			if l.b, err = lbr.Edit(l.lib); err != nil {
				return l.failed(err, l.found.Name)
			}
		}
	}

	for _, pattern := range opts.Delete {
		l.delete(pattern)
	}

	if err := l.insert(); err != nil {
		return err
	}

	if l.changed {
		if err := l.write(); err != nil {
			return err
		}
	}

	if len(opts.Extract) > 0 {
		if err := l.extract(); err != nil {
			return err
		}
	}

	if opts.List {
		return l.list()
	}

	return nil
}

func (l *library) failed(err error, name string) error {
	return vmserrors.Wrap(vmserrors.CLI_LIBRARY, err, name)
}

func (l *library) warn(err error) {
	l.c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CLI_LIBWARNING, err, l.found.Name))
}

// log records a /LOG message, which is displayed once the library is
// written, with its name.
func (l *library) log(status uint32, module string) {
	if l.opts.Log {
		l.logs = append(l.logs, func(name string) error { return vmserrors.New(status, module, name) })
	}
}

// open reads an existing library.
func (l *library) open(loc rms.FileLocation) error {
	data, found, err := l.c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		return fileFailure(err, loc.Name)
	}

	l.found = found

	if l.lib, err = lbr.Open(data); err != nil {
		return l.failed(err, found.Name)
	}

	switch t := l.lib.Type; {
	case l.opts.Macro && t != lbr.TypeMacro, l.opts.Object && t != lbr.TypeObject:
		return l.failed(fmt.Errorf("%s is a %s library", found.Name, t), found.Name)
	}

	return nil
}

func (l *library) delete(pattern string) {
	names := l.b.Match(pattern)
	if len(names) == 0 {
		l.warn(fmt.Errorf("no module matches %s", pattern))
	}

	for _, name := range names {
		_ = l.b.Delete(name)
		l.changed = true
		l.log(vmserrors.CLI_LIBDELETED, name)
	}
}

// insert reads the input files and puts their modules in the library.
func (l *library) insert() error {
	s := l.c.ContainerSession
	replace := l.opts.Replace || (!l.opts.Insert && !l.opts.Create)

	var prev rms.FileLocation

	for i, name := range l.opts.Inputs {
		var (
			loc rms.FileLocation
			err error
		)

		if i == 0 {
			loc, err = s.Locate(name, l.opts.InputHost)
		} else {
			loc, err = s.LocateRelated(name, l.opts.InputHost, prev)
		}

		if err != nil {
			return fileFailure(err, name)
		}

		entries, found, err := l.readInput(loc)
		if err != nil {
			return err
		}

		prev = found

		for _, e := range entries {
			e.Inserted = l.now
			l.b.Updated = l.now

			if !replace {
				if _, ok := l.b.Module(e.Name); ok {
					l.warn(fmt.Errorf("module %s from %s is already in the library", e.Name, found.Name))

					continue
				}

				if err := l.b.Insert(e); err != nil {
					return l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
				}

				l.log(vmserrors.CLI_LIBINSERTED, e.Name)
			} else {
				replaced, err := l.b.Replace(e)
				if err != nil {
					return l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
				}

				if replaced {
					l.log(vmserrors.CLI_LIBREPLACED, e.Name)
				} else {
					l.log(vmserrors.CLI_LIBINSERTED, e.Name)
				}
			}

			l.changed = true
		}
	}

	return nil
}

// readInput reads one input file's modules.
func (l *library) readInput(loc rms.FileLocation) ([]*lbr.Entry, rms.FileLocation, error) {
	s := l.c.ContainerSession

	switch l.b.Type {
	case lbr.TypeMacro:
		loc = withDefaultType(loc, "MAR")

		records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			return nil, found, fileFailure(err, loc.Name)
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		entries, warns, err := l.b.MacroModules(lines, !l.opts.NoSqueeze)
		for _, w := range warns {
			l.warn(fmt.Errorf("%s: %w", found.Name, w))
		}

		if err != nil {
			return nil, found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
		}

		return entries, found, nil

	case lbr.TypeObject:
		loc = withDefaultType(loc, "OBJ")

		records, found, err := s.ReadRecordFile(loc, rms.VariableRecords)
		if err != nil {
			return nil, found, fileFailure(err, loc.Name)
		}

		entries, err := l.b.ObjectModules(records, l.opts.Selective)
		if err != nil {
			return nil, found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
		}

		return entries, found, nil
	}

	return nil, loc, l.failed(fmt.Errorf("LIBRARY can't insert modules in a %s library", l.b.Type), l.found.Name)
}

// write writes the library: over a host file, or as a new version of a
// volume file.
func (l *library) write() error {
	data := l.b.Bytes()

	blocks := make([][]byte, 0, len(data)/512)
	for i := 0; i < len(data); i += 512 {
		blocks = append(blocks, data[i:i+512])
	}

	out := l.found
	if !out.Host && !l.opts.Create {
		out.Name, _ = splitVersion(out.Name)
	}

	created, err := l.c.ContainerSession.CreateRecordFile(out, rms.ImageBlocks, blocks)
	if err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, out.Name)
	}

	l.found = created

	if l.lib, err = lbr.Open(data); err != nil {
		return l.failed(err, created.Name)
	}

	for _, m := range l.logs {
		l.c.Printf("%%%s\n", m(created.Name))
	}

	return nil
}

// extract writes the modules /EXTRACT= names to one file, in the order
// named.
func (l *library) extract() error {
	kind, typ := rms.TextRecords, "TXT"

	switch l.lib.Type {
	case lbr.TypeObject, lbr.TypeShareable:
		kind, typ = rms.VariableRecords, "OBJ"
	case lbr.TypeMacro:
		typ = "MAR"
	case lbr.TypeHelp:
		typ = "HLP"
	}

	var records [][]byte

	n := 0

	for _, pattern := range l.opts.Extract {
		keys := l.lib.Match(pattern)
		if len(keys) == 0 {
			l.warn(fmt.Errorf("no module matches %s", pattern))
		}

		for _, k := range keys {
			m, err := l.lib.Module(k.RFA)
			if err != nil {
				return l.failed(err, l.found.Name)
			}

			records = append(records, m.Records...)
			n++
		}
	}

	if n == 0 {
		return nil
	}

	s := l.c.ContainerSession

	out, err := outputLocation(s, l.opts.Output, l.found, typ)
	if err != nil {
		return fileFailure(err, l.opts.Output)
	}

	out = withDefaultType(out, typ)

	if _, err := s.CreateRecordFile(out, kind, records); err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, out.Name)
	}

	return nil
}

// list lists the library, on the console or to a file.
func (l *library) list() error {
	width := l.opts.Width
	if width == 0 && l.opts.ListFile != "" {
		width = 132
	}

	lines, err := l.lib.List(lbr.ListOptions{
		Name:  l.found.Name,
		Now:   l.now,
		Full:  l.opts.Full,
		Names: l.opts.Names,
		Width: width,
	})
	if err != nil {
		return l.failed(err, l.found.Name)
	}

	if l.opts.ListFile == "" {
		for _, line := range lines {
			l.c.Printf("%s\n", line)
		}

		return nil
	}

	s := l.c.ContainerSession

	out, err := outputLocation(s, l.opts.ListFile, l.found, "LIS")
	if err != nil {
		return fileFailure(err, l.opts.ListFile)
	}

	out = withDefaultType(out, "LIS")

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := s.CreateRecordFile(out, rms.TextRecords, records); err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, out.Name)
	}

	return nil
}
