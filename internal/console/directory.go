package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console DIRECTORY command (docs/PHASE-23.md,
// subtask 5) — the operator-facing half of "list the files on a mounted
// volume". The actual listing logic (glob-matching, per-directory
// grouping, /FULL/FILE/SIZE/DATE formatting) already lives in the sibling
// internal/rms package's Session.Directory (internal/rms/directory.go,
// this same subtask); this file is a thin, console-specific layer on top
// of it, mirroring default.go's SetDefault/ShowDefault wrappers around
// internal/rms.Session.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Directory(...) on a *Console value — see machine.go's own Console type
// for what state it reads (c.ContainerSession). Unlike ShowDefault (which
// prints unconditionally, since internal/rms.Session.DefaultString never
// fails), Directory has to check for two different kinds of failure
// first — see its own doc comment below for why they're reported as two
// different VMS statuses rather than collapsed into one.

// Directory prints the listing of every file matching specText (see
// internal/rms.Session.Directory's own doc comment for exactly how
// specText is resolved, and what opts controls) to the console's output
// stream.
//
// internal/rms.Session.Directory's failures are told apart here so each is
// reported as the VMS status a real operator would actually see for that
// specific condition, rather than one generic error for both:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError (recognized here via errors.As,
//     not by inspecting error text) and is reported as SS_DEVNOTMOUNT,
//     matching Console.Dismount's own status for the same underlying
//     condition.
//   - Anything else — a malformed file specification, or a genuine
//     failure reading a matched file's header while gathering size/date
//     detail — is reported as CLI_BADFILESPEC, matching Console.
//     SetDefault's own status for a bad file specification (the CLI
//     facility, not SYS, since these are argument-validation problems
//     rather than a device/volume state problem).
func (c *Console) Directory(specText string, opts rms.DirectoryOptions) error {
	text, err := c.ContainerSession.Directory(specText, opts)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	c.Printf("%s", text)

	return nil
}
