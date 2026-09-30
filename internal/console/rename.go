package console

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console RENAME command, VMS DCL's RENAME:
//
//	RENAME input-filespec[,...] output-filespec
//
// internal/rms.Session.Rename does the renaming (which files, their new
// names, and the $RENAME for each -- see internal/rms/renamecmd.go); this
// file shows the results the way VMS's RENAME utility (V7.3 CLIUTL
// RENAME.B32) does. With /LOG, each file renamed gets
//
//	%RENAME-I-RENAMED, DUA0:[A]X.TXT;1 renamed to DUA0:[B]Y.TXT;1
//
// and each failure gets a message saying which file, followed by the RMS
// status (and its secondary status, when that says more) -- RENAME.B32's
// ERROR_ROUTINE:
//
//	%RENAME-E-SEARCHFAIL, error searching for DUA0:[A]NOPE.TXT;
//	-RMS-E-FNF, file not found
//
// A file that can't be found is SEARCHFAIL; a new name that can't be
// parsed, or can't be entered (the name and version exist), is OPENOUT;
// a rename to another device is NOTRENAMED/NOTSAMEDEV; anything else is
// OPENIN. As on VMS, RENAME carries on with the other files after a
// failure, and fails as a whole if any file did -- with its messages
// already shown, so the console doesn't show the failure again
// (vmserrors.InhibitMessage).

// The RMS statuses RENAME's messages depend on.
var (
	rmsDEV = vmsdef.Symbols["RMS$_DEV"]
	rmsENT = vmsdef.Symbols["RMS$_ENT"]
)

// Rename renames the files inputs name to output (see this file's opening
// comment). log is /LOG; newVersion is /NEW_VERSION, on unless
// /NONEW_VERSION.
func (c *Console) Rename(inputs []string, output string, log, newVersion bool) error {
	results := c.ContainerSession.Rename(inputs, output, rms.RenameOptions{NewVersion: newVersion})

	var failure string

	for _, r := range results {
		if r.OK() {
			if log {
				c.Printf("%%RENAME-I-RENAMED, %s renamed to %s\n", r.Old, r.New)
			}

			continue
		}

		lines := renameFailureMessage(r)
		for _, line := range lines {
			c.Printf("%s\n", line)
		}

		if failure == "" {
			failure = strings.TrimPrefix(lines[0], "%")
		}
	}

	if failure != "" {
		return vmserrors.InhibitMessage(errors.New(failure))
	}

	return nil
}

// renameFailureMessage is the message RENAME.B32's ERROR_ROUTINE shows
// for r, a line at a time.
func renameFailureMessage(r rms.RenamedFile) []string {
	var head string

	switch {
	case r.Stage == rms.RenameSearching:
		head = "%RENAME-E-SEARCHFAIL, error searching for " + r.Old

	case r.Stage == rms.RenameParsing, r.Status == rmsENT:
		head = "%RENAME-E-OPENOUT, error opening " + r.New + " as output"

	case r.Status == rmsDEV:
		return []string{
			"%RENAME-E-NOTRENAMED, " + r.Old + " not renamed",
			"-RENAME-E-NOTSAMEDEV, Cannot RENAME to a different device",
		}

	default:
		head = "%RENAME-E-OPENIN, error opening " + r.Old + " as input"
	}

	lines := []string{head, conditionLine(r.Status)}

	if r.STV != 0 && r.STV != r.Status {
		lines = append(lines, conditionLine(r.STV))
	}

	return lines
}

// conditionLine is condition value code's message as a continuation line,
// "-RMS-E-FNF, file not found", as $PUTMSG shows a message after the
// first.
func conditionLine(code uint32) string {
	severity := [8]string{"W", "S", "E", "I", "F", "?", "?", "?"}[code&7]

	m, ok := vmsdef.LookupMessage(code)
	if !ok {
		return fmt.Sprintf("-NONAME-%s-NOMSG, Message number %08X", severity, code)
	}

	return "-" + m.Facility + "-" + severity + "-" + m.Ident + ", " + m.Text
}
