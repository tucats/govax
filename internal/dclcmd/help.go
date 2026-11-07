package dclcmd

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is DCL's HELP (docs/PHASE-90 - VMS help.md, subtask 6): the
// command's qualifiers, the libraries it reads, and where its output goes.
// What HELP shows and how it prompts are internal/help's Session; this
// file gives it its libraries, a way to write, and a way to read.
//
// The libraries, as VMS 7.1's HELP found them (testdata/help90's parts B
// and C):
//
//   - The main library is /LIBRARY's, SYS$HELP:HELPLIB.HLB by default, its
//     name parsed with the default SYS$HELP:.HLB ("/LIBRARY=H90DIR:H90A" is
//     H90DIR:H90A.HLB). One that can't be opened is %HELP-E-OPENIN with
//     RMS's reason, and HELP does nothing more.
//   - The user libraries are HLP$LIBRARY, HLP$LIBRARY_1, HLP$LIBRARY_2, ...
//     in the process table, then the group table, then the system table
//     (/USERLIBRARY's choice of them: ALL by default, NONE, or a list), the
//     numbers in each table ending at the first missing one. A library a
//     name leads to that can't be opened is passed over.
//   - /NOLIBRARY makes the first user library the main one; with no user
//     library either, HELP is %LBR-E-NOHLPLIBS.
//
// The output is SYS$OUTPUT, or /OUTPUT's file (HELP.LIS by default, in the
// default directory: "/OUTPUT=[]"); /OUTPUT with no value is SYS$OUTPUT,
// and /NOOUTPUT writes nothing. Lists are 132 columns wide in a file (a
// command procedure's /OUTPUT file too) and the terminal's width at the
// terminal.

// HelpOptions is one HELP command.
type HelpOptions struct {
	// Words are the request, split as internal/help splits it.
	Words []string

	// Prompt is /[NO]PROMPT (on unless /NOPROMPT), Instructions
	// /[NO]INSTRUCTIONS (on unless negated), LibList /[NO]LIBLIST (on
	// unless negated).
	Prompt       bool
	Instructions bool
	LibList      bool

	// Library is /LIBRARY's value, "" for the default; NoLibrary is
	// /NOLIBRARY.
	Library   string
	NoLibrary bool

	// UserLibraries is /USERLIBRARY's list (PROCESS, GROUP, SYSTEM, ALL,
	// NONE), nil for the default (ALL); NoUserLibrary is /NOUSERLIBRARY.
	UserLibraries []string
	NoUserLibrary bool

	// Output is /OUTPUT's file, OutputGiven whether /OUTPUT was given (with
	// Output "" it is SYS$OUTPUT), NoOutput /NOOUTPUT.
	Output      string
	OutputGiven bool
	NoOutput    bool

	// Page is /PAGE's keywords (Paged with none), Paged whether /PAGE was
	// given, NoPage /NOPAGE. Without either, HELP pages when its output is
	// a terminal's.
	Page   []string
	Paged  bool
	NoPage bool

	// Unimplemented names the first qualifier given that waits for a later
	// phase (SEARCH, EXACT, HIGHLIGHT, WRAP, MESSAGE).
	Unimplemented string
}

// When no /LIBRARY is given and SYS$HELP:HELPLIB.HLB can't be opened (no
// system volume, or one without the library), HELP reads govax's own copy,
// the one built into it (internal/bootdata), as the grammars fall back on
// govax's own (Decision 5).

// The statuses of HELP's own messages. VMS 7.1's $STATUS after them wasn't
// shown by the probe: OPENIN is the shared message's code with no facility
// and NOHLPLIBS LBR's facility with no message number, both unconfirmed.
// Each has its message shown already (the inhibit bit).
const (
	helpOpenIn    = 0x1000109A
	helpNoHlpLibs = 0x10268002
	helpRMSFNF    = 0x00018292 // RMS$_FNF
	helpDefault   = "SYS$HELP:.HLB"
	helpMainSpec  = "HELPLIB"
	helpListFile  = "HELP.LIS"
	helpFileWidth = 132
)

// helpLibrary is a library HELP opened: the library and its name for
// "@name" and /LIBLIST ("H90B").
type helpLibrary struct {
	lib  help.Library
	name string
}

// Help implements DCL's HELP. read reads a response to a prompt (ok false
// at the input's end), and toFile says the DCL's output goes to a file now.
func (c *Context) Help(opts HelpOptions, read func(prompt string) (string, bool), toFile bool) error {
	if opts.Unimplemented != "" {
		return vmserrors.New(vmserrors.CLI_HELPNOTIMPL, opts.Unimplemented)
	}

	for _, k := range opts.Page {
		if key, _, _ := strings.Cut(strings.ToUpper(k), "="); strings.HasPrefix("SAVE", key) && key != "" {
			return vmserrors.New(vmserrors.CLI_HELPNOTIMPL, "/PAGE=SAVE")
		}
	}

	users, err := c.userHelpLibraries(opts)
	if err != nil {
		return err
	}

	var main help.Library

	switch {
	case !opts.NoLibrary:
		spec := opts.Library
		if spec == "" {
			spec = helpMainSpec
		}

		l, err := c.openHelpLibrary(spec)
		if err != nil && opts.Library == "" {
			l, err = builtinHelpLibrary(bootdata.HelpLibrary)
		}

		if err != nil {
			return err
		}

		main = l.lib
	case len(users) > 0:
		main = users[0].Library
	default:
		c.Printf("%%LBR-E-NOHLPLIBS, no default help libraries can be opened\n")

		return vmserrors.InhibitMessage(vmserrors.ConditionError{Status: helpNoHlpLibs, Text: "LBR-E-NOHLPLIBS, no default help libraries can be opened"})
	}

	out, width, finish, err := c.helpOutput(opts, toFile)
	if err != nil {
		return err
	}

	s := &help.Session{
		Library: main,
		Open: func(spec string) (help.Library, string, error) {
			l, err := c.openHelpLibrary(spec)
			if err != nil {
				return nil, "", err
			}

			return l.lib, l.name, nil
		},
		Write: func(line string) { fmt.Fprintf(out, "%s\n", line) },
		Read: func(prompt string) (string, error) {
			line, ok := read(prompt)
			if !ok {
				return "", io.EOF
			}

			return line, nil
		},
		Width:   width,
		Options: help.Options{Instructions: opts.Instructions},
		Prompt:  opts.Prompt,
		Users:   users,
		LibList: opts.LibList,
	}

	pager := c.helpPager(opts, out, toFile, s)

	runErr := s.Run(opts.Words)

	if pager != nil {
		pager.end()
	}

	if err := finish(); err != nil {
		return err
	}

	// VMS 7.1's HELP left $STATUS SS$_NORMAL (testdata/help90's part D).
	if runErr == nil && c.SetStatus != nil {
		c.SetStatus(ssNormal)
	}

	return runErr
}

// helpOutput is where HELP writes, at what width, and what to do at its end:
// SYS$OUTPUT, a file of /OUTPUT's (written at the end, a text file as
// CREATE makes one), or nothing.
func (c *Context) helpOutput(opts HelpOptions, toFile bool) (io.Writer, int, func() error, error) {
	none := func() error { return nil }

	switch {
	case opts.NoOutput:
		return io.Discard, helpFileWidth, none, nil
	case opts.OutputGiven && opts.Output != "":
		spec := opts.Output

		loc, err := c.ContainerSession.Locate(spec, false)
		if err != nil {
			return nil, 0, nil, FileFailure(err, spec)
		}

		var b strings.Builder

		finish := func() error {
			text := strings.TrimSuffix(b.String(), "\n")

			var records [][]byte
			if b.Len() > 0 {
				for _, line := range strings.Split(text, "\n") {
					records = append(records, []byte(line))
				}
			}

			if _, err := c.ContainerSession.CreateRecordFile(c.helpListName(loc), rms.TextRecords, records); err != nil {
				return FileFailure(err, spec)
			}

			return nil
		}

		return &b, helpFileWidth, finish, nil
	}

	width := helpFileWidth
	if !toFile {
		width = 80

		if _, w := c.TerminalSize(); w > 0 {
			width = w
		}
	}

	return c.Out, width, none, nil
}

// helpListName is /OUTPUT's file with HELP.LIS's parts where the name
// leaves them out ("/OUTPUT=[]" is HELP.LIS in the default directory).
func (c *Context) helpListName(loc rms.FileLocation) rms.FileLocation {
	if loc.Host {
		return loc
	}

	name := loc.Name
	if at := strings.LastIndexAny(name, ":]>"); at < len(name)-1 {
		return loc.WithDefaultType("LIS")
	}

	loc.Name = name + helpListFile

	return loc
}

// userHelpLibraries are the user libraries /USERLIBRARY chooses, in the
// order HELP searches them, each that can be opened.
func (c *Context) userHelpLibraries(opts HelpOptions) ([]help.NamedLibrary, error) {
	if opts.NoUserLibrary || c.Logicals == nil {
		return nil, nil
	}

	use := map[string]bool{"PROCESS": true, "GROUP": true, "SYSTEM": true}

	if opts.UserLibraries != nil {
		use = map[string]bool{}

		for _, k := range opts.UserLibraries {
			key := strings.ToUpper(strings.TrimSpace(k))

			switch {
			case key == "":
			case strings.HasPrefix("ALL", key):
				use["PROCESS"], use["GROUP"], use["SYSTEM"] = true, true, true
			case strings.HasPrefix("NONE", key):
			case strings.HasPrefix("PROCESS", key):
				use["PROCESS"] = true
			case strings.HasPrefix("GROUP", key):
				use["GROUP"] = true
			case strings.HasPrefix("SYSTEM", key):
				use["SYSTEM"] = true
			default:
				return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
			}
		}
	}

	var libs []help.NamedLibrary

	for _, table := range []struct{ key, name string }{
		{"PROCESS", lnm.ProcessTableName}, {"GROUP", "LNM$GROUP"}, {"SYSTEM", lnm.SystemTableName},
	} {
		if !use[table.key] {
			continue
		}

		for n := 0; ; n++ {
			name := "HLP$LIBRARY"
			if n > 0 {
				name = fmt.Sprintf("HLP$LIBRARY_%d", n)
			}

			e, err := c.Logicals.Translate(table.name, name, lnm.User, lnm.AttrCaseBlind)
			if err != nil || len(e.Equivalences) == 0 {
				break
			}

			// A library that can't be opened is passed over (VMS 7.1's
			// HLP$LIBRARY_1 naming NOSUCH); whether the numbers go on past
			// it is unconfirmed, and govax goes on.
			if l, err := c.openHelpLibrary(e.Equivalences[0].Value); err == nil {
				libs = append(libs, help.NamedLibrary{Name: l.name, Library: l.lib})
			}
		}
	}

	return libs, nil
}

// openHelpLibrary opens the help library spec names, with the default
// SYS$HELP:.HLB: on a volume, or a host file for a host path. One that
// isn't there is OPENIN with RMS's reason, as VMS 7.1's HELP says it:
//
//	%HELP-E-OPENIN, error opening DUA1:[000000]NOSUCH.HLB; as input
//	-RMS-E-FNF, file not found
//
// A file that isn't a help library is OPENIN with LBR's NOTHLPLIB
// (unconfirmed).
func (c *Context) openHelpLibrary(spec string) (helpLibrary, error) {
	s := c.ContainerSession

	loc, err := s.Locate(spec, false)

	if err != nil || !loc.Host {
		found, tried, ferr := s.FindImageFile(spec, helpDefault)
		if ferr != nil {
			return helpLibrary{}, helpOpenFailure(tried, ConditionLine(helpRMSFNF))
		}

		loc = found
	} else {
		loc = loc.WithDefaultType("HLB")
	}

	data, found, err := s.ReadRawFile(loc)
	if err != nil {
		if loc.Host {
			return helpLibrary{}, helpOpenFailure(loc.Name, ConditionLine(helpRMSFNF))
		}

		return helpLibrary{}, FileFailure(err, spec)
	}

	l, err := lbr.Open(data)
	if err != nil || l.Type != lbr.TypeHelp {
		// LBR$_NOTHLPLIB's text, as VMS 7.1 gives it (testdata/help90's
		// LBR90); LBR's messages aren't in vmsdef yet.
		return helpLibrary{}, helpOpenFailure(found.Name, "-LBR-E-NOTHLPLIB, not a HELP library")
	}

	return helpLibrary{lib: help.NewLBRLibrary(l), name: reportFileName(found.Name)}, nil
}

// OpenHelpLibrary opens the help library spec names, as HELP/LIBRARY
// does, or, if it can't be opened, govax's own copy of the library whose
// embedded name (internal/bootdata) is builtin: how the debugger finds
// SYS$HELP:DBG$HELP.HLB.
func (c *Context) OpenHelpLibrary(spec, builtin string) (help.Library, error) {
	l, err := c.openHelpLibrary(spec)
	if err != nil {
		if l, err = builtinHelpLibrary(builtin); err != nil {
			return nil, err
		}
	}

	return l.lib, nil
}

// DebuggerHelpLibrary is the debugger's help library: the file the logical
// name DBG$HELP names, if it's defined (VMS 7.1's debugger read the probe's
// library so, testdata/help90's part F), else SYS$HELP:DBG$HELP.HLB, or
// govax's own copy, whose embedded name is builtin, when that can't be
// opened.
func (c *Context) DebuggerHelpLibrary(builtin string) (help.Library, error) {
	spec := "SYS$HELP:DBG$HELP"

	if c.Logicals != nil {
		if e, err := c.Logicals.Translate("LNM$FILE_DEV", "DBG$HELP", lnm.User, lnm.AttrCaseBlind); err == nil &&
			len(e.Equivalences) > 0 {
			spec = e.Equivalences[0].Value
		}
	}

	return c.OpenHelpLibrary(spec, builtin)
}

// builtinHelpLibrary is govax's own copy of a help library, by its
// embedded name. It's opened afresh each time: a Library keeps the modules
// it has parsed, and HELP may run in several processes' DCL at once.
func builtinHelpLibrary(name string) (helpLibrary, error) {
	data, err := fs.ReadFile(bootdata.FS, name)
	if err != nil {
		return helpLibrary{}, err
	}

	l, err := lbr.Open(data)
	if err != nil {
		return helpLibrary{}, err
	}

	spec := bootdata.HelpLibraryName(name)

	return helpLibrary{lib: help.NewLBRLibrary(l), name: strings.TrimSuffix(spec, ".HLB")}, nil
}

// helpOpenFailure is OPENIN for the library name, with reason, a message's
// line, under it.
func helpOpenFailure(name, reason string) error {
	return vmserrors.ConditionError{
		Status: helpOpenIn,
		Text:   "HELP-E-OPENIN, error opening " + name + " as input\n" + reason,
	}
}

// helpPager is HELP/PAGE at a terminal: VMS 7.1's HELP cleared a video
// terminal's screen before each display (CLEAR_SCREEN, the default; SCROLL
// leaves the screen as it is), and stops after a screenful with paging's
// "Press RETURN to continue" (page.go). Which prompt VMS's HELP shows there
// is unconfirmed: the probe's topics were shorter than a page. VMS also set
// a scrolling region and a character set (testdata/help90's round 2),
// which govax leaves alone.
type helpPager struct {
	c       *Context
	out     io.Writer
	height  int
	used    int
	clear   bool
	stopped bool
}

// helpPager installs a pager on s, or none: /PAGE pages whatever the output,
// /NOPAGE never, and neither pages when HELP writes to a real terminal.
func (c *Context) helpPager(opts HelpOptions, out io.Writer, toFile bool, s *help.Session) *helpPager {
	_, terminal := outputTerminal(out)

	switch {
	case opts.NoPage, out != c.Out, toFile:
		return nil
	case !opts.Paged && !terminal:
		return nil
	}

	height, _ := c.TerminalSize()
	p := &helpPager{c: c, out: out, height: height, clear: true}

	for _, k := range opts.Page {
		switch key := strings.ToUpper(strings.TrimSpace(k)); {
		case key == "":
		case strings.HasPrefix("SCROLL", key):
			p.clear = false
		case strings.HasPrefix("CLEAR_SCREEN", key):
			p.clear = true
		}
	}

	write := s.Write
	s.Write = func(line string) {
		if p.stopped {
			return
		}

		if p.used >= p.height-1 {
			if !c.continuePaging() {
				p.stopped = true

				return
			}

			p.used = 0
		}

		write(line)
		p.used++
	}

	s.BeforeDisplay = func() {
		if p.clear {
			fmt.Fprint(p.out, "\x1b[H\x1b[2J")
		}

		p.used, p.stopped = 0, false
	}

	return p
}

// end puts the terminal's attributes back, as VMS's HELP does at its end.
func (p *helpPager) end() {
	if p.clear {
		fmt.Fprint(p.out, "\x1b[m")
	}
}
