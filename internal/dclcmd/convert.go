package dclcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/fdl"
	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// CONVERT (docs/planning/PHASE-88 - keyed index files.md, subtask 10c; the Record
// Management Utilities manual's chapter 2): the records of up to 10 files copied
// to one, changing their organization and format. The output is made from an FDL
// file, or with the first input's attributes, unless it's to be an existing one.
// An indexed output is sorted by its primary key and loaded fast, by building
// its tree (internal/isam's Load), or by $PUTs with /NOFAST_LOAD.
//
// A record that can't go in is an exception, which isn't a failure: a message,
// the record copied to the exceptions file, and the conversion goes on, ending
// in status 1 (round 3 of the probe, testdata/idx88's IDX88C.COM, sections C6 to
// C10). /EXIT ends it at the first with FATALEXC. Two facility names share a
// facility number: CONVERT's own messages are CONVERT's, and the shared ones
// (OPENIN and so on) CONV's.

// CONVERT's status values, as VMS 7.1's $STATUS showed them. The messages DCL
// shows from the status have no STS$M_INHIB_MSG, and the F ones the utility
// signaled itself have.
const (
	convOpenIn   = 0x10B2109C
	convOpenOut  = 0x10B210A4
	convWriteErr = 0x10B210D4
	convNoWild   = 0x00B2112A
	convConfQual = 0x00B212E2
	convFatalExc = 0x00B2808A
	convInpFiles = 0x00B280BA
	convNoKey    = 0x00B280DA
	convNotSeq   = 0x00B280EA
	sortBadMerge = 0x101C80BC

	fdlStatusWarning = fdlFacility | 0x8058

	convMaxInputs    = 10
	convMaxWorkFiles = 10
)

// convertExceptions are the messages for a record that can't go in.
var convertExceptions = map[string]string{
	"DUP": "duplicate key encountered",
	"RSK": "record shorter than primary key",
	"RTL": "record longer than maximum record length",
	"RTS": "record too short for fixed record format file",
	"SEQ": "record not in order",
	"VFC": "record too short to fill fixed part of VFC record",
}

// ConvertRequest is CONVERT's parameters and qualifiers.
type ConvertRequest struct {
	Inputs []string
	Output string

	FDL string

	// Exceptions is /EXCEPTIONS_FILE; ExceptionsFile its value, "" meaning
	// SYS$OUTPUT.
	Exceptions     bool
	ExceptionsFile string

	// Pad is /PAD; PadValue its value, "" meaning null.
	Pad      bool
	PadValue string

	HasKey bool
	Key    int

	Prolog int

	HasWorkFiles bool
	WorkFiles    int

	Append, NoCreate, Exit, NoFastLoad, FillBuckets, FixedControl, Merge bool
	Share, NoSort, Statistics, Truncate                                  bool
}

// conversion is one CONVERT in progress.
type conversion struct {
	c      *Context
	req    ConvertRequest
	out    *rms.ConvertOutput
	inVFC  int
	pad    byte
	exc    *rms.ConvertOutput
	counts struct{ files, records, exceptions, valid int }
}

// Convert implements CONVERT.
func (c *Context) Convert(req ConvertRequest) error {
	started := time.Now()

	switch {
	case req.Append && req.Merge:
		return vmserrors.ConditionError{Status: convConfQual, Text: "CONV-E-CONFQUAL, conflicting qualifiers"}
	case len(req.Inputs) > convMaxInputs:
		return vmserrors.ConditionError{Status: convInpFiles, Text: "CONVERT-E-INP_FILES, too many input files"}
	}

	for _, name := range req.Inputs {
		if strings.ContainsAny(name, "*%") {
			return vmserrors.ConditionError{Status: convNoWild, Text: "CONV-E-NOWILD, no wildcard permitted"}
		}
	}

	conv := &conversion{c: c, req: req}

	inputs, err := conv.readInputs()
	if err != nil {
		return err
	}

	output, err := conv.outputName()
	if err != nil {
		return err
	}

	if err := conv.openOutput(inputs[0], output); err != nil {
		return err
	}

	failure := conv.run(inputs)

	if conv.exc != nil {
		if err := conv.exc.Close(); err != nil && failure == nil {
			failure = err
		}
	}

	if err := conv.out.Close(); err != nil && failure == nil {
		failure = err
	}

	if failure != nil {
		return failure
	}

	if req.Statistics {
		conv.showStatistics(time.Since(started))
	}

	if c.SetStatus != nil {
		c.SetStatus(1)
	}

	return nil
}

// readInputs reads every input file whole: OPENIN for one it can't open, NOKEY
// for /KEY past an indexed file's keys (a file that isn't indexed takes no notice).
func (conv *conversion) readInputs() ([]*rms.ConvertInput, error) {
	s := conv.c.ContainerSession

	var inputs []*rms.ConvertInput

	for _, name := range conv.req.Inputs {
		key := 0
		if conv.req.HasKey {
			key = conv.req.Key
		}

		in, err := s.ReadConvertInput(name, key, conv.req.Share)

		var open *rms.ConvertOpenError

		switch {
		case errors.Is(err, rms.ErrNoSuchKey):
			return nil, vmserrors.ConditionError{Status: convNoKey, Text: "CONVERT-E-NOKEY, no such key"}
		case errors.As(err, &open):
			return nil, convertOpenFailure(convOpenIn, "OPENIN", open.Name, "input", open.Status, open.STV)
		case err != nil:
			return nil, FileFailure(err, name)
		}

		inputs = append(inputs, in)
		conv.counts.files++
	}

	return inputs, nil
}

// convertOpenFailure is OPENIN or OPENOUT, with RMS's status under it (VMS 7.1
// showed FNF alone, as if with no STV: C10's C101.DAT and C108.TXT).
func convertOpenFailure(status uint32, ident, name, direction string, sts, stv uint32) error {
	return vmserrors.ConditionError{
		Status: status,
		Text:   "CONV-F-" + ident + ", error opening " + name + " as " + direction + "\n" + conditionLineWith(sts, stv),
	}
}

// outputName is the output's name: a wildcard is OPENOUT with RMS$_FNM, the name
// as given (C10), and a name without a type takes the first input's (C64.SEQ).
func (conv *conversion) outputName() (string, error) {
	name := conv.req.Output

	if strings.ContainsAny(name, "*%") {
		return "", convertOpenFailure(convOpenOut, "OPENOUT", strings.ToUpper(name), "output", vmsdef.Symbols["RMS$_FNM"], 0)
	}

	rest, version := rms.SplitVersion(name)
	if strings.Contains(rms.NameType(rest), ".") {
		return name, nil
	}

	first, _ := rms.SplitVersion(conv.req.Inputs[0])

	_, typ, found := strings.Cut(rms.NameType(first), ".")
	if !found {
		return name, nil
	}

	return rest + "." + typ + version, nil
}

// openOutput makes the output, unless it's to be an existing file, and opens it.
func (conv *conversion) openOutput(first *rms.ConvertInput, name string) error {
	c, req := conv.c, conv.req
	s := c.ContainerSession

	created := false
	appending := req.Append

	if !req.NoCreate && !req.Merge && !req.Append {
		created = true

		var made rms.FDLCreated

		if req.FDL != "" {
			def, err := conv.readDefinition()
			if err != nil {
				return err
			}

			made = s.CreateFDL(rms.FDLCreate{Definition: def, Name: name, Owner: c.ownerUIC, Prolog: req.Prolog})
		} else {
			made = s.CreateLike(first, name, req.Prolog)
		}

		if made.Status&1 == 0 {
			return convertOpenFailure(convOpenOut, "OPENOUT", made.Name, "output", made.Status, made.STV)
		}

		name = made.Name
	}

	out, err := s.OpenConvertOutput(name, created, appending)

	var open *rms.ConvertOpenError

	switch {
	case errors.As(err, &open):
		// /APPEND makes a sequential file that isn't there (the manual's
		// "or when you are creating a new sequential file"; unconfirmed).
		if appending && open.Status == vmsdef.Symbols["RMS$_FNF"] {
			made := s.CreateLike(first, name, 0)
			if made.Status&1 == 0 {
				return convertOpenFailure(convOpenOut, "OPENOUT", made.Name, "output", made.Status, made.STV)
			}

			if out, err = s.OpenConvertOutput(made.Name, true, false); err == nil {
				conv.out = out

				return nil
			}

			return FileFailure(err, name)
		}

		return convertOpenFailure(convOpenOut, "OPENOUT", open.Name, "output", open.Status, open.STV)
	case err != nil:
		return FileFailure(err, name)
	}

	conv.out = out

	if req.Append && out.Indexed {
		_ = out.Close()

		return vmserrors.ConditionError{Status: convNotSeq, Text: "CONVERT-E-NOTSEQ, output file is not a sequential file"}
	}

	return nil
}

// readDefinition reads /FDL's file: unlike CREATE/FDL, CONVERT makes no file after
// a warning, its status FDL's WARNING (C10's C103.DAT).
func (conv *conversion) readDefinition() (*fdl.Definition, error) {
	c := conv.c

	text, failure := c.readFDLFile(conv.req.FDL)
	if failure != nil {
		return nil, failure
	}

	def, msgs := fdl.Parse(text, fdl.Options{ParseTime: c.ParseTime})
	c.showFDLMessages(msgs)

	if fdl.Failed(msgs) {
		return nil, vmserrors.ConditionError{Status: fdlStatusError, Text: fdl.Text("FDLERROR", 'E', "")}
	}

	if len(msgs) > 0 {
		return nil, vmserrors.ConditionError{Status: fdlStatusWarning, Text: fdl.Text("WARNING", 'W', "")}
	}

	return def, nil
}

// padCharacter is /PAD's value: a letter or digit, or %D, %O, or %X and a number, of
// which only the low byte counts (%X300 wasn't refused; C10); null with none.
func padCharacter(value string) byte {
	if len(value) < 2 || value[0] != '%' {
		if value == "" {
			return 0
		}

		return value[0]
	}

	base := map[byte]int{'D': 10, 'O': 8, 'X': 16}[value[1]&^0x20]
	if base == 0 {
		return value[0]
	}

	n, err := strconv.ParseUint(value[2:], base, 64)
	if err != nil {
		return 0
	}

	return byte(n)
}

// run converts the records.
func (conv *conversion) run(inputs []*rms.ConvertInput) error {
	req, out := conv.req, conv.out

	conv.inVFC = inputs[0].VFCSize
	conv.pad = padCharacter(req.PadValue)

	if req.Exceptions && req.ExceptionsFile != "" {
		if err := conv.openExceptions(); err != nil {
			return err
		}
	}

	if req.Pad && !out.Fixed {
		conv.c.Printf("%%CONVERT-I-PAD, PAD option ignored, output record format not fixed\n")
	}

	var records []rms.ConvertRecord
	for _, in := range inputs {
		records = append(records, in.Records...)
	}

	conv.counts.records = len(records)

	sorting := out.Indexed && !req.NoSort && !req.Merge
	workFilesBad := req.HasWorkFiles && (req.WorkFiles < 0 || req.WorkFiles > convMaxWorkFiles)

	if sorting {
		same := len(inputs) == 1 && inputs[0].SameKey(out.Primary)
		if workFilesBad && !same {
			return badMerge()
		}

		records = rms.SortRecords(records, out.Primary, len(inputs) > 1)
	}

	fast := out.Indexed && out.Empty && !req.NoFastLoad && !req.Merge
	if fast {
		out.StartFast(req.FillBuckets)
	}

	var last []byte

	for _, r := range records {
		data, control, why := conv.fit(r)
		if why == "" && out.Indexed && !req.Merge && !fast && req.NoSort {
			why = conv.inOrder(data, &last)
		}

		if why == "" {
			var err error

			if why, err = conv.write(data, control, fast); err != nil {
				return err
			}
		}

		if why != "" {
			if err := conv.exception(why, r); err != nil {
				return err
			}

			continue
		}

		conv.counts.valid++
	}

	if !fast {
		return nil
	}

	if err := out.FinishPrimary(); err != nil {
		return fmt.Errorf("convert: %w", err)
	}

	for i := 1; i < out.KeyCount; i++ {
		if workFilesBad {
			return badMerge()
		}

		if err := out.FinishAlternate(i); err != nil {
			return fmt.Errorf("convert: %w", err)
		}
	}

	return nil
}

// badMerge is SORT's refusal of /WORK_FILES past 10, when CONVERT has first to sort.
func badMerge() error {
	return vmserrors.ConditionError{
		Status: sortBadMerge,
		Text:   "SORT-F-BAD_MERGE, number of work files must be between 0 and 10",
	}
}

// fit makes a record what the output takes: its fixed control moved by
// /FIXED_CONTROL, its size by /TRUNCATE and /PAD. It returns the data and control, or
// the name of the exception that stops it.
func (conv *conversion) fit(r rms.ConvertRecord) (data, control []byte, why string) {
	req, out := conv.req, conv.out
	data = r.Data

	switch {
	case out.VFCSize != 0 && conv.inVFC != 0:
		control = make([]byte, out.VFCSize)
		copy(control, r.Control)
	case out.VFCSize != 0 && req.FixedControl:
		if len(data) < out.VFCSize {
			return nil, nil, "VFC"
		}

		control, data = data[:out.VFCSize], data[out.VFCSize:]
	case out.VFCSize != 0:
		control = make([]byte, out.VFCSize)
		if out.PrintControl {
			control[0], control[1] = 0x01, 0x8D
		}
	case conv.inVFC != 0 && req.FixedControl:
		data = append(append(make([]byte, 0, len(r.Control)+len(data)), r.Control...), data...)
	}

	if size := out.MaxRecord; size > 0 && len(data) > size {
		if !req.Truncate {
			return nil, nil, "RTL"
		}

		data = data[:size]
	}

	if out.Fixed && len(data) < out.MaxRecord {
		if !req.Pad {
			return nil, nil, "RTS"
		}

		padded := make([]byte, out.MaxRecord)
		copy(padded, data)

		for i := len(data); i < len(padded); i++ {
			padded[i] = conv.pad
		}

		data = padded
	}

	if out.Indexed {
		if _, ok := out.Primary.Value(data); !ok {
			return nil, nil, "RSK"
		}
	}

	return data, control, ""
}

// inOrder is /NOSORT's check on a $PUT load, as a fast load's Add checks it.
func (conv *conversion) inOrder(data []byte, last *[]byte) string {
	k := conv.out.Primary

	value, _ := k.Value(data)
	if *last != nil {
		c := k.Compare(*last, value)

		switch {
		case c > 0:
			return "SEQ"
		case c == 0 && k.Flags&isam.FlagDuplicates == 0:
			return "DUP"
		}
	}

	*last = value

	return ""
}

// write puts a record in the output, returning the exception that stops it, or an
// error for a failure.
func (conv *conversion) write(data, control []byte, fast bool) (string, error) {
	out := conv.out

	if fast {
		err := out.AddFast(data)

		switch {
		case err == nil:
			return "", nil
		case errors.Is(err, isam.ErrDuplicate):
			return "DUP", nil
		case errors.Is(err, isam.ErrOutOfOrder):
			return "SEQ", nil
		case errors.Is(err, isam.ErrRecordSize):
			return "RTL", nil
		}

		return "", fmt.Errorf("convert: %w", err)
	}

	sts, stv := out.Put(control, data, !conv.req.Merge, conv.req.FillBuckets)

	switch {
	case sts&1 != 0:
		return "", nil
	case sts == vmsdef.Symbols["RMS$_DUP"]:
		return "DUP", nil
	}

	text := "CONV-F-WRITEERR, error writing " + out.Name + "\n" + conditionLineWith(sts, stv)

	return "", vmserrors.ConditionError{Status: convWriteErr, Text: text}
}

// exception reports a record that can't go in: its message, a copy of the input's
// record in the exceptions file or on SYS$OUTPUT, and with /EXIT the end.
func (conv *conversion) exception(why string, r rms.ConvertRecord) error {
	c, req := conv.c, conv.req

	conv.counts.exceptions++
	c.Printf("%%CONVERT-I-%s, %s\n", why, convertExceptions[why])

	switch {
	case conv.exc != nil:
		if sts, stv := conv.exc.Put(nil, r.Data, false, false); sts&1 == 0 {
			text := "CONV-F-WRITEERR, error writing " + conv.exc.Name + "\n" + conditionLineWith(sts, stv)

			return vmserrors.ConditionError{Status: convWriteErr, Text: text}
		}
	case req.Exceptions:
		c.Printf("%s\n", r.Data)
	}

	if req.Exit {
		return vmserrors.ConditionError{Status: convFatalExc, Text: "CONVERT-E-FATALEXC, fatal exception encountered"}
	}

	return nil
}

// openExceptions makes the exceptions file: variable records with carriage
// return control, type .EXC.
func (conv *conversion) openExceptions() error {
	c := conv.c
	s := c.ContainerSession

	loc, err := s.Locate(conv.req.ExceptionsFile, false)
	if err != nil {
		return FileFailure(err, conv.req.ExceptionsFile)
	}

	name := loc.WithDefaultType("EXC").Name

	def, _ := fdl.Parse("FILE; ORGANIZATION sequential\n", fdl.Options{})

	made := s.CreateFDL(rms.FDLCreate{Definition: def, Name: name})
	if made.Status&1 == 0 {
		return convertOpenFailure(convOpenOut, "OPENOUT", made.Name, "output", made.Status, made.STV)
	}

	exc, err := s.OpenConvertOutput(made.Name, true, false)
	if err != nil {
		return FileFailure(err, name)
	}

	conv.exc = exc

	return nil
}

// showStatistics is /STATISTICS's report, in VMS 7.1's layout (C7.7). govax counts
// none of a host's I/O or page faults for a utility, so those are 0, and its CPU time
// is the time it took.
func (conv *conversion) showStatistics(took time.Duration) {
	c := conv.c
	n := conv.counts
	tm := convertTime(took)

	c.Printf(" \n CONVERT Statistics\n")
	c.Printf("Number of Files Processed:%10d\n", n.files)
	c.Printf("Total Records Processed:%12d\tBuffered I/O Count: \t%8d\n", n.records, 0)
	c.Printf("Total Exception Records:%12d\tDirect I/O Count: \t%8d\n", n.exceptions, 0)
	c.Printf("Total Valid Records:%16d\tPage Faults: \t\t%8d\n", n.valid, 0)
	c.Printf("Elapsed Time:%23s\tCPU Time:%23s\n", tm, tm)
}

// convertTime is a time in VMS's delta form, "0 00:00:00.23".
func convertTime(d time.Duration) string {
	cs := int64(d / (10 * time.Millisecond))

	return fmt.Sprintf("%d %02d:%02d:%02d.%02d", cs/8640000, cs/360000%24, cs/6000%60, cs/100%60, cs%100)
}
