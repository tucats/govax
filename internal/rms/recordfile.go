package rms

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file is docs/PHASE-27.md subtask 9's record API: reading every
// record of a file, or creating a file from a list of records, wherever a
// FileLocation (location.go) says the file is. A command like MACRO reads
// its source and writes its object through it without caring which side
// each is on.
//
// A file on a volume has real record boundaries. A host file doesn't, so
// the caller says what kind of file it is, and that decides the host
// layout:
//
//   - TextRecords: lines of text. A host file is lines ending in LF (a CR
//     before the LF is dropped). A new volume file is what VMS makes for
//     text: variable-length records with carriage-return carriage
//     control.
//   - VariableRecords: records whose boundaries are part of the data, such
//     as an object module's. A host file uses ODS-2's own on-disk layout
//     for variable-length records (obj.ReadRecords/obj.WriteRecords): a
//     2-byte length, the data, and a pad byte to an even length, the same
//     bytes a raw copy of the volume file's blocks has. A new volume file
//     has the attributes real VAX MACRO gives an object module.
//   - ImageBlocks: a VMS image, fixed-length 512-byte records. A host
//     file is the blocks one after another; a new volume file has real
//     LINK's attributes (RFM=FIX, 512-byte records).

// RecordKind is the kind of record file, which decides its host layout
// and its attributes on a volume.
type RecordKind int

const (
	// TextRecords is a text file.
	TextRecords RecordKind = iota

	// VariableRecords is a file of binary variable-length records.
	VariableRecords

	// ImageBlocks is a VMS image: fixed-length 512-byte records. A host
	// file is the blocks one after another, as a raw copy of the volume
	// file has them.
	ImageBlocks
)

// imageBlockSize is an image file's record (and block) size.
const imageBlockSize = 512

// recordKindAttributes gives each kind's attributes for a new volume
// file. The object module's are copied from real VAX MACRO's .OBJ files
// (testdata/disks/mar-exchange-vms.dsk): RFM=VAR, no carriage control, no
// maximum record size, and a default extension of 20 blocks. RecordSize
// is filled in with the longest record, as RMS does.
var recordKindAttributes = map[RecordKind]ondisk.RecAttr{
	TextRecords:     {Format: ondisk.RecordFormatVariable, Attributes: ratCarriageReturn},
	VariableRecords: {Format: ondisk.RecordFormatVariable, DefaultExtend: 20},
	// An image's attributes, as real LINK's are (mar-exchange2.dsk):
	// RFM=FIX, 512-byte records.
	ImageBlocks: {Format: ondisk.RecordFormatFixed, RecordSize: imageBlockSize, MaxRecordSize: imageBlockSize},
}

// ratCarriageReturn is FAT$M_IMPLIEDCC (RAT=CR): each record is a line.
const ratCarriageReturn = 0x02

// hostRecordTypes names the file types whose host files COPY keeps in a
// record kind's host layout, so that records survive a copy in either
// direction. A type not listed here is copied as text, or as raw blocks
// with /BINARY.
var hostRecordTypes = map[string]RecordKind{
	"OBJ": VariableRecords,
}

// ReadRecordFile reads every record of the file at loc, and returns them
// with the location of the file actually read: for a volume, its full
// specification with its version. A volume specification must name one
// file; with no version, it's the highest.
func (s *Session) ReadRecordFile(loc FileLocation, kind RecordKind) ([][]byte, FileLocation, error) {
	if loc.Host {
		records, err := s.readHostRecords(loc.Name, kind)

		return records, loc, err
	}

	var (
		records [][]byte
		found   FileLocation
	)

	err := s.firstSpec(loc.Name, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return err
		}

		switch len(matches) {
		case 0:
			return &NotFoundError{Spec: loc.Name}
		case 1:
		default:
			return &AmbiguousError{Spec: loc.Name, Count: len(matches)}
		}

		m := matches[0]

		f, err := vol.OpenFID(m.Fid)
		if err != nil {
			return err
		}

		if records, err = readVolumeRecords(f, kind); err != nil {
			return err
		}

		found = FileLocation{Name: fullSpec(r.Display, m)}

		return nil
	})
	if err != nil {
		return nil, FileLocation{}, fmt.Errorf("rms: reading %s: %w", loc.Name, err)
	}

	return records, found, nil
}

// ReadRawFile reads the bytes of the file at loc, whatever its records: a
// host file whole, or a volume file's blocks up to its end-of-file byte,
// as COPY/BINARY copies it. LINK reads libraries and shareable images this
// way. It returns the location of the file actually read, as
// ReadRecordFile does.
func (s *Session) ReadRawFile(loc FileLocation) ([]byte, FileLocation, error) {
	if loc.Host {
		data, err := s.readHostFile(loc.Name)

		return data, loc, err
	}

	var (
		buf   bytes.Buffer
		found FileLocation
	)

	err := s.firstSpec(loc.Name, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return err
		}

		switch len(matches) {
		case 0:
			return &NotFoundError{Spec: loc.Name}
		case 1:
		default:
			return &AmbiguousError{Spec: loc.Name, Count: len(matches)}
		}

		f, err := vol.OpenFID(matches[0].Fid)
		if err != nil {
			return err
		}

		if err := copyRawTo(&buf, f); err != nil {
			return err
		}

		found = FileLocation{Name: fullSpec(r.Display, matches[0])}

		return nil
	})
	if err != nil {
		return nil, FileLocation{}, fmt.Errorf("rms: reading %s: %w", loc.Name, err)
	}

	return buf.Bytes(), found, nil
}

// fullSpec is a matched file's complete specification.
func fullSpec(device string, m filespec.Match) string {
	return filespec.Spec{Device: device, Dirs: m.Dirs, Name: m.Name, Type: m.Type, Version: fmt.Sprint(m.Version)}.String()
}

// readHostRecords reads a host file (hostfile.go) in kind's layout.
func (s *Session) readHostRecords(path string, kind RecordKind) ([][]byte, error) {
	data, err := s.readHostFile(path)
	if err != nil {
		return nil, err
	}

	return hostRecords(path, data, kind)
}

// hostRecords splits a host file's bytes, read from path, into records in
// kind's layout.
func hostRecords(path string, data []byte, kind RecordKind) ([][]byte, error) {

	switch kind {
	case VariableRecords:
		records, err := obj.ReadRecords(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: not a variable-length record file: %w", path, err)
		}

		return records, nil

	case ImageBlocks:
		if len(data)%imageBlockSize != 0 {
			return nil, fmt.Errorf("%s: %d bytes isn't a whole number of blocks", path, len(data))
		}

		var records [][]byte
		for i := 0; i < len(data); i += imageBlockSize {
			records = append(records, data[i:i+imageBlockSize])
		}

		return records, nil
	}

	return splitLines(data), nil
}

// splitLines splits text into lines, dropping each line's LF and a CR
// before it. A last line with no LF is still a line.
func splitLines(data []byte) [][]byte {
	var out [][]byte

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, len(data)+1)

	for sc.Scan() {
		out = append(out, bytes.Clone(sc.Bytes()))
	}

	return out
}

// readVolumeRecords reads every record of a volume file. A VFC file's
// fixed control bytes aren't part of a text line, so they're dropped.
// Variable records in a file with no record structure (one copied in
// with an older COPY/BINARY, as raw blocks) are read from its bytes in
// the host layout, which is the same as the on-disk one.
func readVolumeRecords(f *volume.File, kind RecordKind) ([][]byte, error) {
	attr := f.Header.RecordAttributes

	if kind == VariableRecords && attr.Format == ondisk.RecordFormatUndefined {
		var buf bytes.Buffer
		if err := copyRawTo(&buf, f); err != nil {
			return nil, err
		}

		return obj.ReadRecords(&buf)
	}

	r, err := odsrms.NewReader(f)
	if err != nil {
		return nil, err
	}

	var out [][]byte

	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}

		if err != nil {
			return nil, err
		}

		if kind == TextRecords && attr.Format == ondisk.RecordFormatVFC && len(rec) >= int(attr.VfcSize) {
			rec = rec[attr.VfcSize:]
		}

		out = append(out, rec)
	}
}

// CreateRecordFile creates a file at loc holding records, as kind's
// layout and attributes say, and returns its location: for a volume, its
// full specification with the version it got (the next one, unless loc
// names a version). A host file is written in full to a temporary file
// first and then renamed, so a failure never leaves a partial file or
// replaces a good one.
func (s *Session) CreateRecordFile(loc FileLocation, kind RecordKind, records [][]byte) (FileLocation, error) {
	if loc.Host {
		return loc, writeHostRecords(loc.Name, kind, records)
	}

	created, err := s.createVolumeRecords(loc.Name, kind, records)
	if err != nil {
		return FileLocation{}, fmt.Errorf("rms: creating %s: %w", loc.Name, err)
	}

	return created, nil
}

// rewriteTempName is the file RewriteRecordFile keeps its safe copy in,
// beside the file it rewrites.
const rewriteTempName, rewriteTempType = "GOVAX$REWRITE", "TMP"

// RewriteRecordFile replaces the contents of the existing file at loc with
// records, keeping its name and version, as a program that updates a file
// in place leaves it (VMS's LIBRARIAN updating a library). A host file is
// replaced as CreateRecordFile replaces one. A volume file's location
// must name its version (as ReadRecordFile's result does).
//
// ods2 can't rewrite a file's blocks in place to a new length, so a volume
// file is replaced in four steps, each leaving a complete copy of either
// the old or the new contents: the records are written to a temporary
// file (GOVAX$REWRITE.TMP) beside it; the old version is deleted; the
// records are written again at the old version; and the temporary file is
// deleted. A failure writing the temporary file changes nothing. If the
// file can't be written again after the old version is deleted, the error
// names the temporary file, which holds the new contents. Going through a
// different name keeps the file's version limit from purging the new
// version: its count of versions is the same afterwards as before. The
// file keeps its version limit too. It gets a new file ID and creation
// date, where an update in place would keep them.
func (s *Session) RewriteRecordFile(loc FileLocation, kind RecordKind, records [][]byte) (FileLocation, error) {
	if loc.Host {
		return loc, writeHostRecords(loc.Name, kind, records)
	}

	failed := func(err error) (FileLocation, error) {
		return FileLocation{}, fmt.Errorf("rms: rewriting %s: %w", loc.Name, err)
	}

	_, spec, err := s.resolveVolume(loc.Name)
	if err != nil {
		return failed(err)
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok || version == 0 {
		return failed(fmt.Errorf("the file's version must be given"))
	}

	// The file's version limit, which the new file would otherwise take
	// from its directory, there being no other version of it.
	limit, err := s.versionLimit(spec.String(), 0, false)
	if err != nil {
		return failed(err)
	}

	temp := spec
	temp.Name, temp.Type, temp.Version = rewriteTempName, rewriteTempType, ""

	tempLoc, err := s.createVolumeRecords(temp.String(), kind, records)
	if err != nil {
		return failed(err)
	}

	if err := s.deleteVersion(spec.String()); err != nil {
		_ = s.deleteVersion(tempLoc.Name)

		return failed(err)
	}

	created, err := s.createVolumeRecords(spec.String(), kind, records)
	if err != nil {
		return failed(fmt.Errorf("%w (the new contents are in %s)", err, tempLoc.Name))
	}

	if _, err := s.versionLimit(created.Name, limit, true); err != nil {
		return created, fmt.Errorf("rms: rewriting %s: %w", loc.Name, err)
	}

	if err := s.deleteVersion(tempLoc.Name); err != nil {
		return created, fmt.Errorf("rms: rewriting %s: removing %s: %w", loc.Name, tempLoc.Name, err)
	}

	return created, nil
}

// versionLimit returns the version limit of the volume file named by its
// full specification, and with set, first sets it to limit.
func (s *Session) versionLimit(text string, limit uint16, set bool) (uint16, error) {
	vol, spec, err := s.resolveVolume(text)
	if err != nil {
		return 0, err
	}

	version, _ := parseOpenVersion(spec.Version)

	dir, _, _, err := resolveVolumeDest(vol, spec)
	if err != nil {
		return 0, err
	}

	entry, err := dir.Lookup(spec.Name+"."+spec.Type, version)
	if err != nil {
		return 0, err
	}

	f, err := vol.OpenFID(entry.Fid)
	if err != nil {
		return 0, err
	}

	if set && f.Header.RecordAttributes.VersionLimit != limit {
		if err := volume.SetVersionLimit(f, limit); err != nil {
			return 0, err
		}
	}

	return f.Header.RecordAttributes.VersionLimit, nil
}

// deleteVersion deletes one version of a volume file, named by its full
// specification.
func (s *Session) deleteVersion(text string) error {
	vol, spec, err := s.resolveVolume(text)
	if err != nil {
		return err
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok || version == 0 {
		return fmt.Errorf("%s: no version to delete", text)
	}

	dir, bm, ib, err := resolveVolumeDest(vol, spec)
	if err != nil {
		return err
	}

	return volume.DeleteFile(dir, spec.Name+"."+spec.Type, version, bm, ib)
}

// writeHostRecords writes a host file in kind's layout, through a
// temporary file in the same directory.
func writeHostRecords(path string, kind RecordKind, records [][]byte) error {
	var buf bytes.Buffer

	switch kind {
	case VariableRecords:
		if err := obj.WriteRecords(&buf, records); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

	case ImageBlocks:
		for _, rec := range records {
			if len(rec) != imageBlockSize {
				return fmt.Errorf("%s: a %d-byte record in an image", path, len(rec))
			}

			buf.Write(rec)
		}

	default:
		for _, rec := range records {
			buf.Write(rec)
			buf.WriteByte('\n')
		}
	}

	// The new file gets an existing one's permissions, or 0644 less the
	// umask, as os.Create would give it.
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := createTemp(path, mode)
	if err != nil {
		return err
	}

	_, err = tmp.Write(buf.Bytes())

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

// createVolumeRecords creates a volume file holding records.
func (s *Session) createVolumeRecords(text string, kind RecordKind, records [][]byte) (FileLocation, error) {
	vol, spec, err := s.resolveVolume(text)
	if err != nil {
		return FileLocation{}, err
	}

	if spec.Name == "" || strings.ContainsAny(spec.Name+spec.Type+spec.Version, "*%") {
		return FileLocation{}, fmt.Errorf("a new file needs one name, with no wildcards")
	}

	if !s.Mounts.Writable(spec.Device) {
		return FileLocation{}, fmt.Errorf("%s: is mounted read-only", spec.Device)
	}

	var version uint16
	
	if spec.Version != "" {
		v, ok := parseOpenVersion(spec.Version)
		if !ok || v > 32767 {
			return FileLocation{}, fmt.Errorf("version %q: a new file needs a version from 1 to 32767", spec.Version)
		}

		version = v
	}

	dir, bm, ib, err := resolveVolumeDest(vol, spec)
	if err != nil {
		return FileLocation{}, err
	}

	fullName := spec.Name + "." + spec.Type

	f, err := vol.CreateFileVersion(dir, fullName, version, newRecordAttributes(kind, records), bm, ib)
	if err != nil {
		return FileLocation{}, err
	}

	if err := putRecords(f, records); err != nil {
		return FileLocation{}, err
	}

	if version == 0 {
		if version, err = lookUpVersion(dir, fullName); err != nil {
			return FileLocation{}, err
		}
	}

	spec.Version = fmt.Sprint(version)

	return FileLocation{Name: spec.String()}, nil
}

// createTemp creates a new, empty file beside path with mode (less the
// umask), for writing path's contents before renaming it into place.
func createTemp(path string, mode fs.FileMode) (*os.File, error) {
	for try := 0; ; try++ {
		name := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s.%d.%d", filepath.Base(path), os.Getpid(), try))

		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil || !errors.Is(err, fs.ErrExist) || try >= 100 {
			if err != nil {
				// Name the file being written, not the temporary one.
				var pe *fs.PathError
				if errors.As(err, &pe) {
					err = &fs.PathError{Op: "create", Path: path, Err: pe.Err}
				}
			}

			return f, err
		}
	}
}

// newRecordAttributes is kind's attributes for a new file holding
// records, with RecordSize the longest record (a fixed-length record
// file's is its record size).
func newRecordAttributes(kind RecordKind, records [][]byte) ondisk.RecAttr {
	attr := recordKindAttributes[kind]
	if attr.Format == ondisk.RecordFormatFixed {
		return attr
	}

	for _, rec := range records {
		if len(rec) > int(attr.RecordSize) {
			attr.RecordSize = uint16(min(len(rec), 0xFFFF))
		}
	}

	return attr
}

// putRecords writes records to a file just created, and closes it.
func putRecords(f *volume.File, records [][]byte) error {
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
