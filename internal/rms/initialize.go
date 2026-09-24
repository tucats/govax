package rms

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 4: the internal/rms half of
// the console INITIALIZE/CONTAINER command -- formatting a brand-new,
// empty ODS-2 volume from nothing, as opposed to every other command in
// this package (Mount, resolveVolume, ...), which all operate on a volume
// that already exists.
//
// # Why this matters, for a reader new to VMS
//
// Before a device can be MOUNTed and used, *something* has to build the
// on-disk data structures a mounted ODS-2 volume expects to find -- an
// index file listing every other file, a master file directory, a bitmap
// tracking free space, and so on. INITIALIZE is that "something": it
// starts from a blank, unformatted container and writes just enough
// structure to make it a valid, empty volume. Real VMS's own INITIALIZE
// command does exactly this to a physical disk; here it does the same
// thing to a container file standing in for one (see docs/PHASE-22.md for
// how containers already stand in for physical disks elsewhere in this
// project).
//
// This is also why INITIALIZE doesn't take a device name the way MOUNT
// does: a freshly initialized volume isn't attached to anything yet. An
// operator runs MOUNT separately afterward to attach it to whichever
// device name they choose -- matching real VMS, where INITIALIZE formats
// a volume without mounting it.

// InitializeContainer creates a new, zero-filled host file at path sized to
// hold blocks 512-byte blocks, and formats it as a fresh, empty ODS-2
// volume -- the Go equivalent of real VMS's own INITIALIZE command, and
// this project's direct behavioral match for the sibling ods2 module's own
// cmdInitialize (github.com/tucats/ods2/cmd/ods2/internal/session/
// initialize.go), read here purely as a *behavioral* spec (see
// docs/PHASE-23.md's "Why this phase looks different from most others").
//
// Matching real VMS's own INITIALIZE (and ods2's own cmdInitialize), the
// freshly built volume is *not* mounted -- a caller runs MountTable.Mount
// separately afterward, on whatever device name they choose, exactly the
// way an operator's own MOUNT command follows a real INITIALIZE.
//
// label becomes the new volume's on-disk label (its VOLNAM, what SHOW
// DEVICE/FULL and this package's own VolumeLabel display later); an empty
// label defaults to "NONAME", matching ods2's own cmdInitialize and, one
// level further down, volume.InitializeOptions.Label's own documented
// default. clusterSize selects the volume's allocation unit in blocks
// (how many blocks are handed out together every time a file grows); 0
// selects volume.Initialize's own default of 1 block per cluster -- a fine
// choice for the small, synthetic containers this project's own
// INITIALIZE mostly builds.
//
// Any failure -- blocks is 0, path can't be created (a missing parent
// directory, a permissions problem, ...), or the container is too small
// for even the minimal reserved-file layout every ODS-2 volume needs --
// is returned as a plain Go error; internal/console's own thin wrapper
// (initialize.go) is what translates that into a real, numbered VMS
// status for the operator to see (docs/PHASE-23.md's "Status-code / error
// translation" design section).
func InitializeContainer(path string, blocks uint32, label string, clusterSize uint16, devType string) error {
	// If blocks were not specified, but a known device type was given, use it's max block
	// size as the container size. If the device was specified but is invalid, complain that
	// it's a bad device name. Otherwise, complain that SIZE was invalid/missing.
	if blocks == 0 {
		if devType != "" {
			devType = strings.ToUpper(devType)
			if devOptions, ok := io.KnownDeviceOptions[devType]; ok {
				blocks = devOptions.MaxBlock
			} else {
				return vmserrors.New(vmserrors.CLI_BADQUALIFIER, "DEVICE")
			}
		} else {
			return vmserrors.New(vmserrors.CLI_BADQUALIFIER, "SIZE")
		}
	}

	c, err := diskimage.Create(path, blocks)
	if err != nil {
		return fmt.Errorf("rms: initializing %s: %w", path, err)
	}

	// diskimage.Create always succeeds in creating the host file before
	// volume.Initialize ever runs, so this container must be closed on
	// every return path from here on -- including a failing
	// volume.Initialize, which otherwise would leak the open file handle
	// even though the (now half-formatted) file itself is left behind for
	// the operator to clean up or retry over, matching real INITIALIZE's
	// own "a failed format doesn't try to guess whether to delete what it
	// was writing" behavior.
	defer func() { _ = c.Close() }()

	opts := volume.InitializeOptions{
		Label:       label,
		ClusterSize: clusterSize,
	}

	if err := volume.Initialize(c, opts); err != nil {
		return fmt.Errorf("rms: initializing %s: %w", path, err)
	}

	return nil
}
