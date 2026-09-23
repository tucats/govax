package console

import (
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console INITIALIZE/CONTAINER command
// (docs/PHASE-23.md, subtask 4) — the operator-facing half of "format a
// brand-new, empty ODS-2 volume". The actual work (writing the on-disk
// volume structures) already lives in the sibling internal/rms package's
// InitializeContainer (initialize.go, this same subtask); this file is a
// thin, console-specific layer on top of it, mirroring mount.go's own
// Mount/Dismount wrappers around internal/rms.MountTable and default.go's
// SetDefault/ShowDefault wrappers around internal/rms.Session.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.InitializeContainer(...) on a *Console value. It doesn't need to read
// or write any of Console's own fields (c.Mounts, c.ContainerSession, ...)
// — unlike Mount, INITIALIZE/CONTAINER doesn't touch the mount table or
// the operator's current default directory at all, it just builds a new,
// unattached container file — but it's still defined as a method (rather
// than a free function) to match every other console command's calling
// convention (d.Console.InitializeContainer(...) from dispatch.go, the
// same shape as d.Console.Mount(...)).

// InitializeContainer formats path as a brand-new, empty ODS-2 volume: a
// zero-filled host file of blocks 512-byte blocks, with just enough
// on-disk structure (an index file, a master file directory, a free-space
// bitmap, ...) to make it a valid volume — see the sibling
// internal/rms.InitializeContainer's own doc comment for the full
// behavioral explanation. Matching real VMS's own INITIALIZE, and ods2's
// own cmdInitialize (this command's direct behavioral reference), the
// volume is not mounted afterward — a following MOUNT command attaches it
// to a device name like any other container.
//
// Any failure (a size of zero blocks, a container path that can't be
// created, a volume too small for the minimal reserved-file layout, a
// label too long for its fixed on-disk width, ...) is reported as the
// real SS_BADPARAM console status rather than a bare Go error, per
// docs/PHASE-23.md's "Status-code / error translation" design section —
// ss_def.h has no INIT-specific status this project can reuse, and every
// one of this command's own failure modes ultimately traces back to a bad
// argument value, the same story SS$_BADPARAM tells for any other system
// service (see SS_BADPARAM's own doc comment in internal/vmserrors).
func (c *Console) InitializeContainer(path string, blocks uint32, label string, clusterSize uint16) error {
	if err := rms.InitializeContainer(path, blocks, label, clusterSize); err != nil {
		return vmserrors.Wrap(vmserrors.SS_BADPARAM, err, path)
	}

	return nil
}
