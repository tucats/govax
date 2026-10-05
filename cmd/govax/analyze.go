package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
)

// analyzeFlags are the analyze subcommand's options, which become the
// ANALYZE/OBJECT command's qualifiers (docs/PHASE-38.md), or with --image
// ANALYZE/IMAGE's (docs/PHASE-40.md).
type analyzeFlags struct {
	output     bool     // --output, or --output-file
	outputFile string   // --output-file
	records    []string // --mhd, --gsd, ...: the record types to show
	include    []string // --include: library modules
	image      bool     // --image: ANALYZE/IMAGE
	header     bool     // --header: the image header only
	fixups     bool     // --fixup-section: the image's fixup section too
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
			LongName:    "image",
			Description: "Analyze image files (ANALYZE/IMAGE) rather than object files",
			OptionType:  cli.BooleanType,
			Action: func(c *cli.Context) error {
				analyze.image = true

				return nil
			},
		},
		{
			LongName:    "header",
			Description: "With --image, show only the image header",
			OptionType:  cli.BooleanType,
			Action: func(c *cli.Context) error {
				analyze.header = true

				return nil
			},
		},
		{
			LongName:    "fixup-section",
			Description: "With --image, show the fixup section with the header",
			OptionType:  cli.BooleanType,
			Action: func(c *cli.Context) error {
				analyze.fixups = true

				return nil
			},
		},
		{
			LongName:    "output",
			Description: "Write the report to NAME.ANL (NAME.ANI with --image) beside the first file",
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

// analyzeCmd runs the analyze subcommand: ANALYZE/OBJECT of the files,
// or with --image ANALYZE/IMAGE.
func analyzeCmd(c *cli.Context) error {
	params := c.FindGlobal().Parameters
	if len(params) == 0 {
		if analyze.image {
			return fmt.Errorf("analyze: expected an image file")
		}

		return fmt.Errorf("analyze: expected an object file")
	}

	command, err := analyzeCommand(params, analyze)
	if err != nil {
		return err
	}

	paths = loadConfigPaths(paths)

	return run(paths, instructionLimit, timeLimit, os.Stdout, nil, []string{command})
}

// analyzeCommand is the console ANALYZE/OBJECT or ANALYZE/IMAGE command
// for the analyze subcommand's files and options. File and module names
// are quoted, so DCL keeps their case. An option of the other kind of
// analysis is an error.
func analyzeCommand(files []string, f analyzeFlags) (string, error) {
	if f.image && (len(f.records) > 0 || len(f.include) > 0) {
		return "", fmt.Errorf("analyze: --include and the record-type options are for object files, not --image")
	}

	if !f.image && (f.header || f.fixups) {
		return "", fmt.Errorf("analyze: --header and --fixup-section need --image")
	}

	command := "ANALYZE/OBJECT"
	if f.image {
		command = "ANALYZE/IMAGE"
	}

	if f.header {
		command += "/HEADER"
	}

	if f.fixups {
		command += "/FIXUP_SECTION"
	}

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

	return command + " " + strings.Join(quoted, ","), nil
}
