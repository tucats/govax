package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tucats/govax/internal/anl"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// ANALYZE/OBJECT (docs/PHASE-38.md) describes object files as VMS's
// ANALYZE/OBJECT does: internal/anl writes the report, and this file finds
// the files and sends the report where it goes.
//
// Every file can be a host file or a file on a mounted ODS-2 volume, by
// the rules MACRO, LINK, and LIBRARY use (rms.Session.Locate), with the
// default type OBJ; a file's bare name is found beside the file before it.
// The report goes to the console, or with /OUTPUT to a text file, by
// default NAME.ANL beside the first input. Each input file's report is
// paged on its own, one after another.

// AnalyzeOptions is one ANALYZE/OBJECT command.
type AnalyzeOptions struct {
	// Files are the files to analyze, and Host an explicit /HOST on
	// them.
	Files []string
	Host  bool

	// Output is /OUTPUT, to OutputFile, or to NAME.ANL beside the first
	// input when OutputFile is empty.
	Output     bool
	OutputFile string

	// Select limits the records shown (/MHD, /GSD, /TIR, /TBT, /DBG,
	// /LNK, /EOM); empty shows every record.
	Select []obj.RecordType

	// CommandLine is the command as typed, which closes each report.
	CommandLine string
}

// AnalyzeObject runs one ANALYZE/OBJECT command. A file that can't be
// read stops the command; a file the analysis finds errors in doesn't,
// and the command then ends with CLI_ANALYZEERRORS once every file's
// report is written.
func (c *Console) AnalyzeObject(opts AnalyzeOptions) error {
	s := c.ContainerSession

	var (
		report   bytes.Buffer
		first    rms.FileLocation
		prev     rms.FileLocation
		problems error
	)

	for i, name := range opts.Files {
		var (
			loc rms.FileLocation
			err error
		)

		if i == 0 {
			loc, err = s.Locate(name, opts.Host)
		} else {
			loc, err = s.LocateRelated(name, opts.Host, prev)
		}

		if err != nil {
			return fileFailure(err, name)
		}

		loc = withDefaultType(loc, "OBJ")

		records, found, err := s.ReadRecordFile(loc, rms.VariableRecords)
		if err != nil {
			return fileFailure(err, loc.Name)
		}

		if i == 0 {
			first = found
		}

		prev = found

		rep := anl.AnalyzeObject(records, anl.ObjectOptions{Select: opts.Select})

		p := anl.NewPager(&report, anl.TitleObject, analyzedFileName(found), strings.TrimSpace(opts.CommandLine))
		if err := p.Write(rep.Lines); err != nil {
			return vmserrors.Wrap(vmserrors.CLI_ANALYZE, err, found.Name)
		}

		if err := p.Close(); err != nil {
			return vmserrors.Wrap(vmserrors.CLI_ANALYZE, err, found.Name)
		}

		if rep.Errors > 0 && problems == nil {
			problems = vmserrors.New(vmserrors.CLI_ANALYZEERRORS, found.Name)
		}

		if !opts.Output {
			c.Printf("%s", latin1ToUTF8(report.Bytes()))
			report.Reset()
		}
	}

	if opts.Output {
		if err := c.writeAnalysis(opts.OutputFile, first, report.Bytes()); err != nil {
			return err
		}
	}

	return problems
}

// writeAnalysis writes a report to the output file name (or NAME.ANL
// beside input), one record per line.
func (c *Console) writeAnalysis(name string, input rms.FileLocation, text []byte) error {
	s := c.ContainerSession

	out, err := outputLocation(s, name, input, "ANL")
	if err != nil {
		return fileFailure(err, name)
	}

	out = withDefaultType(out, "ANL")

	lines := bytes.Split(bytes.TrimSuffix(text, []byte("\n")), []byte("\n"))

	if _, err := s.CreateRecordFile(out, rms.TextRecords, lines); err != nil {
		return objectFailureAs(vmserrors.CLI_ANALYZE, err, out.Name)
	}

	return nil
}

// analyzedFileName is the file specification a report's page headers
// show: a volume file's full specification, as the volume found it, or a
// host file's absolute path.
func analyzedFileName(found rms.FileLocation) string {
	if found.Host {
		if abs, err := filepath.Abs(found.Name); err == nil {
			return abs
		}
	}

	return found.Name
}

// latin1ToUTF8 converts a report's ISO Latin-1 bytes (a hex dump shows
// DEC Multinational characters as themselves) to UTF-8 for the console.
// A report written to a file keeps its bytes, as VMS's does.
func latin1ToUTF8(b []byte) string {
	var sb strings.Builder

	for _, c := range b {
		if c < utf8.RuneSelf {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(rune(c))
		}
	}

	return sb.String()
}

// analyzeRecordQualifiers are ANALYZE/OBJECT's record-type qualifiers and
// the record types each selects. /EOM selects both end of module records,
// and /GSD, /TIR, ... their own types; /MHD selects every header record,
// whose first is the module header.
var analyzeRecordQualifiers = []struct {
	name  string
	types []obj.RecordType
}{
	{"MHD", []obj.RecordType{obj.RecHDR}},
	{"GSD", []obj.RecordType{obj.RecGSD}},
	{"TIR", []obj.RecordType{obj.RecTIR}},
	{"TBT", []obj.RecordType{obj.RecTBT}},
	{"DBG", []obj.RecordType{obj.RecDBG}},
	{"LNK", []obj.RecordType{obj.RecLNK}},
	{"EOM", []obj.RecordType{obj.RecEOM, obj.RecEOMW}},
}
