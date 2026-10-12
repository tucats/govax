package rms

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/vmstime"
	"github.com/tucats/ods2/volume"
)

// What F$FILE_ATTRIBUTES reports of a file (Phase 51): the fields of its
// file header, as RMS's XABs would give them to a program that opened
// it. The header lives in ods2's types, which only this package may
// import, so FileAttributes copies what the lexical function needs into
// plain values.

// File characteristic bits of an ODS-2 header (FH2$L_FILECHAR) that ods2
// doesn't name: read check and write check.
const (
	fchReadCheck  uint32 = 0x08
	fchWriteCheck uint32 = 0x10
)

// Journaling bits of an ODS-2 header (FH2$B_JOURNAL): recovery unit,
// before-image, and after-image journaling.
const (
	journalRU uint8 = 0x02
	journalBI uint8 = 0x04
	journalAI uint8 = 0x08
)

// FileAttributes is one file's attributes, as F$FILE_ATTRIBUTES returns
// them (the DCL Dictionary's Table DCLI-5).
type FileAttributes struct {
	// Name is the file's full name, "DUA0:[WORK]LOGIN.COM;3", or the
	// host path; Device the volume's device name, "DUA0", "" for a host
	// file.
	Name   string
	Device string

	// FID and DID are the file's and its directory's file IDs, written
	// "(7,1,0)"; "" for a host file.
	FID, DID string

	// Organization and Format are FAT$B_RTYPE's halves: the file
	// organization (0 sequential, 1 relative, 2 indexed) and the record
	// format (FAB$C_ codes: 1 FIX, 2 VAR, 3 VFC, 4 STM, 5 STMLF, 6 STMCR;
	// 0 undefined). RecordAttributes is FAT$B_RATTRIB (FAB$M_FTN, CR,
	// PRN, BLK).
	Organization     uint8
	Format           uint8
	RecordAttributes uint8

	// RecordSize is FAB$W_MRS, the largest record allowed (FAT$W_MAXREC,
	// 0 for no limit); LongestRecord XAB$W_LRL, the longest record
	// written to a sequential file (FAT$W_RSIZE).
	RecordSize    uint16
	LongestRecord uint16
	Allocated     uint32 // XAB$L_ALQ: blocks allocated
	EndBlock      uint32 // XAB$L_EBK: the end-of-file block
	FirstFreeByte uint16 // XAB$W_FFB
	BucketSize    uint8  // XAB$B_BKZ
	FixedControl  uint8  // FAB$B_FSZ: a VFC record's fixed control bytes
	Extend        uint16 // XAB$W_DXQ
	GlobalBuffers uint16 // FAB$W_GBC
	VersionLimit  uint16 // 0 for none

	// Characteristics are the header's FH2$L_FILECHAR bits; Journal its
	// FH2$B_JOURNAL bits.
	Characteristics uint32
	Journal         uint8

	// Owner is the owner's UIC, group in the high word and member in the
	// low; Protection the file's protection mask (a bit set denies).
	Owner      uint32
	Protection uint16

	// Revision is the revision count; the times are VMS times (100 ns
	// since 17-NOV-1858), 0 for one never set.
	Revision                           uint16
	Created, Revised, Expires, Backup uint64
}

// Used is how many blocks the file's data uses, as DIRECTORY/SIZE counts
// them: the end-of-file block, less one when the data ends at that
// block's start.
func (a FileAttributes) Used() uint32 {
	if a.EndBlock > 0 && a.FirstFreeByte == 0 {
		return a.EndBlock - 1
	}

	return a.EndBlock
}

// Directory, Contiguous, ContiguousBestTry, Locked, NoBackup, Erase,
// ReadCheck, and WriteCheck are the file characteristics.
func (a FileAttributes) Directory() bool         { return a.Characteristics&ondisk.FchDirectory != 0 }
func (a FileAttributes) Contiguous() bool        { return a.Characteristics&ondisk.FchContig != 0 }
func (a FileAttributes) ContiguousBestTry() bool { return a.Characteristics&ondisk.FchContigB != 0 }
func (a FileAttributes) Locked() bool            { return a.Characteristics&ondisk.FchLocked != 0 }
func (a FileAttributes) NoBackup() bool          { return a.Characteristics&ondisk.FchNoBackup != 0 }
func (a FileAttributes) Erase() bool             { return a.Characteristics&ondisk.FchErase != 0 }
func (a FileAttributes) ReadCheck() bool         { return a.Characteristics&fchReadCheck != 0 }
func (a FileAttributes) WriteCheck() bool        { return a.Characteristics&fchWriteCheck != 0 }

// RecoveryUnit, BeforeImage, and AfterImage are the journaling bits.
func (a FileAttributes) RecoveryUnit() bool { return a.Journal&journalRU != 0 }
func (a FileAttributes) BeforeImage() bool  { return a.Journal&journalBI != 0 }
func (a FileAttributes) AfterImage() bool   { return a.Journal&journalAI != 0 }

// FileAttributes returns the attributes of the one file spec names, as
// F$FILE_ATTRIBUTES reads them. Where the file is, on the host or on a
// mounted volume, follows Locate's rules. On a volume, a field spec
// leaves out is not filled in by a default (no type is an empty type),
// and the newest version is the one unless spec gives another. A name
// with wildcards is a *DCLFileError with RMS$_WLD, and one that finds no
// file is NotFound's. A host file has the attributes govax gives a host
// file it reads as text: stream-LF records, carriage-return carriage
// control, its size in blocks, and its modification time as each date.
func (s *Session) FileAttributes(spec string) (FileAttributes, error) {
	loc, err := s.Locate(spec, false)
	if err != nil {
		return FileAttributes{}, err
	}

	if loc.Host {
		return hostFileAttributes(loc.Name)
	}

	if strings.ContainsAny(spec, "*%") || strings.Contains(spec, "...") {
		name, err := s.ExpandName(spec)
		if err != nil {
			name = strings.ToUpper(spec)
		}

		return FileAttributes{}, newDCLFileError(name, rmsWildcardError)
	}

	var (
		attrs FileAttributes
		found bool
	)

	err = s.firstSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return &NotFoundError{Spec: spec}
		}

		for _, m := range matches {
			if (r.Spec.Name == "" && m.Name != "") || (r.Spec.Type == "" && m.Type != "") {
				continue
			}

			f, err := vol.OpenFID(m.Fid)
			if err != nil {
				return err
			}

			attrs = headerAttributes(f.Header)
			attrs.Name = r.Display + ":" + dirSpec{Elems: m.Dirs}.String() + m.ShortName(';')
			attrs.Device = r.Spec.Device
			attrs.FID = m.Fid.String()
			found = true

			return nil
		}

		return &NotFoundError{Spec: spec}
	})

	var notFound *NotFoundError

	switch {
	case found:
		return attrs, nil
	case err == nil || errors.As(err, &notFound):
		return FileAttributes{}, s.NotFound(spec)
	default:
		return FileAttributes{}, err
	}
}

// ProtectionText is the file's protection as F$FILE_ATTRIBUTES's PRO
// writes it: each category's access, "SYSTEM=RWED, OWNER=RWED, GROUP=RE,
// WORLD=".
func (a FileAttributes) ProtectionText() string {
	categories := []string{"SYSTEM", "OWNER", "GROUP", "WORLD"}
	parts := make([]string, len(categories))

	for i, name := range categories {
		parts[i] = name + "=" + ondisk.ProtectionAccess(a.Protection, i)
	}

	return strings.Join(parts, ", ")
}

// headerAttributes copies a file header's fields into a FileAttributes.
func headerAttributes(h ondisk.FileHeader) FileAttributes {
	ra := h.RecordAttributes

	a := FileAttributes{
		DID:              h.Backlink.String(),
		Organization:     uint8(ra.Format) >> 4,
		Format:           uint8(ra.Format) & 0x0F,
		RecordAttributes: ra.Attributes,
		RecordSize:       ra.MaxRecordSize,
		LongestRecord:    ra.RecordSize,
		Allocated:        ra.HighestBlock,
		EndBlock:         ra.EndOfFileBlock,
		FirstFreeByte:    ra.FirstFreeByte,
		BucketSize:       ra.BucketSize,
		FixedControl:     ra.VfcSize,
		Extend:           ra.DefaultExtend,
		GlobalBuffers:    ra.GlobalBufferCount,
		VersionLimit:     ra.VersionLimit,
		Characteristics:  h.FileCharacteristics,
		Journal:          h.Journaling,
		Owner:            uint32(h.Owner.Group)<<16 | uint32(h.Owner.Member),
		Protection:       h.FileProtection,
	}

	if ident, err := h.Ident(); err == nil {
		a.Revision = ident.Revision
		a.Created = uint64(ident.CreationDate)
		a.Revised = uint64(ident.RevisionDate)
		a.Expires = uint64(ident.ExpirationDate)
		a.Backup = uint64(ident.BackupDate)
	}

	return a
}

// hostFileAttributes is the attributes of the host file at path (see
// Session.FileAttributes). A file that isn't there is RMS$_FNF.
func hostFileAttributes(path string) (FileAttributes, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return FileAttributes{}, newDCLFileError(path, hostOpenStatus(err))
	}

	if err != nil {
		return FileAttributes{}, err
	}

	size := info.Size()
	when := uint64(vmstime.FromTime(info.ModTime()))

	a := FileAttributes{
		Name:             path,
		Format:           uint8(ondisk.RecordFormatStreamLF),
		RecordAttributes: ondisk.AttrImpliedCC,
		Allocated:        uint32((size + 511) / 512),
		EndBlock:         uint32(size/512) + 1,
		FirstFreeByte:    uint16(size % 512),
		Owner:            0x00010004, // [1,4], SYSTEM's, as the console's process
		Created:          when,
		Revised:          when,
	}

	if info.IsDir() {
		a.Characteristics |= ondisk.FchDirectory
	}

	return a, nil
}
