package rms

import (
	"bytes"
	"fmt"

	"github.com/tucats/ods2/filespec"
)

// This file implements docs/PHASE-23.md subtask 8: TYPE, the operator-
// console command that writes one file's content to the console as text.
// Its behavioral reference is the sibling ods2 module's own cmd/ods2/
// internal/session/type.go (read-only, per this phase's own "behavioral
// spec, not code to link against" framing -- see docs/PHASE-23.md's "Why
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
// (CLI_AMBIGUOUS, the same status internal/console/dcl's own verb/qualifier
// ambiguity already uses) rather than lumping it in with an ordinary
// malformed file specification.
type AmbiguousError struct {
	Spec  string
	Count int
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("rms: %s is ambiguous (%d files match); TYPE does not support wildcards", e.Spec, e.Count)
}

// Type renders one file's entire content as text (see records.go's
// writeRecords for exactly how each record format is turned into readable
// lines) and returns it as a single string for the console layer to print
// -- the same "rms computes, console prints" split Session.Directory/
// Session.DefaultString already established.
//
// specText must resolve to exactly one file: an empty or omitted version
// defaults to the highest surviving version (filespec.Parse's own ordinary
// rule, the same default DIRECTORY relies on), but a name/type pattern
// matching more than one file fails with an *AmbiguousError rather than
// picking one arbitrarily or typing all of them -- TYPE never accepts
// wildcards, matching ods2's own cmdType. A pattern matching nothing at all
// fails with a *NotFoundError, the same failure Session.Delete already
// reports this way.
func (s *Session) Type(specText string) (string, error) {
	vol, spec, err := s.resolveVolume(specText)
	if err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("type: %w", &NotFoundError{Spec: specText})
	case 1:
		// Exactly one match -- proceed.
	default:
		return "", fmt.Errorf("type: %w", &AmbiguousError{Spec: specText, Count: len(matches)})
	}

	f, err := vol.OpenFID(matches[0].Fid)
	if err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	var buf bytes.Buffer

	if err := writeRecords(&buf, f, lfLineEnding); err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	return buf.String(), nil
}
