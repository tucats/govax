package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console PURGE command (docs/PHASE-23.md,
// subtask 7) — the operator-facing half of "trim old versions of a name
// down to a keep count". The actual work (glob-matching, the group-by-
// directory/PurgeVersions loop) already lives in the sibling internal/rms
// package's Session.Purge (internal/rms/purge.go, this same subtask); this
// file is a thin, console-specific layer on top of it, mirroring delete.go's
// own Console.Delete.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Purge(...) on a *Console value. Like Console.Delete, it tells
// Session.Purge's different failure modes apart via errors.As (rather than
// inspecting error text) so each is reported as the specific VMS status a
// real operator would see for that exact condition.

// Purge trims every name specText matches down to its keep most recent
// surviving versions, printing one "%PURGE-S-PURGED, NAME.TYPE purged
// (keeping N version(s))" confirmation line per name actually touched —
// see internal/rms.Session.Purge's own doc comment for exactly how specText
// is resolved, and why any version it carries is overridden rather than
// honored.
//
// Session.Purge's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError and is reported as
//     SS_DEVNOTMOUNT, matching Console.Delete's own translation for the
//     same underlying condition.
//   - An invalid keep value (currently just zero) surfaces as an
//     *rms.InvalidLimitError and is reported as CLI_BADLIMIT.
//   - Anything else (a malformed file specification, an unsupported multi-
//     device volume set, or a genuine I/O failure freeing storage) is
//     reported as CLI_BADFILESPEC, matching Console.Delete's own catch-all
//     bucket.
func (c *Console) Purge(specText string, keep uint16) error {
	purged, err := c.ContainerSession.Purge(specText, keep)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var badLimit *rms.InvalidLimitError
		if errors.As(err, &badLimit) {
			return vmserrors.Wrap(vmserrors.CLI_BADLIMIT, err, badLimit.Limit)
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	for _, name := range purged {
		c.Printf("%%PURGE-S-PURGED, %s purged (keeping %d version(s))\n", name, keep)
	}

	return nil
}
