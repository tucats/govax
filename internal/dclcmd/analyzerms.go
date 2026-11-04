package dclcmd

import (
	"errors"
	"path"
	"strings"

	"github.com/tucats/govax/internal/anl"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// ANALYZE/RMS_FILE (docs/PHASE-88 - keyed index files.md, subtask 10d): this file
// finds the files and sends each report where it goes; internal/anl writes it.
//
// VMS's rules, from rounds 1's and 3's logs. A report with /OUTPUT or /NOOUTPUT shows
// %ANLRMS-I-ERRORS (on a file's report, on the terminal and not in the file) and
// leaves %X103B8103; a report with no /OUTPUT, and /SUMMARY and /FDL however they're
// sent, shows nothing more and leaves %X103B8009. The first finding's status replaces either. /FDL with no /OUTPUT goes to NAME.FDL, and of a
// list only the first file has its FDL written; the other reports go to SYS$OUTPUT,
// all of them. A file named but missing is OPENIN, with RMS's status under it.

// anlOpenIn is OPENIN's status, as VMS 7.1's $STATUS showed it.
const anlOpenIn = 0x10B1109A

// AnalyzeRMSOptions is one ANALYZE/RMS_FILE command.
type AnalyzeRMSOptions struct {
	// Files are the files to analyze, and Host an explicit /HOST on them.
	Files []string
	Host  bool
	Mode  anl.RMSMode

	// NoOutput is /NOOUTPUT; HasOutput an /OUTPUT with a file, OutputFile.
	NoOutput   bool
	HasOutput  bool
	OutputFile string

	CommandLine string
}

// AnalyzeRMSFile runs one ANALYZE/RMS_FILE command.
func (c *Context) AnalyzeRMSFile(opts AnalyzeRMSOptions) error {
	s := c.ContainerSession

	toTerminal := opts.HasOutput && strings.EqualFold(opts.OutputFile, "SYS$OUTPUT")
	message := (opts.HasOutput || opts.NoOutput) && opts.Mode != anl.RMSSummary && opts.Mode != anl.RMSFDL

	var (
		output  []byte
		first   string
		status  uint32
		wrote   bool
		options = anl.RMSOptions{Mode: opts.Mode, Command: strings.TrimSpace(opts.CommandLine), Message: message && toTerminal}
	)

	if c.RTL != nil {
		options.OwnerName = c.RTL.IdentifierText
	}

	visit := func(file *rms.AnalyzedFile) error {
		rep := anl.AnalyzeRMSFile(file, options)

		if first == "" {
			first = file.Attr.Name
		}

		if status == 0 {
			status = rep.Status
		}

		if message && !toTerminal {
			c.Printf("%s\n", anl.ErrorsMessage(file.Attr.Name, rep.Errors))
		}

		switch {
		case opts.NoOutput:
		case opts.HasOutput && !toTerminal, opts.Mode == anl.RMSFDL && !toTerminal:
			if opts.Mode != anl.RMSFDL || !wrote {
				output = append(output, rep.Text...)
			}
		default:
			c.Printf("%s", rep.Text)
		}

		wrote = true

		return nil
	}

	for _, spec := range rms.TemporaryDefaults(opts.Files) {
		if err := s.AnalyzeSearch(spec, opts.Host, visit); err != nil {
			return analyzeOpenFailure(err, spec)
		}
	}

	if len(output) > 0 {
		name := opts.OutputFile
		switch {
		case !opts.HasOutput:
			name = reportFileName(first) + ".FDL"
		case opts.Mode == anl.RMSFDL && strings.HasPrefix(name, "."):
			name = reportFileName(first) + name
		case strings.HasPrefix(name, "."):
			name = "ANALYZE" + name
		}

		def := "ANL"
		if opts.Mode == anl.RMSFDL {
			def = "FDL"
		}

		if err := c.writeAnalysis(name, rms.FileLocation{}, def, output); err != nil {
			return err
		}
	}

	if status == 0 {
		status = anl.RMSStatusWritten
		if message {
			status = anl.RMSStatusReported
		}
	}

	if c.SetStatus != nil {
		c.SetStatus(status)
	}

	return nil
}

// reportFileName is a file's name alone, from its whole specification.
func reportFileName(spec string) string {
	if at := strings.LastIndexAny(spec, "]>:"); at >= 0 {
		spec = spec[at+1:]
	}

	if at := strings.IndexAny(spec, ";"); at >= 0 {
		spec = spec[:at]
	}

	return strings.TrimSuffix(spec, path.Ext(spec))
}

// analyzeOpenFailure is OPENIN, for a file that can't be found.
func analyzeOpenFailure(err error, spec string) error {
	var missing *rms.DCLFileError
	if errors.As(err, &missing) {
		return vmserrors.ConditionError{
			Status: anlOpenIn,
			Text:   "ANALYZE-E-OPENIN, error opening " + missing.Name + " as input\n" + conditionLineWith(missing.Status, missing.STV),
		}
	}

	return FileFailure(err, spec)
}
