package rms

import (
	"fmt"
	"strings"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// mountedVolume is what MountTable remembers about one device that
// currently has a container mounted on it.
type mountedVolume struct {
	Volume *volume.Volume

	// Writable records how this volume was mounted (Mount's own writable
	// argument), so a later SYS$CREATE/SYS$PUT handler can reject a write
	// attempt against a read-only mount with a clear RMS-level error (see
	// Writable's own doc comment) instead of failing confusingly partway
	// through, deep inside an ods2 call.
	Writable bool
}

// MountTable tracks which VAX device names currently have an ODS-2 volume
// mounted on them — the Go state behind govax's console MOUNT/DISMOUNT
// commands (internal/console, wired up in a later docs/PHASE-22.md
// subtask) and the thing this package's own SYS$CREATE/SYS$OPEN handlers
// consult to turn a device name like "DUA0:" into a real, usable
// *volume.Volume from the sibling github.com/tucats/ods2 module.
//
// # Why this exists, for a reader new to VMS
//
// Real VMS never lets a program open a file by just naming a host path.
// Every file spec starts with a *device name* ("DUA0:[FOO]BAR.DAT;1"),
// and before anything can be done with a file on that device, an operator
// (or, here, a test, or a govax console session) has to MOUNT a physical
// disk (or, for this emulator, a container file standing in for one) on
// that device name. MountTable is exactly that: a map from device name to
// "which mounted volume, if any, currently backs it".
//
// A MountTable has no concurrency protection of its own (a mutex, for
// example) — govax has no concept of multiple VAX processes running at
// once and sharing one Console (see internal/rtl/environment.go's own
// doc comment on why Environment is one-per-process), so nothing here
// needs to guard against two goroutines calling Mount/Dismount at the
// same time. This matches every other shared table this project already
// has (internal/io's DeviceTable and LogicalNameTable are likewise
// unsynchronized plain Go maps).
type MountTable struct {
	// mounts is keyed by normalized device name (see normalizeDeviceName)
	// so that "DUA0", "DUA0:", and "dua0:" all name the same entry.
	mounts map[string]*mountedVolume
}

// NewMountTable returns an empty MountTable, with nothing mounted yet.
func NewMountTable() *MountTable {
	return &MountTable{mounts: map[string]*mountedVolume{}}
}

// normalizeDeviceName upper-cases name and strips one trailing ':', so
// that "DUA0", "DUA0:", and "dua0:" — all valid ways to write the same
// VMS device name — produce the same map key.
//
// internal/io's own DeviceTable has an unexported helper doing exactly
// this same two-line job; it isn't reused here because this package
// deliberately has no dependency on internal/io at all (docs/PHASE-22.md's
// design has MountTable injected *into* the console/RTL layer, not the
// other way around, so a dependency in that direction would be backwards)
// — and an unexported function can't be called from another package
// regardless. Duplicating two lines is simpler than restructuring package
// boundaries just to share them.
func normalizeDeviceName(name string) string {
	return strings.ToUpper(strings.TrimSuffix(name, ":"))
}

// Mount opens the container file at path and mounts it as an ODS-2 volume
// on device — the Go equivalent of real VMS's MOUNT command, in its
// simplest single-disk form (docs/PHASE-22.md's grammar section covers
// what govax's own console MOUNT command exposes on top of this).
//
// writable selects which of ods2's two container-opening functions is
// used: diskimage.OpenWritable, so this package's SYS$CREATE/SYS$PUT
// handlers can actually write records to the volume, or the read-only
// diskimage.Open. Passing writable=false when nothing will ever write to
// the volume isn't just a safety nicety — it's also the only way to
// mount a container ods2 can't write to at all, such as a raw CD-ROM
// sector dump (see diskimage.WritableContainer's own doc comment in the
// sibling ods2 module for why).
//
// Mount fails if device already has a container mounted on it — matching
// real VMS, where MOUNT-ing an already-mounted device is an error, not an
// implicit re-mount. Call Dismount first if you want to swap containers.
func (t *MountTable) Mount(device, path string, writable bool) error {
	key := normalizeDeviceName(device)

	if _, already := t.mounts[key]; already {
		return fmt.Errorf("rms: %s: already mounted", key)
	}

	var (
		container diskimage.Container
		err       error
	)

	if writable {
		container, err = diskimage.OpenWritable(path)
	} else {
		container, err = diskimage.Open(path)
	}

	if err != nil {
		return fmt.Errorf("rms: mounting %s on %s: %w", path, key, err)
	}

	vol, err := volume.Mount(container)
	if err != nil {
		// volume.Mount failed after the container was already
		// successfully opened, so it's this call's job to close it again
		// — otherwise the open file handle would simply leak.
		_ = container.Close()

		return fmt.Errorf("rms: mounting %s on %s: %w", path, key, err)
	}

	t.mounts[key] = &mountedVolume{Volume: vol, Writable: writable}

	return nil
}

// Dismount flushes and closes device's mounted volume and forgets it, so
// that a later Mount can attach a different container to the same device
// name. The actual flush-pending-writes-then-close-the-container work is
// entirely volume.Volume.Dismount's own job (see its doc comment in the
// sibling ods2 module); this method's only added responsibility is
// forgetting the MountTable entry afterward.
//
// Dismount fails if device has nothing mounted on it.
func (t *MountTable) Dismount(device string) error {
	key := normalizeDeviceName(device)

	entry, ok := t.mounts[key]
	if !ok {
		return fmt.Errorf("rms: %s: not mounted", key)
	}

	if err := entry.Volume.Dismount(); err != nil {
		return fmt.Errorf("rms: dismounting %s: %w", key, err)
	}

	delete(t.mounts, key)

	return nil
}

// Lookup returns the *volume.Volume currently mounted on device, and
// whether one is. This is what this package's SYS$CREATE/SYS$OPEN
// handlers (added in a later docs/PHASE-22.md subtask) call once they've
// parsed a file spec's device name out of a FAB, to turn that name into
// the real volume to operate on.
func (t *MountTable) Lookup(device string) (*volume.Volume, bool) {
	entry, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return nil, false
	}

	return entry.Volume, true
}

// VolumeLabel returns the ASCII volume label recorded in device's mounted
// volume's home block (real VMS's own VOLNAM: whatever INITIALIZE/VOLUME
// SET wrote there when the container was first initialized — see the
// sibling ods2 module's ondisk.HomeBlock.VolumeName), and whether device
// has anything mounted at all.
//
// This exists purely so a caller displaying mount status (SHOW DEVICE/
// FULL, internal/console/device.go's ShowDevices) doesn't need to import
// the sibling ods2 module's own volume/ondisk types just to read one
// string field — keeping that dependency confined to this package, the
// one place govax code is allowed to reach into ods2 directly (see
// mount.go's own package-level design note in docs/PHASE-22.md).
//
// A volume set's label lives on every member's home block identically (it
// describes the logical volume as a whole, not any one member disk), so
// reading it off the first member (Devices[0]) is always correct even for
// a multi-disk mount — not that this phase's MountTable.Mount ever passes
// more than one container in anyway (see its own doc comment).
func (t *MountTable) VolumeLabel(device string) (string, bool) {
	entry, ok := t.mounts[normalizeDeviceName(device)]
	if !ok {
		return "", false
	}

	return entry.Volume.Devices[0].Home.VolumeName, true
}

// Writable reports whether device's mounted volume was mounted with write
// access (Mount's own writable argument) — what a later SYS$CREATE/
// SYS$PUT handler is expected to check before attempting to write, so
// that writing to a read-only-mounted volume fails with a clear,
// RMS-level rmsPrivilegeViolation status (status.go) instead of an
// obscure ods2-level error partway through the attempt.
//
// Writable reports false for a device with nothing mounted at all — the
// same "no, you can't write here" answer a genuinely read-only mount
// would give — on the assumption that a caller who needs to tell those
// two cases apart (no mount at all, versus a read-only mount) has already
// called Lookup first to check.
func (t *MountTable) Writable(device string) bool {
	entry, ok := t.mounts[normalizeDeviceName(device)]

	return ok && entry.Writable
}
