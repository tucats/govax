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
)

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
