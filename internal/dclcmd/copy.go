package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// rmsFEX is RMS$_FEX, "file already exists, not superseded"; rmsPRV
// RMS$_PRV, "insufficient privilege or file protection violation".
var (
	rmsFEX = vmsdef.Symbols["RMS$_FEX"]
	rmsPRV = vmsdef.Symbols["RMS$_PRV"]

	// rmsFUL and ssDEVICEFULL are a full volume's status and STV.
	rmsFUL       = vmsdef.Symbols["RMS$_FUL"]
	ssDEVICEFULL = vmsdef.Symbols["SS$_DEVICEFULL"]
)

// This file implements the console COPY command (docs/planning/PHASE-23.md,
// subtasks 9 and 10) — the operator-facing half of "copy one or more
// files, between a mounted ODS-2 volume and the host filesystem, or
// between two locations on mounted volumes". The actual work (the
// four-direction logic /HOST selects, wildcarded-source-onto-a-directory
// copying, and every other qualifier's own behavior) already lives in the
// sibling internal/rms package's Session.Copy (internal/rms/copy.go);
// this file is a thin, console-specific layer on top of it, mirroring
// Context.Type/Context.Delete's own "rms computes, console prints"
// pattern — extended here to a loop over one result per file, matching
// Context.Delete's own []DeletedFile handling, since a wildcarded COPY
// can touch several files in one command.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Context)", meaning it's called as
// c.Copy(...) on a *Context value. Like Context.Type/Context.Delete, it
// tells Session.Copy's different failure modes apart via errors.As (rather
// than inspecting error text) so each is reported as the specific VMS
// status a real operator would see for that exact condition.

// Copy copies sourceText to destText, in whichever of the four directions
// sourceHost/destHost select, applying every qualifier opts carries — see
// internal/rms.Session.Copy's own doc comment for exactly what each
// direction/qualifier combination means and how each endpoint is
// resolved.
//
// On success, it prints one line per file (or directory) Session.Copy
// reports touching, in whichever of four shapes that result's own Kind
// calls for:
//
//   - rms.CopyCopied: "%COPY-I-COPYING, copying SOURCE to DEST" first if
//     opts.Verbose, then "%COPY-S-COPIED, SOURCE copied to DEST" under
//     opts.Log or opts.Verbose, unless opts.Quiet (copyLogs; VMS's form,
//     each name in full with the blocks copied, for a volume-to-volume
//     copy under /LOG) — plus, if the result carries a non-empty Warning (a
//     /TIME failure that didn't stop the copy — see CopyResult.Warning's
//     own doc comment), a further "%COPY-W-NOTIME, ..." line, but only
//     when opts.Verbose, matching ods2's own cmdCopy convention for the
//     same warning.
//   - rms.CopyTested: always "%COPY-I-TEST, would copy SOURCE to DEST",
//     regardless of opts.Quiet/opts.Verbose (matching ods2's own cmdCopy,
//     which checks /TEST before either of those).
//   - rms.CopyDirCreated / rms.CopyDirTested: the same two patterns as
//     CopyCopied/CopyTested above, but worded for a directory entry
//     materialized under /DIRS ("...creating directory DEST for
//     SOURCE"/"SOURCE materialized as directory DEST"/"would create
//     directory for SOURCE at DEST").
//
// Session.Copy's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - Either sourceText or destText naming a device with nothing
//     currently mounted on it surfaces as an *rms.NotMountedError and is
//     reported as SS_DEVNOTMOUNT, matching Context.Type/Context.Delete's
//     own translation for the same underlying condition.
//   - sourceText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as COPY-E-SEARCHFAIL over
//     RMS$_FNF (or RMS$_DNF), as DELETE and TYPE report theirs.
//   - sourceText matching more than one file, against a destination that
//     doesn't accept several, surfaces as an *rms.MultipleMatchesError
//     and is reported as CLI_AMBIGUOUS, the same status Context.Type
//     already uses for its own single-match restriction
//     (*rms.AmbiguousError) — a different Go type, since Session.Copy's
//     own error text is worded for COPY rather than TYPE, but the same
//     VMS status either way.
//   - Both sourceHost and destHost set (no container endpoint at all)
//     surfaces as an *rms.HostToHostError and is reported as
//     CLI_BADQUALIFIERCOMBO.
//   - Anything else (a malformed file specification, a host path that
//     doesn't exist or names a directory instead of a file, or a genuine
//     I/O failure reading or writing a file's content) is reported as
//     CLI_BADFILESPEC, matching Context.Type/Context.Delete/Context.
//     Directory's own catch-all bucket.
func (c *Context) Copy(sourceText string, sourceHost bool, destText string, destHost bool, opts rms.CopyOptions) error {
	// /PROTECTION's code, checked before anything is copied: a category
	// that isn't S, O, G, or W is DCL's IVKEYW, other bad letters
	// COPY-F-SYNTAX.
	if _, err := qualifierProtection([]string{opts.Protection}, copySyntax); err != nil {
		return err
	}

	results, err := c.copyFiles(sourceText, sourceHost, destText, destHost, opts)
	if err != nil {
		if lnmErr := logicalNameFailure(err); lnmErr != nil {
			return lnmErr
		}

		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		// An input that isn't there is SEARCHFAIL with the name expanded,
		// then RMS$_FNF (RMS$_DNF for a directory that isn't there), as
		// DELETE's and TYPE's are: an error, which ends a procedure or a
		// batch job under DCL's default ON ERROR (unconfirmed: no probe
		// copied a missing file; the severity is taken from VMS's own
		// COPY sessions as remembered, not a log).
		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			missing := c.ContainerSession.NotFound(sourceText)

			text := "COPY-E-SEARCHFAIL, error searching for " + missing.Name + "\n" + ConditionLine(missing.Status)
			if missing.STV != 0 {
				text += "\n" + ConditionLine(missing.STV)
			}

			return vmserrors.ConditionError{Status: copySearchFail | stsInhibitMsg, Text: text}
		}

		var multiple *rms.MultipleMatchesError
		if errors.As(err, &multiple) {
			return vmserrors.Wrap(vmserrors.CLI_AMBIGUOUS, err, "file specification", sourceText)
		}

		// VMS's COPY never supersedes a file (unconfirmed against VMS:
		// the status, taken to be RMS$_FEX's, marked as shown).
		var exists *rms.OutputExistsError
		if errors.As(err, &exists) {
			text := "COPY-E-OPENOUT, error opening " + exists.Spec + " as output\n" + ConditionLine(rmsFEX)

			return vmserrors.ConditionError{Status: rmsFEX, Text: text}
		}

		// A refusal by the process's protection (docs/planning/PHASE-93 -
		// file protection.md, A.4): VMS 7.1's COPY said OPENOUT over
		// RMS$_PRV for an output it may not make, then NOTCOPIED for the
		// input. A directory it may not search is SEARCHFAIL, as TYPE's
		// is (unconfirmed: the probe didn't copy from one).
		if denied, ok := rms.IsDenied(err); ok && denied != nil {
			switch {
			case denied.Output:
				text := "COPY-E-OPENOUT, error opening " + denied.Name + " as output\n" + ConditionLine(rmsPRV)
				if denied.Source != "" {
					text += "\n%COPY-W-NOTCOPIED, " + denied.Source + " not copied"
				}

				return vmserrors.ConditionError{Status: rmsPRV, Text: text}
			case denied.Host:
				// A host input the process may not use (vax.host.sysprv),
				// shown as a volume file it may not read is.
				text := "COPY-E-OPENIN, error opening " + denied.Name + " as input\n" + ConditionLine(rmsPRV)

				return vmserrors.ConditionError{Status: rmsPRV, Text: text}
			case denied.Search:
				text := "COPY-E-SEARCHFAIL, error searching for " + c.expandedName(sourceText) + "\n" + ConditionLine(rmsPRV)

				return vmserrors.ConditionError{Status: rmsPRV, Text: text}
			}
		}

		var input *rms.InputOpenError
		if errors.As(err, &input) {
			text := "COPY-E-OPENIN, error opening " + input.Spec + " as input\n" + ConditionLine(input.Status)

			return vmserrors.ConditionError{Status: input.Status, Text: text}
		}

		var hostToHost *rms.HostToHostError
		if errors.As(err, &hostToHost) {
			return vmserrors.Wrap(vmserrors.CLI_BADQUALIFIERCOMBO, err, "SOURCE/HOST", "DESTINATION/HOST")
		}

		var records *rms.HostRecordsError
		if errors.As(err, &records) {
			return vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, "RECORD_FORMAT")
		}

		// A specification RMS can't parse: SEARCHFAIL for the input's, as a
		// missing input's is, OPENOUT for the output's (unconfirmed which
		// VMS gives each).
		var bad *rms.NameError
		if errors.As(err, &bad) {
			text := "COPY-E-OPENOUT, error opening " + strings.ToUpper(bad.Spec) + " as output\n" + ConditionLine(bad.Status)
			if strings.EqualFold(bad.Spec, sourceText) {
				text = "COPY-E-SEARCHFAIL, error searching for " + strings.ToUpper(bad.Spec) + "\n" + ConditionLine(bad.Status)
			}

			return vmserrors.ConditionError{Status: bad.Status, Text: text}
		}

		// A volume with no room for the output: WRITEERR naming it, then
		// RMS$_FUL and its STV (unconfirmed: docs/planning/PHASE-113 -
		// Fortran errata.md, subtask 11; the file's blocks written so far
		// are left, as govax writes them).
		var full *rms.DeviceFullError
		if errors.As(err, &full) {
			text := "COPY-E-WRITEERR, error writing " + full.Spec + "\n" + ConditionLine(rmsFUL) + "\n" + ConditionLine(ssDEVICEFULL)

			return vmserrors.ConditionError{Status: rmsFUL, Text: text}
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, sourceText)
	}

	for _, r := range results {
		c.printCopyResult(r, opts)
	}

	return nil
}

// copyFiles decides which side of the copy each name is on by the rules
// every command shares (rms.Session.Locate: /HOST, a VMS specification, a
// host path with "/" or "\", then the default directory's side), and
// copies. Without it a name like "./notes.txt" was taken for a file in
// the default directory of a volume (docs/planning/PHASE-69 - backup save
// sets.md, side finding 1).
func (c *Context) copyFiles(sourceText string, sourceHost bool, destText string, destHost bool, opts rms.CopyOptions) ([]rms.CopyResult, error) {
	source, err := c.ContainerSession.Locate(sourceText, sourceHost)
	if err != nil {
		return nil, err
	}

	dest, err := c.ContainerSession.LocateOutput(destText, destHost)
	if err != nil {
		// A host output the process may not make (vax.host.sysprv) is
		// shown as a volume one it may not make is, with the input not
		// copied.
		if denied, ok := rms.IsDenied(err); ok && denied != nil && denied.Host {
			denied.Source = sourceText
			if !source.Host {
				denied.Source = c.expandedName(sourceText)
			}
		}

		return nil, err
	}

	return c.ContainerSession.Copy(sourceText, source.Host, destText, dest.Host, opts)
}

// copyFacility is COPY's facility number, which its own condition values
// carry (unconfirmed: the probe shows COPY's messages, not their values).
const copyFacility = 103

// copySearchFail is COPY's SEARCHFAIL condition value, an error (the
// shared message SHR$_SEARCHFAIL in COPY's facility).
var copySearchFail = uint32(copyFacility<<16) | vmsdef.LibrarySymbols["SHR$_SEARCHFAIL"]&^7 | 2

// copySyntax is COPY's refusal of a /PROTECTION code's letters,
// "%COPY-F-SYNTAX, error parsing 'Z'", as CREATE's is (unconfirmed: the
// probe gave COPY none).
func copySyntax(part string) error {
	status := uint32(copyFacility<<16) | vmsdef.LibrarySymbols["SHR$_SYNTAX"]&^7 | 4

	return vmserrors.ConditionError{Status: status, Text: "COPY-F-SYNTAX, error parsing '" + part + "'"}
}

// printCopyResult prints whichever of the four "%COPY-..." line shapes
// r.Kind calls for — see Context.Copy's own doc comment for each one's
// exact wording and quiet/verbose gating.
// copyLogs says whether COPY reports each file it copied: only under /LOG,
// as VMS's COPY does, or govax's own /VERBOSE, and never under /QUIET
// (docs/planning/PHASE-113 - Fortran errata.md, subtask 11; until then
// govax said COPIED unless /NOLOG).
func copyLogs(opts rms.CopyOptions) bool {
	return (opts.Log || opts.Verbose) && !opts.Quiet
}

func (c *Context) printCopyResult(r rms.CopyResult, opts rms.CopyOptions) {
	switch r.Kind {
	case rms.CopyTested:
		c.Printf("%%COPY-I-TEST, would copy %s to %s\n", r.Source, r.Dest)

	case rms.CopyDirTested:
		c.Printf("%%COPY-I-TEST, would create directory for %s at %s\n", r.Source, r.Dest)

	case rms.CopyDirCreated:
		if opts.Verbose {
			c.Printf("%%COPY-I-COPYING, creating directory %s for %s\n", r.Dest, r.Source)
		}

		if copyLogs(opts) {
			c.Printf("%%COPY-S-COPIED, %s materialized as directory %s\n", r.Source, r.Dest)
		}

	default: // rms.CopyCopied
		if opts.Log && !opts.Quiet && r.FullSource != "" {
			c.Printf("%%COPY-S-COPIED, %s copied to %s (%d block%s)\n", r.FullSource, r.FullDest, r.Blocks, plural(r.Blocks))

			return
		}

		if opts.Verbose {
			c.Printf("%%COPY-I-COPYING, copying %s to %s\n", r.Source, r.Dest)
		}

		if copyLogs(opts) {
			c.Printf("%%COPY-S-COPIED, %s copied to %s\n", r.Source, r.Dest)
		}

		if r.Warning != "" && opts.Verbose {
			c.Printf("%%COPY-W-NOTIME, %s\n", r.Warning)
		}
	}
}
