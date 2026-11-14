package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// A command's UICs and protection codes (docs/planning/PHASE-93 - file
// protection.md, Decision 9). The grammar's $UIC and $PROTECTION types
// have checked their shape; what they mean is read here, as each VMS
// utility reads it, and a fault is the command's own message.

// ssNOSUCHID is SS$_NOSUCHID, a UIC naming an identifier the rights
// database doesn't have.
var ssNOSUCHID = vmsdef.Symbols["SS$_NOSUCHID"]

// identifierLookup is the rights database of the process running the
// command, for security.ParseUIC's names; nil without a process.
func (c *Context) identifierLookup() security.IdentifierLookup {
	if c.RTL == nil || c.RTL.Process == nil {
		return nil
	}

	return func(name string) (uint32, bool) {
		v, status := c.RTL.IdentifierValue(name)

		return v, status == 0
	}
}

// uicStatus is the secondary status for security.ParseUIC's error: a name
// the database doesn't have is SS$_NOSUCHID, anything else SS$_IVIDENT.
func uicStatus(err error) uint32 {
	var missing security.ErrNoSuchIdentifier
	if errors.As(err, &missing) {
		return ssNOSUCHID
	}

	return ssIVIDENT
}

// protectionFault is the part of a protection code, code, a command's
// %xxx-F-SYNTAX names for security.ParseProtection's error: for letters
// that aren't R, W, E, and D, the letters alone, as VMS 7.1's
// CREATE/DIRECTORY named 'Z' of (S:Z); for anything else the whole code
// (unconfirmed for CREATE).
func protectionFault(err error, code string) string {
	var bad security.ErrBadAccess
	if errors.As(err, &bad) {
		if _, letters, ok := strings.Cut(strings.Replace(bad.Item, "=", ":", 1), ":"); ok {
			return letters
		}
	}

	return code
}
