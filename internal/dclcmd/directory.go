package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
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
// The method below has the receiver "(c *Context)", meaning it's called as
// c.Directory(...) on a *Context value — see context.go's Context type
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
//     matching Context.Dismount's own status for the same underlying
//     condition.
//   - Anything else — a malformed file specification, or a genuine
//     failure reading a matched file's header while gathering size/date
//     detail — is reported as CLI_BADFILESPEC, matching Context.
//     SetDefault's own status for a bad file specification (the CLI
//     facility, not SYS, since these are argument-validation problems
//     rather than a device/volume state problem).
func (c *Context) Directory(specText string, opts rms.DirectoryOptions) error {
	return c.DirectoryTo(specText, opts, "")
}

// DirectoryTo is Directory with /OUTPUT: the listing goes to the file
// output names (default type LIS) in place of the console, or to the
// console when output is "". DIRECTORY/OUTPUT with no file is
// DIRECTORY.LIS (the DCL Dictionary), which the caller passes.
func (c *Context) DirectoryTo(specText string, opts rms.DirectoryOptions, output string) error {
	if opts.OwnerName == nil && c.RTL != nil {
		opts.OwnerName = c.RTL.IdentifierText
	}

	text, found, err := c.ContainerSession.Directory(specText, opts)
	if err != nil {
		if lnmErr := logicalNameFailure(err); lnmErr != nil {
			return lnmErr
		}

		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	if output == "" {
		c.Printf("%s", text)
	} else if err := c.writeDirectoryListing(output, text); err != nil {
		return err
	}

	if !found {
		return vmserrors.ConditionError{Status: directNoFiles, Text: "DIRECT-W-NOFILES, no files found"}
	}

	return nil
}

// writeDirectoryListing writes text, DIRECTORY's listing, as the text
// file name names, its type LIS unless it gives one.
func (c *Context) writeDirectoryListing(name, text string) error {
	s := c.ContainerSession

	out, err := s.Locate(name, false)
	if err != nil {
		return FileFailure(err, name)
	}

	out = out.WithDefaultType("LIS")

	var records [][]byte

	if text != "" {
		for line := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
			records = append(records, []byte(line))
		}
	}

	if _, err := s.CreateRecordFile(out, rms.TextRecords, records); err != nil {
		return objectFailureAs(vmserrors.CLI_BADFILESPEC, err, out.Name)
	}

	return nil
}

// directNoFiles is DIRECTORY's NOFILES condition value, a warning: the
// shared message SHR$_NOFILES in DIRECTORY's facility, taken to be 146
// (unconfirmed against VMS: the facility number, and the message, which
// no probe has shown).
var directNoFiles = 146<<16 | vmsdef.LibrarySymbols["SHR$_NOFILES"]

// DirectorySizeMode is /SIZE's value as an rms.SizeMode: USED,
// ALLOCATION, or ALL, or an abbreviation; another is CLI_IVKEYW.
func DirectorySizeMode(value string) (rms.SizeMode, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	modes := map[string]rms.SizeMode{"USED": rms.SizeUsed, "ALLOCATION": rms.SizeAllocation, "ALL": rms.SizeAll}

	if mode, ok := modes[key]; ok {
		return mode, nil
	}

	var matches []rms.SizeMode

	for name, mode := range modes {
		if key != "" && strings.HasPrefix(name, key) {
			matches = append(matches, mode)
		}
	}

	if len(matches) != 1 {
		return 0, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
	}

	return matches[0], nil
}
