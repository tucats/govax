package rms

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is the record side of DCL's EDIT command (internal/dclcmd/edit.go).
// A host editor works on a stream of lines, and a file on a volume is records
// with a layout its header describes. So EDIT turns the records into lines for
// the editor, and what the editor wrote back into records of the same layout, in
// a new version of the file.
//
// How a line becomes a record, by the file's record format:
//
//	Fixed      Blanks fill a short line out to the record size. A line longer
//	           than the record size loses blanks at its end if that's enough,
//	           and is otherwise an error. The editor sees a record without the
//	           blanks at its end, which come back when it's written.
//	Variable,  The line is the record. If the file has a maximum record size, a
//	stream     line past it is treated as a fixed file's is.
//	VFC        As Variable, with control bytes in front. Lines have no place for
//	           them, so every new record gets the same ones (see control).
//
// What can't be edited this way is refused before an editor starts: a file with
// no records (a relative or indexed file, a directory, undefined records), and
// one with records that lines can't hold.

// NotTextError says why a file can't be edited as lines of text.
type NotTextError struct {
	Spec   string
	Reason string
}

func (e *NotTextError) Error() string { return e.Spec + ": " + e.Reason }

// ReadOnlyError says a file to be edited is on a volume mounted read-only, where
// no new version could be made of it.
type ReadOnlyError struct {
	Spec string
}

func (e *ReadOnlyError) Error() string { return e.Spec + ": the volume is mounted read-only" }

// TooLongError says a line of the edited text holds more than a record of the
// file can, not counting blanks at its end.
type TooLongError struct {
	Line   int // the line's number, from 1
	Length int // its length, without the blanks at its end
	Limit  int // the longest record the file takes
}

func (e *TooLongError) Error() string {
	return fmt.Sprintf("line %d is %d characters long, and a record holds %d", e.Line, e.Length, e.Limit)
}

// EditFile is a file being edited: its text as lines for the editor, and what it
// takes to make the new version.
type EditFile struct {
	// Spec is the file: a volume file's full specification, with the version
	// read, or a host file's path.
	Spec string

	// New says the file doesn't exist yet. The editor starts with nothing, and
	// a volume file is made as text files are (RFM=VAR, RAT=CR).
	New bool

	// Text is the file's lines, each ended as the host's text files end them.
	Text []byte

	// MixedControl says the VFC records didn't all have the same control
	// bytes, so that the new version can't keep them (see control).
	MixedControl bool

	host       bool
	attr       ondisk.RecAttr
	inherit    bool // protection and owner are the old version's
	protection uint16
	owner      ondisk.Uic

	// control is what every new record of a VFC file gets: the bytes the old
	// records all had, or the usual ones for a file of its kind.
	control []byte
}

// ScratchName is a name for the file an editor works on: lowercase for a volume
// file, as an editor expects a type it knows to look (foo.for, login.com).
func (e *EditFile) ScratchName() string {
	if e.host {
		return filepath.Base(e.Spec)
	}

	spec, sts := scanSpec(e.Spec)
	if sts != 0 || spec.Name+spec.Type == "" {
		return "edit.tmp"
	}

	name := spec.Name
	if spec.Type != "" {
		name += "." + spec.Type
	}

	return strings.ToLower(name)
}

// OpenForEdit finds the file named, and reads it for editing. A name with no
// version that matches no file is a new file, if its directory exists. Nothing
// about the file is changed, but anything that would stop the new version being
// written is an error here, before anyone has typed anything.
func (s *Session) OpenForEdit(name string, host bool) (*EditFile, error) {
	loc, err := s.Locate(name, host)
	if err != nil {
		return nil, err
	}

	if loc.Host {
		return openHostForEdit(loc.Name)
	}

	file, err := s.openVolumeForEdit(loc.Name)

	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return s.newVolumeFileForEdit(loc.Name, err)
	}

	if err != nil {
		return nil, fmt.Errorf("rms: editing %s: %w", loc.Name, err)
	}

	return file, nil
}

// openHostForEdit reads a host file, or says it's new if its directory exists.
func openHostForEdit(path string) (*EditFile, error) {
	data, err := os.ReadFile(path)

	switch {
	case err == nil:
		return &EditFile{Spec: path, host: true, Text: data}, nil

	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &EditFile{Spec: path, host: true, New: true}, nil
}

// openVolumeForEdit reads the one volume file name matches.
func (s *Session) openVolumeForEdit(name string) (*EditFile, error) {
	var file *EditFile

	err := s.firstSpec(name, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return err
		}

		switch len(matches) {
		case 0:
			return &NotFoundError{Spec: name}
		case 1:
		default:
			return &AmbiguousError{Spec: name, Count: len(matches)}
		}

		spec := r.shownFile(matches[0])

		if !s.Mounts.Writable(r.Spec.Device) {
			return &ReadOnlyError{Spec: spec}
		}

		f, err := vol.OpenFID(matches[0].Fid)
		if err != nil {
			return err
		}

		file, err = readForEdit(f, spec)

		return err
	})

	return file, err
}

// newVolumeFileForEdit is a file that isn't there, if it can be made: notFound is
// the error for the cases where it can't.
func (s *Session) newVolumeFileForEdit(name string, notFound error) (*EditFile, error) {
	vol, spec, err := s.resolveVolume(name)
	if err != nil {
		return nil, fmt.Errorf("rms: editing %s: %w", name, err)
	}

	// A version asked for, or a wildcard, was looking for a file that exists.
	if spec.Version != "" || spec.Name+spec.Type == "" || strings.ContainsAny(spec.Name+spec.Type, "*%") {
		return nil, notFound
	}

	if _, err := filespec.ResolveDirectory(vol, spec.Dirs); err != nil {
		return nil, fmt.Errorf("rms: editing %s: %w", name, err)
	}

	if !s.Mounts.Writable(spec.Device) {
		return nil, &ReadOnlyError{Spec: spec.String()}
	}

	return &EditFile{Spec: spec.String(), New: true, attr: recordKindAttributes[TextRecords]}, nil
}

// readForEdit turns a volume file's records into lines.
func readForEdit(f *volume.File, spec string) (*EditFile, error) {
	refuse := func(reason string) (*EditFile, error) {
		return nil, &NotTextError{Spec: spec, Reason: reason}
	}

	attr := f.Header.RecordAttributes

	switch {
	case f.Header.IsDirectory():
		return refuse("it is a directory")
	case attr.Format.Organization() != ondisk.OrganizationSequential:
		return refuse("it is not a sequential file")
	case attr.Format.Record() == ondisk.RecordFormatUndefined:
		return refuse("its records are undefined")
	case attr.Format.Record() == ondisk.RecordFormatFixed && attr.MaxRecordSize == 0:
		return refuse("its fixed records have no size")
	}

	records, err := readVolumeRecords(f, VariableRecords)
	if err != nil {
		return nil, err
	}

	file := &EditFile{
		Spec:       spec,
		attr:       attr,
		inherit:    true,
		protection: f.Header.FileProtection,
		owner:      f.Header.Owner,
	}

	var text bytes.Buffer

	for i, record := range records {
		line, err := file.lineOf(record)
		if err != nil {
			return refuse(fmt.Sprintf("record %d %s", i+1, err))
		}

		text.Write(line)
		text.WriteString(hostLineEnd)
	}

	file.Text = text.Bytes()

	if attr.Format.Record() == ondisk.RecordFormatVFC && file.control == nil {
		file.control = usualControl(attr)
	}

	return file, nil
}

// lineOf is one record as a line, noting its control bytes if it has any.
func (e *EditFile) lineOf(record []byte) ([]byte, error) {
	line := record

	switch e.attr.Format.Record() {
	case ondisk.RecordFormatFixed:
		line = bytes.TrimRight(line, " ")

	case ondisk.RecordFormatStreamCRLF:
		// A record of this format may end at a lone LF, which is then its last
		// byte (ods2's Reader); as a line it ends the same way.
		line = bytes.TrimSuffix(line, []byte("\n"))

	case ondisk.RecordFormatVFC:
		size := int(e.attr.VfcSize)
		if len(line) < size {
			return nil, errors.New("is shorter than its control bytes")
		}

		e.noteControl(line[:size])
		line = line[size:]
	}

	switch {
	case bytes.IndexByte(line, 0) >= 0:
		return nil, errors.New("holds a null byte")
	case bytes.IndexByte(line, '\n') >= 0:
		return nil, errors.New("holds a line feed")
	case bytes.HasSuffix(line, []byte("\r")):
		return nil, errors.New("ends in a carriage return")
	}

	return line, nil
}

// noteControl takes one VFC record's control bytes into account: the new version
// can keep them only if they're all the same.
func (e *EditFile) noteControl(control []byte) {
	switch {
	case e.MixedControl:
	case e.control == nil:
		e.control = bytes.Clone(control)
	case !bytes.Equal(e.control, control):
		e.MixedControl = true
		e.control = usualControl(e.attr)
	}
}

// usualControl is the control bytes of a VFC file with nothing better to go on.
// Under PRINT, 2 of them say what a line printed one to a line has (see
// HostRecords.controlledLines). Otherwise they are zero, and mean nothing.
func usualControl(attr ondisk.RecAttr) []byte {
	control := make([]byte, attr.VfcSize)
	if attr.Attributes&ondisk.AttrPrintCC != 0 && len(control) == 2 {
		control[0], control[1] = 0x01, 0x8D
	}

	return control
}

// longestLine is the most a record of the file holds, not counting the control
// bytes of a VFC record.
func (e *EditFile) longestLine() int {
	control := int(e.attr.VfcSize)
	limit := maxHostRecord - control

	if size := int(e.attr.MaxRecordSize); size != 0 && size < limit {
		limit = size
	}

	// A record that can't cross a block has its length word to fit in one too.
	if e.attr.Attributes&ondisk.AttrNoSpan != 0 {
		switch e.attr.Format.Record() {
		case ondisk.RecordFormatVariable, ondisk.RecordFormatVFC:
			limit = min(limit, ondisk.BlockSize-2-control)
		}
	}

	return limit
}

// Records turns the edited text into the file's records, or says which line can't
// be one. Nothing is written.
func (e *EditFile) Records(text []byte) ([][]byte, error) {
	lines := hostLines(text)
	limit := e.longestLine()
	records := make([][]byte, 0, len(lines))

	for i, line := range lines {
		if len(line) > limit {
			kept := bytes.TrimRight(line, " ")
			if len(kept) > limit {
				return nil, &TooLongError{Line: i + 1, Length: len(kept), Limit: limit}
			}

			line = line[:limit]
		}

		switch e.attr.Format.Record() {
		case ondisk.RecordFormatFixed:
			line = append(bytes.Clone(line), bytes.Repeat([]byte{' '}, limit-len(line))...)

		case ondisk.RecordFormatVFC:
			line = append(bytes.Clone(e.control), line...)
		}

		records = append(records, line)
	}

	return records, nil
}

// SaveEdit makes the edited text the file's new version, and returns the file as
// it now is and the number of its lines: on a volume, the next version, with the
// attributes, protection, and owner of the version edited; on the host, the file
// replaced. A line that can't be a record (Records) means no new version.
func (s *Session) SaveEdit(e *EditFile, text []byte) (FileLocation, int, error) {
	if e.host {
		if err := replaceHostFile(e.Spec, text); err != nil {
			return FileLocation{}, 0, err
		}

		return FileLocation{Host: true, Name: e.Spec}, len(hostLines(text)), nil
	}

	records, err := e.Records(text)
	if err != nil {
		return FileLocation{}, 0, err
	}

	created, err := s.writeNewVersion(e, records)
	if err != nil {
		return FileLocation{}, 0, fmt.Errorf("rms: writing %s: %w", e.Spec, err)
	}

	return created, len(records), nil
}

// writeNewVersion makes the next version of the file, holding records. If its
// records can't be written, the partial file is deleted.
func (s *Session) writeNewVersion(e *EditFile, records [][]byte) (FileLocation, error) {
	vol, spec, err := s.resolveVolume(e.Spec)
	if err != nil {
		return FileLocation{}, err
	}

	dir, bm, ib, err := resolveVolumeDest(vol, spec)
	if err != nil {
		return FileLocation{}, err
	}

	fullName := spec.Name + "." + spec.Type

	f, err := createVersion(vol, dir, fullName, 0, e.newAttributes(records), bm, ib)
	if err != nil {
		return FileLocation{}, err
	}

	version, err := lookUpVersion(dir, fullName)
	if err != nil {
		return FileLocation{}, err
	}

	spec.Version = fmt.Sprint(version)

	if err := putHostRecords(f, records); err != nil {
		_ = s.deleteVersion(spec.String())

		return FileLocation{}, err
	}

	if e.inherit {
		err := volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) {
			h.FileProtection = e.protection
			h.Owner = e.owner
		})
		if err != nil {
			return FileLocation{}, err
		}
	}

	return FileLocation{Name: spec.String()}, nil
}

// newAttributes is the record attributes of the new version: the old version's,
// without what describes its contents. The longest record is RMS's to record in
// any but a fixed file.
func (e *EditFile) newAttributes(records [][]byte) ondisk.RecAttr {
	attr := e.attr
	attr.HighestBlock, attr.EndOfFileBlock, attr.FirstFreeByte = 0, 0, 0

	if attr.Format.Record() == ondisk.RecordFormatFixed {
		return attr
	}

	attr.RecordSize = 0

	for _, record := range records {
		attr.RecordSize = max(attr.RecordSize, uint16(len(record)-int(attr.VfcSize)))
	}

	return attr
}

// replaceHostFile writes a host file through a temporary file beside it, so that
// a failure leaves the old one alone.
func replaceHostFile(path string, data []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := createTemp(path, mode)
	if err != nil {
		return err
	}

	_, err = tmp.Write(data)

	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}

	if err != nil {
		_ = os.Remove(tmp.Name())
	}

	return err
}
