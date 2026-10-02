package rms

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-34.md's subtask 5: CREATE/DIRECTORY's
// work below the console. The directory files themselves are laid out by
// ods2 (volume.CreateDirectory, filespec.CreateDirectoryPath); this file
// resolves the typed specification as every other console command does
// (logical names, SET DEFAULT's device and directory, relative directories
// like [.SUB] and [-.X]) and turns the qualifiers into the options each new
// directory gets.
//
// # A note for a reader new to VMS
//
// On VMS a directory is itself a file, NAME.DIR;1, entered in its parent
// directory. CREATE/DIRECTORY [A.B.C] makes whichever of [A], [A.B], and
// [A.B.C] don't exist yet, top down. Each new directory has an owner (a
// UIC, "[group,member]"), a protection (who may read, write, execute, or
// delete it), and a default version limit for the files later created in
// it. Without qualifiers, all three come from the parent directory (the
// protection less delete access).

// MaxVersionLimit is the largest version limit VMS allows (and the largest
// version number).
const MaxVersionLimit = 32767

// CreateDirectoryOptions are CREATE/DIRECTORY's qualifiers.
type CreateDirectoryOptions struct {
	// Owner is /OWNER_UIC=uic; nil without it (or with /OWNER_UIC=PARENT):
	// the parent directory's owner, as VMS 7.3 gives a directory a
	// privileged process makes (docs/PHASE-34.md, Decisions 6).
	Owner *ondisk.Uic

	// VersionLimit is /VERSION_LIMIT=n (0 for no limit); nil for the
	// parent's limit.
	VersionLimit *uint16

	// Protection is /PROTECTION's text, "(S:RWED,O:RWED,G:RE,W)"; "" for
	// the parent's protection less delete. A category it leaves out keeps
	// that default.
	Protection string

	// Allocation is /ALLOCATION=n; 0 for VMS's default of 1 block.
	Allocation uint32

	// ProtectionEnable and ProtectionValue are LIB$CREATE_DIR's protection
	// masks: each bit set in ProtectionEnable takes its value from
	// ProtectionValue, and each bit clear from the default (the parent's
	// protection less delete). An enable of 0 leaves the default alone.
	// They apply after Protection, when both are given.
	ProtectionEnable uint16
	ProtectionValue  uint16

	// VolumeOnly makes a specification always mean a mounted volume, never
	// a host directory: a program's LIB$CREATE_DIR (docs/PHASE-34.md,
	// Decisions 7). One that reaches no volume is ErrNotDirectorySpec.
	VolumeOnly bool
}

// CreatedDirectory is one directory CreateDirectory reached.
type CreatedDirectory struct {
	// Name is the directory as VMS reports it, "DUA1:[A.B]" (with the
	// logical name the device was reached through, if it was concealed),
	// or a host path.
	Name string

	// Created is true for a directory this call made, false for one that
	// already existed.
	Created bool
}

// ErrNotDirectorySpec is a CREATE/DIRECTORY specification that names
// something other than one directory: a file name, type, or version, a
// wildcard, or a "..." tree.
var ErrNotDirectorySpec = errors.New("not a directory specification")

// CreateDirectory makes the directory specText names, and each missing
// directory above it, and returns every level of the path, top down,
// saying which it made: the levels a CREATE/DIRECTORY/LOG reports, and
// the last one, which it reports as already existing if this call didn't
// make it. A search list creates in its first element, as $CREATE does.
//
// The directory goes on a mounted volume when specText resolves to one
// (it has a device, or a SET DEFAULT onto a volume is in effect). The
// volume must be mounted writable. Otherwise it's a host directory,
// relative to the host's current directory (see createHostDirectory),
// where the owner, protection, and version limit have no meaning.
//
// If a level can't be made, the levels already made stay made and are
// returned with the error. A specification that doesn't parse, or names
// something other than a directory, is ErrNotDirectorySpec; one too deep
// or with a name too long, volume.ErrDirectoryName; a read-only volume,
// ErrACPWriteLocked; and a device not mounted, a *NotMountedError.
func (s *Session) CreateDirectory(specText string, opts CreateDirectoryOptions) ([]CreatedDirectory, error) {
	if opts.VersionLimit != nil && *opts.VersionLimit > MaxVersionLimit {
		return nil, fmt.Errorf("create directory: version limit %d: want 0 to %d", *opts.VersionLimit, MaxVersionLimit)
	}

	if opts.Protection != "" {
		if _, err := ondisk.ParseProtection(opts.Protection, 0); err != nil {
			return nil, fmt.Errorf("create directory: %w", err)
		}
	}

	text := strings.TrimSpace(specText)
	if text == "" {
		return nil, fmt.Errorf("create directory: no directory given")
	}

	if classifyName(text) == nameHost || (!strings.Contains(text, ":") && !s.DefaultOnVolume()) {
		if opts.VolumeOnly {
			return nil, fmt.Errorf("create directory: %s: %w: no mounted volume", text, ErrNotDirectorySpec)
		}

		return createHostDirectory(text)
	}

	specs, err := expandSpec(s.Logicals, text, s.Default)
	if err != nil {
		var lnmErr *LogicalNameError
		if errors.As(err, &lnmErr) {
			return nil, fmt.Errorf("create directory: %w", err)
		}

		return nil, fmt.Errorf("create directory: %w: %v", ErrNotDirectorySpec, err)
	}

	r := specs[0]
	spec := r.Spec

	if spec.Recursive || spec.Name != "" || spec.Type != "" || spec.Version != "" {
		return nil, fmt.Errorf("create directory: %s: %w", text, ErrNotDirectorySpec)
	}

	for _, d := range spec.Dirs {
		if strings.ContainsAny(d, "*%") {
			return nil, fmt.Errorf("create directory: %s: %w", text, ErrNotDirectorySpec)
		}
	}

	vol, _, err := s.mounted(r)
	if err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	if !s.Mounts.Writable(spec.Device) {
		return nil, fmt.Errorf("create directory: %s: %w", spec.Device, ErrACPWriteLocked)
	}

	if len(vol.Devices) != 1 {
		return nil, fmt.Errorf("create directory: %s is a %d-device volume set; only a single-device volume is supported", spec.Device, len(vol.Devices))
	}

	dev := vol.Devices[0]

	bm, err := dev.Bitmap()
	if err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	ib, err := dev.IndexBitmap()
	if err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	levels, createErr := filespec.CreateDirectoryPath(vol, spec.Dirs, opts.forParent, bm, ib)

	// Levels made before a failure stay made, so the bitmaps are written
	// either way.
	if err := bm.Flush(); err != nil && createErr == nil {
		createErr = err
	}

	if err := ib.Flush(); err != nil && createErr == nil {
		createErr = err
	}

	display := r.Display
	if display == "" {
		display = spec.Device
	}

	created := make([]CreatedDirectory, 0, len(levels))

	for _, l := range levels {
		created = append(created, CreatedDirectory{Name: display + ":" + l.String(), Created: l.Created})
	}

	if createErr != nil {
		return created, fmt.Errorf("create directory: %w", createErr)
	}

	return created, nil
}

// forParent is the options callback filespec.CreateDirectoryPath calls for
// each directory it makes, given the directory it's made in: VMS's
// defaults worked out against that parent, with the qualifiers laid over
// them.
func (o CreateDirectoryOptions) forParent(parent *volume.Directory) volume.DirectoryOptions {
	d := volume.InheritedDirectoryOptions(parent)

	if o.Owner != nil {
		d.Owner = o.Owner
	}

	if o.VersionLimit != nil {
		d.VersionLimit = *o.VersionLimit
	}

	if o.Protection != "" {
		// Can't fail: CreateDirectory parsed the same text first.
		p, _ := ondisk.ParseProtection(o.Protection, *d.Protection)
		d.Protection = &p
	}

	if o.ProtectionEnable != 0 {
		p := o.ProtectionValue&o.ProtectionEnable | *d.Protection&^o.ProtectionEnable
		d.Protection = &p
	}

	d.Allocation = o.Allocation

	return d
}

// createHostDirectory is CreateDirectory for a host directory: text is a
// host path ("work/out"), or a VMS directory ("[.WORK.OUT]", "[WORK.OUT]",
// "[-.OUT]") taken relative to the host's current directory, which serves
// as the host's default directory. Each missing level is made, as on a
// volume.
func createHostDirectory(text string) ([]CreatedDirectory, error) {
	path := text

	if classifyName(text) != nameHost {
		var err error
		if path, err = hostPathOf(text); err != nil {
			return nil, err
		}
	}

	path = filepath.Clean(path)

	// The levels to check or make, top down: each prefix of path. A ".."
	// only moves up, and names nothing to make.
	var (
		levels []string
		prefix string
	)

	if filepath.IsAbs(path) {
		prefix = filepath.VolumeName(path) + string(filepath.Separator)
	}

	for _, part := range strings.Split(strings.TrimPrefix(path, prefix), string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)

		if part != ".." && part != "." {
			levels = append(levels, prefix)
		}
	}

	if len(levels) == 0 {
		return []CreatedDirectory{{Name: path}}, nil
	}

	var created []CreatedDirectory

	for _, p := range levels {
		info, err := os.Stat(p)

		switch {
		case err == nil && info.IsDir():
			created = append(created, CreatedDirectory{Name: p})
		case err == nil:
			return created, fmt.Errorf("create directory: %s: is a file, not a directory", p)
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(p, 0o755); err != nil {
				return created, fmt.Errorf("create directory: %w", err)
			}

			created = append(created, CreatedDirectory{Name: p, Created: true})
		default:
			return created, fmt.Errorf("create directory: %w", err)
		}
	}

	return created, nil
}

// hostPathOf turns a VMS directory, "[A.B]", "[.A.B]", or "[-.A]" (angle
// brackets too), into a host path relative to the current directory:
// "A/B", "A/B", "../A". Each leading "-" is one level up.
func hostPathOf(text string) (string, error) {
	bad := fmt.Errorf("create directory: %s: %w", text, ErrNotDirectorySpec)

	if len(text) < 3 || !(text[0] == '[' && text[len(text)-1] == ']' || text[0] == '<' && text[len(text)-1] == '>') {
		return "", bad
	}

	inner := text[1 : len(text)-1]
	if strings.ContainsAny(inner, "*%[]<>;:") || strings.Contains(inner, "...") {
		return "", bad
	}

	parts := strings.Split(strings.TrimPrefix(inner, "."), ".")

	var out []string

	if up := parts[0]; strings.Trim(up, "-") == "" {
		for range up {
			out = append(out, "..")
		}

		parts = parts[1:]
	}

	for _, p := range parts {
		if strings.Trim(p, "-") == "" {
			return "", bad
		}

		out = append(out, p)
	}

	return filepath.Join(out...), nil
}
