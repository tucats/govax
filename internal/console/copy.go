package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console COPY command (docs/PHASE-23.md,
// subtasks 9 and 10) — the operator-facing half of "copy one or more
// files, between a mounted ODS-2 volume and the host filesystem, or
// between two locations on mounted volumes". The actual work (the
// four-direction logic /HOST selects, wildcarded-source-onto-a-directory
// copying, and every other qualifier's own behavior) already lives in the
// sibling internal/rms package's Session.Copy (internal/rms/copy.go);
// this file is a thin, console-specific layer on top of it, mirroring
// Console.Type/Console.Delete's own "rms computes, console prints"
// pattern — extended here to a loop over one result per file, matching
// Console.Delete's own []DeletedFile handling, since a wildcarded COPY
// can touch several files in one command.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Copy(...) on a *Console value. Like Console.Type/Console.Delete, it
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
//     opts.Verbose, then "%COPY-S-COPIED, SOURCE copied to DEST" unless
//     opts.Quiet — plus, if the result carries a non-empty Warning (a
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
//     reported as SS_DEVNOTMOUNT, matching Console.Type/Console.Delete's
//     own translation for the same underlying condition.
//   - sourceText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as SS_NOSUCHFILE, matching
//     Console.Type/Console.Delete's own translation for the same
//     underlying condition.
//   - sourceText matching more than one file, against a destination that
//     doesn't accept several, surfaces as an *rms.MultipleMatchesError
//     and is reported as CLI_AMBIGUOUS, the same status Console.Type
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
//     CLI_BADFILESPEC, matching Console.Type/Console.Delete/Console.
//     Directory's own catch-all bucket.
func (c *Console) Copy(sourceText string, sourceHost bool, destText string, destHost bool, opts rms.CopyOptions) error {
	results, err := c.ContainerSession.Copy(sourceText, sourceHost, destText, destHost, opts)
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

	for _, r := range results {
		c.printCopyResult(r, opts)
	}

	return nil
}

// printCopyResult prints whichever of the four "%COPY-..." line shapes
// r.Kind calls for — see Console.Copy's own doc comment for each one's
// exact wording and quiet/verbose gating.
func (c *Console) printCopyResult(r rms.CopyResult, opts rms.CopyOptions) {
	switch r.Kind {
	case rms.CopyTested:
		c.Printf("%%COPY-I-TEST, would copy %s to %s\n", r.Source, r.Dest)

	case rms.CopyDirTested:
		c.Printf("%%COPY-I-TEST, would create directory for %s at %s\n", r.Source, r.Dest)

	case rms.CopyDirCreated:
		if opts.Verbose {
			c.Printf("%%COPY-I-COPYING, creating directory %s for %s\n", r.Dest, r.Source)
		}

		if !opts.Quiet {
			c.Printf("%%COPY-S-COPIED, %s materialized as directory %s\n", r.Source, r.Dest)
		}

	default: // rms.CopyCopied
		if opts.Verbose {
			c.Printf("%%COPY-I-COPYING, copying %s to %s\n", r.Source, r.Dest)
		}

		if !opts.Quiet {
			c.Printf("%%COPY-S-COPIED, %s copied to %s\n", r.Source, r.Dest)
		}

		if r.Warning != "" && opts.Verbose {
			c.Printf("%%COPY-W-NOTIME, %s\n", r.Warning)
		}
	}
}
