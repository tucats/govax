package rms

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/PHASE-23.md subtask 9 (COPY's core direction
// logic and its /HOST qualifier, single-match-only) and subtask 10 (the
// remaining qualifier parity -- /BINARY, /QUIET, /VERBOSE, /TEST, /TIME,
// /IGNORE, /DIRS, /STREAM, /VFC, /CRLF, /LF -- plus lifting the
// single-match restriction for a wildcarded source copied onto a directory
// destination, resolving the design section's own open question in favor
// of landing it alongside /DIRS) -- the operator-console command that
// moves one or more files
// between a mounted ODS-2 volume and the host filesystem, or between two
// locations on mounted volumes. Its behavioral reference is the sibling
// ods2 module's own cmd/ods2/internal/session/copy.go (read-only, per
// this phase's own "behavioral spec, not code to link against" framing --
// see docs/PHASE-23.md's "Why this phase looks different from most
// others"); the functions below follow its shape closely (the same
// CreateFile-then-reframe-as-text approach, the same Stream_LF default,
// the same per-qualifier restrictions) without being a literal port.
//
// # A note for a reader new to Go and to this project's COPY design
//
// Real VMS's own COPY (and ods2's own version of it) only ever has one
// "direction switch": is the destination a plain host path, or VMS syntax
// naming a mounted volume? govax's own COPY works differently -- see docs/
// PHASE-23.md's "COPY direction and the /HOST qualifier" design section --
// SOURCE and SOURCE alone decides whether *it* is a host path (via its own
// /HOST qualifier, attached to the SOURCE parameter specifically -- see
// internal/console/dcl's parameter-scoped-qualifier feature), and
// DESTINATION decides the same for itself, completely independently. With
// neither /HOST present, both sides are VMS syntax naming a mounted
// volume. That gives four combinations, each handled by its own function
// below: copyVolumeToVolume, copyFromHost, copyToHost, and the one
// combination that's rejected outright, both sides /HOST at once (see
// Copy's own doc comment).
//
// # Multi-file ("wildcard") copying
//
// A SOURCE naming a container file spec (never a /HOST source -- host
// paths aren't VMS-wildcarded in this project, matching ods2's own
// cmdCopyFromHost, which only ever takes a single host path) may match
// more than one file. That's only accepted when DESTINATION "is a
// directory": for a volume destination, a spec naming no file of its own
// at all (just a device/directory, e.g. "DUA0:[SUBDIR]"); for a host
// destination, an already-existing host directory. Every matched file is
// then copied into that directory under its own name/type (and, on the
// host side, its own version too, since a host directory can hold several
// files sharing a VMS name that would otherwise collide as different
// versions of the same file). A source matching several files against
// any other kind of destination fails with a *MultipleMatchesError --
// matching ods2's own destAcceptsMultiple restriction, minus its further
// destination-name wildcard-substitution case (e.g. copying "*.TXT" onto
// "*.OLD" to rename every match's type), which this project doesn't
// implement (no example in this project's own design doc calls for it).
//
// # Qualifier scope, per direction
//
// Matching ods2's own cmdCopy/cmdCopyFromHost doc comments almost exactly
// (see docs/PHASE-23.md's "COPY qualifier parity" design section):
//
//   - /BINARY, /QUIET, /VERBOSE, /TEST apply to every direction.
//   - /TIME, /IGNORE, /DIRS, /STREAM, /CRLF, /LF only have an effect when
//     the destination is a plain host path (copyToHost) -- a volume
//     destination has no host mtime to preserve, no "corrupt record,
//     fall back to raw bytes" recovery concept of its own (a volume
//     destination is always written from an in-memory rendering, not
//     re-parsed), no subdirectory-mirroring host path segments to create,
//     no notion of "the source's original Stream framing" worth
//     preserving verbatim, and no host-text-file line-ending convention
//     to choose between (its records are always Stream_LF on disk
//     regardless).
//   - /VFC is accepted everywhere for command-line compatibility but has
//     no effect anywhere: unlike ods2 (where VFC interpretation is
//     opt-in), this project's default text-mode copy always expands a
//     VFC file's carriage control the same way TYPE does -- there's no
//     "un-interpreted" text mode /VFC could opt out of. It isn't even
//     read by Go code here; see CopyOptions' own doc comment.

// CopyOptions collects COPY's own qualifiers (docs/PHASE-23.md subtask
// 10), gathered into one value rather than a long, easy-to-transpose list
// of bool parameters -- matching ods2's own copyOptions in spirit, though
// this one also carries Quiet/Verbose/Test, which ods2's version doesn't
// (ods2's cmdCopy reads those three straight off its own Qualifiers value
// instead).
//
// Quiet and Verbose are deliberately never read anywhere in this file:
// they only affect which lines Console.Copy prints once Session.Copy
// returns (internal/console/copy.go), not what actually gets copied, so
// they're carried here purely so dispatch.go (internal/console/
// dispatch.go) only has to build one CopyOptions value from the parsed
// command line and thread it straight through Console.Copy into
// Session.Copy unchanged -- simpler than two separate options types with
// the same qualifiers split awkwardly between them. The grammar's own VFC
// qualifier isn't even represented here at all, for the same "accepted
// but never consulted" reason its own doc comment above explains --
// dispatch.go reads r.Present("VFC") only to note in its own comment why
// it's discarded, not to populate any field here.
type CopyOptions struct {
	// Binary requests a raw, byte-for-byte copy (an Undefined-format
	// destination file on a volume, or the source's exact on-disk bytes
	// verbatim on a host destination) instead of the default text
	// reframing every other direction/format combination uses.
	Binary bool

	// Quiet suppresses the per-file "%COPY-S-COPIED" (or
	// "...materialized as directory...") confirmation line Console.Copy
	// would otherwise print. Never read by this file -- see this type's
	// own doc comment above.
	Quiet bool

	// Verbose additionally prints a "%COPY-I-COPYING" (or "...creating
	// directory...") line before each file/directory is actually
	// touched. Never read by this file -- see this type's own doc
	// comment above.
	Verbose bool

	// Test previews what Copy would do -- for each match, a single
	// "%COPY-I-TEST" line describing the copy or directory-creation that
	// would happen -- without actually reading, writing, or creating
	// anything at all.
	Test bool

	// Time preserves a copied file's VMS revision date as the resulting
	// host file's modification time (os.Chtimes) -- only meaningful for
	// a host destination (copyToHost); ignored everywhere else, matching
	// ods2's own cmdCopy doc comment ("no host mtime to preserve" for a
	// volume destination). A failure to preserve the time is not fatal
	// (matching ods2's own "%COPY-W-NOTIME" warning, which doesn't stop
	// the copy from being reported as successful) -- see CopyResult.
	// Warning.
	Time bool

	// Ignore recovers from a corrupt record encountered while rendering
	// a container source's content as text (records.go's writeRecords)
	// by restarting the destination host file from scratch as an exact,
	// unrendered byte copy instead of leaving a partially written text
	// file behind. Only meaningful for a host destination.
	Ignore bool

	// Dirs, for a host destination only, does two things at once,
	// matching ods2's own cmdCopy exactly: (1) a directory entry among
	// the matched files (e.g. a subdirectory named by a recursive source
	// spec) is materialized as an empty host directory instead of being
	// silently skipped; (2) an ordinary matched file found in a
	// subdirectory of the source has that same subdirectory path
	// mirrored under the destination directory, instead of every match
	// being flattened directly into it.
	Dirs bool

	// Stream, for a host destination only, copies a Stream-format source
	// file's exact on-disk bytes rather than re-splitting it into
	// records and reinserting a (possibly different) line ending --
	// "don't look for a line end character and don't insert anything in
	// output", matching ods2's own doc comment. Has no effect on a
	// non-Stream source format, or once /BINARY has already selected a
	// full raw copy.
	Stream bool

	// CRLF selects "\r\n" as the line ending records.go's writeRecords
	// inserts while rendering a container source as text onto a host
	// destination, in place of the default "\n" (LF). Only meaningful
	// for a host destination, and only when neither /BINARY nor /STREAM
	// (for a Stream-format source) is already copying raw bytes
	// verbatim. Mutually exclusive with /LF -- enforced by
	// internal/bootdata/files/evax.dcl's own "disallow crlf and lf"
	// grammar statement, not by this Go code, so CRLF and an eventual
	// "LF" field would never both need representing here; LF has no
	// field of its own since it's simply CRLF's absence (the default).
	CRLF bool
}

// CopyResultKind distinguishes the four different confirmation/preview
// message shapes Console.Copy needs to print for one CopyResult -- see
// its own constants' doc comments for each one's exact wording, and
// Console.Copy (internal/console/copy.go) for where they're actually
// rendered. Kept as a small enum rather than, say, a pre-rendered message
// string on CopyResult itself, so the "%COPY-..." status-line text stays
// owned by the console layer (matching how Console.Delete/Console.Purge
// build their own confirmation lines from a plain data list, rather than
// internal/rms handing back already-formatted VMS status text).
type CopyResultKind int

const (
	// CopyCopied reports an ordinary file actually copied --
	// Console.Copy prints "%COPY-I-COPYING, copying SOURCE to DEST"
	// first if Verbose, then "%COPY-S-COPIED, SOURCE copied to DEST"
	// unless Quiet.
	CopyCopied CopyResultKind = iota

	// CopyTested reports what an ordinary file copy would have done,
	// under /TEST -- Console.Copy always prints "%COPY-I-TEST, would
	// copy SOURCE to DEST", regardless of Quiet/Verbose (matching ods2's
	// own cmdCopy, which checks /TEST before either of those).
	CopyTested

	// CopyDirCreated reports a matched directory entry materialized as
	// an empty host directory (/DIRS on a host destination) --
	// Console.Copy prints "%COPY-I-COPYING, creating directory DEST for
	// SOURCE" first if Verbose, then "%COPY-S-COPIED, SOURCE
	// materialized as directory DEST" unless Quiet.
	CopyDirCreated

	// CopyDirTested mirrors CopyDirCreated under /TEST -- Console.Copy
	// always prints "%COPY-I-TEST, would create directory for SOURCE at
	// DEST".
	CopyDirTested
)

// CopyResult reports one file (or directory entry) Copy actually acted
// on, as ready-to-print display text, for the console layer's own
// confirmation messages (Console.Copy, internal/console/copy.go) -- the
// same "rms computes, console prints" split Session.Directory/Session.
// Type already established, just as a list (one entry per matched file)
// instead of a single value, matching Session.Delete's own []DeletedFile
// convention for the same reason: a wildcarded COPY can touch several
// files in one call.
//
// Source and Dest are each either a plain host path (used verbatim) or a
// VMS file specification of the shape "NAME.TYPE;version" (for a matched
// source file, which carries no device/directory of its own worth
// repeating) or "DEVICE:[DIRS]NAME.TYPE;version" (for a freshly created
// destination file, whose full location is worth showing, and with no
// ";version" suffix at all under /TEST, since no version has actually
// been assigned yet) -- whichever shape ods2's own cmdCopy confirmation
// messages use for the same case.
type CopyResult struct {
	Kind   CopyResultKind
	Source string
	Dest   string

	// Warning, non-empty only for a Kind == CopyCopied result with /TIME
	// set whose os.Chtimes call itself failed, holds that failure's own
	// text (no "%COPY-W-..." prefix -- Console.Copy adds that). A /TIME
	// failure never fails the whole copy, matching ods2's own cmdCopy
	// ("if err != nil && verbose" -- a warning, not an abort); Console.
	// Copy only prints it when Verbose is set, matching that same
	// condition.
	Warning string
}

// MultipleMatchesError reports that a COPY source specification's name/
// type pattern matched more than one file when the destination doesn't
// accept more than one -- either a volume destination naming a specific
// file of its own (rather than just a directory), or a host destination
// that isn't an existing directory. Given its own type (rather than a
// plain fmt.Errorf string, for the same reason type.go's AmbiguousError
// has one: so the console layer can recognize it via errors.As and report
// a specific status rather than falling back to a generic one), kept
// separate from AmbiguousError because that type's own Error() text is
// hardcoded to say "TYPE does not support wildcards" -- wrong wording for
// a COPY-triggered failure, and (since subtask 10) not even accurate for
// COPY itself anymore: COPY does support copying several matched files at
// once, just only onto a directory destination.
type MultipleMatchesError struct {
	Spec  string
	Count int
}

func (e *MultipleMatchesError) Error() string {
	return fmt.Sprintf("rms: %s matches %d files; destination must be a directory to copy more than one file", e.Spec, e.Count)
}

// HostToHostError reports that both SOURCE and DESTINATION carried /HOST
// on the same COPY command line -- a combination with no container
// endpoint at all, and therefore not this command's job: an ordinary
// host-to-host file copy is what the operating system's own file-copy
// tools are for, matching ods2's own cmdCopyFromHost doc comment's
// reasoning for the same restriction. Given its own type so the console
// layer can report it as CLI_BADQUALIFIERCOMBO (the existing "two
// qualifiers can't both be set" status internal/console/dcl's own
// DISALLOW checking already uses) rather than falling back to a generic
// bad-file-specification error that would misdescribe what's actually
// wrong.
type HostToHostError struct{}

func (e *HostToHostError) Error() string {
	return "copy: /HOST on both SOURCE and DESTINATION has no container endpoint; COPY always needs at least one file on a mounted volume"
}

// Copy copies one or more files from source to destination, in whichever
// of the four directions sourceHost/destHost select:
//
//   - Neither set: source and destination are both VMS file
//     specifications on mounted volumes (copyVolumeToVolume). source may
//     be wildcarded if destination names no file of its own (just a
//     device/directory).
//   - sourceHost only: source is a plain host path (never wildcarded),
//     destination is VMS syntax naming a location on a mounted volume
//     (copyFromHost).
//   - destHost only: source is VMS syntax on a mounted volume, possibly
//     wildcarded if destination is an existing host directory,
//     destination is a plain host path (copyToHost).
//   - Both set: rejected with a *HostToHostError -- see its own doc
//     comment for why.
//
// opts carries every qualifier from docs/PHASE-23.md subtask 10 -- see
// CopyOptions' own doc comment for exactly which ones apply to which
// direction.
func (s *Session) Copy(sourceText string, sourceHost bool, destText string, destHost bool, opts CopyOptions) ([]CopyResult, error) {
	if sourceHost && destHost {
		return nil, fmt.Errorf("copy: %w", &HostToHostError{})
	}

	switch {
	case sourceHost:
		return s.copyFromHost(sourceText, destText, opts)
	case destHost:
		return s.copyToHost(sourceText, destText, opts)
	default:
		return s.copyVolumeToVolume(sourceText, destText, opts)
	}
}

// requireMatchable enforces COPY's own "several matches only allowed onto
// a directory destination" rule, shared by copyVolumeToVolume and
// copyToHost (copyFromHost's source is never wildcarded in the first
// place, so it never needs this check).
func requireMatchable(matches []filespec.Match, destAcceptsMultiple bool, specText string) error {
	if len(matches) == 0 {
		return fmt.Errorf("copy: %w", &NotFoundError{Spec: specText})
	}

	if len(matches) > 1 && !destAcceptsMultiple {
		return fmt.Errorf("copy: %w", &MultipleMatchesError{Spec: specText, Count: len(matches)})
	}

	return nil
}

// copyVolumeToVolume implements the plain "COPY FOO.TXT BAR.TXT" case:
// both source and destination are VMS file specifications resolved
// against the session's current Default, exactly like any other command
// in this phase (see resolveVolume) -- possibly the same mounted volume,
// possibly two different ones. destText may name no file of its own (just
// a device/directory), in which case source may match several files, each
// copied in under its own name/type; a directory entry among the matches
// is always skipped (there's no "create an empty directory" operation a
// plain file copy could perform on a volume destination), regardless of
// /DIRS -- matching ods2's own cmdCopy exactly.
func (s *Session) copyVolumeToVolume(sourceText, destText string, opts CopyOptions) ([]CopyResult, error) {
	srcVol, srcSpec, err := s.resolveVolume(sourceText)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	matches, err := filespec.Glob(srcVol, srcSpec)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	destVol, destSpec, err := s.resolveVolume(destText)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	destIsDirOnly := destSpec.Name == "" && destSpec.Type == ""

	if err := requireMatchable(matches, destIsDirOnly, sourceText); err != nil {
		return nil, err
	}

	var (
		destDir *volume.Directory
		destBm  *volume.Bitmap
		destIb  *volume.IndexBitmap
	)

	if !opts.Test {
		destDir, destBm, destIb, err = resolveVolumeDest(destVol, destSpec)
		if err != nil {
			return nil, fmt.Errorf("copy: %w", err)
		}
	}

	var results []CopyResult

	for _, m := range matches {
		if strings.EqualFold(m.Type, "DIR") {
			continue
		}

		name, typ := destNameType(destSpec, m.Name, m.Type)
		sourceDisplay := matchDisplay(m)

		if opts.Test {
			dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ}
			results = append(results, CopyResult{Kind: CopyTested, Source: sourceDisplay, Dest: dest.String()})

			continue
		}

		src, err := srcVol.OpenFID(m.Fid)
		if err != nil {
			return results, fmt.Errorf("copy: %w", err)
		}

		var version uint16
		if opts.Binary {
			version, err = createBinaryFile(destVol, destDir, destBm, destIb, name, typ, src)
		} else {
			version, err = createTextFile(destVol, destDir, destBm, destIb, name, typ, func(w *odsrms.Writer) error {
				return copyRecordsAsLines(w, src)
			})
		}

		if err != nil {
			return results, fmt.Errorf("copy: %w", err)
		}

		dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ, Version: fmt.Sprint(version)}
		results = append(results, CopyResult{Kind: CopyCopied, Source: sourceDisplay, Dest: dest.String()})
	}

	return results, nil
}

// copyFromHost implements COPY's /HOST-on-SOURCE case ("COPY foo.txt/HOST
// BAR.TXT"): hostPath names a single plain host file (never wildcarded --
// see this file's own top-of-file "Multi-file" doc comment for why), copied
// onto destText, a VMS file specification naming a location on a mounted
// (and writable) volume.
func (s *Session) copyFromHost(hostPath, destText string, opts CopyOptions) ([]CopyResult, error) {
	info, err := os.Stat(hostPath)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("copy: %s: is a directory, not a file", hostPath)
	}

	destVol, destSpec, err := s.resolveVolume(destText)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	hostName, hostType := hostBaseNameType(hostPath)
	name, typ := destNameType(destSpec, hostName, hostType)

	if opts.Test {
		dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ}

		return []CopyResult{{Kind: CopyTested, Source: hostPath, Dest: dest.String()}}, nil
	}

	destDir, destBm, destIb, err := resolveVolumeDest(destVol, destSpec)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	f, err := os.Open(hostPath)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}
	defer f.Close()

	var version uint16
	if opts.Binary {
		version, err = createBinaryFileFromHost(destVol, destDir, destBm, destIb, name, typ, f, info.Size())
	} else {
		version, err = createTextFile(destVol, destDir, destBm, destIb, name, typ, func(w *odsrms.Writer) error {
			return copyHostLinesAsRecords(w, f)
		})
	}

	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ, Version: fmt.Sprint(version)}

	return []CopyResult{{Kind: CopyCopied, Source: hostPath, Dest: dest.String()}}, nil
}

// copyToHost implements COPY's /HOST-on-DESTINATION case ("COPY FOO.TXT
// bar.txt/HOST"): sourceText is a VMS file specification naming one or
// more files on a mounted volume, copied onto destPath, a plain host
// path. destPath may only receive several matched files if it already
// names an existing host directory; every qualifier docs/PHASE-23.md
// subtask 10 added other than /BINARY/QUIET/VERBOSE/TEST only has an
// effect in this direction -- see CopyOptions' own doc comment.
func (s *Session) copyToHost(sourceText, destPath string, opts CopyOptions) ([]CopyResult, error) {
	vol, spec, err := s.resolveVolume(sourceText)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	destIsDir := destIsDirectory(destPath)

	if err := requireMatchable(matches, destIsDir, sourceText); err != nil {
		return nil, err
	}

	lineEnding := lfLineEnding
	if opts.CRLF {
		lineEnding = crlfLineEnding
	}

	var results []CopyResult

	for _, m := range matches {
		sourceDisplay := matchDisplay(m)

		if strings.EqualFold(m.Type, "DIR") {
			result, err := s.copyDirEntryToHost(destPath, m, sourceDisplay, opts)
			if err != nil {
				return results, err
			}

			if result != nil {
				results = append(results, *result)
			}

			continue
		}

		outPath := resolveHostDestPath(destPath, m, opts.Dirs, destIsDir)

		if opts.Test {
			results = append(results, CopyResult{Kind: CopyTested, Source: sourceDisplay, Dest: outPath})

			continue
		}

		src, err := vol.OpenFID(m.Fid)
		if err != nil {
			return results, fmt.Errorf("copy: %w", err)
		}

		result, err := copyOneFileToHost(src, outPath, sourceDisplay, opts, lineEnding)
		if err != nil {
			return results, fmt.Errorf("copy: %w", err)
		}

		results = append(results, result)
	}

	return results, nil
}

// copyDirEntryToHost handles one matched directory entry (e.g. a
// subdirectory named by a recursive source spec) for copyToHost: without
// /DIRS it's silently skipped (nil, nil -- matching ods2's own default),
// with /DIRS it's materialized as an empty host directory (or, under
// /TEST, only described as if it would be).
func (s *Session) copyDirEntryToHost(destPath string, m filespec.Match, sourceDisplay string, opts CopyOptions) (*CopyResult, error) {
	if !opts.Dirs {
		return nil, nil
	}

	dirPath := hostDirPath(destPath, m)

	if opts.Test {
		return &CopyResult{Kind: CopyDirTested, Source: sourceDisplay, Dest: dirPath}, nil
	}

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return nil, fmt.Errorf("copy: creating directory for %s: %w", sourceDisplay, err)
	}

	return &CopyResult{Kind: CopyDirCreated, Source: sourceDisplay, Dest: dirPath}, nil
}

// copyOneFileToHost copies src's content onto outPath (creating any
// missing parent directories along the way, relevant when /DIRS produced
// a nested outPath) according to opts, and applies /TIME afterward as a
// non-fatal warning on failure -- see CopyOptions.Time's own doc comment
// for why a /TIME failure doesn't fail the whole copy.
func copyOneFileToHost(src *volume.File, outPath, sourceDisplay string, opts CopyOptions, lineEnding []byte) (CopyResult, error) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return CopyResult{}, err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return CopyResult{}, err
	}

	copyErr := copyToHostFile(out, src, opts, lineEnding)

	var warning string
	
	if copyErr == nil && opts.Time {
		if timeErr := preserveFileTime(outPath, src); timeErr != nil {
			warning = fmt.Sprintf("could not preserve date on %s: %v", outPath, timeErr)
		}
	}

	if closeErr := out.Close(); copyErr == nil {
		copyErr = closeErr
	}

	if copyErr != nil {
		return CopyResult{}, copyErr
	}

	return CopyResult{Kind: CopyCopied, Source: sourceDisplay, Dest: outPath, Warning: warning}, nil
}

// copyToHostFile writes src's content to out, choosing among /BINARY (an
// exact raw-byte copy), /STREAM on a Stream-format source (also an exact
// raw-byte copy -- see CopyOptions.Stream's own doc comment for why), and
// the default text reframing (records.go's writeRecords). /IGNORE
// recovers from a corrupt record hit while text-reframing by restarting
// out from scratch as a raw-byte copy instead, matching ods2's own
// cmdCopy/copyOneFile.
func copyToHostFile(out *os.File, src *volume.File, opts CopyOptions, lineEnding []byte) error {
	if opts.Binary {
		return copyRawToHost(out, src)
	}

	if opts.Stream && isStreamFormat(src.Header.RecordAttributes.Format) {
		return copyRawToHost(out, src)
	}

	err := writeRecords(out, src, lineEnding)
	if err != nil && opts.Ignore && errors.Is(err, odsrms.ErrCorruptRecord) {
		if _, seekErr := out.Seek(0, io.SeekStart); seekErr != nil {
			return seekErr
		}

		if truncErr := out.Truncate(0); truncErr != nil {
			return truncErr
		}

		return copyRawToHost(out, src)
	}

	return err
}

// isStreamFormat reports whether format is one of the three Stream record
// formats (as opposed to Fixed, Variable, VFC, or Undefined) -- /STREAM's
// own "does this source file even have exact bytes worth preserving
// as-is" check. Functionally matches ods2's own same-named function.
func isStreamFormat(format ondisk.RecordFormat) bool {
	switch format {
	case ondisk.RecordFormatStreamCRLF, ondisk.RecordFormatStreamLF, ondisk.RecordFormatStreamCR:
		return true
	default:
		return false
	}
}

// copyRawToHost copies f's exact valid on-disk bytes (see odsrms.
// FileByteLength) to out with no record interpretation at all, block by
// block -- the shared implementation behind both /BINARY and /STREAM (on
// a Stream-format source) for a host destination. Functionally matches
// ods2's own copyBinary.
func copyRawToHost(out *os.File, f *volume.File) error {
	remaining := odsrms.FileByteLength(f.Header.RecordAttributes)

	buf := make([]byte, ondisk.BlockSize)
	for vbn := uint32(1); remaining > 0; vbn++ {
		if err := f.ReadBlock(vbn, buf); err != nil {
			return err
		}

		n := int64(len(buf))
		if n > remaining {
			n = remaining
		}

		if _, err := out.Write(buf[:n]); err != nil {
			return err
		}

		remaining -= n
	}

	return nil
}

// preserveFileTime sets outPath's modification (and access) time to f's
// own VMS revision date, for /TIME. Functionally matches ods2's own
// same-named function.
func preserveFileTime(outPath string, f *volume.File) error {
	ident, err := f.Header.Ident()
	if err != nil {
		return err
	}

	t := ident.RevisionDate.Time()

	return os.Chtimes(outPath, t, t)
}

// destIsDirectory reports whether dest already names an existing host
// directory. Functionally matches ods2's own same-named function.
func destIsDirectory(dest string) bool {
	info, err := os.Stat(dest)

	return err == nil && info.IsDir()
}

// resolveHostDestPath builds the host output path for one matched
// (non-directory-entry) file m:
//
//   - If destIsDir, the file is written under dest as its own
//     "NAME.TYPE;version" -- under a mirrored copy of its source
//     subdirectory path (m.Dirs) too, if preserveDirs (/DIRS) is set and
//     the match came from a subdirectory.
//   - Otherwise dest is used exactly as given, as a literal output path
//     (only reached when exactly one file is being copied --
//     requireMatchable guarantees that before this is ever called for a
//     non-directory destination).
//
// Unlike ods2's own resolveDestination, there is no "dest's base name
// contains '*'" wildcard-substitution case -- this project doesn't
// implement that (see this file's own top-of-file "Multi-file" doc
// comment).
func resolveHostDestPath(dest string, m filespec.Match, preserveDirs, destIsDir bool) string {
	if !destIsDir {
		return dest
	}

	filename := matchDisplay(m)

	if preserveDirs && len(m.Dirs) > 0 {
		parts := make([]string, 0, len(m.Dirs)+2)
		parts = append(parts, dest)
		parts = append(parts, m.Dirs...)
		parts = append(parts, filename)

		return filepath.Join(parts...)
	}

	return filepath.Join(dest, filename)
}

// hostDirPath builds the host directory path representing matched
// directory entry m under dest: dest, followed by m's own directory path
// (m.Dirs), followed by the directory's own name (m.Name) -- i.e. the
// full path TO and INCLUDING this directory itself. Functionally matches
// ods2's own same-named function.
func hostDirPath(dest string, m filespec.Match) string {
	parts := make([]string, 0, len(m.Dirs)+2)
	parts = append(parts, dest)
	parts = append(parts, m.Dirs...)
	parts = append(parts, m.Name)

	return filepath.Join(parts...)
}

// matchDisplay renders a matched source file as plain "NAME.TYPE;version"
// text (no device/directory -- the same convention Console.Delete's own
// confirmation line already uses), for CopyResult.Source and, in
// resolveHostDestPath, as the literal filename written under a host
// destination directory.
func matchDisplay(m filespec.Match) string {
	return fmt.Sprintf("%s.%s;%d", m.Name, m.Type, m.Version)
}

// destNameType resolves the actual name/type to create for a volume
// destination: destSpec's own Name/Type, if its text supplied one, or
// fallbackName/fallbackType (a matched source file's own name/type, or a
// /HOST source's host-derived name/type) if destText named no file of
// its own at all -- e.g. "COPY FOO.TXT DUA0:[SUBDIR]" naming only a
// directory. Unlike ods2's own volumeDestName, there is no
// wildcard-substitution case ('*'/'%' in destSpec's own Name/Type) to
// handle -- this project doesn't implement that (see this file's own
// top-of-file "Multi-file" doc comment).
func destNameType(destSpec filespec.Spec, fallbackName, fallbackType string) (name, typ string) {
	name, typ = destSpec.Name, destSpec.Type
	if name == "" {
		name = fallbackName
	}

	if typ == "" {
		typ = fallbackType
	}

	return name, typ
}

// resolveVolumeDest resolves destSpec's target Directory on destVol and
// its device's bitmap caches -- the one-time setup a volume destination
// needs before any file is written into it, shared by every direction
// above that creates a file on a volume. Functionally matches ods2's own
// same-named helper (copy.go), reproduced here since that package's own
// internal/ visibility rules out importing it directly.
func resolveVolumeDest(destVol *volume.Volume, destSpec filespec.Spec) (*volume.Directory, *volume.Bitmap, *volume.IndexBitmap, error) {
	destDir, err := filespec.ResolveDirectory(destVol, destSpec.Dirs)
	if err != nil {
		return nil, nil, nil, err
	}

	destBm, err := destDir.Device.Bitmap()
	if err != nil {
		return nil, nil, nil, err
	}

	destIb, err := destDir.Device.IndexBitmap()
	if err != nil {
		return nil, nil, nil, err
	}

	return destDir, destBm, destIb, nil
}

// createTextFile creates a brand-new Stream_LF file name.typ in destDir
// (using destBm/destIb, destDir's device's bitmap caches), auto-assigning
// the next version number the same way CreateFile/Directory.Insert always
// does, calls fill to write its content through the freshly opened
// odsrms.Writer, and returns the version actually used (by looking the
// new directory entry back up, the same way ods2's own
// copyOneFileToVolume/copyHostFileToVolume do, rather than threading it
// back out of CreateFile itself).
//
// Stream_LF is the default (non-/BINARY) destination format on a volume,
// matching ods2's own cmdCopy default; createBinaryFile/
// createBinaryFileFromHost below are its /BINARY counterparts.
func createTextFile(destVol *volume.Volume, destDir *volume.Directory, destBm *volume.Bitmap, destIb *volume.IndexBitmap, name, typ string, fill func(*odsrms.Writer) error) (uint16, error) {
	fullName := name + "." + typ

	dst, err := destVol.CreateFile(destDir, fullName, ondisk.RecAttr{Format: ondisk.RecordFormatStreamLF}, destBm, destIb)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", fullName, err)
	}

	w, err := odsrms.NewWriter(dst)
	if err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	if err := fill(w); err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	if err := w.Close(); err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	return lookUpVersion(destDir, fullName)
}

// createBinaryFile creates a brand-new Undefined-format file name.typ in
// destDir and copies src's exact on-disk bytes into it verbatim (/BINARY,
// container source) -- the volume-destination counterpart of
// createTextFile, sharing its same "look the assigned version back up
// via Directory.Lookup" convention. Functionally matches ods2's own
// copyOneFileToVolume(binary: true).
func createBinaryFile(destVol *volume.Volume, destDir *volume.Directory, destBm *volume.Bitmap, destIb *volume.IndexBitmap, name, typ string, src *volume.File) (uint16, error) {
	fullName := name + "." + typ

	dst, err := destVol.CreateFile(destDir, fullName, ondisk.RecAttr{Format: ondisk.RecordFormatUndefined, MaxRecordSize: ondisk.BlockSize}, destBm, destIb)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", fullName, err)
	}

	if err := copyRawToVolume(dst, src); err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	return lookUpVersion(destDir, fullName)
}

// createBinaryFileFromHost mirrors createBinaryFile for a /HOST source
// (copyFromHost's own /BINARY case), reading from a plain host *os.File
// with a known size instead of an already-open volume.File whose valid
// length comes from odsrms.FileByteLength. Functionally matches ods2's
// own copyHostFileToVolume(binary: true).
func createBinaryFileFromHost(destVol *volume.Volume, destDir *volume.Directory, destBm *volume.Bitmap, destIb *volume.IndexBitmap, name, typ string, src *os.File, size int64) (uint16, error) {
	fullName := name + "." + typ

	dst, err := destVol.CreateFile(destDir, fullName, ondisk.RecAttr{Format: ondisk.RecordFormatUndefined, MaxRecordSize: ondisk.BlockSize}, destBm, destIb)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", fullName, err)
	}

	if err := copyHostRawToVolume(dst, src, size); err != nil {
		return 0, fmt.Errorf("writing %s: %w", fullName, err)
	}

	return lookUpVersion(destDir, fullName)
}

// lookUpVersion recovers the version CreateFile/Directory.Insert just
// auto-assigned to fullName (the highest that existed before this call,
// plus one) by looking the entry back up, rather than threading it back
// out of CreateFile itself -- keeps that bookkeeping entirely inside
// Directory, where it already lives, matching ods2's own identical choice
// in copyOneFileToVolume/copyHostFileToVolume. Shared by createTextFile
// and both createBinaryFile* helpers above.
func lookUpVersion(destDir *volume.Directory, fullName string) (uint16, error) {
	entry, err := destDir.Lookup(fullName, 0)
	if err != nil {
		return 0, fmt.Errorf("looking up newly created %s: %w", fullName, err)
	}

	return entry.Version, nil
}

// copyRawToVolume copies src's exact valid bytes (see odsrms.
// FileByteLength) to dst block by block, then records dst's true byte
// length -- which may end partway through its last block -- via
// CloseWithFinalByte rather than Close's own whole-block-only rounding.
// Functionally matches ods2's own same-named function.
func copyRawToVolume(dst, src *volume.File) error {
	total := odsrms.FileByteLength(src.Header.RecordAttributes)
	remaining := total

	buf := make([]byte, ondisk.BlockSize)
	for vbn := uint32(1); remaining > 0; vbn++ {
		if err := src.ReadBlock(vbn, buf); err != nil {
			return err
		}

		if err := dst.WriteBlock(vbn, buf); err != nil {
			return err
		}

		remaining -= int64(len(buf))
	}

	return dst.CloseWithFinalByte(uint16(total % ondisk.BlockSize))
}

// copyHostRawToVolume copies src's exact bytes (size bytes long) to dst
// block by block, zero-padding the final partial block the same way a
// whole-block write always must, then records dst's true byte length via
// CloseWithFinalByte. Functionally matches ods2's own same-named
// function.
func copyHostRawToVolume(dst *volume.File, src io.Reader, size int64) error {
	buf := make([]byte, ondisk.BlockSize)
	remaining := size

	for vbn := uint32(1); remaining > 0; vbn++ {
		n, err := io.ReadFull(src, buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}

		for i := n; i < len(buf); i++ {
			buf[i] = 0
		}

		if err := dst.WriteBlock(vbn, buf); err != nil {
			return err
		}

		remaining -= int64(n)
	}

	return dst.CloseWithFinalByte(uint16(size % ondisk.BlockSize))
}

// copyRecordsAsLines reframes src's records (whatever its own record
// format is -- Fixed, Variable, VFC, Stream) as a sequence of individual
// Stream_LF records on w, reusing writeRecords' own record-format-aware
// text rendering (VFC carriage-control expansion and all) via
// lineSplitWriter, which re-splits that rendered text back into
// individual '\n'-delimited records for w.Put to apply Stream_LF's own
// framing to. Functionally matches ods2's own copyRecordsToVolume.
//
// This is the volume-destination default (non-/BINARY) text path; unlike
// copyToHostFile's own text path, it always uses lfLineEnding regardless
// of a /CRLF, since /CRLF only chooses a host text file's own line
// ending -- a volume destination's records are always Stream_LF framing
// on disk either way, and lfLineEnding here is purely lineSplitWriter's
// own "where does one record end" delimiter, matching ods2's own
// identical choice in copyRecordsToVolume.
func copyRecordsAsLines(w *odsrms.Writer, src *volume.File) error {
	lw := &lineSplitWriter{put: w.Put}
	if err := writeRecords(lw, src, lfLineEnding); err != nil {
		return err
	}

	if len(lw.buf) > 0 {
		// A trailing "line" with no terminating '\n' at all -- possible
		// when src's own last record has no natural terminator (a VFC
		// record using a carriage-control that suppresses the trailing
		// line feed, most notably). Flushed as a final record rather
		// than silently dropped.
		return w.Put(lw.buf)
	}

	return nil
}

// lineSplitWriter is an io.Writer adapter that calls put once per
// '\n'-terminated line written to it (excluding the '\n' itself),
// buffering only whatever partial line hasn't seen its terminator yet --
// the bridge between writeRecords (which writes a stream of already
// line-terminated text) and odsrms.Writer (whose Put wants each record
// handed to it individually, so it can apply its own destination
// format's framing). Functionally matches ods2's own same-named type.
type lineSplitWriter struct {
	buf []byte
	put func([]byte) error
}

func (l *lineSplitWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != '\n' {
			l.buf = append(l.buf, b)

			continue
		}

		if err := l.put(l.buf); err != nil {
			return 0, err
		}

		l.buf = l.buf[:0]
	}

	return len(p), nil
}

// copyHostLinesAsRecords reframes src's plain host text as a sequence of
// Stream_LF records on w: split on '\n', trimming a trailing '\r' so
// CRLF-terminated host text works the same as LF-terminated text, one
// record per line. Functionally matches ods2's own
// copyHostRecordsToVolume.
func copyHostLinesAsRecords(w *odsrms.Writer, src io.Reader) error {
	reader := bufio.NewReader(src)

	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			if err := w.Put([]byte(line)); err != nil {
				return err
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}

			return readErr
		}
	}
}

// hostBaseNameType splits a host path's base name into a VMS-style
// name/type pair on the LAST '.' (so a name with several dots, e.g.
// "archive.tar.gz", keeps every earlier one as part of the name; a base
// name with no '.' at all gets an empty type), upper-cased since a host
// base name has no VMS convention behind its case at all to begin with.
// Functionally matches ods2's own same-named function; see its own doc
// comment (copy.go, github.com/tucats/ods2) for the fuller reasoning on
// why upper-casing only this one host-sourced name is correct here.
func hostBaseNameType(path string) (name, typ string) {
	base := filepath.Base(path)
	if idx := strings.LastIndexByte(base, '.'); idx != -1 {
		return strings.ToUpper(base[:idx]), strings.ToUpper(base[idx+1:])
	}

	return strings.ToUpper(base), ""
}
