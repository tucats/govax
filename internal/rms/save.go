package rms

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is docs/PHASE-69 - backup save sets.md's subtask 11, RMS's
// side of saving: finding the files an input specifier names on a
// mounted volume, each with the attributes and data a save set keeps of
// it, and making the save set's file. BACKUP's command (internal/dclcmd)
// selects among the files and lays out the save set (internal/backup).
//
// # The order of a save
//
// VMS's BACKUP goes through a directory in its entries' order (by name,
// each name's versions newest first), and in a [dir...] save goes into a
// subdirectory where its directory file's entry is: SUB1.DIR is saved
// after STMLF.TXT and before [B69.SUB1]'s files, and TEXT.TXT comes after
// them (testdata/backup69's log and B69.BCK). Every directory such a
// search goes into is saved as a file, whatever the input's
// file-selection qualifiers say: /BEFORE=TODAY saved only SUB1.DIR and
// SUB2.DIR (B69NONE.BCK), and /BY_OWNER=[200,201] saved them beside
// OWNED.TXT (B69BYO.BCK). A directory file a search without "..." matches
// by name is saved as a file, and not gone into.

// dirStatusSaved is a directory file's DIR_STATUS attribute, as every
// directory in the probe's save sets has it; what its bits mean isn't
// known.
const dirStatusSaved = 0x0F

// SaveFile is one file an input specifier found for a save.
type SaveFile struct {
	// Name is the file's full name, "DUA1:[B69]ACL.TXT;1", as BACKUP's
	// messages name it.
	Name string

	// Attributes are what the save set keeps of the file's header, its
	// directory entry, and its directory (backup.Attributes).
	Attributes backup.Attributes

	// Walked is true for a subdirectory a [dir...] search went into,
	// saved whatever the file selection says.
	Walked bool

	mounts *MountTable
	device string
	vol    *volume.Volume
	fid    ondisk.Fid
}

// Read reads the file's used blocks, VBN 1 up to its end of file (all
// its blocks for a relative or indexed file). The
// file is accessed for reading with others allowed to read it but not
// to write it, so a file some process has open for writing is
// ErrACPAccessConflict (VMS's ACCONFLICT); with ignoreInterlock (BACKUP's
// /IGNORE=INTERLOCK) it's read as it stands, and conflict says so.
func (f *SaveFile) Read(ignoreInterlock bool) (data []byte, conflict bool, err error) {
	a, err := f.mounts.ACPAccessWith(f.device, fileIDFrom(f.fid), ACPAccessMode{NoWrite: true})

	switch {
	case errors.Is(err, ErrACPAccessConflict) && ignoreInterlock:
		file, err := f.vol.OpenFID(f.fid)
		if err != nil {
			return nil, true, err
		}

		data, err := f.blocks(file)

		return data, true, err

	case err != nil:
		return nil, errors.Is(err, ErrACPAccessConflict), err
	}

	data, err = f.blocks(a.file)
	if dErr := a.Deaccess(); dErr != nil && err == nil {
		err = dErr
	}

	return data, false, err
}

// blocks reads file's used blocks, or for a relative or indexed file
// every block it has: VMS saved all 9 of INDEXED.IDX's, its end of file
// at the start of block 9 (whether it saves up to the end-of-file block
// or the allocation, the same here, is unconfirmed).
func (f *SaveFile) blocks(file *volume.File) ([]byte, error) {
	used := file.UsedBlocks()
	if f.Attributes.Organization() != 0 {
		used = file.Blocks()
	}
	data := make([]byte, int(used)*ondisk.BlockSize)

	for vbn := uint32(1); vbn <= used; vbn++ {
		start := int(vbn-1) * ondisk.BlockSize
		if err := file.ReadBlock(vbn, data[start:start+ondisk.BlockSize]); err != nil {
			return nil, fmt.Errorf("rms: reading block %d of %s: %w", vbn, f.Name, err)
		}
	}

	return data, nil
}

// ErrSaveFromHost is an input specifier that names host files: a save
// reads files on mounted volumes only.
var ErrSaveFromHost = errors.New("BACKUP saves files from a mounted volume only")

// SaveSearch finds the files spec names on a mounted volume, in the
// order VMS's BACKUP saves them, calling visit for each. An omitted name,
// type, or version is "*" (the manual's 7.3.3: "if you omit the version
// number, BACKUP processes all versions"). With "...", the search goes
// into every subdirectory below the directories spec names, each found
// first as a Walked file. A directory that isn't there is a
// *DCLFileError with RMS$_DNF, and a search without "..." that finds no
// file one with RMS$_FNF (NotFound's); a [dir...] search may find
// nothing, and calls visit for nothing. visit's error ends the search.
func (s *Session) SaveSearch(spec string, visit func(*SaveFile) error) error {
	found := false

	err := s.firstSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		pattern := r.Spec
		if pattern.Version == "" {
			pattern.Version = "*"
		}

		matches, err := filespec.Glob(vol, pattern)
		if err != nil || len(matches) == 0 && !pattern.Recursive {
			return &NotFoundError{Spec: spec}
		}

		search := &saveSearch{mounts: s.Mounts, device: r.Spec.Device, vol: vol, display: r.Display, recursive: pattern.Recursive, visit: visit, matched: map[string]bool{}}
		for _, m := range matches {
			search.matched[matchKey(m.Dirs, m.Name+"."+m.Type, m.Version)] = true
		}

		tops, err := s.saveTops(vol, pattern, matches)
		if err != nil {
			return err
		}

		found = true

		for _, top := range tops {
			if err := search.directory(top.path, top.dir); err != nil {
				return err
			}
		}

		return nil
	})

	var notFound *NotFoundError

	switch {
	case found:
		return err
	case err == nil, errors.As(err, &notFound):
		return s.NotFound(spec)
	}

	return err
}

// saveTop is a directory a save's search starts in.
type saveTop struct {
	path []string
	dir  *volume.Directory
}

// saveTops are the directories pattern's directory part names: the one
// it names, or with wildcards the ones holding a file it matched, in the
// order the matches found them (a matched directory with no file in it
// is passed over; unconfirmed).
func (s *Session) saveTops(vol *volume.Volume, pattern filespec.Spec, matches []filespec.Match) ([]saveTop, error) {
	if !strings.ContainsAny(strings.Join(pattern.Dirs, "."), "*%") {
		dir, err := filespec.ResolveDirectory(vol, pattern.Dirs)
		if err != nil {
			return nil, &NotFoundError{Spec: pattern.String()}
		}

		return []saveTop{{path: pattern.Dirs, dir: dir}}, nil
	}

	var (
		tops []saveTop
		seen = map[string]bool{}
	)

	for _, m := range matches {
		path := m.Dirs
		if pattern.Recursive {
			path = path[:min(len(path), len(pattern.Dirs))]
		}

		key := strings.Join(path, ".")
		if seen[key] {
			continue
		}

		seen[key] = true

		dir, err := filespec.ResolveDirectory(vol, path)
		if err != nil {
			return nil, err
		}

		tops = append(tops, saveTop{path: path, dir: dir})
	}

	return tops, nil
}

// saveSearch is one search's state.
type saveSearch struct {
	mounts    *MountTable
	device    string
	vol       *volume.Volume
	display   string
	recursive bool
	visit     func(*SaveFile) error
	matched   map[string]bool
}

// matchKey names a file by its directory, name, and version.
func matchKey(dirs []string, name string, version uint16) string {
	return strings.ToUpper(strings.Join(dirs, ".") + "]" + name + fmt.Sprintf(";%d", version))
}

// directory saves dir's files in its entries' order, going into each
// subdirectory where its entry is when the search is recursive.
func (s *saveSearch) directory(path []string, dir *volume.Directory) error {
	entries, err := dir.List()
	if err != nil {
		return err
	}

	for _, e := range entries {
		matched := s.matched[matchKey(path, e.Name, e.Version)]

		var sub *volume.Directory

		if s.recursive && e.Version == 1 && strings.HasSuffix(strings.ToUpper(e.Name), ".DIR") && !e.Fid.Equal(dir.Header.Fid) {
			if d, err := s.vol.OpenDirectory(e.Fid); err == nil {
				sub = d
			}
		}

		if !matched && sub == nil {
			continue
		}

		f, err := s.vol.OpenFID(e.Fid)
		if err != nil {
			return err
		}

		saved := &SaveFile{
			Name:       s.display + ":" + dirSpec{Elems: path}.String() + e.Name + fmt.Sprintf(";%d", e.Version),
			Attributes: savedAttributes(f, dir, path, e),
			Walked:     sub != nil,
			mounts:     s.mounts,
			device:     s.device,
			vol:        s.vol,
			fid:        e.Fid,
		}

		if err := s.visit(saved); err != nil {
			return err
		}

		if sub != nil {
			name, _, _ := strings.Cut(e.Name, ".")
			if err := s.directory(append(append([]string(nil), path...), name), sub); err != nil {
				return err
			}
		}
	}

	return nil
}

// savedAttributes are what a save set keeps of f, found in dir at path
// through entry e: its header's fields, its version limit (both VERLIMIT
// and DIR_VERLIM, which VMS wrote alike for every file: 3 for VERLIM.TXT,
// 5 for [B69.SUB1]'s files, 32767 for none; the directory entry's, or the
// one govax keeps in a file's header when it has one), and dir's owner
// and protection.
func savedAttributes(f *volume.File, dir *volume.Directory, path []string, e ondisk.DirEntry) backup.Attributes {
	h := f.Header

	limit := e.VersionLimit
	if own := h.RecordAttributes.VersionLimit; own != 0 && !h.IsDirectory() {
		limit = own
	}

	if limit == 0 || limit > ondisk.NoVersionLimit {
		limit = ondisk.NoVersionLimit
	}

	a := backup.Attributes{
		Name:            dirSpec{Elems: path}.String() + e.Name + fmt.Sprintf(";%d", e.Version),
		StructureLevel:  h.StructureLevel,
		FID:             savedFileID(h.Fid, true),
		BackLink:        savedFileID(h.Backlink, false),
		Allocated:       h.RecordAttributes.HighestBlock,
		Owner:           uicValue(h.Owner),
		Protection:      h.FileProtection,
		RecordProt:      h.RecordProtection,
		AccessLevel:     h.AccessMode,
		Characteristics: h.FileCharacteristics,
		VersionLimit:    limit,
		Journal:         h.Journaling,
		RUActive:        h.RecoveryUnitActive,
		Highwater:       h.HighWaterMark,
		DirOwner:        uicValue(dir.Header.Owner),
		DirProtection:   dir.Header.FileProtection,
		DirVersionLimit: limit,
	}

	copy(a.RecordAttributes[:], h.Raw()[fatOffset:fatOffset+len(a.RecordAttributes)])

	// The record attributes' version limit word is a directory's default
	// limit; VMS's files have 0 there, their limits being their
	// directory entries'. govax keeps a file's own limit there too, which
	// VERLIMIT has taken.
	if !h.IsDirectory() {
		a.RecordAttributes[fatVersions], a.RecordAttributes[fatVersions+1] = 0, 0
	}

	if id, err := h.Ident(); err == nil {
		a.Revision = id.Revision
		a.Created = uint64(id.CreationDate)
		a.Revised = uint64(id.RevisionDate)
		a.Expires = uint64(id.ExpirationDate)
		a.Backup = uint64(id.BackupDate)
	}

	if acl, err := h.ACL(); err == nil {
		a.ACL = acl
	}

	if a.Directory() {
		a.DirStatus = dirStatusSaved
	}

	return a
}

// fatOffset is where a file header's record attributes area (FH2$W_RECATTR)
// starts, and fatVersions where its version limit word (FAT$W_VERSIONS)
// is in it.
const (
	fatOffset   = 20
	fatVersions = 30
)

// savedFileID is a file ID as a save set's attributes hold it: the file's
// own with relative volume number 1 when it has 0 (as VMS writes FID),
// a backlink as the header has it.
func savedFileID(id ondisk.Fid, own bool) [3]uint16 {
	rvn := id.Rvn
	if own && rvn == 0 {
		rvn = 1
	}

	return [3]uint16{id.Num, id.Seq, uint16(rvn) | uint16(id.Nmx)<<8}
}

// uicValue is a UIC as a save set holds it: group in the high word,
// member in the low.
func uicValue(u ondisk.Uic) uint32 {
	return uint32(u.Group)<<16 | uint32(u.Member)
}

// SaveSetOptions are how CreateSaveSet makes a save set's file.
type SaveSetOptions struct {
	// BlockSize is the save set's block size, its file's record size.
	BlockSize int

	// Owner is the file's owner (the output's /BY_OWNER, or the
	// process's UIC); nil leaves what the file system gives it.
	Owner *ondisk.Uic
}

// SaveSetOutput is a save set's file, made and waiting for its blocks.
type SaveSetOutput struct {
	// Name is the file's full name with its version,
	// "DUA1:[SETS]B69.BCK;1", or the host path; Device the volume's
	// device as named ("DUA1"), "" for a host file.
	Name   string
	Device string

	host *os.File
	file *volume.File
	bm   *volume.Bitmap
	ib   *volume.IndexBitmap
	opts SaveSetOptions
}

// CreateSaveSet makes the save set file loc names, empty, for Finish to
// fill: on a volume a new version (or the version loc gives), on the host
// a new file. It's made before the save reads anything, so a name that
// can't be a file fails first, as VMS's OPENOUT does.
func (s *Session) CreateSaveSet(loc FileLocation, opts SaveSetOptions) (*SaveSetOutput, error) {
	if loc.Host {
		f, err := os.Create(loc.Name)
		if err != nil {
			return nil, fmt.Errorf("rms: creating %s: %w", loc.Name, err)
		}

		return &SaveSetOutput{Name: loc.Name, host: f, opts: opts}, nil
	}

	specs, err := expandSpec(s.Logicals, loc.Name, s.Default)
	if err != nil {
		return nil, err
	}

	r := specs[0]

	vol, spec, err := s.mounted(r)
	if err != nil {
		return nil, err
	}

	if spec.Recursive || spec.Name+spec.Type == "" || strings.ContainsAny(spec.Name+spec.Type+spec.Version+strings.Join(spec.Dirs, "."), "*%") {
		return nil, newDCLFileError(fullName(s, loc.Name), rmsWildcardError)
	}

	if !s.Mounts.Writable(spec.Device) {
		return nil, fmt.Errorf("%s: %w", spec.Device, ErrACPWriteLocked)
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok || version > MaxVersionLimit {
		return nil, fmt.Errorf("rms: %s: version %q: want 1 to %d", loc.Name, spec.Version, MaxVersionLimit)
	}

	dir, bm, ib, err := resolveVolumeDest(vol, spec)
	if err != nil {
		return nil, s.NotFound(loc.Name)
	}

	fullName := spec.Name + "." + spec.Type
	attr := ondisk.RecAttr{
		Format:        ondisk.RecordFormatFixed,
		RecordSize:    uint16(opts.BlockSize),
		MaxRecordSize: uint16(opts.BlockSize),
	}

	f, err := createVersion(vol, dir, fullName, version, attr, bm, ib)
	if err != nil {
		return nil, err
	}

	made, err := createdVersion(dir, fullName, version)
	if err != nil {
		return nil, err
	}

	device := spec.Device

	spec.Device = r.Display
	if spec.Device == "" {
		spec.Device = device
	}

	spec.Version = fmt.Sprint(made)

	return &SaveSetOutput{Name: spec.String(), Device: spec.Device, file: f, bm: bm, ib: ib, opts: opts}, nil
}

// Finish writes data, the save set's blocks, into the file and closes
// it. A volume file gets the blocks data needs, allocated at once (VMS's
// allocation, sets.lst: 441/441), its end of file after them, and the
// owner.
func (o *SaveSetOutput) Finish(data []byte) error {
	if o.host != nil {
		_, err := o.host.Write(data)
		if cErr := o.host.Close(); cErr != nil && err == nil {
			err = cErr
		}

		if err != nil {
			return fmt.Errorf("rms: writing %s: %w", o.Name, err)
		}

		return nil
	}

	err := writeSaveSet(o.file, data, o.bm, o.ib, o.opts.Owner)

	if fErr := o.bm.Flush(); fErr != nil && err == nil {
		err = fErr
	}

	if fErr := o.ib.Flush(); fErr != nil && err == nil {
		err = fErr
	}

	if err != nil {
		return fmt.Errorf("rms: writing %s: %w", o.Name, err)
	}

	return nil
}

// writeSaveSet gives f data's blocks, all allocated at once, and owner.
func writeSaveSet(f *volume.File, data []byte, bm *volume.Bitmap, ib *volume.IndexBitmap, owner *ondisk.Uic) error {
	blocks := uint32((len(data) + ondisk.BlockSize - 1) / ondisk.BlockSize)

	if blocks > f.Blocks() {
		if err := volume.Extend(f, bm, ib, blocks-f.Blocks()); err != nil {
			return fmt.Errorf("%w: %v", ErrACPDeviceFull, err)
		}
	}

	buf := make([]byte, ondisk.BlockSize)
	for vbn := uint32(1); vbn <= blocks; vbn++ {
		clear(buf)
		copy(buf, data[int(vbn-1)*ondisk.BlockSize:])

		if err := f.WriteBlock(vbn, buf); err != nil {
			return err
		}
	}

	if err := f.CloseWithFinalByte(uint16(len(data) % ondisk.BlockSize)); err != nil {
		return err
	}

	if owner == nil {
		return nil
	}

	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) {
		h.Owner = *owner
	})
}
