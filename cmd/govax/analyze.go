package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
)

// analyzeFlags are the analyze subcommand's options, which become the
// ANALYZE/OBJECT command's qualifiers (docs/PHASE-38.md).
type analyzeFlags struct {
	output     bool     // --output, or --output-file
	outputFile string   // --output-file
	records    []string // --mhd, --gsd, ...: the record types to show
	include    []string // --include: library modules
}

// analyze holds the analyze subcommand's options.
var analyze analyzeFlags

// analyzeRecordTypes are the record-type options, each ANALYZE/OBJECT's
// qualifier of the same name.
var analyzeRecordTypes = []struct{ name, description string }{
	{"mhd", "Show the header records"},
	{"gsd", "Show the global symbol directory records"},
	{"tir", "Show the text information and relocation records"},
	{"tbt", "Show the traceback records"},
	{"dbg", "Show the debugger records"},
	{"lnk", "Show the link option records"},
	{"eom", "Show the end of module records"},
}

// analyzeGrammar is the analyze subcommand's own options.
var analyzeGrammar = func() []cli.Option {
	opts := []cli.Option{
		{
			LongName:    "output",
			Description: "Write the report to NAME.ANL beside the first file",
			OptionType:  cli.BooleanType,
			Action: func(c *cli.Context) error {
				analyze.output = true

				return nil
			},
		},
		{
			LongName:    "output-file",
			Description: "Write the report to this file",
			OptionType:  cli.StringType,
			Action: func(c *cli.Context) error {
				analyze.output = true
				analyze.outputFile, _ = c.String("output-file")

				return nil
			},
		},
		{
			LongName:    "include",
			Description: "Analyze these modules of an object library (comma-separated; * and % wildcards)",
			OptionType:  cli.StringType,
			Action: func(c *cli.Context) error {
				text, _ := c.String("include")
				analyze.include = append(analyze.include, strings.Split(text, ",")...)

				return nil
			},
		},
	}

	for _, r := range analyzeRecordTypes {
		name := r.name
		opts = append(opts, cli.Option{
			LongName:    name,
			Description: r.description,
			OptionType:  cli.BooleanType,
			Action: func(c *cli.Context) error {
				analyze.records = append(analyze.records, name)

				return nil
			},
		})
	}

	return opts
}()

// analyzeCmd runs the analyze subcommand: ANALYZE/OBJECT of the files.
func analyzeCmd(c *cli.Context) error {
	params := c.FindGlobal().Parameters
	if len(params) == 0 {
		return fmt.Errorf("analyze: expected an object file")
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{analyzeCommand(params, analyze)})
}

// analyzeCommand is the console ANALYZE/OBJECT command for the analyze
// subcommand's files and options. File and module names are quoted, so
// DCL keeps their case.
func analyzeCommand(files []string, f analyzeFlags) string {
	command := "ANALYZE/OBJECT"

	for _, r := range f.records {
		command += "/" + strings.ToUpper(r)
	}

	switch {
	case f.outputFile != "":
		command += "/OUTPUT=" + dclQuote(f.outputFile)
	case f.output:
		command += "/OUTPUT"
	}

	if len(f.include) > 0 {
		quoted := make([]string, len(f.include))
		for i, n := range f.include {
			quoted[i] = dclQuote(n)
		}

		command += "/INCLUDE=(" + strings.Join(quoted, ",") + ")"
	}

	quoted := make([]string, len(files))
	for i, name := range files {
		quoted[i] = dclQuote(name)
	}

	return command + " " + strings.Join(quoted, ",")
}
