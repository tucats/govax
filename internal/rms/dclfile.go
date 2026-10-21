package rms

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// The files DCL's OPEN opens (docs/PHASE-50 - DCL command procedures.md,
// subtask 17): what READ, WRITE, and CLOSE need of RMS, called from Go
// rather than through a FAB and a RAB in a process's memory.
//
// A DCL file on a volume is opened as $OPEN or $CREATE opens one, by the
// same code: name processing (the default type .DAT, logical names,
// search lists), the file's list of RMS openers and the file system's
// access (sharing.go), so a file DCL has open conflicts with a program's
// open of it as it would on VMS, and DISMOUNT sees it open. Its record
// stream is a FileHandle's (stream.go): READ is a sequential $GET, WRITE
// a $PUT, which must be at the end of the file (RMS$_NEF otherwise), and
// WRITE/UPDATE an $UPDATE of the record READ read last, the same length.
//
// What the DCL Dictionary says of OPEN's files:
//
//   - /READ (the default) opens an existing file to read; /WRITE makes a
//     new file (the next version) to write, or with /READ opens an
//     existing one to read and write, at its start; /APPEND opens an
//     existing file to write at its end.
//   - /SHARE=READ lets others read the file, and /SHARE (or
//     /SHARE=WRITE) read and write it. The manual says that without
//     /SHARE others may read it, but VMS 7.3 kept every other opener out
//     of a file OPEN/WRITE had open, a /SHARE reader and TYPE too, while
//     two /SHARE=READ readers shared one (testdata/dcl50's F9): without
//     /SHARE the sharing is RMS's default, SHRGET for a reader and none
//     for a writer.
//   - Every open that writes may read and update too: VMS 7.3's READ of
//     a file OPEN/WRITE had just made was RMS$_EOF, not RMS$_FAC, and its
//     WRITE/UPDATE RMS$_CUR (F4, F5).
//   - A file OPEN makes has variable records with two bytes of fixed
//     control (VFC) and print-file carriage control (RAT=PRN), as VMS
//     7.3's DIRECTORY/FULL showed (F1). Each record's fixed control is
//     "one line feed before, a carriage return after" (01 8D:
//     unconfirmed, DUMP/RECORD didn't show it), and isn't part of a
//     record READ returns.
//
// A host file is lines of text, read and written as the host's text files
// are (recordfile.go's TextRecords): READ reads a line, WRITE adds one
// at the end, and WRITE/UPDATE writes a line of the same length over the
// one READ read. Host files have no versions: OPEN/WRITE replaces one.

// DCLAccess is how OPEN opens a file: its /READ, /WRITE, and /APPEND, and
// /SHARE, which lets other openers read the file (ShareRead) or read and
// write it (ShareWrite).
type DCLAccess struct {
	Read, Write, Append   bool
	ShareRead, ShareWrite bool
}

// creates reports whether the open makes a new file: /WRITE without
// /READ.
func (a DCLAccess) creates() bool { return a.Write && !a.Read && !a.Append }

// Output reports whether the open is for output (OPENOUT's "as
// output"): only one that makes a file. VMS 7.3 reported an /APPEND or
// /READ/WRITE of a file that isn't there with OPENIN (F3).
func (a DCLAccess) Output() bool { return a.creates() }

// fac is the open's FAB$B_FAC: GET to read, and GET, PUT, and UPD for
// any open that writes (see the top of this file).
func (a DCLAccess) fac() byte {
	if a.Write || a.Append {
		return facGet | facPut | facUpd
	}

	return facGet
}

// shr is the open's FAB$B_SHR: GET for /SHARE=READ, GET, PUT, UPD, and
// DEL for /SHARE, and none without /SHARE (RMS's default).
func (a DCLAccess) shr() byte {
	switch {
	case a.ShareWrite:
		return shrGet | shrPut | shrUpd | shrDel
	case a.ShareRead:
		return shrGet
	}

	return 0
}

// DCLFileError is an OPEN that failed: Status is the RMS status, and STV
// its secondary status, if any; Name the file as name processing
// expanded it ("DUA0:[WORK]NOSUCH.DAT;"), or as it was given when it
// couldn't be expanded. VMS 7.3 showed SS$_NOSUCHFILE under RMS$_DNF,
// and nothing under RMS$_FNF (F3), so only RMS$_DNF has one.
type DCLFileError struct {
	Name   string
	Status uint32
	STV    uint32
}

// newDCLFileError returns the error for an OPEN of name that failed with
// RMS status sts.
func newDCLFileError(name string, sts uint32) *DCLFileError {
	e := &DCLFileError{Name: name, Status: sts}
	if sts == rmsDirNotFound {
		e.STV = ssNoSuchFile
	}

	return e
}

func (e *DCLFileError) Error() string {
	return fmt.Sprintf("rms: opening %s: status %%X%08X", e.Name, e.Status)
}

// DCLFile is a file DCL's OPEN has open: on a volume, a FileHandle, or on
// the host, an os.File.
type DCLFile struct {
	// Name is the file's full name: "DUA0:[WORK]DATA.DAT;3", or the host
	// path.
	Name string

	// Device is the volume's device name, "DUA0", "" for a host file:
	// the logical name OPEN defines is equated to its physical name.
	Device string

	ctx    *Context
	handle *FileHandle
	ifi    uint16

	host *hostDCLFile
}

// OpenDCLFile opens the file at loc as DCL's OPEN does, with access a. A
// failure is a *DCLFileError.
func (s *Session) OpenDCLFile(loc FileLocation, a DCLAccess) (*DCLFile, error) {
	if loc.Host {
		return openHostDCLFile(loc.Name, a)
	}

	ctx := &Context{Mounts: s.Mounts, Logicals: s.Logicals, Session: s, Files: NewFileTable(nil)}

	fail := func(name string, sts uint32) (*DCLFile, error) {
		return nil, newDCLFileError(name, sts)
	}

	names, sts := ctx.expandName(nameInputs{Primary: loc.Name, Default: ".DAT"})
	if sts != 0 {
		return fail(strings.ToUpper(loc.Name), sts)
	}

	fac, shr := a.fac(), a.shr()

	var (
		handle *FileHandle
		ifi    uint16
		found  foundFile
		name   string
	)

	// A search list opens the first element's file that exists, as
	// $OPEN does; a new file goes in the first element's directory.
	for _, p := range names {
		name = p.String()

		if p.FNB&fnbWildcard != 0 {
			return fail(name, rmsWildcardError)
		}

		if a.creates() {
			handle, found, sts = createDCLFile(ctx, fac, shr, p)
			if sts != 0 {
				return fail(name, sts)
			}

			ifi = ctx.Files.Alloc(handle)

			break
		}

		var err error

		ifi, found, sts, err = openOnVolume(ctx, fac, shr, p)
		if err != nil {
			return nil, err
		}

		if sts == 0 {
			handle, _ = ctx.Files.Lookup(ifi)

			break
		}

		if sts != rmsFileNotFound && sts != rmsDeviceNotReady && sts != rmsDirNotFound {
			break
		}
	}

	if handle == nil {
		return fail(name, sts)
	}

	f := &DCLFile{Name: found.resultant().String(), Device: found.Device, ctx: ctx, handle: handle, ifi: ifi}

	if sts := f.arm(fac, a.Append); sts != 0 {
		_ = f.Close()

		return fail(f.Name, sts)
	}

	return f, nil
}

// String is the whole specification.
func (f fileName) String() string {
	return f.Dev + f.Dir + f.Name + f.Type + f.Ver
}

// arm connects f's record stream, as $CONNECT does (connect.go's
// armForFAC): at the first record, or with appending at the end of the
// file.
func (f *DCLFile) arm(fac byte, appending bool) uint32 {
	var rop uint32
	if appending {
		rop = ropEOF
	}

	return f.handle.arm(fac, rop)
}

// dclVFCSize is how many bytes of fixed control a record of a file OPEN
// makes has.
const dclVFCSize = 2

// dclRecordControl is the fixed control Put gives a record of a file
// with two bytes of it: a line feed before the record, and a carriage
// return after it (see the top of this file).
var dclRecordControl = []byte{0x01, 0x8D}

// dclRatPrint is FAT$M_PRINTCC (RAT=PRN): print-file carriage control.
const dclRatPrint = ondisk.AttrPrintCC
// createDCLFile makes the new file OPEN/WRITE opens, as $CREATE makes one
// (create.go's createOnVolume): the next version, unless p names one, a
// sequential file of VFC records with print-file carriage control. It returns the file's handle, opened
// for access fac with sharing shr, and the file made, or the RMS status
// for why not.
func createDCLFile(ctx *Context, fac, shr byte, p parsedName) (*FileHandle, foundFile, uint32) {
	var found foundFile

	spec := p.spec()

	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return nil, found, rmsDeviceNotReady
	}

	if !ctx.Mounts.Writable(spec.Device) {
		return nil, found, rmsPrivilegeViolation
	}

	if spec.Name == "" && spec.Type == "" {
		return nil, found, rmsFileNameError
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return nil, found, rmsDirNotFound
	}

	name := specFileName(spec)

	version, ok := parseOpenVersion(spec.Version)
	if !ok {
		return nil, found, rmsInvalidVersion
	}

	if version == 0 {
		if version, err = dir.NextVersion(name); err != nil {
			return nil, found, rmsDeviceError
		}
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		return nil, found, rmsDeviceError
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		return nil, found, rmsDeviceError
	}

	attr := ondisk.RecAttr{Format: ondisk.RecordFormatVFC, Attributes: dclRatPrint, VfcSize: dclVFCSize}

	file, err := vol.CreateFileVersion(dir, name, version, attr, bm, ib)
	if errors.Is(err, volume.ErrExists) {
		return nil, found, rmsFileExists
	}

	if err != nil {
		return nil, found, rmsDeviceError
	}

	mode := accessMode(fac, shr)

	a, err := vol.AccessFile(file, mode)
	if err != nil {
		return nil, found, rmsDeviceError
	}

	n, t := splitEntryName(name)
	found = foundFile{
		Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
		Name: n, Type: t, Version: version,
		FID: file.Header.Fid, DID: dir.Header.Fid,
	}

	h := &FileHandle{
		File: file, Accessor: a, Mode: mode, Share: effectiveSharing(fac, shr), Writable: true, Access: fac,
		claim: ctx.Mounts.claimOpen(spec.Device, fileIDFrom(file.Header.Fid), fac, shr),
	}

	return h, found, 0
}

// vfcSize is how many bytes of fixed control each of h's records has: 0
// unless its file is VFC.
func (h *FileHandle) vfcSize() int {
	attr := h.File.Header.RecordAttributes
	if attr.Format != ondisk.RecordFormatVFC {
		return 0
	}

	return int(attr.VfcSize)
}

// Get reads f's next record, as a sequential $GET does, without any fixed
// control it has, and returns it with RMS$_NORMAL, or nil and the status
// for why not: RMS$_EOF at the end of the file, RMS$_FAC for a file not
// opened to read.
func (f *DCLFile) Get() ([]byte, uint32) {
	if f.host != nil {
		return f.host.get()
	}

	h := f.handle
	s := &h.stream

	if h.Reader == nil || s.fac&facReads == 0 {
		s.lost()

		return nil, rmsFACNotAllowed
	}

	target := s.next
	if s.atEnd {
		target = h.endOfFile()
	}

	record, end, sts := h.recordAt(target)
	if sts != 0 {
		s.lost()

		if sts == rmsEOF {
			s.next = target
		}

		return nil, sts
	}

	h.readBuffer.touch(f.ctx, target)

	s.current, s.hasCurrent, s.afterFind = target, true, false
	s.next, s.atEnd = end, false

	if n := h.vfcSize(); len(record) >= n {
		record = record[n:]
	}

	return record, rmsNormal
}

// Put writes record at the end of f, as a $PUT does, with
// dclRecordControl as two bytes of fixed control (zeros for any other
// size), and returns RMS$_NORMAL or the status for why not:
// RMS$_FAC for a file not opened to write, RMS$_NEF when the stream isn't
// at the end of the file (a /READ/WRITE file READ hasn't read to its
// end), RMS$_RSZ for a record the file can't hold.
func (f *DCLFile) Put(record []byte) uint32 {
	if f.host != nil {
		return f.host.put(record)
	}

	h := f.handle
	s := &h.stream

	if h.Writer == nil {
		s.lost()

		return rmsFACNotAllowed
	}

	if !h.atEndOfFile() {
		s.lost()

		return rmsNotAtEOF
	}

	h.wrote(f.ctx, h.endOfFile())

	control := make([]byte, h.vfcSize())
	if len(control) == len(dclRecordControl) {
		copy(control, dclRecordControl)
	}

	if err := h.Writer.Put(append(control, record...)); err != nil {
		return rmsRecordTooBig
	}

	h.noteLongest(len(record))

	s.lost()
	s.atEnd = true

	return rmsNormal
}

// Update writes record over the record Get read last, as an $UPDATE does
// (recordops.go's SysUpdate), keeping its fixed control, and returns
// RMS$_NORMAL or the status for why not: RMS$_FAC for a file not opened
// to read and write, RMS$_CUR with no record read, RMS$_RSZ for a record
// of another length (a stream file's may differ, as $UPDATE's may).
func (f *DCLFile) Update(record []byte) uint32 {
	if f.host != nil {
		return f.host.update(record)
	}

	h := f.handle
	s := &h.stream

	switch {
	case s.fac&facUpd == 0 || h.Reader == nil:
		s.lost()

		return rmsFACNotAllowed
	case !s.hasCurrent:
		return rmsNoCurrent
	}

	old, _, sts := h.recordAt(s.current)
	if sts != 0 {
		s.lost()

		return rmsNoCurrent
	}

	n := h.vfcSize()
	data := append(bytes.Clone(old[:min(n, len(old))]), record...)

	terminator, stream := streamTerminator(h.File)
	if len(data) != len(old) {
		if !stream {
			s.lost()

			return rmsRecordTooBig
		}

		data = append(data, terminator...)
		if s.current+int64(len(data)) > h.endOfFile() {
			s.lost()

			return rmsRecordTooBig
		}
	}

	h.wrote(f.ctx, s.current)

	if err := writeAt(h.File, s.current+recordFraming(h.File), data); err != nil {
		s.lost()

		return rmsDeviceError
	}

	s.lost()

	return rmsNormal
}

// Close closes f, as $CLOSE does: what it wrote goes to the volume, and
// it leaves the file's openers.
func (f *DCLFile) Close() error {
	if f.host != nil {
		return f.host.file.Close()
	}

	defer f.ctx.Files.Release(f.ifi)

	return closeVolumeFile(f.handle, nil)
}

// hostDCLFile is a host file DCL has open: lines of text. pos is the
// offset of the next line Get reads, and current the offset and length
// of the last one it read.
type hostDCLFile struct {
	file   *os.File
	reader *bufio.Reader

	readable, writable, updatable bool

	pos           int64
	current       int64
	currentLength int
	hasCurrent    bool
	atEnd         bool
}

// openHostDCLFile opens the host file at path as OPEN does with access a:
// /WRITE alone makes it (or empties one that is there), /APPEND and
// /READ/WRITE need it to exist. Every open that writes may read, and
// /APPEND starts at the end of the file.
func openHostDCLFile(path string, a DCLAccess) (*DCLFile, error) {
	var (
		file *os.File
		err  error
	)

	switch {
	case a.creates():
		file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	case a.Write || a.Append:
		file, err = os.OpenFile(path, os.O_RDWR, 0)
	default:
		file, err = os.Open(path)
	}

	if err != nil {
		return nil, newDCLFileError(path, hostOpenStatus(err))
	}

	fac := a.fac()
	h := &hostDCLFile{file: file, readable: fac&facGet != 0, writable: fac&facPut != 0, updatable: fac&facUpd != 0}

	h.reader = bufio.NewReader(file)

	// A new file is empty: its start is its end.
	h.atEnd = a.creates()

	if a.Append {
		if h.pos, err = file.Seek(0, io.SeekEnd); err != nil {
			_ = file.Close()

			return nil, newDCLFileError(path, rmsDeviceError)
		}

		h.reader.Reset(file)
		h.atEnd = true
	}

	return &DCLFile{Name: path, host: h}, nil
}

// hostOpenStatus is the RMS status for a host file that couldn't be
// opened: RMS$_FNF for one that isn't there, RMS$_DNF when its directory
// isn't, RMS$_PRV for one the host won't let govax open.
func hostOpenStatus(err error) uint32 {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return rmsPrivilegeViolation
	case errors.Is(err, fs.ErrNotExist):
		var pe *fs.PathError
		if errors.As(err, &pe) {
			if dir := parentDir(pe.Path); dir != "" {
				if _, statErr := os.Stat(dir); statErr != nil {
					return rmsDirNotFound
				}
			}
		}

		return rmsFileNotFound
	}

	return rmsDeviceError
}

// parentDir is path's directory, "" for none.
func parentDir(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i <= 0 {
		return ""
	}

	return path[:i]
}

func (h *hostDCLFile) get() ([]byte, uint32) {
	if !h.readable {
		h.hasCurrent = false

		return nil, rmsFACNotAllowed
	}

	line, err := h.reader.ReadBytes('\n')
	if len(line) == 0 && err != nil {
		h.hasCurrent = false
		h.atEnd = true

		if errors.Is(err, io.EOF) {
			return nil, rmsEOF
		}

		return nil, rmsDeviceError
	}

	h.current, h.hasCurrent = h.pos, true
	h.pos += int64(len(line))
	h.atEnd = false

	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	h.currentLength = len(line)

	return line, rmsNormal
}

// put adds record at the end of the file: for a file also opened to
// read, only once Get has reached the end (RMS$_NEF otherwise).
func (h *hostDCLFile) put(record []byte) uint32 {
	if !h.writable {
		h.hasCurrent = false

		return rmsFACNotAllowed
	}

	h.hasCurrent = false

	if h.readable {
		if !h.atEnd {
			return rmsNotAtEOF
		}

		end, err := h.file.Seek(0, io.SeekEnd)
		if err != nil {
			return rmsDeviceError
		}

		h.pos = end
	}

	if _, err := h.file.Write(append(bytes.Clone(record), hostLineEnd...)); err != nil {
		return rmsDeviceError
	}

	if h.readable {
		h.pos += int64(len(record) + len(hostLineEnd))
		h.reader.Reset(h.file)
	}

	return rmsNormal
}

// update writes record over the line Get read last, which must be its
// length.
func (h *hostDCLFile) update(record []byte) uint32 {
	switch {
	case !h.updatable:
		h.hasCurrent = false

		return rmsFACNotAllowed
	case !h.hasCurrent:
		return rmsNoCurrent
	case len(record) != h.currentLength:
		h.hasCurrent = false

		return rmsRecordTooBig
	}

	h.hasCurrent = false

	if _, err := h.file.WriteAt(record, h.current); err != nil {
		return rmsDeviceError
	}

	// What the reader read ahead may be what was just written over.
	if _, err := h.file.Seek(h.pos, io.SeekStart); err != nil {
		return rmsDeviceError
	}

	h.reader.Reset(h.file)

	return rmsNormal
}
