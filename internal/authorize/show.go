package authorize

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file is AUTHORIZE's reports: SHOW of accounts, identifiers, and
// rights, and LIST, which writes the same reports into SYSUAF.LIS and
// RIGHTSLIST.LIS in the default directory (Utility.WriteListing), each as
// VMS 7.1's did in testdata/uaf95 (the phase doc's A.5).

// Statuses the reports leave, as VMS 7.1 left them: SHOW of a username
// that matches nothing RMS's RNF, SHOW/RIGHTS always CLI's ABSENT, each
// with the inhibit bit (cases 129, 95, and 101); LIST always RMS$_NORMAL,
// even when it wrote nothing (cases 117 and 120, round 3's L12).
var (
	showNoMatch  = vmsdef.Symbols["RMS$_RNF"] | stsInhibitMsg
	showRights   = vmsdef.LibrarySymbols["CLI$_ABSENT"] | stsInhibitMsg
	listStatus   = vmsdef.Symbols["RMS$_NORMAL"]
	ssNOSUCHID   = vmsdef.Symbols["SS$_NOSUCHID"]
	ssIVIDENT    = vmsdef.Symbols["SS$_IVIDENT"]
	ssNOPRIV     = vmsdef.Symbols["SS$_NOPRIV"]
	inhibited    = func(code uint32) uint32 { return code | stsInhibitMsg }
	listAccounts = "SYSUAF.LIS"
	listRights   = "RIGHTSLIST.LIS"
)

// readFile reads the authorization file for a report, after the access
// check; a file that can't be read is NAOFIL with what went wrong.
func (u *Utility) readFile() (*uaf.File, bool) {
	if !u.mayRead() {
		return nil, false
	}

	f, err := u.Store.Authorization()
	if err != nil {
		u.message("NAOFIL", "")
		fmt.Fprintf(u.Out, "-GOVAX-E-FILE, %v\n", err)

		return nil, false
	}

	return f, true
}

// matching returns f's accounts whose usernames match pattern, in
// username order (SYSUAF.DAT's primary key; the probe's case 49).
func matching(f *uaf.File, pattern string) []*uaf.Record {
	pattern = strings.ToUpper(strings.TrimSpace(pattern))

	var found []*uaf.Record

	for i := range f.Accounts {
		if wildcardMatch(pattern, f.Accounts[i].Username) {
			found = append(found, &f.Accounts[i])
		}
	}

	slices.SortFunc(found, func(a, b *uaf.Record) int { return strings.Compare(a.Username, b.Username) })

	return found
}

// accountReport is the report of the accounts pattern matches: brief, or
// full with the identifiers held when full. ok is false when none match.
func accountReport(f *uaf.File, pattern string, brief, full bool) (lines []string, ok bool) {
	found := matching(f, pattern)
	if len(found) == 0 {
		return nil, false
	}

	if brief {
		lines = slices.Clone(briefHeading)

		for _, r := range found {
			lines = append(lines, briefLine(r))
		}

		return lines, true
	}

	for _, r := range found {
		lines = append(lines, fullReport(f, r, full)...)
	}

	return lines, true
}

// show is SHOW username [/BRIEF | /FULL]: each account's full report
// (with /FULL, the identifiers it holds too) or a brief line, BADSPC when
// the username matches nothing.
func (u *Utility) show(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	lines, ok := accountReport(f, r.String("USERNAME"), r.Present("BRIEF"), r.Present("FULL"))
	if !ok {
		u.message("BADSPC", "")
		u.status = showNoMatch

		return nil
	}

	u.write(lines)

	return nil
}

// list is LIST [username] [/BRIEF | /FULL]: SHOW's report of the
// accounts (every one by default, brief unless /FULL) written into
// SYSUAF.LIS. A username that matches nothing writes an empty file
// (unconfirmed).
func (u *Utility) list(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	pattern := "*"
	if r.Present("USERNAME") {
		pattern = r.String("USERNAME")
	}

	lines, _ := accountReport(f, pattern, !r.Present("FULL"), r.Present("FULL"))

	u.message("LSTMSG1", "")

	if err := u.writeListing(listAccounts, lines); err != nil {
		u.listFailed(err)

		return nil
	}

	u.message("LSTMSG2", "")
	u.status = listStatus

	return nil
}

// writeListing writes a listing file through the Utility's WriteListing.
func (u *Utility) writeListing(name string, lines []string) error {
	if u.WriteListing == nil {
		return fmt.Errorf("no listing files here")
	}

	return u.WriteListing(name, lines)
}

// listFailed says a listing file couldn't be written: LSTERR with what
// went wrong, the status still RMS$_NORMAL as VMS 7.1 left it.
func (u *Utility) listFailed(err error) {
	u.message("LSTERR", "")

	if code, ok := err.(conditionError); ok {
		u.cause(uint32(code))
	} else {
		fmt.Fprintf(u.Out, "-GOVAX-E-FILE, %v\n", err)
	}

	u.status = listStatus
}

// conditionError is a failure that is a VMS condition value, written
// under AUTHORIZE's message as its own.
type conditionError uint32

func (e conditionError) Error() string { return fmt.Sprintf("condition %%X%08X", uint32(e)) }

// write writes lines to SYS$OUTPUT.
func (u *Utility) write(lines []string) {
	for _, line := range lines {
		fmt.Fprintln(u.Out, line)
	}
}

// identifiers returns every identifier the rights database has, the
// file's and the environmental ones, in name order (RIGHTSLIST.DAT's
// key; the probe's case 6).
func identifiers(f *uaf.File) []uaf.Identifier {
	all := slices.Clone(f.Identifiers)

	for _, env := range uaf.Environmental {
		if !slices.ContainsFunc(all, func(id uaf.Identifier) bool { return id.Name == env.Name || id.Value == env.Value }) {
			all = append(all, env)
		}
	}

	slices.SortFunc(all, func(a, b uaf.Identifier) int { return strings.Compare(a.Name, b.Name) })

	return all
}

// findIdentifier returns the identifier called name, in the file or among
// the environmental ones.
func findIdentifier(f *uaf.File, name string) (uaf.Identifier, bool) {
	if id, ok := f.IdentifierNamed(name); ok {
		return *id, true
	}

	return uaf.EnvironmentalNamed(name, 0)
}

// selectIdentifiers is what SHOW/IDENTIFIER and LIST/IDENTIFIER report:
// the identifier name names, every one for "*", or the one with /VALUE's
// value. A name with a wildcard among other characters is IVIDENT, as on
// VMS 7.1 ("PROJ*", case 87), and nothing found NOSUCHID. noName says
// neither a name nor a value was given.
func (u *Utility) selectIdentifiers(f *uaf.File, r *dcl.Result) (found []uaf.Identifier, code uint32, noName bool) {
	name := strings.ToUpper(strings.TrimSpace(r.String("ID_NAME")))

	switch {
	case r.Present("VALUE"):
		v, ok := u.parseValue(r.String("VALUE"))
		if !ok {
			return nil, 0, false
		}

		for _, id := range identifiers(f) {
			if id.Value == v {
				return []uaf.Identifier{id}, 0, false
			}
		}

		return nil, ssNOSUCHID, false
	case !r.Present("ID_NAME") || name == "":
		return nil, 0, true
	case name == "*":
		return identifiers(f), 0, false
	case strings.ContainsAny(name, "*%"):
		return nil, ssIVIDENT, false
	}

	if id, ok := findIdentifier(f, name); ok {
		return []uaf.Identifier{id}, 0, false
	}

	return nil, ssNOSUCHID, false
}

// identifierReport is SHOW/IDENTIFIER's report of ids: the heading, a
// line each, and with holders each one's holders under it.
func identifierReport(f *uaf.File, ids []uaf.Identifier, holders bool) []string {
	lines := []string{identifierHeading}

	for _, id := range ids {
		lines = append(lines, identifierLine(id.Name, id.Value, id.Attributes))

		if holders && len(id.Holders) > 0 {
			lines = append(lines, holderHeading)

			for _, h := range id.Holders {
				lines = append(lines, holderLine(f, h))
			}
		}
	}

	return lines
}

// showIdentifier is SHOW/IDENTIFIER [name] [/VALUE=...] [/FULL]: the
// identifiers, with their holders under /FULL (case 97). One not found is
// SHOWERR with the system's reason; neither a name nor a value NOIDNAME,
// as LIST/IDENTIFIER said it (unconfirmed for SHOW).
func (u *Utility) showIdentifier(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	ids, code, noName := u.selectIdentifiers(f, r)

	switch {
	case noName:
		u.message("NOIDNAME", "")
	case code != 0:
		u.message("SHOWERR", "")
		u.cause(code)
		u.status = inhibited(code)
	case ids != nil:
		u.write(identifierReport(f, ids, r.Present("FULL")))
	}

	return nil
}

// rightsReport is SHOW/RIGHTS's report of what the UIC of the identifier
// called name holds: the heading and a line each, nothing at all when it
// holds nothing (case 101). code is NOSUCHID for no such identifier and
// IVIDENT for a wildcard (round 3's L12).
func rightsReport(f *uaf.File, name string) (lines []string, code uint32) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if strings.ContainsAny(name, "*%") {
		return nil, ssIVIDENT
	}

	id, ok := f.IdentifierNamed(name)
	if !ok || id.Value&uaf.GeneralBit != 0 {
		return nil, ssNOSUCHID
	}

	held := f.HeldBy(uaf.UIC(id.Value))
	if len(held) == 0 {
		return nil, 0
	}

	lines = []string{rightsHeading}
	for _, h := range held {
		lines = append(lines, identifierLine(h.Identifier.Name, h.Identifier.Value, h.Attributes))
	}

	return lines, 0
}

// showRights is SHOW/RIGHTS name: what the name's UIC holds, the status
// always CLI's ABSENT, as VMS 7.1 left it. A name that is no UIC
// identifier is SHOWERR with NOSUCHID (unconfirmed).
func (u *Utility) showRights(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	if !r.Present("ID_NAME") || strings.TrimSpace(r.String("ID_NAME")) == "" {
		u.message("NOIDNAME", "")

		return nil
	}

	lines, code := rightsReport(f, r.String("ID_NAME"))
	if code != 0 {
		u.message("SHOWERR", "")
		u.cause(code)
		u.status = inhibited(code)

		return nil
	}

	u.write(lines)
	u.status = showRights

	return nil
}

// listIdentifier is LIST/IDENTIFIER: SHOW/IDENTIFIER's report, always
// with the holders, into RIGHTSLIST.LIS (round 3's L8 to L10, L13); no
// name or value is NOIDNAME and no file (case 120).
func (u *Utility) listIdentifier(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	u.status = listStatus

	ids, code, noName := u.selectIdentifiers(f, r)

	switch {
	case noName:
		u.message("NOIDNAME", "")
	case code != 0:
		u.message("LSTMSG1", "")
		u.listFailed(conditionError(code))
	case ids != nil:
		u.message("LSTMSG1", "")

		if err := u.writeListing(listRights, identifierReport(f, ids, true)); err != nil {
			u.listFailed(err)

			return nil
		}

		u.message("RLSTMSG", "")
	}

	return nil
}

// listRightsCommand is LIST/RIGHTS name: SHOW/RIGHTS's report into
// RIGHTSLIST.LIS, an empty file when the UIC holds nothing (case 121); a
// wildcard is LSTERR with IVIDENT and no file (round 3's L12).
func (u *Utility) listRightsCommand(r *dcl.Result) error {
	f, ok := u.readFile()
	if !ok {
		return nil
	}

	u.status = listStatus

	if !r.Present("ID_NAME") || strings.TrimSpace(r.String("ID_NAME")) == "" {
		u.message("NOIDNAME", "")

		return nil
	}

	u.message("LSTMSG1", "")

	lines, code := rightsReport(f, r.String("ID_NAME"))
	if code != 0 {
		u.listFailed(conditionError(code))

		return nil
	}

	if err := u.writeListing(listRights, lines); err != nil {
		u.listFailed(err)

		return nil
	}

	u.message("RLSTMSG", "")

	return nil
}
