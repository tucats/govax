package rms

import (
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/vmstime"
	"github.com/tucats/ods2/volume"
)

// File attributes through the ACP (docs/planning/PHASE-26.md subtask 42).
//
// # What "attributes" are
//
// Everything VMS knows about a file, apart from its data, lives in the
// file's *header*: a 512-byte block in the volume's index file
// (INDEXF.SYS). The header records the file's record format and end of
// file (the 32-byte "record attribute area", which VMS's $FATDEF
// describes and RMS reads to know how to split the file into records),
// its characteristics (contiguous, directory, marked for deletion, ...),
// its owner (a UIC) and protection mask, its name, and its dates
// (created, revised, expires, backed up).
//
// A program reaches these through the disk's $QIO functions with an
// *attribute list* (VMS's $ATRDEF): IO$_ACCESS reads the attributes the
// list names; IO$_MODIFY, IO$_DEACCESS, and IO$_CREATE write them. RMS
// itself depends on this: every time it closes a file it has written, it
// writes the record attribute area back with IO$_DEACCESS, which is how
// a file's exact end of file (down to the byte) gets recorded.
//
// # How govax does it
//
// ACPAttributes is the attributes as plain Go values, in the forms the
// attribute list's buffers hold them (the record attribute area as its 32
// encoded bytes, a UIC as a longword, dates as 64-bit VMS times).
// internal/corevms's disk driver moves them between those buffers and this
// struct; this file moves them between the struct and a header, through
// ods2's volume.UpdateHeader.
//
// Not everything is a program's to set. The allocation (FAT$L_HIBLK, the
// number of blocks the header's map describes) stays whatever the map
// says (UpdateHeader enforces that), and the characteristics' directory
// and marked-for-deletion bits stay as the ACP set them (a program can't
// turn a file into a directory by writing a longword). The name, the
// whole header, the back link to the directory, and the high-water mark
// can be read but not written.

// ACPAttributes is a file's attributes, as its header records them.
type ACPAttributes struct {
	// RecordAttributes is the 32-byte record attribute area ($FATDEF):
	// record format, record size, the allocation and end of file, and
	// so on (ATR$C_RECATTR).
	RecordAttributes [ondisk.RecAttrSize]byte

	// Characteristics is the file characteristics longword (FCH$ bits,
	// ATR$C_UCHAR).
	Characteristics uint32

	// Protection is the protection mask (ATR$C_FPRO): four 4-bit fields
	// for system, owner, group, and world, a set bit denying access.
	Protection uint16

	// Owner is the owner's UIC (ATR$C_UIC): group in the high word,
	// member in the low.
	Owner uint32

	// The dates, as 64-bit VMS binary times (100 ns units since
	// 17-NOV-1858): created, revised, expires, and backed up
	// (ATR$C_CREDATE, REVDATE, EXPDATE, BAKDATE).
	Created, Revised, Expires, Backup uint64

	// Read only.

	// Name is the file's name as its header records it (ATR$C_ASCNAME):
	// "NAME.TYP;VER" on a volume VMS wrote, "NAME.TYP" on one ods2 wrote
	// (it leaves the version to the directory entry).
	Name string

	// Header is the whole header block as the disk holds it
	// (ATR$C_HEADER).
	Header [ondisk.BlockSize]byte

	// Backlink is the file ID of the directory the file was created in
	// (ATR$C_BACKLINK).
	Backlink FileID

	// HighWater is the high-water mark: the first virtual block never
	// written (ATR$C_HIGHWATER).
	HighWater uint32

	// ACLLength is the length in bytes of the file's access control list
	// (ATR$C_ACLLENGTH): 0 for none, 24 for two identifier ACEs (VMS
	// 7.1: the probe's FACLTWO).
	ACLLength uint32
}

// protectedCharacteristics are the characteristics bits only the ACP
// sets: a written ATR$C_UCHAR keeps the header's own values of these.
const protectedCharacteristics = ondisk.FchDirectory | ondisk.FchMarkDel

// attributesOf returns h's attributes. A header whose IDENT area can't be
// decoded (it has none) has an empty name and zero dates.
func attributesOf(h ondisk.FileHeader) ACPAttributes {
	a := ACPAttributes{
		Characteristics: h.FileCharacteristics,
		Protection:      h.FileProtection,
		Owner:           uint32(h.Owner.Group)<<16 | uint32(h.Owner.Member),
		Backlink:        fileIDFrom(h.Backlink),
		HighWater:       h.HighWaterMark,
	}

	if acl, err := h.ACL(); err == nil {
		a.ACLLength = uint32(len(acl))
	}

	copy(a.RecordAttributes[:], ondisk.EncodeRecAttr(h.RecordAttributes))
	copy(a.Header[:], h.Raw())

	if id, err := h.Ident(); err == nil {
		a.Name = strings.TrimRight(id.Filename+id.FilenameExtension, " ")
		a.Created = uint64(id.CreationDate)
		a.Revised = uint64(id.RevisionDate)
		a.Expires = uint64(id.ExpirationDate)
		a.Backup = uint64(id.BackupDate)
	}

	return a
}

// updateAttributes rewrites f's header after change has edited its
// attributes: the writable ones are copied back into the header, the
// read-only ones ignored.
func updateAttributes(f *volume.File, change func(*ACPAttributes)) error {
	a := attributesOf(f.Header)
	change(&a)

	ra, err := ondisk.DecodeRecAttr(a.RecordAttributes[:])
	if err != nil {
		return err
	}

	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, id *ondisk.Ident) {
		h.RecordAttributes = ra // UpdateHeader keeps the allocation
		h.FileCharacteristics = h.FileCharacteristics&protectedCharacteristics | a.Characteristics&^protectedCharacteristics
		h.FileProtection = a.Protection
		h.Owner = ondisk.Uic{Group: uint16(a.Owner >> 16), Member: uint16(a.Owner)}
		id.CreationDate = vmstime.VMSTime(a.Created)
		id.RevisionDate = vmstime.VMSTime(a.Revised)
		id.ExpirationDate = vmstime.VMSTime(a.Expires)
		id.BackupDate = vmstime.VMSTime(a.Backup)
	})
}

// ACPReadAttributes returns the attributes of the file with ID fid on the
// volume mounted on device, without accessing it (IO$_ACCESS without
// IO$M_ACCESS: how DIRECTORY/FULL reads a file's details).
func (t *MountTable) ACPReadAttributes(device string, fid FileID) (ACPAttributes, error) {
	return t.ACPReadAttributesAs(security.SystemCredentials(), device, fid)
}

// ACPReadAttributesAs is ACPReadAttributes for the process with
// credentials creds, which the file must give R or C (ErrNoPrivilege):
// VMS 7.1's ACP refused the probe's accounts the attributes of files
// giving them neither, and gave an owner those of a file giving it no
// access, its control enough (the probe's A.4, "The check").
func (t *MountTable) ACPReadAttributesAs(creds security.Credentials, device string, fid FileID) (ACPAttributes, error) {
	vol, ok := t.Lookup(device)
	if !ok {
		return ACPAttributes{}, ErrACPNotMounted
	}

	f, err := vol.OpenFID(fid.toOds2())
	if err != nil {
		return ACPAttributes{}, ErrACPNoSuchFile
	}

	if !mayEither(creds, vol, &f.Header, security.Read, security.Control) {
		return ACPAttributes{}, ErrNoPrivilege
	}

	return attributesOf(f.Header), nil
}

// ACPWriteAttributes changes the attributes of the file with ID fid on
// the volume mounted on device, without accessing it (IO$_MODIFY on a
// channel with no file accessed). The volume must be mounted writable.
func (t *MountTable) ACPWriteAttributes(device string, fid FileID, change func(*ACPAttributes)) error {
	return t.ACPWriteAttributesAs(security.SystemCredentials(), device, fid, change)
}

// ACPWriteAttributesAs is ACPWriteAttributes for the process with
// credentials creds, by attributeChangeAllowed's rules.
func (t *MountTable) ACPWriteAttributesAs(creds security.Credentials, device string, fid FileID, change func(*ACPAttributes)) error {
	vol, ok := t.Lookup(device)
	if !ok {
		return ErrACPNotMounted
	}

	if !t.Writable(device) {
		return ErrACPWriteLocked
	}

	f, err := vol.OpenFID(fid.toOds2())
	if err != nil {
		return ErrACPNoSuchFile
	}

	if err := attributeChangeAllowed(creds, vol, f, change, false); err != nil {
		return err
	}

	return updateAttributes(f, change)
}

// attributeChangeAllowed reports whether creds let a process make change
// to the attributes of the file f on vol: ErrNoPrivilege if not. A new
// protection or owner needs control of the file, and a new owner one the
// process may give (security.CanChangeOwner); VMS 7.1's ACP refused both
// to the probe's accounts without control (the probe's A.4, "Changing
// profiles"). Any other attribute needs write access, which a file
// accessed for writing (accessed) already has (unconfirmed: the probe
// changed no other attribute of a file not accessed).
func attributeChangeAllowed(creds security.Credentials, vol *volume.Volume, f *volume.File, change func(*ACPAttributes), accessed bool) error {
	if unchecked(creds) {
		return nil
	}

	before := attributesOf(f.Header)
	after := before
	change(&after)

	profile := headerProfile(vol, &f.Header)

	if after.Protection != before.Protection || after.Owner != before.Owner {
		if !security.Check(creds, profile, security.Control).Granted {
			return ErrNoPrivilege
		}

		if after.Owner != before.Owner && !security.CanChangeOwner(creds, profile, security.UIC(after.Owner)) {
			return ErrNoPrivilege
		}
	}

	others := after
	others.Protection, others.Owner = before.Protection, before.Owner

	if !accessed && others != before && !security.Check(creds, profile, security.Write).Granted {
		return ErrNoPrivilege
	}

	return nil
}

// Attributes returns the accessed file's attributes.
func (a *ACPFile) Attributes() ACPAttributes {
	return attributesOf(a.file.Header)
}

// WriteAttributes changes the accessed file's attributes. It needs write
// access (ErrACPReadOnly).
func (a *ACPFile) WriteAttributes(change func(*ACPAttributes)) error {
	if !a.writable {
		return ErrACPReadOnly
	}

	if a.creds != nil {
		var vol *volume.Volume
		if a.mount != nil {
			vol = a.mount.Volume
		}

		if err := attributeChangeAllowed(*a.creds, vol, a.file, change, true); err != nil {
			return err
		}
	}

	return updateAttributes(a.file, change)
}
