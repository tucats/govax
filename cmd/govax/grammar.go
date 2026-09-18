package main

import (
	"os"
	"strings"
	"time"

	"github.com/tucats/gopackages/app-cli/cli"
)

var (
	instructionLimit int
	timeLimit        time.Duration
	paths            []string
	stats            bool
)

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
		LongName:             "run",
		Description:          "Run a VAX/VMS executable",
		OptionType:           cli.Subcommand,
		Action:               runCmd,
		ParametersExpected:   1,
		ParameterDescription: "filename",
	},
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
	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{})
}

func asmCmd(c *cli.Context) error {
	return doCmd(c, "asm")
}

func runCmd(c *cli.Context) error {
	return doCmd(c, "run")
}

func doCmd(c *cli.Context, cmd string) error {
	args := []string{cmd}

	for _, arg := range c.FindGlobal().Parameters {
		args = append(args, arg)
	}

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, args)
}
