package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// typeOpenIn is TYPE's OPENIN condition value, a warning: the shared
// message SHR$_OPENIN in TYPE's facility, 149 (VMS 7.3's TYPE gave
// %X10951238, its SEARCHFAIL, in testdata/dcl50's round 1).
var typeOpenIn = 149<<16 | vmsdef.LibrarySymbols["SHR$_OPENIN"]

// This file implements the console TYPE command (docs/PHASE-23.md,
// subtask 8) — the operator-facing half of "write one file's content to
// the console". The actual work (glob-matching, the exactly-one-match
// rule, record-format-aware text rendering) already lives in the sibling
// internal/rms package's Session.Type (internal/rms/type.go, this same
// subtask); this file is a thin, console-specific layer on top of it,
// mirroring directory.go's own Context.Directory ("rms computes, console
// prints").
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Context)", meaning it's called as
// c.Type(...) on a *Context value. Like Context.Delete, it tells
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
//     SS_DEVNOTMOUNT, matching Context.Delete's own translation for the
//     same underlying condition.
//   - specText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as SS_NOSUCHFILE, matching
//     Context.Delete's own translation for the same underlying condition.
//   - specText matching more than one file surfaces as an
//     *rms.AmbiguousError and is reported as CLI_AMBIGUOUS, the same
//     status internal/dcl's own verb/qualifier ambiguity already
//     uses.
//   - Anything else (a malformed file specification, or a genuine failure
//     reading the file's own record data) is reported as CLI_BADFILESPEC,
//     matching Context.Delete/Context.Directory's own catch-all bucket.
//
// With paged (TYPE/PAGE), the text is shown a screenful at a time, as
// Context.Page shows it (page.go).
//
// Where specText is, on the host or on a mounted volume, follows
// rms.Session.Locate's rules (a host path, or a bare name when the default
// directory isn't on a volume, is a host file); host (/HOST) makes it a
// host file whatever it looks like. A host file's lines are typed as they
// are, each ending in LF (a CR before it dropped). A host file that
// doesn't exist is SS_NOSUCHFILE, as a volume file is.
//
// SYS$INPUT, or a logical name that translates to it, is the process's
// input, typed a line at a time (typeInput): in a command procedure, the
// data lines after the command. A logical name DCL's OPEN defined types
// the file it opened, from where READ has got to (typeOpenFile). Neither
// applies with host.
func (c *Context) Type(specText string, host, paged bool) error {
	if !host {
		if handled, err := c.TypeDCL(specText); handled {
			return err
		}
	}

	loc, err := c.ContainerSession.Locate(specText, host)
	if err != nil {
		return typeFailure(err, specText)
	}

	var text string

	if loc.Host {
		text, err = typeHostFile(c.ContainerSession, loc)
	} else {
		text, err = c.ContainerSession.Type(specText)
	}

	if err != nil {
		return typeFailure(err, specText)
	}

	if paged {
		c.Page(text)

		return nil
	}

	c.Printf("%s", text)

	return nil
}

// typeHostFile reads the host file at loc as text lines and returns them
// joined, each ending in LF.
func typeHostFile(s *rms.Session, loc rms.FileLocation) (string, error) {
	records, _, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return "", err
	}

	var b strings.Builder

	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// typeFailure turns an error from finding or reading TYPE's file into the
// VMS status a real operator would see for that specific condition (see
// Type's doc comment); everything but a locked file is fileFailure's.
func typeFailure(err error, specText string) error {
	// The file another opener keeps readers out of: TYPE's message
	// for it, as VMS 7.3 showed it (testdata/dcl50's F9), with the
	// condition value TYPE's image signals, its message shown.
	var locked *rms.LockedError
	if errors.As(err, &locked) {
		return vmserrors.ConditionError{
			Status: typeOpenIn | stsInhibitMsg,
			Text:   "TYPE-W-OPENIN, error opening " + locked.Name + " as input\n" + ConditionLine(vmsdef.Symbols["RMS$_FLK"]),
		}
	}

	return FileFailure(err, specText)
}
