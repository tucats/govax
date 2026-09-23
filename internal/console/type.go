package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console TYPE command (docs/PHASE-23.md,
// subtask 8) — the operator-facing half of "write one file's content to
// the console". The actual work (glob-matching, the exactly-one-match
// rule, record-format-aware text rendering) already lives in the sibling
// internal/rms package's Session.Type (internal/rms/type.go, this same
// subtask); this file is a thin, console-specific layer on top of it,
// mirroring directory.go's own Console.Directory ("rms computes, console
// prints").
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Type(...) on a *Console value. Like Console.Delete, it tells
// Session.Type's different failure modes apart via errors.As (rather than
// inspecting error text) so each is reported as the specific VMS status a
// real operator would see for that exact condition.

// Type prints specText's resolved file's entire content as text to the
// console's output stream — see internal/rms.Session.Type's own doc
// comment for exactly how specText is resolved, and why it must match
// exactly one file.
//
// Session.Type's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError and is reported as
//     SS_DEVNOTMOUNT, matching Console.Delete's own translation for the
//     same underlying condition.
//   - specText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as SS_NOSUCHFILE, matching
//     Console.Delete's own translation for the same underlying condition.
//   - specText matching more than one file surfaces as an
//     *rms.AmbiguousError and is reported as CLI_AMBIGUOUS, the same
//     status internal/console/dcl's own verb/qualifier ambiguity already
//     uses.
//   - Anything else (a malformed file specification, or a genuine failure
//     reading the file's own record data) is reported as CLI_BADFILESPEC,
//     matching Console.Delete/Console.Directory's own catch-all bucket.
func (c *Console) Type(specText string) error {
	text, err := c.ContainerSession.Type(specText)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, notFound.Spec)
		}

		var ambiguous *rms.AmbiguousError
		if errors.As(err, &ambiguous) {
			return vmserrors.Wrap(vmserrors.CLI_AMBIGUOUS, err, "file specification", specText)
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	c.Printf("%s", text)

	return nil
}
