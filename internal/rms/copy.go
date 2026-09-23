package rms

import (
	"bufio"
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

// This file implements docs/PHASE-23.md subtask 9: COPY's core direction
// logic and its /HOST qualifier -- the operator-console command that moves
// one file's content between a mounted ODS-2 volume and the host
// filesystem, or between two locations on mounted volumes. Its behavioral
// reference is the sibling ods2 module's own cmd/ods2/internal/session/
// copy.go (read-only, per this phase's own "behavioral spec, not code to
// link against" framing -- see docs/PHASE-23.md's "Why this phase looks
// different from most others"); the functions below follow its shape
// closely (the same CreateFile-then-reframe-as-text approach, the same
// Stream_LF default) without being a literal port, and without (yet) its
// full qualifier set -- /BINARY, /QUIET, /VERBOSE, /TEST, /TIME, /IGNORE,
// /DIRS, /STREAM, /VFC, /CRLF, /LF all land in subtask 10.
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
// Only a single, unambiguous file is supported as the source in this
// subtask -- a source spec that matches more than one file on its volume
// fails with a *MultipleMatchesError rather than copying several at once.
// Real multi-file ("wildcard") copying is deliberately deferred: ods2's
// own cmdCopy ties it directly to /DIRS (how a matched subdirectory entry
// is handled) and to per-file destination-name wildcard substitution,
// neither of which subtask 9's scope ("core...direction combinations...
// with no other qualifier") calls for -- see docs/PHASE-23.md's own open
// questions for this subtask.

// CopyResult reports the two endpoints Copy actually used, as ready-to-
// print display text, for the console layer's own confirmation message
// (Console.Copy, internal/console/copy.go) -- the same "rms computes,
// console prints" split Session.Directory/Session.Type already
// established, just with two strings instead of one.
//
// Source and Dest are each either a plain host path (used verbatim) or a
// VMS file specification of the shape "NAME.TYPE;version" (for a matched
// source file, which carries no device/directory of its own worth
// repeating) or "DEVICE:[DIRS]NAME.TYPE;version" (for a freshly created
// destination file, whose full location is worth showing) -- whichever
// shape ods2's own cmdCopy confirmation messages use for the same case.
type CopyResult struct {
	Source string
	Dest   string
}

// MultipleMatchesError reports that a COPY source specification's name/
// type pattern matched more than one file, when exactly one is required.
// It is Copy's own version of the same idea as type.go's AmbiguousError
// (given its own type, rather than a plain fmt.Errorf string, for the
// same reason: so the console layer can recognize it via errors.As and
// report a specific status rather than falling back to a generic one),
// kept as a separate type instead of reusing AmbiguousError because that
// type's own Error() text is hardcoded to say "TYPE does not support
// wildcards" -- wrong wording for a COPY-triggered failure.
type MultipleMatchesError struct {
	Spec  string
	Count int
}

func (e *MultipleMatchesError) Error() string {
	return fmt.Sprintf("rms: %s is ambiguous (%d files match); COPY does not yet support copying more than one file at a time", e.Spec, e.Count)
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

// Copy copies one file from source to destination, in whichever of the
// four directions sourceHost/destHost select:
//
//   - Neither set: source and destination are both VMS file
//     specifications on mounted volumes (copyVolumeToVolume).
//   - sourceHost only: source is a plain host path, destination is VMS
//     syntax naming a location on a mounted volume (copyFromHost).
//   - destHost only: source is VMS syntax on a mounted volume,
//     destination is a plain host path (copyToHost).
//   - Both set: rejected with a *HostToHostError -- see its own doc
//     comment for why.
//
// Absent a future /BINARY qualifier (subtask 10), every direction that
// touches a mounted volume renders/reframes the file's content as plain
// '\n'-terminated text rather than copying raw bytes -- matching ods2's
// own cmdCopy default, and reusing records.go's writeRecords (the same
// record-format-aware renderer Session.Type already established) for the
// container-source cases.
func (s *Session) Copy(sourceText string, sourceHost bool, destText string, destHost bool) (CopyResult, error) {
	if sourceHost && destHost {
		return CopyResult{}, fmt.Errorf("copy: %w", &HostToHostError{})
	}

	switch {
	case sourceHost:
		return s.copyFromHost(sourceText, destText)
	case destHost:
		return s.copyToHost(sourceText, destText)
	default:
		return s.copyVolumeToVolume(sourceText, destText)
	}
}

// copyVolumeToVolume implements the plain "COPY FOO.TXT BAR.TXT" case:
// both source and destination are VMS file specifications resolved
// against the session's current Default, exactly like any other command
// in this phase (see resolveVolume) -- possibly the same mounted volume,
// possibly two different ones.
func (s *Session) copyVolumeToVolume(sourceText, destText string) (CopyResult, error) {
	srcVol, srcSpec, err := s.resolveVolume(sourceText)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	m, err := matchOne(srcVol, srcSpec, sourceText)
	if err != nil {
		return CopyResult{}, err
	}

	src, err := srcVol.OpenFID(m.Fid)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	destVol, destSpec, err := s.resolveVolume(destText)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	name, typ := destNameType(destSpec, m.Name, m.Type)

	destDir, destBm, destIb, err := resolveVolumeDest(destVol, destSpec)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	version, err := createTextFile(destVol, destDir, destBm, destIb, name, typ, func(w *odsrms.Writer) error {
		return copyRecordsAsLines(w, src)
	})
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ, Version: fmt.Sprint(version)}

	return CopyResult{Source: matchDisplay(m), Dest: dest.String()}, nil
}

// copyFromHost implements COPY's /HOST-on-SOURCE case ("COPY foo.txt/HOST
// BAR.TXT"): hostPath names a plain host file (not a file on any mounted
// volume), copied onto destText, a VMS file specification naming a
// location on a mounted (and writable) volume.
func (s *Session) copyFromHost(hostPath, destText string) (CopyResult, error) {
	info, err := os.Stat(hostPath)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	if info.IsDir() {
		return CopyResult{}, fmt.Errorf("copy: %s: is a directory, not a file", hostPath)
	}

	destVol, destSpec, err := s.resolveVolume(destText)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	hostName, hostType := hostBaseNameType(hostPath)
	name, typ := destNameType(destSpec, hostName, hostType)

	destDir, destBm, destIb, err := resolveVolumeDest(destVol, destSpec)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	f, err := os.Open(hostPath)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}
	defer f.Close()

	version, err := createTextFile(destVol, destDir, destBm, destIb, name, typ, func(w *odsrms.Writer) error {
		return copyHostLinesAsRecords(w, f)
	})
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	dest := filespec.Spec{Device: destSpec.Device, Dirs: destSpec.Dirs, Name: name, Type: typ, Version: fmt.Sprint(version)}

	return CopyResult{Source: hostPath, Dest: dest.String()}, nil
}

// copyToHost implements COPY's /HOST-on-DESTINATION case ("COPY FOO.TXT
// bar.txt/HOST"): sourceText is a VMS file specification naming exactly
// one file on a mounted volume, copied onto destPath, a plain host path.
// If destPath already names an existing host directory, the file is
// written there under its own "NAME.TYPE;version" (matching ods2's own
// resolveDestination's directory case); otherwise destPath is used
// exactly as given, as a literal output path.
func (s *Session) copyToHost(sourceText, destPath string) (CopyResult, error) {
	vol, spec, err := s.resolveVolume(sourceText)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	m, err := matchOne(vol, spec, sourceText)
	if err != nil {
		return CopyResult{}, err
	}

	src, err := vol.OpenFID(m.Fid)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	outPath := destPath
	if info, statErr := os.Stat(destPath); statErr == nil && info.IsDir() {
		outPath = filepath.Join(destPath, matchDisplay(m))
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}
	defer out.Close()

	if err := writeRecords(out, src, lfLineEnding); err != nil {
		return CopyResult{}, fmt.Errorf("copy: %w", err)
	}

	return CopyResult{Source: matchDisplay(m), Dest: outPath}, nil
}

// matchOne resolves specText's already-parsed spec against vol and
// requires the result to be exactly one file -- COPY's own "no wildcard
// copying yet" rule (see this file's own top-of-file doc comment).
func matchOne(vol *volume.Volume, spec filespec.Spec, specText string) (filespec.Match, error) {
	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return filespec.Match{}, fmt.Errorf("copy: %w", err)
	}

	switch len(matches) {
	case 0:
		return filespec.Match{}, fmt.Errorf("copy: %w", &NotFoundError{Spec: specText})
	case 1:
		return matches[0], nil
	default:
		return filespec.Match{}, fmt.Errorf("copy: %w", &MultipleMatchesError{Spec: specText, Count: len(matches)})
	}
}

// matchDisplay renders a matched source file as plain "NAME.TYPE;version"
// text (no device/directory -- the same convention Console.Delete's own
// confirmation line already uses), for CopyResult.Source and, in
// copyToHost, as the literal filename written under an existing host
// directory.
func matchDisplay(m filespec.Match) string {
	return fmt.Sprintf("%s.%s;%d", m.Name, m.Type, m.Version)
}

// destNameType resolves the actual name/type to create for a volume
// destination: destSpec's own Name/Type, if its text supplied one, or
// fallbackName/fallbackType (the source file's own name/type, or a
// /HOST source's host-derived name/type) if destText named no file of
// its own at all -- e.g. "COPY FOO.TXT DUA0:[SUBDIR]" naming only a
// directory. Unlike ods2's own volumeDestName, this subtask's "exactly
// one source file" restriction means there's no wildcard-substitution
// case ('*'/'%' in destSpec's own Name/Type) to handle -- that only
// matters once several source files can be copied by one command, out of
// scope here (see this file's own top-of-file doc comment).
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
// Stream_LF is the only format this subtask creates -- absent a future
// /BINARY qualifier (subtask 10), there is no other destination format to
// choose between yet, matching ods2's own cmdCopy default.
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

	// CreateFile/Directory.Insert already assigned the version actually
	// used (the highest that existed for fullName before this call, plus
	// one); looking it back up via Lookup, rather than threading it back
	// out of CreateFile itself, keeps that bookkeeping entirely inside
	// Directory, where it already lives -- matching ods2's own identical
	// choice in copyOneFileToVolume/copyHostFileToVolume.
	entry, err := destDir.Lookup(fullName, 0)
	if err != nil {
		return 0, fmt.Errorf("looking up newly created %s: %w", fullName, err)
	}

	return entry.Version, nil
}

// copyRecordsAsLines reframes src's records (whatever its own record
// format is -- Fixed, Variable, VFC, Stream) as a sequence of individual
// Stream_LF records on w, reusing writeRecords' own record-format-aware
// text rendering (VFC carriage-control expansion and all) via
// lineSplitWriter, which re-splits that rendered text back into
// individual '\n'-delimited records for w.Put to apply Stream_LF's own
// framing to. Functionally matches ods2's own copyRecordsToVolume.
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
