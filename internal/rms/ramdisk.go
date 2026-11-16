package rms

import (
	"errors"
	"fmt"
	"sort"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is RMS's half of RAM disks (docs/planning/PHASE-96 - RAM
// disk.md, Decision 3): ODS-2 volumes whose blocks are held in govax's own
// Go memory, in an ods2 diskimage.MemoryImage, rather than in a host
// container file.
//
// # Why the image isn't kept in the mount table entry
//
// A mount table entry (mountedVolume) lives from MOUNT to DISMOUNT. A RAM
// disk's contents must outlive that: on VMS (DECram) a RAM disk's memory
// belongs to the device, so a DISMOUNT and a later MOUNT find the files
// left there. So the images are kept in a map of their own, ramDisks, by
// device name, made by DefineMemory (DCL's DEFINE/DEVICE of an MD device)
// and kept until the device is defined again or ReleaseMemory runs at the
// session's end.
//
// Each mount is given a *view* of the image (diskimage.MemoryImage.View):
// a container whose Close ends only the view. ods2's Volume.Dismount
// flushes its bitmaps and closes its containers, as it does for a file;
// here that closes the view, and the image keeps its blocks for the next
// mount's view to read.

// MinimumMemoryBlocks is the smallest RAM disk DefineMemory makes, in
// blocks: enough for INITIALIZE's reserved files and room for a few of
// the user's.
const MinimumMemoryBlocks = 100

// Errors DefineMemory and MountMemory return, for DCL to turn into VMS's
// statuses (a NOSUCHDEV, a DEVMOUNT, govax's RAMSIZE and RAMNOWRITE).
var (
	// ErrNotMemory: the device isn't a RAM disk (DefineMemory hasn't made
	// it one).
	ErrNotMemory = errors.New("not a RAM disk")

	// ErrMounted: the device is mounted, so its RAM disk can't be defined
	// again.
	ErrMounted = errors.New("device is mounted")

	// ErrMemoryTooSmall: a RAM disk of fewer than MinimumMemoryBlocks.
	ErrMemoryTooSmall = errors.New("RAM disk is too small")

	// ErrMemoryNoWrite: a read-only first mount, of a volume that would be
	// empty.
	ErrMemoryNoWrite = errors.New("a RAM disk's first mount must be writable")
)

// MemoryLimitError is DefineMemory's error when the RAM disks would total
// more blocks than the limit allows: the total they would have, and the
// limit.
type MemoryLimitError struct {
	Total uint64
	Limit uint32
}

func (e *MemoryLimitError) Error() string {
	return fmt.Sprintf("rms: RAM disks would total %d blocks, over the limit of %d", e.Total, e.Limit)
}

// ramDisk is one RAM disk: its image, its size, and whether a first mount
// has put a volume on it yet.
type ramDisk struct {
	image     *diskimage.MemoryImage
	blocks    uint32
	formatted bool
}

// MemoryVolume is what a RAM disk's first mount makes its new volume with
// (MountMemory). Later mounts find the volume already there, and ignore it.
type MemoryVolume struct {
	// Label is the volume label, at most 12 characters; "" for
	// INITIALIZE's default, NONAME.
	Label string

	// ClusterSize is the allocation unit in blocks, 0 for INITIALIZE's
	// default.
	ClusterSize uint16

	// Owner is the volume's owner UIC, which its reserved files and its
	// master file directory are owned by too; 0 for INITIALIZE's default,
	// [1,4].
	Owner security.UIC

	// Protection is the volume's protection; 0 for none (no restriction).
	Protection uint16
}

// initializeOptions is INITIALIZE's choices for a new volume, shared by
// InitializeContainer and a RAM disk's first mount, so that both have the
// same defaults.
func initializeOptions(label string, clusterSize uint16) volume.InitializeOptions {
	return volume.InitializeOptions{
		Label:       label,
		ClusterSize: clusterSize,
	}
}

// DefineMemory makes device a RAM disk of blocks blocks, all zero, with no
// volume on it until its first mount. If device was a RAM disk already,
// its old image and any volume on it are discarded, and it counts at its
// new size in place of its old one.
//
// It fails, and changes nothing, if device is mounted (ErrMounted), if
// blocks is under MinimumMemoryBlocks (ErrMemoryTooSmall), or if the RAM
// disks would then total more than limit blocks (*MemoryLimitError). The
// total counts each disk's defined size, not what it holds now: a disk
// can never grow past what was allowed when it was made.
func (t *MountTable) DefineMemory(device string, blocks, limit uint32) error {
	key := normalizeDeviceName(device)

	if _, mounted := t.mounts[key]; mounted {
		return fmt.Errorf("rms: %s: %w", key, ErrMounted)
	}

	if blocks < MinimumMemoryBlocks {
		return fmt.Errorf("rms: %s: %d blocks: %w", key, blocks, ErrMemoryTooSmall)
	}

	total := uint64(blocks)

	for name, disk := range t.ramDisks {
		if name != key {
			total += uint64(disk.blocks)
		}
	}

	if total > uint64(limit) {
		return &MemoryLimitError{Total: total, Limit: limit}
	}

	image, err := diskimage.NewMemory(blocks)
	if err != nil {
		return fmt.Errorf("rms: defining %s: %w", key, err)
	}

	if old, ok := t.ramDisks[key]; ok {
		old.image.Release()
	}

	if t.ramDisks == nil {
		t.ramDisks = map[string]*ramDisk{}
	}

	t.ramDisks[key] = &ramDisk{image: image, blocks: blocks}

	return nil
}

// MountMemory mounts device's RAM disk. On its first mount the disk is
// given a new, empty volume made with opts, which must be writable
// (ErrMemoryNoWrite); after that each mount finds the volume as the last
// DISMOUNT left it, and opts is ignored. A device that isn't a RAM disk
// is ErrNotMemory, and one already mounted an error, as for Mount.
//
// Checking a later mount's label against the volume's is the caller's
// (VolumeLabel), as for any disk.
func (t *MountTable) MountMemory(device string, writable bool, opts MemoryVolume) error {
	key := normalizeDeviceName(device)

	disk, ok := t.ramDisks[key]
	if !ok {
		return fmt.Errorf("rms: %s: %w", key, ErrNotMemory)
	}

	if _, already := t.mounts[key]; already {
		return fmt.Errorf("rms: %s: already mounted", key)
	}

	if !disk.formatted {
		if !writable {
			return fmt.Errorf("rms: %s: %w", key, ErrMemoryNoWrite)
		}

		init := initializeOptions(opts.Label, opts.ClusterSize)
		if opts.Owner != 0 {
			init.Owner = ondisk.Uic{Group: uint16(opts.Owner.Group()), Member: uint16(opts.Owner.Member())}
		}

		init.VolumeProtection = opts.Protection

		view := disk.image.View()
		err := volume.Initialize(view, init)

		_ = view.Close()

		if err != nil {
			return fmt.Errorf("rms: initializing %s: %w", key, err)
		}

		disk.formatted = true
	}

	var container diskimage.Container = disk.image.View()
	if !writable {
		// ods2 decides whether it may write a volume by whether its
		// container has a WriteBlock method, so a read-only mount gets a
		// view without one.
		container = readOnlyContainer{container}
	}

	if err := t.mountContainer(key, container, writable); err != nil {
		return fmt.Errorf("rms: mounting %s: %w", key, err)
	}

	t.mounts[key].memory = true

	return nil
}

// readOnlyContainer is a container with only Container's methods, hiding
// a WriteBlock the one inside it has.
type readOnlyContainer struct {
	diskimage.Container
}

// IsMemory reports whether device is a RAM disk (DefineMemory made it
// one), mounted or not.
func (t *MountTable) IsMemory(device string) bool {
	_, ok := t.ramDisks[normalizeDeviceName(device)]

	return ok
}

// MemoryHasVolume reports whether device is a RAM disk a first mount has
// put a volume on, which defining it again would discard.
func (t *MountTable) MemoryHasVolume(device string) bool {
	disk, ok := t.ramDisks[normalizeDeviceName(device)]

	return ok && disk.formatted
}

// MemorySize returns device's RAM disk's size in blocks, and whether
// device is a RAM disk.
func (t *MountTable) MemorySize(device string) (uint32, bool) {
	disk, ok := t.ramDisks[normalizeDeviceName(device)]
	if !ok {
		return 0, false
	}

	return disk.blocks, true
}

// MemoryInUse returns the bytes of host memory device's RAM disk holds
// now, 0 if it isn't one. The image is sparse: it holds only what has
// been written to it, in chunks of 64 KB, and never gives a chunk back
// while the disk exists.
func (t *MountTable) MemoryInUse(device string) int64 {
	disk, ok := t.ramDisks[normalizeDeviceName(device)]
	if !ok {
		return 0
	}

	return disk.image.Resident()
}

// MemoryTotal returns the blocks every RAM disk was defined with, added
// up (what DefineMemory holds against its limit), and the bytes of host
// memory they hold now.
func (t *MountTable) MemoryTotal() (defined uint32, resident int64) {
	for _, disk := range t.ramDisks {
		defined += disk.blocks
		resident += disk.image.Resident()
	}

	return defined, resident
}

// ReleaseMemory frees every RAM disk, at the session's end, after
// DismountAll. A RAM disk still mounted is dismounted first, whatever is
// open on it. Its error is every dismount's that failed.
func (t *MountTable) ReleaseMemory() error {
	devices := make([]string, 0, len(t.ramDisks))
	for key := range t.ramDisks {
		devices = append(devices, key)
	}

	sort.Strings(devices)

	var errs []error

	for _, key := range devices {
		if _, mounted := t.mounts[key]; mounted {
			if err := t.dismount(key, true); err != nil {
				errs = append(errs, err)
			}
		}

		t.ramDisks[key].image.Release()
		delete(t.ramDisks, key)
	}

	return errors.Join(errs...)
}
