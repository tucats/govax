package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/settings"
)

var (
	instructionLimit int
	timeLimit        time.Duration
	paths            []string
	stats            bool

	// mountRequests are the --mount and --mount-write options, in order,
	// which run mounts before anything else.
	mountRequests []mountRequest

	// macroObject and macroNoObject are the macro subcommand's --object
	// and --no-object options.
	macroObject   string
	macroNoObject bool

	// link holds the link subcommand's options.
	link linkFlags
)

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
		LongName:             "run",
		Description:          "Run a VAX/VMS executable",
		OptionType:           cli.Subcommand,
		Action:               runCmd,
		ParametersExpected:   1,
		ParameterDescription: "filename",
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
			macroObject, _ = c.String("object")

			return nil
		},
	},
	{
		LongName:    "no-object",
		Description: "Assemble and report errors without writing an object file",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			macroNoObject = true

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
		LongName:    "brief",
		Description: "Write a brief map: the object modules and the image synopsis",
		OptionType:  cli.BooleanType,
		Action: func(c *cli.Context) error {
			link.mapWanted, link.brief = true, true

			return nil
		},
	},
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

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{macroCommand(params[0], macroObject, macroNoObject)})
}

// macroCommand is the console MACRO command for the macro subcommand's
// source file and options.
func macroCommand(source, object string, noObject bool) string {
	command := "MACRO " + dclQuote(source)

	switch {
	case noObject:
		command += "/NOOBJECT"
	case object != "":
		command += "/OBJECT=" + dclQuote(object)
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

func runCmd(c *cli.Context) error {
	return doCmd(c, "run")
}

func doCmd(c *cli.Context, cmd string) error {
	args := []string{cmd}

	for _, arg := range c.FindGlobal().Parameters {
		args = append(args, arg)
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
