package rms

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/security"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file implements docs/planning/PHASE-23.md subtask 8: TYPE, the operator-
// console command that writes one file's content to the console as text.
// Its behavioral reference is the sibling ods2 module's own cmd/ods2/
// internal/session/type.go (read-only, per this phase's own "behavioral
// spec, not code to link against" framing -- see docs/planning/PHASE-23.md's "Why
// this phase looks different from most others"); Session.Type below
// follows that file's shape closely (the same "exactly one match required"
// rule, the same default-to-highest-version behavior) without being a
// literal port. The actual record-to-text rendering it calls into
// (writeRecords) lives in records.go, shared with subtask 10's own COPY.

// AmbiguousError reports that specText's name/type pattern matched more
// than one file on its resolved volume -- TYPE's own refusal to guess which
// one was meant, matching ods2's own cmdType, which parses its argument
// with no wildcard-expanding behavior of its own at all. Given its own type
// (matching NotMountedError/NotFoundError's own reasoning elsewhere in this
// package), the console layer can report this as a distinct VMS status
// (CLI_AMBIGUOUS, the same status internal/dcl's own verb/qualifier
// ambiguity already uses) rather than lumping it in with an ordinary
// malformed file specification.
//
// Verb names the command refusing, for the message; without one, the
// message just says wildcards aren't allowed (ReadRecordFile's case).
type AmbiguousError struct {
	Spec  string
	Count int
	Verb  string
}

func (e *AmbiguousError) Error() string {
	if e.Verb == "" {
		return fmt.Sprintf("rms: %s is ambiguous (%d files match); wildcards aren't allowed here", e.Spec, e.Count)
	}

	return fmt.Sprintf("rms: %s is ambiguous (%d files match); %s does not support wildcards", e.Spec, e.Count, e.Verb)
}

// LockedError is a file TYPE couldn't open because another opener keeps
// readers out (RMS$_FLK): Name is its full specification.
type LockedError struct {
	Name string
}

func (e *LockedError) Error() string {
	return "rms: " + e.Name + ": file currently locked by another user"
}

// typeSharing is the FAB$B_SHR TYPE opens a file with: every operation,
// so that it types a file others are reading or writing if they let it
// (unconfirmed; VMS 7.3's TYPE of a file DCL's OPEN/WRITE had open was
// RMS$_FLK, testdata/dcl50's F9).
var typeSharing = shrGet | shrPut | shrUpd | shrDel

// Type renders one file's entire content as text (see records.go's
// writeRecords for exactly how each record format is turned into readable
// lines) and returns it as a single string for the console layer to print
// -- the same "rms computes, console prints" split Session.Directory/
// Session.DefaultString already established.
//
// A specification without a wildcard names one file: an empty or omitted
// version is the highest surviving version (filespec.Parse's own ordinary
// rule, the same default DIRECTORY relies on). One with a wildcard types
// every file it matches, each under a heading, as VMS 7.1's TYPE did
// (Phase 98); a name that still matches several files without one is an
// *AmbiguousError. A pattern matching nothing at all fails with a
// *NotFoundError, the same failure Session.Delete already reports this way.
func (s *Session) Type(specText string) (string, error) {
	var buf bytes.Buffer

	// A search list types the first file found (User's Manual §11.7).
	err := s.firstSpec(specText, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := s.globChecked(vol, r.Spec)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			return &NotFoundError{Spec: specText}
		}

		// A wildcard types every file it matches, each under a heading
		// of its name between lines of one blank, as VMS 7.1's TYPE did
		// (testdata/bat98's TYPE BAT98$DISK:[BAT98LOG]BAT98R1.LOG;*):
		//
		//	 (a blank)
		//	DUA1:[BAT98LOG]BAT98R1.LOG;1
		//	 (a blank)
		if !hasWildcard(r.Spec) {
			if len(matches) > 1 {
				return &AmbiguousError{Spec: specText, Count: len(matches), Verb: "TYPE"}
			}

			return s.typeMatch(&buf, vol, r, matches[0])
		}

		for _, m := range matches {
			fmt.Fprintf(&buf, " \n%s\n \n", r.shownFile(m))

			if err := s.typeMatch(&buf, vol, r, m); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	return buf.String(), nil
}

// typeMatch writes the file m's records to buf as TYPE shows them.
func (s *Session) typeMatch(buf *bytes.Buffer, vol *volume.Volume, r resolvedSpec, m filespec.Match) error {
	// TYPE is one of the file's RMS openers while it reads
	// (sharing.go): a file another opener keeps readers out of is
	// RMS$_FLK.
	claim := s.Mounts.claimOpen(r.Spec.Device, fileIDFrom(m.Fid), facGet, typeSharing)
	if claim == nil {
		return &LockedError{Name: fullSpec(r.Spec.Device, m)}
	}

	defer claim.release()

	f, err := s.openChecked(vol, m, security.Read, r.shownFile(m))
	if err != nil {
		return err
	}

	if isIndexed(f) {
		return writeIndexedRecords(buf, f)
	}

	if f.Header.RecordAttributes.Attributes&ondisk.AttrFortranCC != 0 {
		return writeFortranRecords(buf, f)
	}

	return writeRecords(buf, f, lfLineEnding)
}

// writeIndexedRecords writes an indexed file's records to w as TYPE shows
// them, by the primary key, each ending in LF (Phase 88, subtask 9: the order
// is a sequential $GET's on a stream just connected, unconfirmed for TYPE).
func writeIndexedRecords(w io.Writer, f *volume.File) error {
	file, err := openIndexed(f, false)
	if err != nil {
		return err
	}

	var failed error

	err = file.Walk(0, func(e isam.Entry) bool {
		record, err := file.Record(e.RFA)
		if err == nil {
			_, err = w.Write(append(record, '\n'))
		}

		failed = err

		return err == nil
	})
	if err != nil {
		return err
	}

	return failed
}

// writeFortranRecords writes f's records to w as TYPE shows a file of FORTRAN
// carriage control (FAB$M_FTN): each record's first byte is its control, as at
// the terminal (ftncc.go), and not part of the text. A file FORTRAN wrote with
// no blank to spare loses a letter this way: VMS 7.1's TYPE shows the record
// "unit 13" as "nit 13" and "*" as an empty line (testdata/for83's part B). A
// line a "$" record leaves open at the end of the file is ended, for the
// command's next output (unconfirmed).
func writeFortranRecords(w io.Writer, f *volume.File) error {
	r, err := odsrms.NewReader(f)
	if err != nil {
		return err
	}

	open := false

	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		var text []byte

		text, open = ftnTerminalText(rec, open)
		if _, err := w.Write(text); err != nil {
			return err
		}
	}

	if open {
		_, err := w.Write(lfLineEnding)

		return err
	}

	return nil
}
