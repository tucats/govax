package dclcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console PURGE command (docs/planning/PHASE-23.md,
// subtask 7) — the operator-facing half of "trim old versions of a name
// down to a keep count". The actual work (glob-matching, the group-by-
// directory/PurgeVersions loop) already lives in the sibling internal/rms
// package's Session.Purge (internal/rms/purge.go, this same subtask); this
// file is a thin, console-specific layer on top of it, mirroring delete.go's
// own Context.Delete.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Context)", meaning it's called as
// c.Purge(...) on a *Context value. Like Context.Delete, it tells
// Session.Purge's different failure modes apart via errors.As (rather than
// inspecting error text) so each is reported as the specific VMS status a
// real operator would see for that exact condition.

// Purge trims every name specText matches down to its keep most recent
// surviving versions, saying nothing unless log is set (showPurged) — see
// internal/rms.Session.Purge's own doc comment for exactly how specText is
// resolved, and why any version it carries is overridden rather than
// honored.
//
// Session.Purge's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError and is reported as
//     SS_DEVNOTMOUNT, matching Context.Delete's own translation for the
//     same underlying condition.
//   - An invalid /KEEP value (currently just zero) surfaces as an
//     *rms.InvalidLimitError and is reported as DCL's IVVALU, the value
//     its segment (unconfirmed against VMS's PURGE/KEEP=0).
//   - Anything else (a malformed file specification, an unsupported multi-
//     device volume set, or a genuine I/O failure freeing storage) is
//     reported as CLI_BADFILESPEC, matching Context.Delete's own catch-all
//     bucket.
func (c *Context) Purge(specText string, keep uint16, log bool) error {
	purged, err := c.ContainerSession.Purge(specText, keep)

	// Versions the process's protection kept it from deleting are passed
	// over, the others purged, and each is FILNOTPUR over RMS$_PRV after
	// them, as VMS 7.1 said it (docs/planning/PHASE-93 - file
	// protection.md, A.4):
	//
	//	%PURGE-W-FILNOTPUR, error deleting DUA1:[A]B.DAT;1
	//	-RMS-E-PRV, insufficient privilege or file protection violation
	var notDeleted *rms.NotDeletedError
	if errors.As(err, &notDeleted) {
		c.showPurged(purged, log)

		return notDeletedFailure("PURGE-W-FILNOTPUR", notDeleted.Names, purgeFilNotPur)
	}

	if err != nil {
		if lnmErr := logicalNameFailure(err); lnmErr != nil {
			return lnmErr
		}

		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var badLimit *rms.InvalidLimitError
		if errors.As(err, &badLimit) {
			return vmserrors.NewSegment(vmserrors.CLI_IVVALU, fmt.Sprint(badLimit.Limit))
		}

		if _, denied := rms.IsDenied(err); denied {
			text := "PURGE-W-SEARCHFAIL, error searching for " + c.expandedName(specText) + "\n" + ConditionLine(rmsPRV)

			return vmserrors.ConditionError{Status: purgeFilNotPur&^0xFFFF | vmsdef.LibrarySymbols["SHR$_SEARCHFAIL"] | stsInhibitMsg, Text: text}
		}

		// A specification RMS can't parse: SEARCHFAIL over its status, as
		// DELETE's (unconfirmed for PURGE).
		var bad *rms.NameError
		if errors.As(err, &bad) {
			text := "PURGE-W-SEARCHFAIL, error searching for " + strings.ToUpper(bad.Spec) + "\n" + ConditionLine(bad.Status)

			return vmserrors.ConditionError{Status: purgeFilNotPur&^0xFFFF | vmsdef.LibrarySymbols["SHR$_SEARCHFAIL"] | stsInhibitMsg, Text: text}
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	c.showPurged(purged, log)

	return nil
}

// purgeFilNotPur is PURGE's FILNOTPUR condition value, a warning, in
// DELETE's facility, which PURGE is part of (unconfirmed: no probe showed
// its $STATUS).
var purgeFilNotPur = deleteSearchFail&^0xFFFF | vmsdef.LibrarySymbols["SHR$_FILNOTPUR"]

// showPurged writes what PURGE says of the files it purged, as VMS 7.1's
// PURGE does (seen by the author on the simh VAX, 2026-11-22): nothing
// without log, whatever it deleted; with log, each file with its blocks and a
// total when there is more than one, or NOFILPURG when there were none.
func (c *Context) showPurged(purged []rms.DeletedFile, log bool) {
	var blocks uint32

	if !log {
		return
	}

	if len(purged) == 0 {
		c.Printf("%%PURGE-I-NOFILPURG, no files purged\n")

		return
	}

	for _, d := range purged {
		c.Printf("%%PURGE-I-FILPURG, %s deleted (%d block%s)\n", d.Full, d.Blocks, plural(d.Blocks))

		blocks += d.Blocks
	}

	if len(purged) > 1 {
		c.Printf("%%PURGE-I-TOTAL, %d files deleted (%d block%s)\n", len(purged), blocks, plural(blocks))
	}
}
