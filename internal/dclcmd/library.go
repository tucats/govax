package dclcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/planning/PHASE-28.md subtask 7's LIBRARY command, in
// the style of VMS's LIBRARIAN:
//
//	LIBRARY library[/HOST] [input[,...][/HOST]]
//	        [/CREATE[=(option:n,...)]] [/INSERT | /REPLACE] [/DELETE=(module,...)]
//	        [/EXTRACT=(module,...) [/OUTPUT=file]]
//	        [/LIST[=file] [/FULL] [/NAMES] [/WIDTH=n]]
//	        [/HELP | /MACRO | /OBJECT | /SHARE | /TEXT] [/[NO]SQUEEZE] [/SELECTIVE_SEARCH] [/LOG]
//
// It creates or changes a macro library (.MLB), an object library (.OLB),
// a shareable image symbol table library (.OLB, /SHARE; docs/planning/PHASE-68 -
// link shareable images.md, subtask 12), a text library (.TLB, /TEXT;
// docs/planning/PHASE-83 - FORTRAN and VMS.md, subtask 3), or a help library (.HLB,
// /HELP; docs/planning/PHASE-90 - VMS help.md, subtask 3) with internal/lbr, extracts
// modules from one, and lists one as LIBRARY/LIST does. /MACRO, /OBJECT,
// /SHARE, /TEXT, or /HELP picks the type of a new library (an object library by
// default, as on VMS) and the library's default file type; an existing
// library keeps its own type, and naming another is an error.
//
// Input files are inserted in order: macro source (.MAR), one module per
// macro, object files (.OBJ), one module per object module, shareable
// images (.EXE), one module each, holding its universal symbols, or text
// files (.TXT), one module each, named by the file's /MODULE= or for the
// file, or help files (.HLP), a module for each level-1 key (lbr's
// HelpModules). With no
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
// A new library (/CREATE) is laid out whole by an lbr.Builder, as
// LIBRARIAN lays one out, and given /CREATE=BLOCKS's allocation. An
// existing library is changed in place by an lbr.Updater, as LIBRARIAN
// changes one through LBRSHR (docs/planning/PHASE-90 - VMS help.md, subtask 11e):
// each module written into new data, its keys entered, a replaced or
// deleted module's data deleted, and LIBRARIAN's update history records
// added for what changed; a Builder edits a copy alongside, for the
// modules' names and keys. Either way the library is written only when
// every step has succeeded, so an error leaves it as it was. A changed
// library keeps its name, version, and allocation, as LIBRARIAN's does,
// so that a library copied back to VMS is where VMS expects it: a host
// library is replaced, and a volume library is rewritten at its version
// (rms.Session.RewriteRecordFile). /CREATE makes a new file, a new version
// on a volume, as on VMS. /DATA=REDUCE and /DATA=EXPAND write a new
// version laid out afresh (lbr.Rebuild). testdata/help90's round 3 holds
// the libraries all of these make to VMS 7.1's byte for byte.
//
// A text library's failures use LIBRARIAN's messages and statuses, as
// testdata/for83 shows them, and a help library's LIBRARIAN's messages, as
// testdata/help90 shows them; the other types' are still govax's words.
//
// A help file's bad key line (ILLKEYLVL, KEYNAMLNG) ends that file's reading,
// but not the command, as on VMS 7.1: the modules before it are inserted, the
// module it was in is left in the library's data as VMS leaves it
// (lbr.Builder.Abandon), the other input files are read, and the library is
// written. So is an object library given a file of text (RECTYP).

// LibraryOptions is one LIBRARY command.
type LibraryOptions struct {
	// Library is the library file, and LibraryHost an explicit /HOST on
	// it. Inputs are the input files, and InputHost an explicit /HOST on
	// them.
	Library     string
	LibraryHost bool
	Inputs      []string
	InputHost   bool

	// Modules holds each input's /MODULE= (a text module's name), "" for
	// none. It may be shorter than Inputs.
	Modules []string

	// Create is /CREATE, CreateSizes its values ("KEYSIZE:31"), Insert
	// /INSERT, and Replace /REPLACE.
	Create      bool
	CreateSizes []string
	Insert      bool
	Replace     bool

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

	// History is /HISTORY: the listing's update history. Data is
	// /DATA='s keyword, REDUCE or EXPAND, or "".
	History bool
	Data    string

	// Macro is /MACRO, Object /OBJECT, Share /SHARE, Text /TEXT, and Help
	// /HELP.
	// NoSqueeze is /NOSQUEEZE, Selective /SELECTIVE_SEARCH, and Log /LOG.
	Macro     bool
	Object    bool
	Share     bool
	Text      bool
	Help      bool
	NoSqueeze bool
	Selective bool
	Log       bool
}

// library is one LIBRARY command in progress.
type library struct {
	c    *Context
	opts LibraryOptions
	now  uint64

	lib     *lbr.Library // the library as read, or as written
	b       *lbr.Builder // the library being changed, if it is
	// upd is an existing library being changed in place, as LIBRARIAN
	// changes one; b keeps the same changes, for its modules' names and
	// keys. Nil for a new library, which b lays out.
	upd *lbr.Updater
	// changes are the modules each history operation changed, for
	// LIBRARIAN's update history records.
	changes map[byte][]string
	found   rms.FileLocation
	changed bool
	logs    []func(name string) error // /LOG's messages, which name the library written
	after   []func(name string)       // messages written once the library is, which name it
	blocks  uint32                    // a new library's allocation, /CREATE=BLOCKS

	// problem is the worst of the messages report wrote, the command's
	// status once the library is written; 0 for none.
	problem uint32
}

// Library runs one LIBRARY command.
func (c *Context) Library(opts LibraryOptions) error {
	l := &library{c: c, opts: opts, now: vmsdef.Time(time.Now())}

	if err := l.run(); err != nil {
		return err
	}

	if l.problem != 0 {
		return vmserrors.InhibitMessage(vmserrors.ConditionError{Status: l.problem, Text: "LIBRARY"})
	}

	// LIBRARIAN's status for a command that worked, as LINK's
	// (testdata/link68's log: %X10000001 after LIBRARY/CREATE/SHARE).
	if c.SetStatus != nil {
		c.SetStatus(ssNormal | stsInhibitMsg)
	}

	return nil
}

func (l *library) run() error {
	s := l.c.ContainerSession
	opts := l.opts

	typ, libType := lbr.TypeObject, "OLB"

	switch {
	case opts.Macro:
		typ, libType = lbr.TypeMacro, "MLB"
	case opts.Share:
		typ = lbr.TypeShareable
	case opts.Text:
		typ, libType = lbr.TypeText, "TLB"
	case opts.Help:
		typ, libType = lbr.TypeHelp, "HLB"
	}

	// /NAMES lists an object library's global symbols. DCL's own syntax for
	// /TEXT has no such qualifier (testdata/for83's log).
	if opts.Text && opts.Names {
		return vmserrors.NewSegment(vmserrors.CLI_CONFLICT, "NAMES")
	}

	loc, err := s.Locate(opts.Library, opts.LibraryHost)
	if err != nil {
		return FileFailure(err, opts.Library)
	}

	loc = loc.WithDefaultType(libType)

	edit := len(opts.Inputs) > 0 || len(opts.Delete) > 0

	switch {
	case (opts.Insert || opts.Replace) && len(opts.Inputs) == 0:
		return l.failed(fmt.Errorf("/INSERT and /REPLACE need input files"), loc.Name)
	case !opts.Create && !edit && len(opts.Extract) == 0 && !opts.List && opts.Data == "":
		return l.failed(fmt.Errorf("nothing to do: name input files, or /CREATE, /DATA, /DELETE, /EXTRACT, or /LIST"), loc.Name)
	}

	if opts.Data != "" {
		if err := l.rebuild(loc); err != nil {
			return err
		}
	}

	if opts.Create {
		sizes, blocks, err := createSizes(opts.CreateSizes)
		if err != nil {
			return err
		}

		l.blocks = blocks

		if l.b, err = lbr.CreateWith(typ, sizes); err != nil {
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

			if err := l.openUpdater(); err != nil {
				return err
			}
		}
	}

	for _, pattern := range opts.Delete {
		if err := l.delete(pattern); err != nil {
			return err
		}
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

// defaultBlocks is /CREATE=BLOCKS's default: the blocks a new library's
// file is given (VMS 7.1's libraries took 102 on a volume of 3-block
// clusters: testdata/help90).
const defaultBlocks = 100

// createSizes reads /CREATE's values, and BLOCKS's, the file's first
// allocation.
func createSizes(values []string) (lbr.CreateOptions, uint32, error) {
	var sizes lbr.CreateOptions

	blocks := uint32(defaultBlocks)

	for _, value := range values {
		keyword, number, _ := strings.Cut(strings.ToUpper(value), ":")

		n, err := strconv.Atoi(number)
		if err != nil || n < 0 {
			return sizes, 0, vmserrors.New(vmserrors.CLI_IVKEYW, value)
		}

		switch {
		case abbreviates(keyword, "BLOCKS", 1):
			blocks = uint32(n)
		case abbreviates(keyword, "GLOBALS", 1):
			sizes.Globals = n
		case abbreviates(keyword, "HISTORY", 1):
			sizes.History = n
		case abbreviates(keyword, "KEYSIZE", 1):
			sizes.KeySize = n
		case abbreviates(keyword, "MODULES", 1):
			sizes.Modules = n
		default:
			return sizes, 0, vmserrors.New(vmserrors.CLI_IVKEYW, value)
		}
	}

	return sizes, blocks, nil
}

// LIBRARIAN's condition values for a text library's failures, as $STATUS
// shows them in testdata/for83's log (without its inhibit bit).
const (
	librarInsertErr  = 0x008680E2 // %LIBRAR-E-INSERTERR
	librarLookupErr  = 0x008680F2 // %LIBRAR-E-LOOKUPERR
	librarModNameLng = 0x00868102 // %LIBRAR-E-MODNAMLNG
)

// LIBRARIAN's condition values for the messages testdata/help90's round 3
// showed with their $STATUS (docs/planning/PHASE-90 - VMS help.md, subtask 11e).
const (
	librarNoMatch   = 0x00868080 // %LIBRAR-W-NOMTCHFOU
	librarDupModule = 0x00868088 // %LIBRAR-W-DUPMODULE
	librarKeyLevel  = 0x008680D2 // %LIBRAR-E-ILLKEYLVL
	librarKeyLength = 0x008680EA // %LIBRAR-E-KEYNAMLNG
)

// LIBRARIAN's condition values for messages whose $STATUS no probe showed:
// LIBRAR's facility and each message's severity, with no message number
// (unconfirmed).
const (
	librarWarning = 0x00868000 // %LIBRAR-W-HISTERR
	librarError   = 0x00868002 // %LIBRAR-E-RECTYP, INDEXERR
	librarFatal   = 0x00868004 // %LIBRAR-F-MHDERR
)

// objectHeader and objectEnd are the object records a module starts and
// ends with (OBJ$C_HDR, and OBJ$C_EOM and OBJ$C_EOMW).
const (
	objectHeader  = 0
	objectEnd     = 3
	objectEndWord = 7
)

// objectRecordTypes is how many object record types there are (OBJ$C_HDR to
// OBJ$C_EOMW): a record whose first byte is another value isn't an object
// record (RECTYP).
const objectRecordTypes = 8

// stop ends the command with LIBRARIAN's messages, which it writes itself,
// and status: DCL shows nothing more, and $STATUS has the inhibit bit, as it
// does after LIBRARIAN's own.
func (l *library) stop(status uint32, messages ...string) error {
	for _, m := range messages {
		l.c.Printf("%s\n", m)
	}

	return vmserrors.InhibitMessage(vmserrors.ConditionError{Status: status, Text: messages[len(messages)-1]})
}

// report writes LIBRARIAN's messages and goes on, the command's status
// becoming status if that's the worst so far.
func (l *library) report(status uint32, messages ...string) {
	for _, m := range messages {
		l.c.Printf("%s\n", m)
	}

	if l.problem == 0 || status&7 > l.problem&7 {
		l.problem = status
	}
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

// openUpdater starts the in-place change of the library open read: the
// LBRSHR way LIBRARIAN changes an existing library, each module written in
// new data and its keys entered, and the library's update history kept
// (docs/planning/PHASE-90 - VMS help.md, subtask 11e).
func (l *library) openUpdater() error {
	var err error

	if l.upd, err = lbr.OpenUpdater(l.lib.Bytes()); err != nil {
		return l.failed(err, l.found.Name)
	}

	l.changes = map[byte][]string{}

	return nil
}

// rebuild is /DATA=REDUCE or /DATA=EXPAND: the library laid out afresh, its
// records data-reduced or not (lbr.Rebuild), as a new version of the file
// (round 3's WD8.HLB;2 and ;3).
func (l *library) rebuild(loc rms.FileLocation) error {
	var reduce bool

	switch keyword := strings.ToUpper(l.opts.Data); {
	case keyword != "" && abbreviates(keyword, "REDUCE", 1):
		reduce = true
	case keyword != "" && abbreviates(keyword, "EXPAND", 1):
	default:
		return vmserrors.New(vmserrors.CLI_IVKEYW, l.opts.Data)
	}

	if err := l.open(loc); err != nil {
		return err
	}

	data, err := lbr.Rebuild(l.lib, l.now, reduce)
	if err != nil {
		return l.failed(err, l.found.Name)
	}

	// The new version is given the old one's allocation (WD8.HLB;2 and ;3).
	alloc, err := l.c.ContainerSession.AllocatedBlocks(l.found)
	if err != nil {
		return FileFailure(err, l.found.Name)
	}

	created, err := l.c.ContainerSession.CreateRecordFileAllocated(loc, rms.ImageBlocks, libraryBlocks(data), alloc)
	if err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, l.found.Name)
	}

	l.found = created

	if l.lib, err = lbr.Open(data); err != nil {
		return l.failed(err, created.Name)
	}

	return nil
}

// libraryBlocks is a library file's bytes as its blocks.
func libraryBlocks(data []byte) [][]byte {
	blocks := make([][]byte, 0, len(data)/512)
	for i := 0; i < len(data); i += 512 {
		blocks = append(blocks, data[i:i+512])
	}

	return blocks
}

// user is the user name LIBRARIAN's history records give.
func (l *library) user() string {
	if rtl := l.c.RTL; rtl != nil && rtl.Process != nil && rtl.Process.Username != "" {
		return rtl.Process.Username
	}

	return "SYSTEM"
}

// open reads an existing library.
func (l *library) open(loc rms.FileLocation) error {
	data, found, err := l.c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		return FileFailure(err, loc.Name)
	}

	l.found = found

	if l.lib, err = lbr.Open(data); err != nil {
		return l.failed(err, found.Name)
	}

	switch t := l.lib.Type; {
	case l.opts.Macro && t != lbr.TypeMacro, l.opts.Object && t != lbr.TypeObject, l.opts.Share && t != lbr.TypeShareable,
		l.opts.Text && t != lbr.TypeText, l.opts.Help && t != lbr.TypeHelp:
		return l.failed(fmt.Errorf("%s is a %s library", found.Name, t), found.Name)
	}

	return nil
}

func (l *library) delete(pattern string) error {
	names := l.b.Match(pattern)
	if len(names) == 0 {
		// testdata/help90's /DELETE=NOSUCH.
		if l.b.Type == lbr.TypeHelp {
			return l.stop(librarLookupErr,
				fmt.Sprintf("%%LIBRAR-E-LOOKUPERR, error looking up %s in %s", pattern, l.found.Name),
				"-LBR-E-KEYNOTFND, key not found")
		}

		l.warn(fmt.Errorf("no module matches %s", pattern))
	}

	for _, name := range names {
		if l.upd != nil {
			if e, ok := l.b.Module(name); ok {
				if err := l.updateDelete(e); err != nil {
					return l.failed(err, l.found.Name)
				}
			}

			l.changes[lbr.HistoryDeleted] = append(l.changes[lbr.HistoryDeleted], name)
		}

		_ = l.b.Delete(name)
		l.changed = true
		l.log(vmserrors.CLI_LIBDELETED, name)
	}

	return nil
}

// updateDelete takes module e out of the library being changed in place:
// its symbols' keys, its name's, and its data.
func (l *library) updateDelete(e *lbr.Entry) error {
	key, err := l.upd.Key(0, e.Name)
	if err != nil {
		return err
	}

	rfa, ok := l.upd.Lookup(0, key)
	if !ok {
		return lbr.ErrKeyNotFound
	}

	if err := l.deleteSymbols(e); err != nil {
		return err
	}

	if err := l.upd.DeleteKey(0, key); err != nil {
		return err
	}

	return l.upd.DeleteData(rfa)
}

// deleteSymbols takes module e's global symbols out of index 2.
func (l *library) deleteSymbols(e *lbr.Entry) error {
	for _, sym := range e.Symbols {
		k, err := l.upd.Key(1, sym)
		if err != nil {
			return err
		}

		if err := l.upd.DeleteKey(1, k); err != nil {
			return err
		}
	}

	return nil
}

// updateInsert writes module e into the library being changed in place and
// enters its keys: its global symbols first, then its name, as LIBRARIAN
// enters them (Builder.Bytes's comment). A key already there leaves the
// module's data in the library with no key (round 3's WD1.HLB), and
// reports false.
//
// LIBRARIAN reads the header of a module of the same name before it writes
// the new one, so a session's first module goes on in the last data block
// if that's where the old module is (round 3's WD1.HLB, whose BONLY was
// there; round 1b's H90E.HLB, whose ALPHA wasn't, started a new block).
func (l *library) updateInsert(e *lbr.Entry) (bool, error) {
	key, err := l.upd.Key(0, e.Name)
	if err != nil {
		return false, err
	}

	if old, found := l.upd.Lookup(0, key); found {
		l.upd.NoteRead(old)
	}

	rfa := l.upd.PutModule(e)

	// After a key that can't go in, LIBRARIAN ends a module once more,
	// which with no module starts a block of its own with an end of module
	// (WD1.HLB, after each of its two modules).
	if _, dup := l.upd.Lookup(0, key); dup {
		l.upd.PutEnd()

		return false, nil
	}

	if err := l.insertSymbols(e, rfa); err != nil {
		return false, err
	}

	return true, l.upd.InsertKey(0, key, rfa)
}

func (l *library) insertSymbols(e *lbr.Entry, rfa lbr.RFA) error {
	for _, sym := range e.Symbols {
		k, err := l.upd.Key(1, sym)
		if err != nil {
			return err
		}

		if err := l.upd.InsertKey(1, k, rfa); err != nil {
			return err
		}
	}

	return nil
}

// updateReplace writes module e into the library being changed in place,
// in place of old, a module of the same name (nil for none): the old
// module's symbols come out, the new one's go in, its name's key points at
// the new data, and the old data is deleted.
func (l *library) updateReplace(e, old *lbr.Entry) error {
	rfa := l.upd.PutModule(e)

	if old == nil {
		_, err := l.updateInsertKeys(e, rfa)

		return err
	}

	key, err := l.upd.Key(0, e.Name)
	if err != nil {
		return err
	}

	was, ok := l.upd.Lookup(0, key)
	if !ok {
		return lbr.ErrKeyNotFound
	}

	if err := l.deleteSymbols(old); err != nil {
		return err
	}

	if err := l.insertSymbols(e, rfa); err != nil {
		return err
	}

	if err := l.upd.ReplaceKey(0, key, was, rfa); err != nil {
		return err
	}

	return l.upd.DeleteData(was)
}

func (l *library) updateInsertKeys(e *lbr.Entry, rfa lbr.RFA) (bool, error) {
	key, err := l.upd.Key(0, e.Name)
	if err != nil {
		return false, err
	}

	if err := l.insertSymbols(e, rfa); err != nil {
		return false, err
	}

	return true, l.upd.InsertKey(0, key, rfa)
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
			return FileFailure(err, name)
		}

		module := ""
		if i < len(l.opts.Modules) {
			module = l.opts.Modules[i]
		}

		// A wildcard name is every file it matches, in directory order,
		// each an input of its own (BUILDHELP.COM's "[.SOURCES]*.HLP").
		files := []rms.FileLocation{loc}
		if !loc.Host && strings.ContainsAny(loc.Name, "*%") {
			if files, err = l.matchingInputs(loc); err != nil {
				return err
			}
		}

		for _, file := range files {
			found, err := l.insertFile(file, module, replace)
			if err != nil {
				return err
			}

			prev = found
		}
	}

	return nil
}

// inputTypes are each library type's default input file type.
var inputTypes = map[lbr.Type]string{
	lbr.TypeText: "TXT", lbr.TypeHelp: "HLP", lbr.TypeMacro: "MAR",
	lbr.TypeObject: "OBJ", lbr.TypeShareable: "EXE",
}

// matchingInputs is the files a wildcard input names, with the library's
// default input type; none is the first one's file-not-found.
func (l *library) matchingInputs(loc rms.FileLocation) ([]rms.FileLocation, error) {
	if t, ok := inputTypes[l.b.Type]; ok {
		loc = loc.WithDefaultType(t)
	}

	names, err := l.c.ContainerSession.Search(loc.Name)
	if err != nil {
		return nil, FileFailure(err, loc.Name)
	}

	if len(names) == 0 {
		return []rms.FileLocation{loc}, nil
	}

	files := make([]rms.FileLocation, len(names))
	for i, n := range names {
		files[i] = rms.FileLocation{Name: n}
	}

	return files, nil
}

// insertFile inserts (or with replace, replaces) the modules of one input
// file, and returns the file as found.
func (l *library) insertFile(loc rms.FileLocation, module string, replace bool) (rms.FileLocation, error) {
	entries, abandoned, found, err := l.readInput(loc, module)
	if err != nil {
		return found, err
	}

	for _, e := range entries {
		e.Inserted = l.now
		l.b.Updated = l.now

		if !replace {
			if _, ok := l.b.Module(e.Name); ok {
				// LIBRARIAN writes a help module's data before its key
				// finds the name taken (round 3's WD1.HLB).
				if l.upd != nil && l.b.Type == lbr.TypeHelp {
					if _, err := l.updateInsert(e); err != nil {
						return found, l.failed(err, l.found.Name)
					}

					l.changed = true
				}

				if l.b.Type == lbr.TypeText {
					return found, l.stop(librarInsertErr,
						fmt.Sprintf("%%LIBRAR-E-DUPMOD, module %s from file %s already in %s", e.Name, found.Name, l.found.Name),
						fmt.Sprintf("%%LIBRAR-E-INSERTERR, error inserting %s in %s", e.Name, l.found.Name),
						"-LBR-E-DUPKEY, duplicate key in index")
				}

				if l.b.Type == lbr.TypeHelp {
					l.report(librarDupModule, fmt.Sprintf("%%LIBRAR-W-DUPMODULE, module %s from file %s already in %s", e.Name, found.Name, l.found.Name))
				} else {
					l.warn(fmt.Errorf("module %s from %s is already in the library", e.Name, found.Name))
				}

				continue
			}

			if err := l.b.Insert(e); err != nil {
				return found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
			}

			if l.upd != nil {
				if _, err := l.updateInsert(e); err != nil {
					return found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
				}

				l.changes[lbr.HistoryInserted] = append(l.changes[lbr.HistoryInserted], e.Name)
			}

			l.log(vmserrors.CLI_LIBINSERTED, e.Name)
		} else {
			var old *lbr.Entry
			if m, ok := l.b.Module(e.Name); ok {
				old = m
			}

			replaced, err := l.b.Replace(e)
			if err != nil {
				return found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
			}

			if l.upd != nil {
				if err := l.updateReplace(e, old); err != nil {
					return found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
				}

				op := byte(lbr.HistoryInserted)
				if replaced {
					op = lbr.HistoryReplaced
				}

				l.changes[op] = append(l.changes[op], e.Name)
			}

			if replaced {
				l.log(vmserrors.CLI_LIBREPLACED, e.Name)
			} else {
				l.log(vmserrors.CLI_LIBINSERTED, e.Name)
			}
		}

		l.changed = true
	}

	if abandoned != nil {
		abandoned.Inserted = l.now
		l.b.Abandon(abandoned)

		if l.upd != nil {
			l.upd.PutAbandoned(abandoned)
		}

		l.changed = true
	}

	return found, nil
}

// readInput reads one input file's modules. module is the file's /MODULE=,
// which only a text file's module can have.
//
// A help file whose reading stopped at a bad key line also returns the
// module that line was in, as far as it went, for lbr.Builder.Abandon.
func (l *library) readInput(loc rms.FileLocation, module string) ([]*lbr.Entry, *lbr.Entry, rms.FileLocation, error) {
	s := l.c.ContainerSession

	if module != "" && l.b.Type != lbr.TypeText {
		return nil, nil, loc, l.failed(fmt.Errorf("/MODULE names a module of a text library, not a %s library", l.b.Type), l.found.Name)
	}

	switch l.b.Type {
	case lbr.TypeText:
		loc = loc.WithDefaultType("TXT")

		records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			return nil, nil, found, FileFailure(err, loc.Name)
		}

		if module == "" {
			module = sourceStem(found)
		}

		module = strings.ToUpper(module)

		if len(module) > l.b.KeySize {
			return nil, nil, found, l.stop(librarModNameLng,
				fmt.Sprintf("%%LIBRAR-E-MODNAMLNG, illegal module name (%s) of %d characters in %s", module, len(module), found.Name))
		}

		return []*lbr.Entry{lbr.TextModule(module, records)}, nil, found, nil

	case lbr.TypeHelp:
		return l.readHelp(loc)

	case lbr.TypeMacro:
		loc = loc.WithDefaultType("MAR")

		records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			return nil, nil, found, FileFailure(err, loc.Name)
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
			return nil, nil, found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
		}

		return entries, nil, found, nil

	case lbr.TypeObject:
		loc = loc.WithDefaultType("OBJ")

		records, found, err := s.ReadRecordFile(loc, rms.VariableRecords)
		if err != nil {
			return nil, nil, found, FileFailure(err, loc.Name)
		}

		// A file that isn't an object file, a help file's "1 KEY" among them
		// (testdata/help90's H90Q): LIBRARIAN names the record's type and no
		// module, and goes on.
		if len(records) > 0 && len(records[0]) > 0 && records[0][0] >= objectRecordTypes {
			l.report(librarError, fmt.Sprintf("%%LIBRAR-E-RECTYP, illegal record type (%d) in module  in %s", records[0][0], found.Name))

			return nil, nil, found, nil
		}

		if len(records) > 0 && len(records[0]) > 0 && records[0][0] != objectHeader {
			return nil, l.badSequence(records, found), found, nil // nil: the module is in the library already
		}

		entries, err := l.b.ObjectModules(records, l.opts.Selective)
		if err != nil {
			return nil, nil, found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
		}

		return entries, nil, found, nil

	case lbr.TypeShareable:
		data, found, err := s.ReadRawFile(loc.WithDefaultType("EXE"))
		if err != nil {
			return nil, nil, found, FileFailure(err, loc.Name)
		}

		e, err := l.b.ShareableModule(data)
		if err != nil {
			return nil, nil, found, l.failed(fmt.Errorf("%s: %w", found.Name, err), l.found.Name)
		}

		return []*lbr.Entry{e}, nil, found, nil
	}

	return nil, nil, loc, l.failed(fmt.Errorf("LIBRARY can't insert modules in a %s library", l.b.Type), l.found.Name)
}

// badSequence is an object file whose first record isn't a module's header,
// as LIBRARIAN reads it: an illegal record sequence in a module with no
// name, which ends at the first end of module record (and is compiled
// with warnings if that record says so), and whose key, its empty name,
// can't go in. Round 3's WD4.HLB, an object library made from a help
// library, whose header block reads as an end of module record of 512
// bytes, of severity 1. The module's records are left in the library with
// no key, which VMS 7.1 did too (its blocks differ from govax's:
// docs/planning/PHASE-90 - VMS help.md, subtask 11e).
func (l *library) badSequence(records [][]byte, found rms.FileLocation) *lbr.Entry {
	// The module goes in as LIBRARIAN leaves it (Builder.Failed), so
	// nothing is returned for insertFile to abandon.
	l.c.Printf("%%LIBRAR-E-SEQNCE, illegal record sequence in module  in %s\n", found.Name)

	end := len(records)

	for i, r := range records {
		if len(r) > 0 && (r[0] == objectEnd || r[0] == objectEndWord) {
			end = i + 1

			// EOM$B_COMCOD: 1 is warnings, 2 or more errors (the word
			// for errors is unconfirmed).
			switch {
			case len(r) > 1 && r[1] == 1:
				l.c.Printf("%%LIBRAR-W-COMCOD, compilation warnings in module  file %s\n", found.Name)
			case len(r) > 1 && r[1] > 1:
				l.c.Printf("%%LIBRAR-E-COMCOD, compilation errors in module  file %s\n", found.Name)
			}

			break
		}
	}

	// The library's name is the file written, which a new library's isn't
	// yet.
	l.after = append(l.after, func(name string) {
		l.report(librarInsertErr, "%LIBRAR-E-INSERTERR, error inserting  in "+name, "-LBR-E-INVKEY, invalid key")
	})

	e := &lbr.Entry{Records: records[:end], Inserted: l.now}
	l.b.Failed(e)

	if l.upd != nil {
		l.upd.PutModule(e)
		l.upd.PutEnd()
	}

	l.changed = true

	return nil
}

// readHelp reads a help file's modules. A bad key line is LIBRARIAN's
// message, and the file's modules before it are inserted.
func (l *library) readHelp(loc rms.FileLocation) ([]*lbr.Entry, *lbr.Entry, rms.FileLocation, error) {
	loc = loc.WithDefaultType("HLP")

	records, found, err := l.c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return nil, nil, found, FileFailure(err, loc.Name)
	}

	entries, bad := l.b.HelpModules(records)
	if bad == nil {
		return entries, nil, found, nil
	}

	// VMS 7.1 says "expected 1" whatever level would have been allowed
	// (testdata/help90, rounds 1 and 1b).
	switch bad.Problem {
	case lbr.HelpBadLevel:
		l.report(librarKeyLevel, fmt.Sprintf("%%LIBRAR-E-ILLKEYLVL, illegal key level %d; key %s in %s; expected 1", bad.Level, bad.Key, found.Name))
	case lbr.HelpKeyTooLong:
		l.report(librarKeyLength, fmt.Sprintf("%%LIBRAR-E-KEYNAMLNG, key %s name length illegal in %s", bad.Key, found.Name))
	}

	return entries, bad.Partial, found, nil
}

// write writes the library: a new one as a new file (a new version, on a
// volume), and a changed one over the old, keeping its version.
func (l *library) write() error {
	var data []byte

	if l.upd != nil {
		l.upd.SetUpdated(l.now)

		if err := l.putHistory(); err != nil {
			return l.failed(err, l.found.Name)
		}

		data = l.upd.Bytes()
	} else {
		data = l.b.Bytes()
	}

	blocks := libraryBlocks(data)

	s := l.c.ContainerSession
	write := s.RewriteRecordFile

	if l.opts.Create {
		write = func(loc rms.FileLocation, kind rms.RecordKind, records [][]byte) (rms.FileLocation, error) {
			return s.CreateRecordFileAllocated(loc, kind, records, l.blocks)
		}
	}

	created, err := write(l.found, rms.ImageBlocks, blocks)
	if err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, l.found.Name)
	}

	l.found = created

	if l.lib, err = lbr.Open(data); err != nil {
		return l.failed(err, created.Name)
	}

	for _, m := range l.logs {
		l.c.Printf("%%%s\n", m(created.Name))
	}

	for _, f := range l.after {
		f(created.Name)
	}

	return nil
}

// putHistory adds LIBRARIAN's update history records for what the command
// changed in a library that already existed, a record for each operation
// (a new library's modules get none: round 3's H90T, created and then
// changed, had the change's record alone). A library that keeps no history
// gets none.
func (l *library) putHistory() error {
	for _, op := range []byte{lbr.HistoryDeleted, lbr.HistoryInserted, lbr.HistoryReplaced} {
		names := l.changes[op]
		if len(names) == 0 {
			continue
		}

		err := l.upd.PutHistory(lbr.LibrarianHistory(l.now, op, l.user(), names))
		if err != nil && !errors.Is(err, lbr.ErrNoHistory) {
			return err
		}
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

	// A help library's pattern that matches nothing is a warning
	// (testdata/help90's /EXTRACT=* of an empty library).

	var records [][]byte

	n := 0

	for _, pattern := range l.opts.Extract {
		keys := l.lib.Match(pattern)
		if len(keys) == 0 {
			if (l.lib.Type == lbr.TypeHelp || l.lib.Type == lbr.TypeText) && strings.ContainsAny(pattern, "*%") {
				l.report(librarNoMatch, "%LIBRAR-W-NOMTCHFOU, no matches found for "+pattern)

				continue
			}

			if l.lib.Type == lbr.TypeText || l.lib.Type == lbr.TypeHelp {
				return l.stop(librarLookupErr,
					fmt.Sprintf("%%LIBRAR-E-LOOKUPERR, error looking up %s in %s", pattern, l.found.Name),
					"-LBR-E-KEYNOTFND, key not found")
			}

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

	// /OUTPUT=SYS$OUTPUT shows the modules (testdata/help90's part A), as
	// ANALYZE/RMS_FILE's /OUTPUT does.
	if strings.EqualFold(l.opts.Output, "SYS$OUTPUT") && kind == rms.TextRecords {
		for _, r := range records {
			l.c.Printf("%s\n", r)
		}

		return nil
	}

	s := l.c.ContainerSession

	out, err := outputLocation(s, l.opts.Output, l.found, typ)
	if err != nil {
		return FileFailure(err, l.opts.Output)
	}

	out = out.WithDefaultType(typ)

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
		Name:    l.found.Name,
		Now:     l.now,
		Full:    l.opts.Full,
		Names:   l.opts.Names,
		History: l.opts.History,
		Width:   width,
	})

	var (
		history lbr.HistoryFormatError
		header  lbr.ModuleHeaderError
	)

	switch {
	case errors.As(err, &header):
		lines = append(lines, fmt.Sprintf("%%LIBRAR-F-MHDERR, module header error for %s in %s", header.Name, l.found.Name),
			"-LBR-E-INVRFA, invalid RFA")
		l.problem = librarFatal
	case errors.Is(err, lbr.ErrNoIndex):
		lines = append(lines, "%LIBRAR-E-INDEXERR, index error in "+l.found.Name, "-LBR-E-ILLIDXNUM, illegal index number")
		l.problem = librarError
	case errors.As(err, &history):
		// VMS 7.1's LIBRARIAN on a history record the LBR$ routines wrote:
		// its FAO call failed, and these are its messages, numbers and all
		// (round 3's W5 and H90T).
		lines = append(lines,
			"%LIBRAR-E-FAOFAIL, FAO system service failure",
			"-SYSTEM-F-ACCVIO, access violation, reason mask=D7, virtual address=03C00000, PC=00000000, PSL=20000000",
			"%LIBRAR-W-HISTERR, error accessing library update history in "+l.found.Name,
			"-SYSTEM-F-ACCVIO, access violation, reason mask=F0, virtual address=03C00000, PC=00000000, PSL=00000000")
		if l.problem == 0 {
			l.problem = librarWarning
		}
	case err != nil:
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
		return FileFailure(err, l.opts.ListFile)
	}

	out = out.WithDefaultType("LIS")

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := s.CreateRecordFile(out, rms.TextRecords, records); err != nil {
		return objectFailureAs(vmserrors.CLI_LIBRARY, err, out.Name)
	}

	return nil
}
