package dclcmd

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console DELETE command (docs/PHASE-23.md,
// subtask 6) — the operator-facing half of "reclaim a file's storage and
// remove its directory entry". The actual work (glob-matching, the
// required-version check, the DeleteFile/bitmap-flush loop) already lives
// in the sibling internal/rms package's Session.Delete (internal/rms/
// delete.go, this same subtask); this file is a thin, console-specific
// layer on top of it, mirroring directory.go's own Context.Directory.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Context)", meaning it's called as
// c.Delete(...) on a *Context value. Like Context.Directory, it tells
// Session.Delete's different failure modes apart via errors.As (rather than
// inspecting error text) so each is reported as the specific VMS status a
// real operator would see for that exact condition.

// deleteSearchFail is DELETE's SEARCHFAIL condition value, a warning:
// the shared message SHR$_SEARCHFAIL in DELETE's facility, taken to be
// 147 (unconfirmed against VMS: no probe has shown DELETE's $STATUS).
var deleteSearchFail = 147<<16 | vmsdef.LibrarySymbols["SHR$_SEARCHFAIL"]

// Delete removes every file matching specText from its resolved volume
// (see internal/rms.Session.Delete's own doc comment for exactly how
// specText is resolved, and why it must include a specific version). As
// VMS's DELETE, it says nothing of a file it deletes unless log is set
// (/LOG), when each gets
//
//	%DELETE-I-FILDEL, DUA0:[WORK]X.DAT;1 deleted (3 blocks)
//
// its blocks being those it had allocated (unconfirmed against VMS:
// allocated, not used).
//
// Session.Delete's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError and is reported as
//     SS_DEVNOTMOUNT, matching Context.Directory's own translation for the
//     same underlying condition.
//   - specText with no specific version surfaces as an
//     *rms.VersionRequiredError and is reported as CLI_NEEDVERSION.
//   - specText matching no file at all surfaces as an *rms.NotFoundError
//     and is reported as VMS 7.3's DELETE reported it (testdata/dcl50's
//     rounds 4 and 6): SEARCHFAIL with the name expanded, then RMS$_FNF
//     (RMS$_DNF for a directory that isn't there), a warning.
//   - Anything else (a malformed file specification, an unsupported multi-
//     device volume set, or a genuine I/O failure freeing storage) is
//     reported as CLI_BADFILESPEC, matching Context.Directory's own
//     catch-all bucket.
func (c *Context) Delete(specText string, log bool) error {
	deleted, err := c.ContainerSession.Delete(specText)

	if log {
		for _, d := range deleted {
			c.Printf("%%DELETE-I-FILDEL, %s deleted (%d block%s)\n", d.Full, d.Blocks, plural(d.Blocks))
		}
	}

	if err != nil {
		if lnmErr := logicalNameFailure(err); lnmErr != nil {
			return lnmErr
		}

		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var versionRequired *rms.VersionRequiredError
		if errors.As(err, &versionRequired) {
			return vmserrors.Wrap(vmserrors.CLI_NEEDVERSION, err, versionRequired.Spec)
		}

		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			missing := c.ContainerSession.NotFound(specText)

			text := "DELETE-W-SEARCHFAIL, error searching for " + missing.Name + "\n" + ConditionLine(missing.Status)
			if missing.STV != 0 {
				text += "\n" + ConditionLine(missing.STV)
			}

			return vmserrors.ConditionError{Status: deleteSearchFail | stsInhibitMsg, Text: text}
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	return nil
}

// plural is "s" unless n is 1.
func plural[T ~int | ~uint32](n T) string {
	if n == 1 {
		return ""
	}

	return "s"
}
