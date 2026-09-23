package console

import "github.com/tucats/govax/internal/vmserrors"

// This file implements the console SET DEFAULT/SHOW DEFAULT commands
// (docs/PHASE-23.md, subtask 3) — the operator-facing half of "remember a
// current default device/directory so a later DIRECTORY/DELETE/PURGE/COPY/
// TYPE command's partial file spec can be filled in from it". The actual
// state and file-specification parsing already live in the sibling
// internal/rms package's Session (session.go); this file is a thin,
// console-specific layer on top of it, mirroring mount.go's own Mount/
// Dismount wrappers around internal/rms.MountTable.
//
// # A note for a reader new to Go
//
// Both methods below have the receiver "(c *Console)", meaning they're
// called as c.SetDefault(...)/c.ShowDefault() on a *Console value — see
// machine.go's own Console type for what state they read and write
// (c.ContainerSession). Returning a plain "error" (Go's built-in error
// interface) is this whole codebase's convention for "something went
// wrong, describe it to the caller"; internal/vmserrors.VMSError (what
// vmserrors.New/vmserrors.Wrap actually construct) implements that
// interface, so it can be returned here exactly like any other error.

// SetDefault establishes text as the operator's new current default
// device/directory (internal/rms.Session.SetDefault's own doc comment
// explains the parsing/inheritance rules in full). A malformed file
// specification is translated from internal/rms's plain Go error into the
// real CLI_BADFILESPEC console status, matching every other console
// command that reports a bad argument this way (e.g. SET RADIX's
// CLI_BADRADIXVAL, SET MODE's CLI_BADMODE — see dispatch.go's cmdSet).
func (c *Console) SetDefault(text string) error {
	if err := c.ContainerSession.SetDefault(text); err != nil {
		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, text)
	}

	return nil
}

// ShowDefault prints the operator's current default device/directory,
// matching real VMS's own SHOW DEFAULT. It never fails — see
// internal/rms.Session.DefaultString's own doc comment on why there's
// always something displayable, even before any SET DEFAULT has run.
func (c *Console) ShowDefault() error {
	c.Printf("  %s\n", c.ContainerSession.DefaultString())

	return nil
}
