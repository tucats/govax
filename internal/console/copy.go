package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console COPY command (docs/PHASE-23.md,
// subtask 9) — the operator-facing half of "copy one file, between a
// mounted ODS-2 volume and the host filesystem, or between two locations
// on mounted volumes". The actual work (the four-direction logic /HOST
// selects, the text-reframing content copy) already lives in the sibling
// internal/rms package's Session.Copy (internal/rms/copy.go, this same
// subtask); this file is a thin, console-specific layer on top of it,
// mirroring Console.Type/Console.Delete's own "rms computes, console
// prints" pattern.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Copy(...) on a *Console value. Like Console.Type/Console.Delete, it
// tells Session.Copy's different failure modes apart via errors.As (rather
// than inspecting error text) so each is reported as the specific VMS
// status a real operator would see for that exact condition.

// Copy copies sourceText to destText, in whichever of the four directions
// sourceHost/destHost select — see internal/rms.Session.Copy's own doc
// comment for exactly what each combination means and how each endpoint
// is resolved. On success, it prints a single "%COPY-S-COPIED, SOURCE
// copied to DEST" confirmation line, matching ods2's own cmdCopy
// convention (subtask 10 will add /QUIET to suppress this and /VERBOSE
// to print a matching line before the copy begins, matching ods2's own
// qualifier set — neither exists yet).
//
// Session.Copy's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - Either sourceText or destText naming a device with nothing
//     currently mounted on it surfaces as an *rms.NotMountedError and is
//     reported as SS_DEVNOTMOUNT, matching Console.Type/Console.Delete's
//     own translation for the same underlying condition.
//   - sourceText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as SS_NOSUCHFILE, matching
//     Console.Type/Console.Delete's own translation for the same
//     underlying condition.
//   - sourceText matching more than one file surfaces as an
//     *rms.MultipleMatchesError and is reported as CLI_AMBIGUOUS, the
//     same status Console.Type already uses for its own single-match
//     restriction (*rms.AmbiguousError) — a different Go type, since
//     Session.Copy's own error text is worded for COPY rather than TYPE,
//     but the same VMS status either way.
//   - Both sourceHost and destHost set (no container endpoint at all)
//     surfaces as an *rms.HostToHostError and is reported as
//     CLI_BADQUALIFIERCOMBO.
//   - Anything else (a malformed file specification, a host path that
//     doesn't exist or names a directory instead of a file, or a genuine
//     I/O failure reading or writing a file's content) is reported as
//     CLI_BADFILESPEC, matching Console.Type/Console.Delete/Console.
//     Directory's own catch-all bucket.
func (c *Console) Copy(sourceText string, sourceHost bool, destText string, destHost bool) error {
	result, err := c.ContainerSession.Copy(sourceText, sourceHost, destText, destHost)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, notFound.Spec)
		}

		var multiple *rms.MultipleMatchesError
		if errors.As(err, &multiple) {
			return vmserrors.Wrap(vmserrors.CLI_AMBIGUOUS, err, "file specification", sourceText)
		}

		var hostToHost *rms.HostToHostError
		if errors.As(err, &hostToHost) {
			return vmserrors.Wrap(vmserrors.CLI_BADQUALIFIERCOMBO, err, "SOURCE/HOST", "DESTINATION/HOST")
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, sourceText)
	}

	c.Printf("%%COPY-S-COPIED, %s copied to %s\n", result.Source, result.Dest)

	return nil
}
