package rms

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// A Files-11 file says in its header how its bytes make records. A host file says
// nothing: it's a stream of bytes, maybe with line ends. So a copy from the host
// has to be told what file to make, and that is what these qualifiers do. They are
// govax's own, VMS having no host to copy from. Their names are those of FDL's
// RECORD section, where VMS says the same things of a file:
//
//	/RECORD_FORMAT=format     FIXED, VARIABLE, VFC, STREAM, STREAM_LF, STREAM_CR,
//	                          or UNDEFINED
//	/RECORD_SIZE=n            a fixed record's length, or the longest allowed
//	/CARRIAGE_CONTROL=kind    CARRIAGE_RETURN, FORTRAN, PRINT, or NONE
//	/CONTROL_FIELD_SIZE=n     a VFC record's control bytes, 2 if not given
//
// How the host file's bytes become records is /BINARY's choice. Without it they
// are lines of text, and each line a record. With it they are bytes, with no line
// ends to look for: /RECORD_SIZE cuts them into records, or a stream or undefined
// file holds them as they are. A VFC file's control bytes can't be told from its
// text in a stream of bytes, so VFC takes lines only, and an undefined file has no
// records to make of lines, so it takes bytes only.

// HostRecords is what a copy from the host is told to make.
type HostRecords struct {
	Format          string // /RECORD_FORMAT's keyword
	Size            int64  // /RECORD_SIZE, 0 if not given
	CarriageControl string // /CARRIAGE_CONTROL's keyword, "" if not given
	ControlSize     int64  // /CONTROL_FIELD_SIZE, 0 if not given
}

// HostRecordsError is what the record qualifiers can't do, for the command to report
// as a matter of its qualifiers, not its file names.
type HostRecordsError struct {
	Reason string
}

func (e *HostRecordsError) Error() string { return e.Reason }

// Given reports whether the command said anything of the kind.
func (r HostRecords) Given() bool {
	return r != HostRecords{}
}

// maxHostRecord is the longest record of a sequential file (RMS's limit).
const maxHostRecord = 32767

// keyword finds what the user typed among a qualifier's keywords: the keyword
// itself, or as DCL allows, the start of only one.
func keyword(what, typed string, keywords []string) (string, error) {
	typed = strings.ToUpper(typed)

	var found []string

	for _, k := range keywords {
		if k == typed {
			return k, nil
		}

		if strings.HasPrefix(k, typed) {
			found = append(found, k)
		}
	}

	if typed == "" || len(found) != 1 {
		return "", fmt.Errorf("%s=%s: it is one of %s", what, typed, strings.Join(keywords, ", "))
	}

	return found[0], nil
}

// hostFormats are /RECORD_FORMAT's keywords.
var hostFormats = map[string]ondisk.RecordFormat{
	"FIXED":     ondisk.RecordFormatFixed,
	"VARIABLE":  ondisk.RecordFormatVariable,
	"VFC":       ondisk.RecordFormatVFC,
	"STREAM":    ondisk.RecordFormatStreamCRLF,
	"STREAM_LF": ondisk.RecordFormatStreamLF,
	"STREAM_CR": ondisk.RecordFormatStreamCR,
	"UNDEFINED": ondisk.RecordFormatUndefined,
}

// hostCarriageControls are /CARRIAGE_CONTROL's keywords, as a file's record
// attributes hold them.
var hostCarriageControls = map[string]uint8{
	"NONE":            0,
	"FORTRAN":         ondisk.AttrFortranCC,
	"CARRIAGE_RETURN": ratCarriageReturn,
	"PRINT":           ondisk.AttrPrintCC,
}

// The keywords as a message lists them.
var (
	hostFormatNames         = []string{"FIXED", "VARIABLE", "VFC", "STREAM", "STREAM_LF", "STREAM_CR", "UNDEFINED"}
	hostCarriageControlList = []string{"CARRIAGE_RETURN", "FORTRAN", "PRINT", "NONE"}
)

// hostLayout is the file to make: its attributes, and either its records, or with
// raw set, the host file's bytes as they are.
type hostLayout struct {
	attr    ondisk.RecAttr
	records [][]byte
	raw     bool
}

// layout turns the host file's bytes into the file the qualifiers describe. Any
// error is the command's, found before anything is written.
func (r HostRecords) layout(data []byte, binary bool) (hostLayout, error) {
	var out hostLayout

	if r.Format == "" {
		return out, errors.New("/RECORD_SIZE, /CARRIAGE_CONTROL, and /CONTROL_FIELD_SIZE need /RECORD_FORMAT")
	}

	format, err := keyword("/RECORD_FORMAT", r.Format, hostFormatNames)
	if err != nil {
		return out, err
	}

	if r.Size < 0 || r.Size > maxHostRecord {
		return out, fmt.Errorf("/RECORD_SIZE=%d: a record is at most %d bytes", r.Size, maxHostRecord)
	}

	if err := r.checkFor(format, binary); err != nil {
		return out, err
	}

	cc, err := r.carriageControl(format, binary)
	if err != nil {
		return out, err
	}

	out.attr = ondisk.RecAttr{Format: hostFormats[format], Attributes: cc, MaxRecordSize: uint16(r.Size)}

	switch format {
	case "UNDEFINED":
		out.raw = true
		out.attr.MaxRecordSize = ondisk.BlockSize
	case "STREAM", "STREAM_LF", "STREAM_CR":
		if binary {
			out.raw = true

			return out, nil
		}

		out.records = hostLines(data)
	case "FIXED":
		out.attr.RecordSize = uint16(r.Size)

		if binary {
			out.records, err = cutWhole(data, r.Size)
		} else {
			out.records, err = paddedLines(data, r.Size)
		}
	case "VARIABLE":
		if binary {
			out.records = cut(data, r.Size)
		} else {
			out.records = hostLines(data)
		}
	case "VFC":
		out.attr.VfcSize = uint8(r.controlSize())
		out.records = r.controlledLines(data, cc)
	}

	if err != nil {
		return out, err
	}

	return out, out.setLongest(r)
}

// checkFor refuses the combinations that mean nothing.
func (r HostRecords) checkFor(format string, binary bool) error {
	switch {
	case format == "FIXED" && r.Size == 0:
		return errors.New("/RECORD_FORMAT=FIXED needs /RECORD_SIZE")
	case format == "VARIABLE" && binary && r.Size == 0:
		return errors.New("/BINARY needs /RECORD_SIZE to cut bytes into variable-length records")
	case format == "VFC" && binary:
		return errors.New("/RECORD_FORMAT=VFC takes lines of text, not /BINARY")
	case format == "UNDEFINED" && !binary:
		return errors.New("/RECORD_FORMAT=UNDEFINED has no lines to make records of: it needs /BINARY")
	case format == "UNDEFINED" && r.Size != 0:
		return errors.New("/RECORD_FORMAT=UNDEFINED has no record size")
	case format != "VFC" && r.ControlSize != 0:
		return errors.New("/CONTROL_FIELD_SIZE is for /RECORD_FORMAT=VFC")
	case r.ControlSize < 0 || r.ControlSize > 255:
		return fmt.Errorf("/CONTROL_FIELD_SIZE=%d: it is 1 to 255", r.ControlSize)
	}

	return nil
}

// controlSize is a VFC record's control bytes.
func (r HostRecords) controlSize() int64 {
	if r.ControlSize == 0 {
		return 2
	}

	return r.ControlSize
}

// carriageControl is the record attribute asked for, or what the rest of the
// command implies: lines of text are printed a line at a time (PRINT in a VFC file,
// whose control bytes say how), and bytes are not to be interpreted.
func (r HostRecords) carriageControl(format string, binary bool) (uint8, error) {
	if r.CarriageControl != "" {
		kind, err := keyword("/CARRIAGE_CONTROL", r.CarriageControl, hostCarriageControlList)
		if err != nil {
			return 0, err
		}

		if kind == "PRINT" && format != "VFC" {
			return 0, errors.New("/CARRIAGE_CONTROL=PRINT is for /RECORD_FORMAT=VFC")
		}

		return hostCarriageControls[kind], nil
	}

	switch {
	case binary || format == "UNDEFINED":
		return 0, nil
	case format == "VFC":
		return ondisk.AttrPrintCC, nil
	}

	return ratCarriageReturn, nil
}

// setLongest records the longest record, as RMS does, and holds the records to a
// maximum the command gave.
func (l *hostLayout) setLongest(r HostRecords) error {
	control := int(l.attr.VfcSize)

	for i, rec := range l.records {
		text := len(rec) - control

		if r.Size != 0 && text > int(r.Size) {
			return fmt.Errorf("record %d is %d bytes, longer than /RECORD_SIZE=%d", i+1, text, r.Size)
		}

		if text > maxHostRecord {
			return fmt.Errorf("record %d is %d bytes, longer than a record can be, %d", i+1, text, maxHostRecord)
		}

		if text > int(l.attr.RecordSize) {
			l.attr.RecordSize = uint16(text)
		}
	}

	return nil
}

// hostLines is a host file's lines, each without its line end, whichever end the
// host's files use.
func hostLines(data []byte) [][]byte {
	var lines [][]byte

	scanner := bufio.NewReader(bytes.NewReader(data))

	for {
		line, err := scanner.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r")))
		}

		if err != nil {
			return lines
		}
	}
}

// paddedLines is a host file's lines as fixed-length records, blanks filling each
// out, as a FORTRAN or COBOL program's text files have them.
func paddedLines(data []byte, size int64) ([][]byte, error) {
	lines := hostLines(data)

	for i, line := range lines {
		if int64(len(line)) > size {
			return nil, fmt.Errorf("line %d is %d bytes, longer than /RECORD_SIZE=%d", i+1, len(line), size)
		}

		lines[i] = append(append(make([]byte, 0, size), line...), bytes.Repeat([]byte{' '}, int(size)-len(line))...)
	}

	return lines, nil
}

// cut is data in records of size bytes, the last perhaps shorter.
func cut(data []byte, size int64) [][]byte {
	var records [][]byte

	for int64(len(data)) > size {
		records = append(records, data[:size])
		data = data[size:]
	}

	if len(data) > 0 {
		records = append(records, data)
	}

	return records
}

// cutWhole is cut for a fixed-length file, which has no shorter record to end with.
func cutWhole(data []byte, size int64) ([][]byte, error) {
	if int64(len(data))%size != 0 {
		return nil, fmt.Errorf("%d bytes aren't a whole number of %d-byte records", len(data), size)
	}

	return cut(data, size), nil
}

// controlledLines is a host file's lines as VFC records, control bytes first, which
// Writer.Put wants as part of the record. Under PRINT, 2 control bytes say what a
// line printed one to a line has: a line feed before it (1) and a carriage return
// after (^X8D: the character, with the high bit saying it is one). Otherwise they
// are zero, and mean nothing.
func (r HostRecords) controlledLines(data []byte, cc uint8) [][]byte {
	control := make([]byte, r.controlSize())
	if cc == ondisk.AttrPrintCC && len(control) == 2 {
		control[0], control[1] = 0x01, 0x8D
	}

	lines := hostLines(data)
	for i, line := range lines {
		lines[i] = append(append(make([]byte, 0, len(control)+len(line)), control...), line...)
	}

	return lines
}

// createLaidOutFile makes the file a layout describes, from the host file's bytes
// data, and returns the version it has (createVersion's rules).
func createLaidOutFile(destVol *volume.Volume, destDir *volume.Directory, destBm *volume.Bitmap, destIb *volume.IndexBitmap, name, typ string, version uint16, layout hostLayout, data []byte) (uint16, error) {
	fullName := name + "." + typ

	dst, err := createVersion(destVol, destDir, fullName, version, layout.attr, destBm, destIb)
	if err != nil {
		return 0, err
	}

	if layout.raw {
		err = copyHostRawToVolume(dst, bytes.NewReader(data), int64(len(data)))
	} else {
		err = putHostRecords(dst, layout.records)
	}

	if err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	return createdVersion(destDir, fullName, version)
}

// putHostRecords writes records to a file just created, and closes it.
func putHostRecords(f *volume.File, records [][]byte) error {
	w, err := odsrms.NewWriter(f)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := w.Put(rec); err != nil {
			return err
		}
	}

	return w.Close()
}
