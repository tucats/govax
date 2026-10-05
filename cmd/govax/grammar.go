package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/vmserrors"
)

var (
	instructionLimit int
	timeLimit        time.Duration
	paths            []string
	stats            bool

	// mountRequests are the --mount and --mount-write options, in order,
	// which run mounts before anything else.
	mountRequests []mountRequest

	// macro holds the macro subcommand's options.
	macro macroFlags

	// link holds the link subcommand's options.
	link linkFlags

	// library holds the library subcommand's options.
	library libraryFlags
)

// libraryFlags are the library subcommand's options, which become the
// LIBRARY command's qualifiers.
type libraryFlags struct {
	create, insert, replace bool     // --create, --insert, --replace
	delete, extract         []string // --delete, --extract: module names
	output                  string   // --output
	list                    bool     // --list, or --list-file
	listFile                string   // --list-file
	full, names             bool     // --full, --names
	macro, object           bool     // --macro, --object
	noSqueeze               bool     // --no-squeeze
	selective               bool     // --selective-search
	log                     bool     // --log
}

// macroFlags are the macro subcommand's options, which become the MACRO
// command's qualifiers.
type macroFlags struct {
	object    string   // --object
	noObject  bool     // --no-object
	list      bool     // --list, or --list-file
	listFile  string   // --list-file
	show      []string // --show: MACRO's /SHOW=
	noShow    []string // --no-show: MACRO's /NOSHOW=
	xref      bool     // --cross-reference, or --cross-reference-kinds
	xrefKinds []string // --cross-reference-kinds: MACRO's /CROSS_REFERENCE=
	enable    []string // --enable: MACRO's /ENABLE=
	disable   []string // --disable: MACRO's /DISABLE=
	debug     []string // --debug: MACRO's /DEBUG=
	noDebug   bool     // --no-debug: MACRO's /NODEBUG
	libraries []string // --library, repeatable: MACRO's /LIBRARY=
}

// linkFlags are the link subcommand's options, which become the LINK
// command's qualifiers.
type linkFlags struct {
	executable   string // --executable
	noExecutable bool   // --no-executable
	noTraceback  bool   // --no-traceback
	noSysLib     bool   // --no-syslib
	mapWanted    bool   // --map, or --map-file
	mapFile      string // --map-file
	brief        bool   // --brief
	// libraries and options are --library and --options, each of which
	// can be repeated: files LINK gets with /LIBRARY and /OPTIONS.
	libraries, options []string
}

// mountRequest is one --mount DEVICE=container option.
type mountRequest struct {
	device, path string
	write        bool
}

var grammar = []cli.Option{
	{
		LongName:    "stats",
		ShortName:   "s",
		Description: "Display execution stats when done",
		OptionType:  cli.BooleanType,
		Action:      setStats,
	},
	{
		LongName:    "path",
		ShortName:   "p",
		Description: "Search path for file names",
		OptionType:  cli.StringListType,
		Action:      setPaths,
	},
	{
		LongName:    "instruction-limit",
		ShortName:   "i",
		Aliases:     []string{"instructions"},
		Description: "Maximum number of instructions to execute",
		OptionType:  cli.IntType,
		Action:      setInstructionLimit,
	},
	{
		LongName:    "time-limit",
		ShortName:   "t",
		Aliases:     []string{"time", "duration"},
		Description: "Maximum elapsed time to execute",
		OptionType:  cli.StringType,
		Action:      setTimeLimit,
	},
	{
		LongName:             "mount",
		ShortName:            "m",
		Description:          "Mount a container read-only before running (DEVICE=container; repeatable)",
		ParameterDescription: "device=container",
		OptionType:           cli.StringType,
		Action:               func(c *cli.Context) error { return addMount(c, "mount", false) },
	},
	{
		LongName:             "mount-write",
		Description:          "Mount a container for writing before running (DEVICE=container; repeatable)",
		ParameterDescription: "device=container",
		OptionType:           cli.StringType,
		Action:               func(c *cli.Context) error { return addMount(c, "mount-write", true) },
	},
	{
		LongName:    "console",
		Description: "Execute VAX console commands",
		OptionType:  cli.Subcommand,
		Action:      consoleCmd,
		DefaultVerb: true,
	},
	{
		LongName:             "asm",
		Aliases:              []string{"assemble"},
		Description:          "Assemble VAX source file",
		OptionType:           cli.Subcommand,
		Action:               asmCmd,
		ParametersExpected:   1,
		ParameterDescription: "filename",
	},
	{
		LongName:             "macro",
		Description:          "Assemble a MACRO-32 source file into an object module",
		OptionType:           cli.Subcommand,
		Action:               macroCmd,
		ParametersExpected:   1,
		ParameterDescription: "source",
		Value:                macroGrammar,
	},
	{
		LongName:             "link",
		Description:          "Link object modules into a VMS executable image",
		OptionType:           cli.Subcommand,
		Action:               linkCmd,
		ParametersExpected:   -99,
		ParameterDescription: "object...",
		Value:                linkGrammar,
	},
	{
		LongName:             "analyze",
		Description:          "Analyze object files or object library modules, as ANALYZE/OBJECT does",
		OptionType:           cli.Subcommand,
		Action:               analyzeCmd,
		ParametersExpected:   -99,
		ParameterDescription: "file...",
		Value:                analyzeGrammar,
	},
	{
		LongName:             "library",
		Description:          "Create, change, extract from, or list a macro or object library",
		OptionType:           cli.Subcommand,
		Action:               libraryCmd,
		ParametersExpected:   -99,
		ParameterDescription: "library [input...]",
		Value:                libraryGrammar,
	},
	{
		LongName:             "run",
		Description:          "Run a VAX/VMS executable",
		OptionType:           cli.Subcommand,
		Action:               runCmd,
		ParametersExpected:   -99,
		ParameterDescription: "filename [text...]",
	},
}

// macroGrammar is the macro subcommand's own options.
var macroGrammar = []cli.Option{
	{
		LongName:    "object",
		ShortName:   "o",
		Description: "Object file name (default: the source's, with type .obj)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			macro.object, _ = c.String("object")

			return nil
		},
	},
	{
		LongName:    "no-object",
		Description: "Assemble and report errors without writing an object file",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			macro.noObject = true

			return nil
		},
	},
	{
		LongName:    "list",
		Description: "Write a listing file (the source's name, with type .lis)",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			macro.list = true

			return nil
		},
	},
	{
		LongName:    "list-file",
		Description: "Write a listing file with this name",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			macro.list = true
			macro.listFile, _ = c.String("list-file")

			return nil
		},
	},
	{
		LongName:    "show",
		Description: "Listing options to turn on, separated by commas (EXPANSIONS, BINARY, ...)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("show")
			macro.show = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "no-show",
		Description: "Listing options to turn off, separated by commas (CALLS, CONDITIONALS, ...)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("no-show")
			macro.noShow = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "cross-reference",
		Description: "End the listing with a cross reference of symbols and macros",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			macro.xref = true

			return nil
		},
	},
	{
		LongName:    "cross-reference-kinds",
		Description: "What the cross reference lists, separated by commas (SYMBOLS, MACROS, OPCODES, DIRECTIVES, REGISTERS, ALL)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("cross-reference-kinds")
			macro.xref = true
			macro.xrefKinds = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "enable",
		Description: "Assembler functions to turn on, separated by commas (TRACEBACK, DEBUG, SUPPRESSION, ...)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("enable")
			macro.enable = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "disable",
		Description: "Assembler functions to turn off, separated by commas (TRACEBACK, GLOBAL, ...)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("disable")
			macro.disable = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "debug",
		Description: "Debugger and traceback records to write, separated by commas (ALL, SYMBOLS, TRACEBACK, NONE)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			list, _ := c.String("debug")
			macro.debug = strings.Split(list, ",")

			return nil
		},
	},
	{
		LongName:    "no-debug",
		Description: "Write no debugger or traceback records",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			macro.noDebug = true

			return nil
		},
	},
	{
		LongName:    "library",
		Description: "A macro library to search ahead of STARLET.MLB (repeatable)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			name, _ := c.String("library")
			macro.libraries = append(macro.libraries, name)

			return nil
		},
	},
}

// linkGrammar is the link subcommand's own options.
var linkGrammar = []cli.Option{
	{
		LongName:    "executable",
		ShortName:   "e",
		Description: "Image file name (default: the first object's, with type .exe)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			link.executable, _ = c.String("executable")

			return nil
		},
	},
	{
		LongName:    "no-executable",
		Description: "Link and report errors without writing an image",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.noExecutable = true

			return nil
		},
	},
	{
		LongName:    "no-traceback",
		Description: "Don't start the image through SYS$IMGSTA",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.noTraceback = true

			return nil
		},
	},
	{
		LongName:    "no-syslib",
		Description: "Don't search IMAGELIB.OLB and STARLET.OLB",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.noSysLib = true

			return nil
		},
	},
	{
		LongName:    "map",
		Description: "Write a link map (default: the first object's name, with type .map)",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.mapWanted = true

			return nil
		},
	},
	{
		LongName:    "map-file",
		Description: "Write a link map to this file",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			link.mapWanted = true
			link.mapFile, _ = c.String("map-file")

			return nil
		},
	},
	{
		LongName:    "library",
		Description: "An object or shareable image library to search (repeatable)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			name, _ := c.String("library")
			link.libraries = append(link.libraries, name)

			return nil
		},
	},
	{
		LongName:    "options",
		Description: "A LINK options file (repeatable)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			name, _ := c.String("options")
			link.options = append(link.options, name)

			return nil
		},
	},
	{
		LongName:    "brief",
		Description: "Write a brief map: the object modules and the image synopsis",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.mapWanted, link.brief = true, true

			return nil
		},
	},
}

// libraryGrammar is the library subcommand's own options.
var libraryGrammar = []cli.Option{
	libraryBool("create", "Create a new library", &library.create),
	libraryBool("insert", "Insert the input files' modules; a module already there is a warning", &library.insert),
	libraryBool("replace", "Replace modules with the input files' (the default with input files)", &library.replace),
	libraryList("delete", "Delete these modules (comma-separated; * and % wildcards)", &library.delete),
	libraryList("extract", "Extract these modules (comma-separated; * and % wildcards)", &library.extract),
	{
		LongName:    "output",
		Description: "The file for extracted modules (default: the library's name, with type .obj or .mar)",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			library.output, _ = c.String("output")

			return nil
		},
	},
	libraryBool("list", "List the library", &library.list),
	{
		LongName:    "list-file",
		Description: "List the library to this file",
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			library.list = true
			library.listFile, _ = c.String("list-file")

			return nil
		},
	},
	libraryBool("full", "List each module's ident, insertion time, and symbol count", &library.full),
	libraryBool("names", "List each object module's global symbols", &library.names),
	libraryBool("macro", "A macro library (.mlb)", &library.macro),
	libraryBool("object", "An object library (.olb, the default)", &library.object),
	libraryBool("no-squeeze", "Keep macros' comments and trailing blanks", &library.noSqueeze),
	libraryBool("selective-search", "Mark inserted object modules for selective search", &library.selective),
	libraryBool("log", "Report each module inserted, replaced, or deleted", &library.log),
}

// libraryBool is a library subcommand option that sets a flag.
func libraryBool(name, description string, flag *bool) cli.Option {
	return cli.Option{
		LongName:    name,
		Description: description,
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			*flag = true

			return nil
		},
	}
}

// libraryList is a library subcommand option holding comma-separated
// module names; it can also be repeated.
func libraryList(name, description string, list *[]string) cli.Option {
	return cli.Option{
		LongName:    name,
		Description: description,
		OptionType:  cli.StringType,
		Action: func(c *cli.Context) error {
			text, _ := c.String(name)
			*list = append(*list, strings.Split(text, ",")...)

			return nil
		},
	}
}

// addMount records one --mount or --mount-write option. Each use of the
// option calls its action, so the options can be repeated.
func addMount(c *cli.Context, name string, write bool) error {
	text, _ := c.String(name)

	device, path, ok := strings.Cut(text, "=")
	if !ok || device == "" || path == "" {
		return fmt.Errorf("--%s %q: expected DEVICE=container", name, text)
	}

	mountRequests = append(mountRequests, mountRequest{device: device, path: path, write: write})

	return nil
}

func setStats(c *cli.Context) error {
	stats = true

	return nil
}

func setInstructionLimit(c *cli.Context) error {
	instructionLimit, _ = c.Integer("instruction-limit")

	return nil
}

func setTimeLimit(c *cli.Context) error {
	text, _ := c.String("time-limit")

	if d, err := time.ParseDuration(text); err != nil {
		return err
	} else {
		timeLimit = d
	}

	return nil
}

func setPaths(c *cli.Context) error {
	list, _ := c.String("paths")
	paths = strings.Split(list, ",")

	return nil
}

func consoleCmd(c *cli.Context) error {
	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{})
}

func asmCmd(c *cli.Context) error {
	return doCmd(c, "asm")
}

// macroCmd runs the console's MACRO command. The file names are quoted,
// so DCL keeps their case and a host path's "/" isn't read as a
// qualifier.
func macroCmd(c *cli.Context) error {
	params := c.FindGlobal().Parameters
	if len(params) != 1 {
		return fmt.Errorf("macro: expected one source file")
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{macroCommand(params[0], macro)})
}

// macroCommand is the console MACRO command for the macro subcommand's
// source file and options.
func macroCommand(source string, f macroFlags) string {
	command := "MACRO " + dclQuote(source)

	switch {
	case f.noObject:
		command += "/NOOBJECT"
	case f.object != "":
		command += "/OBJECT=" + dclQuote(f.object)
	}

	switch {
	case f.listFile != "":
		command += "/LIST=" + dclQuote(f.listFile)
	case f.list:
		command += "/LIST"
	}

	// DCL takes one of /SHOW and /NOSHOW; --show wins.
	switch {
	case len(f.show) > 0:
		command += "/SHOW=(" + strings.Join(f.show, ",") + ")"
	case len(f.noShow) > 0:
		command += "/NOSHOW=(" + strings.Join(f.noShow, ",") + ")"
	}

	switch {
	case len(f.xrefKinds) > 0:
		command += "/CROSS_REFERENCE=(" + strings.Join(f.xrefKinds, ",") + ")"
	case f.xref:
		command += "/CROSS_REFERENCE"
	}

	if len(f.enable) > 0 {
		command += "/ENABLE=(" + strings.Join(f.enable, ",") + ")"
	}

	if len(f.disable) > 0 {
		command += "/DISABLE=(" + strings.Join(f.disable, ",") + ")"
	}

	// DCL takes one of /DEBUG and /NODEBUG; --debug wins.
	switch {
	case len(f.debug) > 0:
		command += "/DEBUG=(" + strings.Join(f.debug, ",") + ")"
	case f.noDebug:
		command += "/NODEBUG"
	}

	if len(f.libraries) > 0 {
		quoted := make([]string, len(f.libraries))
		for i, name := range f.libraries {
			quoted[i] = dclQuote(name)
		}

		command += "/LIBRARY=(" + strings.Join(quoted, ",") + ")"
	}

	return command
}

// dclQuote quotes a file name for a DCL command line.
func dclQuote(s string) string {
	return `"` + s + `"`
}

// linkCmd runs the console's LINK command for the objects given.
func linkCmd(c *cli.Context) error {
	objects := c.FindGlobal().Parameters
	if len(objects) == 0 {
		return fmt.Errorf("link: expected one or more object files")
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{linkCommand(objects, link)})
}

// linkCommand is the console LINK command for the link subcommand's
// objects and options, each file name quoted as macroCommand quotes them.
func linkCommand(objects []string, f linkFlags) string {
	quoted := make([]string, len(objects))
	for i, o := range objects {
		quoted[i] = dclQuote(o)
	}

	for _, name := range f.libraries {
		quoted = append(quoted, dclQuote(name)+"/LIBRARY")
	}

	for _, name := range f.options {
		quoted = append(quoted, dclQuote(name)+"/OPTIONS")
	}

	command := "LINK " + strings.Join(quoted, ",")

	switch {
	case f.noExecutable:
		command += "/NOEXECUTABLE"
	case f.executable != "":
		command += "/EXECUTABLE=" + dclQuote(f.executable)
	}

	if f.noTraceback {
		command += "/NOTRACEBACK"
	}

	if f.noSysLib {
		command += "/NOSYSLIB"
	}

	switch {
	case f.mapFile != "":
		command += "/MAP=" + dclQuote(f.mapFile)
	case f.mapWanted:
		command += "/MAP"
	}

	if f.brief {
		command += "/BRIEF"
	}

	return command
}

// libraryCmd runs the console's LIBRARY command for the library and input
// files given.
func libraryCmd(c *cli.Context) error {
	params := c.FindGlobal().Parameters
	if len(params) == 0 {
		return fmt.Errorf("library: expected a library file")
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{libraryCommand(params[0], params[1:], library)})
}

// libraryCommand is the console LIBRARY command for the library
// subcommand's files and options. File and module names are quoted, so
// DCL keeps their case.
func libraryCommand(lib string, inputs []string, f libraryFlags) string {
	command := "LIBRARY " + dclQuote(lib)

	if len(inputs) > 0 {
		quoted := make([]string, len(inputs))
		for i, name := range inputs {
			quoted[i] = dclQuote(name)
		}

		command += " " + strings.Join(quoted, ",")
	}

	for _, q := range []struct {
		set  bool
		name string
	}{
		{f.create, "/CREATE"}, {f.insert, "/INSERT"}, {f.replace, "/REPLACE"},
		{f.macro, "/MACRO"}, {f.object, "/OBJECT"}, {f.noSqueeze, "/NOSQUEEZE"},
		{f.selective, "/SELECTIVE_SEARCH"}, {f.log, "/LOG"},
	} {
		if q.set {
			command += q.name
		}
	}

	modules := func(names []string) string {
		quoted := make([]string, len(names))
		for i, n := range names {
			quoted[i] = dclQuote(n)
		}

		return "(" + strings.Join(quoted, ",") + ")"
	}

	if len(f.delete) > 0 {
		command += "/DELETE=" + modules(f.delete)
	}

	if len(f.extract) > 0 {
		command += "/EXTRACT=" + modules(f.extract)
	}

	if f.output != "" {
		command += "/OUTPUT=" + dclQuote(f.output)
	}

	switch {
	case f.listFile != "":
		command += "/LIST=" + dclQuote(f.listFile)
	case f.list:
		command += "/LIST"
	}

	if f.full {
		command += "/FULL"
	}

	if f.names {
		command += "/NAMES"
	}

	return command
}

// runCmd runs the console's RUN command for an image. Any parameters after
// the image's file name are its command text, which it reads with
// LIB$GET_FOREIGN, as a foreign command's image does.
func runCmd(c *cli.Context) error {
	params := c.FindGlobal().Parameters
	if len(params) == 0 {
		return vmserrors.New(vmserrors.CLI_NOFILE)
	}

	console.RunCommandLine = strings.Join(params[1:], " ")
	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{"run", dclQuote(params[0])})
}

// doCmd runs the console command cmd on the subcommand's parameters, each
// quoted so DCL keeps a host file name's case and its "/" isn't read as a
// qualifier.
func doCmd(c *cli.Context, cmd string) error {
	args := []string{cmd}

	for _, arg := range c.FindGlobal().Parameters {
		args = append(args, dclQuote(arg))
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, args)
}

// See if there is a "vax.path" config item. If so, add it to the
// provided path list.
func loadConfigPaths(paths []string) []string {
	text := settings.Get("vax.path")
	if text == "" {
		return paths
	}

	// You can specify multiple path names by quoting them and separating
	// them by commas.
	delim := ","

	// Split the string and evaluate each one. If the item is quoted, then
	// strip away the quotes.
	items := strings.Split(text, delim)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, "\"") {
			if unquoted, err := strconv.Unquote(strings.TrimSpace(item)); err == nil {
				item = unquoted
			}
		}

		// Aadd to the path list.
		paths = append(paths, item)
	}

	return paths
}
